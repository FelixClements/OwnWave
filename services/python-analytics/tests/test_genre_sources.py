import numpy as np
import pytest

from genre_discogs400 import Discogs400GenreSource
from genre_sources import get_genre_sources
from genre_tag_source import TagGenreSource


def test_get_genre_sources_includes_tags():
    sources = get_genre_sources()
    assert any(isinstance(s, TagGenreSource) for s in sources)


def test_tag_genre_source_id():
    assert TagGenreSource().source_id == "tags"


def _discogs400(activations, labels):
    """A Discogs400 source whose models return fixed per-label activations."""

    class Loader:
        def configure(self, **kwargs):
            pass

        def __call__(self):
            # 90 s of 16 kHz mono audio, as essentia's MonoLoader returns it.
            return np.zeros(16000 * 90, dtype=np.float32)

    source = Discogs400GenreSource()
    source._loader = Loader()
    source._embed_model = lambda snippet: np.ones((len(snippet) // 16000, 4), dtype=np.float32)
    source._class_model = lambda embeddings: np.tile(np.array(activations, dtype=np.float32), (len(embeddings), 1))
    source._labels = labels
    return source


def test_discogs400_predict_parses_snippet_activations():
    source = _discogs400([0.1, 0.8, 0.5], ["Rock---Punk", "Electronic---Techno", "Jazz"])

    predictions = source.predict("/music/track.flac")

    assert [(p.main_genre, p.sub_genre) for p in predictions] == [("Electronic", "Techno"), ("Jazz", "Jazz")]
    assert predictions[0].confidence == pytest.approx(0.8)
    assert all(p.source == "discogs400" for p in predictions)


def test_discogs400_predict_keeps_best_label_below_threshold():
    source = _discogs400([0.05, 0.18, 0.12], ["Rock---Punk", "Electronic---Trance", "Electronic---Psy-Trance"])

    predictions = source.predict("/music/track.flac")

    assert [(p.main_genre, p.sub_genre) for p in predictions] == [("Electronic", "Trance")]
    assert predictions[0].confidence == pytest.approx(0.18)
