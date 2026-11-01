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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// CurrentVersion is the schema version this build writes. A file from a newer
// build is refused rather than rewritten with fields this build would drop.
const CurrentVersion = 4

// DefaultProfileName is the profile a fresh install uses.
const DefaultProfileName = "默认"

// Defaults are the values a fresh install starts from. They describe a plain
// user process: no TUN device, no system proxy, no elevation.
const (
	defaultServerPort      = 443
	defaultLoginDomain     = "local"
	defaultSocksBind       = "127.0.0.1:1080"
	defaultHTTPBind        = "127.0.0.1:1081"
	defaultHandoffBind     = "127.0.0.1:1082"
	defaultKeepAlivePeriod = 90 * time.Second
)

// Config is the on-disk settings file.
//
// Only the file's own framing is global. Everything a session uses lives in
// one profile, because exactly one account is connected at a time: a second
// copy of a listener port or a log level could only disagree with the first.
type Config struct {
	Version       int       `toml:"version"`
	ActiveProfile string    `toml:"active_profile"`
	Profiles      []Profile `toml:"profiles"`

	// path is the settings file this configuration was read from, empty for a
	// configuration that was never loaded. It is not encoded: the profiles'
	// client data lives beside the file that names them, so --config moves the
	// account material with the settings instead of leaving it in the default
	// directory. A value copy carries it, which is why the settings window's
	// draft keeps pointing at the same place.
	path string
}

// AppConfig holds when the account connects and how the window starts. It is
// part of a profile because a user may want one account to connect on start and
// another to wait.
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
}

// LoggingConfig is the account's log level. The log file is one per process, so
// the level in effect is the connected account's.
type LoggingConfig struct {
	Verbose bool `toml:"verbose"`
}

// IdentityConfig is what the portal learns about this account's client. Two
// accounts may present the same device (a copy carries the identity over) or
// different ones; the file says which.
//
// The value is hand-edited only; the settings window shows it read-only and can
// open the file.
type IdentityConfig struct {
	// DeviceID is the 32-hex id the portal sees as deviceId. It is generated
	// once and seeded into every profile's client data.
	DeviceID string `toml:"device_id"`
	// MACAddresses is the hardware address the portal records for this
	// installation. Like DeviceID it is generated once and seeded into every
	// profile, because it names the machine, not the account: a per-profile
	// value would make one machine look like several endpoints.
	MACAddresses []string `toml:"mac_addresses"`
	// ClientVersion is the aTrust client version the UA reports. Empty takes
	// the default the build was written against.
	ClientVersion string `toml:"client_version"`
	// OSFamily selects the vocabulary the UA uses: "uos" reports Linux/UOS,
	// "windows" reports Windows.
	OSFamily string `toml:"os_family"`
	// OSArch is the uname -m spelling the UA carries.
	OSArch string `toml:"os_arch"`
	// UserAgent overrides the derived UA. Empty derives it from
	// ClientVersion, OSFamily and OSArch, so a version bump needs one edit.
	UserAgent string `toml:"user_agent,omitempty"`
	// ProcessName, ProcessPath and ProcessVersion replace the tunnel process
	// block the client would otherwise report. Empty keeps the client's own
	// block, which is what a build without this setting sends.
	ProcessName    string `toml:"process_name"`
	ProcessPath    string `toml:"process_path"`
	ProcessVersion string `toml:"process_version"`
}

// Identity families. The family selects the UA vocabulary. The default,
// "linux", is the truthful one; "uos" and "windows" carry the vocabulary those
// platforms expect and are only used when the file asks for them.
const (
	IdentityOSLinux   = "linux"
	IdentityOSUOS     = "uos"
	IdentityOSWindows = "windows"
)

// defaultClientVersion is the version the alternate presets report when
// client_version is empty. The truthful family carries no version.
const defaultClientVersion = "2.5.16.41"

// NewIdentity returns the derived part of a fresh identity. The random half
// (the device id and the hardware address) is filled by EnsureIdentity so that
// it is generated once per account rather than once per launch.
func NewIdentity() IdentityConfig {
	return IdentityConfig{
		OSFamily: IdentityOSLinux,
		OSArch:   archName(),
	}
}

