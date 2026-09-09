package store

import (
	"database/sql"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

func (s *Store) UpsertProject(project model.Project) error {
	now := time.Now().UTC().UnixMilli()
	_, err := s.db.Exec(`
        INSERT INTO projects(id, identity, root, remote, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            identity = excluded.identity,
            root = excluded.root,
            remote = excluded.remote,
            updated_at = excluded.updated_at`,
		project.ID, project.Identity, project.Root, project.Remote, now, now)
	return err
}

func (s *Store) InsertEvent(event model.Event) error {
	transaction, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()

	createdAt := event.CreatedAt.UTC().UnixMilli()
	if err := upsertEventProject(transaction, event, createdAt); err != nil {
		return err
	}
	sessionID := event.Agent + ":" + event.SessionID
	if err := upsertSession(transaction, sessionID, event, createdAt); err != nil {
		return err
	}
	inserted, err := insertEvent(transaction, sessionID, event, createdAt)
	if err != nil {
		return err
	}
	if inserted && isCheckpoint(event.EventName) {
		if err := createExtractionJob(transaction, sessionID, event, createdAt); err != nil {
			return err
		}
	}
	return transaction.Commit()
}

func upsertEventProject(transaction *sql.Tx, event model.Event, createdAt int64) error {
	_, err := transaction.Exec(`
        INSERT INTO projects(id, identity, root, remote, created_at, updated_at)
        VALUES (?, ?, ?, '', ?, ?)
        ON CONFLICT(id) DO UPDATE SET root = excluded.root, updated_at = excluded.updated_at`,
		event.ProjectID, event.ProjectID, event.ProjectRoot, createdAt, createdAt)
	return err
}

func upsertSession(transaction *sql.Tx, sessionID string, event model.Event, createdAt int64) error {
	_, err := transaction.Exec(`
        INSERT INTO sessions(id, agent, external_id, project_id, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET project_id = excluded.project_id, updated_at = excluded.updated_at`,
		sessionID, event.Agent, event.SessionID, event.ProjectID, createdAt, createdAt)
	return err
}

func insertEvent(transaction *sql.Tx, sessionID string, event model.Event, createdAt int64) (bool, error) {
	result, err := transaction.Exec(`
        INSERT INTO events(id, session_id, project_id, agent, event_name, payload, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO NOTHING`,
		event.ID, sessionID, event.ProjectID, event.Agent, event.EventName, string(event.Payload), createdAt)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func createExtractionJob(transaction *sql.Tx, sessionID string, event model.Event, createdAt int64) error {
	var checkpointRowID int64
	if err := transaction.QueryRow(`SELECT rowid FROM events WHERE id = ?`, event.ID).Scan(&checkpointRowID); err != nil {
		return err
	}
	var previousRowID int64
	if err := transaction.QueryRow(`
        SELECT COALESCE(MAX(checkpoint_rowid), 0)
        FROM extraction_jobs
        WHERE session_id = ? AND checkpoint_rowid < ?`, sessionID, checkpointRowID).Scan(&previousRowID); err != nil {
		return err
	}
	_, err := transaction.Exec(`
        INSERT INTO extraction_jobs(
            id, session_id, project_id, agent, checkpoint_event_id,
            checkpoint_rowid, previous_checkpoint_rowid, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, sessionID, event.ProjectID, event.Agent, event.ID,
		checkpointRowID, previousRowID, createdAt, createdAt)
	return err
}

func isCheckpoint(eventName string) bool {
	switch eventName {
	case "Stop", "Interrupt", "SessionEnd", "PreCompact", "SubagentStop":
		return true
	default:
		return false
	}
}
