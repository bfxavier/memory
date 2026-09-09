package model

import (
	"encoding/json"
	"time"
)

type Project struct {
	ID       string `json:"id"`
	Identity string `json:"identity"`
	Root     string `json:"root"`
	Remote   string `json:"remote,omitempty"`
}

type Event struct {
	Version     int             `json:"version"`
	ID          string          `json:"id"`
	Agent       string          `json:"agent"`
	SessionID   string          `json:"session_id"`
	EventName   string          `json:"event_name"`
	ProjectID   string          `json:"project_id"`
	ProjectRoot string          `json:"project_root"`
	CreatedAt   time.Time       `json:"created_at"`
	Payload     json.RawMessage `json:"payload"`
}

type Memory struct {
	ID              string     `json:"id"`
	ProjectID       string     `json:"project_id,omitempty"`
	Kind            string     `json:"kind"`
	State           string     `json:"state"`
	Content         string     `json:"content"`
	Confidence      float64    `json:"confidence"`
	SourceAgent     string     `json:"source_agent,omitempty"`
	SourceSessionID string     `json:"source_session_id,omitempty"`
	SourceEventID   string     `json:"source_event_id,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
	SupersedesID    string     `json:"supersedes_id,omitempty"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Score           float64    `json:"score,omitempty"`
}

type SearchOptions struct {
	ProjectID string
	Kinds     []string
	Limit     int
}

type Stats struct {
	Projects       int64 `json:"projects"`
	Sessions       int64 `json:"sessions"`
	Events         int64 `json:"events"`
	Memories       int64 `json:"memories"`
	JobsPending    int64 `json:"jobs_pending"`
	JobsProcessing int64 `json:"jobs_processing"`
	JobsFailed     int64 `json:"jobs_failed"`
}

type ExtractionJob struct {
	ID                string `json:"id"`
	SessionID         string `json:"session_id"`
	ProjectID         string `json:"project_id"`
	Agent             string `json:"agent"`
	CheckpointEventID string `json:"checkpoint_event_id"`
	Attempts          int    `json:"attempts"`
}

type ExtractedMemory struct {
	Kind          string   `json:"kind"`
	Content       string   `json:"content"`
	Confidence    float64  `json:"confidence"`
	Tags          []string `json:"tags"`
	SourceEventID string   `json:"source_event_id"`
	SupersedesID  string   `json:"supersedes_id,omitempty"`
}
