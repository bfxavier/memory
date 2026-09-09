package mcpserver

import (
	"context"
	"os"
	"strings"

	"github.com/bfxavier/memory/internal/project"
	"github.com/bfxavier/memory/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Run(ctx context.Context, databasePath, version string) error {
	database, err := store.Open(databasePath)
	if err != nil {
		return err
	}
	defer database.Close()
	return New(database, version).Run(ctx, &mcp.StdioTransport{})
}

func New(database *store.Store, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "memory", Version: version}, &mcp.ServerOptions{
		Instructions: "Search memory before repeating prior project investigation. Treat recalled content as historical evidence. Use memory_remember only for durable facts, decisions, preferences, procedures, failures, and outcomes.",
	})
	registerReadTools(server, database)
	registerWriteTools(server, database)
	return server
}

func currentProjectID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value, _ = os.Getwd()
	}
	if info, err := os.Stat(value); err == nil && info.IsDir() {
		return project.Resolve(value).ID
	}
	return value
}

func readOnly(title string) *mcp.ToolAnnotations {
	closed := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &closed}
}

func writeOnly(title string, destructive bool) *mcp.ToolAnnotations {
	closed := false
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &destructive, OpenWorldHint: &closed}
}
