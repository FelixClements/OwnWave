"""Failed analysis is remembered and not retried every scan (#119, ADR 0006)."""

import os
from types import SimpleNamespace

import psycopg
import pytest

import folder_importer
import genre_audio
from library_scan import scan_library
from library_scan.options import ScanOptions
from test_library_scan import write_flac

FEATURES = dict(
    bpm=120.0,
    key="C",
    loudness=-10.0,
    energy=0.5,
    valence=0.5,
    outro_start_seconds=None,
    ideal_crossfade_seconds=None,
    intro_start_seconds=None,
    outro_end_seconds=None,
    chroma=None,
    spectral_centroid=None,
    mfcc=None,
)


class FakeAnalysis:
    """Stands in for the feature analyzers and the audio genre model."""

    def __init__(self):
        self.features_ok = False
        self.genre_ok = False
        self.feature_calls = []
        self.genre_calls = []

    # Analyzer interface (analyzers.analyze_file calls analyze).
    def analyze(self, path):
        self.feature_calls.append(path)
        if not self.features_ok:
            raise ValueError("cannot decode")
        return SimpleNamespace(**FEATURES)

    def genre_model(self):
        fake = self

        class Model:
            def analyze(self, path):
                fake.genre_calls.append(path)
                if not fake.genre_ok:
                    return None
                return genre_audio.AudioGenreResult(
                    activations=[0.5], embedding=[0.0] * 1280, model_version=genre_audio.MODEL_VERSION
                )

        return Model()

    def reset_calls(self):
        self.feature_calls.clear()
        self.genre_calls.clear()


@pytest.fixture
def fake(monkeypatch):
    f = FakeAnalysis()
    model = f.genre_model()
    monkeypatch.setattr(folder_importer, "get_analyzers", lambda: [f])
    monkeypatch.setattr(genre_audio, "get_model", lambda: model)
    monkeypatch.setattr(genre_audio, "MODEL_VERSION", "genre-model-1")
    return f


def scan(root, force=False):
    return scan_library(root, options=ScanOptions(analyze_audio=True, force=force))


def failures(url) -> dict:
    with psycopg.connect(url) as conn:
        rows = conn.execute("SELECT kind, model_version, error FROM track_analysis_failures").fetchall()
    return {kind: (version, error) for kind, version, error in rows}


def analysed(url) -> tuple:
    with psycopg.connect(url) as conn:
        features = conn.execute("SELECT count(*) FROM audio_features").fetchone()[0]
        genres = conn.execute("SELECT count(*) FROM track_audio_genres").fetchone()[0]
    return features, genres


def touch(path):
    st = path.stat()
    os.utime(path, (st.st_atime, st.st_mtime + 10))


def test_failed_analysis_is_recorded_and_not_retried_next_scan(db_url, tmp_path, fake):
    song = write_flac(tmp_path / "A" / "song.flac", "Song")

    result = scan(tmp_path)
    assert fake.feature_calls == [str(song)]
    assert fake.genre_calls == [str(song)]
    assert result.model_failed == 1
    recorded = failures(db_url)
    assert recorded.keys() == {"features", "genre"}
    assert recorded["features"][0] is None
    assert "All analyzers failed" in recorded["features"][1]
    assert recorded["genre"] == ("genre-model-1", "genre analysis failed")

    fake.reset_calls()
    result = scan(tmp_path)
    assert fake.feature_calls == []
    assert fake.genre_calls == []
    assert result.total_files == 0  # the plan queued nothing
    assert failures(db_url).keys() == {"features", "genre"}


def test_changed_file_retries_and_success_clears_the_failures(db_url, tmp_path, fake):
    song = write_flac(tmp_path / "A" / "song.flac", "Song")
    scan(tmp_path)
    assert failures(db_url).keys() == {"features", "genre"}

    # A change that fails again keeps a fresh failure on record.
    touch(song)
    fake.reset_calls()
    scan(tmp_path)
    assert fake.feature_calls == [str(song)]
    assert fake.genre_calls == [str(song)]
    assert failures(db_url).keys() == {"features", "genre"}

    touch(song)
    fake.reset_calls()
    fake.features_ok = fake.genre_ok = True
    scan(tmp_path)
    assert fake.feature_calls == [str(song)]
    assert fake.genre_calls == [str(song)]
    assert failures(db_url) == {}
    assert analysed(db_url) == (1, 1)


def test_new_model_version_retries_genre_only(db_url, tmp_path, fake, monkeypatch):
    song = write_flac(tmp_path / "A" / "song.flac", "Song")
    scan(tmp_path)

    monkeypatch.setattr(genre_audio, "MODEL_VERSION", "genre-model-2")
    fake.reset_calls()
    scan(tmp_path)
    assert fake.genre_calls == [str(song)]
    assert fake.feature_calls == []  # the features failure has no model version
    assert failures(db_url)["genre"][0] == "genre-model-2"

    fake.reset_calls()
    scan(tmp_path)
    assert fake.genre_calls == []

    monkeypatch.setattr(genre_audio, "MODEL_VERSION", "genre-model-3")
    fake.genre_ok = True
    scan(tmp_path)
    assert failures(db_url).keys() == {"features"}
    assert analysed(db_url) == (0, 1)


def test_force_ignores_failures_and_success_clears_them(db_url, tmp_path, fake):
    song = write_flac(tmp_path / "A" / "song.flac", "Song")
    scan(tmp_path)

    fake.reset_calls()
    scan(tmp_path, force=True)
    assert fake.feature_calls == [str(song)]
    assert fake.genre_calls == [str(song)]
    assert failures(db_url).keys() == {"features", "genre"}

    fake.reset_calls()
    fake.features_ok = fake.genre_ok = True
    scan(tmp_path, force=True)
    assert fake.feature_calls == [str(song)]
    assert failures(db_url) == {}
    assert analysed(db_url) == (1, 1)


def test_failure_rows_go_with_their_track(db_url, tmp_path, fake):
    song = write_flac(tmp_path / "A" / "song.flac", "Song")
    write_flac(tmp_path / "B" / "other.flac", "Other", freq=550.0)
    scan(tmp_path)
    assert len(failures_rows(db_url)) == 4

    song.unlink()
    scan(tmp_path)
    assert len(failures_rows(db_url)) == 2


def failures_rows(url) -> list:
    with psycopg.connect(url) as conn:
        return conn.execute("SELECT track_id, kind FROM track_analysis_failures").fetchall()
