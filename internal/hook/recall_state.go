package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/paths"
)

// A session must not be re-shown a memory, so the list has to outlast the
// session rather than the file. At six per prompt this covers well over a
// thousand prompts, and only bounds a runaway.
const maxTrackedInjections = 10000

// The ids a session has already been shown, so search can exclude them while
// selecting candidates. Filtering after the limit would shrink the result to
// nothing instead of refilling it from the matches below the cut.
func alreadyInjected(appPaths paths.Paths, sessionID string) []string {
	path := injectionLogPath(appPaths, sessionID)
	if path == "" {
		return nil
	}
	return loadInjections(path)
}

func recordInjections(appPaths paths.Paths, sessionID string, memories []model.Memory) {
	if len(memories) == 0 {
		return
	}
	path := injectionLogPath(appPaths, sessionID)
	if path == "" {
		return
	}
	injected := loadInjections(path)
	for _, memory := range memories {
		injected = append(injected, memory.ID)
	}
	if len(injected) > maxTrackedInjections {
		injected = injected[len(injected)-maxTrackedInjections:]
	}
	data, err := json.Marshal(injected)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func loadInjections(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var injected []string
	if err := json.Unmarshal(data, &injected); err != nil {
		return nil
	}
	return injected
}

func injectionLogPath(appPaths paths.Paths, sessionID string) string {
	if appPaths.Home == "" {
		return ""
	}
	return filepath.Join(appPaths.Home, "recall", sessionFileName(sessionID)+".json")
}

func sessionFileName(sessionID string) string {
	var builder strings.Builder
	for _, character := range sessionID {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '-', character == '_':
			builder.WriteRune(character)
		default:
			builder.WriteRune('-')
		}
		if builder.Len() >= 96 {
			break
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}
