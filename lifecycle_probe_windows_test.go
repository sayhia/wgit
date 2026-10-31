//go:build windows

package main

// Windows has no signal-0 probe; the pid-lifecycle assertions are skipped
// there (processGone always reports gone), while the rest of the lifecycle
// tests still run.
func processGone(int) bool {
	return true
}
