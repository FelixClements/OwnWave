"""Rate-limited JSON GET requests for the outside web services.

MusicBrainz, Discogs and Last.fm each limit how often one client may call
them and want a User-Agent that names the app. One JsonClient per service
keeps the gap between its requests and backs off when the service says it
is busy.
"""

import json
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Callable, Optional

# HTTP answers that mean "too many requests, try again shortly".
BUSY_CODES = (429, 503)
ATTEMPTS = 4


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
    ):
        self.user_agent = user_agent
        self.min_interval = min_interval
        self.headers = dict(headers or {})
        self.busy_wait = busy_wait
        self.timeout = timeout
        self._open = opener or urllib.request.urlopen
        self._sleep = sleep
        self._clock = clock
        self._last: Optional[float] = None
        # Requests sent, retries included (callers budget a run by it).
        self.requests = 0

    def get(self, url: str, params: Optional[dict] = None) -> Optional[dict]:
        """The decoded JSON body, or None when the service answers 404.

        Other HTTP errors raise urllib.error.HTTPError after the retries.
        """
        if params:
            url = f"{url}?{urllib.parse.urlencode(params)}"
        headers = {"User-Agent": self.user_agent, "Accept": "application/json", **self.headers}
        req = urllib.request.Request(url, headers=headers)
        for attempt in range(ATTEMPTS):
            if self._last is not None:
                wait = self._last + self.min_interval - self._clock()
                if wait > 0:
                    self._sleep(wait)
            self._last = self._clock()
            self.requests += 1
            try:
                with self._open(req, timeout=self.timeout) as resp:
                    return json.loads(resp.read().decode("utf-8"))
            except urllib.error.HTTPError as exc:
                if exc.code == 404:
                    return None
                if exc.code in BUSY_CODES and attempt < ATTEMPTS - 1:
                    self._sleep(self.busy_wait * (attempt + 1))
                    continue
                raise
        raise RuntimeError("unreachable")
