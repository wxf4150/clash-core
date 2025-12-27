package main

import (
	"flag"
	"fmt"
	"github.com/Dreamacro/clash/tunnel"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/log"

	"go.uber.org/automaxprocs/maxprocs"
)

var (
	flagset            map[string]bool
	version            bool
	testConfig         bool
	homeDir            string
	configFile         string
	externalUI         string
	externalController string
	secret             string
	// mmdb download flags
	downloadMMDBFlag bool
	mmdbURL          string
	// control flags
	reloadFlag  bool
	restartFlag bool
	daemonFlag  bool
)

func init() {
	flag.StringVar(&homeDir, "d", "", "set configuration directory")
	flag.StringVar(&configFile, "f", "", "specify configuration file")
	flag.StringVar(&externalUI, "ext-ui", "", "override external ui directory")
	flag.StringVar(&externalController, "ext-ctl", "", "override external controller address")
	flag.StringVar(&secret, "secret", "", "override secret for RESTful API")
	flag.BoolVar(&version, "v", false, "show current version of clash")
	flag.BoolVar(&testConfig, "t", false, "test configuration and exit")
	// mmdb download flags
	flag.BoolVar(&downloadMMDBFlag, "download-mmdb", false, "download mmdb and exit")
	flag.StringVar(&mmdbURL, "mmdb-url", "", "mmdb download url override")
	// control flags
	flag.BoolVar(&reloadFlag, "reload", false, "send reload signal to running clash instance")
	flag.BoolVar(&restartFlag, "restart", false, "send restart signal to running clash instance")
	flag.BoolVar(&daemonFlag, "daemon", false, "running a clash instance in background")
	flag.Parse()

	flagset = map[string]bool{}
	flag.Visit(func(f *flag.Flag) {
		flagset[f.Name] = true
	})
}

func downloadMMDBWithProgress(url, outPath string) error {
	if url == "" {
		// default URL as used in config/initial.go
		url = "https://cdn.jsdelivr.net/gh/Dreamacro/maxmind-geoip@release/Country.mmdb"
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	if outPath == "" {
		outPath = C.Path.MMDB()
	}
	tmpPath := outPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	var downloaded int64
	start := time.Now()
	contentLen := resp.ContentLength

	// progress ticker
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	done := make(chan struct{})
	go func() {
		var prev int64
		for {
			select {
			case <-ticker.C:
				delta := downloaded - prev
				prev = downloaded
				secs := time.Since(start).Seconds()
				avg := float64(downloaded) / secs
				if contentLen > 0 {
					percent := float64(downloaded) / float64(contentLen) * 100
					fmt.Printf("\rDownloaded: %d / %d (%.2f%%) | Speed: %s/s | Avg: %s/s",
						downloaded, contentLen, percent, humanizeBytes(delta), humanizeBytes(int64(avg)))
				} else {
					fmt.Printf("\rDownloaded: %d | Speed: %s/s | Avg: %s/s",
						downloaded, humanizeBytes(delta), humanizeBytes(int64(avg)))
				}
			case <-done:
				return
			}
		}
	}()

	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			wn, werr := f.Write(buf[:n])
			if werr != nil {
				return werr
			}
			downloaded += int64(wn)
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return rerr
		}
	}

	// stop progress printer and print final
	done <- struct{}{}
	elapsed := time.Since(start).Seconds()
	avgSpeed := float64(downloaded) / elapsed
	fmt.Printf("\rDownloaded: %d bytes | Elapsed: %.1fs | Avg speed: %s/s\n", downloaded, elapsed, humanizeBytes(int64(avgSpeed)))

	if err := f.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "sync error: %v\n", err)
	}
	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, outPath); err != nil {
		return err
	}

	return nil
}

func humanizeBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	d := float64(b)
	exp := 0
	for d >= unit && exp < 4 {
		d /= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.2f %s", d, suffix[exp])
}

func getPIDFile() string {
	return filepath.Join(C.Path.HomeDir(), "clash.pid")
}

func writePIDFile() error {
	pidFile := getPIDFile()
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0600)
}

func readPIDFile() (int, error) {
	pidFile := getPIDFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	var pid int
	_, err = fmt.Sscanf(string(data), "%d", &pid)
	return pid, err
}

func removePIDFile() {
	pidFile := getPIDFile()
	os.Remove(pidFile)
}

func sendSignalToPID(pid int, sig syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}
	// Try to send signal 0 first to check if process exists
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("process %d does not exist or is not accessible", pid)
	}
	return process.Signal(sig)
}

func sendSignalToRunningInstance(sig syscall.Signal, actionName string) error {
	pid, err := readPIDFile()
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}
	if err := sendSignalToPID(pid, sig); err != nil {
		return fmt.Errorf("failed to send %s signal to process %d: %w", actionName, pid, err)
	}
	return nil
}

// The following helpers are platform-specific and implemented in separate files:
//   - signals_unix.go (non-Windows)
//   - signals_windows.go (Windows)
// They provide: hasReloadSignal, hasRestartSignal, getReloadSignal, getRestartSignal,
// reloadNotifySignals, restartNotifySignals

