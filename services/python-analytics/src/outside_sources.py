"""Settings, status and match stats of the outside genre sources (issue #148).

The admin "Outside sources" page sets each source's switch and, for Discogs
and Last.fm, its token or API key. A value saved on the page overrides the
stack env (MUSICBRAINZ_ENABLED, DISCOGS_ENABLED / DISCOGS_TOKEN,
LASTFM_ENABLED / LASTFM_API_KEY); with none saved, the env decides, as
before. Saved secrets are encrypted (secret_box) and never leave this
service: the API reports only whether one is set, where it comes from and
its last 4 characters.

The stats read outside_genres (what was looked up, found or not) against
the library (what could be looked up at all), plus how many of the raw
values the genre catalog can translate.
"""

import logging
import urllib.error
from dataclasses import dataclass
from functools import lru_cache
from typing import Dict, List, Optional

from psycopg.types.json import Jsonb

import secret_box
from outside_genres import (
    DISCOGS,
    DISCOGS_RELEASE,
    LASTFM,
    LASTFM_TRACK,
    MB_RECORDING,
    MB_RELEASE,
    MB_RELEASE_GROUP,
    MUSICBRAINZ,
    REQUESTS_PER_RUN,
    SOURCES,
    make_client,
)

log = logging.getLogger(__name__)

NAMES = {MUSICBRAINZ: "MusicBrainz", DISCOGS: "Discogs", LASTFM: "Last.fm"}
# The sources that need a secret, and the env var holding it.
KEY_ENV = {DISCOGS: "DISCOGS_TOKEN", LASTFM: "LASTFM_API_KEY"}
SWITCH_ENV = {MUSICBRAINZ: "MUSICBRAINZ_ENABLED", DISCOGS: "DISCOGS_ENABLED", LASTFM: "LASTFM_ENABLED"}
# outside_genres sources each source writes.
ROW_SOURCES = {
    MUSICBRAINZ: (MB_RECORDING, MB_RELEASE, MB_RELEASE_GROUP),
    DISCOGS: (DISCOGS_RELEASE,),
    LASTFM: (LASTFM_TRACK,),
}
MAX_KEY_LEN = 200
# Raw values listed per source on the page.
TOP_VALUES = 15
TOP_UNTRANSLATED = 30


class UnknownSource(ValueError):
    pass


def check_source(source: str) -> str:
    if source not in SOURCES:
        raise UnknownSource(f"unknown source {source!r}")
    return source


# --- Settings ------------------------------------------------------------------


@dataclass
class SourceSettings:
    source: str
    enabled: bool
    enabled_from: str  # page or env
    # The token or key in use. It stays in this service: never return it.
    key: Optional[str] = None
    key_from: Optional[str] = None  # page, env, or None when unset
    key_last4: str = ""
    # A key is saved on the page but cannot be decrypted (the encryption key changed).
    key_unreadable: bool = False
    key_updated_at: Optional[object] = None
    last_run: Optional[dict] = None
    last_run_at: Optional[object] = None

    @property
    def needs_key(self) -> bool:
        return self.source in KEY_ENV

    @property
    def usable(self) -> bool:
        """Switched on, with its key when it needs one."""
        return self.enabled and (not self.needs_key or bool(self.key))

    def __repr__(self) -> str:  # keep the key out of logs and tracebacks
        return f"SourceSettings({self.source}, enabled={self.enabled}, key_from={self.key_from})"


def _row(conn, source: str):
    return conn.execute(
        "SELECT enabled, secret, secret_last4, secret_updated_at, last_run, last_run_at"
        " FROM outside_sources WHERE source = %s",
        (source,),
    ).fetchone()


def settings(conn, source: str) -> SourceSettings:
    """A source's effective settings: the page's values, else the env's."""
    import config

    check_source(source)
    row = _row(conn, source)
    enabled, secret, last4, updated_at, last_run, last_run_at = row or (None, None, None, None, None, None)
    s = SourceSettings(
        source,
        enabled=bool(getattr(config, SWITCH_ENV[source])) if enabled is None else enabled,
        enabled_from="env" if enabled is None else "page",
        key_updated_at=updated_at,
        last_run=last_run,
        last_run_at=last_run_at,
    )
    if not s.needs_key:
        return s
    if secret is not None:
        try:
            s.key, s.key_from, s.key_last4 = secret_box.decrypt(secret, source), "page", last4 or ""
            return s
        except (secret_box.SecretUnreadable, RuntimeError, ValueError):
            s.key_unreadable = True
    env_key = getattr(config, KEY_ENV[source]) or ""
    if env_key:
        s.key, s.key_from, s.key_last4 = env_key, "env", secret_box.last4(env_key)
    return s


def all_settings(conn) -> Dict[str, SourceSettings]:
    return {source: settings(conn, source) for source in SOURCES}


