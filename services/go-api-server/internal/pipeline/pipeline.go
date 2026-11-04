// Package pipeline keeps stations up to date with the library. After a
// library scan or genre scoring (the analytics worker says so with a
// Postgres NOTIFY), after playlist changes, at startup and every hour, it
// matches playlist tracks to local files and refreshes the genre stations.
//
// NOTIFY only wakes the runner: the state it acts on is all in the
// database, so a missed notice is caught up by the next run.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ownwave/api/internal/playlists"
	"ownwave/api/internal/stations"
)

// Channel is the NOTIFY channel the analytics worker posts to. The
// payload names the step that finished, e.g. scan_finished or genres_scored.
const Channel = "ownwave_pipeline"

// Triggers the API itself uses.
const (
	TriggerStartup   = "startup"
	TriggerReconnect = "reconnect"
	TriggerHourly    = "hourly"
	TriggerAdmin     = "admin"
	TriggerSettings  = "settings"
	TriggerAdded     = "playlist_added"
	TriggerSynced    = "playlist_synced"
	TriggerDeleted   = "playlist_deleted"
)

// keepRuns bounds the run log. Runs that changed nothing are dropped
// except the newest, which says when the pipeline last ran.
const keepRuns = 50

var triggerPattern = regexp.MustCompile(`^[a-z][a-z_]{0,39}$`)

// Run is one pass of the pipeline as the admin view shows it.
type Run struct {
	ID         int64      `json:"id"`
	Trigger    string     `json:"trigger"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	// Matched counts playlist tracks newly matched to a local file; nil
	// when the run did not match.
	Matched *int              `json:"matched,omitempty"`
	Changes []stations.Change `json:"changes"`
	Error   *string           `json:"error,omitempty"`
}

// Matcher matches playlist tracks without a local file.
type Matcher interface {
	MatchPending(ctx context.Context) (int, error)
}

// Refresher brings the genre stations in line with the library.
type Refresher interface {
	Refresh(ctx context.Context) ([]stations.Change, error)
}

type Runner struct {
	pool    *pgxpool.Pool
	matcher Matcher
	refresh Refresher
	kick    chan string
}

func New(pool *pgxpool.Pool, matcher *playlists.Service, refresh *stations.Service) *Runner {
	return &Runner{pool: pool, matcher: matcher, refresh: refresh, kick: make(chan string, 1)}
}

// Run matches pending playlist tracks (when match is set), refreshes the
// genre stations and logs the run. Concurrent runs are serialised by the
// refresh's advisory lock.
func (r *Runner) Run(ctx context.Context, trigger string, match bool) (Run, error) {
	run := Run{Trigger: trigger, StartedAt: time.Now(), Changes: []stations.Change{}}
	var err error
	if match {
		var n int
		if n, err = r.matcher.MatchPending(ctx); err == nil {
			run.Matched = &n
		}
	}
	if err == nil {
		run.Changes, err = r.refresh.Refresh(ctx)
		if run.Changes == nil {
			run.Changes = []stations.Change{}
		}
	}
	now := time.Now()
	run.FinishedAt = &now
	if err != nil {
		msg := err.Error()
		run.Error = &msg
	}
	if logErr := r.record(ctx, &run); logErr != nil && err == nil {
		err = logErr
	}
	if len(run.Changes) > 0 || (run.Matched != nil && *run.Matched > 0) {
		slog.Info("station refresh", "trigger", trigger, "matched", run.Matched, "changes", len(run.Changes))
	}
	return run, err
}

func (r *Runner) record(ctx context.Context, run *Run) error {
	changes, err := json.Marshal(run.Changes)
	if err != nil {
		return err
	}
	// The log outlives a cancelled request.
	ctx = context.WithoutCancel(ctx)
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO station_refresh_runs (trigger, started_at, finished_at, matched, changes, error)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id
	`, run.Trigger, run.StartedAt, run.FinishedAt, run.Matched, changes, run.Error).Scan(&run.ID); err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		DELETE FROM station_refresh_runs
		WHERE id < $1 AND changes = '[]'::jsonb AND error IS NULL AND COALESCE(matched, 0) = 0
	`, run.ID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		DELETE FROM station_refresh_runs
		WHERE id <= (SELECT id FROM station_refresh_runs ORDER BY id DESC OFFSET $1 LIMIT 1)
	`, keepRuns)
	return err
}

