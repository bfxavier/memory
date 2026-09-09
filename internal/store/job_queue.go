package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

const (
	maxExtractionAttempts = 5
	processingLease       = 10 * time.Minute
	attemptRetention      = 7 * 24 * time.Hour
)

var ErrExtractionRateLimited = errors.New("extraction hourly job limit reached")

func (s *Store) ClaimExtractionJob(ctx context.Context, maxJobsPerHour int) (model.ExtractionJob, []model.Event, error) {
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.ExtractionJob{}, nil, err
	}
	defer transaction.Rollback()
	now := time.Now().UTC()
	if err := enforceExtractionRateLimit(ctx, transaction, maxJobsPerHour, now); err != nil {
		return model.ExtractionJob{}, nil, err
	}
	job, err := claimExtractionJob(ctx, transaction, now)
	if err != nil {
		return model.ExtractionJob{}, nil, err
	}
	events, err := loadExtractionEvents(ctx, transaction, job.ID)
	if err != nil {
		return model.ExtractionJob{}, nil, err
	}
	if err := transaction.Commit(); err != nil {
		return model.ExtractionJob{}, nil, err
	}
	return job, events, nil
}

func (s *Store) FailExtractionJob(job model.ExtractionJob, failure error) error {
	state := "retry"
	if job.Attempts >= maxExtractionAttempts {
		state = "failed"
	}
	delay := time.Duration(1<<min(job.Attempts, 8)) * time.Second
	message := failure.Error()
	if len(message) > 1000 {
		message = message[:1000]
	}
	now := time.Now().UTC()
	_, err := s.db.Exec(`
        UPDATE extraction_jobs
        SET state = ?, next_attempt_at = ?, last_error = ?, updated_at = ?
        WHERE id = ?`, state, now.Add(delay).UnixMilli(), message, now.UnixMilli(), job.ID)
	return err
}

func (s *Store) RetryFailedExtractionJobs() (int64, error) {
	result, err := s.db.Exec(`
        UPDATE extraction_jobs
        SET state = 'pending', attempts = 0, next_attempt_at = 0, last_error = '', updated_at = ?
        WHERE state = 'failed'`, time.Now().UTC().UnixMilli())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func enforceExtractionRateLimit(ctx context.Context, transaction *sql.Tx, limit int, now time.Time) error {
	if _, err := transaction.ExecContext(ctx, `DELETE FROM extraction_attempts WHERE attempted_at < ?`, now.Add(-attemptRetention).UnixMilli()); err != nil {
		return err
	}
	var attempts int
	if err := transaction.QueryRowContext(ctx, `SELECT COUNT(*) FROM extraction_attempts WHERE attempted_at >= ?`, now.Add(-time.Hour).UnixMilli()).Scan(&attempts); err != nil {
		return err
	}
	if attempts >= limit {
		return ErrExtractionRateLimited
	}
	return nil
}

func claimExtractionJob(ctx context.Context, transaction *sql.Tx, now time.Time) (model.ExtractionJob, error) {
	row := transaction.QueryRowContext(ctx, `
        SELECT id, session_id, project_id, agent, checkpoint_event_id, attempts
        FROM extraction_jobs
        WHERE attempts < ? AND (
            state = 'pending' OR
            (state = 'retry' AND next_attempt_at <= ?) OR
            (state = 'processing' AND updated_at <= ?)
        )
        ORDER BY created_at
        LIMIT 1`, maxExtractionAttempts, now.UnixMilli(), now.Add(-processingLease).UnixMilli())
	var job model.ExtractionJob
	if err := row.Scan(&job.ID, &job.SessionID, &job.ProjectID, &job.Agent, &job.CheckpointEventID, &job.Attempts); err != nil {
		return model.ExtractionJob{}, err
	}
	result, err := transaction.ExecContext(ctx, `
        UPDATE extraction_jobs
        SET state = 'processing', attempts = attempts + 1, updated_at = ?
        WHERE id = ? AND attempts = ?`, now.UnixMilli(), job.ID, job.Attempts)
	if err != nil {
		return model.ExtractionJob{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return model.ExtractionJob{}, err
	}
	if changed != 1 {
		return model.ExtractionJob{}, sql.ErrNoRows
	}
	job.Attempts++
	if _, err := transaction.ExecContext(ctx, `INSERT INTO extraction_attempts(job_id, attempted_at) VALUES (?, ?)`, job.ID, now.UnixMilli()); err != nil {
		return model.ExtractionJob{}, err
	}
	return job, nil
}

func loadExtractionEvents(ctx context.Context, transaction *sql.Tx, jobID string) ([]model.Event, error) {
	rows, err := transaction.QueryContext(ctx, `
        SELECT id, agent, substr(session_id, instr(session_id, ':') + 1), event_name,
               project_id, '', created_at, payload
        FROM events
        WHERE session_id = (SELECT session_id FROM extraction_jobs WHERE id = ?)
          AND rowid > (SELECT previous_checkpoint_rowid FROM extraction_jobs WHERE id = ?)
          AND rowid <= (SELECT checkpoint_rowid FROM extraction_jobs WHERE id = ?)
        ORDER BY rowid`, jobID, jobID, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []model.Event{}
	for rows.Next() {
		var event model.Event
		var createdAt int64
		var payload string
		if err := rows.Scan(&event.ID, &event.Agent, &event.SessionID, &event.EventName,
			&event.ProjectID, &event.ProjectRoot, &createdAt, &payload); err != nil {
			return nil, err
		}
		event.Version = 1
		event.CreatedAt = time.UnixMilli(createdAt).UTC()
		event.Payload = json.RawMessage(payload)
		events = append(events, event)
	}
	return events, rows.Err()
}
