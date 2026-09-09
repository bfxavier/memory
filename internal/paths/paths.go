package paths

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

type Paths struct {
	Home     string
	Database string
	Spool    string
	Failed   string
	Config   string
}

func Resolve() (Paths, error) {
	home, err := dataHome()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Home:     home,
		Database: filepath.Join(home, "memory.db"),
		Spool:    filepath.Join(home, "spool", "incoming"),
		Failed:   filepath.Join(home, "spool", "failed"),
		Config:   filepath.Join(home, "config.json"),
	}, nil
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.Home, p.Spool, p.Failed} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func dataHome() (string, error) {
	if value := os.Getenv("MEMORY_HOME"); value != "" {
		return filepath.Abs(value)
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		if value := os.Getenv("LOCALAPPDATA"); value != "" {
			return filepath.Join(value, "memory"), nil
		}
		return filepath.Join(userHome, "AppData", "Local", "memory"), nil
	case "darwin":
		return filepath.Join(userHome, "Library", "Application Support", "memory"), nil
	default:
		if value := os.Getenv("XDG_DATA_HOME"); value != "" {
			return filepath.Join(value, "memory"), nil
		}
		if userHome == "" {
			return "", errors.New("cannot resolve data directory")
		}
		return filepath.Join(userHome, ".local", "share", "memory"), nil
	}
}
