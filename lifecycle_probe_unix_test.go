//go:build !windows

package main

import (
	"syscall"
)

// processGone reports whether a pid is no longer present in the process table
// (reaped). A zombie still answers signal 0, so this distinguishes "killed and
// waited on" from "killed but lingering as a zombie".
func processGone(pid int) bool {
	return syscall.Kill(pid, 0) == syscall.ESRCH
}
