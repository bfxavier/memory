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
	"golang.org/x/text/unicode/norm"
)

const memorySelect = `
SELECT id, project_id, kind, state, content, confidence,
       source_agent, source_session_id, source_event_id, tags,
       supersedes_id, valid_until, created_at, updated_at, 0 AS score
FROM memories`

const (
	weightCoverage   = 1.0
	weightLexical    = 0.3
	weightConfidence = 0.2
	weightRecency    = 0.2
	weightProject    = 0.1
	recencyHalfLife  = 30 * 24 * time.Hour
	maxQueryTerms    = 16
)

type scanner interface {
	Scan(...any) error
}

func (s *Store) Search(ctx context.Context, query string, options model.SearchOptions) ([]model.Memory, error) {
	terms := QueryTerms(query)
	if len(terms) == 0 {
		return []model.Memory{}, nil
	}
	limit := clampLimit(options.Limit)
	arguments := []any{ftsMatch(terms)}
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
	arguments = append(arguments, candidateLimit(limit))
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
	candidates, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	return rank(candidates, terms, options, limit), nil
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

func rank(candidates []model.Memory, terms []string, options model.SearchOptions, limit int) []model.Memory {
	excluded := make(map[string]bool, len(options.ExcludeIDs))
	for _, id := range options.ExcludeIDs {
		excluded[id] = true
	}
	maxLexical := 0.0
	for _, candidate := range candidates {
		if candidate.Score > maxLexical {
			maxLexical = candidate.Score
		}
	}
	now := time.Now()
	ranked := make([]model.Memory, 0, len(candidates))
	for _, candidate := range candidates {
		if excluded[candidate.ID] {
			continue
		}
		matched := coverage(candidate, terms)
		if matched < options.MinCoverage {
			continue
		}
		candidate.Score = relevance(candidate, matched, maxLexical, now)
		ranked = append(ranked, candidate)
	}
	sort.SliceStable(ranked, func(first, second int) bool {
		return ranked[first].Score > ranked[second].Score
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	return ranked
}

func relevance(memory model.Memory, matched, maxLexical float64, now time.Time) float64 {
	score := weightCoverage * matched
	if maxLexical > 0 {
		score += weightLexical * clampUnit(memory.Score/maxLexical)
	}
	score += weightConfidence * clampUnit(memory.Confidence)
	score += weightRecency * decay(now.Sub(memory.UpdatedAt))
	if memory.ProjectID != "" {
		score += weightProject
	}
	return score
}

func coverage(memory model.Memory, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	haystack := tokenSet(memory.Content + " " + strings.Join(memory.Tags, " "))
	matched := 0
	for _, term := range terms {
		if haystack[term] {
			matched++
		}
	}
	return float64(matched) / float64(len(terms))
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
	words := strings.FieldsFunc(foldDiacritics(strings.ToLower(value)), func(character rune) bool {
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

func foldDiacritics(value string) string {
	decomposed := norm.NFD.String(value)
	folded := make([]rune, 0, len(decomposed))
	for _, character := range decomposed {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		folded = append(folded, character)
	}
	return norm.NFC.String(string(folded))
}

func tokenSet(value string) map[string]bool {
	set := map[string]bool{}
	for _, word := range tokenize(value) {
		set[word] = true
	}
	return set
}

func candidateLimit(limit int) int {
	candidates := limit * 6
	if candidates < 60 {
		candidates = 60
	}
	if candidates > 300 {
		candidates = 300
	}
	return candidates
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
