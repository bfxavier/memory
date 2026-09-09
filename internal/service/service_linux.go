//go:build linux

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const systemdUnit = "memory.service"

func Install() (Result, error) {
	executable, err := executablePath()
	if err != nil {
		return Result{}, err
	}
	configHome, err := os.UserConfigDir()
	if err != nil {
		return Result{}, err
	}
	definition := filepath.Join(configHome, "systemd", "user", systemdUnit)
	unit := fmt.Sprintf(`[Unit]
Description=Memory event worker

[Service]
ExecStart=%s worker
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
`, strconv.Quote(executable))
	if err := writeDefinition(definition, []byte(unit)); err != nil {
		return Result{}, err
	}
	if err := run("systemctl", "--user", "daemon-reload"); err != nil {
		return Result{Installed: true, Definition: definition}, err
	}
	if err := run("systemctl", "--user", "enable", "--now", systemdUnit); err != nil {
		return Result{Installed: true, Definition: definition}, err
	}
	return Result{Installed: true, Active: true, Definition: definition}, nil
}

func Uninstall() (Result, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return Result{}, err
	}
	definition := filepath.Join(configHome, "systemd", "user", systemdUnit)
	_ = exec.Command("systemctl", "--user", "disable", "--now", systemdUnit).Run()
	if err := os.Remove(definition); err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return Result{}, nil
}

func Status() Result {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return Result{}
	}
	definition := filepath.Join(configHome, "systemd", "user", systemdUnit)
	_, statErr := os.Stat(definition)
	active := exec.Command("systemctl", "--user", "is-active", "--quiet", systemdUnit).Run() == nil
	return Result{Installed: statErr == nil, Active: active, Definition: definition}
}
