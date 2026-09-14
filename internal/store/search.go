package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/bfxavier/memory/internal/model"
)

const memorySelect = `
SELECT id, project_id, kind, state, content, confidence,
       source_agent, source_session_id, source_event_id, tags,
       supersedes_id, valid_until, created_at, updated_at, 0 AS score, rowid
FROM memories`

const (
	weightLexical    = 0.3
	weightConfidence = 0.2
	weightRecency    = 0.2
	weightProject    = 0.1
	recencyHalfLife  = 30 * 24 * time.Hour
	maxQueryTerms    = 16
	idLookupChunk    = 400
)

type scanner interface {
	Scan(...any) error
}

func (s *Store) Search(ctx context.Context, query string, options model.SearchOptions) ([]model.Memory, error) {
	terms := QueryTerms(query)
	if len(terms) == 0 {
		return []model.Memory{}, nil
	}
	matched, err := s.matchedTermCounts(ctx, terms, options)
	if err != nil {
		return nil, err
	}
	excluded, err := s.rowsForIDs(ctx, options.ExcludeIDs)
	if err != nil {
		return nil, err
	}
	rows := coveringRows(matched, excluded, len(terms), options.MinCoverage)
	if len(rows) == 0 {
		return []model.Memory{}, nil
	}
	candidates, err := s.memoriesByRow(ctx, terms, rows)
	if err != nil {
		return nil, err
	}
	for index := range candidates {
		candidates[index].Coverage = float64(matched[candidates[index].Row]) / float64(len(terms))
	}
	return rank(candidates, options, clampLimit(options.Limit)), nil
}