// EnsureIdentity fills every account's device identity and reports whether
// anything changed. The caller persists the file, so the same id survives every
// launch. A hand-edited value wins: an existing non-empty id is never replaced.
//
// An account that already logged in has a device id in its client data. That
// value is adopted instead of minting a second device, so upgrading does not
// invalidate the session material already on disk.
func (c *Config) EnsureIdentity() bool {
	changed := false
	for index := range c.Profiles {
		if c.ensureProfileIdentity(&c.Profiles[index]) {
			changed = true
		}
	}
	return changed
}

func (c *Config) ensureProfileIdentity(profile *Profile) bool {
	changed := false
	if strings.TrimSpace(profile.Ident.OSFamily) == "" {
		profile.Ident.OSFamily = IdentityOSLinux
		changed = true
	}
	if strings.TrimSpace(profile.Ident.OSArch) == "" {
		profile.Ident.OSArch = archName()
		changed = true
	}
	if strings.TrimSpace(profile.Ident.DeviceID) == "" {
		if stored := c.storedDeviceID(*profile); stored != "" {
			profile.Ident.DeviceID = stored
		} else {
			profile.Ident.DeviceID = randomHexID()
		}
		changed = true
	}
	if len(profile.Ident.MACAddresses) == 0 {
		profile.Ident.MACAddresses = []string{randomMACAddress()}
		changed = true
	}
	return changed
}

// randomMACAddress returns a locally administered unicast address: the portal
// records it as the endpoint's hardware address, and the two flag bits are what
// keep a generated value from colliding with a real vendor prefix.
func randomMACAddress() string {
	mac := make([]byte, 6)
	if _, err := rand.Read(mac); err != nil {
		panic(fmt.Sprintf("generate a hardware address: %v", err))
	}
	mac[0] = mac[0]&0xfe | 0x02
	return net.HardwareAddr(mac).String()
}

// storedDeviceID returns the device id an existing profile already reported,
// or an empty string. The client data file is written by the session, not by
// this package, so the read is deliberately tolerant: a missing or malformed
// file just means "no previous device".
func (c *Config) storedDeviceID(profile Profile) string {
	if c.path == "" {
		return ""
	}
	path, err := c.ClientDataPathFor(profile)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var record struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return ""
	}
	return strings.TrimSpace(record.DeviceID)
}

// validate refuses a hand-edited device id the portal cannot read: it is the
// 32-hex form the protocol uses.
func (i IdentityConfig) validate() error {
	if id := strings.TrimSpace(i.DeviceID); id != "" {
		if err := validateDeviceID(id); err != nil {
			return err
		}
	}
	if err := validateMACAddresses(i.MACAddresses); err != nil {
		return err
	}
	switch i.OSFamily {
	case "", IdentityOSLinux, IdentityOSUOS, IdentityOSWindows:
	default:
		return fmt.Errorf("identity os_family %q is not one of %q, %q or %q",
			i.OSFamily, IdentityOSLinux, IdentityOSUOS, IdentityOSWindows)
	}
	return nil
}

// validateDeviceID refuses an id the portal cannot read: it is the 32-hex form
// the protocol uses.
func validateDeviceID(id string) error {
	if len(id) != 32 {
		return fmt.Errorf("device_id %q is not 32 hex characters", id)
	}
	if _, err := hex.DecodeString(id); err != nil {
		return fmt.Errorf("device_id %q is not hexadecimal", id)
	}
	return nil
}

// validateMACAddresses refuses an address the portal cannot read.
func validateMACAddresses(addresses []string) error {
	for _, address := range addresses {
		if _, err := net.ParseMAC(strings.TrimSpace(address)); err != nil {
			return fmt.Errorf("mac_addresses entry %q is not a hardware address", address)
		}
	}
	return nil
}

