# Plan: First-Time Setup Wizard

**One-sentence summary:** Build a mandatory, database-backed first-time setup flow that creates the first admin account, scans the library, reports what was found, generates one station per selected main genre with normalized names, and ensures every track belongs to at least one station.

---

## 1. Goal

When OwnWave is opened for the first time, the user is guided through a single wizard. At the end:

- A first admin account exists.
- The configured music folder has been scanned.
- The user sees what was found: track count, main genres, sub-genres, and failed files.
- Main-genre stations are generated with clean names (e.g. `Electronic`, not `Genre: Electronic / Ambient`).
- An `Uncategorized` station is created if any track could not be placed.
- **Every track is in at least one station.**
- A `setup_completed` flag is persisted so the wizard never runs again.

## 2. Decisions from user feedback

| Question | Decision |
|---|---|
| Skip / advanced mode | **No** — the wizard is mandatory on first run. |
| Account setup | **Yes** — create the first admin user during setup if none exists. |
| Audio quality / playback settings | **No** for v1. FLAC only, no extra settings. |
| Import listening history | **No**. |
| Onboarding tip cards | **No**. |
| Persist setup state | **Yes** — store `setup_completed` in the database, not React state. |
| Station name casing | **Normalize to title case** (`Electronic`, `Hip Hop`, `Rock`). |
| Unassigned tracks | **Automatically** place them in an `Uncategorized` station. |
| Sub-genre stations | **Hidden** for the first setup; main genres only. |

## 3. Non-goals (for this pass)

- No optional "advanced" skip.
- No playback-quality or stream-format settings.
- No import of external history.
- No sub-genre auto-stations until the user explicitly creates them later.
- No onboarding tour or tooltips.

## 4. Schema changes

### 4.1 `app_state` table

A small key-value table for global state. First key: `setup_completed`.

```sql
CREATE TABLE app_state (
    key TEXT PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4.2 `users` table (if not already present)

Store the first admin account. Existing `adminodin` account can be migrated into this table on first run, or the wizard creates a new account.

```sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## 5. Backend work

### 5.1 Database helpers

In `services/python-analytics/src/db.py`:

- `get_app_state(conn, key) -> dict | None`
- `set_app_state(conn, key, value)`
- `count_tracks(conn) -> int`
- `count_users(conn) -> int` (or use existing auth check)
- `get_tracks_without_station(conn) -> List[UUID]`

### 5.2 Python API endpoints

In `services/python-analytics/src/api.py`:

- `GET /setup/status`
  - Returns `setup_completed`, `has_users`, `track_count`.
- `POST /setup/account`
  - Body: `{ username, password }`.
  - Creates the first admin user if `count_users() == 0`.
- `POST /setup/scan`
  - Triggers `trigger_library_scan.delay(MUSIC_DIR)`.
  - Returns `{ job_id, status }`.
- `GET /setup/scan/{job_id}`
  - Reuses the existing `GET /jobs/{job_id}` job status + stats.
- `GET /setup/summary`
  - Returns the post-scan summary: total tracks, main-genre counts, sub-genre counts, failed paths, tracks without a station.
- `POST /setup/stations`
  - Body: `{ selected_main_genres: List[str] }`.
  - Creates one station per selected main genre with a clean, title-cased name.
  - Creates an `Uncategorized` station for any track that is not covered.
  - Returns `{ created: int, uncovered: int }`.
- `POST /setup/complete`
  - Sets `app_state.setup_completed = true`.

### 5.3 Station creation rules

- Build the list of main genres from `track_genres` grouped by `main_genre`, counting distinct tracks.
- Pre-select main genres that already have `>= GENRE_MIN_TRACKS_PER_STATION` tracks (default 5).
- Station name = `toTitleCase(main_genre)`.
- Station `seed_type` = `genre` with `main_genre` filter.
- If a track has no `track_genres` rows, or its main genre is not selected, add it to `Uncategorized`.
- The `Uncategorized` station uses `seed_type = any` or a track-id list of the unassigned tracks.

### 5.4 Go API

