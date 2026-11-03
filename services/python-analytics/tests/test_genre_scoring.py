import pytest

import genre_scoring as gs

LABELS = ["Electronic---Techno", "Electronic---House", "Rock---Punk"]


def test_blend_shares_missing_source_weight():
    only_tags = gs.blend({"tags": {"rock": 1.0}})
    assert only_tags == pytest.approx({"rock": 1.0})

    both = gs.blend({"audio": {"techno": 1.0}, "tags": {"house": 1.0}})
    total = gs.WEIGHTS["audio"] + gs.WEIGHTS["tags"]
    assert both == pytest.approx({"techno": gs.WEIGHTS["audio"] / total, "house": gs.WEIGHTS["tags"] / total})


def test_assign_primary_and_close_secondaries():
    genres = gs.assign({"techno": 0.5, "house": 0.35, "trance": 0.1, "ambient": 0.05})
    assert [(g.genre, g.is_primary) for g in genres] == [("techno", True), ("house", False)]


def test_assign_caps_genres_per_track():
    genres = gs.assign({"a": 0.26, "b": 0.25, "c": 0.25, "d": 0.24})
    assert len(genres) == gs.MAX_GENRES


def test_score_track_tags_override_weak_audio():
    # Audio is unsure between techno and house; the tag decides.
    genres = gs.score_track(LABELS, [0.3, 0.28, 0.0], "House", None)
    assert genres[0].genre == "house" and genres[0].is_primary


def test_score_track_uses_musicbrainz_votes():
    mb = [{"name": "punk", "count": 5}, {"name": "rock", "count": 1}]
    genres = gs.score_track(None, None, None, mb)
    assert genres[0].genre == "punk"


def test_score_track_without_sources_is_empty():
    assert gs.score_track(None, None, "90s", []) == []


def test_artist_mb_tags_falls_back_to_primary_artist():
    by_key = {"charlotte de witte": [{"name": "techno", "count": 3}]}
    assert gs.artist_mb_tags("Charlotte de Witte, Theo Nasa", by_key) == by_key["charlotte de witte"]
    assert gs.artist_mb_tags("Unknown", by_key) is None
