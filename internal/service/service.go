package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Result struct {
	Installed  bool   `json:"installed"`
	Active     bool   `json:"active"`
	Definition string `json:"definition,omitempty"`
}

func executablePath() (string, error) {
	value, err := os.Executable()
	if err != nil {
		return "", err
	}
	value, err = filepath.EvalSymlinks(value)
	if err != nil {
		return "", err
	}
	return filepath.Abs(value)
}

func writeDefinition(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func run(name string, args ...string) error {
	command := exec.Command(name, args...)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, output)
	}
	return nil
}
