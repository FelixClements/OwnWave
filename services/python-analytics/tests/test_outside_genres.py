"""Outside genre sources (issue #135, ADR 0008 decision 5): lookups by ID only,
cached per release and recording, refetched when old. HTTP is faked; no test
calls MusicBrainz, Discogs or Last.fm."""

import io
import json
import urllib.error
import urllib.parse

import psycopg
import pytest

import http_json
import outside_genres as og
import task_queue
import tasks
from discogs import DiscogsClient, release_id_from_url
from http_json import JsonClient
from lastfm import LastfmClient, LastfmError
from musicbrainz import MusicBrainzClient

REL = "11111111-1111-1111-1111-111111111111"
RG = "22222222-2222-2222-2222-222222222222"
REC_A = "aaaaaaaa-0000-0000-0000-000000000001"
REC_B = "aaaaaaaa-0000-0000-0000-000000000002"
REC_LOOSE = "aaaaaaaa-0000-0000-0000-000000000003"  # on no tagged release
REC_ODD = "aaaaaaaa-0000-0000-0000-000000000004"  # tagged release does not list it


class FakeHttp:
    """Answers by URL path (and optional query items); records every request."""

    def __init__(self, routes=None):
        self.routes = dict(routes or {})
        self.requests = []

    def __call__(self, req, timeout):
        self.requests.append(req)
        url = urllib.parse.urlsplit(req.full_url)
        query = dict(urllib.parse.parse_qsl(url.query))
        for (path, match), answer in self.routes.items():
            if url.path == path and all(query.get(k) == v for k, v in match):
                if isinstance(answer, Exception):
                    raise answer
                if answer == 404:
                    raise urllib.error.HTTPError(req.full_url, 404, "not found", {}, None)
                return io.BytesIO(json.dumps(answer).encode())
        raise urllib.error.HTTPError(req.full_url, 404, "not found", {}, None)

    def paths(self):
        return [urllib.parse.urlsplit(r.full_url).path for r in self.requests]


def route(path, **match):
    return (path, tuple(sorted(match.items())))


def no_sleep(_):
    pass


def mb(http):
    return MusicBrainzClient("https://mb.test", "OwnWave-test/1.0", opener=http, sleep=no_sleep)


def discogs(http):
    return DiscogsClient("secret-token", "OwnWave-test/1.0", base_url="https://dc.test", opener=http, sleep=no_sleep)


def lastfm(http):
    return LastfmClient("key", "OwnWave-test/1.0", base_url="https://lf.test/2.0/", opener=http, sleep=no_sleep)


def genre(name, count):
    return {"id": f"g-{name}", "name": name, "count": count}


# --- Clients -------------------------------------------------------------


def test_json_client_answers_none_on_404_and_retries_when_busy():
    busy = urllib.error.HTTPError("u", 429, "slow down", {}, None)
    http = FakeHttp({route("/x"): {"ok": 1}})
    calls = iter([busy])

    def opener(req, timeout):
        for exc in calls:
            raise exc
        return http(req, timeout)

    slept = []
    c = JsonClient("ua", 1.0, opener=opener, sleep=slept.append, busy_wait=3.0, rand=lambda: 0.0)
    assert c.get("https://h.test/x") == {"ok": 1}
    assert c.get("https://h.test/missing") is None
    assert c.requests == 3 and c.retries == 1
    assert 3.0 in slept


def test_musicbrainz_lookups_send_ids_and_user_agent():
    http = FakeHttp({route(f"/ws/2/release/{REL}"): {"id": REL}})
    c = mb(http)
    assert c.release(REL) == {"id": REL}
    req = http.requests[0]
    assert req.get_header("User-agent") == "OwnWave-test/1.0"
    query = dict(urllib.parse.parse_qsl(urllib.parse.urlsplit(req.full_url).query))
    assert query == {"inc": "genres release-groups url-rels", "fmt": "json"}
    assert c.recording("unknown") is None


