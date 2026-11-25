"""Admin Outside sources page (issue #148): keys saved on the page override
env, are encrypted at rest and never come back out; status and match stats.
HTTP is faked; no test calls MusicBrainz, Discogs or Last.fm, and every key
here is made up."""

import asyncio
import io
import json
import urllib.error
import urllib.parse

import psycopg
import pytest
from fastapi import HTTPException
from fastapi.exceptions import RequestValidationError
from psycopg.types.json import Jsonb

import api
import outside_genres as og
import outside_sources as osrc
import secret_box
import tasks

PAGE_KEY = "fake-page-key-0000000000000000wxyz"
ENV_KEY = "fake-env-key-11111111111111111abcd"
OTHER_KEY = "fake-other-key-2222222222222222lmno"


@pytest.fixture
def env(monkeypatch):
    monkeypatch.setattr("config.ANALYTICS_API_SECRET", "test-analytics-secret-0123456789")
    monkeypatch.setattr("config.SECRETS_KEY", "")
    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", True)
    monkeypatch.setattr("config.DISCOGS_ENABLED", True)
    monkeypatch.setattr("config.LASTFM_ENABLED", True)
    monkeypatch.setattr("config.DISCOGS_TOKEN", "")
    monkeypatch.setattr("config.LASTFM_API_KEY", ENV_KEY)


@pytest.fixture
def conn(db_url, env):
    with psycopg.connect(db_url) as c:
        yield c


class FakeHttp:
    """Answers by URL path; records every request."""

    def __init__(self, routes=None):
        self.routes = dict(routes or {})
        self.requests = []

    def __call__(self, req, timeout):
        self.requests.append(req)
        path = urllib.parse.urlsplit(req.full_url).path
        answer = self.routes.get(path, 404)
        if isinstance(answer, Exception):
            raise answer
        if isinstance(answer, int):
            body = io.BytesIO(json.dumps({"message": "nope"}).encode())
            raise urllib.error.HTTPError(req.full_url, answer, "error", {}, body)
        return io.BytesIO(json.dumps(answer).encode())


def http_error(code, body):
    return urllib.error.HTTPError("u", code, "error", {}, io.BytesIO(json.dumps(body).encode()))


# --- Encryption ----------------------------------------------------------------


def test_secret_box_round_trip_and_binding(env):
    blob = secret_box.encrypt(PAGE_KEY, "discogs")
    assert PAGE_KEY.encode() not in blob
    assert secret_box.decrypt(blob, "discogs") == PAGE_KEY
    # Bound to its source: not readable as another source's secret.
    with pytest.raises(secret_box.SecretUnreadable):
        secret_box.decrypt(blob, "lastfm")
    # Another key (SECRETS_KEY set, or the analytics secret rotated) cannot read it.
    with pytest.raises(secret_box.SecretUnreadable):
        secret_box.decrypt(blob, "discogs", material="another-key-material-0123")
    with pytest.raises(secret_box.SecretUnreadable):
        secret_box.decrypt(b"\x02" + blob[1:], "discogs")
    # Fresh nonce every time.
    assert secret_box.encrypt(PAGE_KEY, "discogs") != blob


def test_secrets_key_takes_precedence(env, monkeypatch):
    blob = secret_box.encrypt(PAGE_KEY, "discogs")
    monkeypatch.setattr("config.SECRETS_KEY", "dedicated-secrets-key-0123456789")
    with pytest.raises(secret_box.SecretUnreadable):
        secret_box.decrypt(blob, "discogs")
    assert secret_box.decrypt(secret_box.encrypt(PAGE_KEY, "discogs"), "discogs") == PAGE_KEY


def test_last4_hides_short_secrets():
    assert secret_box.last4(PAGE_KEY) == "wxyz"
    assert secret_box.last4("short") == ""


# --- Settings: page over env -----------------------------------------------------


def test_env_decides_without_page_values(conn):
    s = osrc.all_settings(conn)
    assert (s["lastfm"].key, s["lastfm"].key_from, s["lastfm"].key_last4) == (ENV_KEY, "env", "abcd")
    assert s["discogs"].key is None and not s["discogs"].usable
    assert s["musicbrainz"].enabled_from == "env"
    assert osrc.enabled_sources(conn) == ["musicbrainz", "lastfm"]


