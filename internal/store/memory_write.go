package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bfxavier/memory/internal/model"
	"github.com/google/uuid"
)

var validKinds = map[string]bool{
	"decision":   true,
	"fact":       true,
	"preference": true,
	"procedure":  true,
	"failure":    true,
	"outcome":    true,
	"note":       true,
}

type memoryExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

func prepareMemory(memory model.Memory, now time.Time) (model.Memory, error) {
	memory.Content = strings.TrimSpace(memory.Content)
	if memory.Content == "" {
		return model.Memory{}, errors.New("memory content is required")
	}
	if memory.Kind == "" {
		memory.Kind = "note"
	}
	if !validKinds[memory.Kind] {
		return model.Memory{}, fmt.Errorf("invalid memory kind %q", memory.Kind)
	}
	if memory.ID == "" {
		memory.ID = uuid.NewString()
	}
	if memory.State == "" {
		memory.State = "active"
	}
	if memory.Confidence == 0 {
		memory.Confidence = 1
	}
	if memory.CreatedAt.IsZero() {
		memory.CreatedAt = now
	}
	memory.UpdatedAt = now
	return memory, nil
}

func writeMemory(executor memoryExecutor, memory model.Memory) (model.Memory, error) {
	tags, err := json.Marshal(memory.Tags)
	if err != nil {
		return model.Memory{}, err
	}
	fingerprint := fingerprint(memory.ProjectID, memory.Content)
	_, err = executor.Exec(`
        INSERT INTO memories(
            id, project_id, kind, state, content, confidence, source_agent,
            source_session_id, source_event_id, tags, fingerprint, supersedes_id,
            valid_until, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(project_id, fingerprint) DO UPDATE SET
            state = 'active',
            kind = excluded.kind,
            confidence = MAX(memories.confidence, excluded.confidence),
            tags = excluded.tags,
            supersedes_id = CASE
                WHEN excluded.supersedes_id != '' THEN excluded.supersedes_id
                ELSE memories.supersedes_id
            END,
            updated_at = excluded.updated_at`,
		memory.ID, memory.ProjectID, memory.Kind, memory.State, memory.Content,
		memory.Confidence, memory.SourceAgent, memory.SourceSessionID,
		memory.SourceEventID, string(tags), fingerprint, memory.SupersedesID,
		nullTime(memory.ValidUntil), memory.CreatedAt.UnixMilli(), memory.UpdatedAt.UnixMilli())
	if err != nil {
		return model.Memory{}, err
	}
	return scanMemory(executor.QueryRow(memorySelect+` WHERE project_id = ? AND fingerprint = ?`, memory.ProjectID, fingerprint))
}

func markSuperseded(executor memoryExecutor, id, projectID string, now time.Time) error {
	result, err := executor.Exec(`
        UPDATE memories
        SET state = 'superseded', valid_until = ?, updated_at = ?
        WHERE id = ? AND project_id = ? AND state = 'active'`,
		now.UnixMilli(), now.UnixMilli(), id, projectID)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("memory %s was not superseded", id)
	}
	return nil
}

func validateSupersession(transaction *sql.Tx, id, projectID, replacementFingerprint string) error {
	var state, currentFingerprint string
	if err := transaction.QueryRow(`SELECT state, fingerprint FROM memories WHERE id = ? AND project_id = ?`, id, projectID).Scan(&state, &currentFingerprint); err != nil {
		return fmt.Errorf("load superseded memory %s: %w", id, err)
	}
	if state != "active" {
		return fmt.Errorf("memory %s is not active", id)
	}
	if currentFingerprint == replacementFingerprint {
		return fmt.Errorf("memory %s cannot supersede itself", id)
	}
	return nil
}

func fingerprint(projectID, content string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	sum := sha256.Sum256([]byte(projectID + "\x00" + normalized))
	return hex.EncodeToString(sum[:])
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().UnixMilli()
}
