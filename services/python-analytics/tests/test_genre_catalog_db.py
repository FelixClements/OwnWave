"""The genre v2 switch migration on a database in its v1 state."""

import tomllib

import psycopg

import genre_catalog as gc
import genre_scoring


def _apply(conn, sql):
    conn.execute(sql)


def _v1_state(conn):
    """A v1 database: the v1 catalog and user data that stores v1 genre ids."""
    genre_scoring.sync_catalog(conn)

    def station(name, genres=(), auto=None, active=False, created="2026-01-01"):
        return conn.execute(
            """
            INSERT INTO stations (name, genres, auto_genre, auto_active, created_at)
            VALUES (%s, %s, %s, %s, %s) RETURNING id
            """,
            (name, list(genres), auto, active, created),
        ).fetchone()[0]

    conn.execute("INSERT INTO genre_corrections (artist, genre) VALUES ('Someone', 'dance')")
    conn.execute("INSERT INTO station_suggestion_dismissals (pool_key, genre) VALUES ('p1', 'techno')")
    return station


def test_switch_migration_builds_the_catalog_and_keeps_user_data(db_url):
    up, down = gc.switch_migration(gc.catalog())
    with psycopg.connect(db_url, autocommit=True) as conn:
        station = _v1_state(conn)
        mixed = station("Mixed", ["techno", "house", "gone"])
        everything = station("Everything")
        techno = station("Techno", auto="techno", active=True)

        _apply(conn, up)

        rows = {r[0]: r[1:] for r in conn.execute("SELECT id, kind, parent_id, is_station FROM station_genres")}
        assert rows["house"] == ("genre", None, True)
        assert rows["deep_house"] == ("style", "house", True)
        assert rows["hardcore_punk"] == ("style", "punk", True)
        assert rows["other"] == ("genre", None, False)
        assert rows["comedy"] == ("style", "other", False)
        assert len(rows) == len(gc.catalog().genres) + len(gc.catalog().styles)

        def genres_of(sid):
            return conn.execute("SELECT genres, auto_genre FROM stations WHERE id = %s", (sid,)).fetchone()

        # The draft keeps the v1 ids, so user data is unchanged.
        assert genres_of(mixed) == (["gone", "house", "techno"], None)
        assert genres_of(everything) == ([], None)
        assert genres_of(techno) == ([], "techno")
        assert conn.execute("SELECT genre FROM genre_corrections").fetchall() == [("dance",)]
        assert conn.execute("SELECT COUNT(*) FROM station_suggestion_dismissals").fetchone()[0] == 0

        # The analytics service keeps the new catalog in sync.
        conn.execute("UPDATE station_genres SET name = 'Old' WHERE id = 'deep_house'")
        conn.execute("INSERT INTO station_genres (id, name, position, kind, parent_id) VALUES ('stale', 'x', 0, 'style', 'house')")
        gc.sync_catalog(conn)
        assert conn.execute("SELECT name FROM station_genres WHERE id = 'deep_house'").fetchone() == ("Deep House",)
        assert conn.execute("SELECT COUNT(*) FROM station_genres WHERE id = 'stale'").fetchone() == (0,)

        # A style needs a genre.
        try:
            conn.execute("INSERT INTO station_genres (id, name, position, kind) VALUES ('orphan', 'x', 0, 'style')")
            raise AssertionError("a style without a genre was accepted")
        except psycopg.errors.CheckViolation:
            pass

        _apply(conn, down)
        cols = {r[0] for r in conn.execute("SELECT column_name FROM information_schema.columns WHERE table_name = 'station_genres'")}
        assert cols == {"id", "name", "position", "is_station"}
        assert genres_of(techno) == ([], "techno")
        genre_scoring.sync_catalog(conn)  # the v1 service still works after the down migration


MERGING = """
version = "test"
[[genre]]
id = "techno"
name = "Techno"
replaces = ["house"]
[[genre]]
id = "house"
name = "House"
replaces = ["techno"]
[[genre]]
id = "club"
name = "Club"
replaces = ["dance", "synth"]
styles = ["Nu-Disco"]
[[genre]]
id = "rest"
name = "Rest"
replaces = ["trance", "drum_and_bass", "bass", "hard_dance", "ambient", "experimental", "hip_hop", "soul", "pop", "rock", "indie", "metal", "punk", "jazz", "blues", "classical", "folk", "latin", "reggae", "world", "soundtrack", "other"]
"""


def test_switch_migration_moves_ids_and_keeps_one_station_per_genre(db_url):
    up, down = gc.switch_migration(gc.Catalog(tomllib.loads(MERGING)))
    with psycopg.connect(db_url, autocommit=True) as conn:
        station = _v1_state(conn)
        filtered = station("Filtered", ["dance", "synth", "techno"])
        techno = station("Techno", auto="techno", active=True)
        house = station("House", auto="house", active=True)
        dance = station("Dance", auto="dance", active=False, created="2025-01-01")
        synth = station("Synth", auto="synth", active=True, created="2026-06-01")

        _apply(conn, up)

        def auto(sid):
            row = conn.execute("SELECT auto_genre FROM stations WHERE id = %s", (sid,)).fetchone()
            return row[0] if row else "deleted"

        # Swapped ids do not trip the one-station-per-genre index.
        assert auto(techno) == "house"
        assert auto(house) == "techno"
        # Two old genres became one: the active station keeps it.
        assert auto(synth) == "club"
        assert auto(dance) == "deleted"
        assert conn.execute("SELECT genres FROM stations WHERE id = %s", (filtered,)).fetchone() == (["club", "house"],)
        assert conn.execute("SELECT genre FROM genre_corrections").fetchall() == [("club",)]
        assert conn.execute("SELECT parent_id FROM station_genres WHERE id = 'nu_disco'").fetchone() == ("club",)

        _apply(conn, down)
        assert auto(techno) == "techno"
        assert auto(synth) == "dance"
        assert conn.execute("SELECT genres FROM stations WHERE id = %s", (filtered,)).fetchone() == (["dance", "techno"],)
