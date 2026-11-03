"""Blend genre sources into each track's station genres.

Sources, each first turned into a distribution over station genres
(genre_taxonomy):

- audio: the Discogs400 model's activations,
- tags: the file's genre tag(s),
- musicbrainz: the artist's MusicBrainz tags, weighted by votes.

The blend is a weighted average; a missing source's weight is shared by
the others. The top genre is the track's primary genre; any other genre
scoring at least SECONDARY_RATIO of the top is a secondary genre, so a
track can sit in more than one station.
"""

import logging
from dataclasses import dataclass
from typing import Dict, List, Optional

import genre_taxonomy as tx

log = logging.getLogger(__name__)

WEIGHTS = {"audio": 0.5, "tags": 0.35, "musicbrainz": 0.15}
SECONDARY_RATIO = 0.6
MAX_GENRES = 3
MAX_MB_TAGS = 8


@dataclass
class StationGenre:
    genre: str
    score: float
    is_primary: bool


def blend(sources: Dict[str, Dict[str, float]], weights: Dict[str, float] = WEIGHTS) -> Dict[str, float]:
    present = {s: d for s, d in sources.items() if d and weights.get(s, 0) > 0}
    total_w = sum(weights[s] for s in present)
    if total_w <= 0:
        return {}
    out: Dict[str, float] = {}
    for s, dist in present.items():
        w = weights[s] / total_w
        for g, v in dist.items():
            out[g] = out.get(g, 0.0) + w * v
    return out


def assign(scores: Dict[str, float]) -> List[StationGenre]:
    ranked = sorted(scores.items(), key=lambda kv: (-kv[1], kv[0]))
    if not ranked:
        return []
    top = ranked[0][1]
    out = [StationGenre(ranked[0][0], ranked[0][1], True)]
    for g, v in ranked[1:MAX_GENRES]:
        if v >= top * SECONDARY_RATIO:
            out.append(StationGenre(g, v, False))
    return out


def track_sources(
    labels: Optional[List[str]],
    activations: Optional[List[float]],
    genre_tag: Optional[str],
    mb_tags: Optional[List[dict]],
) -> Dict[str, Dict[str, float]]:
    sources: Dict[str, Dict[str, float]] = {}
    if labels and activations and len(labels) == len(activations):
        sources["audio"] = tx.discogs_distribution(labels, activations)
    if genre_tag:
        sources["tags"] = tx.text_distribution(tx.split_tag(genre_tag))
    if mb_tags:
        top = mb_tags[:MAX_MB_TAGS]
        sources["musicbrainz"] = tx.text_distribution([t["name"] for t in top], [t["count"] for t in top])
    return {s: d for s, d in sources.items() if d}


def score_track(labels, activations, genre_tag, mb_tags) -> List[StationGenre]:
    return assign(blend(track_sources(labels, activations, genre_tag, mb_tags)))


def primary_of(dist: Dict[str, float]) -> Optional[str]:
    return max(dist.items(), key=lambda kv: (kv[1], kv[0]))[0] if dist else None


# --- database pass -----------------------------------------------------------

_TRACKS_SQL = """
    SELECT t.id, a.name, t.genre_tag, g.activations
    FROM tracks t
    LEFT JOIN artists a ON a.id = t.artist_id
    LEFT JOIN track_audio_genres g ON g.track_id = t.id
"""


def _mb_tags_by_key(conn) -> Dict[str, List[dict]]:
    rows = conn.execute("SELECT name_key, tags FROM musicbrainz_artists WHERE mbid IS NOT NULL").fetchall()
    return {r[0]: r[1] for r in rows}


def artist_mb_tags(credit: Optional[str], by_key: Dict[str, List[dict]]) -> Optional[List[dict]]:
    """MusicBrainz tags for a credit: the whole credit first, then its primary artist."""
    from musicbrainz import name_key, primary_artist

    if not credit:
        return None
    tags = by_key.get(name_key(credit))
    if tags is None:
        tags = by_key.get(name_key(primary_artist(credit)))
    return tags


