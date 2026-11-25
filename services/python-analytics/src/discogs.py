"""Release genres and styles from the Discogs API.

Releases are looked up by Discogs release id only; the id comes from the
MusicBrainz release's Discogs URL relation (see outside_genres). Discogs
allows 60 authenticated requests a minute and wants a User-Agent that names
the app; the client keeps one request per DISCOGS_MIN_INTERVAL seconds.
"""

import re
import time
from typing import Callable, Optional

from http_json import JsonClient

API_URL = "https://api.discogs.com"
MIN_INTERVAL = 1.1  # seconds between requests (60 a minute, with a margin)
BUSY_WAIT = 10.0  # Discogs counts requests over a moving minute

# https://www.discogs.com/release/249504-Rick-Astley-Never-Gonna-Give-You-Up
_RELEASE_URL = re.compile(r"^https?://(?:www\.)?discogs\.com/(?:[a-z]{2}/)?release/(\d+)", re.IGNORECASE)


def release_id_from_url(url: str) -> Optional[str]:
    """The release id in a Discogs release URL, or None for any other URL."""
    m = _RELEASE_URL.match(url or "")
    return m.group(1) if m else None


class DiscogsClient:
    def __init__(
        self,
        token: str,
        user_agent: str,
        base_url: str = API_URL,
        opener: Optional[Callable] = None,
        sleep: Callable[[float], None] = time.sleep,
        clock: Callable[[], float] = time.monotonic,
    ):
        self.base_url = base_url.rstrip("/")
        self.http = JsonClient(
            user_agent,
            MIN_INTERVAL,
            headers={"Authorization": f"Discogs token={token}"},
            opener=opener,
            sleep=sleep,
            clock=clock,
            busy_wait=BUSY_WAIT,
        )

    @property
    def requests(self) -> int:
        return self.http.requests

    def identity(self) -> Optional[dict]:
        """Who the token belongs to: one cheap request that fails with 401 for a bad token."""
        return self.http.get(f"{self.base_url}/oauth/identity")

    def release(self, release_id: str) -> Optional[dict]:
        """{"genres": [...], "styles": [...]} of a release, or None if Discogs does not know it."""
        data = self.http.get(f"{self.base_url}/releases/{release_id}")
        if data is None:
            return None
        return {"genres": list(data.get("genres") or []), "styles": list(data.get("styles") or [])}
