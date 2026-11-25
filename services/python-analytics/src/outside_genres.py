"""Fetch and cache genres from outside sources, matched by ID only.

ADR 0008 decision 5: MusicBrainz by recording and release ids, Discogs
through the MusicBrainz release's Discogs link, Last.fm by exact artist and
title. No name search. Each recording, release, release group and Last.fm
track is looked up once and stored in outside_genres in the source's own
words, also when the source has nothing. It is looked up again once the row
is older than the max age, or sooner when an admin marks everything due.

Songs without MusicBrainz ids skip MusicBrainz and Discogs. Translating the
stored words into genres and styles is the genre engine's job, not this one.

A run spends at most a request budget per source, so the background job
works through a large library in short steps (see tasks.fetch_outside_genres).
"""

import logging
from dataclasses import dataclass, field
from typing import Iterable, List, Optional

from psycopg.types.json import Jsonb

from discogs import DiscogsClient, release_id_from_url
from lastfm import LastfmClient, LastfmError
from musicbrainz import MusicBrainzClient

log = logging.getLogger(__name__)

MB_RECORDING = "mb_recording"
MB_RELEASE = "mb_release"
MB_RELEASE_GROUP = "mb_release_group"
DISCOGS_RELEASE = "discogs_release"
LASTFM_TRACK = "lastfm_track"

# Joins artist and title in a lastfm_track ref.
LASTFM_SEP = "\x1f"

# Requests per run of a source's task: about 2.5 minutes of MusicBrainz (1
# a second), 1 minute of Discogs and 1 minute of Last.fm.
MB_REQUESTS_PER_RUN = 140
DISCOGS_REQUESTS_PER_RUN = 55
LASTFM_REQUESTS_PER_RUN = 240
# A source stops for the run after this many failed lookups. A lookup that
# succeeded after a busy answer and a retry (see http_json) is not one.
MAX_FAILURES = 5


def lastfm_ref(artist: str, title: str) -> str:
    return f"{artist}{LASTFM_SEP}{title}"


# The sources, each fetched by its own background task chain.
MUSICBRAINZ = "musicbrainz"
DISCOGS = "discogs"
LASTFM = "lastfm"
SOURCES = (MUSICBRAINZ, DISCOGS, LASTFM)

REQUESTS_PER_RUN = {
    MUSICBRAINZ: MB_REQUESTS_PER_RUN,
    DISCOGS: DISCOGS_REQUESTS_PER_RUN,
    LASTFM: LASTFM_REQUESTS_PER_RUN,
}


def make_client(source: str, key: Optional[str] = None, opener=None):
    """A client for one source; key is its Discogs token or Last.fm API key
    (see outside_sources.settings for where it comes from)."""
    import config

    ua = config.OUTSIDE_USER_AGENT
    if source == MUSICBRAINZ:
        return MusicBrainzClient(config.MUSICBRAINZ_URL, ua, opener=opener)
    if source == DISCOGS:
        return DiscogsClient(key or "", ua, opener=opener)
    if source == LASTFM:
        return LastfmClient(key or "", ua, opener=opener)
    raise ValueError(f"unknown outside genre source {source!r}")


@dataclass
class Budget:
    """Request budget and failure count of one source in one run."""

    requests: int
    failures: int = 0
    stopped: bool = False  # too many failures, or a bad credential
    refused: bool = False  # the source refused the token or key
    exhausted: bool = False  # the budget ran out with work left

    def left(self, client) -> bool:
        if self.stopped:
            return False
        if client.requests >= self.requests:
            self.exhausted = True
            return False
        return True

    def failed(self, what: str, exc: Exception) -> None:
        self.failures += 1
        log.warning("outside genres: %s failed: %s", what, exc)
        if self.failures >= MAX_FAILURES:
            self.stopped = True


@dataclass
class RunSummary:
    stored: int = 0
    # Rows whose genres changed (new rows with genres included).
    new_data: int = 0
    by_source: dict = field(default_factory=dict)
    # MusicBrainz releases stored with a Discogs link.
    discogs_links: int = 0
    failures: int = 0
    # Work is left for a next run (the budget ran out).
    more: bool = False
    # The source refused the token or key; the run stopped.
    refused: bool = False
    # Requests sent, retries included.
    requests: int = 0
    # Requests sent again after a busy answer (HTTP 429 or 503, Last.fm error 29).
    retries: int = 0

    @property
    def status(self) -> str:
        """ok, refused_key, or failed (stopped after MAX_FAILURES failed lookups)."""
        if self.refused:
            return "refused_key"
        return "failed" if self.failures >= MAX_FAILURES else "ok"

    def as_dict(self) -> dict:
        return {
            "status": self.status,
            "requests": self.requests,
            "retries": self.retries,
            "stored": self.stored,
            "new_data": self.new_data,
            "by_source": self.by_source,
            "discogs_links": self.discogs_links,
            "failures": self.failures,
            "more": self.more,
        }


