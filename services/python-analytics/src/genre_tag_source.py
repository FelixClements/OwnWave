from tags import extract_genres_from_tags, read_tags

from models import GenrePrediction


class TagGenreSource:
    source_id = "tags"

    def predict(self, path: str) -> list[GenrePrediction]:
        return extract_genres_from_tags(read_tags(path))