In `services/go-api-server/handlers.go` and `main.go`:

- Proxy the above Python `/setup/*` routes under `GET/POST /setup/*`.
- Use the existing auth middleware once the account step is done.

## 6. Frontend work

### 6.1 New page: `/setup`

A single `app/setup/page.tsx` that renders a multi-step wizard. The page is reachable directly and is used by the root layout to redirect first-run users.

### 6.2 Step components (no code yet)

1. **Step 0 — Redirect guard**
   - On mount, call `trpc.setupStatus.useQuery()`.
   - If `setup_completed == true`, redirect to `/library`.

2. **Step 1 — Account creation**
   - If `has_users == false`, show username/password form.
   - If `has_users == true`, show a confirmation step.

3. **Step 2 — Library scan**
   - Show `MUSIC_DIR` and a "Start scan" button.
   - On click, call `trpc.setupScan.useMutation()`.
   - Show the existing `ScanStatusPanel` with live polling.
   - Block the Next button until `status == 'completed'`.

4. **Step 3 — What we found**
   - Total tracks.
   - Main-genre list with track counts.
   - Sub-genre count (informational only; not selectable here).
   - Failed files with paths and errors.
   - Tracks not in any station (will be covered in the next step).

5. **Step 4 — Pick main-genre stations**
   - Checklist of main genres with track counts.
   - Pre-selected if count >= threshold.
   - Show station name preview: `Electronic`, `Rock`, etc.
   - Disable "Create stations" if none selected.

6. **Step 5 — Coverage report**
   - After stations are created, show:
     - Created stations and track counts.
     - Number of tracks moved to `Uncategorized`.
   - Confirm that `uncovered == 0`.

7. **Step 6 — Done**
   - "Setup complete" message.
   - Button to go to `/stations/manage` or `/library`.
   - Call `trpc.setupComplete.useMutation()` to set the flag.

### 6.3 tRPC procedures

In `apps/t3-web-frontend/server/routers/app.ts`:

- `setupStatus` → `GET /setup/status`
- `setupCreateAccount` → `POST /setup/account`
- `setupScan` → `POST /setup/scan`
- `setupScanStatus` → `GET /setup/scan/{job_id}`
- `setupSummary` → `GET /setup/summary`
- `setupStations` → `POST /setup/stations`
- `setupComplete` → `POST /setup/complete`

In `apps/t3-web-frontend/lib/api.ts`:

- Corresponding `fetch`/`post` helpers.

### 6.4 Root layout guard

In `apps/t3-web-frontend/app/layout.tsx` or `app/page.tsx`:

- On app load, check `setup_completed`.
- If `false`, redirect to `/setup`.
- After login, the login page should also respect this guard.

## 7. UX / copy details

- Station naming:
  - Input: `main_genre` string.
  - Output: `toTitleCase(main_genre)`.
  - Examples:
    - `electronic` → `Electronic`
    - `hip hop` → `Hip Hop`
    - `rock` → `Rock`
    - `dance-p	hop` → `Dance-Pop`
- `Uncategorized` is always created when needed, even if the user did not select it.
- Sub-genres are shown only as a read-only count in Step 3.
- The user cannot create sub-genre stations from the wizard.

## 8. Verification plan

1. Start with a fresh database.
2. Open the app; confirm redirect to `/setup`.
3. Create first account; confirm `has_users` becomes true.
4. Start scan; confirm live status panel works.
5. After scan, confirm Step 3 shows correct counts.
6. In Step 4, confirm main genres are pre-selected by threshold.
7. Create stations; confirm names are title-cased and no `Genre:` prefix.
8. Confirm `Uncategorized` is created for unassigned tracks.
9. Query `tracks` and `station_tracks`; confirm 100% track coverage.
10. Complete setup; confirm `setup_completed` is set and the user lands on the main app.

## 9. Open questions (only if they change the plan)

1. Should the `Uncategorized` station always be visible in `stations/manage`, or hidden from the main list?
2. Should failed files be re-scanned automatically at the end of the wizard, or left for a later manual rescan?
