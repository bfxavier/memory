//go:build darwin

package service

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
)

const launchAgentLabel = "dev.memory.worker"

func Install() (Result, error) {
	executable, err := executablePath()
	if err != nil {
		return Result{}, err
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	definition := filepath.Join(userHome, "Library", "LaunchAgents", launchAgentLabel+".plist")
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>worker</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/dev/null</string>
  <key>StandardErrorPath</key>
  <string>/dev/null</string>
</dict>
</plist>
`, launchAgentLabel, html.EscapeString(executable))
	if err := writeDefinition(definition, []byte(plist)); err != nil {
		return Result{}, err
	}
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+launchAgentLabel).Run()
	if err := run("launchctl", "bootstrap", domain, definition); err != nil {
		return Result{Installed: true, Definition: definition}, err
	}
	return Result{Installed: true, Active: true, Definition: definition}, nil
}

func Uninstall() (Result, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	definition := filepath.Join(userHome, "Library", "LaunchAgents", launchAgentLabel+".plist")
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	_ = exec.Command("launchctl", "bootout", domain+"/"+launchAgentLabel).Run()
	if err := os.Remove(definition); err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	return Result{}, nil
}

func Status() Result {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return Result{}
	}
	definition := filepath.Join(userHome, "Library", "LaunchAgents", launchAgentLabel+".plist")
	_, statErr := os.Stat(definition)
	domain := fmt.Sprintf("gui/%d/%s", os.Getuid(), launchAgentLabel)
	active := exec.Command("launchctl", "print", domain).Run() == nil
	return Result{Installed: statErr == nil, Active: active, Definition: definition}
}