def test_release_recordings_reads_every_page():
    page1 = {"recording-count": 101, "recordings": [{"id": str(i)} for i in range(100)]}
    page2 = {"recording-count": 101, "recordings": [{"id": "100"}]}
    http = FakeHttp({
        route("/ws/2/recording", release=REL, offset="0"): page1,
        route("/ws/2/recording", release=REL, offset="100"): page2,
    })
    assert len(mb(http).release_recordings(REL)) == 101
    assert len(http.requests) == 2


@pytest.mark.parametrize("url,expected", [
    ("https://www.discogs.com/release/249504-Rick-Astley", "249504"),
    ("http://discogs.com/release/12", "12"),
    ("https://www.discogs.com/de/release/7", "7"),
    ("https://www.discogs.com/master/96559", None),
    ("https://example.com/release/1", None),
])
def test_discogs_release_url(url, expected):
    assert release_id_from_url(url) == expected


def test_discogs_release_sends_token_and_reads_genres_and_styles():
    http = FakeHttp({route("/releases/42"): {"genres": ["Electronic"], "styles": ["Techno", "Minimal"]}})
    c = discogs(http)
    assert c.release("42") == {"genres": ["Electronic"], "styles": ["Techno", "Minimal"]}
    assert http.requests[0].get_header("Authorization") == "Discogs token=secret-token"
    assert c.release("43") is None


def test_lastfm_matches_exact_artist_and_title():
    http = FakeHttp({route("/2.0/", artist="Amelie Lens", track="Hypnotized"): {
        "toptags": {"tag": [{"name": "techno", "count": 100}, {"name": "belgian", "count": 12}]}
    }})
    c = lastfm(http)
    assert c.track_top_tags("Amelie Lens", "Hypnotized") == [
        {"name": "techno", "count": 100}, {"name": "belgian", "count": 12},
    ]
    query = dict(urllib.parse.parse_qsl(urllib.parse.urlsplit(http.requests[0].full_url).query))
    assert query["method"] == "track.getTopTags"
    assert query["autocorrect"] == "0"


def test_lastfm_lone_tag_not_found_and_bad_key():
    lone = {"toptags": {"tag": {"name": "house", "count": 100}}}
    not_found = {"error": 6, "message": "Track not found"}
    bad_key = urllib.error.HTTPError("u", 403, "forbidden", {}, io.BytesIO(b'{"error": 10, "message": "Invalid API key"}'))
    http = FakeHttp({
        route("/2.0/", track="lone"): lone,
        route("/2.0/", track="gone"): not_found,
        route("/2.0/", track="key"): bad_key,
    })
    c = lastfm(http)
    assert c.track_top_tags("a", "lone") == [{"name": "house", "count": 100}]
    assert c.track_top_tags("a", "gone") is None
    with pytest.raises(LastfmError) as err:
        c.track_top_tags("a", "key")
    assert err.value.bad_key


def test_raw_genre_lists_keep_the_source_words():
    assert og.mb_genres({"genres": [genre("house", 1), genre("deep house", 4)]}) == [
        {"name": "deep house", "count": 4, "kind": "genre"},
        {"name": "house", "count": 1, "kind": "genre"},
    ]
    assert og.discogs_genres({"genres": ["Electronic"], "styles": ["Deep House"]}) == [
        {"name": "Electronic", "count": None, "kind": "genre"},
        {"name": "Deep House", "count": None, "kind": "style"},
    ]
    rels = [{"type": "discogs", "url": {"resource": "https://www.discogs.com/master/1"}},
            {"type": "discogs", "url": {"resource": "https://www.discogs.com/release/77"}}]
    assert og.discogs_link({"relations": rels}) == "77"
    assert og.discogs_link({}) is None


# --- Fetch and cache (database) -------------------------------------------


@pytest.fixture
def conn(db_url):
    with psycopg.connect(db_url) as c:
        yield c


