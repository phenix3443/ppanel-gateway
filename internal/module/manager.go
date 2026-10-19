// Package module is the gateway's in-memory service registry and lifecycle
// controller: it tracks registered modules, polls their heartbeats, and runs
// the download/extract/swap/restart update pipeline against GitHub releases.
package module

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/phenix3443/ppanel-gateway/internal/archive"
	"github.com/phenix3443/ppanel-gateway/internal/process"
	"github.com/phenix3443/ppanel-gateway/internal/version"
)

// Module kinds: a service module is a long-running binary the gateway
// supervises; a static module is a web bundle extracted in place.
const (
	KindService = "service"
	KindStatic  = "static"
)

// maxHeartbeatFailures is the consecutive-failure threshold before a module is
// marked unhealthy.
const maxHeartbeatFailures = 5

// Info is the stored record for one registered service.
type Info struct {
	ServiceName    string
	ServiceURL     string
	ProxyPath      string
	HeartbeatURL   string
	Repository     string
	ServiceVersion string
	Secret         string
	Kind           string // KindService | KindStatic
	LastHeartbeat  time.Time
	Healthy        bool

	failCount int
}

// releaseSource is the GitHub-facing capability UpdateModule needs; the github
// package satisfies it, and tests inject a fake.
type releaseSource interface {
	LatestVersion(ctx context.Context, repoURL string) (string, error)
	SystemAssetURL(ctx context.Context, repoURL, tag string) (string, error)
	Download(ctx context.Context, url, dest string) error
}

// Manager is the service registry and lifecycle controller.
type Manager struct {
	mu       sync.RWMutex
	modules  map[string]*Info
	sups     map[string]*process.Supervisor
	releases releaseSource
	workDir  string // holds modules/<name> and version.lock
	client   *http.Client
}

// NewManager constructs an empty registry rooted at workDir.
func NewManager(workDir string, releases releaseSource) *Manager {
	return &Manager{
		modules:  make(map[string]*Info),
		sups:     make(map[string]*process.Supervisor),
		releases: releases,
		workDir:  workDir,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func normalizeVersion(v string) string {
	if v != "" && v[0] != 'v' {
		return "v" + v
	}
	return v
}

// RegisterModule records (or replaces) a module registration: the version is
// normalized with a leading "v" and the module starts healthy.
func (m *Manager) RegisterModule(info *Info) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info.ServiceVersion = normalizeVersion(info.ServiceVersion)
	if info.Kind == "" {
		info.Kind = KindService
	}
	info.LastHeartbeat = time.Now()
	info.Healthy = true
	info.failCount = 0
	m.modules[info.ServiceName] = info
}

// FindModule looks up a module by service name.
func (m *Manager) FindModule(name string) (*Info, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.modules[name]
	return info, ok
}

// RemoveModule drops a registration.
func (m *Manager) RemoveModule(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.modules, name)
}

// Authorized reports whether secret matches the module's stored secret. A
// module registered with no secret accepts any.
func (m *Manager) Authorized(name, secret string) (ok, exists bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, found := m.modules[name]
	if !found {
		return false, false
	}
	if info.Secret == "" {
		return true, true
	}
	return info.Secret == secret, true
}

// IsHealthy returns a module's current health flag (set by the active poller).
func (m *Manager) IsHealthy(name string) (healthy, exists bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, found := m.modules[name]
	if !found {
		return false, false
	}
	return info.Healthy, true
}

// QueryLatestVersion returns the latest release version for the module's repo
// (the 30-minute cache lives in the release source).
func (m *Manager) QueryLatestVersion(ctx context.Context, name string) (string, error) {
	info, ok := m.FindModule(name)
	if !ok {
		return "", fmt.Errorf("module %q not registered", name)
	}
	if info.Repository == "" {
		return "", fmt.Errorf("module %q has no repository", name)
	}
	return m.releases.LatestVersion(ctx, info.Repository)
}

// CheckVersion reports the current and latest versions and whether an update is
// due (current < latest).
func (m *Manager) CheckVersion(ctx context.Context, name string) (current, latest string, updateNeeded bool, err error) {
	info, ok := m.FindModule(name)
	if !ok {
		return "", "", false, fmt.Errorf("module %q not registered", name)
	}
	latest, err = m.QueryLatestVersion(ctx, name)
	if err != nil {
		return info.ServiceVersion, "", false, err
	}
	return info.ServiceVersion, latest, version.Compare(info.ServiceVersion, latest) < 0, nil
}

// lockPath is the update mutex file.
func (m *Manager) lockPath() string { return filepath.Join(m.workDir, "version.lock") }

func (m *Manager) acquireLock() (release func(), err error) {
	p := m.lockPath()
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("update already in progress (version.lock held): %w", err)
	}
	_ = f.Close()
	return func() { _ = os.Remove(p) }, nil
}

// supervisor returns (creating if needed) the child supervisor for a service
// module whose binary is modules/<name>.
func (m *Manager) supervisor(name string) *process.Supervisor {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sups[name]; ok {
		return s
	}
	bin := filepath.Join(m.workDir, "modules", name)
	s := process.New(bin, "run")
	m.sups[name] = s
	return s
}

