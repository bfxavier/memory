package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/model"
)

func TestMain(main *testing.M) {
	if os.Getenv("MEMORY_HOST_TEST_HELPER") == "1" {
		hostTestHelper()
		os.Exit(0)
	}
	os.Exit(main.Run())
}

func TestExtractValidatedMemories(t *testing.T) {
	active := []model.Memory{{ID: "memory-1", Kind: "outcome", Content: "Claude live recall failed."}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", request.URL.Path)
		}
		var input chatRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		if len(input.Messages) != 2 || !strings.Contains(input.Messages[1].Content, "MEMORY memory-1") {
			t.Errorf("active memory missing from extraction input: %#v", input.Messages)
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{
			"role":    "assistant",
			"content": `{"memories":[{"kind":"outcome","content":"Claude live recall passed.","confidence":0.95,"tags":["claude"],"source_event_id":"event-2","supersedes_id":"memory-1"}]}`,
		}}}}
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer server.Close()
	settings := config.Default().Extraction
	settings.Enabled = true
	settings.BaseURL = server.URL + "/v1"
	settings.Model = "test"
	memories, err := New(settings).Extract(context.Background(), testEvents(), active)
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].Kind != "outcome" || memories[0].SourceEventID != "event-2" || memories[0].SupersedesID != "memory-1" {
		t.Fatalf("unexpected memories: %#v", memories)
	}
}

func TestExtractRejectsUnknownProvenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"choices":[{"message":{"role":"assistant","content":"{\"memories\":[{\"kind\":\"fact\",\"content\":\"bad\",\"confidence\":1,\"source_event_id\":\"missing\"}]}"}}]}`)
	}))
	defer server.Close()
	settings := config.Default().Extraction
	settings.Enabled = true
	settings.BaseURL = server.URL
	settings.Model = "test"
	if _, err := New(settings).Extract(context.Background(), testEvents(), nil); err == nil {
		t.Fatal("expected provenance validation error")
	}
}

func TestExtractRejectsUnknownSupersession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(writer, `{"choices":[{"message":{"role":"assistant","content":"{\"memories\":[{\"kind\":\"fact\",\"content\":\"new\",\"confidence\":1,\"tags\":[],\"source_event_id\":\"event-2\",\"supersedes_id\":\"missing\"}]}"}}]}`)
	}))
	defer server.Close()
	settings := config.Default().Extraction
	settings.BaseURL = server.URL
	settings.Model = "test"
	if _, err := New(settings).Extract(context.Background(), testEvents(), nil); err == nil {
		t.Fatal("expected supersession validation error")
	}
}

func TestRenderEventsPreservesChronologyAndBudget(t *testing.T) {
	rendered, ids := renderEvents(testEvents(), 4096)
	if strings.Index(rendered, "event-1") > strings.Index(rendered, "event-2") {
		t.Fatalf("events are not chronological: %s", rendered)
	}
	if !ids["event-1"] || !ids["event-2"] {
		t.Fatalf("missing IDs: %#v", ids)
	}
}

func TestHostExtractors(t *testing.T) {
	t.Setenv("MEMORY_HOST_TEST_HELPER", "1")
	settings := config.Default().Extraction
	settings.CodexCommand = os.Args[0]
	settings.ClaudeCommand = os.Args[0]
	client := NewHost(settings)
	for _, agent := range []string{"codex", "claude"} {
		memories, err := client.Extract(context.Background(), agent, testEvents(), nil)
		if err != nil {
			t.Fatalf("%s extraction: %v", agent, err)
		}
		if len(memories) != 1 || memories[0].SourceEventID != "event-2" {
			t.Fatalf("%s memories: %#v", agent, memories)
		}
	}
}

func hostTestHelper() {
	response := `{"memories":[{"kind":"outcome","content":"Host extraction passed.","confidence":0.9,"tags":[],"source_event_id":"event-2","supersedes_id":""}]}`
	for index, argument := range os.Args {
		if argument == "--output-last-message" && index+1 < len(os.Args) {
			_ = os.WriteFile(os.Args[index+1], []byte(response), 0o600)
			return
		}
	}
	fmt.Print(response)
}

func testEvents() []model.Event {
	return []model.Event{
		{ID: "event-1", EventName: "UserPromptSubmit", CreatedAt: time.Unix(1, 0), Payload: json.RawMessage(`{"prompt":"test it"}`)},
		{ID: "event-2", EventName: "Stop", CreatedAt: time.Unix(2, 0), Payload: json.RawMessage(`{"last_assistant_message":"Claude live recall passed"}`)},
	}
}