def add_library(conn):
    """One tagged release (two songs), a song on an untagged-release, a song
    whose release does not list its recording, and a song with no ids."""
    artist = conn.execute("INSERT INTO artists (name) VALUES ('Amelie Lens') RETURNING id").fetchone()[0]
    rel = conn.execute(
        "INSERT INTO releases (mb_release_id, mb_release_group_id, folder, title) VALUES (%s, %s, '/m/r', 'R') RETURNING id",
        (REL, RG),
    ).fetchone()[0]
    plain = conn.execute("INSERT INTO releases (folder, title) VALUES ('/m/p', 'P') RETURNING id").fetchone()[0]
    rows = [
        ("/m/r/1.flac", "One", rel, REC_A),
        ("/m/r/2.flac", "Two", rel, REC_B),
        ("/m/r/3.flac", "Three", rel, REC_ODD),
        ("/m/p/1.flac", "Loose", plain, REC_LOOSE),
        ("/m/p/2.flac", "Untagged", plain, None),
    ]
    for path, title, release_id, rec in rows:
        conn.execute(
            "INSERT INTO tracks (path, title, artist_id, release_id, mb_recording_id) VALUES (%s, %s, %s, %s, %s)",
            (path, title, artist, release_id, rec),
        )
    conn.commit()


def mb_routes():
    return {
        route(f"/ws/2/release/{REL}"): {
            "id": REL,
            "genres": [genre("techno", 3)],
            "release-group": {"id": RG, "genres": [genre("techno", 5), genre("acid techno", 1)]},
            "relations": [{"type": "discogs", "url": {"resource": "https://www.discogs.com/release/555-X"}}],
        },
        route("/ws/2/recording", release=REL): {
            "recording-count": 2,
            "recordings": [{"id": REC_A, "genres": [genre("hard techno", 2)]}, {"id": REC_B, "genres": []}],
        },
        route(f"/ws/2/recording/{REC_LOOSE}"): {"id": REC_LOOSE, "genres": [genre("acid", 1)]},
        route(f"/ws/2/recording/{REC_ODD}"): 404,
    }


def cached(conn, source=None):
    rows = conn.execute(
        "SELECT source, ref, found, genres, discogs_release_id FROM outside_genres"
        " WHERE %s::text IS NULL OR source = %s ORDER BY source, ref",
        (source, source),
    ).fetchall()
    return {(r[0], r[1]): (r[2], r[3], r[4]) for r in rows}


def test_musicbrainz_fetch_matches_by_id_and_skips_songs_without_ids(conn):
    add_library(conn)
    http = FakeHttp(mb_routes())
    summary = og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)

    rows = cached(conn)
    assert rows[("mb_release", REL)] == (True, [{"name": "techno", "count": 3, "kind": "genre"}], "555")
    assert rows[("mb_release_group", RG)][1][0] == {"name": "techno", "count": 5, "kind": "genre"}
    assert rows[("mb_recording", REC_A)] == (True, [{"name": "hard techno", "count": 2, "kind": "genre"}], None)
    assert rows[("mb_recording", REC_B)] == (True, [], None)
    assert rows[("mb_recording", REC_LOOSE)][0] is True
    # MusicBrainz does not know it: cached as not found, so not asked again.
    assert rows[("mb_recording", REC_ODD)] == (False, [], None)
    assert len(rows) == 6

    # One release lookup, one recording list for the release, then only the
    # recordings it did not cover. No search and nothing for the untagged song.
    assert http.paths() == [
        f"/ws/2/release/{REL}", "/ws/2/recording", f"/ws/2/recording/{REC_LOOSE}", f"/ws/2/recording/{REC_ODD}",
    ]
    assert summary.more is False
    assert summary.discogs_links == 1
    assert summary.new_data == 4  # rows with genres: release, group, REC_A, REC_LOOSE


