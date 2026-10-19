// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Loads etc/ppanel.yaml [E contract token]. The minimal config that boots the
// original is Name/Host/Port/Mode (confirmed live 2026-10-19). Modules declares
// the supervised services (the live run had a default ppanel-server module);
// field names mirror the registration contract.
package config

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"

	"ppanel-gateway/reconstructed/core/proxy"
	"ppanel-gateway/reconstructed/internal/module"
)

// ModuleConfig declares one supervised module.
type ModuleConfig struct {
	ServiceName    string `yaml:"ServiceName"`
	Repository     string `yaml:"Repository"`
	ServiceURL     string `yaml:"ServiceURL"`
	ProxyPath      string `yaml:"ProxyPath"`
	HeartbeatURL   string `yaml:"HeartbeatURL"`
	ServiceVersion string `yaml:"ServiceVersion"`
	Secret         string `yaml:"Secret"`
	Kind           string `yaml:"Kind"` // "service" (default) | "static"
}

// Config is the go-zero-style gateway config.
type Config struct {
	Name    string         `yaml:"Name"`
	Host    string         `yaml:"Host"`
	Port    int            `yaml:"Port"`
	Mode    string         `yaml:"Mode"`
	Secret  string         `yaml:"Secret"` // default module secret (<= SECRET_KEY)
	Modules []ModuleConfig `yaml:"Modules"`
}

// Load reads and parses the YAML config, applying go-zero-like defaults.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.Mode == "" {
		c.Mode = "dev"
	}
	return &c, nil
}

// ApplyEnvOverrides lets the GATEWAY_* / SECRET_KEY env vars override the file.
// [E] env tokens GATEWAY_MODE/GATEWAY_PORT/GATEWAY_LOG_LEVEL/SECRET_KEY.
func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("GATEWAY_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Port = p
		}
	}
	if v := os.Getenv("GATEWAY_MODE"); v != "" {
		c.Mode = v
	}
	if v := os.Getenv("SECRET_KEY"); v != "" {
		c.Secret = v
	}
}

// ListenAddr is Host:Port.
func (c *Config) ListenAddr() string { return fmt.Sprintf("%s:%d", c.Host, c.Port) }

// Bootstrap registers each configured module, brings it up to the latest
// release (best effort — a network failure is logged, not fatal, matching the
// live run which proceeded to serve), and wires its proxy route.
func (c *Config) Bootstrap(ctx context.Context, mgr *module.Manager, pub *proxy.Router) {
	for _, mc := range c.Modules {
		secret := mc.Secret
		if secret == "" {
			secret = c.Secret
		}
		kind := mc.Kind
		if kind == "" {
			kind = module.KindService
		}
		mgr.RegisterModule(&module.Info{
			ServiceName:    mc.ServiceName,
			ServiceURL:     mc.ServiceURL,
			ProxyPath:      mc.ProxyPath,
			HeartbeatURL:   mc.HeartbeatURL,
			Repository:     mc.Repository,
			ServiceVersion: mc.ServiceVersion,
			Secret:         secret,
			Kind:           kind,
		})
		if mc.Repository != "" {
			if err := mgr.UpdateModule(ctx, mc.ServiceName); err != nil {
				log.Printf("bootstrap %s: update skipped: %v", mc.ServiceName, err)
			}
		}
		if mc.ProxyPath != "" && mc.ServiceURL != "" {
			if u, err := url.Parse(mc.ServiceURL); err == nil {
				pub.AddRoute(mc.ProxyPath, u)
			}
		}
	}
}
