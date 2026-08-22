from typing import Any, Optional
from uuid import UUID

SEED_TYPES = frozenset(
    {"track", "artist", "album", "mood", "cluster", "genre", "sub_genre", "uncategorized"}
)

_NUMERIC_FILTER_KEYS = (
    "min_bpm",
    "max_bpm",
    "min_energy",
    "max_energy",
    "min_valence",
    "max_valence",
)


def parse_seed(raw: Optional[dict]) -> Optional[dict]:
    """Normalize a stored or API seed dict to always use the ``type`` key."""
    if not raw:
        return None
    out = {k: v for k, v in raw.items() if v is not None}
    if "type" not in out and "seed_type" in out:
        out["type"] = out.pop("seed_type")
    elif "seed_type" in out:
        out.pop("seed_type", None)
    seed_type = out.get("type")
    if seed_type and seed_type not in SEED_TYPES:
        raise ValueError(f"unsupported seed type: {seed_type}")
    return out or None


def _append_numeric_filters(filters: dict, req: Any) -> None:
    for key in _NUMERIC_FILTER_KEYS:
        value = getattr(req, key, None)
        if value is not None:
            filters[key] = value


def seed_from_request(req: Any) -> dict:
    """Convert a create/update station request into a canonical seed filter."""
    filters: dict = {}

    if req.seed_type == "track" and req.track_id:
        filters = {"type": "track", "track_id": str(req.track_id)}
    elif req.seed_type == "artist" and req.artist_id:
        filters = {"type": "artist", "artist_id": str(req.artist_id)}
    elif req.seed_type == "album" and req.album_id:
        filters = {"type": "album", "album_id": str(req.album_id)}
    elif req.seed_type == "cluster" and req.cluster_id is not None:
        filters = {"type": "cluster", "cluster_id": req.cluster_id}
    elif req.seed_type == "mood":
        filters = {"type": "mood"}
    elif req.seed_type == "genre" and req.main_genre:
        filters = {"type": "genre", "main_genre": req.main_genre}
    elif req.seed_type == "sub_genre" and req.main_genre and req.sub_genre:
        filters = {
            "type": "sub_genre",
            "main_genre": req.main_genre,
            "sub_genre": req.sub_genre,
        }
    elif req.seed_type == "uncategorized":
        filters = {"type": "uncategorized"}

    _append_numeric_filters(filters, req)
    return filters


def track_seed_id(seed: Optional[dict]) -> Optional[UUID]:
    if not seed or seed.get("type") != "track":
        return None
    track_id = seed.get("track_id")
    return UUID(track_id) if track_id else None
