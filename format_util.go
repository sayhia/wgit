package main

import (
	"fmt"
	"strconv"
)

// formatByteSize formats a byte count into a human-readable string with units (B, KB, MB, GB).
// Supports negative sizes (prefixed with "-").
func formatByteSize(sz int64) string {
	if sz < 0 {
		return "-" + formatByteSize(-sz)
	}
	if sz < 1024 {
		return strconv.FormatInt(sz, 10) + " B"
	} else if sz < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(sz)/1024.0)
	} else if sz < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(sz)/(1024.0*1024.0))
	}
	return fmt.Sprintf("%.2f GB", float64(sz)/(1024.0*1024.0*1024.0))
}

// formatBytes delegates to formatByteSize for consistent sizing across Git services.
func formatBytes(b int64) string {
	return formatByteSize(b)
}

// formatAssetSize delegates to formatByteSize for consistent asset sizing.
func formatAssetSize(sz int64) string {
	return formatByteSize(sz)
}

// formatSignedByteSize formats a byte count with an explicit "+" or "-" sign.
func formatSignedByteSize(sz int64) string {
	if sz > 0 {
		return "+" + formatByteSize(sz)
	}
	return formatByteSize(sz)
}
