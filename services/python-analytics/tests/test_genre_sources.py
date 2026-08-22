from genre_sources import get_genre_sources
from genre_tag_source import TagGenreSource


def test_get_genre_sources_includes_tags():
    sources = get_genre_sources()
    assert any(isinstance(s, TagGenreSource) for s in sources)


def test_tag_genre_source_id():
    assert TagGenreSource().source_id == "tags"
