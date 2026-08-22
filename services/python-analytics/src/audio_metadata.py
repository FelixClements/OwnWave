"""Shared file/tag helpers for library scanning."""

from datetime import datetime, timezone
from pathlib import Path
from typing import Optional, Tuple

SUPPORTED_EXTS = {".flac", ".mp3"}


def get_file_stats(path: Path) -> Tuple[int, datetime]:
    stat = path.stat()
    mtime = datetime.fromtimestamp(stat.st_mtime, tz=timezone.utc)
    return stat.st_size, mtime


def parse_int_tag(value) -> Optional[int]:
    if not value:
        return None
    try:
        return int(str(value).split("/")[0])
    except Exception:
        return None


def get_duration(path: str) -> Optional[float]:
    try:
        from mutagen import File as MutagenFile

        return getattr(MutagenFile(path).info, "length", None)
    except Exception:
        return None


def get_sample_info(path: str) -> Tuple[Optional[int], Optional[int]]:
    try:
        from mutagen import File as MutagenFile

        info = MutagenFile(path).info
        return getattr(info, "sample_rate", None), getattr(info, "channels", None)
    except Exception:
        return None, None
