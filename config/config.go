// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package config holds the settings goTrust persists between launches.
//
// The file lives under the user's config directory, never next to the binary:
// a packaged app cannot usually write where it was installed, and a settings
// file that travels with the binary invites committing credentials.
//
// The file is versioned TOML. Version 1 was a flat JSON file that was never
// released; this build ignores it and starts from the defaults.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// CurrentVersion is the schema version this build writes. A file from a newer
// build is refused rather than rewritten with fields this build would drop.
const CurrentVersion = 2

// DefaultProfileName is the profile a fresh install uses.
const DefaultProfileName = "默认"

// Defaults are the values a fresh install starts from. They describe a plain
// user process: no TUN device, no system proxy, no elevation.
const (
	defaultServerPort      = 443
	defaultLoginDomain     = "local"
	defaultSocksBind       = "127.0.0.1:1080"
	defaultHTTPBind        = "127.0.0.1:1081"
	defaultKeepAlivePeriod = 90 * time.Second
)

// Config is the on-disk settings file.
type Config struct {
	Version       int           `toml:"version"`
	ActiveProfile string        `toml:"active_profile"`
	App           AppConfig     `toml:"app"`
	Proxy         ProxyConfig   `toml:"proxy"`
	Logging       LoggingConfig `toml:"logging"`
	Profiles      []Profile     `toml:"profiles"`
}

// AppConfig holds the launch and lifecycle choices that belong to the
// application rather than to one account.
type AppConfig struct {
	// Autostart mirrors the desktop's own start-at-login entry. The setting is
	// reconciled with the system at every launch (see the README).
	Autostart bool `toml:"autostart"`
	// StartHidden opens only the tray icon.
	StartHidden bool `toml:"start_hidden"`
	// AutoConnect logs in when the app becomes ready, if a saved password
	// makes that possible without asking first. A stored session is not a
	// credential: it ends with its connection.
	AutoConnect bool `toml:"auto_connect"`
	// AutoReconnect retries a lost session on the 1/2/4/8/16 s ladder, five
	// attempts at most. Off by default: a full login rotates the device token
	// and drops other clients of the same account.
	AutoReconnect bool `toml:"auto_reconnect"`
}

// LoggingConfig is global: one log file serves every profile.
type LoggingConfig struct {
	Verbose bool `toml:"verbose"`
}

// ProxyConfig is the pair of local listeners. An empty bind disables that
// listener, so a user who only wants SOCKS5 can turn HTTP off. The binds are
// global on purpose: switching accounts must not move the ports other
// applications already point at.
type ProxyConfig struct {
	SOCKS5Bind string `toml:"socks5_bind"`
	HTTPBind   string `toml:"http_bind"`
}

// Profile is one account: the gateway, the credentials, the keep-alive and
// the diagnostics that go with it.
type Profile struct {
	Name   string       `toml:"name"`
	Server ServerConfig `toml:"server"`
	Auth   AuthConfig   `toml:"auth"`
	Keep   KeepConfig   `toml:"keepalive"`
	Debug  DebugConfig  `toml:"debug"`
}

// ServerConfig identifies the aTrust gateway.
type ServerConfig struct {
	Address string `toml:"address"`
	Port    int    `toml:"port"`
}

// AuthConfig holds what the settings window collects. The password is stored
// only when the user asks for it; otherwise it is kept in memory for one run.
type AuthConfig struct {
	Username string `toml:"username"`
	// Domain is the login domain that belongs to AuthType. aTrust matches the
	// pair exactly; an empty AuthType means "use the method the gateway
	// offers" and ignores the domain.
	Domain string `toml:"domain"`
	// AuthType is the authType the gateway offers, chosen with 获取认证方式.
	// Empty selects automatically: the only offered method is used, several
	// ask the user.
	AuthType  string `toml:"auth_type"`
	StorePass bool   `toml:"store_password"`
	Password  string `toml:"password,omitempty"`
	// TOTPSecret is written only when the user opts in during a code
	// challenge. It is part of the versioned schema so that opt-in does not
	// need another version bump.
	TOTPSecret string `toml:"totp_secret,omitempty"`
}

