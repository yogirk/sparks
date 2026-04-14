package manifest

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// IngestStatus is the three-state lifecycle of an ingest row. "in_progress"
// blocks a concurrent `sparks ingest --prepare`; "completed" is the success
// path; "aborted" lets callers record recovery without deleting history.
type IngestStatus string

const (
	IngestInProgress IngestStatus = "in_progress"
	IngestCompleted  IngestStatus = "completed"
	IngestAborted    IngestStatus = "aborted"
)

// IngestRecord mirrors the ingests table.
type IngestRecord struct {
	ID          int64
	StartedAt   time.Time
	FinalizedAt *time.Time
	Status      IngestStatus
	EntryCount  int
	CommitSHA   string
}

// ErrIngestInProgress is returned from OpenIngest if another prepare is
// already active. Callers should either abort the stale one or retry.
var ErrIngestInProgress = errors.New("manifest: another ingest is already in_progress")

// ErrNoActiveIngest is returned from CompleteIngest if there's no matching
// in_progress row — typically means `--finalize` was called without
// `--prepare` first.
var ErrNoActiveIngest = errors.New("manifest: no in_progress ingest to finalize")

// CurrentIngest returns the active in_progress row, or ErrNotFound if none.
func (d *DB) CurrentIngest() (IngestRecord, error) {
	row := d.sql.QueryRow(
		`SELECT id, started_at, finalized_at, status, COALESCE(entry_count, 0), COALESCE(commit_sha, '')
		   FROM ingests WHERE status = ?
		  ORDER BY id DESC LIMIT 1`,
		string(IngestInProgress),
	)
	return scanIngest(row)
}

// OpenIngest inserts a new in_progress row. Returns ErrIngestInProgress if
// one is already active — callers must either finalize or abort the
// existing row before starting a new one.
func (d *DB) OpenIngest() (IngestRecord, error) {
	if cur, err := d.CurrentIngest(); err == nil {
		return cur, ErrIngestInProgress
	} else if !errors.Is(err, ErrNotFound) {
		return IngestRecord{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := d.sql.Exec(
		`INSERT INTO ingests(started_at, status) VALUES(?, ?)`,
		now, string(IngestInProgress),
	)
	if err != nil {
		return IngestRecord{}, fmt.Errorf("open ingest: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return IngestRecord{}, err
	}
	started, _ := time.Parse(time.RFC3339Nano, now)
	return IngestRecord{
		ID:        id,
		StartedAt: started,
		Status:    IngestInProgress,
	}, nil
}

// CompleteIngest marks the in_progress row as completed and records the
// entry count + commit SHA. Returns ErrNoActiveIngest if no in_progress
// row matches id (typically means it was aborted or never existed).
func (d *DB) CompleteIngest(id int64, entryCount int, commitSHA string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := d.sql.Exec(
		`UPDATE ingests
		    SET status = ?, finalized_at = ?, entry_count = ?, commit_sha = ?
		  WHERE id = ? AND status = ?`,
		string(IngestCompleted), now, entryCount, commitSHA,
		id, string(IngestInProgress),
	)
	if err != nil {
		return fmt.Errorf("complete ingest %d: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNoActiveIngest
	}
	return nil
}

// AbortIngest flips any in_progress row to aborted. Safe to call even
// when no row is active (returns nil).
func (d *DB) AbortIngest(id int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.sql.Exec(
		`UPDATE ingests
		    SET status = ?, finalized_at = ?
		  WHERE id = ? AND status = ?`,
		string(IngestAborted), now,
		id, string(IngestInProgress),
	)
	if err != nil {
		return fmt.Errorf("abort ingest %d: %w", id, err)
	}
	return nil
}

func scanIngest(row *sql.Row) (IngestRecord, error) {
	var (
		rec           IngestRecord
		startedAt     string
		finalizedNS   sql.NullString
		status        string
	)
	err := row.Scan(&rec.ID, &startedAt, &finalizedNS, &status, &rec.EntryCount, &rec.CommitSHA)
	if err == sql.ErrNoRows {
		return IngestRecord{}, ErrNotFound
	}
	if err != nil {
		return IngestRecord{}, err
	}
	rec.Status = IngestStatus(status)
	rec.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	if finalizedNS.Valid {
		t, _ := time.Parse(time.RFC3339Nano, finalizedNS.String)
		rec.FinalizedAt = &t
	}
	return rec, nil
}
