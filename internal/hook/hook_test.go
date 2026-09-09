package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/paths"
	"github.com/bfxavier/memory/internal/project"
	"github.com/bfxavier/memory/internal/spool"
	"github.com/bfxavier/memory/internal/store"
)

func TestPromptCaptureAndRecall(t *testing.T) {
	appPaths := testPaths(t)
	if err := appPaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(appPaths.Database)
	if err != nil {
		t.Fatal(err)
	}
	resolved := project.Resolve(t.TempDir())
	memory, err := database.Remember(model.Memory{
		ProjectID: resolved.ID,
		Kind:      "decision",
		Content:   "Use SQLite FTS5 for recall",
	})
	if err != nil {
		t.Fatal(err)
	}
	database.Close()

	input := hookInput("UserPromptSubmit", resolved.Root, map[string]any{"prompt": "Which SQLite search should we use?"})
	var output bytes.Buffer
	Execute("codex", "UserPromptSubmit", bytes.NewReader(input), &output, appPaths)
	var response struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.HookSpecificOutput.HookEventName != "UserPromptSubmit" {
		t.Fatalf("event name = %q", response.HookSpecificOutput.HookEventName)
	}
	if !strings.Contains(response.HookSpecificOutput.AdditionalContext, memory.ID) {
		t.Fatalf("context does not contain memory ID: %s", response.HookSpecificOutput.AdditionalContext)
	}

	entries, err := os.ReadDir(appPaths.Spool)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("spool entries = %d, want 1", len(entries))
	}
}

func TestSessionStartRendersActiveDigest(t *testing.T) {
	appPaths := testPaths(t)
	if err := appPaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(appPaths.Database)
	if err != nil {
		t.Fatal(err)
	}
	resolved := project.Resolve(t.TempDir())
	old, err := database.Remember(model.Memory{
		ProjectID: resolved.ID, Kind: "decision", Content: "Use the old provider",
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := database.Correct(old.ID, "Use the host subscription provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.Remember(model.Memory{
		ProjectID: resolved.ID, Kind: "failure", Content: "The old provider required another API key",
	}); err != nil {
		t.Fatal(err)
	}
	database.Close()

	var output bytes.Buffer
	Execute("codex", "SessionStart", bytes.NewReader(hookInput("SessionStart", resolved.Root, nil)), &output, appPaths)
	var response struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(output.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	contextText := response.HookSpecificOutput.AdditionalContext
	if !strings.Contains(contextText, "Decisions:\n") || !strings.Contains(contextText, "Failures:\n") || !strings.Contains(contextText, current.ID) {
		t.Fatalf("missing digest groups: %s", contextText)
	}
	if strings.Contains(contextText, old.Content) {
		t.Fatalf("digest contains superseded memory: %s", contextText)
	}
}

func TestMalformedAndFailedHooksStaySilent(t *testing.T) {
	appPaths := testPaths(t)
	appPaths.Spool = filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(appPaths.Spool, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	Execute("codex", "PostToolUse", strings.NewReader(`{"bad"`), &output, appPaths)
	if output.Len() != 0 {
		t.Fatalf("malformed PostToolUse output = %q", output.String())
	}

	output.Reset()
	Execute("codex", "Stop", strings.NewReader(`{"session_id":"s","cwd":"."}`), &output, appPaths)
	if output.String() != "{}" {
		t.Fatalf("failed Stop output = %q, want {}", output.String())
	}
}

func TestSecretsAndLargeOutputAreSanitized(t *testing.T) {
	appPaths := testPaths(t)
	input := hookInput("PostToolUse", t.TempDir(), map[string]any{
		"authorization": "Bearer secret-value-1234567890",
		"tool_input":    map[string]any{"file_path": ".env"},
		"tool_response": "DATABASE_PASSWORD=do-not-store\n" + strings.Repeat("x", maxStringBytes*2),
		"screenshot":    strings.Repeat("a", 1000),
	})
	Execute("claude", "PostToolUse", bytes.NewReader(input), io.Discard, appPaths)
	var captured model.Event
	count, err := spool.Consume(appPaths.Spool, appPaths.Failed, func(event model.Event) error {
		captured = event
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("consumed %d events, want 1", count)
	}
	payload := string(captured.Payload)
	if strings.Contains(payload, "secret-value") || strings.Contains(payload, "do-not-store") || strings.Contains(payload, "screenshot") {
		t.Fatalf("sensitive payload was retained: %s", payload)
	}
	if !strings.Contains(payload, "[REDACTED]") {
		t.Fatalf("payload was not redacted: %s", payload)
	}
}

func TestLargeOutputIsTruncated(t *testing.T) {
	appPaths := testPaths(t)
	input := hookInput("PostToolUse", t.TempDir(), map[string]any{
		"tool_input":    map[string]any{"command": "generate output"},
		"tool_response": strings.Repeat("x", maxStringBytes*2),
	})
	Execute("claude", "PostToolUse", bytes.NewReader(input), io.Discard, appPaths)
	var captured model.Event
	_, err := spool.Consume(appPaths.Spool, appPaths.Failed, func(event model.Event) error {
		captured = event
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(captured.Payload), "[TRUNCATED]") {
		t.Fatalf("payload was not truncated: %s", captured.Payload)
	}
}

func TestMissingDatabaseIsNotCreatedByHook(t *testing.T) {
	appPaths := testPaths(t)
	input := hookInput("SessionStart", t.TempDir(), nil)
	Execute("codex", "SessionStart", bytes.NewReader(input), io.Discard, appPaths)
	if _, err := os.Stat(appPaths.Database); !os.IsNotExist(err) {
		t.Fatalf("database was created by hook: %v", err)
	}
}

func TestEmptyProjectSessionStartReturnsMarker(t *testing.T) {
	appPaths := testPaths(t)
	if err := appPaths.Ensure(); err != nil {
		t.Fatal(err)
	}
	database, err := store.Open(appPaths.Database)
	if err != nil {
		t.Fatal(err)
	}
	database.Close()
	var output bytes.Buffer
	input := hookInput("SessionStart", t.TempDir(), nil)
	Execute("codex", "SessionStart", bytes.NewReader(input), &output, appPaths)
	if !strings.Contains(output.String(), "No active memories are stored for this project yet.") {
		t.Fatalf("missing empty-project marker: %s", output.String())
	}
}

func TestExtractorProcessDoesNotCaptureRecursively(t *testing.T) {
	t.Setenv("MEMORY_EXTRACTOR", "1")
	appPaths := testPaths(t)
	input := hookInput("Stop", t.TempDir(), nil)
	var output bytes.Buffer
	Execute("codex", "Stop", bytes.NewReader(input), &output, appPaths)
	if output.String() != "{}" {
		t.Fatalf("Stop output = %q", output.String())
	}
	entries, err := os.ReadDir(appPaths.Spool)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("captured %d recursive events", len(entries))
	}
}

func testPaths(t *testing.T) paths.Paths {
	t.Helper()
	home := t.TempDir()
	return paths.Paths{
		Home: home, Database: filepath.Join(home, "memory.db"),
		Spool: filepath.Join(home, "spool", "incoming"), Failed: filepath.Join(home, "spool", "failed"),
	}
}

func hookInput(eventName, cwd string, extra map[string]any) []byte {
	value := map[string]any{"session_id": "session-1", "hook_event_name": eventName, "cwd": cwd}
	for key, item := range extra {
		value[key] = item
	}
	data, _ := json.Marshal(value)
	return data
}
