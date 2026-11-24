import re
from typing import Any, List, Optional

from mutagen import File as MutagenFile
from mutagen.flac import FLAC
from mutagen.mp3 import MP3


# Version of what read_tags reads. Raise it when read_tags learns a new tag:
# tracks store the version they were read with, and a scan re-reads every
# file whose track has an older one, even when the file did not change.
# 1: album artist, ISRC, MusicBrainz ids, release type, compilation (#134).
TAGS_VERSION = 1

_MB_ID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")

# Album artists that mark a compilation, compared case-insensitively.
VARIOUS_ARTISTS = {"various artists", "various", "va", "v.a."}
# The MusicBrainz "Various Artists" artist.
VARIOUS_ARTISTS_MBID = "89ad4ac3-39f7-470e-963a-56509c546377"


def read_tags(path: str) -> dict[str, Any]:
    """Read audio tags using mutagen.

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
        tags["albumartist"] = get("albumartist") or get("album artist")
        tags["isrc"] = _isrc(get("isrc"))
        # MUSICBRAINZ_TRACKID names the recording, not the release track.
        tags["mb_recording_id"] = _first_mb_id(get_all("musicbrainz_trackid"))
        tags["mb_release_id"] = _first_mb_id(get_all("musicbrainz_albumid"))
        tags["mb_release_group_id"] = _first_mb_id(get_all("musicbrainz_releasegroupid"))
        tags["mb_artist_ids"] = _mb_ids(get_all("musicbrainz_artistid")) or None
        tags["mb_album_artist_ids"] = _mb_ids(get_all("musicbrainz_albumartistid")) or None
        # Picard writes RELEASETYPE to FLAC and "MusicBrainz Album Type" to MP3.
        tags["releasetype"] = get_all("releasetype") or get_all("musicbrainz_albumtype")
        tags["compilation"] = get("compilation")

    if not tags:
        tags["title"] = _filename_title(path)
        tags["artist"] = "Unknown Artist"
        tags["album"] = "Unknown Album"

    return tags


def tagged_compilation(tags: dict[str, Any]) -> bool:
    """Whether a file's tags say its release is a compilation: album artist
    "Various Artists", a MusicBrainz release type of compilation, or the
    compilation flag (iTunes, Picard)."""
    album_artist = (tags.get("albumartist") or "").strip().casefold()
    if album_artist in VARIOUS_ARTISTS:
        return True
    if VARIOUS_ARTISTS_MBID in (tags.get("mb_album_artist_ids") or []):
        return True
    release_type = (tags.get("releasetype") or "").casefold()
    if "compilation" in re.split(r"[;,/\s]+", release_type):
        return True
    return (tags.get("compilation") or "").strip() == "1"


def _mb_ids(value: Optional[str]) -> List[str]:
    """MusicBrainz ids (UUIDs) in a tag value, in order, without repeats.
    Multi-value tags arrive joined by "; " or "/" (ID3v2.3)."""
    ids: List[str] = []
    for mbid in _MB_ID.findall((value or "").lower()):
        if mbid not in ids:
            ids.append(mbid)
    return ids


def _first_mb_id(value: Optional[str]) -> Optional[str]:
    ids = _mb_ids(value)
    return ids[0] if ids else None


def _isrc(value: Optional[str]) -> Optional[str]:
    """An ISRC normalised to 12 upper-case characters (dashes and spaces
    dropped), or None when the tag is not one."""
    if not value:
        return None
    isrc = re.sub(r"[\s-]", "", value).upper()
    return isrc if re.fullmatch(r"[A-Z]{2}[A-Z0-9]{3}[0-9]{7}", isrc) else None


def _filename_title(path: str) -> str:
    from pathlib import Path

    p = Path(path)
    return p.stem
