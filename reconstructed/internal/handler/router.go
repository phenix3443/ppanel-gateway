// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Mirrors updater/internal/handler.Router.RegisterRoutes. The four routes and
// their methods are [E] evidenced from binary strings + pclntab handler
// symbols. The /api and /sub proxy mounts are [I].
package handler

import (
	"net/http"

	"ppanel-gateway/reconstructed/core/proxy"
	basic "ppanel-gateway/reconstructed/internal/handler/basic"
)

// RegisterRoutes wires the control-plane endpoints onto mux. [E] symbol
// Router.RegisterRoutes.
func RegisterRoutes(mux *http.ServeMux, d *basic.Deps, pub *proxy.Router) {
	// Control plane — exact methods + paths recovered from the binary [E].
	mux.Handle("POST /basic/register", basic.RegisterServiceHandler(d))
	mux.Handle("GET /basic/heartbeat", basic.HeartbeatHandler(d))
	mux.Handle("GET /basic/check/version", basic.CheckServiceVersionHandler(d))
	mux.Handle("POST /basic/update", basic.UpdateServiceHandler(d))

	// Public path classes /api and /sub proxy to registered upstreams [I].
	// Static /admin and / are served elsewhere (see cmd/gateway). The proxy
	// Router is passed in so mounts can be added as modules register.
	if pub != nil {
		mux.Handle("/api/", pub)
		mux.Handle("/sub/", pub)
	}
}
