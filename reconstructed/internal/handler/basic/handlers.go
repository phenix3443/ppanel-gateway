// RECONSTRUCTION — not original source. See ../../../REVERSE-ENGINEERING.md.
//
// Mirrors updater/internal/handler/basic + updater/internal/logic/basic.
// Behavior is now [E] from the 2026-10-19 decompilation:
//   - every endpoint validates the request secret (401 on mismatch, 404 if the
//     module is unknown)
//   - /basic/heartbeat returns the module's current health flag (set by the
//     active poller), not a fresh probe
//   - /basic/check/version compares current vs latest (GitHub, 30m cache)
//   - /basic/update runs the real update pipeline
package basic

import (
	"encoding/json"
	"net/http"

	"ppanel-gateway/reconstructed/internal/module"
	"ppanel-gateway/reconstructed/internal/types"
)

// Deps are the collaborators handlers need (go-zero: ServiceContext).
type Deps struct {
	Manager *module.Manager
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// authParam reads service_name/secret from the query (GET) falling back to
// nothing; POST handlers decode them from the body instead.
func authParam(r *http.Request) (name, secret string) {
	return r.URL.Query().Get("service_name"), r.URL.Query().Get("secret")
}

// RegisterServiceHandler handles POST /basic/register. [E].
func RegisterServiceHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RegisterServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, types.RegisterServiceResponse{Message: "invalid body"})
			return
		}
		d.Manager.RegisterModule(&module.Info{
			ServiceName:    req.ServiceName,
			ServiceURL:     req.ServiceURL,
			ProxyPath:      req.ProxyPath,
			HeartbeatURL:   req.HeartbeatURL,
			Repository:     req.Repository,
			ServiceVersion: req.ServiceVersion,
			Secret:         req.Secret,
		})
		writeJSON(w, http.StatusOK, types.RegisterServiceResponse{Success: true})
	}
}

// HeartbeatHandler handles GET /basic/heartbeat. [E] 0x7eddc0: validate
// service_name + secret, return the stored health flag.
func HeartbeatHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, secret := authParam(r)
		ok, exists := d.Manager.Authorized(name, secret)
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not registered"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid secret"})
			return
		}
		healthy, _ := d.Manager.IsHealthy(name)
		writeJSON(w, http.StatusOK, map[string]bool{"alive": healthy})
	}
}

// CheckServiceVersionHandler handles GET /basic/check/version. [E] 0x7eda40.
func CheckServiceVersionHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, secret := authParam(r)
		ok, exists := d.Manager.Authorized(name, secret)
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not registered"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid secret"})
			return
		}
		current, latest, updateNeeded, err := d.Manager.CheckVersion(r.Context(), name)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, types.ServiceVersionResponse{
			ServiceName:    name,
			CurrentVersion: current,
			LatestVersion:  latest,
			UpdateNeeded:   updateNeeded,
		})
	}
}

// UpdateServiceHandler handles POST /basic/update. [E] 0x7ee560: version-gate,
// download, dispatch by module kind, .bak swap + restart for a service module.
func UpdateServiceHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, types.UpdateServiceResponse{Message: "invalid body"})
			return
		}
		ok, exists := d.Manager.Authorized(req.ServiceName, req.Secret)
		if !exists {
			writeJSON(w, http.StatusNotFound, types.UpdateServiceResponse{Message: "not registered"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusUnauthorized, types.UpdateServiceResponse{Message: "invalid secret"})
			return
		}
		if err := d.Manager.UpdateModule(r.Context(), req.ServiceName); err != nil {
			writeJSON(w, http.StatusInternalServerError, types.UpdateServiceResponse{Message: "update failed: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, types.UpdateServiceResponse{Success: true, Message: "update successful"})
	}
}
