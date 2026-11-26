"""Genre catalog v2: genres, styles, and what each genre source says.

ADR 0008 replaces the flat list of station genres (genre_taxonomy.py) with
two levels: every song gets one genre (House) and at most one style (Deep
House). Styles are Discogs styles; a genre is a curated group of styles.

The list itself lives in genre_catalog.toml, so it can be edited and
approved (issue #138) without touching code. This module reads it, checks
it, and translates every source vocabulary into a Translation:

- Discogs400 labels ("Electronic---Deep House") and Discogs styles
- Discogs genres ("Electronic"), which only fence
- MusicBrainz genres, Last.fm tags and file genre tags (free text)

A Translation of a specific value names one genre (and maybe a style). A
broad value (a broad tag such as "Pop/Rock") names no genre and only fences
the song to a set of genres. Which sources are strong or weak is the
decision engine's business, not this module's.

switch_migration() generated the genre v2 switch, migration
038_genre_v2_switch (the catalog plus the old -> new genre id mapping), at
the release. That migration is frozen; the analytics service writes later
catalog edits to station_genres at startup (sync_catalog). Renaming or
removing an id that stations or corrections store needs its own migration.
"""

from __future__ import annotations

import re
import tomllib
import unicodedata
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path
from typing import Dict, FrozenSet, Iterable, List, Optional, Sequence, Tuple

CATALOG_FILE = Path(__file__).with_name("genre_catalog.toml")

# Same rule as genreIDPattern in services/go-api-server/internal/stations/station.go.
ID_PATTERN = re.compile(r"^[a-z][a-z0-9_]{0,39}$")

# The v1 station genre ids (genre_taxonomy.STATION_GENRES). Stations, genre
# corrections and dismissed suggestions store these; the switch migration
# moves each to the genre whose `replaces` lists it.
V1_GENRE_IDS: Tuple[str, ...] = (
    "techno", "house", "trance", "drum_and_bass", "bass", "hard_dance", "dance", "synth",
    "ambient", "experimental", "hip_hop", "soul", "pop", "rock", "indie", "metal", "punk",
    "jazz", "blues", "classical", "folk", "latin", "reggae", "world", "soundtrack", "other",
)

_TAG_SPLIT = re.compile(r"\s*(?:;|,|/|\\|\|| - )\s*")


class CatalogError(ValueError):
    """genre_catalog.toml is inconsistent."""


@dataclass(frozen=True)
class Genre:
    id: str
    name: str
    station: bool
    replaces: Tuple[str, ...]
    styles: Tuple[str, ...]  # style ids, in display order


@dataclass(frozen=True)
class Style:
    id: str
    name: str
    genre: str


@dataclass(frozen=True)
class Translation:
    """What one source value says about a song's genre.

    Specific: `genre` is set (and `style` when the value names one), and
    the fence is that genre alone. Broad: `genre` is None and `fence` holds
    the genres the song may still get.
    """

    genre: Optional[str]
    style: Optional[str]
    fence: FrozenSet[str]

    @property
    def broad(self) -> bool:
        return self.genre is None

    @staticmethod
    def of_genre(genre: str, style: Optional[str] = None) -> "Translation":
        return Translation(genre, style, frozenset({genre}))

    @staticmethod
    def of_fence(genres: Iterable[str]) -> "Translation":
        return Translation(None, None, frozenset(genres))


def fold(text: str) -> str:
    """Comparable form of a genre text: "Hip-Hop" -> "hip hop", "R&B" -> "r and b"."""
    t = unicodedata.normalize("NFKD", text or "")
    t = "".join(c for c in t if not unicodedata.combining(c)).lower().replace("&", " and ")
    return re.sub(r"[^a-z0-9]+", " ", t).strip()


def slug(name: str) -> str:
    """Id for a style name: "Deep House" -> "deep_house"."""
    return fold(name).replace(" ", "_")


def split_tag(raw: str) -> List[str]:
    """Split a raw file genre tag such as "Techno; Hard Techno" into its parts."""
    return [p for p in (s.strip() for s in _TAG_SPLIT.split(raw or "")) if p]


