"""The genre engine v2 decision rules (ADR 0008), without a database."""

import time

import numpy as np
import pytest

import genre_catalog as gc
import genre_engine as ge
from genre_engine import SongInput

DIMS = 16


@pytest.fixture(scope="module")
def cat():
    return gc.catalog()


def axis(i: int, wobble: float = 0.0, seed: int = 0) -> np.ndarray:
    """A unit-ish vector along axis i, slightly moved so songs differ."""
    v = np.zeros(DIMS, dtype=np.float32)
    v[i] = 1.0
    if wobble:
        v += np.random.default_rng(seed).normal(0, wobble, DIMS).astype(np.float32)
    return v


def labelled(tag: str, at: int, n: int, prefix: str, wobble: float = 0.01) -> list:
    """n songs tagged `tag` that sound alike (along axis `at`)."""
    return [SongInput(f"{prefix}{i}", genre_tag=tag, embedding=axis(at, wobble, seed=at * 1000 + i))
            for i in range(n)]


def by_id(result) -> dict:
    return {d.track_id: d for d in result.decisions}


def one(cat, song: SongInput, *others: SongInput, labels=None) -> ge.Decision:
    return by_id(ge.decide([song, *others], cat, labels))[song.track_id]


# --- corrections ---------------------------------------------------------------


def test_correction_wins_over_every_source(cat):
    d = one(cat, SongInput("s", genre_tag="Techno", discogs=[{"name": "Deep House", "kind": "style"}],
                           correction="jazz", correction_scope=ge.RELEASE_SCOPE))
    assert (d.genre, d.source, d.confidence) == ("jazz", ge.CORRECTION, 1.0)
    assert d.why["correction"] == {"scope": "release", "value": "jazz"}


def test_correction_may_name_a_style(cat):
    d = one(cat, SongInput("s", genre_tag="Techno", correction="deep_house", correction_scope=ge.SONG_SCOPE))
    assert (d.genre, d.style) == ("house", "deep_house")


def test_resolve_correction(cat):
    assert ge.resolve_correction("house", cat) == ("house", None)
    assert ge.resolve_correction("hard_techno", cat) == ("techno", "hard_techno")
    assert ge.resolve_correction("gone", cat) is None
    assert ge.resolve_correction(None, cat) is None
    # v1 ids are moved by the switch migration, not read.
    assert ge.resolve_correction("dance", cat) is None


# --- strong sources --------------------------------------------------------------


def test_agreeing_strong_sources_decide(cat):
    d = one(cat, SongInput(
        "s", genre_tag="Techno",
        mb_recording=[{"name": "techno", "count": 2}],
        discogs=[{"name": "Electronic", "kind": "genre"}, {"name": "Hard Techno", "kind": "style"}],
    ))
    assert (d.genre, d.source, d.confidence) == ("techno", ge.STRONG_SOURCES, 1.0)
    assert d.why["agreeing"] == ["discogs", "genre_tag", "mb_recording"]
    assert (d.style, d.why["style"]["from"]) == ("hard_techno", "discogs")


def test_disagreeing_strong_sources_leave_the_song_unsure(cat):
    d = one(cat, SongInput("s", genre_tag="Techno", discogs=[{"name": "Deep House", "kind": "style"}]))
    assert d.unsure and d.source == ge.UNSURE
    assert d.unsure_reason == ge.STRONG_DISAGREE
    assert d.why["sources"]["genre_tag"] == {"genre": "techno"}
    assert d.why["sources"]["discogs"]["genre"] == "house"


def test_a_strong_source_naming_several_genres_only_fences(cat):
    styles = [{"name": "Deep House", "kind": "style"}, {"name": "Hard Techno", "kind": "style"}]
    d = one(cat, SongInput("s", genre_tag="Techno", discogs=styles))
    assert (d.genre, d.source) == ("techno", ge.STRONG_SOURCES)
    assert d.fence == ["house", "techno"]
    # The style comes from a strong source inside the decided genre.
    assert d.style == "hard_techno"


def test_electronic_tag_and_discogs_deep_house_is_house(cat):
    d = one(cat, SongInput("s", genre_tag="Electronic", discogs=[{"name": "Deep House", "kind": "style"}]))
    assert (d.genre, d.style, d.source) == ("house", "deep_house", ge.STRONG_SOURCES)
    assert "house" in d.fence and "pop" not in d.fence