def test_cached_lookups_wait_until_they_are_old(conn):
    add_library(conn)
    og.fetch_due(conn, og.MUSICBRAINZ, mb(FakeHttp(mb_routes())), max_age=180)

    http = FakeHttp(mb_routes())
    again = og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert http.requests == [] and again.stored == 0

    conn.execute(
        "UPDATE outside_genres SET fetched_at = NOW() - interval '200 days' WHERE source = 'mb_recording' AND ref = %s",
        (REC_LOOSE,),
    )
    conn.commit()
    http = FakeHttp(mb_routes())
    refreshed = og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert http.paths() == [f"/ws/2/recording/{REC_LOOSE}"]
    assert refreshed.stored == 1 and refreshed.new_data == 0  # same genres as before


def test_marking_everything_due_refetches_all(conn):
    add_library(conn)
    og.fetch_due(conn, og.MUSICBRAINZ, mb(FakeHttp(mb_routes())), max_age=180)
    assert og.mark_all_due(conn) == 6
    conn.commit()
    http = FakeHttp(mb_routes())
    og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert len(http.requests) == 4


def test_release_group_without_embedded_genres_is_looked_up(conn):
    add_library(conn)
    routes = mb_routes()
    routes[route(f"/ws/2/release/{REL}")] = {"id": REL, "release-group": {"id": RG}, "genres": []}
    routes[route(f"/ws/2/release-group/{RG}")] = {"id": RG, "genres": [genre("techno", 9)]}
    http = FakeHttp(routes)
    og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert f"/ws/2/release-group/{RG}" in http.paths()
    assert cached(conn)[("mb_release_group", RG)][1] == [{"name": "techno", "count": 9, "kind": "genre"}]
    assert cached(conn)[("mb_release", REL)][2] is None  # no Discogs link


def test_budget_leaves_work_for_the_next_run(conn, monkeypatch):
    add_library(conn)
    monkeypatch.setitem(og.REQUESTS_PER_RUN, og.MUSICBRAINZ, 2)
    http = FakeHttp(mb_routes())
    first = og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert first.more is True
    assert len(http.requests) == 2  # the release and its recordings
    second = og.fetch_due(conn, og.MUSICBRAINZ, mb(FakeHttp(mb_routes())), max_age=180)
    assert second.more is False
    assert len(cached(conn, "mb_recording")) == 4


def test_failures_are_not_cached_and_stop_the_source(conn, monkeypatch):
    add_library(conn)
    monkeypatch.setattr(og, "MAX_FAILURES", 1)
    down = urllib.error.HTTPError("u", 500, "down", {}, None)
    http = FakeHttp({route(f"/ws/2/release/{REL}"): down})
    summary = og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert summary.failures == 1 and summary.more is False
    assert cached(conn) == {}


class Busy:
    """Answers busy (503) the first `times` requests to each path, then like `http`."""

    def __init__(self, http, times):
        self.http, self.times, self.seen = http, times, {}

    def __call__(self, req, timeout):
        path = urllib.parse.urlsplit(req.full_url).path
        self.seen[path] = self.seen.get(path, 0) + 1
        if self.seen[path] <= self.times:
            raise urllib.error.HTTPError(req.full_url, 503, "busy", {}, None)
        return self.http(req, timeout)


def test_busy_answers_that_succeed_on_retry_are_not_failures(conn, monkeypatch):
    add_library(conn)
    monkeypatch.setattr(og, "MAX_FAILURES", 1)
    summary = og.fetch_due(conn, og.MUSICBRAINZ, mb(Busy(FakeHttp(mb_routes()), times=2)), max_age=180)
    assert summary.failures == 0 and summary.status == "ok"
    assert summary.retries == 2 * 4  # release, its recordings, and the two loose recordings
    assert summary.requests == 3 * 4
    assert summary.as_dict()["retries"] == 8
    assert len(cached(conn, "mb_recording")) == 4


