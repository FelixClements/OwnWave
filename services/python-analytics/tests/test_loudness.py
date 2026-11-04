import numpy as np
import pytest
import soundfile as sf

from analyzer_loudness import LoudnessAnalyzer, integrated_loudness
from analyzers import analyze_file


def tone(path, seconds=3.0, amplitude=0.5, rate=44100):
    t = np.linspace(0, seconds, int(rate * seconds), endpoint=False)
    sf.write(path, (amplitude * np.sin(2 * np.pi * 1000 * t)).astype("float32"), rate, format="FLAC")
    return str(path)


def test_louder_files_measure_louder(tmp_path):
    loud = LoudnessAnalyzer().analyze(tone(tmp_path / "loud.flac", amplitude=0.5)).loudness
    quiet = LoudnessAnalyzer().analyze(tone(tmp_path / "quiet.flac", amplitude=0.05)).loudness
    assert -10 < loud < 0
    # 20 dB less amplitude is 20 LU quieter.
    assert loud - quiet == pytest.approx(20, abs=0.5)


def test_silence_and_short_clips_have_no_loudness():
    assert integrated_loudness(np.zeros((44100, 1), dtype="float32"), 44100) is None
    assert integrated_loudness(np.ones((1000, 1), dtype="float32"), 44100) is None


def test_only_loudness_is_measured(tmp_path):
    features = analyze_file(tone(tmp_path / "a.flac"))
    assert features.loudness is not None
    assert features.bpm is None and features.key is None


def test_unreadable_file_reports_the_analyzer(tmp_path):
    bad = tmp_path / "bad.flac"
    bad.write_bytes(b"not audio")
    with pytest.raises(RuntimeError, match="LoudnessAnalyzer"):
        analyze_file(str(bad))
