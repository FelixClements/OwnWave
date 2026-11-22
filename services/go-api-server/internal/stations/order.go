package stations

import (
	"math/rand/v2"
	"sort"
	"strings"
	"time"
)

// artistGap is how many other tracks must play before an artist repeats,
// when the pool allows it.
const artistGap = 3

// spreadArtists reorders items so the same artist does not play twice
// within gap tracks, keeping the given order as far as it can. At each
// position it takes the first remaining track whose artist is not among
// the last gap artists, except that an artist with just enough slots
// left to keep its spacing goes first. When no track qualifies (e.g. a
// one-artist pool) it takes the first remaining track.
func spreadArtists(items []queued, gap int) []queued {
	out := make([]queued, 0, len(items))
	rest := append([]queued(nil), items...)
	remaining := map[string]int{}
	for _, it := range rest {
		if a := artistKey(it.track); a != "" {
			remaining[a]++
		}
	}
	recent := make([]string, 0, gap)
	for len(rest) > 0 {
		pick, found := 0, false
		for i, it := range rest {
			a := artistKey(it.track)
			if containsArtist(recent, a) {
				continue
			}
			// c tracks spaced gap apart need (c-1)*(gap+1)+1 slots.
			if a != "" && (remaining[a]-1)*(gap+1)+1 >= len(rest) {
				pick, found = i, true
				break
			}
			if !found {
				pick, found = i, true
			}
		}
		it := rest[pick]
		rest = append(rest[:pick], rest[pick+1:]...)
		out = append(out, it)
		if a := artistKey(it.track); a != "" {
			remaining[a]--
			recent = append(recent, a)
			if len(recent) > gap {
				recent = recent[1:]
			}
		}
	}
	return out
}

func artistKey(t QueueTrack) string {
	if t.Artist == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*t.Artist))
}

func containsArtist(recent []string, a string) bool {
	if a == "" {
		return false
	}
	for _, r := range recent {
		if r == a {
			return true
		}
	}
	return false
}

// playedAgeGroup is the width of the age groups orderQueue shuffles within.
const playedAgeGroup = 7 * 24 * time.Hour

// orderQueue dedupes by title+artist, then orders songs so every one gets a
// turn: never played on this station first, then played longest ago, then
// (after everything else) those played within recentHours, least recent
// first. Never-played songs and each week-wide age group are shuffled, so
// repeat listens still vary. It then spaces out artists and truncates to a page.
func orderQueue(items []queued, recentHours int, now time.Time) []QueueTrack {
	cutoff := now.Add(-time.Duration(recentHours) * time.Hour)
	// Two entries with one title+artist play as one: it counts as played
	// when either was, so the other cannot bring it back early.
	first := make(map[string]int, len(items))
	unique := make([]queued, 0, len(items))
	for _, it := range items {
		artist := ""
		if it.track.Artist != nil {
			artist = *it.track.Artist
		}
		key := strings.ToLower(strings.TrimSpace(it.track.Title) + "|" + strings.TrimSpace(artist))
		if i, seen := first[key]; seen {
			if p := it.lastPlayed; p != nil && (unique[i].lastPlayed == nil || p.After(*unique[i].lastPlayed)) {
				unique[i].lastPlayed = p
			}
			continue
		}
		first[key] = len(unique)
		unique = append(unique, it)
	}

	var never, recent []queued
	played := map[int64][]queued{}
	for _, it := range unique {
		switch {
		case it.lastPlayed == nil:
			never = append(never, it)
		case it.lastPlayed.Before(cutoff):
			age := int64(now.Sub(*it.lastPlayed) / playedAgeGroup)
			played[age] = append(played[age], it)
		default:
			recent = append(recent, it)
		}
	}
	shuffle := func(list []queued) {
		rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })
	}
	shuffle(never)
	ages := make([]int64, 0, len(played))
	for age := range played {
		ages = append(ages, age)
	}
	sort.Slice(ages, func(i, j int) bool { return ages[i] > ages[j] })
	fresh := never
	for _, age := range ages {
		shuffle(played[age])
		fresh = append(fresh, played[age]...)
	}
	sort.SliceStable(recent, func(i, j int) bool { return recent[i].lastPlayed.Before(*recent[j].lastPlayed) })

	ordered := spreadArtists(append(fresh, recent...), artistGap)
	if len(ordered) > QueuePageSize {
		ordered = ordered[:QueuePageSize]
	}
	queue := make([]QueueTrack, len(ordered))
	for i, it := range ordered {
		queue[i] = it.track
		queue[i].Position = i
	}
	return queue
}
