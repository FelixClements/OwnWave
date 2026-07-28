from typing import List

from analyzer_librosa import LibrosaAnalyzer
from analyzer_essentia import EssentiaAnalyzer, is_available
from models import AudioFeatures, Analyzer, merge_features


def get_analyzers() -> List[Analyzer]:
    analyzers: List[Analyzer] = [LibrosaAnalyzer()]
    if is_available():
        try:
            analyzers.append(EssentiaAnalyzer())
        except Exception:
            pass
    return analyzers


def analyze_file(path: str, analyzers: List[Analyzer] = None) -> AudioFeatures:
    if analyzers is None:
        analyzers = get_analyzers()

    results: List[AudioFeatures] = []
    for analyzer in analyzers:
        try:
            results.append(analyzer.analyze(path))
        except Exception:
            # If one analyzer fails, continue with the others.
            pass

    if not results:
        raise RuntimeError(f"All analyzers failed for {path}")

    # Start with the librosa result (first) and merge in the rest.
    merged = results[0]
    for extra in results[1:]:
        merged = merge_features(merged, extra)
    return merged
