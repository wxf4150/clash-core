//go:build windows
// +build windows

package main

import (
	"fmt"
	"net/http"
	"os"
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