// UserAgentString is the UA the portal sees. An explicit user_agent wins.
// Otherwise the default family states what the client is; the uos and windows
// presets send the vocabulary those platforms use, only when the file selects
// them.
func (i IdentityConfig) UserAgentString() string {
	if strings.TrimSpace(i.UserAgent) != "" {
		return i.UserAgent
	}
	arch := i.OSArch
	if arch == "" {
		arch = archName()
	}
	switch i.OSFamily {
	case IdentityOSWindows:
		version := i.ClientVersion
		if version == "" {
			version = defaultClientVersion
		}
		return fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) aTrustTray/%s Chrome/83.0.4103.94 Electron/9.0.2 Safari/537.36 aTrustTray-Windows-Plat-Windows10-%s SPCClientType",
			version, uaArch(arch))
	case IdentityOSUOS:
		version := i.ClientVersion
		if version == "" {
			version = defaultClientVersion
		}
		return fmt.Sprintf("Mozilla/5.0 (X11; Linux %s) AppleWebKit/537.36 (KHTML, like Gecko) aTrustTray/%s Chrome/83.0.4103.94 Electron/9.0.2 Safari/537.36 aTrustTray-Linux-Plat-UOS-%s SPCClientType",
			arch, version, uaArch(arch))
	default:
		if strings.TrimSpace(i.ClientVersion) != "" {
			return fmt.Sprintf("goTrust/%s (aTrust client; Linux %s)", i.ClientVersion, arch)
		}
		return fmt.Sprintf("goTrust (aTrust client; Linux %s)", arch)
	}
}

// uaArch is the short architecture suffix the UA uses.
func uaArch(arch string) string {
	switch arch {
	case "x86_64", "amd64":
		return "x64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return arch
	}
}

// archName is the uname -m spelling the protocol expects, not GOARCH.
func archName() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i686"
	default:
		return runtime.GOARCH
	}
}

// randomHexID returns the 32-uppercase-hex form the protocol uses for
// deviceId.
func randomHexID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		// A failing entropy source is fatal for a login client; prefer a
		// panic over an identity that silently repeats on every launch.
		panic(fmt.Sprintf("generate an identity id: %v", err))
	}
	return strings.ToUpper(hex.EncodeToString(buffer))
}

// newProfileID returns the opaque local identity a profile keeps for its
// lifetime. It never reaches the gateway; it only keys local state, so the
// format is free to change as long as it stays unique and TOML-safe.
func newProfileID() string { return rand.Text() }

// ProxyConfig is the pair of local listeners. An empty bind disables that
// listener, so a user who only wants SOCKS5 can turn HTTP off. The binds are
// global on purpose: switching accounts must not move the ports other
// applications already point at.
type ProxyConfig struct {
	SOCKS5Bind string `toml:"socks5_bind"`
	HTTPBind   string `toml:"http_bind"`
}

// HandoffConfig holds the loopback endpoint that serves the PAC. A missing
// listen takes the default; an explicit empty value disables the endpoint, so
// the pointer is what keeps those two apart.
type HandoffConfig struct {
	Listen *string `toml:"listen,omitempty"`
}

// Address is the endpoint the PAC is served on, empty to disable it.
func (h HandoffConfig) Address() string {
	if h.Listen == nil {
		return defaultHandoffBind
	}
	return strings.TrimSpace(*h.Listen)
}

// SetListen stores an explicit endpoint, empty to disable it.
func (h *HandoffConfig) SetListen(address string) {
	address = strings.TrimSpace(address)
	h.Listen = &address
}

// Profile is one account: the gateway, the credentials, the keep-alive and
// the diagnostics that go with it.
type Profile struct {
	// ID is this profile's stable local identity. It is generated once, kept
	// across renames, and never reused, so per-profile state can key off it
	// instead of the mutable name. A hand-edited value stays unless another
	// profile already owns it.
	ID     string         `toml:"id"`
	Name   string         `toml:"name"`
	Server ServerConfig   `toml:"server"`
	Auth   AuthConfig     `toml:"auth"`
	App    AppConfig      `toml:"app"`
	Proxy  ProxyConfig    `toml:"proxy"`
	Log    LoggingConfig  `toml:"logging"`
	Hand   HandoffConfig  `toml:"handoff"`
	Keep   KeepConfig     `toml:"keepalive"`
	Ident  IdentityConfig `toml:"identity"`
	// AutoReconnect retries this account's lost session on the 1/2/4/8/16 s
	// ladder, five attempts at most. It belongs to the account rather than the
	// process: a full login rotates the device token and drops the account's
	// other clients, which one account may accept and another may not.
	AutoReconnect bool `toml:"auto_reconnect"`
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
	// AuthType is the authType the gateway offers, chosen with Load methods.
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

// NewProfile returns the settings one profile starts from. A new account
// stores its password unless the user unticks Save password: the settings file is
// mode 0600, and a stored password is what lets auto-connect work. An existing
// profile keeps whatever its file says.
func NewProfile(name string) Profile {
	enabled := true
	listen := defaultHandoffBind
	return Profile{
		ID:     newProfileID(),
		Name:   name,
		Server: ServerConfig{Port: defaultServerPort},
		Auth:   AuthConfig{Domain: defaultLoginDomain, StorePass: true},
		Proxy:  ProxyConfig{SOCKS5Bind: defaultSocksBind, HTTPBind: defaultHTTPBind},
		Hand:   HandoffConfig{Listen: &listen},
		Keep:   KeepConfig{Enabled: &enabled, Interval: defaultKeepAlivePeriod.String()},
		Ident:  NewIdentity(),
	}
}

// EnsureProfileIDs fills a missing id and breaks a duplicate, and reports
// whether anything changed. The caller persists the file, so an upgraded
// settings file gets ids once instead of a new one on every load. A hand-edited
// id wins unless another profile already owns it.
func (c *Config) EnsureProfileIDs() bool {
	changed := false
	used := make(map[string]bool, len(c.Profiles))
	for index := range c.Profiles {
		profile := &c.Profiles[index]
		id := profile.ID
		if strings.TrimSpace(id) == "" || used[id] {
			id = newProfileID()
			for used[id] {
				id = newProfileID()
			}
			profile.ID = id
			changed = true
		}
		used[id] = true
	}
	return changed
}

// Default returns the settings a fresh install uses.
func Default() Config {
	return Config{
		Version:       CurrentVersion,
		ActiveProfile: DefaultProfileName,
		Profiles:      []Profile{NewProfile(DefaultProfileName)},
	}
}

// Dir is the directory holding the settings and the client data files.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate the user config directory: %w", err)
	}
	return filepath.Join(base, "gotrust"), nil
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

