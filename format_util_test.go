package main

import (
	"testing"
)

func TestFormatByteSize(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{-1536, "-1.5 KB"},
		{1024 * 1024, "1.00 MB"},
		{5 * 1024 * 1024, "5.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{2560 * 1024 * 1024, "2.50 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{7680 * 1024 * 1024 * 1024, "7.50 TB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1.00 PB"},
		{-5 * 1024 * 1024, "-5.00 MB"},
	}

	for _, tc := range tests {
		got := formatByteSize(tc.input)
		if got != tc.want {
			t.Errorf("formatByteSize(%d) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFormatSignedByteSize(t *testing.T) {
	if got := formatSignedByteSize(1024); got != "+1.0 KB" {
		t.Errorf("formatSignedByteSize(1024) = %q, want +1.0 KB", got)
	}
	if got := formatSignedByteSize(-1024); got != "-1.0 KB" {
		t.Errorf("formatSignedByteSize(-1024) = %q, want -1.0 KB", got)
	}
	if got := formatSignedByteSize(0); got != "0 B" {
		t.Errorf("formatSignedByteSize(0) = %q, want 0 B", got)
	}
}

func TestFormatDelegates(t *testing.T) {
	if got := formatBytes(2048); got != "2.0 KB" {
		t.Errorf("formatBytes(2048) = %q, want 2.0 KB", got)
	}
	if got := formatAssetSize(2048); got != "2.0 KB" {
		t.Errorf("formatAssetSize(2048) = %q, want 2.0 KB", got)
	}
}