class Catalog:
    """The genre/style list and the translation tables, checked on load."""

    def __init__(self, data: dict, v1_ids: Sequence[str] = V1_GENRE_IDS):
        self.version: str = str(data.get("version", ""))
        self.genres: Dict[str, Genre] = {}
        self.styles: Dict[str, Style] = {}
        self._discogs_plain: Dict[str, Translation] = {}
        self._discogs_qualified: Dict[Tuple[str, str], Translation] = {}
        self._load_genres(data.get("genre") or [])
        self._discogs_ambiguous = self._ambiguous_discogs_names()
        self._discogs_genres = {
            name: Translation.of_fence(self._genre_ids(genres, f"discogs_genres.{name}"))
            for name, genres in (data.get("discogs_genres") or {}).items()
        }
        text = data.get("text") or {}
        self._ignore = frozenset(fold(t) for t in text.get("ignore") or [])
        self._lastfm_ignore = frozenset(fold(t) for t in (data.get("lastfm") or {}).get("ignore") or [])
        self._broad = {
            fold(tag): Translation.of_fence(self._genre_ids(genres, f"text.broad.{tag}"))
            for tag, genres in (text.get("broad") or {}).items()
        }
        self._aliases = {fold(k): self._resolve(v, f"text.aliases.{k}") for k, v in (text.get("aliases") or {}).items()}
        self._names = self._name_index()
        self._rules = []
        for rule in text.get("rule") or []:
            genre = rule.get("genre")
            if genre not in self.genres:
                raise CatalogError(f"text.rule {rule.get('pattern')!r}: unknown genre {genre!r}")
            self._rules.append((re.compile(rule["pattern"]), Translation.of_genre(genre)))
        self._check_replaces(v1_ids)

    # --- loading -----------------------------------------------------------------

    def _load_genres(self, entries: list) -> None:
        for entry in entries:
            gid = entry.get("id", "")
            if not ID_PATTERN.match(gid):
                raise CatalogError(f"bad genre id {gid!r}")
            if gid in self.genres:
                raise CatalogError(f"genre {gid!r} listed twice")
            self.genres[gid] = Genre(gid, entry["name"], bool(entry.get("station", True)), tuple(entry.get("replaces") or []), ())
        for entry in entries:
            gid = entry["id"]
            whole = Translation.of_genre(gid)
            for name in entry.get("discogs") or []:
                self._add_discogs(name, whole, f"genre {gid}")
            style_ids = []
            for raw in entry.get("styles") or []:
                if isinstance(raw, str):
                    name, discogs = raw, [raw]
                else:
                    name, discogs = raw["name"], raw.get("discogs") or [raw["name"]]
                sid = slug(name)
                if not ID_PATTERN.match(sid):
                    raise CatalogError(f"style {name!r} makes a bad id {sid!r}")
                if sid in self.genres:
                    raise CatalogError(f"style {name!r} has the id of a genre; list it under that genre's `discogs`")
                if sid in self.styles:
                    raise CatalogError(f"style {name!r} listed twice ({self.styles[sid].genre} and {gid})")
                self.styles[sid] = Style(sid, name, gid)
                style_ids.append(sid)
                for d in discogs:
                    self._add_discogs(d, Translation.of_genre(gid, sid), f"style {name}")
            g = self.genres[gid]
            self.genres[gid] = Genre(g.id, g.name, g.station, g.replaces, tuple(style_ids))

    def _add_discogs(self, discogs: str, t: Translation, where: str) -> None:
        parent, sep, name = discogs.partition("---")
        if sep:
            key = (parent, name)
            if key in self._discogs_qualified:
                raise CatalogError(f"Discogs style {discogs!r} listed twice ({where})")
            self._discogs_qualified[key] = t
        else:
            if discogs in self._discogs_plain:
                raise CatalogError(f"Discogs style {discogs!r} listed twice ({where})")
            self._discogs_plain[discogs] = t

    def _ambiguous_discogs_names(self) -> Dict[str, Translation]:
        """Discogs style names only listed with their Discogs genre: without
        it they fence to every genre they could mean."""
        out: Dict[str, set] = {}
        for (_, name), t in self._discogs_qualified.items():
            if name not in self._discogs_plain:
                out.setdefault(name, set()).add(t.genre)
        return {name: Translation.of_fence(genres) for name, genres in out.items()}

    def _genre_ids(self, ids: Iterable[str], where: str) -> List[str]:
        ids = list(ids)
        unknown = [g for g in ids if g not in self.genres]
        if unknown or not ids:
            raise CatalogError(f"{where}: unknown or no genres {unknown}")
        return ids

    def _resolve(self, target: str, where: str) -> Translation:
        """An alias target: a style name or a genre id."""
        for s in self.styles.values():
            if s.name == target:
                return Translation.of_genre(s.genre, s.id)
        if target in self.genres:
            return Translation.of_genre(target)
        raise CatalogError(f"{where}: {target!r} is neither a style name nor a genre id")

    def _name_index(self) -> Dict[str, Translation]:
        index: Dict[str, Translation] = {}

        def add(text: str, t: Translation) -> None:
            key = fold(text)
            if key in index and index[key] != t:
                raise CatalogError(f"text {text!r} means both {index[key]} and {t}")
            index[key] = t

        for name, t in self._discogs_plain.items():
            add(name, t)
        for s in self.styles.values():
            add(s.name, Translation.of_genre(s.genre, s.id))
        for g in self.genres.values():
            add(g.id.replace("_", " "), Translation.of_genre(g.id))
            add(g.name, Translation.of_genre(g.id))
        return index

    def _check_replaces(self, v1_ids: Sequence[str]) -> None:
        seen: Dict[str, str] = {}
        for g in self.genres.values():
            for old in g.replaces:
                if old in seen:
                    raise CatalogError(f"old genre {old!r} replaced by both {seen[old]} and {g.id}")
                seen[old] = g.id
        missing = [old for old in v1_ids if old not in seen]
        if missing:
            raise CatalogError(f"no genre replaces the old genres {missing}")

    # --- translations ---------------------------------------------------------------

    def discogs_style(self, name: str, discogs_genres: Iterable[str] = ()) -> Optional[Translation]:
        """A Discogs style, e.g. from a Discogs release. Its Discogs genres
        settle names that mean different things under different genres."""
        for parent in discogs_genres:
            t = self._discogs_qualified.get((parent, name))
            if t is not None:
                return t
        return self._discogs_plain.get(name) or self._discogs_ambiguous.get(name)

    def discogs_genre(self, name: str) -> Optional[Translation]:
        """A Discogs genre such as "Electronic": a fence, never a genre."""
        return self._discogs_genres.get(name)

    def discogs400_label(self, label: str) -> Optional[Translation]:
        """A Discogs400 audio model label such as "Electronic---Techno"."""
        parent, sep, name = label.partition("---")
        if not sep:
            return self.discogs_genre(label)
        t = self.discogs_style(name, (parent,))
        if t is not None and not t.broad:
            return t
        return self.discogs_genre(parent)

    def text(self, text: str, extra_ignore: FrozenSet[str] = frozenset()) -> Optional[Translation]:
        """One free-text genre (one part of a tag, a MusicBrainz genre, a
        Last.fm tag), or None when it says nothing about the genre."""
        key = fold(text)
        if not key or key in self._ignore or key in extra_ignore:
            return None
        found = self._exact(key)
        if found is not None:
            return found
        for pattern, t in self._rules:
            if pattern.search(key):
                return t
        return None

    def _exact(self, key: str) -> Optional[Translation]:
        return self._broad.get(key) or self._aliases.get(key) or self._names.get(key)

    def musicbrainz_genre(self, name: str) -> Optional[Translation]:
        """A MusicBrainz recording, release or artist genre such as "deep house"."""
        return self.text(name)

    def lastfm_tag(self, tag: str) -> Optional[Translation]:
        """A Last.fm tag such as "psytrance" or "seen live"."""
        return self.text(tag, self._lastfm_ignore)

    def genre_tag(self, raw: str) -> List[Translation]:
        """A file genre tag, which may hold several genres ("Techno; Hard Techno").

        A tag known as a whole ("Pop/Rock", "Hip-Hop/Rap") is not split.
        """
        key = fold(raw)
        if not key or key in self._ignore:
            return []
        whole = self._exact(key)
        if whole is not None:
            return [whole]
        out: List[Translation] = []
        for part in split_tag(raw):
            t = self.text(part)
            if t is not None and t not in out:
                out.append(t)
        return out

    # --- the switch ------------------------------------------------------------------

    def id_map(self) -> Dict[str, str]:
        """Old (v1) genre id -> new genre id."""
        return {old: g.id for g in self.genres.values() for old in g.replaces}


