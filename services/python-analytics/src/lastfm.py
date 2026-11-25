"""Track tags from the Last.fm API.

A track is matched by its exact artist and title (track.getTopTags with
autocorrect off), never by search. Last.fm asks clients to stay under five
requests a second and to send a User-Agent that names the app. Its API is
free for non-commercial use, which a home server is.
"""

import json
import time
import urllib.error
from typing import Callable, List, Optional

from http_json import JsonClient

API_URL = "https://ws.audioscrobbler.com/2.0/"
MIN_INTERVAL = 0.25  # seconds between requests (4 a second)

# Last.fm error codes (https://www.last.fm/api/errorcodes).
NOT_FOUND = 6  # "Track not found" (also artist not found)
BAD_KEY_ERRORS = {4, 9, 10, 14, 26}  # authentication failed, invalid or suspended key
RATE_LIMITED = 29  # "Rate limit exceeded": the client backs off and retries


class LastfmError(Exception):
    def __init__(self, code: int, message: str):
        super().__init__(f"Last.fm error {code}: {message}")
        self.code = code

    @property
    def bad_key(self) -> bool:
        return self.code in BAD_KEY_ERRORS


class LastfmClient:
    def __init__(
        self,
        api_key: str,
        user_agent: str,
        base_url: str = API_URL,
        opener: Optional[Callable] = None,
        sleep: Callable[[float], None] = time.sleep,
        clock: Callable[[], float] = time.monotonic,
    ):
        self.api_key = api_key
        self.base_url = base_url
        self.http = JsonClient(
            user_agent, MIN_INTERVAL, opener=opener, sleep=sleep, clock=clock, busy_body=_rate_limited
        )

    @property
    def requests(self) -> int:
        return self.http.requests

    @property
    def retries(self) -> int:
        return self.http.retries

    def check_key(self) -> None:
        """One cheap request that raises LastfmError when Last.fm refuses the key."""
        params = {"method": "chart.getTopTags", "limit": "1", "api_key": self.api_key, "format": "json"}
        try:
            data = self.http.get(self.base_url, params)
        except urllib.error.HTTPError as exc:
            data = _error_body(exc)
            if data is None:
                raise
        if data and "error" in data:
            raise LastfmError(int(data.get("error") or 0), str(data.get("message", "")))

    def track_top_tags(self, artist: str, title: str) -> Optional[List[dict]]:
        """[{"name", "count"}] for the exact artist and title, or None when Last.fm does not know the track."""
        params = {
            "method": "track.getTopTags",
            "artist": artist,
            "track": title,
            "autocorrect": "0",
            "api_key": self.api_key,
            "format": "json",
        }
        try:
            data = self.http.get(self.base_url, params)
        except urllib.error.HTTPError as exc:
            # Last.fm answers errors with a JSON body naming the error code.
            data = _error_body(exc)
            if data is None:
                raise
        if data is None:
            return None
        if "error" in data:
            code = int(data.get("error") or 0)
            if code == NOT_FOUND:
                return None
            raise LastfmError(code, str(data.get("message", "")))
        tags = (data.get("toptags") or {}).get("tag") or []
        if isinstance(tags, dict):  # Last.fm's JSON gives a lone tag as an object
            tags = [tags]
        return [
            {"name": t["name"], "count": int(t.get("count") or 0)}
            for t in tags
            if isinstance(t, dict) and t.get("name")
        ]


def _rate_limited(data: dict) -> bool:
    try:
        return int(data.get("error") or 0) == RATE_LIMITED
    except (TypeError, ValueError):
        return False


def _error_body(exc: urllib.error.HTTPError) -> Optional[dict]:
    try:
        data = json.loads(exc.read().decode("utf-8"))
    except Exception:
        return None
    return data if isinstance(data, dict) and "error" in data else None
