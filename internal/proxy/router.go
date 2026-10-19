// Package proxy reverse-proxies incoming requests to registered upstream
// modules, choosing the upstream by longest matching path prefix.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Router dispatches requests to upstreams by longest matching path prefix.
type Router struct {
	mu     sync.RWMutex
	routes map[string]*httputil.ReverseProxy // prefix -> proxy
}

// NewRouter constructs an empty Router.
func NewRouter() *Router {
	return &Router{routes: make(map[string]*httputil.ReverseProxy)}
}

// AddRoute mounts target as the upstream for the given path prefix.
func (r *Router) AddRoute(prefix string, target *url.URL) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[prefix] = httputil.NewSingleHostReverseProxy(target)
}

// RemoveRoute unmounts a prefix.
func (r *Router) RemoveRoute(prefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, prefix)
}

// ServeHTTP routes by longest matching prefix, 404-ing when none matches.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	p := r.matchPrefix(req.URL.Path)
	rp := r.routes[p]
	r.mu.RUnlock()
	if rp == nil {
		http.NotFound(w, req)
		return
	}
	rp.ServeHTTP(w, req)
}

// matchPrefix returns the longest registered prefix that path starts with, or
// "" if none.
func (r *Router) matchPrefix(path string) string {
	var prefixes []string
	for p := range r.routes {
		if strings.HasPrefix(path, p) {
			prefixes = append(prefixes, p)
		}
	}
	if len(prefixes) == 0 {
		return ""
	}
	sort.Slice(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	return prefixes[0]
}
