package server

import (
	"encoding/json"
	"net/http"

	"github.com/phenix3443/ppanel-gateway/internal/module"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// authQuery reads service_name/secret from the query string (used by the GET
// endpoints; the POST endpoints carry them in the body).
func authQuery(r *http.Request) (name, secret string) {
	return r.URL.Query().Get("service_name"), r.URL.Query().Get("secret")
}

// register handles POST /basic/register.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var req RegisterServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, RegisterServiceResponse{Message: "invalid body"})
		return
	}
	s.modules.RegisterModule(&module.Info{
		ServiceName:    req.ServiceName,
		ServiceURL:     req.ServiceURL,
		ProxyPath:      req.ProxyPath,
		HeartbeatURL:   req.HeartbeatURL,
		Repository:     req.Repository,
		ServiceVersion: req.ServiceVersion,
		Secret:         req.Secret,
	})
	writeJSON(w, http.StatusOK, RegisterServiceResponse{Success: true})
}

// heartbeat handles GET /basic/heartbeat: validate service_name + secret and
// return the module's stored health flag.
func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	name, secret := authQuery(r)
	if !s.authorize(w, name, secret) {
		return
	}
	healthy, _ := s.modules.IsHealthy(name)
	writeJSON(w, http.StatusOK, map[string]bool{"alive": healthy})
}

// checkVersion handles GET /basic/check/version.
func (s *Server) checkVersion(w http.ResponseWriter, r *http.Request) {
	name, secret := authQuery(r)
	if !s.authorize(w, name, secret) {
		return
	}
	current, latest, updateNeeded, err := s.modules.CheckVersion(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ServiceVersionResponse{
		ServiceName:    name,
		CurrentVersion: current,
		LatestVersion:  latest,
		UpdateNeeded:   updateNeeded,
	})
}

// update handles POST /basic/update.
func (s *Server) update(w http.ResponseWriter, r *http.Request) {
	var req UpdateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, UpdateServiceResponse{Message: "invalid body"})
		return
	}
	ok, exists := s.modules.Authorized(req.ServiceName, req.Secret)
	if !exists {
		writeJSON(w, http.StatusNotFound, UpdateServiceResponse{Message: "not registered"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, UpdateServiceResponse{Message: "invalid secret"})
		return
	}
	if err := s.modules.UpdateModule(r.Context(), req.ServiceName); err != nil {
		writeJSON(w, http.StatusInternalServerError, UpdateServiceResponse{Message: "update failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, UpdateServiceResponse{Success: true, Message: "update successful"})
}

// authorize validates a (name, secret) pair, writing the appropriate error
// response (404 unknown module, 401 bad secret) and reporting whether the
// caller may proceed.
func (s *Server) authorize(w http.ResponseWriter, name, secret string) bool {
	ok, exists := s.modules.Authorized(name, secret)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not registered"})
		return false
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid secret"})
		return false
	}
	return true
}
