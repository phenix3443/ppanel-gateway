// Package server exposes the gateway's control-plane HTTP API, the reverse
// proxy for public traffic, and static file hosting.
package server

import (
	"net/http"
	"path/filepath"

	"github.com/phenix3443/ppanel-gateway/internal/module"
	"github.com/phenix3443/ppanel-gateway/internal/proxy"
)

// Server serves the control-plane endpoints and routes public traffic to
// registered upstream modules.
type Server struct {
	modules *module.Manager
	proxy   *proxy.Router
}

// New returns a Server backed by the given module manager and proxy router.
// The proxy may be nil, in which case /api and /sub are not mounted.
func New(mgr *module.Manager, pr *proxy.Router) *Server {
	return &Server{modules: mgr, proxy: pr}
}

// Handler builds the gateway's HTTP handler: the control plane under /basic,
// the reverse proxy for /api and /sub, and static hosting for /admin and /
// rooted at workDir/static.
func (s *Server) Handler(workDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /basic/register", s.register)
	mux.HandleFunc("GET /basic/heartbeat", s.heartbeat)
	mux.HandleFunc("GET /basic/check/version", s.checkVersion)
	mux.HandleFunc("POST /basic/update", s.update)

	if s.proxy != nil {
		mux.Handle("/api/", s.proxy)
		mux.Handle("/sub/", s.proxy)
	}

	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.Dir(filepath.Join(workDir, "static", "admin")))))
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join(workDir, "static", "user"))))
	return mux
}
