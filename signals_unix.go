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

// spawnDaemon 启动后台进程，stdout/stderr 重定向到系统临时目录下的 clash.log
func spawnDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(exe)

	logPath := filepath.Join(os.TempDir(), "clash.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}

	cmd.Stdin = nil
	cmd.Stdout = f
	cmd.Stderr = f

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		f.Close()
		return err
	}

	err = f.Close()
	if err == nil {
		fmt.Printf("Spawning background daemon, stdout/stderr -> %s\n", logPath)
		fmt.Println("Spawned new daemon successfully")
	}
	return err
}
