package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/bfxavier/memory/internal/hook"
	"github.com/bfxavier/memory/internal/mcpserver"
	"github.com/bfxavier/memory/internal/paths"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output io.Writer) error {
	if len(args) == 0 {
		return usage(output)
	}
	appPaths, err := paths.Resolve()
	if err != nil {
		return err
	}
	switch args[0] {
	case "init":
		return runInit(appPaths, output)
	case "hook":
		if len(args) < 3 {
			return errors.New("usage: memory hook <codex|claude> <event>")
		}
		hook.Execute(args[1], args[2], input, output, appPaths)
		return nil
	case "worker":
		return runWorker(args[1:], appPaths, output)
	case "service":
		return runService(args[1:], output)
	case "provider":
		return runProvider(args[1:], appPaths, output)
	case "mcp":
		return runMCP(appPaths)
	case "remember":
		return runRemember(args[1:], appPaths, output)
	case "forget":
		return runForget(args[1:], appPaths, output)
	case "correct":
		return runCorrect(args[1:], appPaths, output)
	case "inspect":
		return runInspect(args[1:], appPaths, output)
	case "search":
		return runSearch(args[1:], appPaths, output)
	case "recent":
		return runRecent(args[1:], appPaths, output)
	case "export":
		return runExport(appPaths, output)
	case "doctor", "status":
		return runDoctor(appPaths, output)
	case "version", "--version", "-v":
		_, err := fmt.Fprintln(output, version)
		return err
	case "help", "--help", "-h":
		return usage(output)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runMCP(appPaths paths.Paths) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return mcpserver.Run(ctx, appPaths.Database, version)
}
