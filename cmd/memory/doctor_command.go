package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/paths"
	"github.com/bfxavier/memory/internal/service"
)

func runDoctor(appPaths paths.Paths, output io.Writer) error {
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	ctx := context.Background()
	if err := database.Integrity(ctx); err != nil {
		return err
	}
	stats, err := database.Stats(ctx)
	if err != nil {
		return err
	}
	spoolCount, spoolBytes, err := directoryStats(appPaths.Spool)
	if err != nil {
		return err
	}
	settings, err := config.Load(appPaths.Config)
	if err != nil {
		return err
	}
	return writeJSON(output, map[string]any{
		"database":     appPaths.Database,
		"integrity":    "ok",
		"service":      service.Status(),
		"extraction":   settings.Extraction,
		"stats":        stats,
		"spool_events": spoolCount,
		"spool_bytes":  spoolBytes,
	})
}

func directoryStats(directory string) (int, int64, error) {
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	count := 0
	var size int64
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, 0, err
		}
		count++
		size += info.Size()
	}
	return count, size, nil
}
