//go:build windows
// +build windows

package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func hasReloadSignal() bool {
	return true
}

func hasRestartSignal() bool {
	return false
}

func getReloadSignal() syscall.Signal {
	return syscall.SIGHUP
}

func getRestartSignal() syscall.Signal {
	return syscall.Signal(0)
}

func reloadNotifySignals() []os.Signal {
	return []os.Signal{syscall.SIGHUP}
}

func restartNotifySignals() []os.Signal {
	return nil
}

// sendReloadToController sends a POST to /configs/reload on the local external controller.
// controller can be like "127.0.0.1:9090" or "http://127.0.0.1:9090". If empty, default is 127.0.0.1:9090.
func sendReloadToController(controller, secret string) error {
	if controller == "" {
		controller = "127.0.0.1:9090"
	}
	var urlStr string
	if strings.Contains(controller, "://") {
		urlStr = strings.TrimRight(controller, "/") + "/configs/reload"
	} else {
		urlStr = "http://" + strings.TrimRight(controller, "/") + "/configs/reload"
	}

	req, err := http.NewRequest("POST", urlStr, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	return nil
}

// sendRestartToController sends a POST to /configs/restart on the local external controller.
// controller can be like "127.0.0.1:9090" or "http://127.0.0.1:9090". If empty, default is 127.0.0.1:9090.
func sendRestartToController(controller, secret string) error {
	if controller == "" {
		controller = "127.0.0.1:9090"
	}
	var urlStr string
	if strings.Contains(controller, "://") {
		urlStr = strings.TrimRight(controller, "/") + "/configs/restart"
	} else {
		urlStr = "http://" + strings.TrimRight(controller, "/") + "/configs/restart"
	}

	req, err := http.NewRequest("POST", urlStr, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}
	return nil
}

// spawnDaemon (Windows) — 使用 CreationFlags 脱离父进程，并把输出写到 clash.log
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

	const detached = 0x00000008
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | detached,
	}

	if err := cmd.Start(); err != nil {
		f.Close()
		return err
	}

	return f.Close()
}
