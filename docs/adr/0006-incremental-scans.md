# 0006: Incremental library scans

Status: accepted, 2026-11-22. Issue #117, map #116. Migrations come with
#119 (`track_analysis_failures`) and #120 (`scan_jobs.rerun_requested`).

## Context

Python (`services/python-analytics`) builds `tracks` from the music folder
with a library scan. Two things start one:

- **The file watcher** (`watcher.py`) queues a scan once a new file has been
  quiet for 60 s (at most 900 s after the first change, then a 30 s
  cooldown).
- **SoulLite** calls `POST /api/admin/scan` after each sync (ADR 0003). Go
  proxies it to Python's `POST /scan`, which defaults to Python's own
  `MUSIC_DIR` (#115).

Every scan does the whole library's work, however little changed. Measured
in the live worker logs on 2026-11-22 (09:28–09:35 UTC): 3 scans in about
5 minutes while SoulLite downloaded 3 songs (7,175 → 7,178 tracks). Each
scan took **about 95 s and ran about 5,900 `process_audio_folder` tasks**
for one new song.

The time goes here:

1. **Every folder, every time.** `trigger_library_scan` (`tasks.py`) lists
   every music folder and creates a child job and a task for each one,
   changed or not.
2. **Downstream work always runs.** `finalize_scan` walks the tree again to
   prune, then always queues `rebuild_analysis`, which queues MusicBrainz and
   `score_genres` over all tracks, then sends NOTIFY, which starts the Go
   pipeline.

The same code has four side problems:

- **No retention.** `scan_jobs` and `scan_job_progress` rows are never
  deleted. Each scan adds about 5,900 child rows to each table.
- **Failures repeat.** A file whose analysis fails has no features row, so
  `_import_file` (`folder_importer.py`) analyses it again on every scan. The
  same happens when the genre model returns nothing. Seen live:
  `01 - Shout.flac` logs "All analyzers failed" on every scan.
- **A change during a running scan is lost.** `queue_library_scan` returns
  the running job to a new request. A file that changes after its folder's
  task ran is missed until something else changes.
- **Downstream work runs even when nothing changed** (point 2 above).

## Decision

There is still **one kind of scan, always over the whole library**. It is
made cheap instead of being split.

1. **Diff first** (#118). A planning step replaces the per-folder fan-out:
   - **One walk.** `library_scan/plan.py` walks the root once with
     `os.scandir` and records `{path: (size, mtime)}` for every supported
     audio file.
   - **One query.** It loads every track under the root with its size,
     mtime, whether it has features, and its audio genre model version.
   - **Which folders.** A folder is imported only if one of its files is
     new, has a different size or mtime, has no features, or has an
     outdated genre model version, or if the scan uses `force`.
   - **Tasks carry their plan.** Each `process_audio_folder` task gets its
     file list in its args, so `import_folder` does not list the folder or
     read stats again.
   - **Early stop.** When no folder needs importing and no track is missing
     on disk, the master job completes at once: no child jobs, no tasks, no
     `finalize_scan`.
2. **Skip downstream work when nothing changed** (#118). In `finalize_scan`,
   "changed" means at least one track was imported, analysed, moved or
   deleted (the folder results plus prune). If nothing changed, the master
   job completes without `rebuild_analysis` and without NOTIFY.
3. **`score_genres` stays a full-library rescore.** When it runs, it
   rescores every track (about 2 s). Copies (#111, #112) and genre
   corrections are per song, so rescoring only the touched tracks would give
   wrong results.
4. **Remember analysis failures** (#119). A new Python-owned table,
   `track_analysis_failures (track_id, kind, model_version, error,
   failed_at)`, keyed by `(track_id, kind)` with `kind` in `features` or
   `genre`, and `ON DELETE CASCADE` from `tracks`.
   - A row is written when feature analysis fails, or when the genre model
     returns nothing. Genre rows store the model version.
   - The planning step skips analysis with a failure on record: any
     `features` row, or a `genre` row for the current model version.
   - A track's rows are deleted when its size or mtime changes. A new model
     version retries genre failures. `force` ignores all failures.
5. **Coalesce requests** (#120). A new column,
   `scan_jobs.rerun_requested BOOLEAN NOT NULL DEFAULT false`.
   `queue_library_scan` then:
   - returns a pending scan as it is;
   - sets `rerun_requested` on a running scan and returns it;
   - creates a new scan only when none is active.

   When a scan finishes (in `finalize_scan` or the early stop) with
   `rerun_requested` set, it queues exactly one follow-up scan, however many
   requests came in.
6. **Retention** (#120). At the end of each scan:
   - the summed child progress is written to the master's
     `scan_job_progress` row;
   - the child jobs are deleted (their progress rows go with them through
     the cascade);
   - master jobs beyond the newest 100 are deleted.

   The same migration cleans up existing rows the same way.

### Why not partial scans

A partial scan (only the watcher's folder, or only SoulLite's new files)
looks cheaper, but prune cannot work from it. ADR 0005's contract says
Python keeps a track's id when its file moves. `match_moved_tracks`
(`library_scan/prune.py`) pairs a track whose file vanished with a track
first seen in this scan. It can only tell "moved" from "deleted" when it sees
the full listing: a partial scan of the new folder does not know the old file
is gone, and a partial scan of the old folder does not know where the file
went. With two scan kinds, prune would need two code paths, and the one that
gets moves wrong would detach listening history, likes and copies.

With the diff first, a full scan costs one walk and one query, so the full
listing stays and there is one prune path. A moved file still keeps its
track id: its new folder is imported (new file), and prune matches it to the
vanished row as before.

## Consequences

- **Expected cost (⚠ estimates, not measured):** about 5–10 s for one new
  song, and about 1–3 s for a scan that finds nothing to do. #118 logs the
  walk and query times and records them on the real `/music` in #116.
- **A scan with nothing to do is nearly free**, so the watcher and SoulLite
  can both keep asking for scans. Neither trigger changes.
- **A failed file is analysed once**, then again only after the file
  changes, the model version changes, or a `force` scan. Until #119 lands,
  failed files are still retried on every scan.
- **A change during a running scan is picked up** by one follow-up scan.
- **`scan_jobs` stays small**: at most 100 master jobs, and child rows only
  while a scan runs.
- **The admin scan view must read master totals for finished scans** once
  child rows are deleted. Progress while a scan runs still uses the children.
  #120 checks `apps/web/src/lib/scan.ts` and the Python jobs API.
- **Two new Python-owned schema items**, `track_analysis_failures` and
  `scan_jobs.rerun_requested`, are listed under "Planned" in ADR 0005.
- **The watcher's timings stay** (60 s quiet, 900 s max wait, 30 s cooldown).
  Map #116 decided they become env settings; that has no ticket yet.

### Open questions

- **Is `/music` a network mount?** That decides whether the walk stays at
  1–3 s. #118 measures it.
- **Worker memory.** The worker used 9.9 GB on 2026-11-22. Whether the
  models are loaded once per worker process is out of scope here; it gets its
  own ticket if it matters.
