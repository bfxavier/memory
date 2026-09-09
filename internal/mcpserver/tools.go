package mcpserver

import (
	"context"

	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerReadTools(server *mcp.Server, database *store.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_search", Description: "Search durable local memory using full-text relevance.", Annotations: readOnly("Search memory"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input SearchInput) (*mcp.CallToolResult, MemoriesOutput, error) {
		projectID := currentProjectID(input.Project)
		if input.All {
			projectID = ""
		}
		memories, err := database.Search(ctx, input.Query, model.SearchOptions{
			ProjectID: projectID, Kinds: input.Kinds, Limit: input.Limit,
		})
		return nil, MemoriesOutput{Memories: views(memories)}, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_recent", Description: "List the most recently updated active memories.", Annotations: readOnly("Recent memories"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input RecentInput) (*mcp.CallToolResult, MemoriesOutput, error) {
		projectID := currentProjectID(input.Project)
		if input.All {
			projectID = ""
		}
		memories, err := database.Recent(ctx, model.SearchOptions{ProjectID: projectID, Limit: input.Limit})
		return nil, MemoriesOutput{Memories: views(memories)}, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_get", Description: "Get one memory with provenance and state.", Annotations: readOnly("Get memory"),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input GetInput) (*mcp.CallToolResult, MemoryOutput, error) {
		memory, err := database.Get(input.ID)
		return nil, MemoryOutput{Memory: view(memory)}, err
	})
}

func registerWriteTools(server *mcp.Server, database *store.Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_remember", Description: "Store an explicit durable memory locally.", Annotations: writeOnly("Remember", false),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input RememberInput) (*mcp.CallToolResult, MutationOutput, error) {
		projectID := currentProjectID(input.Project)
		if input.Global {
			projectID = ""
		}
		memory, err := database.Remember(model.Memory{
			ProjectID: projectID, Kind: input.Kind, Content: input.Content, Confidence: 1, Tags: input.Tags,
		})
		return nil, MutationOutput{Changed: err == nil, Memory: view(memory)}, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_forget", Description: "Retract a memory without destroying its provenance.", Annotations: writeOnly("Forget memory", true),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input GetInput) (*mcp.CallToolResult, MutationOutput, error) {
		memory, err := database.Forget(input.ID)
		return nil, MutationOutput{Changed: err == nil, Memory: view(memory)}, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_correct", Description: "Supersede an incorrect memory with corrected content.", Annotations: writeOnly("Correct memory", true),
	}, func(_ context.Context, _ *mcp.CallToolRequest, input CorrectInput) (*mcp.CallToolResult, MutationOutput, error) {
		memory, err := database.Correct(input.ID, input.Replacement)
		return nil, MutationOutput{Changed: err == nil, Memory: view(memory)}, err
	})
}
