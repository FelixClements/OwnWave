import json
import tomllib
from pathlib import Path

import pytest

import genre_catalog as gc
import genre_taxonomy as tx
from genre_catalog import Translation

LABELS = json.loads((Path(__file__).parent / "data" / "discogs400_labels.json").read_text())
CAT = gc.catalog()


def genre(g, style=None):
    return Translation.of_genre(g, style)


def fence(*genres):
    return Translation.of_fence(genres)


# --- the list ----------------------------------------------------------------------


def test_every_style_has_one_genre_and_a_valid_id():
    for sid, style in CAT.styles.items():
        assert gc.ID_PATTERN.match(sid)
        assert sid not in CAT.genres
        assert [g.id for g in CAT.genres.values() if sid in g.styles] == [style.genre]
    assert all(gc.ID_PATTERN.match(g) for g in CAT.genres)


def test_v1_ids_are_the_old_station_genres():
    assert gc.V1_GENRE_IDS == tuple(tx.STATION_GENRES)


def test_every_v1_genre_moves_to_a_new_genre():
    id_map = CAT.id_map()
    assert set(id_map) == set(gc.V1_GENRE_IDS)
    assert set(id_map.values()) <= set(CAT.genres)


def test_only_other_is_not_a_station():
    assert [g.id for g in CAT.genres.values() if not g.station] == ["other"]


# --- Discogs400 labels and Discogs styles --------------------------------------------


def test_every_discogs400_label_names_a_genre_inside_its_discogs_genre():
    for label in LABELS:
        t = CAT.discogs400_label(label)
        assert t is not None and not t.broad, label
        parent = label.split("---", 1)[0]
        assert t.genre in CAT.discogs_genre(parent).fence, label


def test_most_discogs400_labels_name_a_style():
    # Only labels that stand for a genre as a whole ("Electronic---Techno") have none.
    without = sorted(label for label in LABELS if CAT.discogs400_label(label).style is None)
    assert len(without) <= 15, without


@pytest.mark.parametrize(
    "label, expected",
    [
        ("Electronic---Techno", genre("techno")),
        ("Electronic---Hard Techno", genre("techno", "hard_techno")),
        ("Electronic---Deep House", genre("house", "deep_house")),
        ("Electronic---Psy-Trance", genre("trance", "psy_trance")),
        ("Electronic---Drum n Bass", genre("drum_and_bass", "drum_n_bass")),
        ("Electronic---Hardcore", genre("hard_dance", "hardcore")),
        ("Rock---Hardcore", genre("punk", "hardcore_punk")),
        ("Rock---Black Metal", genre("metal", "black_metal")),
        ("Rock---Deathcore", genre("metal", "deathcore")),
        ("Hip Hop---Trap", genre("hip_hop", "trap")),
        ("Funk / Soul---Disco", genre("house", "disco")),
        ("Electronic---Disco", genre("house", "disco")),
        ("Electronic---Dubstep", genre("drum_and_bass", "dubstep")),
        ("Pop---K-pop", genre("asian_pop", "k_pop")),
        ("Latin---Bossanova", genre("latin", "bossa_nova")),
        ("Jazz---Bossa Nova", genre("latin", "bossa_nova")),
        ("Non-Music---Comedy", genre("other", "comedy")),
        ("Electronic---Not A Style", fence(*CAT.discogs_genre("Electronic").fence)),
    ],
)
def test_discogs400_label(label, expected):
    assert CAT.discogs400_label(label) == expected


def test_discogs_style_uses_the_discogs_genre_for_ambiguous_names():
    assert CAT.discogs_style("Hardcore", ["Rock"]) == genre("punk", "hardcore_punk")
    assert CAT.discogs_style("Hardcore", ["Electronic"]) == genre("hard_dance", "hardcore")
    # Without it, "Hardcore" only fences.
    assert CAT.discogs_style("Hardcore") == fence("hard_dance", "punk")
    assert CAT.discogs_style("Minimal Techno", ["Electronic"]) == genre("techno", "minimal_techno")
    assert CAT.discogs_style("Nope") is None


