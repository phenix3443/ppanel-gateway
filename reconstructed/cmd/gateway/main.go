// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Entry point mirroring the original's single `-f <config>` flag [E] and the
// GATEWAY_* / SECRET_KEY env vars [E]. It wires the recovered control plane,
// the reverse proxy, static hosting, and the active heartbeat poller. The
// default-module bootstrap (download+extract+spawn ppanel-server on boot) that
// the live run showed is driven by config Modules; see config.go.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ppanel-gateway/reconstructed/core/github"
	"ppanel-gateway/reconstructed/core/proxy"
	"ppanel-gateway/reconstructed/internal/config"
	"ppanel-gateway/reconstructed/internal/handler"
	basic "ppanel-gateway/reconstructed/internal/handler/basic"
	"ppanel-gateway/reconstructed/internal/module"
)

func main() {
	configPath := flag.String("f", "etc/ppanel.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config %s: %v", *configPath, err)
	}
	cfg.ApplyEnvOverrides() // GATEWAY_PORT/GATEWAY_MODE/... [E]
	log.Printf("gateway (reconstruction) name=%s mode=%s addr=%s:%d", cfg.Name, cfg.Mode, cfg.Host, cfg.Port)

	workDir := envOr("GATEWAY_WORKDIR", ".")
	gh := github.NewClient()
	mgr := module.NewManager(workDir, gh)
	pub := proxy.NewRouter()

	// Bootstrap configured modules: register, then bring up to latest + proxy.
	cfg.Bootstrap(context.Background(), mgr, pub)

	deps := &basic.Deps{Manager: mgr}
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, deps, pub)
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir(workDir+"/static/admin"))))
	mux.Handle("/", http.FileServer(http.Dir(workDir+"/static/user")))

	// Active heartbeat scheduler. [E] live interval 1m.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	mgr.StartHeartbeatLoop(ctx, time.Minute)

	addr := cfg.ListenAddr()
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")
	mgr.Shutdown() // stop supervised child processes [E]
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
