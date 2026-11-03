import numpy as np
import pytest

import genre_audio
from genre_audio import SAMPLE_RATE, AudioGenreModel, analysis_windows


def test_windows_sit_at_fixed_fractions():
    assert analysis_windows(300, 20) == [(50.0, 70.0), (140.0, 160.0), (215.0, 235.0)]


def test_short_tracks_are_analysed_whole():
    assert analysis_windows(50, 20) == [(0.0, 50)]
    assert analysis_windows(0, 20) == []


def test_loader_is_built_with_the_model_sample_rate(monkeypatch):
    # Regression: reconfiguring a MonoLoader with only a filename reset its
    # sample rate to 44.1 kHz, so the model heard every track 2.75x slower.
    seen = {}

    class FakeES:
        @staticmethod
        def MonoLoader(**params):
            seen.update(params)
            return lambda: np.zeros(10)

    monkeypatch.setattr(genre_audio, "es", FakeES, raising=False)
    genre_audio.load_mono_16k("/music/a.flac")
    assert seen == {"filename": "/music/a.flac", "sampleRate": 16000, "resampleQuality": 4}


def test_analyze_averages_all_windows(monkeypatch):
    monkeypatch.setattr(genre_audio, "load_mono_16k", lambda path: np.zeros(SAMPLE_RATE * 300, dtype=np.float32))
    windows = []

    def embed(chunk):
        windows.append(len(chunk) / SAMPLE_RATE)
        return np.full((2, 4), len(windows), dtype=np.float32)

    def classify(emb):
        # Activation of label 0 = embedding value, so the mean shows every window counted.
        return np.stack([emb[:, 0], np.zeros(len(emb))], axis=1)

    model = AudioGenreModel(embed, classify, ["Electronic---Techno", "Rock---Punk"], window_seconds=20)
    result = model.analyze("/music/a.flac")
    assert windows == [20.0, 20.0, 20.0]
    assert result.activations == pytest.approx([2.0, 0.0])
    assert len(result.embedding) == 4
    assert result.model_version == genre_audio.MODEL_VERSION


def test_analyze_returns_none_when_loading_fails(monkeypatch):
    def broken(path):
        raise RuntimeError("bad file")

    monkeypatch.setattr(genre_audio, "load_mono_16k", broken)
    assert AudioGenreModel(None, None, [], 20).analyze("/x.flac") is None


def test_loader_falls_back_to_ffmpeg(monkeypatch):
    class FakeES:
        @staticmethod
        def MonoLoader(**params):
            raise RuntimeError("Audio file has more than 2 channels.")

    monkeypatch.setattr(genre_audio, "es", FakeES, raising=False)
    monkeypatch.setattr(genre_audio, "_ffmpeg_mono_16k", lambda path: np.ones(3, dtype=np.float32))
    assert genre_audio.load_mono_16k("/music/surround.flac").tolist() == [1.0, 1.0, 1.0]
