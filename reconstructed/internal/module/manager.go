// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Mirrors updater/internal/module. Symbols [E]: Manager.{FindModule,
// RegisterModule, RemoveModule, Heartbeat, GetModuleHealth, UpdateInfo,
// QueryLastestVersion, Start, Stop, Restart, Rollback, RunBinary,
// UpdateModule}, NewManager, FreePort. The in-memory registry below [I] covers
// the registry/health/version-tracking surface. Process supervision and update
// application ([U] unrecovered bodies) are left as explicit TODO stubs rather
// than invented behavior.
package module

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Info is the stored record for one registered service. Fields correspond to
// the registration payload [E].
type Info struct {
	ServiceName    string
	ServiceURL     string
	ProxyPath      string
	HeartbeatURL   string
	Repository     string
	ServiceVersion string
	LastHeartbeat  time.Time
}

// Health is the reported liveness of a module. [I] shape.
type Health struct {
	ServiceName string
	Alive       bool
	LastSeen    time.Time
}

// Manager is the service registry + lifecycle controller.
type Manager struct {
	mu      sync.RWMutex
	modules map[string]*Info // keyed by service_name [I]
}

// NewManager constructs an empty registry. [E] symbol NewManager.
func NewManager() *Manager {
	return &Manager{modules: make(map[string]*Info)}
}

// RegisterModule records (or replaces) a module registration. [E] symbol.
func (m *Manager) RegisterModule(info *Info) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info.LastHeartbeat = time.Now()
	m.modules[info.ServiceName] = info
}

// FindModule looks up a module by service_name. [E] symbol Manager.FindModule.
func (m *Manager) FindModule(name string) (*Info, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.modules[name]
	return info, ok
}

// RemoveModule drops a registration. [E] symbol Manager.RemoveModule.
func (m *Manager) RemoveModule(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.modules, name)
}

// Heartbeat records a liveness ping for a module. [E] symbol Manager.Heartbeat.
func (m *Manager) Heartbeat(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	info, ok := m.modules[name]
	if !ok {
		return fmt.Errorf("module %q not registered", name)
	}
	info.LastHeartbeat = time.Now()
	return nil
}

// GetModuleHealth reports health for a module. [E] symbol Manager.GetModuleHealth.
// Note: the original binary spells the latest-version method "QueryLastestVersion"
// (sic); the typo is preserved below to document the real symbol.
func (m *Manager) GetModuleHealth(name string, staleAfter time.Duration) (Health, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	info, ok := m.modules[name]
	if !ok {
		return Health{}, false
	}
	return Health{
		ServiceName: name,
		Alive:       time.Since(info.LastHeartbeat) <= staleAfter,
		LastSeen:    info.LastHeartbeat,
	}, true
}

// QueryLastestVersion (sic — matches binary symbol) would query the module's
// GitHub repository for the latest release. [U] body unrecovered.
func (m *Manager) QueryLastestVersion(name string) (string, error) {
	return "", fmt.Errorf("QueryLastestVersion: not reconstructed (requires GitHub release lookup; see REVERSE-ENGINEERING.md [U])")
}

// UpdateModule downloads and applies an update for the named service.
// [E] decompilation (updater/internal/module.(*Manager).UpdateModule @0x7e67e0)
// shows a ".bak" backup-and-swap: the existing artifact is moved aside to
// "<path>.bak" before the new one is installed, which is what enables Rollback.
// The download/extract half ([U]) is not reconstructed, so this returns an
// error rather than faking a successful install.
func (m *Manager) UpdateModule(name, version string) error {
	return fmt.Errorf("UpdateModule: backup-and-swap via .bak is evidenced, but the download/extract pipeline is not reconstructed (see REVERSE-ENGINEERING.md [U])")
}

// backupSuffix is the suffix used when moving the current artifact aside before
// installing an update. [E] — ".bak" referenced by UpdateModule.
const backupSuffix = ".bak"

// FreePort returns an available TCP port. [E] symbol FreePort.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