def test_discogs_genres_only_fence():
    assert CAT.discogs_genre("Classical") == fence("classical")
    assert CAT.discogs_genre("Electronic").broad
    assert CAT.discogs_genre("Polka") is None


# --- file genre tags ---------------------------------------------------------------


@pytest.mark.parametrize(
    "tag, expected",
    [
        ("Techno", [genre("techno")]),
        ("Techno; Hard Techno", [genre("techno"), genre("techno", "hard_techno")]),
        ("Deep House", [genre("house", "deep_house")]),
        ("Big Room", [genre("house")]),
        ("Drum & Bass", [genre("drum_and_bass", "drum_n_bass")]),
        ("Drum 'n' Bass", [genre("drum_and_bass", "drum_n_bass")]),
        ("Liquid Funk", [genre("drum_and_bass", "drum_n_bass")]),
        ("Hip-Hop", [genre("hip_hop")]),
        ("Hip-Hop/Rap", [genre("hip_hop")]),
        ("Rap", [genre("hip_hop")]),
        ("R&B", [genre("soul")]),
        ("Synthpop", [genre("synth", "synth_pop")]),
        ("Hardcore Punk", [genre("punk", "hardcore_punk")]),
        ("Garage Rock", [genre("rock", "garage_rock")]),
        ("UK Garage", [genre("drum_and_bass", "uk_garage")]),
        ("Alternative Rock", [genre("rock", "alternative_rock")]),
        ("K-Pop", [genre("asian_pop", "k_pop")]),
        ("Reggaeton", [genre("latin", "reggaeton")]),
        ("Dancehall", [genre("reggae", "dancehall")]),
        ("Soundtrack", [genre("soundtrack")]),
        ("Schlager", [genre("pop", "schlager")]),
        ("Hardstyle", [genre("hard_dance", "hardstyle")]),
        ("Melodic Techno", [genre("techno")]),
        ("Afro House", [genre("house")]),
        ("Rock & Roll", [genre("rock", "rock_and_roll")]),
        ("Musique concrète", [genre("experimental", "musique_concrete")]),
        # #138 proposal: Indie & Alternative is part of Rock, Chillwave of Synth.
        ("Indie Rock", [genre("rock", "indie_rock")]),
        ("Shoegaze", [genre("rock", "shoegaze")]),
        ("Chillwave", [genre("synth", "chillwave")]),
        ("Stage & Screen", [genre("soundtrack")]),
        ("Latijnse Muziek", [genre("latin")]),
        ("Amapiano", [genre("house")]),
        # Owner approved 2026-11-25: Bass & Breaks is part of D&B, Dance & Disco of
        # House, and K-pop & J-pop is its own genre.
        ("Dubstep", [genre("drum_and_bass", "dubstep")]),
        ("Jungle", [genre("drum_and_bass", "jungle")]),
        ("Disco", [genre("house", "disco")]),
        ("Eurodance", [genre("house", "eurodance")]),
        ("J-Pop", [genre("asian_pop", "j_pop")]),
        ("City Pop", [genre("asian_pop", "city_pop")]),
    ],
)
def test_genre_tag(tag, expected):
    assert CAT.genre_tag(tag) == expected


@pytest.mark.parametrize(
    "tag, expected",
    [
        # The rule mistakes of the v1 tag rules.
        ("Industrial Metal", genre("metal")),
        ("Drone Metal", genre("metal")),
        ("Rap Metal", genre("metal")),
        ("Trap Metal", genre("metal")),
        ("Deathcore", genre("metal", "deathcore")),
        ("Garage Punk", genre("punk")),
    ],
)
def test_genre_tag_fixes_v1_rule_mistakes(tag, expected):
    assert CAT.genre_tag(tag) == [expected]


ELECTRONIC = {"techno", "house", "trance", "drum_and_bass", "hard_dance", "synth", "ambient", "experimental"}


