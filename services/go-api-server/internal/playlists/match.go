package playlists

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"ownwave/api/internal/spotify"
)

// LocalTrack is a library track the matcher can pick.
type LocalTrack struct {
	ID          string
	Title       string
	Artist      string
	DurationSec float64
}

const (
	MethodExact = "exact"
	MethodFuzzy = "fuzzy"
	MethodNone  = "none"

	// minFuzzySimilarity is the title similarity a same-artist track needs
	// when the normalised titles differ.
	minFuzzySimilarity = 0.85
	// maxNoArtistDurationDiff bounds the duration gap for local tracks
	// without an artist tag, which can only match on title + duration.
	maxNoArtistDurationDiff = 3.0
)

// Matcher resolves Spotify tracks to local tracks by artist, title, version
// (remix/live/…) and, as a tiebreak, duration.
type Matcher struct {
	tracks   []indexed
	byKey    map[string][]int
	byArtist map[string][]int
}

type indexed struct {
	LocalTrack
	key     string
	artists []string
}

func NewMatcher(tracks []LocalTrack) *Matcher {
	m := &Matcher{byKey: map[string][]int{}, byArtist: map[string][]int{}}
	for i, t := range tracks {
		it := indexed{LocalTrack: t, key: titleKey(t.Title), artists: artistNames(t.Artist)}
		m.tracks = append(m.tracks, it)
		m.byKey[it.key] = append(m.byKey[it.key], i)
		for _, a := range it.artists {
			m.byArtist[a] = append(m.byArtist[a], i)
		}
	}
	return m
}

// Match returns the local track id for t and how it was found, or ok=false.
func (m *Matcher) Match(t spotify.Track) (trackID, method string, ok bool) {
	var artists []string
	for _, a := range t.Artists {
		artists = append(artists, artistNames(a)...)
	}
	key := titleKey(t.Title)
	durSec := float64(t.DurationMS) / 1000

	var exact []int
	for _, i := range m.byKey[key] {
		c := m.tracks[i]
		if overlaps(c.artists, artists) ||
			(len(c.artists) == 0 && c.DurationSec > 0 && math.Abs(c.DurationSec-durSec) <= maxNoArtistDurationDiff) {
			exact = append(exact, i)
		}
	}
	if best, found := m.closest(exact, durSec); found {
		return m.tracks[best].ID, MethodExact, true
	}

	bestSim, fuzzy := 0.0, []int{}
	seen := map[int]bool{}
	for _, a := range artists {
		for _, i := range m.byArtist[a] {
			if seen[i] {
				continue
			}
			seen[i] = true
			sim := similarity(key, m.tracks[i].key)
			switch {
			case sim < minFuzzySimilarity || sim < bestSim:
			case sim > bestSim:
				bestSim, fuzzy = sim, []int{i}
			default:
				fuzzy = append(fuzzy, i)
			}
		}
	}
	if best, found := m.closest(fuzzy, durSec); found {
		return m.tracks[best].ID, MethodFuzzy, true
	}
	return "", MethodNone, false
}

// closest picks the candidate whose duration is nearest to durSec.
func (m *Matcher) closest(cands []int, durSec float64) (int, bool) {
	if len(cands) == 0 {
		return 0, false
	}
	sort.SliceStable(cands, func(a, b int) bool {
		return math.Abs(m.tracks[cands[a]].DurationSec-durSec) < math.Abs(m.tracks[cands[b]].DurationSec-durSec)
	})
	return cands[0], true
}

func overlaps(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

var (
	bracketed    = regexp.MustCompile(`[(\[]([^)\]]*)[)\]]`)
	featuring    = regexp.MustCompile(`(?i)^(feat\.?|ft\.?|featuring|with)\s`)
	trailingFeat = regexp.MustCompile(`(?i)\s(feat\.?|ft\.?|featuring)\s.*$`)
	artistSep    = regexp.MustCompile(`(?i)\s*(,|&|;|/|\sx\s|\sfeat\.?\s|\sft\.?\s|\sfeaturing\s|\svs\.?\s|\sand\s)\s*`)
	year         = regexp.MustCompile(`^(19|20)\d\d$`)
)

// versionNoise are words that name an edit or release of the same
// recording, so "Extended Mix" matches "Radio Edit" or no version at all.
var versionNoise = map[string]bool{
	"original": true, "mix": true, "extended": true, "radio": true, "edit": true,
	"version": true, "remaster": true, "remastered": true, "club": true, "album": true,
	"single": true, "explicit": true, "clean": true, "mono": true, "stereo": true, "digital": true,
}

// titleKey normalises a title to "base|version": featured artists are
// dropped, and the version (bracketed or after " - ") keeps only the words
// that make it a different recording, e.g. "space 92 remix".
func titleKey(title string) string {
	var versions []string
	base := bracketed.ReplaceAllStringFunc(title, func(group string) string {
		versions = append(versions, group[1:len(group)-1])
		return " "
	})
	if i := strings.Index(base, " - "); i >= 0 {
		versions = append(versions, base[i+3:])
		base = base[:i]
	}
	base = trailingFeat.ReplaceAllString(base, "")

	var words []string
	for _, v := range versions {
		if featuring.MatchString(strings.TrimSpace(v)) {
			continue
		}
		for _, w := range strings.Fields(normalize(v)) {
			if !versionNoise[w] && !year.MatchString(w) {
				words = append(words, w)
			}
		}
	}
	sort.Strings(words)
	return normalize(base) + "|" + strings.Join(words, " ")
}

// artistNames splits an artist credit ("A & B feat. C") into normalised names.
func artistNames(credit string) []string {
	var out []string
	for _, a := range artistSep.Split(credit, -1) {
		if a = normalize(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

var stripMarks = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// normalize lowercases, drops accents and punctuation, and spells "&" as "and".
func normalize(s string) string {
	s, _, _ = transform.String(stripMarks, strings.ToLower(s))
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		if r == '\'' || r == '’' {
			return -1 // "don't" == "dont"
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// similarity is 1 - normalised Levenshtein distance.
func similarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 && len(rb) == 0 {
		return 1
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return 1 - float64(prev[len(rb)])/float64(max(len(ra), len(rb)))
}
