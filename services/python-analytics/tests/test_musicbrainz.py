import io
import json
import urllib.error

import pytest

from musicbrainz import MIN_INTERVAL, MusicBrainzClient, name_key, primary_artist


class FakeOpener:
    def __init__(self, responses):
        self.responses = list(responses)
        self.requests = []

    def __call__(self, req, timeout):
        self.requests.append(req)
        r = self.responses.pop(0)
        if isinstance(r, Exception):
            raise r
        return io.BytesIO(json.dumps(r).encode())


class FakeClock:
    def __init__(self):
        self.now = 100.0
        self.slept = []

    def sleep(self, s):
        self.slept.append(s)
        self.now += s

    def __call__(self):
        return self.now


def client(responses):
    opener, clock = FakeOpener(responses), FakeClock()
    c = MusicBrainzClient("https://mb.test", "OwnWave-test/1.0", opener=opener, sleep=clock.sleep, clock=clock)
    return c, opener, clock


def artist(name, score=100, tags=(), **extra):
    return {"id": f"id-{name}", "name": name, "sort-name": name, "score": score,
            "tags": [{"name": n, "count": c} for n, c in tags], **extra}


def test_lookup_returns_tags_sorted_by_votes_and_sends_user_agent():
    c, opener, _ = client([{"artists": [artist("Amelie Lens", tags=[("house", 1), ("techno", 7), ("zero", 0)])]}])
    found = c.lookup_artist("Amélie Lens")
    assert found.mbid == "id-Amelie Lens"
    assert found.tags == [{"name": "techno", "count": 7}, {"name": "house", "count": 1}]
    assert opener.requests[0].get_header("User-agent") == "OwnWave-test/1.0"
    assert "artist%3A%22Am" in opener.requests[0].full_url


def test_lookup_rejects_low_scores_and_other_names():
    c, _, _ = client([{"artists": [artist("Lens", score=100), artist("Amelie Lens", score=80)]}])
    assert c.lookup_artist("Amelie Lens").mbid is None


def test_lookup_accepts_alias():
    c, _, _ = client([{"artists": [artist("Beyoncé", aliases=[{"name": "Beyonce Knowles"}])]}])
    assert c.lookup_artist("Beyonce Knowles").mbid == "id-Beyoncé"


def test_requests_are_spaced_by_the_rate_limit():
    c, _, clock = client([{"artists": []}, {"artists": []}])
    c.lookup_artist("A")
    c.lookup_artist("B")
    assert clock.slept == [pytest.approx(MIN_INTERVAL)]


def test_retries_on_503():
    busy = urllib.error.HTTPError("u", 503, "busy", {}, None)
    c, opener, _ = client([busy, {"artists": [artist("A")]}])
    assert c.lookup_artist("A").mbid == "id-A"
    assert len(opener.requests) == 2


def test_name_helpers():
    assert name_key("  Amélie   LENS ") == "amelie lens"
    assert primary_artist("Charlotte de Witte, Theo Nasa") == "Charlotte de Witte"
    assert primary_artist("Lilly Palmer feat. Maddix") == "Lilly Palmer"
    assert primary_artist("Solo") == "Solo"
