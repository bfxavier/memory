//go:build windows

package service

import (
	"os/exec"
)

const taskName = "Memory Worker"

func Install() (Result, error) {
	executable, err := executablePath()
	if err != nil {
		return Result{}, err
	}
	command := `"` + executable + `" worker`
	if err := run("schtasks.exe", "/Create", "/F", "/SC", "ONLOGON", "/TN", taskName, "/TR", command); err != nil {
		return Result{}, err
	}
	if err := run("schtasks.exe", "/Run", "/TN", taskName); err != nil {
		return Result{Installed: true}, err
	}
	return Result{Installed: true, Active: true}, nil
}

func Uninstall() (Result, error) {
	_ = exec.Command("schtasks.exe", "/End", "/TN", taskName).Run()
	if err := run("schtasks.exe", "/Delete", "/F", "/TN", taskName); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}

func Status() Result {
	installed := exec.Command("schtasks.exe", "/Query", "/TN", taskName).Run() == nil
	return Result{Installed: installed, Active: installed}
}
