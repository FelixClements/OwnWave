"""Rate-limited JSON GET requests for the outside web services.

MusicBrainz, Discogs and Last.fm each limit how often one client may call
them and want a User-Agent that names the app. One JsonClient per service
keeps the gap between its requests and backs off when the service says it
is busy.

A busy answer (HTTP 429 or 503, or a body the caller marks as busy, such as
Last.fm's error 29) is retried up to ATTEMPTS times in all. The wait is the
service's Retry-After header (seconds or an HTTP date) when it sends one,
otherwise an exponential back-off with jitter, never more than
MAX_BUSY_WAIT. The gap between requests still applies after a wait. Only a
request that is still busy after the last attempt raises, so a lookup that
succeeded on retry does not count as a failure.
"""

import email.utils
import io
import json
import math
import random
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import timezone
from typing import Callable, Optional

# HTTP answers that mean "too many requests, try again shortly".
BUSY_CODES = (429, 503)
# Requests per lookup, the first one included.
ATTEMPTS = 5
# Longest wait between two attempts, in seconds. A Retry-After longer than
# this is not waited out: the lookup fails now and a later run tries again.
MAX_BUSY_WAIT = 60.0
# A back-off without Retry-After is stretched by up to this fraction at random,
# so that clients that were refused together do not retry together.
JITTER = 0.5


def retry_after(value: Optional[str], now: float) -> Optional[float]:
    """Seconds to wait from a Retry-After header (delay-seconds or HTTP date),
    or None when there is no usable value. now is the wall clock (time.time)."""
    if not value:
        return None
    value = value.strip()
    try:
        seconds = float(value)
    except ValueError:
        try:
            when = email.utils.parsedate_to_datetime(value)
        except (TypeError, ValueError, IndexError):
            return None
        if when is None:
            return None
        if when.tzinfo is None:
            when = when.replace(tzinfo=timezone.utc)
        seconds = when.timestamp() - now
    if not math.isfinite(seconds):
        return None
    return max(0.0, seconds)


class JsonClient:
    def __init__(
        self,
        user_agent: str,
        min_interval: float,
        headers: Optional[dict] = None,
        opener: Optional[Callable] = None,
        sleep: Callable[[float], None] = time.sleep,
        clock: Callable[[], float] = time.monotonic,
        busy_wait: float = 2.0,
        timeout: float = 20,
        busy_body: Optional[Callable[[dict], bool]] = None,
        rand: Callable[[], float] = random.random,
        wall_clock: Callable[[], float] = time.time,
    ):
        self.user_agent = user_agent
        self.min_interval = min_interval
        self.headers = dict(headers or {})
        # First back-off without Retry-After; it doubles on every attempt.
        self.busy_wait = busy_wait
        self.timeout = timeout
        # Marks a JSON body as a busy answer (for services that say so in the body).
        self.busy_body = busy_body
        self._open = opener or urllib.request.urlopen
        self._sleep = sleep
        self._clock = clock
        self._rand = rand
        self._wall_clock = wall_clock
        self._last: Optional[float] = None
        # Requests sent, retries included (callers budget a run by it).
        self.requests = 0
        # Requests sent again after a busy answer.
        self.retries = 0

    def get(self, url: str, params: Optional[dict] = None) -> Optional[dict]:
        """The decoded JSON body, or None when the service answers 404.

        Other HTTP errors raise urllib.error.HTTPError after the retries.
        """
        if params:
            url = f"{url}?{urllib.parse.urlencode(params)}"
        headers = {"User-Agent": self.user_agent, "Accept": "application/json", **self.headers}
        req = urllib.request.Request(url, headers=headers)
        for attempt in range(ATTEMPTS):
            if attempt:
                self.retries += 1
            last_attempt = attempt == ATTEMPTS - 1
            self._pace()
            try:
                with self._open(req, timeout=self.timeout) as resp:
                    data = json.loads(resp.read().decode("utf-8"))
                if last_attempt or not self._is_busy_body(data):
                    return data
                wait = self._backoff(attempt, None)
            except urllib.error.HTTPError as exc:
                if exc.code == 404:
                    return None
                busy = exc.code in BUSY_CODES
                if not busy and self.busy_body is not None:
                    # Read the body to see whether it says "busy"; give it
                    # back to the caller, who may read it too.
                    body = exc.read() if exc.fp is not None else b""
                    busy = self._is_busy_body(_json_or_none(body))
                    exc = urllib.error.HTTPError(exc.url, exc.code, exc.msg, exc.hdrs, io.BytesIO(body))
                if not busy or last_attempt:
                    raise exc
                wait = self._backoff(attempt, exc.headers)
                if wait is None:
                    raise exc
            if wait > 0:
                self._sleep(wait)
        raise RuntimeError("unreachable")

    def _pace(self) -> None:
        if self._last is not None:
            wait = self._last + self.min_interval - self._clock()
            if wait > 0:
                self._sleep(wait)
        self._last = self._clock()
        self.requests += 1

    def _is_busy_body(self, data) -> bool:
        return self.busy_body is not None and isinstance(data, dict) and bool(self.busy_body(data))

    def _backoff(self, attempt: int, headers) -> Optional[float]:
        """Seconds to wait before the next attempt, or None to give up now
        (the service asked for a wait longer than MAX_BUSY_WAIT)."""
        asked = retry_after(headers.get("Retry-After") if headers is not None else None, self._wall_clock())
        if asked is not None:
            return asked if asked <= MAX_BUSY_WAIT else None
        wait = self.busy_wait * (2 ** attempt) * (1 + JITTER * self._rand())
        return min(wait, MAX_BUSY_WAIT)


def _json_or_none(body: bytes):
    try:
        return json.loads(body.decode("utf-8"))
    except Exception:
        return None