def test_page_key_overrides_env_and_removing_it_falls_back(conn):
    osrc.save_key(conn, "lastfm", f"  {PAGE_KEY}\n")
    osrc.save_key(conn, "discogs", OTHER_KEY)
    conn.commit()
    s = osrc.all_settings(conn)
    assert (s["lastfm"].key, s["lastfm"].key_from, s["lastfm"].key_last4) == (PAGE_KEY, "page", "wxyz")
    assert s["discogs"].key == OTHER_KEY and s["discogs"].usable
    # Stored encrypted: the database never holds the key itself.
    raw = conn.execute("SELECT secret, secret_last4 FROM outside_sources WHERE source = 'lastfm'").fetchone()
    assert PAGE_KEY.encode() not in bytes(raw[0]) and raw[1] == "wxyz"

    osrc.save_key(conn, "lastfm", None)
    s = osrc.settings(conn, "lastfm")
    assert (s.key, s.key_from) == (ENV_KEY, "env")


def test_page_switch_overrides_env(conn, monkeypatch):
    osrc.save_switch(conn, "musicbrainz", False)
    assert osrc.settings(conn, "musicbrainz").enabled is False
    assert osrc.settings(conn, "musicbrainz").enabled_from == "page"
    monkeypatch.setattr("config.LASTFM_ENABLED", False)
    osrc.save_switch(conn, "lastfm", True)
    assert osrc.enabled_sources(conn) == ["lastfm"]
    osrc.save_switch(conn, "musicbrainz", None)
    assert osrc.settings(conn, "musicbrainz").enabled_from == "env"
    assert osrc.enabled_sources(conn) == ["musicbrainz", "lastfm"]


def test_unreadable_page_key_falls_back_to_env(conn, monkeypatch):
    osrc.save_key(conn, "lastfm", PAGE_KEY)
    monkeypatch.setattr("config.ANALYTICS_API_SECRET", "rotated-analytics-secret-0123456")
    s = osrc.settings(conn, "lastfm")
    assert (s.key, s.key_from, s.key_unreadable) == (ENV_KEY, "env", True)


@pytest.mark.parametrize("bad", ["", "   ", "has space inside", "x" * 201, "tab\there"])
def test_bad_keys_are_refused(conn, bad):
    with pytest.raises(ValueError):
        osrc.save_key(conn, "discogs", bad)


def test_musicbrainz_takes_no_key(conn):
    with pytest.raises(ValueError):
        osrc.save_key(conn, "musicbrainz", PAGE_KEY)
    with pytest.raises(osrc.UnknownSource):
        osrc.save_switch(conn, "spotify", True)


def test_settings_repr_hides_the_key(conn):
    osrc.save_key(conn, "discogs", PAGE_KEY)
    assert PAGE_KEY not in repr(osrc.settings(conn, "discogs"))


def test_task_uses_the_page_key(db_url, env, monkeypatch):
    with psycopg.connect(db_url) as c:
        osrc.save_key(c, "lastfm", PAGE_KEY)
        c.commit()
    used = []

    def client(source, key=None):
        used.append(key)
        return og.LastfmClient(key, "ua", base_url="https://lf.test/2.0/", opener=FakeHttp(), sleep=lambda _: None)

    monkeypatch.setattr(og, "make_client", client)
    tasks.fetch_outside_genres("lastfm")
    assert used == [PAGE_KEY]


def test_genre_refresh_follows_the_page_musicbrainz_switch(db_url, env):
    with psycopg.connect(db_url) as c:
        osrc.save_switch(c, "musicbrainz", False)
        tasks.queue_genre_refresh(c)
        c.commit()
        rows = c.execute("SELECT task, args->>'source' FROM task_queue ORDER BY id").fetchall()
    assert rows == [("score_genres", None), ("fetch_outside_genres", "lastfm")]


# --- Last run and state ------------------------------------------------------------