// KeepConfig drives the application-traffic keep-alive. On gateways that drop
// an idle session, only a real request through the tunnel holds it open, so
// Targets is the setting that matters; Interval only sets the cadence.
type KeepConfig struct {
	// Enabled is a pointer so that an omitted key takes the default (on)
	// instead of the zero value. Use IsEnabled.
	Enabled  *bool    `toml:"enabled,omitempty"`
	Interval string   `toml:"interval"`
	Targets  []string `toml:"targets"`
}

// IsEnabled reports whether the keep-alive runs. A missing key means on.
func (k KeepConfig) IsEnabled() bool { return k.Enabled == nil || *k.Enabled }

// SetEnabled stores an explicit on or off.
func (k *KeepConfig) SetEnabled(on bool) { k.Enabled = &on }

// DebugConfig turns on diagnostics that are useful when a gateway behaves
// differently from the one this project was written against.
type DebugConfig struct {
	ClientData string `toml:"client_data_path"`
}

// NewProfile returns the settings one profile starts from.
func NewProfile(name string) Profile {
	enabled := true
	return Profile{
		Name:   name,
		Server: ServerConfig{Port: defaultServerPort},
		Auth:   AuthConfig{Domain: defaultLoginDomain},
		Keep:   KeepConfig{Enabled: &enabled, Interval: defaultKeepAlivePeriod.String()},
	}
}

// Default returns the settings a fresh install uses.
func Default() Config {
	return Config{
		Version:       CurrentVersion,
		ActiveProfile: DefaultProfileName,
		Proxy: ProxyConfig{
			SOCKS5Bind: defaultSocksBind,
			HTTPBind:   defaultHTTPBind,
		},
		Profiles: []Profile{NewProfile(DefaultProfileName)},
	}
}

// Dir is the directory holding the settings and the client data files.
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
	return filepath.Join(dir, "config.toml"), nil
}

