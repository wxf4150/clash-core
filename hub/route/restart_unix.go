//go:build !windows
// +build !windows

package route

import (
	"syscall"
	"time"
)

func triggerRestart() {
	// Give time for response to be sent
	time.Sleep(restartDelay)
	// Send SIGUSR1 to self to trigger restart logic in main
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
}