def score_library(conn, labels: Optional[List[str]]) -> dict:
    """Recompute track_station_genres for every track. The caller commits."""
    mb = _mb_tags_by_key(conn)
    rows = conn.execute(_TRACKS_SQL).fetchall()
    values = []
    scored = 0
    for track_id, artist, genre_tag, activations in rows:
        genres = score_track(labels, activations, genre_tag, artist_mb_tags(artist, mb))
        if genres:
            scored += 1
        values.extend((track_id, g.genre, g.score, g.is_primary) for g in genres)
    conn.execute("DELETE FROM track_station_genres")
    with conn.cursor() as cur:
        cur.executemany(
            "INSERT INTO track_station_genres (track_id, genre, score, is_primary) VALUES (%s, %s, %s, %s)",
            values,
        )
    log.info("scored genres for %d of %d tracks", scored, len(rows))
    return {"tracks": len(rows), "scored": scored}


# --- reports -------------------------------------------------------------------

# A warning is raised when one genre holds more than this share of tracks.
DOMINANCE_WARNING = 0.5


def distribution(conn) -> dict:
    """Primary/any counts per station genre, with a warning on heavy skew."""
    total = conn.execute("SELECT COUNT(DISTINCT track_id) FROM track_station_genres").fetchone()[0]
    rows = conn.execute(
        """
        SELECT genre, COUNT(*) FILTER (WHERE is_primary), COUNT(*)
        FROM track_station_genres GROUP BY genre
        """
    ).fetchall()
    counts = {r[0]: (r[1], r[2]) for r in rows}
    genres = [
        {
            "genre": g,
            "name": name,
            "primary_tracks": counts.get(g, (0, 0))[0],
            "tracks": counts.get(g, (0, 0))[1],
            "station": g not in tx.NON_STATION_GENRES,
        }
        for g, name in tx.STATION_GENRES.items()
    ]
    warnings = []
    for g in genres:
        if total and g["primary_tracks"] / total > DOMINANCE_WARNING:
            warnings.append(
                f"{g['name']} is the primary genre of {g['primary_tracks']} of {total} tracks "
                f"({100 * g['primary_tracks'] // total}%). Check the genre report."
            )
    return {"scored_tracks": total, "genres": genres, "warnings": warnings}


def agreement_report(conn, labels: Optional[List[str]]) -> dict:
    """How often the audio model alone agrees with a file's genre tag.

    Only tracks whose tag maps to exactly one station genre count: those
    tags are the ground truth.
    """
    rows = conn.execute(
        """
        SELECT t.genre_tag, g.activations FROM tracks t
        JOIN track_audio_genres g ON g.track_id = t.id
        WHERE t.genre_tag IS NOT NULL AND t.genre_tag <> ''
        """
    ).fetchall()
    per_genre: Dict[str, Dict[str, int]] = {}
    confusion: Dict[tuple, int] = {}
    for genre_tag, activations in rows:
        tag_dist = tx.text_distribution(tx.split_tag(genre_tag))
        if len(tag_dist) != 1 or not labels:
            continue
        truth = next(iter(tag_dist))
        predicted = primary_of(tx.discogs_distribution(labels, activations))
        stats = per_genre.setdefault(truth, {"tracks": 0, "agree": 0})
        stats["tracks"] += 1
        if predicted == truth:
            stats["agree"] += 1
        else:
            confusion[(truth, predicted)] = confusion.get((truth, predicted), 0) + 1
    total = sum(s["tracks"] for s in per_genre.values())
    agree = sum(s["agree"] for s in per_genre.values())
    return {
        "tracks": total,
        "audio_agreement": round(agree / total, 3) if total else None,
        "per_genre": {
            g: {**s, "agreement": round(s["agree"] / s["tracks"], 3)}
            for g, s in sorted(per_genre.items(), key=lambda kv: -kv[1]["tracks"])
        },
        "top_confusions": [
            {"tag": t, "audio": p, "tracks": n}
            for (t, p), n in sorted(confusion.items(), key=lambda kv: -kv[1])[:15]
        ],
    }
