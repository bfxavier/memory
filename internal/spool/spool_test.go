package spool

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

func TestConcurrentAppendAndIdempotentConsume(t *testing.T) {
	directory := t.TempDir()
	incoming := directory + "/incoming"
	failed := directory + "/failed"
	const total = 200
	var wait sync.WaitGroup
	errors := make(chan error, total)
	for index := 0; index < total; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			payload, _ := json.Marshal(map[string]any{"index": index})
			event := model.Event{
				Version: 1, ID: fmt.Sprintf("event-%03d", index), Agent: "codex",
				SessionID: "shared", EventName: "PostToolUse", ProjectID: "project",
				ProjectRoot: directory, CreatedAt: time.Unix(0, int64(index)).UTC(), Payload: payload,
			}
			if err := Append(incoming, event); err != nil {
				errors <- err
			}
		}(index)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	count, err := Consume(incoming, failed, func(event model.Event) error {
		if seen[event.ID] {
			t.Fatalf("duplicate event %s", event.ID)
		}
		seen[event.ID] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != total || len(seen) != total {
		t.Fatalf("processed %d events and saw %d, want %d", count, len(seen), total)
	}
	count, err = Consume(incoming, failed, func(model.Event) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("second consume processed %d events, want 0", count)
	}
}

func TestOversizedEventIsRejected(t *testing.T) {
	payload, err := json.Marshal(string(make([]byte, MaxEventBytes)))
	if err != nil {
		t.Fatal(err)
	}
	event := model.Event{ID: "large", Payload: payload}
	if err := Append(t.TempDir(), event); err == nil {
		t.Fatal("expected oversized event error")
	}
}
