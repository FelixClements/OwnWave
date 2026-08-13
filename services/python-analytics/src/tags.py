from typing import Any, List, Optional

from models import GenrePrediction
from mutagen import File as MutagenFile
from mutagen.flac import FLAC
from mutagen.mp3 import MP3


def read_tags(path: str) -> dict[str, Any]:
    """Read basic audio tags using mutagen."""
    try:
        f = MutagenFile(path)
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

    if isinstance(f, (FLAC, MP3)):
        tags["title"] = get("title") or _filename_title(path)
        tags["artist"] = get("artist") or "Unknown Artist"
        tags["album"] = get("album") or "Unknown Album"
        tags["date"] = get("date")
        tags["genre"] = get("genre")
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


def extract_genres_from_tags(tags: dict[str, Any]) -> List[GenrePrediction]:
    """Parse a raw genre tag into main/sub genre predictions.

    Splits on common delimiters (;, /, \\, ' - '). The first part is the
    main genre, the second part is the sub genre. If only one value is
    present, both main and sub are set to that value.
    """
    raw = tags.get("genre")
    if not raw:
        return []

    raw = str(raw).strip()
    if not raw:
        return []

    # Split on the first delimiter found, preferring separators that imply hierarchy.
    for sep in (" - ", ", ", ";", ",", "/", "\\"):
        if sep in raw:
            parts = [p.strip() for p in raw.split(sep, 1)]
            main = parts[0]
            sub = parts[1] if len(parts) > 1 and parts[1] else main
            if main and sub:
                return [GenrePrediction(main, sub, 1.0, "tags")]

    # Single value: use it for both main and sub genre.
    return [GenrePrediction(raw, raw, 1.0, "tags")]
