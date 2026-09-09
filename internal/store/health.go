package store

import (
	"context"
	"errors"

	"github.com/bfxavier/memory/internal/model"
)

func (s *Store) Stats(ctx context.Context) (model.Stats, error) {
	var stats model.Stats
	tables := []struct {
		name  string
		value *int64
	}{
		{"projects", &stats.Projects},
		{"sessions", &stats.Sessions},
		{"events", &stats.Events},
		{"memories", &stats.Memories},
	}
	for _, table := range tables {
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table.name).Scan(table.value); err != nil {
			return model.Stats{}, err
		}
	}
	states := []struct {
		name  string
		value *int64
	}{
		{"pending", &stats.JobsPending},
		{"retry", &stats.JobsPending},
		{"processing", &stats.JobsProcessing},
		{"failed", &stats.JobsFailed},
	}
	for _, state := range states {
		var count int64
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM extraction_jobs WHERE state = ?`, state.name).Scan(&count); err != nil {
			return model.Stats{}, err
		}
		*state.value += count
	}
	return stats, nil
}

func (s *Store) Integrity(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return errors.New(result)
	}
	return nil
}
