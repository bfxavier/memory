package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/bfxavier/memory/internal/paths"
	"github.com/bfxavier/memory/internal/service"
	"github.com/bfxavier/memory/internal/store"
	"github.com/bfxavier/memory/internal/worker"
)

func runInit(appPaths paths.Paths, output io.Writer) error {
	if err := appPaths.Ensure(); err != nil {
		return err
	}
	database, err := store.Open(appPaths.Database)
	if err != nil {
		return err
	}
	defer database.Close()
	return writeJSON(output, map[string]any{
		"database": appPaths.Database,
		"spool":    appPaths.Spool,
		"ready":    true,
	})
}

func runWorker(args []string, appPaths paths.Paths, output io.Writer) error {
	flags := commandFlags("worker")
	once := flags.Bool("once", false, "process current backlog and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *once {
		result, err := worker.RunOnce(appPaths)
		if err != nil {
			return err
		}
		return writeJSON(output, result)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return worker.Run(ctx, appPaths)
}

func runService(args []string, output io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: memory service <install|uninstall|status>")
	}
	var result service.Result
	var err error
	switch args[0] {
	case "install":
		result, err = service.Install()
	case "uninstall":
		result, err = service.Uninstall()
	case "status":
		result = service.Status()
	default:
		return fmt.Errorf("unknown service command %q", args[0])
	}
	if err != nil {
		return err
	}
	return writeJSON(output, result)
}