// ResourcesPath is where the last granted resource snapshot is cached. It
// lives beside the settings file, so --config points the cache at the same
// place as the configuration it belongs to.
func ResourcesPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "resources.json")
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
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Version > CurrentVersion {
		return Config{}, fmt.Errorf(
			"%s uses settings version %d, which this build does not understand (latest %d)",
			path, cfg.Version, CurrentVersion)
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
	cfg.Version = CurrentVersion
	cfg.normalise()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	var encoded bytes.Buffer
	encoded.WriteString("# goTrust settings. This file can hold a password; it is written 0600.\n")
	if err := toml.NewEncoder(&encoded).Encode(cfg); err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if raw := encoded.Bytes(); len(raw) > 0 && raw[len(raw)-1] != '\n' {
		encoded.WriteByte('\n')
	}

	// Write beside the target and rename: a crash mid-write must not leave a
	// truncated settings file behind.
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("create temporary settings file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("restrict settings permissions: %w", err)
	}
	if _, err := temporary.Write(encoded.Bytes()); err != nil {
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

// Clone deep-copies the settings, so the settings window can edit a draft
// without touching the configuration a live session is using.
func (c Config) Clone() Config {
	clone := c
	clone.Profiles = make([]Profile, len(c.Profiles))
	for index, profile := range c.Profiles {
		clone.Profiles[index] = profile
		clone.Profiles[index].Keep.Targets = append([]string(nil), profile.Keep.Targets...)
		if profile.Keep.Enabled != nil {
			enabled := *profile.Keep.Enabled
			clone.Profiles[index].Keep.Enabled = &enabled
		}
	}
	return clone
}

// ProfileIndex reports where name sits in Profiles, or -1.
func (c Config) ProfileIndex(name string) int {
	for index := range c.Profiles {
		if c.Profiles[index].Name == name {
			return index
		}
	}
	return -1
}

// ActiveIndex reports where the active profile sits, or -1.
func (c Config) ActiveIndex() int { return c.ProfileIndex(c.ActiveProfile) }

// Active returns a copy of the active profile. A hand-edited file whose
// active_profile names nothing falls back to the first profile, which normalise
// also stores back into the ActiveProfile field.
func (c Config) Active() Profile {
	if index := c.ActiveIndex(); index >= 0 {
		return c.Profiles[index]
	}
	if len(c.Profiles) > 0 {
		return c.Profiles[0]
	}
	return NewProfile(DefaultProfileName)
}

// ProfileNames lists the profiles in file order.
func (c Config) ProfileNames() []string {
	names := make([]string, len(c.Profiles))
	for index := range c.Profiles {
		names[index] = c.Profiles[index].Name
	}
	return names
}

// SelectProfile makes name the active profile.
func (c *Config) SelectProfile(name string) error {
	if c.ProfileIndex(name) < 0 {
		return fmt.Errorf("profile %q does not exist", name)
	}
	c.ActiveProfile = name
	return nil
}

// AddProfile appends a new profile with default settings and makes it active.
func (c *Config) AddProfile(name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return -1, errors.New("the profile name is empty")
	}
	if c.ProfileIndex(name) >= 0 {
		return -1, fmt.Errorf("profile %q already exists", name)
	}
	c.Profiles = append(c.Profiles, NewProfile(name))
	c.ActiveProfile = name
	return len(c.Profiles) - 1, nil
}

// DuplicateProfile copies the profile at index under newName.
func (c *Config) DuplicateProfile(index int, newName string) (int, error) {
	if index < 0 || index >= len(c.Profiles) {
		return -1, errors.New("no profile is selected")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return -1, errors.New("the profile name is empty")
	}
	if c.ProfileIndex(newName) >= 0 {
		return -1, fmt.Errorf("profile %q already exists", newName)
	}
	clone := c.Profiles[index]
	clone.Name = newName
	clone.Keep.Targets = append([]string(nil), clone.Keep.Targets...)
	if clone.Keep.Enabled != nil {
		enabled := *clone.Keep.Enabled
		clone.Keep.Enabled = &enabled
	}
	c.Profiles = append(c.Profiles, clone)
	c.ActiveProfile = newName
	return len(c.Profiles) - 1, nil
}

// RenameProfile renames the profile at index, keeping it active if it was.
func (c *Config) RenameProfile(index int, newName string) error {
	if index < 0 || index >= len(c.Profiles) {
		return errors.New("no profile is selected")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("the profile name is empty")
	}
	if existing := c.ProfileIndex(newName); existing >= 0 && existing != index {
		return fmt.Errorf("profile %q already exists", newName)
	}
	wasActive := c.ActiveProfile == c.Profiles[index].Name
	c.Profiles[index].Name = newName
	if wasActive {
		c.ActiveProfile = newName
	}
	return nil
}

// DeleteProfile removes the profile at index. The last profile cannot be
// deleted: a settings file with no account has no useful state.
func (c *Config) DeleteProfile(index int) error {
	if index < 0 || index >= len(c.Profiles) {
		return errors.New("no profile is selected")
	}
	if len(c.Profiles) == 1 {
		return errors.New("the last profile cannot be deleted")
	}
	name := c.Profiles[index].Name
	c.Profiles = append(c.Profiles[:index], c.Profiles[index+1:]...)
	if c.ActiveProfile == name {
		if index >= len(c.Profiles) {
			index = len(c.Profiles) - 1
		}
		c.ActiveProfile = c.Profiles[index].Name
	}
	return nil
}

// KeepAliveInterval parses the active profile's Interval, falling back to the
// default when it is empty or malformed: a bad duration must not stop the
// keep-alive entirely.
func (c Config) KeepAliveInterval() time.Duration {
	return c.Active().KeepAliveInterval()
}

// KeepAliveInterval is the profile's cadence with the same fallback.
func (p Profile) KeepAliveInterval() time.Duration {
	if p.Keep.Interval == "" {
		return defaultKeepAlivePeriod
	}
	parsed, err := time.ParseDuration(p.Keep.Interval)
	if err != nil || parsed <= 0 {
		return defaultKeepAlivePeriod
	}
	return parsed
}

// ServerAddress is what the aTrust client expects: host without a port.
func (c Config) ServerAddress() string { return c.Active().ServerAddress() }

// ServerAddress trims the stored host.
func (p Profile) ServerAddress() string { return strings.TrimSpace(p.Server.Address) }

// ClientDataPath resolves where the active profile's aTrust client data is
// stored. The file carries cookies, the sign key and the sid ticket, so it
// lives beside the settings with owner-only permissions.
func (c Config) ClientDataPath() (string, error) {
	return c.ClientDataPathFor(c.Active())
}

// ClientDataPathFor resolves one profile's client data file. Profiles must not
// share it: a stored ticket restores whatever account obtained it.
func (c Config) ClientDataPathFor(profile Profile) (string, error) {
	if custom := strings.TrimSpace(profile.Debug.ClientData); custom != "" {
		return custom, nil
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if profile.Name == DefaultProfileName {
		// The v1 file name, so a session stored before profiles keeps working.
		return filepath.Join(dir, "client_data.json"), nil
	}
	return filepath.Join(dir, "client_data-"+profileSlug(profile.Name)+".json"), nil
}

// ProxyAddresses reports the listeners this configuration asks for.
func (c Config) ProxyAddresses() (socks5, http string) {
	return strings.TrimSpace(c.Proxy.SOCKS5Bind), strings.TrimSpace(c.Proxy.HTTPBind)
}

// Validate reports settings that cannot log in, before anything is dialled.
func (c Config) Validate() error {
	if err := c.Active().Validate(); err != nil {
		return err
	}
	socks5, http := c.ProxyAddresses()
	if socks5 == "" && http == "" {
		return errors.New("both proxy listeners are disabled")
	}
	return nil
}

// Validate reports the login fields the aTrust client needs.
func (p Profile) Validate() error {
	if p.ServerAddress() == "" {
		return errors.New("server address is empty")
	}
	if p.Server.Port <= 0 || p.Server.Port > 65535 {
		return fmt.Errorf("server port %s is out of range", strconv.Itoa(p.Server.Port))
	}
	if p.Auth.Username == "" {
		return errors.New("username is empty")
	}
	return nil
}

// normalise fills the defaults a hand-edited file may omit, so a partial file
// behaves like a complete one instead of silently disabling a listener.
func (c *Config) normalise() {
	c.Version = CurrentVersion
	if c.Proxy.SOCKS5Bind == "" && c.Proxy.HTTPBind == "" {
		c.Proxy.SOCKS5Bind = defaultSocksBind
		c.Proxy.HTTPBind = defaultHTTPBind
	}
	if len(c.Profiles) == 0 {
		c.Profiles = []Profile{NewProfile(DefaultProfileName)}
	}

	used := make(map[string]bool, len(c.Profiles))
	for index := range c.Profiles {
		profile := &c.Profiles[index]
		profile.normalise()
		profile.Name = uniqueName(strings.TrimSpace(profile.Name), used)
		used[profile.Name] = true
	}
	if c.ProfileIndex(c.ActiveProfile) < 0 {
		c.ActiveProfile = c.Profiles[0].Name
	}
}

func (p *Profile) normalise() {
	if strings.TrimSpace(p.Name) == "" {
		p.Name = DefaultProfileName
	}
	if p.Server.Port == 0 {
		p.Server.Port = defaultServerPort
	}
	if strings.TrimSpace(p.Auth.Domain) == "" {
		p.Auth.Domain = defaultLoginDomain
	}
	if p.Keep.Interval == "" {
		p.Keep.Interval = defaultKeepAlivePeriod.String()
	}
	if p.Keep.Enabled == nil {
		enabled := true
		p.Keep.Enabled = &enabled
	}
}

// uniqueName makes a profile name usable as a key: two profiles with the same
// name would also mean two profiles fighting over one client data file.
func uniqueName(name string, used map[string]bool) string {
	if name == "" {
		name = DefaultProfileName
	}
	if !used[name] {
		return name
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s (%d)", name, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}

// profileSlug turns a profile name into a portable file name stem. A name that
// needs rewriting also carries a hash, so two names cannot collapse into one
// file.
func profileSlug(name string) string {
	var slug strings.Builder
	rewritten := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '.':
			slug.WriteRune(r)
		default:
			slug.WriteByte('_')
			rewritten = true
		}
	}
	stem := slug.String()
	if stem == "" {
		stem = "profile"
		rewritten = true
	}
	if len(stem) > 40 {
		stem = stem[:40]
		rewritten = true
	}
	if !rewritten {
		return stem
	}
	var hash uint32 = 2166136261
	for _, b := range []byte(name) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	return fmt.Sprintf("%s-%08x", stem, hash)
}
