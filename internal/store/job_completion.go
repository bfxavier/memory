package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

func (s *Store) CompleteExtractionJob(job model.ExtractionJob, extracted []model.ExtractedMemory) (int, error) {
	transaction, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer transaction.Rollback()
	now := time.Now().UTC()
	sourceSessionID := externalSessionID(job.SessionID)
	superseded := make(map[string]bool, len(extracted))
	for _, candidate := range extracted {
		if err := storeExtractedMemory(transaction, job, sourceSessionID, candidate, now, superseded); err != nil {
			return 0, err
		}
	}
	result, err := transaction.Exec(`
        UPDATE extraction_jobs
        SET state = 'complete', last_error = '', updated_at = ?
        WHERE id = ? AND state = 'processing'`, now.UnixMilli(), job.ID)
	if err != nil {
		return 0, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, sql.ErrNoRows
	}
	if err := transaction.Commit(); err != nil {
		return 0, err
	}
	return len(extracted), nil
}

func storeExtractedMemory(transaction *sql.Tx, job model.ExtractionJob, sourceSessionID string, candidate model.ExtractedMemory, now time.Time, superseded map[string]bool) error {
	memory, err := prepareMemory(model.Memory{
		ProjectID:       job.ProjectID,
		Kind:            candidate.Kind,
		Content:         candidate.Content,
		Confidence:      candidate.Confidence,
		SourceAgent:     job.Agent,
		SourceSessionID: sourceSessionID,
		SourceEventID:   candidate.SourceEventID,
		Tags:            candidate.Tags,
		SupersedesID:    candidate.SupersedesID,
	}, now)
	if err != nil {
		return err
	}
	if candidate.SupersedesID != "" {
		if superseded[candidate.SupersedesID] {
			return fmt.Errorf("memory %s is superseded more than once", candidate.SupersedesID)
		}
		if err := validateSupersession(transaction, candidate.SupersedesID, job.ProjectID, fingerprint(job.ProjectID, memory.Content)); err != nil {
			return err
		}
	}
	written, err := writeMemory(transaction, memory)
	if err != nil {
		return err
	}
	if candidate.SupersedesID == "" {
		return nil
	}
	if written.ID == candidate.SupersedesID {
		return fmt.Errorf("memory %s cannot supersede itself", candidate.SupersedesID)
	}
	if err := markSuperseded(transaction, candidate.SupersedesID, job.ProjectID, now); err != nil {
		return err
	}
	superseded[candidate.SupersedesID] = true
	return nil
}

func externalSessionID(sessionID string) string {
	if _, externalID, found := strings.Cut(sessionID, ":"); found {
		return externalID
	}
	return sessionID
}
