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
