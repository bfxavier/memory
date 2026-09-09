package mcpserver

import (
	"time"

	"github.com/bfxavier/memory/internal/model"
)

type SearchInput struct {
	Query   string   `json:"query" jsonschema:"Search terms"`
	Project string   `json:"project,omitempty" jsonschema:"Project ID or local project path"`
	Kinds   []string `json:"kinds,omitempty" jsonschema:"Optional memory kinds"`
	Limit   int      `json:"limit,omitempty" jsonschema:"Maximum results, from 1 to 100"`
	All     bool     `json:"all,omitempty" jsonschema:"Search every project instead of the current project"`
}

type RecentInput struct {
	Project string `json:"project,omitempty" jsonschema:"Project ID or local project path"`
	Limit   int    `json:"limit,omitempty" jsonschema:"Maximum results, from 1 to 100"`
	All     bool   `json:"all,omitempty" jsonschema:"List memories from every project instead of the current project"`
}

type GetInput struct {
	ID string `json:"id" jsonschema:"Memory ID"`
}

type RememberInput struct {
	Content string   `json:"content" jsonschema:"Durable fact, decision, preference, procedure, failure, outcome, or note"`
	Project string   `json:"project,omitempty" jsonschema:"Project ID or local project path. Empty means the current project"`
	Kind    string   `json:"kind,omitempty" jsonschema:"Memory kind"`
	Tags    []string `json:"tags,omitempty" jsonschema:"Optional tags"`
	Global  bool     `json:"global,omitempty" jsonschema:"Store the memory for every project"`
}

type CorrectInput struct {
	ID          string `json:"id" jsonschema:"Memory ID to supersede"`
	Replacement string `json:"replacement" jsonschema:"Correct replacement content"`
}

type MutationOutput struct {
	Changed bool       `json:"changed"`
	Memory  MemoryView `json:"memory,omitempty"`
}

type MemoryOutput struct {
	Memory MemoryView `json:"memory"`
}

type MemoriesOutput struct {
	Memories []MemoryView `json:"memories"`
}

type MemoryView struct {
	ID              string   `json:"id"`
	ProjectID       string   `json:"project_id,omitempty"`
	Kind            string   `json:"kind"`
	State           string   `json:"state"`
	Content         string   `json:"content"`
	Confidence      float64  `json:"confidence"`
	SourceAgent     string   `json:"source_agent,omitempty"`
	SourceSessionID string   `json:"source_session_id,omitempty"`
	SourceEventID   string   `json:"source_event_id,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	SupersedesID    string   `json:"supersedes_id,omitempty"`
	ValidUntil      string   `json:"valid_until,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Score           float64  `json:"score,omitempty"`
}

func views(memories []model.Memory) []MemoryView {
	output := make([]MemoryView, 0, len(memories))
	for _, memory := range memories {
		output = append(output, view(memory))
	}
	return output
}

func view(memory model.Memory) MemoryView {
	result := MemoryView{
		ID:              memory.ID,
		ProjectID:       memory.ProjectID,
		Kind:            memory.Kind,
		State:           memory.State,
		Content:         memory.Content,
		Confidence:      memory.Confidence,
		SourceAgent:     memory.SourceAgent,
		SourceSessionID: memory.SourceSessionID,
		SourceEventID:   memory.SourceEventID,
		Tags:            memory.Tags,
		SupersedesID:    memory.SupersedesID,
		Score:           memory.Score,
	}
	if !memory.CreatedAt.IsZero() {
		result.CreatedAt = memory.CreatedAt.Format(time.RFC3339Nano)
	}
	if !memory.UpdatedAt.IsZero() {
		result.UpdatedAt = memory.UpdatedAt.Format(time.RFC3339Nano)
	}
	if memory.ValidUntil != nil {
		result.ValidUntil = memory.ValidUntil.Format(time.RFC3339Nano)
	}
	return result
}
