// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Entry point mirroring the original's single `-f <config>` flag [E] and
// gateway-mode env vars [E]. This wires the recovered control plane + a static
// file server for /admin and /. It does NOT reproduce the update pipeline or
// child-process supervision ([U]); those are stubbed in their packages.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"ppanel-gateway/reconstructed/core/proxy"
	"ppanel-gateway/reconstructed/internal/handler"
	basic "ppanel-gateway/reconstructed/internal/handler/basic"
	"ppanel-gateway/reconstructed/internal/module"
)

func main() {
	// Original exposes exactly one flag: -f, default etc/ppanel.yaml [E].
	configPath := flag.String("f", "etc/ppanel.yaml", "config file path")
	flag.Parse()
	log.Printf("gateway (reconstruction) config=%s", *configPath)

	// Env vars recovered from the binary [E].
	port := envOr("GATEWAY_PORT", "8080")
	_ = os.Getenv("GATEWAY_MODE")
	_ = os.Getenv("GATEWAY_LOG_LEVEL")
	_ = os.Getenv("SECRET_KEY")

	mgr := module.NewManager()
	pub := proxy.NewRouter()
	deps := &basic.Deps{Manager: mgr}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, deps, pub)

	// Static hosting: /admin -> ./static/admin, / -> ./static/user [E config].
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir("./static/admin"))))
	mux.Handle("/", http.FileServer(http.Dir("./static/user")))

	addr := ":" + port
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