// legacyV3 is the shape this build used to write: one set of application,
// proxy, handoff, logging and identity settings beside the accounts. Reading it
// is the whole migration, and the fields are kept as written so that a value the
// user set is not silently replaced by a default.
type legacyV3 struct {
	Version       int            `toml:"version"`
	ActiveProfile string         `toml:"active_profile"`
	App           legacyApp      `toml:"app"`
	Proxy         ProxyConfig    `toml:"proxy"`
	Handoff       legacyHandoff  `toml:"handoff"`
	Logging       LoggingConfig  `toml:"logging"`
	Identity      IdentityConfig `toml:"identity"`
	Profiles      []legacyV3Prof `toml:"profiles"`

	// path is where the file was read from; LoadFile fills it after decoding.
	path string
}

// legacyApp accepted an auto-reconnect key that now belongs to the account.
// Reading it here is what lets a file written by the previous build load; the
// value is discarded rather than moved, because that build only ever wrote the
// default and there is no account it could be attributed to.
type legacyApp struct {
	Autostart     bool `toml:"autostart"`
	StartHidden   bool `toml:"start_hidden"`
	AutoConnect   bool `toml:"auto_connect"`
	AutoReconnect bool `toml:"auto_reconnect"`
}

func (l legacyApp) app() AppConfig {
	return AppConfig{Autostart: l.Autostart, StartHidden: l.StartHidden, AutoConnect: l.AutoConnect}
}

type legacyHandoff struct {
	Listen *string `toml:"listen"`
}

type legacyV3Prof struct {
	ID            string         `toml:"id"`
	Name          string         `toml:"name"`
	Server        ServerConfig   `toml:"server"`
	Auth          AuthConfig     `toml:"auth"`
	Keep          KeepConfig     `toml:"keepalive"`
	AutoReconnect bool           `toml:"auto_reconnect"`
	Client        IdentityConfig `toml:"identity"`
	// Debug held the custom client data path. The field is read so that a file
	// carrying it still loads, and then dropped: the path is derived from the
	// account name now.
	Debug struct {
		ClientData string `toml:"client_data_path"`
	} `toml:"debug"`
}