@pytest.mark.parametrize(
    "tag, genres",
    [
        ("Pop/Rock", {"pop", "asian_pop", "rock", "metal", "punk"}),
        ("Rock/Pop", {"pop", "asian_pop", "rock", "metal", "punk"}),
        ("Electronic", ELECTRONIC),
        ("Electronica", ELECTRONIC),
        ("EDM", ELECTRONIC - {"ambient", "experimental"}),
        ("Dance", {"techno", "house", "trance", "drum_and_bass", "hard_dance", "synth", "pop", "asian_pop"}),
        ("Hardcore", {"hard_dance", "punk"}),
        ("Alternative", {"rock", "punk", "metal"}),
        ("Lo-Fi", {"ambient", "hip_hop", "rock"}),
        ("Singer-Songwriter", {"folk", "pop", "asian_pop", "rock"}),
        # #138: "Electro" alone tags synthwave, techno and EDM alike in this library.
        ("Electro", {"synth", "techno", "house", "trance", "drum_and_bass", "hard_dance"}),
        ("Électronique", ELECTRONIC),
        ("Dance & EDM", ELECTRONIC - {"ambient", "experimental"}),
        ("Musiques du monde", {"world", "latin", "reggae", "folk"}),
    ],
)
def test_broad_tags_fence(tag, genres):
    [t] = CAT.genre_tag(tag)
    assert t.broad
    assert t.fence == genres


def test_electro_fences_but_its_discogs_style_names_synth():
    # A synthwave release tagged "Electro, Chillwave, Synthwave" is Synth.
    parts = CAT.genre_tag("Electro, Chillsynth, Chillwave, Synthwave")
    assert parts[0].broad
    assert [(t.genre, t.style) for t in parts[1:]] == [("synth", "synthwave"), ("synth", "chillwave")]
    assert CAT.discogs_style("Electro", ["Electronic"]) == genre("synth", "electro")
    assert CAT.genre_tag("Electro House") == [genre("house", "electro_house")]


def test_old_indie_moves_to_rock():
    assert CAT.id_map()["indie"] == "rock"
    assert "indie" not in CAT.genres


def test_old_bass_and_dance_move_to_drum_and_bass_and_house():
    assert CAT.id_map()["bass"] == "drum_and_bass"
    assert CAT.id_map()["dance"] == "house"
    assert "bass" not in CAT.genres and "dance" not in CAT.genres
    assert not CAT.genres["asian_pop"].replaces


def test_drum_n_bass_is_a_style_so_the_model_style_pick_can_choose_it():
    # Without it, D&B songs got the strongest other style in the merged genre
    # (Dubstep or Halftime) from the Discogs400 model (#138).
    assert CAT.styles["drum_n_bass"].genre == "drum_and_bass"
    assert CAT.genre_tag("DnB") == [genre("drum_and_bass", "drum_n_bass")]


def test_broad_tag_fences_name_real_genres():
    for t in CAT._broad.values():
        assert t.fence and t.fence <= set(CAT.genres)


@pytest.mark.parametrize("tag", ["", "Other", "Misc", "90S", "Female Vocalists", "Cher", "seen live", "Instrumental"])
def test_genre_tag_ignores_non_genres(tag):
    assert CAT.genre_tag(tag) == []


def test_genre_tag_splits_and_dedupes():
    assert CAT.genre_tag("House, Deep House / House") == [genre("house"), genre("house", "deep_house")]
    assert CAT.genre_tag("Electronic; Techno") == [fence(*ELECTRONIC), genre("techno")]
    assert CAT.genre_tag("Pop, 90S; Female Vocalists") == [genre("pop")]


# --- MusicBrainz and Last.fm --------------------------------------------------------


@pytest.mark.parametrize(
    "name, expected",
    [
        ("deep house", genre("house", "deep_house")),
        ("drum and bass", genre("drum_and_bass", "drum_n_bass")),
        ("psytrance", genre("trance", "psy_trance")),
        ("hip hop", genre("hip_hop")),
        ("synth-pop", genre("synth", "synth_pop")),
        ("black metal", genre("metal", "black_metal")),
        ("alternative rock", genre("rock", "alternative_rock")),
        ("electronic", fence(*ELECTRONIC)),
        ("pop", genre("pop")),
        ("contemporary r&b", genre("soul", "contemporary_r_and_b")),
        ("bossa nova", genre("latin", "bossa_nova")),
        ("spoken word", genre("other", "spoken_word")),
    ],
)
def test_musicbrainz_genre(name, expected):
    assert CAT.musicbrainz_genre(name) == expected


