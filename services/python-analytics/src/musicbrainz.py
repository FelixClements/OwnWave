"""Genres and tags from the MusicBrainz web service.

Artist tags: one search request per artist name returns the best-matching
artist and its community tags (genres and other tags with vote counts).
Recording, release and release group genres are looked up by MusicBrainz id
only (see outside_genres). MusicBrainz allows
about one request per second per client and requires a descriptive
User-Agent; the client keeps to that.
"""

import logging
import re
import time
import unicodedata
from dataclasses import dataclass, field
from typing import Callable, List, Optional

from http_json import JsonClient

log = logging.getLogger(__name__)

MIN_SCORE = 90  # MusicBrainz search score (0-100) required to accept a match
MIN_INTERVAL = 1.1  # seconds between requests
BROWSE_LIMIT = 100  # MusicBrainz's largest page

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
        # 503 is MusicBrainz's rate-limit answer: the client honours Retry-After or
        # backs off, and retries.
        self.http = JsonClient(user_agent, MIN_INTERVAL, opener=opener, sleep=sleep, clock=clock)

    @property
    def requests(self) -> int:
        return self.http.requests

    @property
    def retries(self) -> int:
        return self.http.retries

    def _get(self, path: str, params: dict) -> Optional[dict]:
        return self.http.get(f"{self.base_url}{path}", {**params, "fmt": "json"})

    # Lookups by MusicBrainz id (genre engine v2, ADR 0008: no name search).
    # Each returns None when MusicBrainz does not know the id.

    def release(self, mbid: str) -> Optional[dict]:
        """A release with its genres, release group and URL relations."""
        return self._get(f"/ws/2/release/{mbid}", {"inc": "genres release-groups url-rels"})

    def release_group(self, mbid: str) -> Optional[dict]:
        return self._get(f"/ws/2/release-group/{mbid}", {"inc": "genres"})

    def recording(self, mbid: str) -> Optional[dict]:
        return self._get(f"/ws/2/recording/{mbid}", {"inc": "genres"})

    def ping(self) -> Optional[dict]:
        """One cheap request (the first genre of the genre list), for the admin Test button."""
        return self._get("/ws/2/genre/all", {"limit": "1"})

    def release_recordings(self, mbid: str) -> List[dict]:
        """Every recording on a release, with genres, in pages of BROWSE_LIMIT."""
        recordings: List[dict] = []
        while True:
            page = self._get(
                "/ws/2/recording",
                {"release": mbid, "inc": "genres", "limit": str(BROWSE_LIMIT), "offset": str(len(recordings))},
            )
            got = (page or {}).get("recordings") or []
            recordings.extend(got)
            if not got or len(recordings) >= int((page or {}).get("recording-count", 0)):
                return recordings

    def lookup_artist(self, name: str) -> ArtistTags:
        """Best confident match for an artist name, or ArtistTags(None) if none."""
        key = name_key(name)
        if not key:
            return ArtistTags(None)
        query = 'artist:"{}"'.format(name.replace('"', ""))
        data = self._get("/ws/2/artist/", {"query": query, "limit": "5"}) or {}
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