def enabled_sources(conn) -> List[str]:
    """The sources switched on, with their key set when they need one."""
    return [source for source, s in all_settings(conn).items() if s.usable]


def _ensure_row(conn, source: str) -> None:
    conn.execute("INSERT INTO outside_sources (source) VALUES (%s) ON CONFLICT DO NOTHING", (source,))


def save_switch(conn, source: str, enabled: Optional[bool]) -> None:
    """Switch a source on or off on the page; None goes back to the env switch. The caller commits."""
    check_source(source)
    _ensure_row(conn, source)
    conn.execute("UPDATE outside_sources SET enabled = %s WHERE source = %s", (enabled, source))


def save_key(conn, source: str, key: Optional[str]) -> None:
    """Save (encrypted) or, with None, remove a source's page key. The caller commits."""
    check_source(source)
    if source not in KEY_ENV:
        raise ValueError(f"{NAMES[source]} needs no key")
    _ensure_row(conn, source)
    if key is None:
        conn.execute(
            "UPDATE outside_sources SET secret = NULL, secret_last4 = NULL, secret_updated_at = NOW() WHERE source = %s",
            (source,),
        )
        return
    key = clean_key(key)
    conn.execute(
        "UPDATE outside_sources SET secret = %s, secret_last4 = %s, secret_updated_at = NOW() WHERE source = %s",
        (secret_box.encrypt(key, source), secret_box.last4(key), source),
    )


def clean_key(key: str) -> str:
    key = (key or "").strip()
    if not key:
        raise ValueError("the key is empty")
    if len(key) > MAX_KEY_LEN or any(c.isspace() or not c.isprintable() for c in key):
        raise ValueError("that does not look like a token or API key")
    return key


def record_run(conn, source: str, run: dict) -> None:
    """Remember a fetch run's outcome for the page. The caller commits."""
    _ensure_row(conn, source)
    conn.execute(
        "UPDATE outside_sources SET last_run = %s, last_run_at = NOW() WHERE source = %s",
        (Jsonb(run), source),
    )


# --- Test button -----------------------------------------------------------------


def _scrub(message: str, key: Optional[str]) -> str:
    return message.replace(key, "…") if key else message


def check_connection(conn, source: str, key: Optional[str] = None, opener=None) -> dict:
    """One cheap real request with the given key, or else the key in use.

    Returns {"ok", "message"}; the message never holds the key.
    """
    from lastfm import LastfmError

    check_source(source)
    name = NAMES[source]
    if key is not None and source in KEY_ENV:
        key = clean_key(key)
    elif source in KEY_ENV:
        key = settings(conn, source).key
        if not key:
            return {"ok": False, "message": f"No {name} key is set."}
    else:
        key = None
    client = make_client(source, key, opener=opener)
    try:
        if source == MUSICBRAINZ:
            client.ping()
            return {"ok": True, "message": "MusicBrainz answered."}
        if source == DISCOGS:
            if client.identity() is None:
                return {"ok": False, "message": "Discogs did not recognise the request (HTTP 404)."}
            return {"ok": True, "message": "Discogs accepted the token."}
        client.check_key()
        return {"ok": True, "message": "Last.fm accepted the key."}
    except LastfmError as exc:
        what = "refused the key" if exc.bad_key else "answered with an error"
        return {"ok": False, "message": _scrub(f"Last.fm {what}: {exc}", key)}
    except urllib.error.HTTPError as exc:
        if source == DISCOGS and exc.code in (401, 403):
            return {"ok": False, "message": f"Discogs refused the token (HTTP {exc.code})."}
        return {"ok": False, "message": _scrub(f"{name} answered HTTP {exc.code}.", key)}
    except Exception as exc:  # network errors, bad JSON
        log.info("outside sources: %s test failed: %s", name, type(exc).__name__)
        return {"ok": False, "message": _scrub(f"Could not reach {name}: {exc}", key)}


# --- Status and stats ------------------------------------------------------------

_COUNTS = """
    count(*) AS covered,
    count(*) FILTER (WHERE {looked}) AS looked_up,
    count(*) FILTER (WHERE {found}) AS found,
    count(*) FILTER (WHERE ({looked}) AND NOT ({found})) AS not_found,
    count(*) FILTER (WHERE NOT ({fresh})) AS due
"""


def _counts(looked: str, found: str, fresh: str) -> str:
    return _COUNTS.format(looked=looked, found=found, fresh=fresh)


