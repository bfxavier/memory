package integrationconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type hookConfig struct {
	Hooks map[string][]struct {
		Hooks []struct {
			Command        string `json:"command"`
			CommandWindows string `json:"commandWindows"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func TestHookPackages(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		agent string
		path  string
	}{
		{"codex", filepath.Join(root, "integrations", "codex", "memory", "hooks", "hooks.json")},
		{"claude", filepath.Join(root, "integrations", "claude", "memory", "hooks", "hooks.json")},
	}
	for _, test := range tests {
		t.Run(test.agent, func(t *testing.T) {
			data, err := os.ReadFile(test.path)
			if err != nil {
				t.Fatal(err)
			}
			var config hookConfig
			if err := json.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			for _, event := range []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "PreCompact", "Stop", "SessionEnd", "SubagentStart", "SubagentStop"} {
				groups := config.Hooks[event]
				if len(groups) == 0 {
					t.Fatalf("missing %s hook", event)
				}
				for _, group := range groups {
					for _, handler := range group.Hooks {
						if !strings.Contains(handler.Command, "memory hook "+test.agent+" "+event) {
							t.Fatalf("unexpected %s command %q", event, handler.Command)
						}
						if test.agent == "codex" && !strings.Contains(handler.CommandWindows, "memory.exe hook codex "+event) {
							t.Fatalf("unexpected Windows %s command %q", event, handler.CommandWindows)
						}
					}
				}
			}
		})
	}
}

func TestMarketplaces(t *testing.T) {
	root := repositoryRoot(t)
	tests := []struct {
		path       string
		sourcePath string
	}{
		{filepath.Join(root, ".agents", "plugins", "marketplace.json"), "./integrations/codex/memory"},
		{filepath.Join(root, ".claude-plugin", "marketplace.json"), "./integrations/claude/memory"},
	}
	for _, test := range tests {
		data, err := os.ReadFile(test.path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		plugins, ok := document["plugins"].([]any)
		if !ok || len(plugins) != 1 {
			t.Fatalf("invalid plugins in %s", test.path)
		}
		plugin, ok := plugins[0].(map[string]any)
		if !ok || plugin["name"] != "memory" {
			t.Fatalf("invalid memory plugin in %s", test.path)
		}
		source := plugin["source"]
		if object, ok := source.(map[string]any); ok {
			source = object["path"]
		}
		if source != test.sourcePath {
			t.Fatalf("unexpected source %v in %s", source, test.path)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
