import json
from collections import Counter
from pathlib import Path

import pytest

import genre_taxonomy as tx

LABELS = json.loads((Path(__file__).parent / "data" / "discogs400_labels.json").read_text())


def test_fixture_is_the_full_label_list():
    assert len(LABELS) == 400


def test_every_override_names_a_real_label():
    # A typo in an override would silently fall back to the parent default.
    known = set(LABELS)
    missing = [label for label in tx._DISCOGS_MAP if label not in known]
    assert missing == []


def test_every_label_maps_to_a_station_genre():
    assert {tx.discogs_label_genre(label) for label in LABELS} <= set(tx.STATION_GENRES)


def test_no_station_genre_collects_a_large_share_of_labels():
    # The old pipeline grouped by parent: Electronic held 113 of 400 labels.
    counts = Counter(tx.discogs_label_genre(label) for label in LABELS)
    assert max(counts.values()) <= 40, counts.most_common(3)


@pytest.mark.parametrize(
    "label, genre",
    [
        ("Electronic---Techno", "techno"),
        ("Electronic---Hard Techno", "techno"),
        ("Electronic---Tech House", "house"),
        ("Electronic---Psy-Trance", "trance"),
        ("Electronic---Drum n Bass", "drum_and_bass"),
        ("Electronic---Vaporwave", "synth"),
        ("Electronic---Ambient", "ambient"),
        ("Electronic---Experimental", "experimental"),
        ("Electronic---Dance-pop", "pop"),
        ("Hip Hop---Trap", "hip_hop"),
        ("Rock---Black Metal", "metal"),
        ("Rock---Punk", "punk"),
        ("Rock---Indie Rock", "indie"),
        ("Rock---Classic Rock", "rock"),
        ("Funk / Soul---Disco", "dance"),
        ("Stage & Screen---Score", "soundtrack"),
        ("Non-Music---Comedy", "other"),
    ],
)
def test_discogs_label_genre(label, genre):
    assert tx.discogs_label_genre(label) == genre


def test_discogs_distribution_takes_the_strongest_label_per_genre():
    labels = ["Electronic---Techno", "Electronic---Hard Techno", "Electronic---Ambient", "Electronic---Drone"]
    # Two ambient labels at 0.3 must not outscore one techno label at 0.5.
    dist = tx.discogs_distribution(labels, [0.5, 0.1, 0.3, 0.3])
    assert dist == pytest.approx({"techno": 0.5 / 0.8, "ambient": 0.3 / 0.8})


@pytest.mark.parametrize(
    "text, genre",
    [
        ("Hip-Hop", "hip_hop"),
        ("Rap", "hip_hop"),
        ("Drum & Bass", "drum_and_bass"),
        ("drum and bass", "drum_and_bass"),
        ("Liquid Funk", "drum_and_bass"),
        ("Hard Techno", "techno"),
        ("Minimal", "techno"),
        ("Tech House", "house"),
        ("Big Room", "house"),
        ("Hardstyle", "hard_dance"),
        ("UK Garage", "bass"),
        ("Garage Rock", "rock"),
        ("Alternative Rock", "indie"),
        ("Indie Pop", "indie"),
        ("Hardcore Punk", "punk"),
        ("Metalcore", "metal"),
        ("Synthpop", "synth"),
        ("Trip Hop", "ambient"),
        ("Dance-Pop", "pop"),
        ("K-Pop", "pop"),
        ("R&B", "soul"),
        ("Reggaeton", "latin"),
        ("Dancehall", "reggae"),
        ("Singer-Songwriter", "folk"),
        ("Soundtrack", "soundtrack"),
    ],
)
def test_text_genre(text, genre):
    assert tx.text_genre(text) == {genre: 1.0}


@pytest.mark.parametrize("text", ["90S", "Female Vocalists", "Cher", "", "seen live"])
def test_text_genre_ignores_non_genres(text):
    assert tx.text_genre(text) is None


def test_generic_electronic_spreads_over_electronic_genres():
    dist = tx.text_genre("Electronic")
    assert set(dist) == set(tx.ELECTRONIC_GENRES)
    assert sum(dist.values()) == pytest.approx(1.0)


def test_split_tag_and_text_distribution():
    assert tx.split_tag("Pop, Dance, 90S; Female Vocalists") == ["Pop", "Dance", "90S", "Female Vocalists"]
    dist = tx.text_distribution(["Techno", "Hard Techno", "House", "seen live"])
    assert dist == pytest.approx({"techno": 2 / 3, "house": 1 / 3})


def test_text_distribution_uses_weights():
    dist = tx.text_distribution(["rock", "pop"], [3, 1])
    assert dist == pytest.approx({"rock": 0.75, "pop": 0.25})
