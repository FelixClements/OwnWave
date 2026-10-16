from dataclasses import dataclass


@dataclass(frozen=True)
class ScanOptions:
    force: bool = False
    analyze_audio: bool = True