def load(path: Path = CATALOG_FILE, v1_ids: Sequence[str] = V1_GENRE_IDS) -> Catalog:
    return Catalog(tomllib.loads(path.read_text(encoding="utf-8")), v1_ids)


@lru_cache(maxsize=1)
def catalog() -> Catalog:
    """The catalog from genre_catalog.toml."""
    return load()


# --- database ----------------------------------------------------------------------


def catalog_rows(cat: Catalog) -> List[tuple]:
    """station_genres rows (id, name, position, is_station, kind, parent_id):
    genres first, then styles; position orders genres, and styles within their genre."""
    rows = [(g.id, g.name, i, g.station, "genre", None) for i, g in enumerate(cat.genres.values())]
    for g in cat.genres.values():
        rows.extend((sid, cat.styles[sid].name, i, g.station, "style", g.id) for i, sid in enumerate(g.styles))
    return rows


def sync_catalog(conn, cat: Optional[Catalog] = None) -> None:
    """Write genres and styles to station_genres (needs the switch migration).
    The caller commits."""
    cat = cat or catalog()
    rows = catalog_rows(cat)
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO station_genres (id, name, position, is_station, kind, parent_id)
            VALUES (%s, %s, %s, %s, %s, %s)
            ON CONFLICT (id) DO UPDATE SET
                name = EXCLUDED.name, position = EXCLUDED.position, is_station = EXCLUDED.is_station,
                kind = EXCLUDED.kind, parent_id = EXCLUDED.parent_id
            """,
            rows,
        )
        cur.execute("DELETE FROM station_genres WHERE id <> ALL(%s)", ([r[0] for r in rows],))


def _q(value) -> str:
    if value is None:
        return "NULL"
    if isinstance(value, bool):
        return "TRUE" if value else "FALSE"
    if isinstance(value, int):
        return str(value)
    return "'" + str(value).replace("'", "''") + "'"


def _values(rows: Sequence[tuple], indent: str = "    ") -> str:
    return ",\n".join(indent + "(" + ", ".join(_q(v) for v in row) + ")" for row in rows)


def _remap_sql(pairs: Sequence[Tuple[str, str]]) -> str:
    """Move every stored genre id through the mapping `pairs` (from, to)."""
    return f"""CREATE TEMP TABLE genre_id_map (old_id TEXT PRIMARY KEY, new_id TEXT NOT NULL);
