package extractor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func createTemporaryFile(pattern, content string) (string, func(), error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	remove := func() { _ = os.Remove(path) }
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		remove()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		remove()
		return "", nil, err
	}
	return path, remove, nil
}

func resolveCommand(configured, name string) (string, error) {
	if configured != "" {
		path, err := exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("find configured %s command: %w", name, err)
		}
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, candidate := range commandCandidates(name) {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s command not found; configure its absolute path", name)
}

func commandCandidates(name string) []string {
	base := []string{}
	if executable, err := os.Executable(); err == nil {
		base = append(base, filepath.Join(filepath.Dir(executable), name))
	}
	if home, err := os.UserHomeDir(); err == nil {
		base = append(base,
			filepath.Join(home, ".local", "bin", name),
			filepath.Join(home, ".bun", "bin", name),
			filepath.Join(home, "AppData", "Local", "Programs", name, name),
		)
	}
	extensions := []string{""}
	if runtime.GOOS == "windows" {
		extensions = []string{".exe", ".cmd", ".bat", ""}
	}
	candidates := make([]string, 0, len(base)*len(extensions))
	for _, path := range base {
		for _, extension := range extensions {
			candidates = append(candidates, path+extension)
		}
	}
	return candidates
}

func extractorEnvironment() []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "MEMORY_EXTRACTOR=") {
			environment = append(environment, value)
		}
	}
	return append(environment, "MEMORY_EXTRACTOR=1")
}

func commandError(name string, err error, stderr string) error {
	message := strings.TrimSpace(clip(stderr, 1000))
	if message == "" {
		return fmt.Errorf("%s extraction: %w", name, err)
	}
	return fmt.Errorf("%s extraction: %w: %s", name, err, message)
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(value []byte) (int, error) {
	written := len(value)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		_, _ = b.buffer.Write(value)
	}
	return written, nil
}

func (b *limitedBuffer) String() string {
	return b.buffer.String()
}