def test_refused_token_is_recorded_and_cleared_by_a_new_key(db_url, env, monkeypatch):
    with psycopg.connect(db_url) as c:
        osrc.save_key(c, "discogs", PAGE_KEY)
        c.execute(
            "INSERT INTO outside_genres (source, ref, found, discogs_release_id) VALUES ('mb_release', 'r1', true, '555')"
        )
        c.commit()
    http = FakeHttp({"/releases/555": 401})
    monkeypatch.setattr(
        og, "make_client",
        lambda source, key=None: og.DiscogsClient(key, "ua", base_url="https://dc.test", opener=http, sleep=lambda _: None),
    )
    result = tasks.fetch_outside_genres("discogs")
    assert result["status"] == "refused_key"
    assert http.requests[0].get_header("Authorization") == f"Discogs token={PAGE_KEY}"
    with psycopg.connect(db_url) as c:
        s = osrc.settings(c, "discogs")
        assert s.last_run["status"] == "refused_key" and s.last_run["requests"] == 1 and s.last_run["retries"] == 0
        assert osrc.state(s) == "refused_key"
        osrc.save_key(c, "discogs", OTHER_KEY)
        c.commit()
        assert osrc.state(osrc.settings(c, "discogs")) == "on"


def test_states(conn, monkeypatch):
    assert osrc.state(osrc.settings(conn, "discogs")) == "missing_key"
    osrc.save_switch(conn, "lastfm", False)
    assert osrc.state(osrc.settings(conn, "lastfm")) == "off"
    assert osrc.state(osrc.settings(conn, "musicbrainz")) == "on"


# --- Status and match stats ----------------------------------------------------------

R1 = "11111111-1111-1111-1111-111111111111"
R2 = "22222222-2222-2222-2222-222222222222"
REC1 = "aaaaaaaa-0000-0000-0000-000000000001"
REC2 = "aaaaaaaa-0000-0000-0000-000000000002"


def add_library(c):
    artist = c.execute("INSERT INTO artists (name) VALUES ('Amelie Lens') RETURNING id").fetchone()[0]
    r1 = c.execute("INSERT INTO releases (mb_release_id, folder, title) VALUES (%s, '/m/1', 'One') RETURNING id", (R1,)).fetchone()[0]
    r2 = c.execute("INSERT INTO releases (mb_release_id, folder, title) VALUES (%s, '/m/2', 'Two') RETURNING id", (R2,)).fetchone()[0]
    r3 = c.execute("INSERT INTO releases (folder, title) VALUES ('/m/3', 'Three') RETURNING id").fetchone()[0]
    for path, title, rel, rec in [
        ("/m/1/a.flac", "A", r1, REC1),
        ("/m/1/b.flac", "B", r1, None),
        ("/m/2/c.flac", "C", r2, REC2),
        ("/m/3/d.flac", "D", r3, None),
    ]:
        c.execute(
            "INSERT INTO tracks (path, title, artist_id, release_id, mb_recording_id) VALUES (%s, %s, %s, %s, %s)",
            (path, title, artist, rel, rec),
        )

    def row(source, ref, found, genres=(), discogs=None, old=False):
        c.execute(
            "INSERT INTO outside_genres (source, ref, found, genres, discogs_release_id, fetched_at)"
            " VALUES (%s, %s, %s, %s, %s, CASE WHEN %s THEN 'epoch'::timestamptz ELSE NOW() END)",
            (source, ref, found, Jsonb(list(genres)), discogs, old),
        )

    techno = {"name": "techno", "count": 3, "kind": "genre"}
    row("mb_release", R1, True, [techno], discogs="555")
    row("mb_release", R2, False, old=True)  # not found, and due again
    row("mb_recording", REC1, True, [techno, {"name": "zzz-not-a-genre", "count": 1, "kind": "genre"}])
    row("discogs_release", "555", True, [
        {"name": "Electronic", "count": None, "kind": "genre"},
        {"name": "Techno", "count": None, "kind": "style"},
    ])
    row("lastfm_track", "Amelie Lens\x1fA", True, [{"name": "seen live", "count": 100, "kind": "tag"}])
    row("lastfm_track", "Amelie Lens\x1fB", False)
    c.commit()


def by_source(status):
    return {s["source"]: s for s in status["sources"]}