def test_musicbrainz_release_genre_is_strong(cat):
    d = one(cat, SongInput("s", mb_release=[{"name": "heavy metal", "count": 1}]))
    assert (d.genre, d.style, d.source) == ("metal", "heavy_metal", ge.STRONG_SOURCES)


# --- weak sources and the fence ---------------------------------------------------


def test_weak_sources_never_pick_the_genre(cat):
    d = one(cat, SongInput("s", lastfm=[{"name": "house", "count": 100}],
                           artist_genres=[{"name": "house", "count": 5}]))
    assert d.unsure and d.unsure_reason == ge.NO_AUDIO
    assert d.fence == ["house"]


def test_low_lastfm_weights_do_not_fence(cat):
    d = one(cat, SongInput("s", lastfm=[{"name": "house", "count": 100}, {"name": "metal", "count": 2}]))
    assert d.fence == ["house"]


def test_an_empty_fence_makes_the_song_unsure(cat):
    d = one(cat, SongInput("s", artist_genres=[{"name": "heavy metal", "count": 3}],
                           lastfm=[{"name": "deep house", "count": 100}]))
    assert d.unsure and d.unsure_reason == ge.EMPTY_FENCE
    assert d.fence == []


def test_a_strong_source_outside_the_fence_makes_the_song_unsure(cat):
    d = one(cat, SongInput("s", genre_tag="Techno", artist_genres=[{"name": "heavy metal", "count": 3}]))
    assert d.unsure and d.unsure_reason == ge.OUTSIDE_FENCE


def test_broad_tag_fences(cat):
    d = one(cat, SongInput("s", genre_tag="Pop/Rock"))
    assert d.unsure and d.fence == ["asian_pop", "metal", "pop", "punk", "rock"]


# --- release lean -----------------------------------------------------------------


def release(n_house: int, n_techno: int, compilation: bool = False) -> list:
    songs = [SongInput(f"h{i}", release_id="r", compilation=compilation, genre_tag="House") for i in range(n_house)]
    songs += [SongInput(f"t{i}", release_id="r", compilation=compilation, genre_tag="Techno") for i in range(n_techno)]
    return songs


def test_release_lean_at_70_percent(cat):
    d = one(cat, SongInput("s", release_id="r"), *release(4, 1))
    assert (d.genre, d.source, d.confidence) == ("house", ge.RELEASE, 0.8)
    assert d.why["release"] == {"genre": "house", "share": 0.8, "decided": 5}


def test_release_lean_needs_70_percent(cat):
    d = one(cat, SongInput("s", release_id="r"), *release(3, 2))
    assert d.unsure and d.unsure_reason == ge.NO_AUDIO


def test_compilations_skip_the_release_lean(cat):
    d = one(cat, SongInput("s", release_id="r", compilation=True), *release(5, 0, compilation=True))
    assert d.unsure and d.unsure_reason == ge.NO_AUDIO


def test_release_lean_stays_inside_the_fence(cat):
    d = one(cat, SongInput("s", release_id="r", lastfm=[{"name": "techno", "count": 100}]), *release(5, 0))
    assert d.unsure and d.unsure_reason == ge.NO_AUDIO


def test_a_sure_classifier_overrules_the_release_lean(cat):
    trance = labelled("Trance", 2, 20, "tr")
    house = [SongInput(f"h{i}", release_id="r", genre_tag="House", embedding=axis(1)) for i in range(4)]
    d = one(cat, SongInput("s", release_id="r", embedding=axis(2)), *house, *trance)
    assert d.unsure and d.unsure_reason == ge.RELEASE_VS_CLASSIFIER
    assert d.why["classifier"]["genre"] == "trance"


# --- classifier -----------------------------------------------------------------


def test_untagged_metal_song_fenced_by_its_artist_is_metal(cat):
    # It sounds like the techno songs, but its artist is a metal artist.
    techno = labelled("Techno", 0, 15, "te")
    metal = labelled("Heavy Metal", 1, 10, "me")
    song = SongInput("s", embedding=axis(0) + 0.3 * axis(1), artist_genres=[{"name": "heavy metal", "count": 7}])
    d = one(cat, song, *techno, *metal)
    assert (d.genre, d.source) == ("metal", ge.CLASSIFIER)
    assert d.confidence == 1.0
    # Without the artist's fence the classifier would call it techno.
    free = one(cat, SongInput("s", embedding=song.embedding), *techno, *metal)
    assert free.genre == "techno"