// migrateV3 copies the shared settings into every account. The values are
// stamped in as written, so an account keeps exactly the behaviour it had
// before the file was split.
func (l legacyV3) migrateV3() Config {
	cfg := Config{Version: CurrentVersion, ActiveProfile: l.ActiveProfile}
	cfg.Profiles = make([]Profile, 0, len(l.Profiles))
	// A file without a hardware address predates the field. Every account then
	// takes the same generated value: the shared identity is what the file
	// described, and minting one per account would make one machine look like
	// several devices on the first launch after the upgrade.
	if len(l.Identity.MACAddresses) == 0 {
		l.Identity.MACAddresses = []string{randomMACAddress()}
	}
	for _, profile := range l.Profiles {
		identity := l.Identity
		// An account-level identity wins field by field; empty means "the
		// shared value", which is now the starting point.
		if id := strings.TrimSpace(profile.Client.DeviceID); id != "" {
			identity.DeviceID = id
		}
		if len(profile.Client.MACAddresses) > 0 {
			identity.MACAddresses = profile.Client.MACAddresses
		}
		if v := strings.TrimSpace(profile.Client.ClientVersion); v != "" {
			identity.ClientVersion = v
		}
		if v := strings.TrimSpace(profile.Client.OSFamily); v != "" {
			identity.OSFamily = v
		}
		if v := strings.TrimSpace(profile.Client.OSArch); v != "" {
			identity.OSArch = v
		}
		if v := strings.TrimSpace(profile.Client.UserAgent); v != "" {
			identity.UserAgent = v
		}
		if v := strings.TrimSpace(profile.Client.ProcessName); v != "" {
			identity.ProcessName = v
		}
		if v := strings.TrimSpace(profile.Client.ProcessPath); v != "" {
			identity.ProcessPath = v
		}
		if v := strings.TrimSpace(profile.Client.ProcessVersion); v != "" {
			identity.ProcessVersion = v
		}
		cfg.Profiles = append(cfg.Profiles, Profile{
			ID:            profile.ID,
			Name:          profile.Name,
			Server:        profile.Server,
			Auth:          profile.Auth,
			App:           l.App.app(),
			Proxy:         l.Proxy,
			Log:           l.Logging,
			Hand:          HandoffConfig{Listen: l.Handoff.Listen},
			Keep:          profile.Keep,
			Ident:         identity,
			AutoReconnect: profile.AutoReconnect,
		})
	}
	return cfg
}

// decode reads one settings file. A file this build wrote is decoded straight
// into Config; only a file from before the settings moved into the accounts has
// its keys read through the old shape and distributed.
func decode(raw []byte) (Config, error) {
	var head struct {
		Version int `toml:"version"`
	}
	if _, err := toml.Decode(string(raw), &head); err != nil {
		return Config{}, err
	}
	if head.Version > CurrentVersion {
		return Config{}, fmt.Errorf("settings version %d is newer than this build understands (%d)",
			head.Version, CurrentVersion)
	}

	if head.Version >= CurrentVersion {
		var cfg Config
		meta, err := toml.Decode(string(raw), &cfg)
		if err != nil {
			return Config{}, err
		}
		if err := rejectUnknown(meta); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	var legacy legacyV3
	meta, err := toml.Decode(string(raw), &legacy)
	if err != nil {
		return Config{}, err
	}
	if err := rejectUnknown(meta); err != nil {
		return Config{}, err
	}
	return legacy.migrateV3(), nil
}

// rejectUnknown refuses a key the schema does not know: it is a typo or a hand
// edit, and running a configuration the file does not describe is worse.
func rejectUnknown(meta toml.MetaData) error {
	undecoded := meta.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}
	keys := make([]string, len(undecoded))
	for index, key := range undecoded {
		keys[index] = key.String()
	}
	return fmt.Errorf("unknown settings: %s", strings.Join(keys, ", "))
}

// LoadFile reads one settings file. Missing files yield Default(). Profile ids
// are left as read; LoadOrCreate repairs them and persists the result, because
// an id minted for a read-only load would be lost on the next launch.
func LoadFile(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := Default()
		cfg.path = path
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	cfg, err := decode(raw)
	if err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	cfg.path = path
	cfg.normalise()
	return cfg, nil
}

// LoadOrCreate loads the settings at path, or the default location when path is
// empty, and returns them together with the file they came from. The install
// identity is generated once and written back, so the portal sees the same
// device id after a restart; a hand-edited value stays.
func LoadOrCreate(path string) (Config, string, error) {
	var (
		cfg Config
		err error
	)
	if path != "" {
		cfg, err = LoadFile(path)
	} else {
		cfg, err = Load()
		if err == nil {
			path, err = Path()
		}
	}
	if err != nil {
		return Config{}, path, err
	}
	changed := cfg.EnsureIdentity()
	if cfg.EnsureProfileIDs() {
		changed = true
	}
	if changed {
		if err := SaveFile(path, cfg); err != nil {
			return Config{}, path, fmt.Errorf("save the generated settings: %w", err)
		}
	}
	return cfg, path, nil
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

// SaveFile writes one settings file atomically. The written copy carries an id
// for every profile, so a hand-built configuration cannot leave the file
// without one; NewProfile and LoadOrCreate already supply them.
func SaveFile(path string, cfg Config) error {
	cfg.Version = CurrentVersion
	cfg.normalise()
	cfg.EnsureProfileIDs()

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
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync settings: %w", err)
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
		clone.Profiles[index] = cloneProfile(profile)
	}
	return clone
}

