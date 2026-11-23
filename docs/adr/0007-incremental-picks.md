# 0007: Picks follow the library song by song

Status: accepted, 2026-11-23. Migration 033. Issue #132. Replaces the library
signature of [ADR 0002](0002-library-genre-stations.md) decision 8; the pick
rules of decision 7 stay.

## Context

ADR 0002 rebuilt a playlist's picks whenever its input changed. The input
held a library signature: the number of embeddings, the newest
`analyzed_at` and the number of copies. Every newly analysed track changed
it, so every background run (`scan_finished`, `genres_scored`, the hourly
tick, startup) rebuilt all 69 playlists. Each rebuild searched the 5 nearest
songs of every playlist track: 4.7k searches over 7.3k embeddings. With
exact scans that was 14 minutes at 100% CPU. The HNSW index (migration 032)
cut it to about 6 s, but one new song still rebuilt everything, rebuilds
could run back to back, and nothing logged them.

On the live library, songs are close together: the median song has 280 others
at similarity 0.7 or more, the 90th percentile 806. A playlist's own songs
are often each other's nearest, so filtering them out can empty a short list.
Going down each playlist track's nearest songs, the fifth song that survives
the playlist's filters (or the end of the songs above the floor) sits at
position 5 or less for 41% of tracks, 10 or less for 83%, 20 or less for 93%,
and past 120 for 3.4%.

## Decision

1. **Each song keeps a list of its nearest songs.** `song_neighbours` holds,
   for every best copy with an embedding, its nearest best copies with cosine
   similarity 0.7 or more, up to the list's depth. `song_neighbour_lists`
   holds one row per list: the `analyzed_at` of the embedding it was built
   from, the rules version, the depth and a revision that changes whenever
   the list's contents change. A list with fewer rows than its depth holds
   every song above the floor, so it is complete.

2. **Depth starts at 20 and grows per song.** A playlist drops its own
   songs and removed picks from each of its songs' lists. When a full list
   then offers fewer than 5, it grows 4 times (80, 320, …) until it offers
   5 or is complete. On the live library 338 of 7353 lists grew. Growing
   only adds songs past a list's end, so other playlists' picks stay the
   same and the revision stays too. A fixed depth would either waste rows
   (120 still misses 3.4% of tracks) or need a live search as fallback,
   which would make those playlists follow the whole library again.

3. **The Go pipeline keeps the lists, from a diff.** `RebuildPicks` with
   `followLibrary` (background runs, after `GroupCopies` and
   `MatchPending`) first compares the library with `song_neighbour_lists`:
   - Songs without a list, or whose `analyzed_at` or rules version changed,
     get a new list through the HNSW index.
   - A list that holds a song that was deleted, became a copy or got a new
     embedding is rebuilt: with an entry missing it would look complete.
   - A song that is gone or became a copy loses its list.
   - New songs enter existing lists they now belong in. A full list takes a
     song closer than its last entry and drops the farthest, and a shorter
     list takes any song above the floor. Similarity is symmetric, so this
     compares each new song with every list's song exactly, once: about 60 ms
     per new song. With more than 200 new songs, or new rules, every list
     is rebuilt through the index instead (about 20 s).

   A diff rather than a watermark such as "`analyzed_at` after the last run":
   a watermark misses deleted songs and copy changes, and a crashed run would
   need its own recovery. The diff reads no embeddings when nothing changed
   (about 40 ms), it is idempotent, and the first run after the migration
   fills the empty tables with the same code. Not triggers: the analytics
   worker writes `track_audio_genres` in bulk, so per-row work would run
   inside its transactions and against Go's table ownership
   ([ADR 0005](0005-table-ownership.md)). The pipeline already runs after
   every scan.

4. **A playlist's input is its own.** `picks_input` holds the rules version,
   a hash of the playlist's songs, a hash of its removed picks, and a hash of
   its songs' list revisions. Only a change to one of those rebuilds the
   playlist. A new song rebuilds only the playlists whose songs it is near. A
   removal now refills its place at the next run, which before had to wait
   for a library change. Requests (`followLibrary` off) do not touch the
   lists; they rebuild playlists whose own songs or removals changed.

5. **Rebuilding is a join.** A playlist's picks come from its songs' lists:
   each song offers its first 5 songs that are not in the playlist, not
   removed and not copies. The ranking (hits, then best similarity), the 0.7
   floor, the title and artist dedupe in `choosePicks`, the limit of one pick
   per playlist song, permanent removals, and picks that another playlist
   matches all stay as ADR 0002 decision 7 set them.

6. **Every rebuild is logged**: duration, playlists rebuilt, lists
   recomputed, extended, removed and grown, and whether all lists were
   rebuilt.

## Measurements

On a copy of the live database (7474 embeddings, 7353 songs, 69 playlists,
4705 playlist tracks):

| Run | Time | Playlists rebuilt |
|---|---|---|
| First run: fill every list, rebuild every playlist | 24 s | 69 |
| Nothing changed | 0.08 s | 0 |
| One new song near a playlist song | 0.7 s | 5 |
| Ten new songs | 2.1 s | 34 |
| Ten songs deleted | 1.7 s | 36 |
| 150 new songs | 11 s | 67 |
| Before: any library change, per-track HNSW searches | 6.4 s | 69 |

The picks after the first run were identical to the live picks: 4527 of
4527, same order. After the changes above, rebuilding every list from nothing
gave the same picks as the incremental runs.

## Consequences

- 169k list rows, 28 MB with indexes. The first run after deploying holds
  the picks lock for about 25 s.
- A song analysed again rebuilds the lists it is in even when its embedding
  did not change: ten such songs cost 3 s and rebuilt 403 lists. Re-analysis
  happens after a file change or a new model, so this is rare; storing a
  hash of the embedding would avoid it.
- The lists come from the HNSW index, which is approximate. On the live
  library it agreed with exact scans. Adding new songs to lists is exact.
- Lists only grow. A song that stops being a playlist track keeps its longer
  list until its embedding or the rules change.
- `pickNeighbours` and `minPickSimilarity` are part of the list rules: changing
  them means bumping `neighboursVersion` as well as `picksVersion`.
