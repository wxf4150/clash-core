//go:build windows
// +build windows

package route

import (
	"github.com/Dreamacro/clash/log"
	"os"
	"time"
)

func triggerRestart() {
	// On Windows, syscall.Kill and SIGUSR1 are not supported. Perform a graceful exit
	// after a short delay so the HTTP response can be sent. An external process
	// manager should restart the application if desired.
	log.Infoln("Windows platform detected: exiting process for restart")
	time.Sleep(restartDelay)
	os.Exit(0)
}
