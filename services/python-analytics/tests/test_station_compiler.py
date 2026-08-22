from uuid import uuid4

import pytest

from station.compiler import distance, key_penalty, smart_queue
from station.filter import apply_numeric_filter
from station.seed import parse_seed, seed_from_request


class _FakeRequest:
    def __init__(self, **kwargs):
        for key, value in kwargs.items():
            setattr(self, key, value)


def test_parse_seed_normalizes_seed_type():
    assert parse_seed({"seed_type": "genre", "main_genre": "rock"}) == {
        "type": "genre",
        "main_genre": "rock",
    }


def test_parse_seed_keeps_type_key():
    assert parse_seed({"type": "artist", "artist_id": "abc"}) == {
        "type": "artist",
        "artist_id": "abc",
    }


def test_seed_from_request_uses_type_key():
    req = _FakeRequest(seed_type="genre", main_genre="jazz")
    assert seed_from_request(req) == {"type": "genre", "main_genre": "jazz"}


def test_seed_from_request_keeps_numeric_filters_with_genre():
    req = _FakeRequest(seed_type="genre", main_genre="rock", min_bpm=100, max_bpm=140)
    assert seed_from_request(req) == {
        "type": "genre",
        "main_genre": "rock",
        "min_bpm": 100,
        "max_bpm": 140,
    }


def test_distance_prefers_similar_tracks():
    close = distance({"bpm": 120, "energy": 0.5, "valence": 0.5}, {"bpm": 122, "energy": 0.52, "valence": 0.48})
    far = distance({"bpm": 120, "energy": 0.5, "valence": 0.5}, {"bpm": 90, "energy": 0.1, "valence": 0.9})
    assert close < far


def test_key_penalty_same_key_is_zero():
    assert key_penalty("C", "C") == 0.0
    assert key_penalty("C", "G") == 0.6


def test_smart_queue_starts_from_seed_track():
    seed_id = uuid4()
    other_id = uuid4()
    tracks = [
        {"id": seed_id, "bpm": 120, "energy": 0.5, "valence": 0.5, "feature_vector": None},
        {"id": other_id, "bpm": 121, "energy": 0.51, "valence": 0.49, "feature_vector": None},
    ]

    queue = smart_queue(
        conn=None,  # type: ignore[arg-type]
        tracks=tracks,
        length=2,
        novelty=0.0,
        seed_id=seed_id,
        get_similar=lambda _track_id, _limit: [],
    )

    assert queue[0]["id"] == seed_id
    assert len(queue) == 2


def test_apply_numeric_filter_limits_bpm():
    tracks = [
        {"id": 1, "bpm": 90, "energy": 0.5, "valence": 0.5},
        {"id": 2, "bpm": 120, "energy": 0.5, "valence": 0.5},
    ]
    filtered = apply_numeric_filter(tracks, {"min_bpm": 100, "max_bpm": 130})
    assert [t["id"] for t in filtered] == [2]
