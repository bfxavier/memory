package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/paths"
	"github.com/bfxavier/memory/internal/spool"
	"github.com/bfxavier/memory/internal/store"
)

func TestRunOnceExtractsCheckpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"choices":[{"message":{"role":"assistant","content":"{\"memories\":[{\"kind\":\"decision\",\"content\":\"Use automatic checkpoint extraction.\",\"confidence\":0.98,\"tags\":[\"extraction\"],\"source_event_id\":\"stop-1\"}]}"}}]}`)
	}))
	defer server.Close()
	home := t.TempDir()
	appPaths := paths.Paths{
		Home: home, Database: filepath.Join(home, "memory.db"), Config: filepath.Join(home, "config.json"),
		Spool: filepath.Join(home, "spool", "incoming"), Failed: filepath.Join(home, "spool", "failed"),
	}
	settings := config.Default()
	settings.Extraction.Provider = "openai"
	settings.Extraction.BaseURL = server.URL + "/v1"
	settings.Extraction.Model = "test"
	if err := config.Save(appPaths.Config, settings); err != nil {
		t.Fatal(err)
	}
	for index, event := range []model.Event{
		{ID: "prompt-1", Agent: "codex", SessionID: "session-1", EventName: "UserPromptSubmit", ProjectID: "project-a", ProjectRoot: home, Payload: json.RawMessage(`{"prompt":"implement extraction"}`)},
		{ID: "stop-1", Agent: "codex", SessionID: "session-1", EventName: "Stop", ProjectID: "project-a", ProjectRoot: home, Payload: json.RawMessage(`{"last_assistant_message":"implemented extraction"}`)},
	} {
		event.Version = 1
		event.CreatedAt = time.Unix(int64(index+1), 0).UTC()
		if err := spool.Append(appPaths.Spool, event); err != nil {
			t.Fatal(err)
		}
	}
	result, err := RunOnce(appPaths)
	if err != nil {
		t.Fatal(err)
	}
	if result.Events != 2 || result.Jobs != 1 || result.Memories != 1 || result.Failed != 0 {
		t.Fatalf("unexpected worker result: %#v", result)
	}
	database, err := store.Open(appPaths.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	memories, err := database.Search(context.Background(), "checkpoint extraction", model.SearchOptions{ProjectID: "project-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].SourceEventID != "stop-1" || memories[0].SourceAgent != "codex" {
		t.Fatalf("unexpected memories: %#v", memories)
	}
}