func (s *Store) matchedTermCounts(ctx context.Context, terms []string, options model.SearchOptions) (map[int64]int, error) {
	conditions := []string{"memories_fts MATCH ?", "m.state = 'active'"}
	scope := []any{}
	if options.ProjectID != "" {
		conditions = append(conditions, "(m.project_id = ? OR m.project_id = '')")
		scope = append(scope, options.ProjectID)
	}
	if len(options.Kinds) > 0 {
		conditions = append(conditions, "m.kind IN ("+placeholders(len(options.Kinds))+")")
		for _, kind := range options.Kinds {
			scope = append(scope, kind)
		}
	}
	statement := `
        SELECT m.rowid FROM memories_fts
        JOIN memories m ON m.rowid = memories_fts.rowid
        WHERE ` + strings.Join(conditions, " AND ")
	matched := map[int64]int{}
	for _, term := range terms {
		arguments := append([]any{`"` + term + `"`}, scope...)
		rows, err := s.db.QueryContext(ctx, statement, arguments...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var row int64
			if scanErr := rows.Scan(&row); scanErr != nil {
				rows.Close()
				return nil, scanErr
			}
			matched[row]++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return matched, nil
}

func (s *Store) rowsForIDs(ctx context.Context, ids []string) (map[int64]bool, error) {
	excluded := map[int64]bool{}
	for start := 0; start < len(ids); start += idLookupChunk {
		end := start + idLookupChunk
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		arguments := make([]any, len(chunk))
		for index, id := range chunk {
			arguments[index] = id
		}
		rows, err := s.db.QueryContext(ctx,
			`SELECT rowid FROM memories WHERE id IN (`+placeholders(len(chunk))+`)`, arguments...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var row int64
			if scanErr := rows.Scan(&row); scanErr != nil {
				rows.Close()
				return nil, scanErr
			}
			excluded[row] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return excluded, nil
}

func coveringRows(matched map[int64]int, excluded map[int64]bool, terms int, minCoverage float64) []int64 {
	required := 1
	if minCoverage > 0 {
		required = int(math.Ceil(minCoverage * float64(terms)))
		if required < 1 {
			required = 1
		}
	}
	rows := make([]int64, 0, len(matched))
	for row, count := range matched {
		if count < required || excluded[row] {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(first, second int) bool {
		if matched[rows[first]] != matched[rows[second]] {
			return matched[rows[first]] > matched[rows[second]]
		}
		return rows[first] > rows[second]
	})
	return rows
}

func (s *Store) memoriesByRow(ctx context.Context, terms []string, rows []int64) ([]model.Memory, error) {
	memories := []model.Memory{}
	for start := 0; start < len(rows); start += idLookupChunk {
		end := start + idLookupChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[start:end]
		arguments := make([]any, 0, len(chunk)+1)
		arguments = append(arguments, ftsMatch(terms))
		for _, row := range chunk {
			arguments = append(arguments, row)
		}
		result, err := s.db.QueryContext(ctx, `
        SELECT m.id, m.project_id, m.kind, m.state, m.content, m.confidence,
               m.source_agent, m.source_session_id, m.source_event_id, m.tags,
               m.supersedes_id, m.valid_until, m.created_at, m.updated_at,
               -bm25(memories_fts) AS score, m.rowid
        FROM memories_fts
        JOIN memories m ON m.rowid = memories_fts.rowid
        WHERE memories_fts MATCH ? AND m.rowid IN (`+placeholders(len(chunk))+`)`, arguments...)
		if err != nil {
			return nil, err
		}
		batch, err := scanMemories(result)
		result.Close()
		if err != nil {
			return nil, err
		}
		memories = append(memories, batch...)
	}
	return memories, nil
}

func placeholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
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

func (s *Store) CountActive(ctx context.Context, projectID string) (int, error) {
	arguments := []any{}
	conditions := []string{"state = 'active'"}
	if projectID != "" {
		conditions = append(conditions, "(project_id = ? OR project_id = '')")
		arguments = append(arguments, projectID)
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memories WHERE `+strings.Join(conditions, " AND "), arguments...)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
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

func rank(candidates []model.Memory, options model.SearchOptions, limit int) []model.Memory {
	maxLexical := 0.0
	for _, candidate := range candidates {
		if candidate.Score > maxLexical {
			maxLexical = candidate.Score
		}
	}
	now := time.Now()
	ranked := make([]model.Memory, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Coverage < options.MinCoverage {
			continue
		}
		candidate.Score = relevance(candidate, maxLexical, options.ProjectID, now)
		ranked = append(ranked, candidate)
	}
	sort.SliceStable(ranked, func(first, second int) bool {
		if ranked[first].Coverage != ranked[second].Coverage {
			return ranked[first].Coverage > ranked[second].Coverage
		}
		return ranked[first].Score > ranked[second].Score
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func relevance(memory model.Memory, maxLexical float64, requestedProject string, now time.Time) float64 {
	score := 0.0
	if maxLexical > 0 {
		score += weightLexical * clampUnit(memory.Score/maxLexical)
	}
	score += weightConfidence * clampUnit(memory.Confidence)
	score += weightRecency * decay(now.Sub(memory.UpdatedAt))
	if requestedProject != "" && memory.ProjectID == requestedProject {
		score += weightProject
	}
	return score
}

func decay(age time.Duration) float64 {
	if age <= 0 {
		return 1
	}
	return math.Exp(-math.Ln2 * float64(age) / float64(recencyHalfLife))
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
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
		&createdAt, &updatedAt, &memory.Score, &memory.Row,
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

func QueryTerms(value string) []string {
	seen := map[string]bool{}
	terms := []string{}
	for _, word := range tokenize(value) {
		if stopWords[word] || seen[word] {
			continue
		}
		seen[word] = true
		terms = append(terms, word)
		if len(terms) == maxQueryTerms {
			break
		}
	}
	return terms
}

func ftsMatch(terms []string) string {
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+term+`"`)
	}
	return strings.Join(quoted, " OR ")
}

func tokenize(value string) []string {
	words := strings.FieldsFunc(strings.ToLower(value), func(character rune) bool {
		return !(unicode.IsLetter(character) || unicode.IsDigit(character))
	})
	filtered := make([]string, 0, len(words))
	for _, word := range words {
		if word != "" {
			filtered = append(filtered, word)
		}
	}
	return filtered
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
