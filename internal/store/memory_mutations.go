package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

func (s *Store) Remember(memory model.Memory) (model.Memory, error) {
	prepared, err := prepareMemory(memory, time.Now().UTC())
	if err != nil {
		return model.Memory{}, err
	}
	return writeMemory(s.db, prepared)
}

func (s *Store) Forget(id string) error {
	result, err := s.db.Exec(`UPDATE memories SET state = 'retracted', updated_at = ? WHERE id = ?`, time.Now().UTC().UnixMilli(), id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) Correct(id, replacement string) (model.Memory, error) {
	replacement = strings.TrimSpace(replacement)
	if replacement == "" {
		return model.Memory{}, errors.New("replacement content is required")
	}
	current, err := s.Get(id)
	if err != nil {
		return model.Memory{}, err
	}
	if fingerprint(current.ProjectID, replacement) == fingerprint(current.ProjectID, current.Content) {
		return model.Memory{}, errors.New("replacement is identical to the current memory")
	}

	transaction, err := s.db.Begin()
	if err != nil {
		return model.Memory{}, err
	}
	defer transaction.Rollback()
	now := time.Now().UTC()
	replacementMemory, err := prepareMemory(model.Memory{
		ProjectID:       current.ProjectID,
		Kind:            current.Kind,
		Content:         replacement,
		Confidence:      1,
		SourceAgent:     current.SourceAgent,
		SourceSessionID: current.SourceSessionID,
		SourceEventID:   current.SourceEventID,
		Tags:            current.Tags,
		SupersedesID:    current.ID,
	}, now)
	if err != nil {
		return model.Memory{}, err
	}
	written, err := writeMemory(transaction, replacementMemory)
	if err != nil {
		return model.Memory{}, err
	}
	if written.ID == current.ID {
		return model.Memory{}, errors.New("replacement resolved to the current memory")
	}
	if err := markSuperseded(transaction, current.ID, current.ProjectID, now); err != nil {
		return model.Memory{}, err
	}
	if err := transaction.Commit(); err != nil {
		return model.Memory{}, err
	}
	return written, nil
}

func (s *Store) Get(id string) (model.Memory, error) {
	return scanMemory(s.db.QueryRow(memorySelect+` WHERE id = ?`, id))
}
