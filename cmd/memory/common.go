package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/bfxavier/memory/internal/paths"
	"github.com/bfxavier/memory/internal/project"
	"github.com/bfxavier/memory/internal/store"
)

func openStore(appPaths paths.Paths) (*store.Store, error) {
	if err := appPaths.Ensure(); err != nil {
		return nil, err
	}
	return store.Open(appPaths.Database)
}

func resolveProjectID(value string, global bool) string {
	if global {
		return ""
	}
	if value == "" {
		value, _ = os.Getwd()
	}
	if info, err := os.Stat(value); err == nil && info.IsDir() {
		return project.Resolve(value).ID
	}
	return value
}

func commandFlags(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func writeJSON(output io.Writer, value any) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func usage(output io.Writer) error {
	_, err := fmt.Fprintln(output, `memory <command>

Commands:
  init
  hook <codex|claude> <event>
  worker [--once]
  service <install|uninstall|status>
  provider host [--codex-model name] [--claude-model name]
  provider configure --model name [--url URL] [--api-key-env NAME]
  provider <disable|retry|status>
  mcp
  remember [--kind kind] [--global|--project path] <content>
  search [--all|--project path] [--limit n] <query>
  recent [--all|--project path] [--limit n]
  inspect <id>
  correct <id> <replacement>
  forget <id>
  export
  doctor
  version`)
	return err
}