// ForDisk returns a copy that the settings file may carry. The TOTP secret is
// a second credential for the same account, so it follows the password toggle:
// unchecking Save password must not leave it on disk. The in-memory configuration
// keeps both for the run that wrote it, which is why this is a copy.
func (c Config) ForDisk() Config {
	disk := c.Clone()
	for index := range disk.Profiles {
		if !disk.Profiles[index].Auth.StorePass {
			disk.Profiles[index].Auth.Password = ""
			disk.Profiles[index].Auth.TOTPSecret = ""
		}
	}
	return disk
}

// ActiveProfileRef is the active profile for editing: a pointer into the slice,
// so a caller can change one field of it. Active() returns a copy, which is what
// a reader wants but a writer cannot use.
func (c *Config) ActiveProfileRef() *Profile {
	if index := c.ActiveIndex(); index >= 0 {
		return &c.Profiles[index]
	}
	if len(c.Profiles) > 0 {
		return &c.Profiles[0]
	}
	return nil
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

// NewProfileNamed appends an account with default settings and its own device
// identity, and makes it active. The new account presents a separate device to
// the portal, which is what an unrelated account is; an account that should look
// like the one already configured is added by copying it instead.
func (c *Config) NewProfileNamed(name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return -1, errors.New("the profile name is empty")
	}
	if c.ProfileIndex(name) >= 0 {
		return -1, fmt.Errorf("profile %q already exists", name)
	}
	profile := NewProfile(name)
	// A blank account starts with the listeners the current one uses, so
	// switching to it does not move the ports an application already points at.
	// The identity is left to EnsureIdentity, which mints a new device.
	if active := c.ActiveProfileRef(); active != nil {
		profile.Proxy = active.Proxy
		profile.Hand = active.Hand
	}
	c.Profiles = append(c.Profiles, profile)
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
	// The copy keeps the identity of the profile it came from: a copied account
	// is the same machine to the portal, which is what a second account on one
	// host means. Only the name and the local id are new, so the copy owns its
	// own client data file and its own place in the list.
	clone := cloneProfile(c.Profiles[index])
	clone.ID = newProfileID()
	clone.Name = newName
	c.Profiles = append(c.Profiles, clone)
	c.ActiveProfile = newName
	return len(c.Profiles) - 1, nil
}

