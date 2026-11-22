package playlists

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// copiesLock serialises GroupCopies across requests and API instances.
const copiesLock = 4_242_102

// sameRecording says whether two library files hold one recording: the
// same normalised title and version, a shared artist and the same length.
// Files without a title, an artist or a length are never grouped.
func sameRecording(a, b indexed) bool {
	return a.key == b.key && !strings.HasPrefix(a.key, "|") && overlaps(a.artists, b.artists) &&
		a.DurationSec > 0 && b.DurationSec > 0 && math.Abs(a.DurationSec-b.DurationSec) <= sameLengthWithin
}

// findCopies groups the files of one song and returns, for every file that
// is not its song's best copy, the best copy's id. The best copy is the one
// the matcher prefers: highest sample rate, then largest file; the lowest id
// breaks a tie, so the choice is stable between runs.
func findCopies(tracks []LocalTrack) map[string]string {
	byKey := map[string][]indexed{}
	for _, t := range tracks {
		it := indexed{LocalTrack: t, key: titleKey(t.Title), artists: artistNames(t.Artist)}
		byKey[it.key] = append(byKey[it.key], it)
	}
	out := map[string]string{}
	for _, files := range byKey {
		// Union-find over the files with this title.
		parent := make([]int, len(files))
		for i := range parent {
			parent[i] = i
		}
		var find func(int) int
		find = func(i int) int {
			if parent[i] != i {
				parent[i] = find(parent[i])
			}
			return parent[i]
		}
		for i := range files {
			for j := i + 1; j < len(files); j++ {
				if sameRecording(files[i], files[j]) {
					parent[find(i)] = find(j)
				}
			}
		}
		songs := map[int][]LocalTrack{}
		for i, f := range files {
			songs[find(i)] = append(songs[find(i)], f.LocalTrack)
		}
		for _, copies := range songs {
			if len(copies) < 2 {
				continue
			}
			sort.Slice(copies, func(i, j int) bool {
				if better(copies[i], copies[j]) {
					return true
				}
				if better(copies[j], copies[i]) {
					return false
				}
				return copies[i].ID < copies[j].ID
			})
			for _, c := range copies[1:] {
				out[c.ID] = copies[0].ID
			}
		}
	}
	return out
}

// loadLocal reads every library file the matcher and the grouping work on.
func loadLocal(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) ([]LocalTrack, error) {
	rows, err := q.Query(ctx, `
		SELECT t.id::text, t.title, COALESCE(a.name, ''), COALESCE(t.duration_seconds, 0),
		       COALESCE(t.sample_rate, 0), COALESCE(t.file_size, 0)
		FROM tracks t LEFT JOIN artists a ON a.id = t.artist_id
	`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (LocalTrack, error) {
		var t LocalTrack
		err := row.Scan(&t.ID, &t.Title, &t.Artist, &t.DurationSec, &t.SampleRate, &t.FileSize)
		return t, err
	})
}

// GroupCopies records which library files are copies of one song
// (track_copies) and returns how many files changed. It never touches a
// file on disk or the tracks table, which the library scan owns
// (docs/adr/0005). Running it again changes nothing.
func (s *Service) GroupCopies(ctx context.Context) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, copiesLock); err != nil {
		return 0, err
	}
	local, err := loadLocal(ctx, tx)
	if err != nil {
		return 0, err
	}
	want := findCopies(local)

	rows, err := tx.Query(ctx, `SELECT track_id::text, copy_of::text FROM track_copies`)
	if err != nil {
		return 0, err
	}
	have := map[string]string{}
	var id, copyOf string
	if _, err := pgx.ForEachRow(rows, []any{&id, &copyOf}, func() error { have[id] = copyOf; return nil }); err != nil {
		return 0, err
	}

	unset := []string{}
	for id := range have {
		if _, ok := want[id]; !ok {
			unset = append(unset, id)
		}
	}
	setIDs, setTo := []string{}, []string{}
	for id, best := range want {
		if have[id] != best {
			setIDs, setTo = append(setIDs, id), append(setTo, best)
		}
	}
	if len(unset) == 0 && len(setIDs) == 0 {
		return 0, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM track_copies WHERE track_id = ANY($1::uuid[])`, unset); err != nil {
		return 0, err
	}
	// The joins skip a file the scan deleted since loadLocal read it.
	if _, err := tx.Exec(ctx, `
		INSERT INTO track_copies (track_id, copy_of)
		SELECT v.id, v.best
		FROM unnest($1::uuid[], $2::uuid[]) AS v(id, best)
		JOIN tracks t ON t.id = v.id
		JOIN tracks b ON b.id = v.best
		ON CONFLICT (track_id) DO UPDATE SET copy_of = EXCLUDED.copy_of
	`, setIDs, setTo); err != nil {
		return 0, err
	}
	return len(unset) + len(setIDs), tx.Commit(ctx)
}
