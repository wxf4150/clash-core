//go:build !windows
// +build !windows

package main

import (
	"os"
	"syscall"
)

func hasReloadSignal() bool {
	return true
}

func hasRestartSignal() bool {
	return true
}

func getReloadSignal() syscall.Signal {
	return syscall.SIGHUP
}

func getRestartSignal() syscall.Signal {
	return syscall.SIGUSR1
}

func reloadNotifySignals() []os.Signal {
	return []os.Signal{syscall.SIGHUP}
}

func restartNotifySignals() []os.Signal {
	return []os.Signal{syscall.SIGUSR1}
}
