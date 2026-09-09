//go:build !darwin && !linux && !windows

package service

import "errors"

func Install() (Result, error) {
	return Result{}, errors.New("worker service is not supported on this platform")
}

func Uninstall() (Result, error) {
	return Result{}, errors.New("worker service is not supported on this platform")
}

func Status() Result {
	return Result{}
}