func main() {
	maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))
	if version {
		fmt.Printf("Clash %s %s %s with %s %s\n", C.Version, runtime.GOOS, runtime.GOARCH, runtime.Version(), C.BuildTime)
		return
	}

	// Set home directory first so PID file path is correct
	if homeDir != "" {
		if !filepath.IsAbs(homeDir) {
			currentDir, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get current directory: %v\n", err)
				os.Exit(1)
			}
			homeDir = filepath.Join(currentDir, homeDir)
		}
		C.SetHomeDir(homeDir)
	}
	if configFile != "" {
		if !filepath.IsAbs(configFile) {
			currentDir, _ := os.Getwd()
			configFile = filepath.Join(currentDir, configFile)
		}
		C.SetConfig(configFile)
	} else {
		configFile := filepath.Join(C.Path.HomeDir(), C.Path.Config())
		C.SetConfig(configFile)
	}

	if err := config.Init(C.Path.HomeDir()); err != nil {
		log.Fatalln("Initial configuration directory error: %s", err.Error())
	}

	if testConfig || reloadFlag || restartFlag || daemonFlag {
		if _, err := executor.Parse(); err != nil {
			log.Errorln(err.Error())
			fmt.Printf("configuration file %s test failed\n", C.Path.Config())
			os.Exit(1)
		}
		fmt.Printf("configuration file %s test is successful\n", C.Path.Config())
	}
	if testConfig {
		return
	}
	if daemonFlag {
		spawnDaemon()
		return
	}

	// Handle reload flag
	if reloadFlag {
		if runtime.GOOS == "windows" {
			// On Windows, send a HTTP request to local controller instead of using signals
			if err := sendReloadToController(externalController, secret); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to send reload via controller: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Reload request sent to controller successfully")
			return
		}

		if hasReloadSignal() {
			if err := sendSignalToRunningInstance(getReloadSignal(), "reload"); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to send reload signal: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Reload signal sent successfully")
		} else {
			fmt.Fprintln(os.Stderr, "Reload signal unsupported on this platform; use the /configs/reload API or restart manually")
			os.Exit(1)
		}
		return
	}

	// Handle restart flag
	if restartFlag {
		if runtime.GOOS == "windows" {
			// On Windows, send a HTTP request to local controller instead of using signals
			if err := sendRestartToController(externalController, secret); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to send restart via controller: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Restart request sent to controller successfully")
		} else {
			if hasRestartSignal() {
				if err := sendSignalToRunningInstance(getRestartSignal(), "restart"); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to send restart signal: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Restart signal sent successfully")
			} else {
				fmt.Println(os.Stderr, "Restart signal unsupported on this platform; use the /configs/restart API or restart manually")
				os.Exit(1)
			}
		}

		// Spawn a new background daemon after a short delay to allow the current
		time.Sleep(1500 * time.Millisecond)
		spawnDaemon()
		return
	}

	var options []hub.Option
	if flagset["ext-ui"] {
		options = append(options, hub.WithExternalUI(externalUI))
	}
	if flagset["ext-ctl"] {
		options = append(options, hub.WithExternalController(externalController))
	}
	if flagset["secret"] {
		options = append(options, hub.WithSecret(secret))
	}

	if err := hub.Parse(options...); err != nil {
		log.Fatalln("Parse config error: %s", err.Error())
	}

	if downloadMMDBFlag {
		url := mmdbURL
		if url == "" {
			// use default URL
			url = "https://cdn.jsdelivr.net/gh/Dreamacro/maxmind-geoip@release/Country.mmdb"
		}
		fmt.Printf("Downloading mmdb from %s...\n", url)
		if err := downloadMMDBWithProgress(url, ""); err != nil {
			log.Fatalln("Download mmdb error: %s", err.Error())
		}
		fmt.Println("MMDB download completed.")
		return
	}

	// Write PID file for the running instance
	if err := writePIDFile(); err != nil {
		log.Warnln("Failed to write PID file:", err.Error())
	} else {
		defer removePIDFile()
	}

	sigCh := make(chan os.Signal, 1)
	hupCh := make(chan os.Signal, 1)
	usr1Ch := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Register platform-supported reload/restart signals for notification
	if s := reloadNotifySignals(); len(s) > 0 {
		signal.Notify(hupCh, s...)
	}
	if s := restartNotifySignals(); len(s) > 0 {
		signal.Notify(usr1Ch, s...)
	}

	for {
		select {
		case <-hupCh:
			log.Infoln("Received SIGHUP signal, reloading configuration...")
			if cfg, err := executor.Parse(); err != nil {
				log.Errorln("Failed to reload configuration:", err.Error())
			} else {
				// make proxy replacement close synchronously for reload to ensure old resources are released
				tunnel.SetCloseOnReplaceSync(true)
				executor.ApplyConfig(cfg, false)
				// revert to async close for subsequent updates
				tunnel.SetCloseOnReplaceSync(false)
				log.Infoln("Configuration reloaded successfully")
			}
		case <-usr1Ch:
			log.Infoln("Received SIGUSR1 signal, restarting application...")
			// To restart, we need to re-execute the process
			// This is a simple approach - stop and let the process manager restart us
			// For a proper restart, external process managers (systemd, docker, etc.) should be used
			log.Infoln("Application restart requested. Exiting for process manager to restart...")
			os.Exit(0)
		case <-sigCh:
			log.Infoln("Shutting down...")
			return
		}
	}
}
