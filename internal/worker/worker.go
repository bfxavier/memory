package worker

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/bfxavier/memory/internal/config"
	"github.com/bfxavier/memory/internal/extractor"
	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/paths"
	"github.com/bfxavier/memory/internal/project"
	"github.com/bfxavier/memory/internal/spool"
	"github.com/bfxavier/memory/internal/store"
)

type Result struct {
	Events   int `json:"events"`
	Jobs     int `json:"jobs"`
	Memories int `json:"memories"`
	Failed   int `json:"failed"`
}

func RunOnce(appPaths paths.Paths) (Result, error) {
	if err := appPaths.Ensure(); err != nil {
		return Result{}, err
	}
	database, err := store.Open(appPaths.Database)
	if err != nil {
		return Result{}, err
	}
	defer database.Close()
	result := Result{}
	result.Events, err = spool.Consume(appPaths.Spool, appPaths.Failed, func(event model.Event) error {
		resolved := project.Resolve(event.ProjectRoot)
		if err := database.UpsertProject(resolved); err != nil {
			return err
		}
		return database.InsertEvent(event)
	})
	if err != nil {
		return result, err
	}
	settings, err := config.Load(appPaths.Config)
	if err != nil {
		return result, err
	}
	if !settings.Extraction.Enabled {
		return result, nil
	}
	for range 4 {
		job, events, claimErr := database.ClaimExtractionJob(context.Background(), settings.Extraction.MaxJobsPerHour)
		if errors.Is(claimErr, sql.ErrNoRows) || errors.Is(claimErr, store.ErrExtractionRateLimited) {
			break
		}
		if claimErr != nil {
			return result, claimErr
		}
		result.Jobs++
		active, activeErr := database.ActiveProject(context.Background(), job.ProjectID, 64)
		if activeErr != nil {
			if err := database.FailExtractionJob(job, activeErr); err != nil {
				return result, err
			}
			result.Failed++
			continue
		}
		var memories []model.ExtractedMemory
		var extractionErr error
		if settings.Extraction.Provider == "host" {
			memories, extractionErr = extractor.NewHost(settings.Extraction).Extract(context.Background(), job.Agent, events, active)
		} else {
			memories, extractionErr = extractor.New(settings.Extraction).Extract(context.Background(), events, active)
		}
		if extractionErr != nil {
			if err := database.FailExtractionJob(job, extractionErr); err != nil {
				return result, err
			}
			result.Failed++
			continue
		}
		count, completeErr := database.CompleteExtractionJob(job, memories)
		if completeErr != nil {
			if err := database.FailExtractionJob(job, completeErr); err != nil {
				return result, err
			}
			result.Failed++
			continue
		}
		result.Memories += count
	}
	return result, nil
}

func Run(ctx context.Context, appPaths paths.Paths) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := RunOnce(appPaths); err != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