_CUTOFF = "NOW() - make_interval(days => %(max_age)s)"
_ROW = "o.ref IS NOT NULL"
_ROW_FOUND = "coalesce(o.found, false)"
_ROW_FRESH = f"o.ref IS NOT NULL AND o.fetched_at > {_CUTOFF}"
# MusicBrainz songs: by recording id, or by release when they have none.
_MB_SONG_COUNTS = _counts(
    "rec.ref IS NOT NULL OR rel.ref IS NOT NULL",
    "coalesce(rec.found, false) OR coalesce(rel.found, false)",
    f"(rec.ref IS NOT NULL AND rec.fetched_at > {_CUTOFF})"
    f" OR (t.mb_recording_id IS NULL AND rel.ref IS NOT NULL AND rel.fetched_at > {_CUTOFF})",
)
_ROW_COUNTS = _counts(_ROW, _ROW_FOUND, _ROW_FRESH)
_LOOKED_COUNTS = _counts("looked", "found", "fresh")

# Each query returns covered, looked_up, found, not_found, due: what could
# be looked up at all, and what the cache says about it.
_MATCH_SQL = {
    (MUSICBRAINZ, "releases"): f"""
        SELECT {_ROW_COUNTS}
        FROM releases r
        LEFT JOIN outside_genres o ON o.source = 'mb_release' AND o.ref = r.mb_release_id
        WHERE r.mb_release_id IS NOT NULL
    """,
    # A song is looked up through its recording id, or through its release
    # when it has no recording id.
    (MUSICBRAINZ, "songs"): f"""
        SELECT {_MB_SONG_COUNTS}
        FROM tracks t
        LEFT JOIN releases r ON r.id = t.release_id
        LEFT JOIN outside_genres rec ON rec.source = 'mb_recording' AND rec.ref = t.mb_recording_id
        LEFT JOIN outside_genres rel ON rel.source = 'mb_release' AND rel.ref = r.mb_release_id
        WHERE t.mb_recording_id IS NOT NULL OR r.mb_release_id IS NOT NULL
    """,
    # Discogs is reached only through a MusicBrainz release's Discogs link.
    (DISCOGS, "releases"): f"""
        SELECT {_ROW_COUNTS}
        FROM releases r
        JOIN outside_genres m ON m.source = 'mb_release' AND m.ref = r.mb_release_id
                             AND m.discogs_release_id IS NOT NULL
        LEFT JOIN outside_genres o ON o.source = 'discogs_release' AND o.ref = m.discogs_release_id
    """,
    (DISCOGS, "songs"): f"""
        SELECT {_ROW_COUNTS}
        FROM tracks t
        JOIN releases r ON r.id = t.release_id
        JOIN outside_genres m ON m.source = 'mb_release' AND m.ref = r.mb_release_id
                             AND m.discogs_release_id IS NOT NULL
        LEFT JOIN outside_genres o ON o.source = 'discogs_release' AND o.ref = m.discogs_release_id
    """,
    (LASTFM, "songs"): f"""
        SELECT {_ROW_COUNTS}
        FROM tracks t
        JOIN artists a ON a.id = t.artist_id
        LEFT JOIN outside_genres o ON o.source = 'lastfm_track' AND o.ref = a.name || chr(31) || t.title
        WHERE btrim(a.name) <> '' AND btrim(t.title) <> '' AND lower(a.name) <> 'unknown artist'
    """,
    # Last.fm matches songs; a release counts once any of its songs does.
    (LASTFM, "releases"): f"""
        SELECT {_LOOKED_COUNTS}
        FROM (
            SELECT bool_or({_ROW}) AS looked, bool_or({_ROW_FOUND}) AS found, bool_and({_ROW_FRESH}) AS fresh
            FROM tracks t
            JOIN artists a ON a.id = t.artist_id
            LEFT JOIN outside_genres o ON o.source = 'lastfm_track' AND o.ref = a.name || chr(31) || t.title
            WHERE t.release_id IS NOT NULL
              AND btrim(a.name) <> '' AND btrim(t.title) <> '' AND lower(a.name) <> 'unknown artist'
            GROUP BY t.release_id
        ) x
    """,
}


def _match(conn, source: str, level: str, max_age: int) -> dict:
    row = conn.execute(_MATCH_SQL[(source, level)], {"max_age": max_age}).fetchone()
    return dict(zip(("covered", "looked_up", "found", "not_found", "due"), (int(v or 0) for v in row)))


@lru_cache(maxsize=1)
def _catalog():
    import genre_catalog

    try:
        return genre_catalog.catalog()
    except Exception as exc:  # a broken catalog must not break the page
        log.warning("outside sources: genre catalog unavailable: %s", exc)
        return None


def _translate(cat, source: str, name: str, kind: str):
    if source == DISCOGS:
        return cat.discogs_style(name) if kind == "style" else cat.discogs_genre(name)
    if source == LASTFM:
        return cat.lastfm_tag(name)
    return cat.musicbrainz_genre(name)