INSERT INTO genre_id_map (old_id, new_id) VALUES
{_values(pairs)};

-- Playlist-station genre filters (user data). Unknown ids are kept as they are.
UPDATE stations s SET genres = ARRAY(
    SELECT DISTINCT COALESCE(m.new_id, g)
    FROM unnest(s.genres) AS g LEFT JOIN genre_id_map m ON m.old_id = g
    ORDER BY 1
)
WHERE s.genres <> '{{}}';

-- Genre stations: one per genre. When several old genres move to one, the
-- active, then oldest, station keeps it; the others are deleted.
DELETE FROM stations s USING (
    SELECT s2.id, ROW_NUMBER() OVER (
        PARTITION BY m.new_id ORDER BY s2.auto_active DESC, s2.created_at, s2.id
    ) AS n
    FROM stations s2 JOIN genre_id_map m ON m.old_id = s2.auto_genre
) dup
WHERE s.id = dup.id AND dup.n > 1;
-- Two steps, so swapping two ids never trips the one-station-per-genre index.
UPDATE stations s SET auto_genre = '~' || m.new_id
FROM genre_id_map m WHERE s.auto_genre = m.old_id AND m.new_id <> m.old_id;
UPDATE stations SET auto_genre = substr(auto_genre, 2) WHERE auto_genre LIKE '~%';

-- Admin corrections (user data).
UPDATE genre_corrections c SET genre = m.new_id
FROM genre_id_map m WHERE c.genre = m.old_id AND m.new_id <> m.old_id;

-- Dismissed station suggestions are cleared: they name old genres.
DELETE FROM station_suggestion_dismissals;

DROP TABLE genre_id_map;
"""


def switch_migration(cat: Catalog) -> Tuple[str, str]:
    """(up, down) SQL of the genre v2 switch for this catalog."""
    header = f"""-- GENERATED from services/python-analytics/src/genre_catalog.toml
-- (catalog version {cat.version}) by genre_catalog.py at the genre v2 release.
-- Frozen: do not edit or regenerate it. Later catalog edits reach
-- station_genres through genre_catalog.sync_catalog at startup.
--
-- Genre engine v2 switch (docs/adr/0008-genre-engine-v2.md): the catalog
-- holds genres and styles, and stored v1 genre ids move to the new genres.
"""
    up = (
        header
        + f"""
ALTER TABLE station_genres
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'genre',
    ADD COLUMN parent_id TEXT REFERENCES station_genres(id) ON DELETE CASCADE,
    ADD CONSTRAINT station_genres_kind CHECK (kind IN ('genre', 'style')),
    -- A style belongs to exactly one genre; a genre to none.
    ADD CONSTRAINT station_genres_parent CHECK ((kind = 'style') = (parent_id IS NOT NULL));
CREATE INDEX idx_station_genres_parent ON station_genres (parent_id) WHERE parent_id IS NOT NULL;

{_remap_sql(sorted(cat.id_map().items()))}
-- The catalog: genres, then their styles (the analytics service keeps it in sync).
DELETE FROM station_genres;
INSERT INTO station_genres (id, name, position, is_station, kind, parent_id) VALUES
{_values(catalog_rows(cat))};
"""
    )
    # Back to v1 ids: a genre goes to the first old genre it replaced, a style
    # to its genre's; a genre that replaced none goes to "other".
    back: Dict[str, str] = {}
    for g in cat.genres.values():
        back[g.id] = g.replaces[0] if g.replaces else "other"
        for sid in g.styles:
            back[sid] = back[g.id]
    down = (
        header
        + f"""
{_remap_sql(sorted(back.items()))}
-- The v1 analytics service refills the catalog at startup.
DELETE FROM station_genres;
DROP INDEX IF EXISTS idx_station_genres_parent;
ALTER TABLE station_genres
    DROP CONSTRAINT station_genres_parent,
    DROP CONSTRAINT station_genres_kind,
    DROP COLUMN parent_id,
    DROP COLUMN kind;
"""
    )
    return up, down
