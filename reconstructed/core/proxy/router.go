// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Mirrors updater/core/proxy. Symbols [E]: Router.{AddRoute,RemoveRoute,
// ServeHTTP}, NewRouter, matchPrefix, getClientIP. Bodies [I]: a longest-prefix
// reverse-proxy router is the natural implementation of those symbols plus the
// observed /api and /sub path classes; the original body was not decompiled.
package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"sync"
)

// Router dispatches requests to upstreams by longest matching path prefix.
type Router struct {
	mu     sync.RWMutex
	routes map[string]*httputil.ReverseProxy // prefix -> proxy
}

// NewRouter constructs an empty Router. [E] symbol NewRouter.
func NewRouter() *Router {
	return &Router{routes: make(map[string]*httputil.ReverseProxy)}
}

// AddRoute mounts target as the upstream for the given path prefix.
// [E] symbol Router.AddRoute.
func (r *Router) AddRoute(prefix string, target *url.URL) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[prefix] = httputil.NewSingleHostReverseProxy(target)
}

// RemoveRoute unmounts a prefix. [E] symbol Router.RemoveRoute.
func (r *Router) RemoveRoute(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, prefix)
}

// ServeHTTP routes by longest matching prefix. [E] symbol Router.ServeHTTP.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	p := r.matchPrefix(req.URL.Path)
	proxy := r.routes[p]
	r.mu.RUnlock()
	if proxy == nil {
		http.NotFound(w, req)
		return
	}
	proxy.ServeHTTP(w, req)
}

// matchPrefix returns the longest registered prefix that path starts with, or
// "" if none. [E] symbol matchPrefix; longest-match semantics [I].
func (r *Router) matchPrefix(path string) string {
	prefixes := make([]string, 0, len(r.routes))
	for p := range r.routes {
		if hasPrefix(path, p) {
			prefixes = append(prefixes, p)
		}
	}
	if len(prefixes) == 0 {
		return ""
	}
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	return prefixes[0]
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// getClientIP extracts the client IP, honoring X-Forwarded-For / X-Real-IP.
// [E] symbol getClientIP; header precedence [I].
func getClientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		if i := indexByte(xff, ','); i >= 0 {
			return trimSpace(xff[:i])
		}
		return trimSpace(xff)
	}
	if xr := req.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