def _describe(cat, t) -> Optional[str]:
    """A translation as the page shows it: "Electronic / Techno", or a fence."""
    if t is None:
        return None
    if t.genre:
        g = cat.genres.get(t.genre)
        text = g.name if g else t.genre
        if t.style and t.style in cat.styles:
            text += f" / {cat.styles[t.style].name}"
        return text
    names = sorted(cat.genres[g].name if g in cat.genres else g for g in t.fence)
    return "one of " + ", ".join(names)


def genre_yield(conn, source: str) -> dict:
    """How many found rows carry genres, and how many raw values the catalog translates."""
    rows = ROW_SOURCES[source]
    found_rows, with_genres = conn.execute(
        """
        SELECT count(*), count(*) FILTER (WHERE jsonb_array_length(genres) > 0)
        FROM outside_genres WHERE source = ANY(%s) AND found
        """,
        (list(rows),),
    ).fetchone()
    values = conn.execute(
        """
        SELECT g->>'name', coalesce(g->>'kind', 'genre'), count(*)
        FROM outside_genres o, jsonb_array_elements(o.genres) g
        WHERE o.source = ANY(%s) AND o.found AND coalesce(g->>'name', '') <> ''
        GROUP BY 1, 2 ORDER BY 3 DESC, 1, 2
        """,
        (list(rows),),
    ).fetchall()
    cat = _catalog()
    top, untranslated = [], []
    translated = 0
    for name, kind, count in values:
        to = _describe(cat, _translate(cat, source, name, kind)) if cat else None
        item = {"name": name, "kind": kind, "rows": int(count), "translated_to": to}
        if to is not None:
            translated += 1
        elif len(untranslated) < TOP_UNTRANSLATED:
            untranslated.append(item)
        if len(top) < TOP_VALUES:
            top.append(item)
    return {
        "found_rows": int(found_rows),
        "with_genres": int(with_genres),
        "values": len(values),
        "translated_values": translated if cat else None,
        "untranslated_values": len(values) - translated if cat else None,
        "top": top,
        "untranslated": untranslated,
    }


def _queue_state(conn) -> Dict[str, set]:
    rows = conn.execute(
        """
        SELECT args->>'source', status FROM task_queue
        WHERE task = 'fetch_outside_genres' AND status IN ('pending', 'running')
        """
    ).fetchall()
    out: Dict[str, set] = {s: set() for s in SOURCES}
    for source, status in rows:
        if source in out:
            out[source].add(status)
    return out


def _last_refresh(conn, source: str):
    return conn.execute(
        "SELECT max(fetched_at) FROM outside_genres WHERE source = ANY(%s) AND fetched_at > 'epoch'",
        (list(ROW_SOURCES[source]),),
    ).fetchone()[0]


def state(s: SourceSettings) -> str:
    """on, off, missing_key or refused_key."""
    if not s.enabled:
        return "off"
    if s.needs_key and not s.key:
        return "missing_key"
    refused = (s.last_run or {}).get("status") == "refused_key"
    # A key saved or removed after the refused run has not been refused yet.
    if refused and (s.key_updated_at is None or s.last_run_at is None or s.last_run_at > s.key_updated_at):
        return "refused_key"
    return "on"


def _key_status(s: SourceSettings) -> Optional[dict]:
    if not s.needs_key:
        return None
    return {
        "set": bool(s.key),
        "source": s.key_from,
        "last4": s.key_last4 or None,
        "unreadable": s.key_unreadable,
        "env_var": KEY_ENV[s.source],
    }


def status(conn, max_age: int) -> dict:
    """Everything the admin page shows. Never includes a secret."""
    songs, releases = conn.execute("SELECT (SELECT count(*) FROM tracks), (SELECT count(*) FROM releases)").fetchone()
    queue = _queue_state(conn)
    out = []
    for source, s in all_settings(conn).items():
        out.append({
            "source": source,
            "name": NAMES[source],
            "state": state(s),
            "enabled": s.enabled,
            "enabled_from": s.enabled_from,
            "env_switch": SWITCH_ENV[source],
            "key": _key_status(s),
            "running": "running" in queue[source],
            "queued": "pending" in queue[source],
            "requests_per_run": REQUESTS_PER_RUN[source],
            "last_run": None if s.last_run is None else {**_last_run(s.last_run), "at": s.last_run_at},
            "last_refresh": _last_refresh(conn, source),
            "songs": _match(conn, source, "songs", max_age),
            "releases": _match(conn, source, "releases", max_age),
            "genre_yield": genre_yield(conn, source),
        })
    return {
        "library": {"songs": int(songs), "releases": int(releases)},
        "max_age_days": max_age,
        "sources": out,
    }


def _last_run(run: dict) -> dict:
    return {
        "status": str(run.get("status") or "ok"),
        "requests": int(run.get("requests") or 0),
        "retries": int(run.get("retries") or 0),
        "stored": int(run.get("stored") or 0),
        "failures": int(run.get("failures") or 0),
        "more": bool(run.get("more")),
    }
