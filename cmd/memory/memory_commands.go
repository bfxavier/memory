package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/paths"
)

func runRemember(args []string, appPaths paths.Paths, output io.Writer) error {
	flags := commandFlags("remember")
	projectPath := flags.String("project", "", "project path or ID")
	global := flags.Bool("global", false, "store as global memory")
	kind := flags.String("kind", "note", "memory kind")
	if err := flags.Parse(args); err != nil {
		return err
	}
	content := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if content == "" {
		return errors.New("memory content is required")
	}
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	memory, err := database.Remember(model.Memory{
		ProjectID: resolveProjectID(*projectPath, *global),
		Kind:      *kind,
		Content:   content,
	})
	if err != nil {
		return err
	}
	return writeJSON(output, memory)
}

func runForget(args []string, appPaths paths.Paths, output io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: memory forget <id>")
	}
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.Forget(args[0]); err != nil {
		return err
	}
	return writeJSON(output, map[string]any{"changed": true, "id": args[0]})
}

func runCorrect(args []string, appPaths paths.Paths, output io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: memory correct <id> <replacement>")
	}
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	memory, err := database.Correct(args[0], strings.Join(args[1:], " "))
	if err != nil {
		return err
	}
	return writeJSON(output, memory)
}

func runInspect(args []string, appPaths paths.Paths, output io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: memory inspect <id>")
	}
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	memory, err := database.Get(args[0])
	if err != nil {
		return err
	}
	return writeJSON(output, memory)
}

func runSearch(args []string, appPaths paths.Paths, output io.Writer) error {
	flags := commandFlags("search")
	projectPath := flags.String("project", "", "project path or ID")
	all := flags.Bool("all", false, "search all projects")
	limit := flags.Int("limit", 10, "maximum results")
	if err := flags.Parse(args); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if query == "" {
		return errors.New("search query is required")
	}
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	projectID := ""
	if !*all {
		projectID = resolveProjectID(*projectPath, false)
	}
	memories, err := database.Search(context.Background(), query, model.SearchOptions{ProjectID: projectID, Limit: *limit})
	if err != nil {
		return err
	}
	return writeJSON(output, memories)
}

func runRecent(args []string, appPaths paths.Paths, output io.Writer) error {
	flags := commandFlags("recent")
	projectPath := flags.String("project", "", "project path or ID")
	all := flags.Bool("all", false, "include all projects")
	limit := flags.Int("limit", 10, "maximum results")
	if err := flags.Parse(args); err != nil {
		return err
	}
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	projectID := ""
	if !*all {
		projectID = resolveProjectID(*projectPath, false)
	}
	memories, err := database.Recent(context.Background(), model.SearchOptions{ProjectID: projectID, Limit: *limit})
	if err != nil {
		return err
	}
	return writeJSON(output, memories)
}

func runExport(appPaths paths.Paths, output io.Writer) error {
	database, err := openStore(appPaths)
	if err != nil {
		return err
	}
	defer database.Close()
	memories, err := database.All(context.Background())
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	for _, memory := range memories {
		if err := encoder.Encode(memory); err != nil {
			return err
		}
	}
	return nil
}
