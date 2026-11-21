//go:build !windows

package main

import "errors"

// Persistent credential encryption is deliberately Windows-only. Non-Windows
// builds can still use WAVE, but credentials are kept in memory for that run.
func protectSecret([]byte) ([]byte, error) {
	return nil, errors.New("secure credential storage requires Windows DPAPI")
}

func unprotectSecret([]byte) ([]byte, error) {
	return nil, errors.New("secure credential storage requires Windows DPAPI")
}