// UpdateModule applies an update for the named module: version-gate, then for a
// service module download the asset, stop the child, rename current->.bak, move
// new->current, restart, and roll back from .bak on failure. A static module
// extracts the bundle in place.
func (m *Manager) UpdateModule(ctx context.Context, name string) error {
	info, ok := m.FindModule(name)
	if !ok {
		return fmt.Errorf("module %q not registered", name)
	}
	latest, err := m.QueryLatestVersion(ctx, name)
	if err != nil {
		return fmt.Errorf("query latest: %w", err)
	}
	if version.Compare(info.ServiceVersion, latest) >= 0 {
		return nil // already up to date
	}

	release, err := m.acquireLock()
	if err != nil {
		return err
	}
	defer release()

	assetURL, err := m.releases.SystemAssetURL(ctx, info.Repository, latest)
	if err != nil {
		return fmt.Errorf("select asset: %w", err)
	}

	modDir := filepath.Join(m.workDir, "modules")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		return err
	}
	tarball := filepath.Join(modDir, name+".download.tar.gz")
	if err := m.releases.Download(ctx, assetURL, tarball); err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = os.Remove(tarball) }()

	switch info.Kind {
	case KindStatic:
		// Static web bundle: extract in place into modules/<name>.
		dest := filepath.Join(modDir, name)
		if err := archive.ExtractTarGz(tarball, dest); err != nil {
			return fmt.Errorf("extract static: %w", err)
		}
	default: // KindService
		if err := m.updateServiceBinary(name, tarball); err != nil {
			return err
		}
	}

	m.mu.Lock()
	info.ServiceVersion = normalizeVersion(latest)
	m.mu.Unlock()
	return nil
}

// updateServiceBinary performs the stop -> .bak swap -> move -> restart ->
// rollback sequence for a service module.
func (m *Manager) updateServiceBinary(name, tarball string) error {
	modDir := filepath.Join(m.workDir, "modules")
	staging := filepath.Join(modDir, name+".new.d")
	_ = os.RemoveAll(staging)
	if err := archive.ExtractTarGz(tarball, staging); err != nil {
		return fmt.Errorf("extract service: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	newBin, err := findBinary(staging, name)
	if err != nil {
		return err
	}
	current := filepath.Join(modDir, name)
	backup := current + ".bak"

	sup := m.supervisor(name)
	_ = sup.Stop() // stop the child before swapping

	// rename(current -> current.bak) when a current binary exists.
	if _, statErr := os.Stat(current); statErr == nil {
		if err := os.Rename(current, backup); err != nil {
			_ = sup.Start()
			return fmt.Errorf("backup current: %w", err)
		}
	}
	// move new -> current; on failure roll back from .bak and restart.
	if err := os.Rename(newBin, current); err != nil {
		if _, bErr := os.Stat(backup); bErr == nil {
			_ = os.Rename(backup, current)
		}
		_ = sup.Start()
		return fmt.Errorf("install new: %w", err)
	}
	_ = os.Chmod(current, 0o755)
	if err := sup.Start(); err != nil {
		// restart failed: roll back and try to restart the previous binary.
		if _, bErr := os.Stat(backup); bErr == nil {
			_ = os.Remove(current)
			_ = os.Rename(backup, current)
			_ = sup.Start()
		}
		return fmt.Errorf("restart after update: %w", err)
	}
	return nil
}

// findBinary locates the service executable inside an extracted staging dir:
// first a top-level file named exactly name, else the first regular file.
func findBinary(staging, name string) (string, error) {
	direct := filepath.Join(staging, name)
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct, nil
	}
	var found string
	err := filepath.WalkDir(staging, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() {
			return err
		}
		if filepath.Base(p) == name {
			found = p
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no binary %q found in release archive", name)
	}
	return found, nil
}

// StartHeartbeatLoop polls every module's heartbeat URL on the given interval
// until ctx is cancelled, marking a module unhealthy after more than
// maxHeartbeatFailures consecutive failures.
func (m *Manager) StartHeartbeatLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				m.pollOnce(ctx)
			}
		}
	}()
}

func (m *Manager) pollOnce(ctx context.Context) {
	m.mu.RLock()
	targets := make([]*Info, 0, len(m.modules))
	for _, info := range m.modules {
		if info.HeartbeatURL != "" {
			targets = append(targets, info)
		}
	}
	m.mu.RUnlock()

	for _, info := range targets {
		ok := m.probe(ctx, info.HeartbeatURL)
		m.mu.Lock()
		if ok {
			info.failCount = 0
			info.Healthy = true
			info.LastHeartbeat = time.Now()
		} else {
			info.failCount++
			if info.failCount > maxHeartbeatFailures {
				info.Healthy = false
			}
		}
		m.mu.Unlock()
	}
}

func (m *Manager) probe(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < http.StatusInternalServerError
}

// Shutdown stops every supervised child process.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	sups := make([]*process.Supervisor, 0, len(m.sups))
	for _, s := range m.sups {
		sups = append(sups, s)
	}
	m.mu.Unlock()
	for _, s := range sups {
		_ = s.Stop()
	}
}

// FreePort returns an available TCP port.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}
