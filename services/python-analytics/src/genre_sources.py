from typing import List

from config import ENABLE_GENRE_ANALYSIS
from genre_discogs400 import Discogs400GenreSource
from genre_tag_source import TagGenreSource
from models import GenreSource

_cached_sources: List[GenreSource] | None = None


def get_genre_sources() -> List[GenreSource]:
    global _cached_sources
    if _cached_sources is not None:
        return _cached_sources

    sources: List[GenreSource] = [TagGenreSource()]
    if ENABLE_GENRE_ANALYSIS:
        ml = Discogs400GenreSource.from_config()
        if ml is not None:
            sources.append(ml)
    _cached_sources = sources
    return sources

