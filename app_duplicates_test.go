package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestHashFileReportsDigestAndReadBytes(t *testing.T) {
	data := []byte("VaultApp duplicate test")
	path := filepath.Join(t.TempDir(), "candidate.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, bytesRead, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(data)
	if digest != hex.EncodeToString(want[:]) || bytesRead != int64(len(data)) {
		t.Fatalf("hashFile = %q, %d; want %q, %d", digest, bytesRead, hex.EncodeToString(want[:]), len(data))
	}
}

func TestHashFileReportsUnreadablePath(t *testing.T) {
	if digest, bytesRead, err := hashFile(filepath.Join(t.TempDir(), "missing")); err == nil || digest != "" || bytesRead != 0 {
		t.Fatalf("missing hashFile = %q, %d, %v", digest, bytesRead, err)
	}
}