def test_techno_tag_that_sounds_like_trance_keeps_the_tag_and_is_flagged(cat):
    trance = [SongInput(f"tr{i}", genre_tag="Trance", embedding=axis(2)) for i in range(17)]
    techno_near = [SongInput(f"tn{i}", genre_tag="Techno", embedding=axis(2)) for i in range(3)]
    techno_far = labelled("Techno", 0, 10, "tf")
    d = one(cat, SongInput("s", genre_tag="Techno", embedding=axis(2)), *trance, *techno_near, *techno_far)
    assert (d.genre, d.source) == ("techno", ge.STRONG_SOURCES)
    assert d.review_reason == ge.CLASSIFIER_DISAGREES
    assert d.why["classifier"]["genre"] == "trance"
    assert d.why["classifier"]["share"] == pytest.approx(0.85)


def test_a_classifier_below_80_percent_against_the_tag_does_not_flag(cat):
    trance = [SongInput(f"tr{i}", genre_tag="Trance", embedding=axis(2)) for i in range(15)]
    techno = [SongInput(f"tn{i}", genre_tag="Techno", embedding=axis(2)) for i in range(5)]
    d = one(cat, SongInput("s", genre_tag="Techno", embedding=axis(2)), *trance, *techno)
    assert d.genre == "techno" and d.review_reason is None


def test_the_classifier_decides_at_60_percent(cat):
    house = [SongInput(f"h{i}", genre_tag="House", embedding=axis(3)) for i in range(13)]
    techno = [SongInput(f"t{i}", genre_tag="Techno", embedding=axis(3)) for i in range(7)]
    d = one(cat, SongInput("s", embedding=axis(3)), *house, *techno)
    assert (d.genre, d.source, d.confidence) == ("house", ge.CLASSIFIER, 0.65)
    assert d.why["classifier"]["runner_ups"] == [["techno", 0.35]]


def test_the_classifier_is_unsure_below_60_percent(cat):
    house = [SongInput(f"h{i}", genre_tag="House", embedding=axis(3)) for i in range(11)]
    techno = [SongInput(f"t{i}", genre_tag="Techno", embedding=axis(3)) for i in range(9)]
    d = one(cat, SongInput("s", embedding=axis(3)), *house, *techno)
    assert d.unsure and d.unsure_reason == ge.CLASSIFIER_UNSURE


def test_only_the_k_nearest_labels_vote(cat):
    near = [SongInput(f"h{i}", genre_tag="House", embedding=axis(3)) for i in range(20)]
    far = [SongInput(f"t{i}", genre_tag="Techno", embedding=axis(3) + 0.5 * axis(4)) for i in range(30)]
    d = one(cat, SongInput("s", embedding=axis(3)), *near, *far)
    assert (d.genre, d.confidence) == ("house", 1.0)


def test_broad_tags_lastfm_and_untagged_songs_never_teach(cat):
    teachers = [SongInput(f"b{i}", genre_tag="Electronic", embedding=axis(5)) for i in range(5)]
    teachers += [SongInput(f"l{i}", lastfm=[{"name": "techno", "count": 100}], embedding=axis(5)) for i in range(5)]
    teachers += [SongInput(f"u{i}", embedding=axis(5)) for i in range(5)]
    result = ge.decide([SongInput("s", embedding=axis(5)), *teachers], cat)
    assert result.test_score["labels"] == 0
    assert by_id(result)["s"].unsure_reason == ge.CLASSIFIER_UNSURE


def test_corrections_teach_the_classifier(cat):
    fixed = [SongInput(f"c{i}", genre_tag="Techno", correction="jazz", correction_scope="artist", embedding=axis(6))
             for i in range(5)]
    d = one(cat, SongInput("s", embedding=axis(6)), *fixed)
    assert (d.genre, d.source) == ("jazz", ge.CLASSIFIER)


