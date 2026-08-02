from models import AudioFeatures


def is_available() -> bool:
    try:
        import essentia  # noqa: F401
        return True
    except Exception:
        return False


class EssentiaAnalyzer:
    """Optional analyzer that uses essentia when installed."""

    def __init__(self):
        if not is_available():
            raise ImportError("essentia is not installed or not importable")

    def analyze(self, path: str) -> AudioFeatures:
        import essentia.standard as es
        import numpy as np

        audio = es.MonoLoader(filename=path)()
        sr = es.MetadataReader(filename=path)()[3] if hasattr(es, "MetadataReader") else 44100

        bpm, _, _, _ = es.RhythmExtractor2013()(audio)

        key, scale, _ = es.KeyExtractor()(audio)
        key_str = f"{key} {scale}" if key and scale else ""

        # Loudness: try EBU R128 integrated loudness
        loudness = None
        if hasattr(es, "LoudnessEBUR128"):
            try:
                _, _, loudness = es.LoudnessEBUR128(sampleRate=sr)(audio)
            except Exception:
                pass

        if loudness is None and hasattr(es, "Loudness"):
            try:
                loudness = es.Loudness()(audio)
            except Exception:
                pass

        # Energy and valence proxies
        energy = None
        valence = None
        if hasattr(es, "Energy"):
            try:
                energy = float(es.Energy()(audio))
            except Exception:
                pass

        if energy is not None:
            # Simple valence proxy using high-frequency content
            try:
                spec = es.Spectrum()(audio)
                centroid = es.Centroid(range=sr / 2)(spec)
                valence = float(np.clip(0.6 * energy + 0.4 * (centroid / (sr / 2)), 0.0, 1.0))
            except Exception:
                valence = energy

        duration = float(es.Duration()(audio))
        # Essentia does not give a clean "outro" marker, so fall back to librosa later.
        outro_start = None

        ideal_crossfade = _ideal_crossfade(float(bpm))

        return AudioFeatures(
            bpm=float(bpm),
            key=key_str,
            loudness=None if loudness is None else round(float(loudness), 2),
            energy=None if energy is None else round(energy, 4),
            valence=None if valence is None else round(valence, 4),
            outro_start_seconds=None if outro_start is None else round(outro_start, 3),
            ideal_crossfade_seconds=round(ideal_crossfade, 3),
            intro_start_seconds=None,
            outro_end_seconds=None,
            chroma=None,
        )


def _ideal_crossfade(bpm: float) -> float:
    import numpy as np

    if bpm <= 0:
        return 4.0
    return float(np.clip(4.0 * (60.0 / bpm), 2.0, 10.0))