@pytest.mark.parametrize(
    "tag, expected",
    [
        ("Psytrance", genre("trance", "psy_trance")),
        ("chillout", genre("ambient")),
        ("dnb", genre("drum_and_bass", "drum_n_bass")),
        ("electronic", fence(*ELECTRONIC)),
        ("seen live", None),
        ("british", None),
        ("female vocalists", None),
        ("80s", None),
    ],
)
def test_lastfm_tag(tag, expected):
    assert CAT.lastfm_tag(tag) == expected


def test_lastfm_ignores_only_apply_to_lastfm():
    assert CAT.lastfm_tag("british") is None
    assert CAT.musicbrainz_genre("british") is None  # no rule matches either
    assert CAT.lastfm_tag("live") is None


# --- loading errors ------------------------------------------------------------------

BASE = """
version = "test"
[[genre]]
id = "techno"
name = "Techno"
replaces = ["techno"]
styles = ["Acid"]
[[genre]]
id = "house"
name = "House"
replaces = ["house"]
styles = ["Deep House"]
"""


def make(extra="", base=BASE, v1=("techno", "house")):
    return gc.Catalog(tomllib.loads(base + extra), v1)


def test_small_catalog_loads():
    cat = make('[text.broad]\n"club" = ["techno", "house"]\n')
    assert list(cat.genres) == ["techno", "house"]
    assert cat.genre_tag("club") == [fence("techno", "house")]
    assert cat.genre_tag("Acid") == [genre("techno", "acid")]


@pytest.mark.parametrize(
    "extra, v1, message",
    [
        ('[[genre]]\nid = "x"\nname = "X"\nstyles = ["Acid"]\n', ("techno", "house"), "listed twice"),
        ('[[genre]]\nid = "x"\nname = "X"\nstyles = ["Techno"]\n', ("techno", "house"), "id of a genre"),
        ('[[genre]]\nid = "Bad Id"\nname = "X"\n', ("techno", "house"), "bad genre id"),
        ('[text.broad]\n"club" = ["techno", "trance"]\n', ("techno", "house"), "unknown"),
        ('[text.aliases]\n"tekk" = "Tekk"\n', ("techno", "house"), "neither a style name nor a genre id"),
        ('[[text.rule]]\npattern = "x"\ngenre = "jazz"\n', ("techno", "house"), "unknown genre"),
        ('[discogs_genres]\n"Electronic" = []\n', ("techno", "house"), "no genres"),
        ("", ("techno", "house", "trance"), "no genre replaces"),
        ('[[genre]]\nid = "x"\nname = "X"\nreplaces = ["house"]\n', ("techno", "house"), "replaced by both"),
        ('[[genre]]\nid = "x"\nname = "Deep House"\n', ("techno", "house"), "means both"),
    ],
)
def test_catalog_errors(extra, v1, message):
    with pytest.raises(gc.CatalogError, match=message):
        make(extra, v1=v1)


# --- the switch migration -------------------------------------------------------------


def test_switch_migration_maps_old_ids_and_lists_styles_under_their_genre():
    cat = make(
        '[[genre]]\nid = "club"\nname = "Club"\nreplaces = ["dance", "synth"]\nstyles = ["Nu-Disco"]\n',
        v1=("techno", "house", "dance", "synth"),
    )
    up, down = gc.switch_migration(cat)
    assert "('dance', 'club')" in up and "('synth', 'club')" in up
    assert "('nu_disco', 'Nu-Disco', 0, TRUE, 'style', 'club')" in up
    assert "('club', 'Club', 2, TRUE, 'genre', NULL)" in up
    # Down: a genre goes back to the first old id it replaced, a style to its genre's.
    assert "('club', 'dance')" in down and "('nu_disco', 'dance')" in down


def test_style_names_are_quoted():
    cat = make('[[genre]]\nid = "x"\nname = "Children\'s"\nreplaces = []\n')
    up, _ = gc.switch_migration(cat)
    assert "'Children''s'" in up
