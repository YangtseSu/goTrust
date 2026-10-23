// Package config holds the settings goTrust persists between launches.
//
// The file lives under the user's config directory, never next to the binary:
// a packaged app cannot usually write where it was installed, and a settings
// file that travels with the binary invites committing credentials.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Defaults are the values a fresh install starts from. They describe a plain
// user process: no TUN device, no system proxy, no elevation.
const (
	defaultServerPort      = 443
	defaultSocksBind       = "127.0.0.1:1080"
	defaultHTTPBind        = "127.0.0.1:1081"
	defaultKeepAlivePeriod = 60 * time.Second
)

// Config is the on-disk settings file.
type Config struct {
	Server ServerConfig `json:"server"`
	Auth   AuthConfig   `json:"auth"`
	Proxy  ProxyConfig  `json:"proxy"`
	Keep   KeepConfig   `json:"keepalive"`
	Debug  DebugConfig  `json:"debug"`
}

// ServerConfig identifies the aTrust gateway.
type ServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// AuthConfig holds what the login form collects. The password is stored only
// when the user asks for it; otherwise it is kept in memory for one run.
type AuthConfig struct {
	Username  string `json:"username"`
	Domain    string `json:"domain"`
	AuthType  string `json:"auth_type"`
	StorePass bool   `json:"store_password"`
	Password  string `json:"password,omitempty"`
}

// ProxyConfig is the pair of local listeners. An empty bind disables that
// listener, so a user who only wants SOCKS5 can turn HTTP off.
type ProxyConfig struct {
	SOCKS5Bind string `json:"socks5_bind"`
	HTTPBind   string `json:"http_bind"`
}

// KeepConfig drives the application-traffic keep-alive. On gateways that drop
// an idle session, only a real request through the tunnel holds it open, so
// Targets is the setting that matters; Interval only sets the cadence.
type KeepConfig struct {
	Enabled  bool     `json:"enabled"`
	Interval string   `json:"interval"`
	Targets  []string `json:"targets"`
}

// DebugConfig turns on diagnostics that are useful when a gateway behaves
// differently from the one this project was written against.
type DebugConfig struct {
	Verbose    bool   `json:"verbose"`
	ClientData string `json:"client_data_path"`
}

// Default returns the settings a fresh install uses.
func Default() Config {
	return Config{
		Server: ServerConfig{Port: defaultServerPort},
		Proxy: ProxyConfig{
			SOCKS5Bind: defaultSocksBind,
			HTTPBind:   defaultHTTPBind,
		},
		Keep: KeepConfig{
			Enabled:  true,
			Interval: defaultKeepAlivePeriod.String(),
		},
	}
}

// Dir is the directory holding the settings and the client data file.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(base, "goTrust"), nil
}

// Path is the default settings file location.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the settings file, creating the directory if it does not exist.
// A missing file is not an error: it yields the defaults, which is what a
// first launch must see.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	return LoadFile(path)
}

// LoadFile reads one settings file. Missing files yield Default().
func LoadFile(path string) (Config, error) {
	cfg := Default()

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.normalise()
	return cfg, nil
}

// Save writes the settings file with owner-only permissions: it can hold a
// password, and a world-readable settings file has leaked far too often.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveFile(path, cfg)
}

// SaveFile writes one settings file atomically.
func SaveFile(path string, cfg Config) error {
	cfg.normalise()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	raw = append(raw, '\n')

	// Write beside the target and rename: a crash mid-write must not leave a
	// truncated settings file behind.
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return fmt.Errorf("create temporary settings file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("restrict settings permissions: %w", err)
	}
	if _, err := temporary.Write(raw); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close settings: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// KeepAliveInterval parses Interval, falling back to the default when it is
// empty or malformed: a bad duration must not stop the keep-alive entirely.
func (c Config) KeepAliveInterval() time.Duration {
	if c.Keep.Interval == "" {
		return defaultKeepAlivePeriod
	}
	parsed, err := time.ParseDuration(c.Keep.Interval)
	if err != nil || parsed <= 0 {
		return defaultKeepAlivePeriod
	}
	return parsed
}

// ServerAddress is what the aTrust client expects: host without a port.
func (c Config) ServerAddress() string {
	return strings.TrimSpace(c.Server.Address)
}

// ClientDataPath resolves where the aTrust client data is stored. The file
// carries cookies, the sign key and the sid ticket, so it lives beside the
// settings with owner-only permissions.
func (c Config) ClientDataPath() (string, error) {
	if c.Debug.ClientData != "" {
		return c.Debug.ClientData, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client_data.json"), nil
}

// ProxyAddresses reports the listeners this configuration asks for.
func (c Config) ProxyAddresses() (socks5, http string) {
	return strings.TrimSpace(c.Proxy.SOCKS5Bind), strings.TrimSpace(c.Proxy.HTTPBind)
}

// Validate reports settings that cannot work, before anything is dialled.
func (c Config) Validate() error {
	if c.ServerAddress() == "" {
		return errors.New("server address is empty")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server port %s is out of range", strconv.Itoa(c.Server.Port))
	}
	if c.Auth.Username == "" {
		return errors.New("username is empty")
	}
	socks5, http := c.ProxyAddresses()
	if socks5 == "" && http == "" {
		return errors.New("both proxy listeners are disabled")
	}
	return nil
}

// normalise fills the defaults a hand-edited file may omit, so a partial file
// behaves like a complete one instead of silently disabling a listener.
func (c *Config) normalise() {
	if c.Server.Port == 0 {
		c.Server.Port = defaultServerPort
	}
	if c.Keep.Interval == "" {
		c.Keep.Interval = defaultKeepAlivePeriod.String()
	}
	if c.Proxy.SOCKS5Bind == "" && c.Proxy.HTTPBind == "" {
		c.Proxy.SOCKS5Bind = defaultSocksBind
		c.Proxy.HTTPBind = defaultHTTPBind
	}
}
