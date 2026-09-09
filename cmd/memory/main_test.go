package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/bfxavier/memory/internal/model"
)

func TestMemoryCommands(t *testing.T) {
	t.Setenv("MEMORY_HOME", t.TempDir())
	if err := run([]string{"init"}, bytes.NewReader(nil), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"remember", "--global", "--kind", "decision", "Use SQLite for recall"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var remembered model.Memory
	if err := json.Unmarshal(output.Bytes(), &remembered); err != nil {
		t.Fatal(err)
	}
	if remembered.Kind != "decision" || remembered.Content != "Use SQLite for recall" {
		t.Fatalf("unexpected memory: %#v", remembered)
	}

	output.Reset()
	if err := run([]string{"search", "--all", "SQLite"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var results []model.Memory
	if err := json.Unmarshal(output.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != remembered.ID {
		t.Fatalf("unexpected search results: %#v", results)
	}

	output.Reset()
	if err := run([]string{"correct", remembered.ID, "Use SQLite FTS5 for recall"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var corrected model.Memory
	if err := json.Unmarshal(output.Bytes(), &corrected); err != nil {
		t.Fatal(err)
	}
	if corrected.SupersedesID != remembered.ID {
		t.Fatalf("unexpected correction: %#v", corrected)
	}

	output.Reset()
	if err := run([]string{"forget", corrected.ID}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := run([]string{"inspect", corrected.ID}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var retracted model.Memory
	if err := json.Unmarshal(output.Bytes(), &retracted); err != nil {
		t.Fatal(err)
	}
	if retracted.State != "retracted" {
		t.Fatalf("memory state = %q", retracted.State)
	}
}

func TestProviderCommands(t *testing.T) {
	t.Setenv("MEMORY_HOME", t.TempDir())
	var output bytes.Buffer
	if err := run([]string{"provider", "host", "--codex-model", "luna-test", "--claude-model", "haiku-test"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := run([]string{"provider", "status"}, bytes.NewReader(nil), &output); err != nil {
		t.Fatal(err)
	}
	var extraction struct {
		Provider    string `json:"provider"`
		CodexModel  string `json:"codex_model"`
		ClaudeModel string `json:"claude_model"`
	}
	if err := json.Unmarshal(output.Bytes(), &extraction); err != nil {
		t.Fatal(err)
	}
	if extraction.Provider != "host" || extraction.CodexModel != "luna-test" || extraction.ClaudeModel != "haiku-test" {
		t.Fatalf("unexpected provider config: %#v", extraction)
	}
}
