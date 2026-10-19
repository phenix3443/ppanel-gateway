// Command gateway supervises and reverse-proxies the ppanel module(s) in a
// single container: it loads etc/ppanel.yaml, brings each configured module up
// to its latest GitHub release, serves the control plane, proxies public
// traffic, and polls module heartbeats.
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

	"github.com/phenix3443/ppanel-gateway/internal/config"
	"github.com/phenix3443/ppanel-gateway/internal/github"
	"github.com/phenix3443/ppanel-gateway/internal/module"
	"github.com/phenix3443/ppanel-gateway/internal/proxy"
	"github.com/phenix3443/ppanel-gateway/internal/server"
)

func main() {
	configPath := flag.String("f", "etc/ppanel.yaml", "config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config %s: %v", *configPath, err)
	}
	cfg.ApplyEnvOverrides()
	log.Printf("gateway name=%s mode=%s addr=%s", cfg.Name, cfg.Mode, cfg.ListenAddr())

	workDir := envOr("GATEWAY_WORKDIR", ".")
	gh := github.NewClient()
	mgr := module.NewManager(workDir, gh)
	pub := proxy.NewRouter()

	// Bootstrap configured modules: register, bring up to latest, wire proxy.
	cfg.Bootstrap(context.Background(), mgr, pub)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Active heartbeat scheduler.
	mgr.StartHeartbeatLoop(ctx, time.Minute)

	addr := cfg.ListenAddr()
	srv := &http.Server{Addr: addr, Handler: server.New(mgr, pub).Handler(workDir)}
	go func() {
		log.Printf("listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down")
	mgr.Shutdown() // stop supervised child processes
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
