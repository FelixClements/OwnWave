package stations

import (
	"strings"
	"testing"
)

func TestStyleStationsUseTheirOwnThresholds(t *testing.T) {
	cases := []struct {
		kind          string
		active        bool
		tracks        int
		want          bool
		wantShowAt    int
		wantHideBelow int
	}{
		{CatalogStyle, false, StyleStationShowAt, true, StyleStationShowAt, StyleStationHideBelow},
		{CatalogStyle, false, StyleStationShowAt - 1, false, StyleStationShowAt, StyleStationHideBelow},
		{CatalogStyle, true, StyleStationHideBelow, true, StyleStationShowAt, StyleStationHideBelow},
		{CatalogStyle, true, StyleStationHideBelow - 1, false, StyleStationShowAt, StyleStationHideBelow},
		{CatalogGenre, false, GenreStationShowAt, true, GenreStationShowAt, GenreStationHideBelow},
		{CatalogGenre, true, GenreStationHideBelow - 1, false, GenreStationShowAt, GenreStationHideBelow},
	}
	for _, c := range cases {
		a := autoStation{kind: c.kind, active: c.active, tracks: c.tracks, stillAStationGenre: true}
		if got := a.shouldShow(true); got != c.want {
			t.Errorf("%s active=%v tracks=%d: shown %v, want %v", c.kind, c.active, c.tracks, got, c.want)
		}
		if show, hide := a.thresholds(); show != c.wantShowAt || hide != c.wantHideBelow {
			t.Errorf("%s thresholds %d/%d", c.kind, show, hide)
		}
	}
	if (autoStation{kind: CatalogStyle, tracks: 100, stillAStationGenre: true}).shouldShow(false) {
		t.Error("switched off hides style stations too")
	}
}

func TestSongsWithGenresSkipsUnsureSongsAndUnknownStyles(t *testing.T) {
	sql := songsWithGenres("$1", true)
	for _, want := range []string{"g.is_primary", "d.genre IS NOT NULL", "kind = 'style'"} {
		if !strings.Contains(sql, want) {
			t.Errorf("missing %q in %s", want, sql)
		}
	}
	if strings.Contains(songsWithGenres("$1", false), "is_primary") {
		t.Error("a custom station's genre filter takes every station genre")
	}
}
