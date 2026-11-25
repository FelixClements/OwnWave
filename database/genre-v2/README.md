# Genre engine v2 switch migration (pending)

`genre_v2_switch.up.sql` and `.down.sql` are **generated** from
`services/python-analytics/src/genre_catalog.toml` by
`services/python-analytics/src/genre_catalog.py`. Do not edit them by hand.

They are kept out of `database/migrations/` on purpose: the new genre engine
replaces the old one in one release (ADR 0008, decision 6), so the catalog
change and the old → new genre id mapping must not reach the live database
before that release.

The up migration:

- adds `kind` (`genre` or `style`) and `parent_id` (a style's genre) to
  `station_genres`, and fills it with the genres and styles of the catalog;
- moves the v1 genre ids stored in `stations.genres`, `stations.auto_genre`
  and `genre_corrections.genre` to the new genres (each new genre's
  `replaces` list in the TOML file), keeping one genre station per genre;
- clears `station_suggestion_dismissals`.

`track_station_genres` is left to the new engine, which rewrites it.

## Workflow

1. Edit `genre_catalog.toml` (the list is a draft until it is approved in #138).
2. Regenerate: `python services/python-analytics/src/genre_catalog.py`.
   CI checks the files are current (`tests/test_genre_catalog.py`).
3. At the release, write the real migration with the next free number and
   delete the pending copies:
   `python services/python-analytics/src/genre_catalog.py --number NNN`.

## The engine follows the switch

The genre engine v2 (`services/python-analytics/src/genre_engine.py`, issue
#137) is wired in but only runs once this migration is in: `score_genres`
and the API's startup catalog sync check `station_genres.kind`
(`genre_engine.switched`). Until then the live app keeps the v1 blend. So
moving this file into `database/migrations/` at the release is the switch;
nothing else needs flipping.

Migration `036_genre_decisions` (already in `database/migrations/`, harmless
before the switch) adds where the engine stores its results:
`track_genre_decisions`, release-scope `genre_corrections.release_id`, and the
`genre_review_queue` view.

Before the release, run the engine in bulk on a copy of the database (#138):

    DATABASE_URL=<copy> PYTHONPATH=services/python-analytics/src \
        python services/python-analytics/src/main.py genre-engine [--write]

It prints counts, the classifier's test score and the agreement with the v1
engine as JSON. It is read only unless `--write`, which stores the decisions
and the test score but never `track_station_genres`.
