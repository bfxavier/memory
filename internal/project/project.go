package project

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/bfxavier/memory/internal/model"
)

func Resolve(cwd string) model.Project {
	root := canonicalPath(cwd)
	identity := root
	remote := ""
	if repoRoot, gitDir, ok := findGitRoot(root); ok {
		root = repoRoot
		commonDir := resolveCommonDir(gitDir)
		identity = canonicalPath(commonDir)
		remote = readOrigin(filepath.Join(commonDir, "config"))
		if remote != "" {
			identity = normalizeRemote(remote)
		}
	}
	sum := sha256.Sum256([]byte(identity))
	return model.Project{
		ID:       hex.EncodeToString(sum[:12]),
		Identity: identity,
		Root:     root,
		Remote:   remote,
	}
}

func findGitRoot(start string) (string, string, bool) {
	current := start
	for {
		marker := filepath.Join(current, ".git")
		info, err := os.Stat(marker)
		if err == nil {
			if info.IsDir() {
				return current, marker, true
			}
			if gitDir, ok := readGitDir(marker, current); ok {
				return current, gitDir, true
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", false
		}
		current = parent
	}
}

func readGitDir(marker, root string) (string, bool) {
	data, err := os.ReadFile(marker)
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(data))
	if !strings.HasPrefix(value, "gitdir:") {
		return "", false
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(value, "gitdir:"))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(root, gitDir)
	}
	return canonicalPath(gitDir), true
}

func resolveCommonDir(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return gitDir
	}
	commonDir := strings.TrimSpace(string(data))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}
	return canonicalPath(commonDir)
}

func readOrigin(configPath string) string {
	file, err := os.Open(configPath)
	if err != nil {
		return ""
	}
	defer file.Close()
	inOrigin := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(line, "url") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func normalizeRemote(remote string) string {
	value := strings.TrimSpace(strings.TrimSuffix(remote, ".git"))
	value = strings.TrimSuffix(value, "/")
	if strings.HasPrefix(value, "git@") {
		value = strings.Replace(value, ":", "/", 1)
	}
	return strings.ToLower(value)
}

func canonicalPath(value string) string {
	if value == "" {
		value = "."
	}
	abs, err := filepath.Abs(value)
	if err == nil {
		value = abs
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		value = resolved
	}
	value = filepath.Clean(value)
	if runtime.GOOS == "windows" {
		value = strings.ToLower(value)
	}
	return value
}
