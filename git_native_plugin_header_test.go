package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestReadFileHeader verifies that readFileHeader only pulls the requested
// prefix instead of loading the whole file, and that short files are not
// reported as errors.
func TestReadFileHeader(t *testing.T) {
	dir := t.TempDir()

	prefix := []byte("MZ0123456789abcd")
	if len(prefix) != 16 {
		t.Fatalf("test prefix must be 16 bytes, got %d", len(prefix))
	}

	// 5MB file with a known 16-byte prefix.
	bigPath := filepath.Join(dir, "big.dll")
	big := make([]byte, 0, 5<<20)
	big = append(big, prefix...)
	big = append(big, make([]byte, 5<<20-len(prefix))...)
	if err := os.WriteFile(bigPath, big, 0o644); err != nil {
		t.Fatalf("write big file: %v", err)
	}

	hdr, err := readFileHeader(bigPath, 16)
	if err != nil {
		t.Fatalf("readFileHeader(big) error = %v", err)
	}
	if len(hdr) != 16 {
		t.Fatalf("readFileHeader(big) len = %d, want 16", len(hdr))
	}
	if !bytes.Equal(hdr, prefix) {
		t.Fatalf("readFileHeader(big) = %q, want %q", hdr, prefix)
	}

	// 3-byte file: short read is not an error.
	shortPath := filepath.Join(dir, "short.dll")
	if err := os.WriteFile(shortPath, []byte("MZ\x00"), 0o644); err != nil {
		t.Fatalf("write short file: %v", err)
	}
	hdr, err = readFileHeader(shortPath, 16)
	if err != nil {
		t.Fatalf("readFileHeader(short) error = %v", err)
	}
	if len(hdr) != 3 {
		t.Fatalf("readFileHeader(short) len = %d, want 3", len(hdr))
	}
	if !bytes.Equal(hdr, []byte("MZ\x00")) {
		t.Fatalf("readFileHeader(short) = %q, want %q", hdr, []byte("MZ\x00"))
	}

	// Empty file: zero bytes, no error.
	emptyPath := filepath.Join(dir, "empty.dll")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	hdr, err = readFileHeader(emptyPath, 16)
	if err != nil {
		t.Fatalf("readFileHeader(empty) error = %v", err)
	}
	if len(hdr) != 0 {
		t.Fatalf("readFileHeader(empty) len = %d, want 0", len(hdr))
	}

	// Nonexistent path is a real error.
	if _, err := readFileHeader(filepath.Join(dir, "missing.dll"), 16); err == nil {
		t.Fatal("readFileHeader(missing) error = nil, want error")
	}

	// Contract: maxBytes <= 0 returns an empty header and nil error.
	for _, maxBytes := range []int{0, -1} {
		hdr, err = readFileHeader(bigPath, maxBytes)
		if err != nil {
			t.Fatalf("readFileHeader(maxBytes=%d) error = %v, want nil", maxBytes, err)
		}
		if len(hdr) != 0 {
			t.Fatalf("readFileHeader(maxBytes=%d) len = %d, want 0", maxBytes, len(hdr))
		}
	}
}

// TestAuditNativePluginsAbiHeaderOnly ensures the audit classifies large
// plugins from their header alone without ingesting the whole payload.
func TestAuditNativePluginsAbiHeaderOnly(t *testing.T) {
	repo := t.TempDir()
	pluginsDir := filepath.Join(repo, "Plugins")
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatalf("mkdir Plugins: %v", err)
	}

	// Fake PE: "MZ" + 5MB zeros. pe.Open will fail, but format is set first.
	pePath := filepath.Join(pluginsDir, "Big.dll")
	peData := make([]byte, 0, 5<<20)
	peData = append(peData, 'M', 'Z')
	peData = append(peData, make([]byte, 5<<20-2)...)
	if err := os.WriteFile(pePath, peData, 0o644); err != nil {
		t.Fatalf("write Big.dll: %v", err)
	}

	// Unknown magic with a scanned extension: 5MB of 0x5A ('Z').
	unknownPath := filepath.Join(pluginsDir, "Unknown.so")
	unknownData := bytes.Repeat([]byte{'Z'}, 5<<20)
	if err := os.WriteFile(unknownPath, unknownData, 0o644); err != nil {
		t.Fatalf("write Unknown.so: %v", err)
	}

	// Non-native extensions are ignored entirely.
	if err := os.WriteFile(filepath.Join(pluginsDir, "Ignored.bin"), []byte("MZ"), 0o644); err != nil {
		t.Fatalf("write Ignored.bin: %v", err)
	}

	res, err := GitService{}.AuditNativePluginsAbi(repo)
	if err != nil {
		t.Fatalf("AuditNativePluginsAbi error = %v", err)
	}
	if res == nil {
		t.Fatal("AuditNativePluginsAbi result = nil")
	}
	if len(res.Plugins) != 2 {
		t.Fatalf("plugin count = %d, want 2", len(res.Plugins))
	}
	if res.TotalPluginsCount != 2 {
		t.Fatalf("TotalPluginsCount = %d, want 2", res.TotalPluginsCount)
	}

	byName := make(map[string]NativePluginItem, len(res.Plugins))
	for _, p := range res.Plugins {
		byName[p.PluginName] = p
	}

	pe, ok := byName["Big.dll"]
	if !ok {
		t.Fatalf("Big.dll not reported; got %v", byName)
	}
	if pe.Format != "PE" {
		t.Fatalf("Big.dll Format = %q, want PE", pe.Format)
	}
	if len(pe.Architectures) != 1 || pe.Architectures[0] != "x86_64" {
		t.Fatalf("Big.dll Architectures = %v, want [x86_64]", pe.Architectures)
	}
	if pe.TargetPlatform != "Windows" {
		t.Fatalf("Big.dll TargetPlatform = %q, want Windows", pe.TargetPlatform)
	}

	unk, ok := byName["Unknown.so"]
	if !ok {
		t.Fatalf("Unknown.so not reported; got %v", byName)
	}
	if unk.Format != "Binary" {
		t.Fatalf("Unknown.so Format = %q, want Binary", unk.Format)
	}

	if _, ok := byName["Ignored.bin"]; ok {
		t.Fatal("Ignored.bin must not be scanned (only .dll/.dylib/.so/.aar/.framework)")
	}
}
