from typing import Any, Optional

from mutagen import File as MutagenFile
from mutagen.flac import FLAC
from mutagen.mp3 import MP3


def read_tags(path: str) -> dict[str, Any]:
    """Read basic audio tags using mutagen.

    easy=True gives MP3s the same key names as FLAC ("title", "genre"…)
    instead of raw ID3 frame ids.
    """
    try:
        f = MutagenFile(path, easy=True)
    except Exception:
        f = None

    if f is None:
        return {}

    tags = {}

    def get(key: str) -> Optional[str]:
        for k in (key, key.upper()):
            if k in f:
                val = f[k]
                if isinstance(val, list):
                    return str(val[0])
                return str(val)
        return None

    def get_all(key: str) -> Optional[str]:
        """All values of a multi-value tag, joined with "; "."""
        for k in (key, key.upper()):
            if k in f:
                val = f[k]
                vals = val if isinstance(val, list) else [val]
                joined = "; ".join(str(v).strip() for v in vals if str(v).strip())
                return joined or None
        return None

    if isinstance(f, (FLAC, MP3)):
        tags["title"] = get("title") or _filename_title(path)
        tags["artist"] = get("artist") or "Unknown Artist"
        tags["album"] = get("album") or "Unknown Album"
        tags["date"] = get("date")
        tags["genre"] = get_all("genre")
        tags["tracknumber"] = get("tracknumber")

    if not tags:
        tags["title"] = _filename_title(path)
        tags["artist"] = "Unknown Artist"
        tags["album"] = "Unknown Album"

    return tags


def _filename_title(path: str) -> str:
    from pathlib import Path

    p = Path(path)
    return p.stem
