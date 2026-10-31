package main

import (
	"fmt"
	"strconv"
)

// formatByteSize formats a byte count into a human-readable string with units
// (B, KB, MB, GB, TB, PB). Supports negative sizes (prefixed with "-").
func formatByteSize(sz int64) string {
	if sz < 0 {
		return "-" + formatByteSize(-sz)
	}
	const KiB = 1024
	const MiB = 1024 * KiB
	const GiB = 1024 * MiB
	const TiB = 1024 * GiB
	const PiB = 1024 * TiB
	switch {
	case sz < KiB:
		return strconv.FormatInt(sz, 10) + " B"
	case sz < MiB:
		return fmt.Sprintf("%.1f KB", float64(sz)/KiB)
	case sz < GiB:
		return fmt.Sprintf("%.2f MB", float64(sz)/MiB)
	case sz < TiB:
		return fmt.Sprintf("%.2f GB", float64(sz)/GiB)
	case sz < PiB:
		return fmt.Sprintf("%.2f TB", float64(sz)/TiB)
	default:
		return fmt.Sprintf("%.2f PB", float64(sz)/PiB)
	}
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