// cloneProfile deep-copies one profile, so editing the copy cannot reach into
// the slice it came from.
func cloneProfile(profile Profile) Profile {
	clone := profile
	clone.Keep.Targets = append([]string(nil), profile.Keep.Targets...)
	if profile.Keep.Enabled != nil {
		enabled := *profile.Keep.Enabled
		clone.Keep.Enabled = &enabled
	}
	clone.Ident.MACAddresses = append([]string(nil), profile.Ident.MACAddresses...)
	if profile.Hand.Listen != nil {
		listen := *profile.Hand.Listen
		clone.Hand.Listen = &listen
	}
	return clone
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

// MinimumKeepAliveInterval is the shortest cadence the app accepts. Every probe
// is a real HTTP request over the tunnel, so a hand-edited value below this
// would hammer the gateway and the account's session.
const MinimumKeepAliveInterval = 10 * time.Second

// KeepAliveInterval is the profile's cadence with the same fallback. A positive
// value below the minimum is clamped instead of reverting to the default: the
// user asked for a fast probe, not for the 90 s default.
func (p Profile) KeepAliveInterval() time.Duration {
	if p.Keep.Interval == "" {
		return defaultKeepAlivePeriod
	}
	parsed, err := time.ParseDuration(p.Keep.Interval)
	if err != nil || parsed <= 0 {
		return defaultKeepAlivePeriod
	}
	if parsed < MinimumKeepAliveInterval {
		return MinimumKeepAliveInterval
	}
	return parsed
}

// ServerAddress is what the aTrust client expects: host without a port.
func (c Config) ServerAddress() string { return c.Active().ServerAddress() }

// ServerAddress trims the stored host.
func (p Profile) ServerAddress() string { return strings.TrimSpace(p.Server.Address) }

// ClientDataPathFor resolves one profile's client data file. The path is
// derived from the name, never configured: two accounts that pointed at one
// file would overwrite each other's sign key and ticket, and the result is a
// tunnel that comes up while every authorization through it is refused. The
// file lives beside the settings file that names the profile, so --config moves
// the account material too; a configuration that was never loaded falls back to
// the user config directory.
func (c Config) ClientDataPathFor(profile Profile) (string, error) {
	dir := ""
	if c.path != "" {
		dir = filepath.Dir(c.path)
	} else {
		defaultDir, err := Dir()
		if err != nil {
			return "", err
		}
		dir = defaultDir
	}
	if profile.Name == DefaultProfileName {
		// The v1 file name, so a session stored before profiles keeps working.
		return filepath.Join(dir, "client_data.json"), nil
	}
	return filepath.Join(dir, "client_data-"+profileSlug(profile.Name)+".json"), nil
}

// ProxyAddresses reports the listeners this account asks for.
func (p Profile) ProxyAddresses() (socks5, http string) {
	return strings.TrimSpace(p.Proxy.SOCKS5Bind), strings.TrimSpace(p.Proxy.HTTPBind)
}

// Validate reports settings that cannot log in, before anything is dialled.
func (c Config) Validate() error {
	// Every account is checked, not only the active one: a typo in an account
	// that is not in use yet must not surface as a failed login later.
	for _, profile := range c.Profiles {
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("account %q: %w", profile.Name, err)
		}
	}
	if len(c.Profiles) == 0 {
		return errors.New("the file has no account")
	}
	active := c.Active()
	socks5, http := active.ProxyAddresses()
	if socks5 == "" && http == "" {
		return errors.New("both proxy listeners are disabled")
	}
	for _, listener := range []struct{ name, bind string }{
		{"SOCKS5", socks5},
		{"HTTP", http},
	} {
		if listener.bind == "" {
			continue
		}
		if err := ValidateBind(listener.bind); err != nil {
			return fmt.Errorf("%s proxy bind: %w", listener.name, err)
		}
	}
	if listen := active.Hand.Address(); listen != "" {
		if err := ValidateBind(listen); err != nil {
			return fmt.Errorf("handoff bind: %w", err)
		}
	}
	return nil
}

// ValidateBind checks one proxy listener address. Both listeners stay on the
// loopback interface: a wider bind would let the machine's network use the
// tunnel, and a host name would be resolved by the system resolver the app
// deliberately avoids.
func ValidateBind(bind string) error {
	host, port, err := net.SplitHostPort(bind)
	if err != nil {
		return fmt.Errorf("%q is not host:port", bind)
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return fmt.Errorf("port %q is out of range 1-65535", port)
	}
	if host == "" {
		return errors.New("the host is empty, which listens on every interface")
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("host %q is not an IP address", host)
	}
	if !ip.IsLoopback() {
		return fmt.Errorf("host %q is not a loopback address", host)
	}
	return nil
}

// Validate reports the fields the aTrust client and the listeners need.
func (p Profile) Validate() error {
	if err := p.Ident.validate(); err != nil {
		return err
	}
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
	if len(c.Profiles) == 0 {
		c.Profiles = []Profile{NewProfile(DefaultProfileName)}
	}
	// A file that predates the split carried one identity and one set of
	// listeners for the whole install. The migration stamps them into every
	// account, so a launch right after the upgrade presents the device the
	// portal already knows rather than a new one per account.

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
	if p.Proxy.SOCKS5Bind == "" && p.Proxy.HTTPBind == "" {
		p.Proxy.SOCKS5Bind = defaultSocksBind
		p.Proxy.HTTPBind = defaultHTTPBind
	}
	if strings.TrimSpace(p.Ident.OSFamily) == "" {
		p.Ident.OSFamily = IdentityOSLinux
	}
	if strings.TrimSpace(p.Ident.OSArch) == "" {
		p.Ident.OSArch = archName()
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
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			slug.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			// Kept for readability, but a case-insensitive filesystem cannot
			// tell "Work" from "work", so the stem takes a hash suffix below.
			slug.WriteRune(r)
			rewritten = true
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