def store(conn, summary: RunSummary, source: str, ref: str, genres: Optional[List[dict]],
          discogs_release_id: Optional[str] = None) -> None:
    """Upsert one lookup and commit; genres None means the source does not know the ref."""
    found = genres is not None
    genres = genres or []
    old = conn.execute(
        "SELECT genres FROM outside_genres WHERE source = %s AND ref = %s", (source, ref)
    ).fetchone()
    conn.execute(
        """
        INSERT INTO outside_genres (source, ref, found, genres, discogs_release_id, fetched_at)
        VALUES (%s, %s, %s, %s, %s, NOW())
        ON CONFLICT (source, ref) DO UPDATE SET
            found = EXCLUDED.found, genres = EXCLUDED.genres,
            discogs_release_id = EXCLUDED.discogs_release_id, fetched_at = NOW()
        """,
        (source, ref, found, Jsonb(genres), discogs_release_id),
    )
    conn.commit()
    summary.stored += 1
    summary.by_source[source] = summary.by_source.get(source, 0) + 1
    if (old[0] if old else []) != genres:
        summary.new_data += 1


def mark_all_due(conn, source: Optional[str] = None) -> int:
    """Make every cached lookup (of one source) due again; the caller commits."""
    cur = conn.execute(
        "UPDATE outside_genres SET fetched_at = 'epoch' WHERE %s::text IS NULL OR source = %s",
        (source, source),
    )
    return cur.rowcount


# --- Raw genre lists, in each source's own words -------------------------


def mb_genres(entity: dict) -> List[dict]:
    """MusicBrainz genres of a recording, release or release group, most votes first."""
    out = [
        {"name": g["name"], "count": int(g.get("count") or 0), "kind": "genre"}
        for g in entity.get("genres") or []
        if g.get("name")
    ]
    out.sort(key=lambda g: (-g["count"], g["name"]))
    return out


def discogs_genres(release: dict) -> List[dict]:
    return [{"name": n, "count": None, "kind": "genre"} for n in release.get("genres", [])] + [
        {"name": n, "count": None, "kind": "style"} for n in release.get("styles", [])
    ]


def lastfm_genres(tags: Iterable[dict]) -> List[dict]:
    return [{"name": t["name"], "count": t["count"], "kind": "tag"} for t in tags]


def discogs_link(release: dict) -> Optional[str]:
    """The Discogs release id of a MusicBrainz release's Discogs URL relation."""
    for rel in release.get("relations") or []:
        if rel.get("type") != "discogs":
            continue
        found = release_id_from_url((rel.get("url") or {}).get("resource", ""))
        if found:
            return found
    return None


# --- What is due ----------------------------------------------------------

_FRESH = """
    EXISTS (SELECT 1 FROM outside_genres o
            WHERE o.source = %(source)s AND o.ref = {ref}
              AND o.fetched_at > NOW() - make_interval(days => %(max_age)s))
"""


def _fresh(ref_sql: str) -> str:
    return _FRESH.format(ref=ref_sql)


def due_releases(conn, max_age: int) -> List[str]:
    sql = f"""
        SELECT r.mb_release_id FROM releases r
        WHERE r.mb_release_id IS NOT NULL AND NOT {_fresh("r.mb_release_id")}
        ORDER BY r.mb_release_id
    """
    return [r[0] for r in conn.execute(sql, {"source": MB_RELEASE, "max_age": max_age}).fetchall()]


def due_release_groups(conn, max_age: int) -> List[str]:
    sql = f"""
        SELECT DISTINCT r.mb_release_group_id FROM releases r
        WHERE r.mb_release_group_id IS NOT NULL AND NOT {_fresh("r.mb_release_group_id")}
        ORDER BY 1
    """
    return [r[0] for r in conn.execute(sql, {"source": MB_RELEASE_GROUP, "max_age": max_age}).fetchall()]


def due_recordings(conn, max_age: int) -> List[str]:
    """Recordings to look up one by one.

    A song whose release has a MusicBrainz id waits until its release was
    looked up: that lookup lists the release's recordings with their genres,
    one request for the whole release. Whatever it missed is looked up here.
    """
    sql = f"""
        SELECT DISTINCT t.mb_recording_id FROM tracks t
        LEFT JOIN releases r ON r.id = t.release_id
        WHERE t.mb_recording_id IS NOT NULL
          AND NOT {_fresh("t.mb_recording_id")}
          AND (r.mb_release_id IS NULL OR EXISTS (
                SELECT 1 FROM outside_genres o
                WHERE o.source = 'mb_release' AND o.ref = r.mb_release_id
                  AND o.fetched_at > NOW() - make_interval(days => %(max_age)s)))
        ORDER BY 1
    """
    return [r[0] for r in conn.execute(sql, {"source": MB_RECORDING, "max_age": max_age}).fetchall()]


def due_discogs_releases(conn, max_age: int) -> List[str]:
    sql = f"""
        SELECT DISTINCT m.discogs_release_id FROM outside_genres m
        WHERE m.source = 'mb_release' AND m.discogs_release_id IS NOT NULL
          AND NOT {_fresh("m.discogs_release_id")}
        ORDER BY 1
    """
    return [r[0] for r in conn.execute(sql, {"source": DISCOGS_RELEASE, "max_age": max_age}).fetchall()]


