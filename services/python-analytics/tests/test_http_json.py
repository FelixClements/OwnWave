"""Busy answers in the shared JSON client (issue #151): Retry-After, back-off
and retries. HTTP and time are faked; no test calls a real API."""

import email.message
import email.utils
import io
import json
import urllib.error

import pytest

from discogs import DiscogsClient
from http_json import ATTEMPTS, MAX_BUSY_WAIT, JsonClient, retry_after
from lastfm import LastfmClient, LastfmError

WALL = 1_800_000_000.0  # the fake wall clock, for Retry-After dates


class FakeOpener:
    """Answers each request with the next response: a dict (JSON body) or an exception."""

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


def busy(code=503, retry_after_value=None, body=None):
    headers = email.message.Message()
    if retry_after_value is not None:
        headers["Retry-After"] = retry_after_value
    fp = io.BytesIO(json.dumps(body).encode()) if body is not None else None
    return urllib.error.HTTPError("https://h.test/x", code, "busy", headers, fp)


def client(responses, min_interval=1.1, busy_wait=2.0, rand=lambda: 0.0, **kw):
    opener, clock = FakeOpener(responses), FakeClock()
    c = JsonClient(
        "ua", min_interval, opener=opener, sleep=clock.sleep, clock=clock,
        busy_wait=busy_wait, rand=rand, wall_clock=lambda: WALL, **kw,
    )
    return c, opener, clock


def test_retry_after_reads_seconds_and_http_dates():
    assert retry_after("7", WALL) == 7.0
    assert retry_after(" 1.5 ", WALL) == 1.5
    assert retry_after(email.utils.formatdate(WALL + 30, usegmt=True), WALL) == pytest.approx(30)
    assert retry_after(email.utils.formatdate(WALL - 30, usegmt=True), WALL) == 0.0  # already past
    assert retry_after("-3", WALL) == 0.0
    for bad in (None, "", "soon", "inf", "nan"):
        assert retry_after(bad, WALL) is None


def test_waits_for_retry_after_in_seconds():
    c, opener, clock = client([busy(503, "7"), {"ok": 1}])
    assert c.get("https://h.test/x") == {"ok": 1}
    assert clock.slept == [7.0]
    assert len(opener.requests) == 2
    assert (c.requests, c.retries) == (2, 1)


def test_waits_for_retry_after_as_an_http_date():
    when = email.utils.formatdate(WALL + 12, usegmt=True)
    c, _, clock = client([busy(429, when), {"ok": 1}])
    assert c.get("https://h.test/x") == {"ok": 1}
    assert clock.slept == [pytest.approx(12)]


def test_backs_off_exponentially_without_retry_after():
    c, _, clock = client([busy(), busy(), busy(), {"ok": 1}], busy_wait=2.0)
    assert c.get("https://h.test/x") == {"ok": 1}
    assert clock.slept == [2.0, 4.0, 8.0]
    assert c.retries == 3


def test_back_off_has_jitter_and_a_cap():
    c, _, clock = client([busy()] * (ATTEMPTS - 1) + [{"ok": 1}], busy_wait=10.0, rand=lambda: 1.0)
    assert c.get("https://h.test/x") == {"ok": 1}
    # 10, 20, 40 and 80 seconds, each stretched by half, never over the cap.
    assert clock.slept == [15.0, 30.0, MAX_BUSY_WAIT, MAX_BUSY_WAIT]


def test_keeps_the_gap_between_requests_after_a_short_wait():
    c, _, clock = client([busy(503, "0"), {"ok": 1}], min_interval=1.1)
    assert c.get("https://h.test/x") == {"ok": 1}
    assert clock.slept == [pytest.approx(1.1)]  # the rate limit, not the zero wait


def test_gives_up_after_the_last_attempt():
    c, opener, _ = client([busy()] * ATTEMPTS)
    with pytest.raises(urllib.error.HTTPError) as err:
        c.get("https://h.test/x")
    assert err.value.code == 503
    assert len(opener.requests) == ATTEMPTS
    assert (c.requests, c.retries) == (ATTEMPTS, ATTEMPTS - 1)


def test_a_retry_after_over_the_cap_fails_now():
    c, opener, clock = client([busy(503, str(int(MAX_BUSY_WAIT) + 1)), {"ok": 1}])
    with pytest.raises(urllib.error.HTTPError):
        c.get("https://h.test/x")
    assert len(opener.requests) == 1 and clock.slept == []


def test_other_errors_are_not_retried():
    c, opener, _ = client([busy(500), {"ok": 1}])
    with pytest.raises(urllib.error.HTTPError):
        c.get("https://h.test/x")
    assert len(opener.requests) == 1 and c.retries == 0


# --- The services ---------------------------------------------------------------


def test_discogs_honours_retry_after_on_429():
    opener, clock = FakeOpener([busy(429, "5"), {"genres": ["Electronic"], "styles": []}]), FakeClock()
    dc = DiscogsClient("t", "ua", base_url="https://dc.test", opener=opener, sleep=clock.sleep, clock=clock)
    assert dc.release("1") == {"genres": ["Electronic"], "styles": []}
    assert clock.slept == [5.0]
    assert dc.retries == 1


def lastfm(responses):
    opener, clock = FakeOpener(responses), FakeClock()
    lf = LastfmClient("key", "ua", base_url="https://lf.test/2.0/", opener=opener, sleep=clock.sleep, clock=clock)
    return lf, opener, clock


RATE_LIMITED = {"error": 29, "message": "Rate limit exceeded"}
TAGS = {"toptags": {"tag": [{"name": "techno", "count": 100}]}}


def test_lastfm_retries_error_29_in_an_ok_answer():
    lf, opener, clock = lastfm([RATE_LIMITED, TAGS])
    assert lf.track_top_tags("A", "T") == [{"name": "techno", "count": 100}]
    assert len(opener.requests) == 2 and lf.retries == 1
    assert len(clock.slept) == 1 and 2.0 <= clock.slept[0] <= 3.0  # back-off with jitter


def test_lastfm_retries_error_29_in_an_error_answer():
    lf, opener, _ = lastfm([busy(400, body=RATE_LIMITED), TAGS])
    assert lf.track_top_tags("A", "T") == [{"name": "techno", "count": 100}]
    assert lf.retries == 1


def test_lastfm_gives_up_on_error_29_after_the_last_attempt():
    lf, opener, _ = lastfm([RATE_LIMITED] * ATTEMPTS)
    with pytest.raises(LastfmError) as err:
        lf.track_top_tags("A", "T")
    assert err.value.code == 29 and not err.value.bad_key
    assert len(opener.requests) == ATTEMPTS


def test_lastfm_other_errors_still_reach_the_caller():
    lf, opener, _ = lastfm([busy(403, body={"error": 10, "message": "Invalid API key"})])
    with pytest.raises(LastfmError) as err:
        lf.track_top_tags("A", "T")
    assert err.value.bad_key
    assert len(opener.requests) == 1 and lf.retries == 0