def test_status_counts_coverage_matches_and_yield(conn):
    add_library(conn)
    status = osrc.status(conn, max_age=180)
    assert status["library"] == {"songs": 4, "releases": 3}
    s = by_source(status)

    mb = s["musicbrainz"]
    assert mb["releases"] == {"covered": 2, "looked_up": 2, "found": 1, "not_found": 1, "due": 1}
    # A: recording found. B: no recording id, through its release (found).
    # C: recording not looked up, its release not found and old: due.
    assert mb["songs"] == {"covered": 3, "looked_up": 3, "found": 2, "not_found": 1, "due": 1}
    assert mb["genre_yield"]["found_rows"] == 2 and mb["genre_yield"]["with_genres"] == 2
    top = {v["name"]: v for v in mb["genre_yield"]["top"]}
    assert top["techno"]["rows"] == 2 and top["techno"]["translated_to"]
    assert [v["name"] for v in mb["genre_yield"]["untranslated"]] == ["zzz-not-a-genre"]
    assert mb["genre_yield"]["translated_values"] == 1 and mb["genre_yield"]["untranslated_values"] == 1

    dc = s["discogs"]
    assert dc["releases"] == {"covered": 1, "looked_up": 1, "found": 1, "not_found": 0, "due": 0}
    assert dc["songs"]["covered"] == 2 and dc["songs"]["found"] == 2
    assert dc["state"] == "missing_key" and dc["key"]["set"] is False
    assert dc["genre_yield"]["translated_values"] == 2

    lf = s["lastfm"]
    assert lf["songs"] == {"covered": 4, "looked_up": 2, "found": 1, "not_found": 1, "due": 2}
    assert lf["releases"] == {"covered": 3, "looked_up": 1, "found": 1, "not_found": 0, "due": 2}
    assert lf["genre_yield"]["untranslated"][0]["name"] == "seen live"
    assert lf["last_refresh"] is not None and lf["last_run"] is None


def test_status_never_returns_a_secret(conn, monkeypatch):
    monkeypatch.setattr("config.DISCOGS_TOKEN", OTHER_KEY)
    osrc.save_key(conn, "lastfm", PAGE_KEY)
    conn.commit()
    status = api.OutsideSources.model_validate(osrc.status(conn, 180)).model_dump_json()
    for secret in (PAGE_KEY, ENV_KEY, OTHER_KEY):
        assert secret not in status
        assert secret[:-4] not in status
    s = by_source(json.loads(status))
    assert s["lastfm"]["key"] == {"set": True, "source": "page", "last4": "wxyz", "unreadable": False, "env_var": "LASTFM_API_KEY"}
    assert s["discogs"]["key"]["source"] == "env" and s["discogs"]["key"]["last4"] == "lmno"
    assert s["musicbrainz"]["key"] is None


def test_status_shows_running_and_queued(conn):
    conn.execute("INSERT INTO task_queue (task, args, status) VALUES ('fetch_outside_genres', '{\"source\": \"musicbrainz\"}', 'running')")
    conn.execute("INSERT INTO task_queue (task, args) VALUES ('fetch_outside_genres', '{\"source\": \"lastfm\"}')")
    s = by_source(osrc.status(conn, 180))
    assert (s["musicbrainz"]["running"], s["musicbrainz"]["queued"]) == (True, False)
    assert (s["lastfm"]["running"], s["lastfm"]["queued"]) == (False, True)


# --- Test button ---------------------------------------------------------------------


def test_check_musicbrainz(conn, monkeypatch):
    monkeypatch.setattr("config.MUSICBRAINZ_URL", "https://mb.test")
    http = FakeHttp({"/ws/2/genre/all": {"genres": [{"name": "acid"}]}})
    assert osrc.check_connection(conn, "musicbrainz", opener=http) == {"ok": True, "message": "MusicBrainz answered."}
    assert len(http.requests) == 1


def test_check_discogs_with_a_given_key_saves_nothing(conn, monkeypatch):
    http = FakeHttp({"/oauth/identity": {"username": "someone"}})
    result = osrc.check_connection(conn, "discogs", OTHER_KEY, opener=http)
    assert result == {"ok": True, "message": "Discogs accepted the token."}
    assert http.requests[0].get_header("Authorization") == f"Discogs token={OTHER_KEY}"
    assert osrc.settings(conn, "discogs").key is None


