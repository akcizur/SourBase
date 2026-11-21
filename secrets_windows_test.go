//go:build windows

package main

import (
	"bytes"
	"testing"
)

func TestDPAPIProtectRoundTrip(t *testing.T) {
	plain := []byte("WAVE IPTV DPAPI round-trip test")
	ciphertext, err := protectSecret(plain)
	if err != nil {
		t.Fatalf("protectSecret failed: %v", err)
	}
	if bytes.Equal(ciphertext, plain) {
		t.Fatal("DPAPI output unexpectedly matches plaintext")
	}
	decoded, err := unprotectSecret(ciphertext)
	if err != nil {
		t.Fatalf("unprotectSecret failed: %v", err)
	}
	if !bytes.Equal(decoded, plain) {
		t.Fatalf("round-trip mismatch: got %q", decoded)
	}
}
