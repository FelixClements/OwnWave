"""Artist genre tags from the MusicBrainz web service.

One search request per artist name returns the best-matching artist and its
community tags (genres and other tags with vote counts). MusicBrainz allows
about one request per second per client and requires a descriptive
User-Agent; the client keeps to that.
"""

import json
import logging
import re
import time
import unicodedata
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from typing import Callable, List, Optional

log = logging.getLogger(__name__)

MIN_SCORE = 90  # MusicBrainz search score (0-100) required to accept a match
MIN_INTERVAL = 1.1  # seconds between requests

_CREDIT_SPLIT = re.compile(
    r"\s*(?:,|&|;|/|\sx\s|\sfeat\.?\s|\sft\.?\s|\sfeaturing\s|\svs\.?\s|\swith\s)\s*", re.IGNORECASE
)


def name_key(name: str) -> str:
    """Case-, accent- and spacing-insensitive key for an artist name."""
    s = unicodedata.normalize("NFKD", name or "")
    s = "".join(c for c in s if not unicodedata.combining(c))
    return " ".join(s.lower().split())


def primary_artist(credit: str) -> str:
    """First artist of a credit such as "A, B feat. C" (the one we look up)."""
    parts = [p for p in _CREDIT_SPLIT.split(credit or "") if p.strip()]
    return parts[0].strip() if parts else ""


@dataclass
class ArtistTags:
    mbid: Optional[str]
    tags: List[dict] = field(default_factory=list)  # [{"name": str, "count": int}]


class MusicBrainzClient:
    def __init__(
        self,
        base_url: str,
        user_agent: str,
        opener: Optional[Callable] = None,
        sleep: Callable[[float], None] = time.sleep,
        clock: Callable[[], float] = time.monotonic,
    ):
        self.base_url = base_url.rstrip("/")
        self.user_agent = user_agent
        self._open = opener or urllib.request.urlopen
        self._sleep = sleep
        self._clock = clock
        self._last = 0.0

    def _get(self, path: str, params: dict) -> dict:
        url = f"{self.base_url}{path}?{urllib.parse.urlencode(params)}"
        req = urllib.request.Request(url, headers={"User-Agent": self.user_agent, "Accept": "application/json"})
        for attempt in range(4):
            wait = self._last + MIN_INTERVAL - self._clock()
            if wait > 0:
                self._sleep(wait)
            self._last = self._clock()
            try:
                with self._open(req, timeout=20) as resp:
                    return json.loads(resp.read().decode("utf-8"))
            except urllib.error.HTTPError as exc:
                # 503 is MusicBrainz's rate-limit answer: back off and retry.
                if exc.code in (429, 503) and attempt < 3:
                    self._sleep(2.0 * (attempt + 1))
                    continue
                raise
        raise RuntimeError("unreachable")

    def lookup_artist(self, name: str) -> ArtistTags:
        """Best confident match for an artist name, or ArtistTags(None) if none."""
        key = name_key(name)
        if not key:
            return ArtistTags(None)
        query = 'artist:"{}"'.format(name.replace('"', ""))
        data = self._get("/ws/2/artist/", {"query": query, "fmt": "json", "limit": "5"})
        for artist in data.get("artists", []):
            if int(artist.get("score", 0)) < MIN_SCORE:
                continue
            names = {name_key(artist.get("name", "")), name_key(artist.get("sort-name", ""))}
            names |= {name_key(a.get("name", "")) for a in artist.get("aliases", []) or []}
            if key not in names:
                continue
            tags = [
                {"name": t["name"], "count": int(t.get("count", 0))}
                for t in artist.get("tags", []) or []
                if t.get("name") and int(t.get("count", 0)) > 0
            ]
            tags.sort(key=lambda t: -t["count"])
            return ArtistTags(artist.get("id"), tags)
        return ArtistTags(None)