// Runs returns the newest runs first.
func (r *Runner) Runs(ctx context.Context, limit int) ([]Run, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, trigger, started_at, finished_at, matched, changes, error
		FROM station_refresh_runs ORDER BY id DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Run, error) {
		var run Run
		var changes []byte
		if err := row.Scan(&run.ID, &run.Trigger, &run.StartedAt, &run.FinishedAt, &run.Matched, &changes, &run.Error); err != nil {
			return run, err
		}
		err := json.Unmarshal(changes, &run.Changes)
		return run, err
	})
}

// Trigger asks the background loop for a run. Triggers that arrive while
// one is already waiting fold into it.
func (r *Runner) Trigger(trigger string) {
	select {
	case r.kick <- trigger:
	default:
	}
}

// Start runs the background loop and listens on Channel until ctx ends.
// The first connection triggers a startup run, each reconnect a catch-up run.
func (r *Runner) Start(ctx context.Context, dsn string) {
	go r.work(ctx)
	go r.listen(ctx, dsn)
}

func (r *Runner) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case trigger := <-r.kick:
			if _, err := r.Run(ctx, trigger, true); err != nil && ctx.Err() == nil {
				slog.Error("station refresh failed", "trigger", trigger, "error", err)
			}
		}
	}
}

func (r *Runner) listen(ctx context.Context, dsn string) {
	backoff := time.Second
	first := true
	for ctx.Err() == nil {
		trigger := TriggerReconnect
		if first {
			trigger = TriggerStartup
		}
		err := r.listenOnce(ctx, dsn, func() { first = false; backoff = time.Second; r.Trigger(trigger) })
		if ctx.Err() != nil {
			return
		}
		slog.Warn("pipeline listener disconnected", "error", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, time.Minute)
	}
}

// listenOnce listens until the connection fails; listening calls back once
// LISTEN is in place, so nothing sent after the catch-up run is missed.
func (r *Runner) listenOnce(ctx context.Context, dsn string, listening func()) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(context.WithoutCancel(ctx))
	if _, err := conn.Exec(ctx, "LISTEN "+Channel); err != nil {
		return err
	}
	listening()
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		r.Trigger(NotifyTrigger(n.Payload))
	}
}

// NotifyTrigger names the run a NOTIFY payload starts.
func NotifyTrigger(payload string) string {
	if triggerPattern.MatchString(payload) {
		return payload
	}
	return "notify"
}

// Scan is the newest library scan.
type Scan struct {
	ID         string     `json:"id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
}

// GenreScoring is the newest genre scoring run of the analytics worker.
type GenreScoring struct {
	At     time.Time `json:"at"`
	Tracks int       `json:"tracks"`
	Scored int       `json:"scored"`
}

// Status is the admin view of the pipeline: its steps' last runs, and the
// station refreshes newest first.
type Status struct {
	LastScan     *Scan         `json:"last_scan"`
	GenresScored *GenreScoring `json:"genres_scored"`
	Runs         []Run         `json:"runs"`
}

// Status reports the last scan, the last genre scoring and up to limit
// station refreshes.
func (r *Runner) Status(ctx context.Context, limit int) (Status, error) {
	var out Status
	var scan Scan
	err := r.pool.QueryRow(ctx, `
		SELECT id::text, status, created_at, finished_at, error FROM scan_jobs
		WHERE parent_id IS NULL ORDER BY created_at DESC LIMIT 1
	`).Scan(&scan.ID, &scan.Status, &scan.CreatedAt, &scan.FinishedAt, &scan.Error)
	switch {
	case err == nil:
		out.LastScan = &scan
	case !errors.Is(err, pgx.ErrNoRows):
		return out, err
	}
	var scoring GenreScoring
	err = r.pool.QueryRow(ctx, `
		SELECT updated_at, COALESCE((value->>'tracks')::int, 0), COALESCE((value->>'scored')::int, 0)
		FROM app_state WHERE key = 'genres_scored'
	`).Scan(&scoring.At, &scoring.Tracks, &scoring.Scored)
	switch {
	case err == nil:
		out.GenresScored = &scoring
	case !errors.Is(err, pgx.ErrNoRows):
		return out, err
	}
	out.Runs, err = r.Runs(ctx, limit)
	return out, err
}
