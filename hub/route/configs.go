package route

import (
	"net/http"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Dreamacro/clash/component/resolver"
	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/listener"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/samber/lo"
)

const (
	// restartDelay is the time to wait before sending restart signal
	// to allow HTTP response to be sent to client
	restartDelay = 100 * time.Millisecond
)

func configRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConfigs)
	r.Put("/", updateConfigs)
	r.Patch("/", patchConfigs)
	r.Post("/reload", reloadConfigs)
	r.Post("/restart", restartApp)
	return r
}

func getConfigs(w http.ResponseWriter, r *http.Request) {
	general := executor.GetGeneral()
	render.JSON(w, r, general)
}

func patchConfigs(w http.ResponseWriter, r *http.Request) {
	general := struct {
		Port        *int               `json:"port"`
		SocksPort   *int               `json:"socks-port"`
		RedirPort   *int               `json:"redir-port"`
		TProxyPort  *int               `json:"tproxy-port"`
		MixedPort   *int               `json:"mixed-port"`
		AllowLan    *bool              `json:"allow-lan"`
		BindAddress *string            `json:"bind-address"`
		Mode        *tunnel.TunnelMode `json:"mode"`
		LogLevel    *log.LogLevel      `json:"log-level"`
		IPv6        *bool              `json:"ipv6"`
	}{}
	if err := render.DecodeJSON(r.Body, &general); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	if general.Mode != nil {
		tunnel.SetMode(*general.Mode)
	}

	if general.LogLevel != nil {
		log.SetLevel(*general.LogLevel)
	}

	if general.IPv6 != nil {
		resolver.DisableIPv6 = !*general.IPv6
	}

	if general.AllowLan != nil {
		listener.SetAllowLan(*general.AllowLan)
	}

	if general.BindAddress != nil {
		listener.SetBindAddress(*general.BindAddress)
	}

	ports := listener.GetPorts()
	ports.Port = lo.FromPtrOr(general.Port, ports.Port)
	ports.SocksPort = lo.FromPtrOr(general.SocksPort, ports.SocksPort)
	ports.RedirPort = lo.FromPtrOr(general.RedirPort, ports.RedirPort)
	ports.TProxyPort = lo.FromPtrOr(general.TProxyPort, ports.TProxyPort)
	ports.MixedPort = lo.FromPtrOr(general.MixedPort, ports.MixedPort)

	listener.ReCreatePortsListeners(*ports, tunnel.TCPIn(), tunnel.UDPIn())

	render.NoContent(w, r)
}

func updateConfigs(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Path    string `json:"path"`
		Payload string `json:"payload"`
	}{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	var cfg *config.Config
	var err error

	if req.Payload != "" {
		cfg, err = executor.ParseWithBytes([]byte(req.Payload))
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}
	} else {
		if req.Path == "" {
			req.Path = C.Path.Config()
		}
		if !filepath.IsAbs(req.Path) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("path is not a absolute path"))
			return
		}

		cfg, err = executor.ParseWithPath(req.Path)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}
	}

	executor.ApplyConfig(cfg, force)
	render.NoContent(w, r)
}

func reloadConfigs(w http.ResponseWriter, r *http.Request) {
	cfg, err := executor.Parse()
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError(err.Error()))
		return
	}

	executor.ApplyConfig(cfg, false)
	render.NoContent(w, r)
}

func restartApp(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, render.M{"message": "restarting"})

	// Give time for response to be sent
	go func() {
		time.Sleep(restartDelay)
		syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	}()
}
