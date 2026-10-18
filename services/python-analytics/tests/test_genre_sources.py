import pytest

from genre_sources import get_genre_sources
from genre_tag_source import TagGenreSource


def test_get_genre_sources_includes_tags():
    sources = get_genre_sources()
    assert any(isinstance(s, TagGenreSource) for s in sources)


def test_tag_genre_source_id():
    assert TagGenreSource().source_id == "tags"


def test_discogs400_predict_parses_snippet_activations():
    import numpy as np

    from genre_discogs400 import Discogs400GenreSource

    class Loader:
        def configure(self, **kwargs):
            pass

        def __call__(self):
            # 90 s of 16 kHz mono audio, as essentia's MonoLoader returns it.
            return np.zeros(16000 * 90, dtype=np.float32)

    source = Discogs400GenreSource()
    source._loader = Loader()
    source._embed_model = lambda snippet: np.ones((len(snippet) // 16000, 4), dtype=np.float32)
    source._class_model = lambda embeddings: np.tile(np.array([0.1, 0.8, 0.5], dtype=np.float32), (len(embeddings), 1))
    source._labels = ["Rock---Punk", "Electronic---Techno", "Jazz"]

    predictions = source.predict("/music/track.flac")

    assert [(p.main_genre, p.sub_genre) for p in predictions] == [("Electronic", "Techno"), ("Jazz", "Jazz")]
    assert predictions[0].confidence == pytest.approx(0.8)
    assert all(p.source == "discogs400" for p in predictions)
