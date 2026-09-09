package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"github.com/bfxavier/memory/internal/model"
)

const memorySelect = `
SELECT id, project_id, kind, state, content, confidence,
       source_agent, source_session_id, source_event_id, tags,
       supersedes_id, valid_until, created_at, updated_at, 0 AS score
FROM memories`

type scanner interface {
	Scan(...any) error
}

func (s *Store) Search(ctx context.Context, query string, options model.SearchOptions) ([]model.Memory, error) {
	match := ftsQuery(query)
	if match == "" {
		return s.Recent(ctx, options)
	}
	arguments := []any{match}
	conditions := []string{"memories_fts MATCH ?", "m.state = 'active'"}
	if options.ProjectID != "" {
		conditions = append(conditions, "(m.project_id = ? OR m.project_id = '')")
		arguments = append(arguments, options.ProjectID)
	}
	if len(options.Kinds) > 0 {
		placeholders := make([]string, len(options.Kinds))
		for index, kind := range options.Kinds {
			placeholders[index] = "?"
			arguments = append(arguments, kind)
		}
		conditions = append(conditions, "m.kind IN ("+strings.Join(placeholders, ",")+")")
	}
	arguments = append(arguments, clampLimit(options.Limit))
	rows, err := s.db.QueryContext(ctx, `
        SELECT m.id, m.project_id, m.kind, m.state, m.content, m.confidence,
               m.source_agent, m.source_session_id, m.source_event_id, m.tags,
               m.supersedes_id, m.valid_until, m.created_at, m.updated_at,
               -bm25(memories_fts) AS score
        FROM memories_fts
        JOIN memories m ON m.rowid = memories_fts.rowid
        WHERE `+strings.Join(conditions, " AND ")+`
        ORDER BY bm25(memories_fts), m.updated_at DESC
        LIMIT ?`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (s *Store) Recent(ctx context.Context, options model.SearchOptions) ([]model.Memory, error) {
	arguments := []any{}
	conditions := []string{"state = 'active'"}
	if options.ProjectID != "" {
		conditions = append(conditions, "(project_id = ? OR project_id = '')")
		arguments = append(arguments, options.ProjectID)
	}
	arguments = append(arguments, clampLimit(options.Limit))
	rows, err := s.db.QueryContext(ctx, memorySelect+`
        WHERE `+strings.Join(conditions, " AND ")+`
        ORDER BY updated_at DESC
        LIMIT ?`, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (s *Store) ActiveProject(ctx context.Context, projectID string, limit int) ([]model.Memory, error) {
	rows, err := s.db.QueryContext(ctx, memorySelect+`
        WHERE state = 'active' AND project_id = ?
        ORDER BY updated_at DESC
        LIMIT ?`, projectID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (s *Store) All(ctx context.Context) ([]model.Memory, error) {
	rows, err := s.db.QueryContext(ctx, memorySelect+` ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func scanMemory(row scanner) (model.Memory, error) {
	var memory model.Memory
	var tags string
	var validUntil sql.NullInt64
	var createdAt, updatedAt int64
	err := row.Scan(
		&memory.ID, &memory.ProjectID, &memory.Kind, &memory.State, &memory.Content,
		&memory.Confidence, &memory.SourceAgent, &memory.SourceSessionID,
		&memory.SourceEventID, &tags, &memory.SupersedesID, &validUntil,
		&createdAt, &updatedAt, &memory.Score,
	)
	if err != nil {
		return model.Memory{}, err
	}
	if err := json.Unmarshal([]byte(tags), &memory.Tags); err != nil {
		return model.Memory{}, err
	}
	memory.CreatedAt = time.UnixMilli(createdAt).UTC()
	memory.UpdatedAt = time.UnixMilli(updatedAt).UTC()
	if validUntil.Valid {
		value := time.UnixMilli(validUntil.Int64).UTC()
		memory.ValidUntil = &value
	}
	return memory, nil
}

func scanMemories(rows *sql.Rows) ([]model.Memory, error) {
	memories := []model.Memory{}
	for rows.Next() {
		memory, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

func ftsQuery(value string) string {
	words := strings.FieldsFunc(value, func(character rune) bool {
		return !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_')
	})
	if len(words) > 16 {
		words = words[:16]
	}
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		if word != "" {
			quoted = append(quoted, `"`+word+`"`)
		}
	}
	return strings.Join(quoted, " OR ")
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 100 {
		return 100
	}
	return limit
}
