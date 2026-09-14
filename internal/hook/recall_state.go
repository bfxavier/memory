package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bfxavier/memory/internal/model"
	"github.com/bfxavier/memory/internal/paths"
)

const (
	maxTrackedInjections = 10000
	staleInjectionAge    = 7 * 24 * time.Hour
)

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

func forgetInjections(appPaths paths.Paths, sessionID string) {
	if path := injectionLogPath(appPaths, sessionID); path != "" {
		_ = os.Remove(path)
	}
	sweepStaleInjections(appPaths)
}

func sweepStaleInjections(appPaths paths.Paths) {
	if appPaths.Home == "" {
		return
	}
	directory := filepath.Join(appPaths.Home, "recall")
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-staleInjectionAge)
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(directory, entry.Name()))
	}
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
