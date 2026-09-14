package store

import (
	"context"
	"testing"
	"time"

	"github.com/bfxavier/memory/internal/model"
)

const project = "project-a"

func TestMemorySharingOneCommonWordIsNotRecalled(t *testing.T) {
	database := seeded(t, map[string]string{
		"escaping": "az rest rejects a JSON body escaped with Windows-style backslash quotes on macOS",
		"password": "A password generated via az vm run-command invoke is written to Azure activity logs",
	})
	results := search(t, database, "az rest JSON body escaping", 0.5)
	if len(results) != 1 || results[0].Content[:8] != "az rest " {
		t.Fatalf("expected only the escaping memory, got %s", contents(results))
	}
}

func TestSharedStopWordsDoNotReachTheCoverageFloor(t *testing.T) {
	database := seeded(t, map[string]string{
		"meaningful": "run the deploy from a release branch",
		"stopwords":  "we need to do this from the one that is in here",
	})
	results := search(t, database, "we need to run the deploy from this branch", 0.5)
	if len(results) != 1 || results[0].Content != "run the deploy from a release branch" {
		t.Fatalf("stop words carried relevance: %s", contents(results))
	}
}

func TestCoverageOutranksRepeatedSingleTerm(t *testing.T) {
	database := seeded(t, map[string]string{
		"repeat":   "okta okta okta okta okta okta okta okta",
		"complete": "okta role group gates metabase sign-in",
	})
	results := search(t, database, "metabase okta role group gate", 0.5)
	if len(results) == 0 || results[0].Content != "okta role group gates metabase sign-in" {
		t.Fatalf("coverage did not outrank term frequency: %s", contents(results))
	}
}

func TestConfidenceBreaksCoverageTies(t *testing.T) {
	database := open(t)
	remember(t, database, model.Memory{
		ProjectID: project, Kind: "fact", Confidence: 0.3,
		Content: "netbird prod tier membership is declared in vpn.json",
	})
	confident := remember(t, database, model.Memory{
		ProjectID: project, Kind: "fact", Confidence: 1,
		Content: "netbird prod tier membership is authoritative in vpn.json",
	})
	results := search(t, database, "netbird prod tier membership", 0.5)
	if len(results) != 2 || results[0].ID != confident.ID {
		t.Fatalf("confidence did not break the tie: %s", contents(results))
	}
}

func TestRecencyBreaksCoverageAndConfidenceTies(t *testing.T) {
	database := open(t)
	stale := remember(t, database, model.Memory{
		ProjectID: project, Kind: "fact", Confidence: 1,
		Content: "clickhouse reader access is granted per environment",
	})
	fresh := remember(t, database, model.Memory{
		ProjectID: project, Kind: "fact", Confidence: 1,
		Content: "clickhouse reader access is granted per data platform",
	})
	age(t, database, stale.ID, time.Now().Add(-365*24*time.Hour))
	results := search(t, database, "clickhouse reader access granted", 0.5)
	if len(results) != 2 || results[0].ID != fresh.ID {
		t.Fatalf("recency did not break the tie: %s", contents(results))
	}
}

func TestUnderscoreSplitsLikeTheSearchIndex(t *testing.T) {
	database := seeded(t, map[string]string{
		"otel": "ClickHouse otel_logs lacks timing columns for the observability query",
	})
	if results := search(t, database, "clickhouse otel logs observability query", 0.5); len(results) != 1 {
		t.Fatalf("underscored token did not count toward coverage: %s", contents(results))
	}
}

func TestCountActiveIncludesGlobalMemories(t *testing.T) {
	database := open(t)
	remember(t, database, model.Memory{ProjectID: project, Kind: "fact", Content: "a project memory"})
	remember(t, database, model.Memory{Kind: "preference", Content: "a global memory"})
	remember(t, database, model.Memory{ProjectID: "project-b", Kind: "fact", Content: "another project"})
	count, err := database.CountActive(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

func open(t *testing.T) *Store {
	t.Helper()
	database, err := Open(t.TempDir() + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func seeded(t *testing.T, contents map[string]string) *Store {
	t.Helper()
	database := open(t)
	for _, content := range contents {
		remember(t, database, model.Memory{ProjectID: project, Kind: "fact", Content: content})
	}
	return database
}

func remember(t *testing.T, database *Store, memory model.Memory) model.Memory {
	t.Helper()
	written, err := database.Remember(memory)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

func age(t *testing.T, database *Store, id string, when time.Time) {
	t.Helper()
	if _, err := database.db.Exec(
		`UPDATE memories SET updated_at = ?, created_at = ? WHERE id = ?`,
		when.UnixMilli(), when.UnixMilli(), id); err != nil {
		t.Fatal(err)
	}
}

func search(t *testing.T, database *Store, query string, minCoverage float64) []model.Memory {
	t.Helper()
	results, err := database.Search(context.Background(), query, model.SearchOptions{
		ProjectID: project, Limit: 10, MinCoverage: minCoverage,
	})
	if err != nil {
		t.Fatal(err)
	}
	return results
}

func contents(memories []model.Memory) string {
	joined := ""
	for _, memory := range memories {
		joined += "\n  " + memory.Content
	}
	if joined == "" {
		return "(none)"
	}
	return joined
}