def test_busy_after_every_attempt_is_a_failure(conn, monkeypatch):
    add_library(conn)
    monkeypatch.setattr(og, "MAX_FAILURES", 1)
    http = Busy(FakeHttp(mb_routes()), times=99)
    summary = og.fetch_due(conn, og.MUSICBRAINZ, mb(http), max_age=180)
    assert summary.failures == 1 and summary.status == "failed"
    assert summary.requests == http_json.ATTEMPTS and summary.retries == http_json.ATTEMPTS - 1
    assert cached(conn) == {}


def test_discogs_follows_the_musicbrainz_link(conn):
    add_library(conn)
    og.fetch_due(conn, og.MUSICBRAINZ, mb(FakeHttp(mb_routes())), max_age=180)
    http = FakeHttp({route("/releases/555"): {"genres": ["Electronic"], "styles": ["Techno"]}})
    summary = og.fetch_due(conn, og.DISCOGS, discogs(http), max_age=180)
    assert http.paths() == ["/releases/555"]
    assert cached(conn, "discogs_release") == {("discogs_release", "555"): (True, [
        {"name": "Electronic", "count": None, "kind": "genre"},
        {"name": "Techno", "count": None, "kind": "style"},
    ], None)}
    assert summary.new_data == 1


def test_discogs_stops_on_a_refused_token(conn):
    add_library(conn)
    og.fetch_due(conn, og.MUSICBRAINZ, mb(FakeHttp(mb_routes())), max_age=180)
    refused = urllib.error.HTTPError("u", 401, "unauthorized", {}, None)
    summary = og.fetch_due(conn, og.DISCOGS, discogs(FakeHttp({route("/releases/555"): refused})), max_age=180)
    assert summary.more is False and summary.stored == 0


def test_lastfm_looks_up_each_artist_and_title_once(conn):
    add_library(conn)
    http = FakeHttp({
        route("/2.0/", track="One"): {"toptags": {"tag": [{"name": "techno", "count": 100}]}},
        route("/2.0/", track="Two"): {"error": 6, "message": "Track not found"},
    })
    summary = og.fetch_due(conn, og.LASTFM, lastfm(http), max_age=180)
    assert len(http.requests) == 5  # every song has an artist and title; no ids needed
    rows = cached(conn, "lastfm_track")
    assert rows[("lastfm_track", og.lastfm_ref("Amelie Lens", "One"))] == (
        True, [{"name": "techno", "count": 100, "kind": "tag"}], None)
    assert rows[("lastfm_track", og.lastfm_ref("Amelie Lens", "Two"))][0] is False
    assert summary.stored == 5
    http = FakeHttp()
    og.fetch_due(conn, og.LASTFM, lastfm(http), max_age=180)
    assert http.requests == []


# --- Background task ------------------------------------------------------


def queued(db_url):
    with psycopg.connect(db_url) as c:
        return c.execute("SELECT task, args FROM task_queue ORDER BY id").fetchall()


@pytest.fixture
def all_on(monkeypatch):
    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", True)
    monkeypatch.setattr("config.DISCOGS_ENABLED", True)
    monkeypatch.setattr("config.LASTFM_ENABLED", True)
    monkeypatch.setattr("config.DISCOGS_TOKEN", "fake-discogs-token")
    monkeypatch.setattr("config.LASTFM_API_KEY", "fake-lastfm-key")


def test_sources_without_credentials_are_switched_off(db_url, monkeypatch):
    import outside_sources

    monkeypatch.setattr("config.MUSICBRAINZ_ENABLED", True)
    monkeypatch.setattr("config.DISCOGS_ENABLED", True)
    monkeypatch.setattr("config.LASTFM_ENABLED", True)
    monkeypatch.setattr("config.DISCOGS_TOKEN", "")
    monkeypatch.setattr("config.LASTFM_API_KEY", "")
    with psycopg.connect(db_url) as c:
        assert outside_sources.enabled_sources(c) == ["musicbrainz"]
    assert tasks.fetch_outside_genres("lastfm") == {"source": "lastfm", "skipped": "switched off"}