def test_a_copy_learns_from_its_songs_label_but_a_label_never_votes_for_itself(cat):
    best = SongInput("best", song_id="song", genre_tag="House", embedding=axis(7))
    copy = SongInput("copy", song_id="song", best_copy=False, embedding=axis(7))
    result = by_id(ge.decide([best, copy], cat))
    assert (result["copy"].genre, result["copy"].source) == ("house", ge.CLASSIFIER)
    # The only label is the song itself: held out, nothing votes.
    assert result["best"].why["classifier"]["genre"] is None


# --- style ------------------------------------------------------------------------

MODEL_LABELS = ["Electronic---Deep House", "Electronic---Techno", "Electronic---Minimal"]


def test_style_from_the_discogs400_model_when_confident(cat):
    d = one(cat, SongInput("s", genre_tag="House", activations=[0.5, 0.9, 0.1]), labels=MODEL_LABELS)
    assert (d.genre, d.style) == ("house", "deep_house")
    assert d.why["style"] == {"style": "deep_house", "from": "discogs400", "activation": 0.5}


def test_no_style_when_the_model_is_not_confident(cat):
    d = one(cat, SongInput("s", genre_tag="Techno", activations=[0.9, 0.9, 0.1]), labels=MODEL_LABELS)
    assert (d.genre, d.style) == ("techno", None)


def test_a_strong_style_beats_the_model(cat):
    d = one(cat, SongInput("s", genre_tag="Techno; Minimal", discogs=[{"name": "Hard Techno", "kind": "style"}],
                           activations=[0.0, 0.0, 0.9]), labels=MODEL_LABELS)
    assert (d.genre, d.style, d.why["style"]["from"]) == ("techno", "minimal", "genre_tag")


# --- test score and summary ---------------------------------------------------------


def test_test_score_is_leave_one_out_accuracy(cat):
    songs = labelled("House", 8, 6, "h") + labelled("Techno", 9, 6, "t")
    # One techno-tagged song sounds like house: held out, the classifier misses it.
    songs.append(SongInput("odd", genre_tag="Techno", embedding=axis(8)))
    score = ge.decide(songs, cat).test_score
    assert score["labels"] == 13
    assert score["accuracy"] == round(12 / 13, 3)
    assert score["per_genre"]["house"]["accuracy"] == 1.0
    assert score["per_genre"]["techno"]["accuracy"] == round(6 / 7, 3)
    assert score["decides"] == 1.0


def test_summary_counts(cat):
    result = ge.decide([
        SongInput("a", genre_tag="Techno"),
        SongInput("b", genre_tag="Techno", discogs=[{"name": "Deep House", "kind": "style"}]),
        SongInput("c", correction="jazz", correction_scope="song"),
    ], cat)
    s = ge.summarize(result)
    assert (s["tracks"], s["decided"], s["unsure"]) == (3, 2, 1)
    assert s["by_source"] == {ge.STRONG_SOURCES: 1, ge.UNSURE: 1, ge.CORRECTION: 1}
    assert s["unsure_reasons"] == {ge.STRONG_DISAGREE: 1}
    assert s["genres"] == {"techno": 1, "jazz": 1}


def test_full_library_rescore_stays_in_seconds(cat):
    """About the live library: 7,800 songs, 1,280-d embeddings, 3,000 trusted labels."""
    rng = np.random.default_rng(1)
    centres = rng.normal(size=(12, 1280)).astype(np.float32)
    tags = ["Techno", "House", "Trance", "Heavy Metal", "Pop", "Jazz", "Ambient", "Punk", "Soul", "Folk", "Rock",
            "Hip Hop"]
    songs = []
    for i in range(7800):
        c = i % 12
        emb = centres[c] + rng.normal(scale=0.8, size=1280).astype(np.float32)
        tag = tags[c] if i < 3000 else ("Electronic" if i % 3 == 0 else None)
        songs.append(SongInput(i, release_id=i // 10, genre_tag=tag, embedding=emb,
                               artist_genres=[{"name": "rock", "count": 1}] if c == 10 and i % 2 else ()))
    started = time.monotonic()
    result = ge.decide(songs, cat)
    elapsed = time.monotonic() - started
    assert len(result.decisions) == 7800
    assert result.test_score["labels"] == 3000
    assert elapsed < 30, elapsed  # a few seconds on a laptop; generous for CI
