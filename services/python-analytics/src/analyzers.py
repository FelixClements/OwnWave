from typing import List, Optional

from analyzer_loudness import LoudnessAnalyzer
from models import Analyzer, AudioFeatures


def get_analyzers() -> List[Analyzer]:
    return [LoudnessAnalyzer()]


def analyze_file(path: str, analyzers: Optional[List[Analyzer]] = None) -> AudioFeatures:
    """Run the analyzers; the first one that succeeds wins."""
    errors = []
    for analyzer in analyzers or get_analyzers():
        try:
            return analyzer.analyze(path)
        except Exception as exc:
            errors.append(f"{type(analyzer).__name__}: {exc}")
    raise RuntimeError(f"All analyzers failed for {path}: {'; '.join(errors)}")