def test_check_discogs_refused(conn):
    http = FakeHttp({"/oauth/identity": 401})
    result = osrc.check_connection(conn, "discogs", OTHER_KEY, opener=http)
    assert result == {"ok": False, "message": "Discogs refused the token (HTTP 401)."}


def test_check_without_a_key(conn):
    assert osrc.check_connection(conn, "discogs") == {"ok": False, "message": "No Discogs key is set."}


def test_check_lastfm_bad_key_and_saved_key(conn):
    http = FakeHttp({"/2.0/": http_error(403, {"error": 10, "message": "Invalid API key"})})
    result = osrc.check_connection(conn, "lastfm", opener=http)
    assert result["ok"] is False and result["message"].startswith("Last.fm refused the key")
    # The saved (here env) key was the one sent.
    query = dict(urllib.parse.parse_qsl(urllib.parse.urlsplit(http.requests[0].full_url).query))
    assert query["api_key"] == ENV_KEY and query["method"] == "chart.getTopTags"
    ok = FakeHttp({"/2.0/": {"tags": {"tag": []}}})
    assert osrc.check_connection(conn, "lastfm", opener=ok)["ok"] is True


def test_check_error_messages_never_hold_the_key(conn):
    http = FakeHttp({"/2.0/": urllib.error.URLError(f"could not resolve ...?api_key={OTHER_KEY}")})
    result = osrc.check_connection(conn, "lastfm", OTHER_KEY, opener=http)
    assert result["ok"] is False and OTHER_KEY not in result["message"]


# --- API -------------------------------------------------------------------------------


@pytest.fixture
def api_db(db_url, env):
    return db_url


def test_api_saves_switch_and_key_and_queues_the_source(api_db):
    assert api.update_outside_source(api.OutsideSourceUpdate(source="discogs", key=PAGE_KEY)) == {"status": "saved"}
    api.update_outside_source(api.OutsideSourceUpdate(source="musicbrainz", enabled=False))
    with psycopg.connect(api_db) as c:
        assert osrc.settings(c, "discogs").key == PAGE_KEY
        assert osrc.settings(c, "musicbrainz").enabled is False
        queued = c.execute("SELECT args->>'source' FROM task_queue WHERE task = 'fetch_outside_genres'").fetchall()
    assert queued == [("discogs",)]
    # Fields left out stay; null goes back to env / removes the page key.
    api.update_outside_source(api.OutsideSourceUpdate(source="discogs", enabled=None))
    api.update_outside_source(api.OutsideSourceUpdate(source="discogs", key=None))
    with psycopg.connect(api_db) as c:
        s = osrc.settings(c, "discogs")
    assert (s.enabled_from, s.key) == ("env", None)


def test_api_bad_key_does_not_echo_it(api_db):
    with pytest.raises(HTTPException) as err:
        api.update_outside_source(api.OutsideSourceUpdate(source="lastfm", key=f"{PAGE_KEY} extra"))
    assert err.value.status_code == 400 and PAGE_KEY not in str(err.value.detail)


def test_api_needs_key_material_to_save(api_db, monkeypatch):
    monkeypatch.setattr("config.ANALYTICS_API_SECRET", "")
    with pytest.raises(HTTPException) as err:
        api.update_outside_source(api.OutsideSourceUpdate(source="lastfm", key=PAGE_KEY))
    assert err.value.status_code == 409


def test_validation_errors_do_not_echo_the_body():
    exc = RequestValidationError([{"loc": ("body", "source"), "msg": "bad", "type": "literal_error", "input": {"key": PAGE_KEY}}])
    resp = asyncio.run(api.validation_error(None, exc))
    assert resp.status_code == 422 and PAGE_KEY.encode() not in resp.body


def test_api_status_and_test(api_db):
    status = api.outside_sources_status()
    assert [s["source"] for s in status["sources"]] == ["musicbrainz", "discogs", "lastfm"]
    assert api.test_outside_source(api.OutsideSourceTestRequest(source="discogs")) == {
        "ok": False, "message": "No Discogs key is set."
    }
