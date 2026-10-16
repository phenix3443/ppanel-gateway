// RECONSTRUCTION — not original source. See ../../../REVERSE-ENGINEERING.md.
//
// Mirrors updater/internal/handler/basic + updater/internal/logic/basic.
// Symbols [E]: RegisterServiceHandler, HeartbeatHandler,
// CheckServiceVersionHandler, UpdateServiceHandler and their *Logic.* methods.
// Route strings [E]. Handler bodies [I]: decode -> logic -> JSON, the standard
// go-zero handler shape; original bodies were not decompiled.
package basic

import (
	"encoding/json"
	"net/http"
	"time"

	"ppanel-gateway/reconstructed/internal/module"
	"ppanel-gateway/reconstructed/internal/types"
)

// Deps are the collaborators handlers need (go-zero: ServiceContext). [I].
type Deps struct {
	Manager *module.Manager
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
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
		})
		writeJSON(w, http.StatusOK, types.RegisterServiceResponse{Success: true})
	}
}

// HeartbeatHandler handles GET /basic/heartbeat. [E].
func HeartbeatHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("service_name")
		if err := d.Manager.Heartbeat(name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"alive": true})
	}
}

// CheckServiceVersionHandler handles GET /basic/check/version. [E].
// Current version is served from the registry [I]; latest-version lookup is
// [U] unrecovered and reported as empty rather than fabricated.
func CheckServiceVersionHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("service_name")
		info, ok := d.Manager.FindModule(name)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not registered"})
			return
		}
		writeJSON(w, http.StatusOK, types.ServiceVersionResponse{
			ServiceName:    name,
			CurrentVersion: info.ServiceVersion,
			LatestVersion:  "", // [U] GitHub release lookup not reconstructed
			UpdateNeeded:   false,
		})
	}
}

// UpdateServiceHandler handles POST /basic/update. [E].
// The update pipeline body is [U] unrecovered; this returns 501 rather than
// pretending to update.
func UpdateServiceHandler(d *Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, types.UpdateServiceResponse{Message: "invalid body"})
			return
		}
		if _, ok := d.Manager.FindModule(req.ServiceName); !ok {
			writeJSON(w, http.StatusNotFound, types.UpdateServiceResponse{Message: "not registered"})
			return
		}
		writeJSON(w, http.StatusNotImplemented, types.UpdateServiceResponse{
			Message: "update pipeline not reconstructed (see REVERSE-ENGINEERING.md [U])",
		})
	}
}

// StaleAfter is the default heartbeat staleness window. [I] placeholder.
const StaleAfter = 30 * time.Second