def test_config_reads_the_env_switches_and_keys(monkeypatch):
    import importlib

    import config

    monkeypatch.delenv("DISCOGS_TOKEN", raising=False)
    monkeypatch.setenv("LASTFM_API_KEY", "  ")
    monkeypatch.setenv("DISCOGS_ENABLED", "false")
    try:
        importlib.reload(config)
        # The switches are read as set; a missing key is checked per source
        # (outside_sources), because the admin page may supply one.
        assert config.DISCOGS_ENABLED is False and config.LASTFM_ENABLED is True
        assert config.DISCOGS_TOKEN == "" and config.LASTFM_API_KEY == ""
    finally:
        monkeypatch.undo()
        importlib.reload(config)


def test_queue_skips_sources_already_queued(db_url, all_on):
    with psycopg.connect(db_url) as c:
        assert tasks.queue_outside_genres(c) == ["musicbrainz", "discogs", "lastfm"]
        assert tasks.queue_outside_genres(c) == []
        assert tasks.queue_outside_genres(c, ["discogs"]) == []
        c.commit()
    assert [a["source"] for _, a in queued(db_url)] == ["musicbrainz", "discogs", "lastfm"]


def test_task_runs_on_until_done_then_queues_a_rescore(db_url, all_on, monkeypatch):
    with psycopg.connect(db_url) as c:
        add_library(c)
    monkeypatch.setitem(og.REQUESTS_PER_RUN, og.MUSICBRAINZ, 2)
    monkeypatch.setattr(og, "make_client", lambda source, key=None: mb(FakeHttp(mb_routes())))

    first = tasks.fetch_outside_genres("musicbrainz")
    assert first["more"] is True
    # The release brought a Discogs link: the Discogs fetch starts too.
    assert queued(db_url) == [
        ("fetch_outside_genres", {"source": "musicbrainz", "new_data": 3}),
        ("fetch_outside_genres", {"source": "discogs"}),
    ]
    with psycopg.connect(db_url) as c:
        c.execute("DELETE FROM task_queue")
        c.commit()
    second = tasks.fetch_outside_genres("musicbrainz", new_data=3)
    assert second["more"] is False
    assert queued(db_url) == [("score_genres", {})]


def test_no_rescore_without_new_data(db_url, all_on, monkeypatch):
    monkeypatch.setattr(og, "make_client", lambda source, key=None: mb(FakeHttp()))
    assert tasks.fetch_outside_genres("musicbrainz")["stored"] == 0
    assert queued(db_url) == []


def test_task_waits_while_the_artist_fetch_holds_musicbrainz(db_url, all_on):
    with psycopg.connect(db_url) as holder:
        holder.execute("SELECT pg_advisory_lock(%s)", (tasks._MB_LOCK,))
        result = tasks.fetch_outside_genres("musicbrainz", new_data=2)
        assert result["skipped"] == "another fetch is running"
        with psycopg.connect(db_url) as c:
            row = c.execute(
                "SELECT task, args, run_after > NOW() + interval '30 seconds' FROM task_queue"
            ).fetchone()
        assert row == ("fetch_outside_genres", {"source": "musicbrainz", "new_data": 2}, True)


def test_genre_refresh_starts_the_outside_fetch(db_url, all_on):
    with psycopg.connect(db_url) as c:
        tasks.queue_genre_refresh(c)
        c.commit()
    assert [t for t, _ in queued(db_url)] == ["fetch_artist_genres"] + ["fetch_outside_genres"] * 3


def test_enqueue_later(db_url):
    with psycopg.connect(db_url) as c:
        task_queue.enqueue_later(c, 3600, "score_genres")
        c.commit()
        assert task_queue.claim(c) is None
