package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

func TestMemoryLifecycleAndSearch(t *testing.T) {
	database, err := Open(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	first, err := database.Remember(model.Memory{
		ProjectID: "project-a",
		Kind:      "decision",
		Content:   "Use SQLite FTS5 for local recall",
	})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := database.Remember(model.Memory{
		ProjectID: "project-a",
		Kind:      "decision",
		Content:   "  Use  SQLite FTS5 for local recall  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ID != first.ID {
		t.Fatalf("duplicate ID = %q, want %q", duplicate.ID, first.ID)
	}
	if _, err := database.Remember(model.Memory{
		ProjectID: "project-b",
		Kind:      "decision",
		Content:   "Use Postgres for cloud storage",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Remember(model.Memory{
		Kind:    "preference",
		Content: "Never store credentials",
	}); err != nil {
		t.Fatal(err)
	}

	results, err := database.Search(context.Background(), "SQLite", model.SearchOptions{ProjectID: "project-a", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != first.ID {
		t.Fatalf("unexpected project search results: %#v", results)
	}
	global, err := database.Search(context.Background(), "credentials", model.SearchOptions{ProjectID: "project-a", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(global) != 1 || global[0].ProjectID != "" {
		t.Fatalf("unexpected global search results: %#v", global)
	}

	corrected, err := database.Correct(first.ID, "Use SQLite FTS5 before considering embeddings")
	if err != nil {
		t.Fatal(err)
	}
	if corrected.SupersedesID != first.ID || corrected.State != "active" {
		t.Fatalf("unexpected correction: %#v", corrected)
	}
	old, err := database.Get(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if old.State != "superseded" {
		t.Fatalf("old state = %q, want superseded", old.State)
	}
	if _, err := database.Correct(corrected.ID, corrected.Content); err == nil {
		t.Fatal("expected identical correction to fail")
	}
	stillActive, err := database.Get(corrected.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stillActive.State != "active" {
		t.Fatalf("identical correction changed state to %q", stillActive.State)
	}
	if err := database.Forget(corrected.ID); err != nil {
		t.Fatal(err)
	}
	forgotten, err := database.Get(corrected.ID)
	if err != nil {
		t.Fatal(err)
	}
	if forgotten.State != "retracted" {
		t.Fatalf("forgotten state = %q, want retracted", forgotten.State)
	}
	if err := database.Integrity(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestExtractionJobLifecycle(t *testing.T) {
	database, err := Open(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	for index, eventName := range []string{"UserPromptSubmit", "Stop"} {
		payload, _ := json.Marshal(map[string]any{"value": eventName})
		if err := database.InsertEvent(model.Event{
			ID: eventName, Agent: "codex", SessionID: "session-1", EventName: eventName,
			ProjectID: "project-a", ProjectRoot: t.TempDir(),
			CreatedAt: time.Unix(int64(index+1), 0), Payload: payload,
		}); err != nil {
			t.Fatal(err)
		}
	}
	job, events, err := database.ClaimExtractionJob(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if job.Attempts != 1 || len(events) != 2 {
		t.Fatalf("unexpected job or events: %#v %#v", job, events)
	}
	count, err := database.CompleteExtractionJob(job, []model.ExtractedMemory{{
		Kind: "outcome", Content: "The live recall test passed.", Confidence: 0.9,
		SourceEventID: "Stop",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stored %d memories", count)
	}
	memories, err := database.Search(context.Background(), "recall", model.SearchOptions{ProjectID: "project-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].SourceSessionID != "session-1" || memories[0].SourceEventID != "Stop" {
		t.Fatalf("unexpected extracted memory: %#v", memories)
	}
	if _, _, err := database.ClaimExtractionJob(context.Background(), 12); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("claim after completion: %v", err)
	}
}

func TestExtractionSupersedesActiveMemory(t *testing.T) {
	database, err := Open(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	old, err := database.Remember(model.Memory{
		ProjectID: "project-a", Kind: "fact", Content: "Automatic extraction is disabled.",
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"value": "enabled"})
	if err := database.InsertEvent(model.Event{
		ID: "stop-1", Agent: "codex", SessionID: "session-1", EventName: "Stop",
		ProjectID: "project-a", CreatedAt: time.Unix(2, 0), Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}
	job, _, err := database.ClaimExtractionJob(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.CompleteExtractionJob(job, []model.ExtractedMemory{{
		Kind: "fact", Content: "Automatic extraction is enabled.", Confidence: 1,
		SourceEventID: "stop-1", SupersedesID: old.ID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	previous, err := database.Get(old.ID)
	if err != nil {
		t.Fatal(err)
	}
	if previous.State != "superseded" || previous.ValidUntil == nil {
		t.Fatalf("old memory was not superseded: %#v", previous)
	}
	active, err := database.Search(context.Background(), "automatic extraction", model.SearchOptions{ProjectID: "project-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].SupersedesID != old.ID || active[0].Content != "Automatic extraction is enabled." {
		t.Fatalf("unexpected active memories: %#v", active)
	}
}

func TestInvalidMemoryDoesNotWrite(t *testing.T) {
	database, err := Open(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Remember(model.Memory{Kind: "guess", Content: "maybe"}); err == nil {
		t.Fatal("expected invalid kind error")
	}
	stats, err := database.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Memories != 0 {
		t.Fatalf("memories = %d, want 0", stats.Memories)
	}
}
