package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPMemoryLifecycle(t *testing.T) {
	ctx := context.Background()
	database, err := store.Open(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	server := New(database, "test")
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	toolNames := map[string]bool{}
	for tool, err := range clientSession.Tools(ctx, nil) {
		if err != nil {
			t.Fatal(err)
		}
		toolNames[tool.Name] = true
	}
	for _, name := range []string{"memory_search", "memory_recent", "memory_get", "memory_remember", "memory_forget", "memory_correct"} {
		if !toolNames[name] {
			t.Fatalf("tool %q was not registered", name)
		}
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "memory_remember",
		Arguments: map[string]any{
			"content": "Use deterministic local search",
			"project": "project-a",
			"kind":    "decision",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("remember returned an MCP error: %#v", result.Content)
	}
	memories, err := database.Search(ctx, "deterministic", model.SearchOptions{ProjectID: "project-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 {
		t.Fatalf("stored memories = %d, want 1", len(memories))
	}
	remembered := memories[0]

	result, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "memory_search",
		Arguments: map[string]any{"query": "deterministic", "project": "project-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(result.Content) == 0 {
		t.Fatalf("search returned no content: %#v", result)
	}

	result, err = clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "memory_forget",
		Arguments: map[string]any{"id": remembered.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("forget returned an MCP error: %#v", result.Content)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var forgotten MutationOutput
	if err := json.Unmarshal(encoded, &forgotten); err != nil {
		t.Fatal(err)
	}
	if !forgotten.Changed || forgotten.Memory.ID != remembered.ID || forgotten.Memory.Content != remembered.Content || forgotten.Memory.State != "retracted" {
		t.Fatalf("unexpected forget response: %#v", forgotten)
	}
}