def due_lastfm_tracks(conn, max_age: int) -> List[tuple]:
    """(artist, title) of songs not looked up on Last.fm yet, as tagged."""
    sql = f"""
        SELECT DISTINCT a.name, t.title FROM tracks t
        JOIN artists a ON a.id = t.artist_id
        WHERE btrim(a.name) <> '' AND btrim(t.title) <> '' AND lower(a.name) <> 'unknown artist'
          AND NOT {_fresh("a.name || chr(31) || t.title")}
        ORDER BY 1, 2
    """
    return [tuple(r) for r in conn.execute(sql, {"source": LASTFM_TRACK, "max_age": max_age}).fetchall()]


# --- One run --------------------------------------------------------------


def fetch_due(conn, source: str, client, max_age: int) -> RunSummary:
    """Look up what is due from one source, within its request budget."""
    summary = RunSummary()
    budget = Budget(REQUESTS_PER_RUN[source])
    fetch = {MUSICBRAINZ: _fetch_musicbrainz, DISCOGS: _fetch_discogs, LASTFM: _fetch_lastfm}[source]
    fetch(conn, client, budget, summary, max_age)
    summary.failures = budget.failures
    summary.refused = budget.refused
    summary.requests = client.requests
    summary.retries = client.retries
    summary.more = budget.exhausted and not budget.stopped
    return summary


def _fetch_musicbrainz(conn, mb: MusicBrainzClient, budget: Budget, summary: RunSummary, max_age: int) -> None:
    for release_id in due_releases(conn, max_age):
        if not budget.left(mb):
            return
        try:
            release = mb.release(release_id)
            recordings = mb.release_recordings(release_id) if release is not None else []
        except Exception as exc:
            budget.failed(f"MusicBrainz release {release_id}", exc)
            continue
        if release is None:
            store(conn, summary, MB_RELEASE, release_id, None)
            continue
        discogs_id = discogs_link(release)
        store(conn, summary, MB_RELEASE, release_id, mb_genres(release), discogs_release_id=discogs_id)
        if discogs_id:
            summary.discogs_links += 1
        group = release.get("release-group") or {}
        # The release group's genres come along when MusicBrainz includes them.
        if group.get("id") and "genres" in group:
            store(conn, summary, MB_RELEASE_GROUP, group["id"], mb_genres(group))
        for rec in recordings:
            if rec.get("id"):
                store(conn, summary, MB_RECORDING, rec["id"], mb_genres(rec))

    for group_id in due_release_groups(conn, max_age):
        if not budget.left(mb):
            return
        try:
            group = mb.release_group(group_id)
        except Exception as exc:
            budget.failed(f"MusicBrainz release group {group_id}", exc)
            continue
        store(conn, summary, MB_RELEASE_GROUP, group_id, None if group is None else mb_genres(group))

    for recording_id in due_recordings(conn, max_age):
        if not budget.left(mb):
            return
        try:
            rec = mb.recording(recording_id)
        except Exception as exc:
            budget.failed(f"MusicBrainz recording {recording_id}", exc)
            continue
        store(conn, summary, MB_RECORDING, recording_id, None if rec is None else mb_genres(rec))


def _fetch_discogs(conn, discogs: DiscogsClient, budget: Budget, summary: RunSummary, max_age: int) -> None:
    for release_id in due_discogs_releases(conn, max_age):
        if not budget.left(discogs):
            return
        try:
            release = discogs.release(release_id)
        except Exception as exc:
            if getattr(exc, "code", None) in (401, 403):
                log.warning(
                    "outside genres: Discogs refused the token (HTTP %s); check it on the admin"
                    " Outside sources page or in DISCOGS_TOKEN", exc.code,
                )
                budget.stopped = budget.refused = True
                return
            budget.failed(f"Discogs release {release_id}", exc)
            continue
        store(conn, summary, DISCOGS_RELEASE, release_id, None if release is None else discogs_genres(release))


def _fetch_lastfm(conn, lastfm: LastfmClient, budget: Budget, summary: RunSummary, max_age: int) -> None:
    for artist, title in due_lastfm_tracks(conn, max_age):
        if not budget.left(lastfm):
            return
        try:
            tags = lastfm.track_top_tags(artist, title)
        except LastfmError as exc:
            if exc.bad_key:
                log.warning(
                    "outside genres: Last.fm refused the API key (error %s); check it on the admin"
                    " Outside sources page or in LASTFM_API_KEY", exc.code,
                )
                budget.stopped = budget.refused = True
                return
            budget.failed("Last.fm track", exc)
            continue
        except Exception as exc:
            budget.failed("Last.fm track", exc)
            continue
        store(conn, summary, LASTFM_TRACK, lastfm_ref(artist, title), None if tags is None else lastfm_genres(tags))
