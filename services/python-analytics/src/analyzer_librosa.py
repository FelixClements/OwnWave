import os
import subprocess
import tempfile

import librosa
import numpy as np
import soundfile as sf
from pyloudnorm import Meter

from models import AudioFeatures

KEY_NAMES = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"]
MAJOR_PROFILE = np.array(
    [6.35, 2.23, 3.48, 2.33, 4.38, 4.09, 2.52, 5.19, 2.39, 3.66, 2.29, 2.88]
)
MINOR_PROFILE = np.array(
    [6.33, 2.68, 3.52, 5.38, 2.60, 3.53, 2.54, 4.75, 3.98, 2.69, 3.34, 3.17]
)


class LibrosaAnalyzer:
    """Default analyzer using librosa + mutagen."""

    def analyze(self, path: str) -> AudioFeatures:
        # Load with soundfile, then resample/mono with librosa to avoid the
        # deprecated audioread fallback.
        y, sr = self._load(path)
        if y.ndim > 1:
            # soundfile returns (samples, channels); downmix to mono.
            y = np.mean(y, axis=1)
        if sr != 22050:
            y = librosa.resample(y=y, orig_sr=sr, target_sr=22050, res_type="kaiser_fast")
            sr = 22050
        duration = librosa.get_duration(y=y, sr=sr)

        tempo, beat_frames = librosa.beat.beat_track(y=y, sr=sr)
        if isinstance(tempo, np.ndarray):
            tempo = float(tempo[0])
        else:
            tempo = float(tempo)

        beat_times = librosa.frames_to_time(beat_frames, sr=sr)

        chroma = librosa.feature.chroma_stft(y=y, sr=sr)
        chroma_mean = np.mean(chroma, axis=1)
        chroma_sum = np.sum(chroma_mean)
        if chroma_sum > 0:
            chroma_mean = chroma_mean / chroma_sum

        key = _detect_key(chroma_mean)

        # Loudness
        meter = Meter(sr)
        # pyloudnorm wants (samples, channels)
        if y.ndim == 1:
            loudness = meter.integrated_loudness(y.reshape(-1, 1))
        else:
            loudness = meter.integrated_loudness(y.T)

        # Energy and valence
        rms = librosa.feature.rms(y=y)
        energy = float(np.mean(rms))

        spec_cent = librosa.feature.spectral_centroid(y=y, sr=sr)
        sc_mean = float(np.mean(spec_cent))
        sc_norm = min(sc_mean / (sr / 2), 1.0)
        valence = float(np.clip(0.6 * energy + 0.4 * sc_norm, 0.0, 1.0))

        mfcc = librosa.feature.mfcc(y=y, sr=sr, n_mfcc=13)
        mfcc_mean = np.mean(mfcc, axis=1)

        ideal_crossfade = _ideal_crossfade(tempo)
        outro_start = _detect_outro_start(y, sr, beat_frames, beat_times, duration, ideal_crossfade)

        _, (start, end) = librosa.effects.trim(
            y, top_db=60, frame_length=2048, hop_length=512
        )
        intro_start = float(librosa.frames_to_time(start, sr=sr, hop_length=512))
        outro_end = float(librosa.frames_to_time(end, sr=sr, hop_length=512))

        return AudioFeatures(
            bpm=round(tempo, 2),
            key=key,
            loudness=round(float(loudness), 2),
            energy=round(energy, 4),
            valence=round(valence, 4),
            outro_start_seconds=round(outro_start, 3),
            ideal_crossfade_seconds=round(ideal_crossfade, 3),
            intro_start_seconds=round(intro_start, 3),
            outro_end_seconds=round(outro_end, 3),
            chroma=chroma_mean.tolist(),
            spectral_centroid=round(sc_mean, 2),
            mfcc=mfcc_mean.tolist(),
        )


    @staticmethod
    def _load(path: str):
        """Load audio with soundfile, falling back to an ffmpeg WAV conversion."""
        try:
            return sf.read(path, dtype="float32")
        except Exception:
            # Some FLACs have non-seekable frames; convert through ffmpeg.
            with tempfile.NamedTemporaryFile(suffix=".wav", delete=False) as tmp:
                tmp_path = tmp.name
            try:
                ffmpeg = os.environ.get("FFMPEG_PATH", "ffmpeg")
                subprocess.run(
                    [ffmpeg, "-y", "-i", path, tmp_path],
                    check=True,
                    stdout=subprocess.DEVNULL,
                    stderr=subprocess.DEVNULL,
                )
                return sf.read(tmp_path, dtype="float32")
            finally:
                try:
                    os.remove(tmp_path)
                except OSError:
                    pass


def _detect_key(chroma_mean: np.ndarray) -> str:
    best_corr = -1.0
    best_key = "C major"
    for i in range(12):
        major_rot = np.roll(MAJOR_PROFILE, i)
        minor_rot = np.roll(MINOR_PROFILE, i)
        major_corr = np.corrcoef(major_rot, chroma_mean)[0, 1]
        minor_corr = np.corrcoef(minor_rot, chroma_mean)[0, 1]
        if major_corr > best_corr:
            best_corr = major_corr
            best_key = f"{KEY_NAMES[i]} major"
        if minor_corr > best_corr:
            best_corr = minor_corr
            best_key = f"{KEY_NAMES[i]} minor"
    return best_key


def _ideal_crossfade(bpm: float) -> float:
    if bpm <= 0:
        return 4.0
    return float(np.clip(4.0 * (60.0 / bpm), 2.0, 10.0))


def _detect_outro_start(
    y: np.ndarray,
    sr: int,
    beat_frames: np.ndarray,
    beat_times: np.ndarray,
    duration: float,
    ideal_crossfade: float,
) -> float:
    if duration < 60.0 or len(beat_times) == 0:
        return float(np.clip(duration * 0.75, 0.0, duration))

    # We want a strong beat no earlier than 40s from the end and with
    # enough time left for the crossfade plus a small safety buffer.
    min_start = duration - 40.0
    max_start = duration - ideal_crossfade - 2.0
    if max_start <= min_start:
        return float(np.clip(duration - ideal_crossfade - 2.0, 0.0, duration))

    candidates = beat_times[(beat_times >= min_start) & (beat_times <= max_start)]
    if len(candidates) == 0:
        return float(np.clip(duration - ideal_crossfade - 2.0, 0.0, duration))

    onset_env = librosa.onset.onset_strength(y=y, sr=sr)
    candidate_frames = librosa.time_to_frames(candidates, sr=sr)
    candidate_frames = np.clip(candidate_frames, 0, len(onset_env) - 1)
    onset_values = onset_env[candidate_frames]
    best_idx = int(np.argmax(onset_values))
    return float(candidates[best_idx])
