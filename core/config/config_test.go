// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultHasOneProfileAndBothListeners(t *testing.T) {
	cfg := Default()
	if cfg.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", cfg.Version, CurrentVersion)
	}
	if len(cfg.Profiles) != 1 || cfg.ActiveProfile != DefaultProfileName {
		t.Fatalf("profiles = %v, active = %q", cfg.ProfileNames(), cfg.ActiveProfile)
	}
	socks5, httpBind := cfg.ProxyAddresses()
	if socks5 == "" || httpBind == "" {
		t.Fatalf("defaults disable a listener: socks5=%q http=%q", socks5, httpBind)
	}
	profile := cfg.Active()
	if profile.Server.Port != 443 {
		t.Fatalf("default port = %d, want 443", profile.Server.Port)
	}
	if profile.Auth.Domain != "local" {
		t.Fatalf("default login domain = %q, want local", profile.Auth.Domain)
	}
	if !profile.Keep.IsEnabled() || profile.KeepAliveInterval() != 90*time.Second {
		t.Fatalf("default keep-alive = %v / %v", profile.Keep.IsEnabled(), profile.KeepAliveInterval())
	}
}

func TestTOMLRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")

	want := Default()
	want.ActiveProfile = "工作"
	want.App.Autostart = true
	want.App.AutoConnect = true
	want.Logging.Verbose = true
	want.Proxy.HTTPBind = ""
	if _, err := want.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	index := want.ProfileIndex("工作")
	want.Profiles[index].Server.Address = "vpn.example.com"
	want.Profiles[index].Server.Port = 4430
	want.Profiles[index].Auth.Username = "alice"
	want.Profiles[index].Auth.Domain = "Radius"
	want.Profiles[index].Auth.StorePass = true
	want.Profiles[index].Auth.Password = `pa"ss\word`
	want.Profiles[index].Keep.SetEnabled(false)
	want.Profiles[index].Keep.Interval = "90s"
	want.Profiles[index].Keep.Targets = []string{"http://192.0.2.5/health"}
	want.Profiles[index].Debug.ClientData = "/tmp/custom-client-data.json"

	if err := SaveFile(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "[[profiles]]") {
		t.Fatalf("saved file is not profile TOML:\n%s", raw)
	}
	if !strings.Contains(string(raw), "active_profile = \"工作\"") {
		t.Fatalf("active profile not written:\n%s", raw)
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ActiveProfile != "工作" || len(got.Profiles) != 2 {
		t.Fatalf("profiles = %v, active = %q", got.ProfileNames(), got.ActiveProfile)
	}
	profile := got.Active()
	if profile.Server.Address != "vpn.example.com" || profile.Server.Port != 4430 {
		t.Fatalf("server = %+v", profile.Server)
	}
	if profile.Auth.Password != want.Profiles[index].Auth.Password {
		t.Fatalf("password round-trip lost the value")
	}
	if profile.Auth.Domain != "Radius" || !profile.Auth.StorePass {
		t.Fatalf("auth = %+v", profile.Auth)
	}
	if profile.Keep.IsEnabled() {
		t.Fatal("keep-alive enabled flag was lost")
	}
	if len(profile.Keep.Targets) != 1 || profile.Keep.Targets[0] != "http://192.0.2.5/health" {
		t.Fatalf("targets = %v", profile.Keep.Targets)
	}
	if got.Proxy.HTTPBind != "" {
		t.Fatalf("http bind = %q, want empty (disabled)", got.Proxy.HTTPBind)
	}
	if socks5, _ := got.ProxyAddresses(); socks5 == "" {
		t.Fatal("socks bind was lost")
	}
	if !got.Logging.Verbose || !got.App.Autostart || !got.App.AutoConnect {
		t.Fatalf("global settings lost: %+v %+v", got.App, got.Logging)
	}
}

func TestSaveRestrictsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := SaveFile(path, Default()); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %o, want 600: the file can hold a password", mode)
	}
}

// A hand-edited file that omits most fields must keep working with defaults.
func TestPartialFileTakesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	partial := `
[[profiles]]
[profiles.server]
address = "vpn.example.com"
[profiles.auth]
username = "alice"
`
	if err := os.WriteFile(path, []byte(partial), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	socks5, httpBind := cfg.ProxyAddresses()
	if socks5 == "" || httpBind == "" {
		t.Fatalf("partial file disabled a listener: socks5=%q http=%q", socks5, httpBind)
	}
	profile := cfg.Active()
	if profile.Name != DefaultProfileName {
		t.Fatalf("name = %q, want %q", profile.Name, DefaultProfileName)
	}
	if profile.Server.Port != 443 {
		t.Fatalf("port = %d, want the default 443", profile.Server.Port)
	}
	if profile.Auth.Domain != "local" {
		t.Fatalf("domain = %q, want local", profile.Auth.Domain)
	}
	if !profile.Keep.IsEnabled() {
		t.Fatal("an omitted keep-alive setting must default to enabled")
	}
	if profile.KeepAliveInterval() != 90*time.Second {
		t.Fatalf("interval = %v, want 90s", profile.KeepAliveInterval())
	}
	if cfg.ActiveProfile != DefaultProfileName {
		t.Fatalf("active = %q, want the only profile", cfg.ActiveProfile)
	}
}

// An explicitly disabled keep-alive must stay disabled.
func TestKeepAliveExplicitlyDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `
[[profiles]]
name = "默认"
[profiles.keepalive]
enabled = false
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Active().Keep.IsEnabled() {
		t.Fatal("enabled = false was ignored")
	}
}

// Version 1 was never released, so a leftover JSON file is ignored rather than
// migrated; there is no real v1 state to preserve.
func TestVersionOneJSONIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	dir := filepath.Join(home, "gotrust")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `{"server":{"address":"vpn.example.com","port":8443}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Active().Server.Address != "" {
		t.Fatalf("the unreleased JSON file was read: %+v", cfg.Active().Server)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); !os.IsNotExist(err) {
		t.Fatalf("Load created a settings file before the user saved: %v", err)
	}
}

func TestNewerVersionIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("version = 99\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadFile(path); err == nil {
		t.Fatal("a newer settings version was accepted")
	}
}

func TestProfileOperations(t *testing.T) {
	cfg := Default()
	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if cfg.ActiveProfile != "工作" {
		t.Fatalf("active = %q, want the new profile", cfg.ActiveProfile)
	}
	if _, err := cfg.AddProfile("工作"); err == nil {
		t.Fatal("duplicate name accepted")
	}

	index := cfg.ProfileIndex("工作")
	if index < 0 {
		t.Fatal("profile not found")
	}
	copied, err := cfg.DuplicateProfile(index, "工作 副本")
	if err != nil || copied != 2 {
		t.Fatalf("duplicate: %d %v", copied, err)
	}
	if cfg.ActiveProfile != "工作 副本" {
		t.Fatalf("active = %q after duplicate", cfg.ActiveProfile)
	}

	if err := cfg.DeleteProfile(2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("profiles = %v", cfg.ProfileNames())
	}
	cfg.Profiles = cfg.Profiles[:1]
	cfg.ActiveProfile = cfg.Profiles[0].Name
	if err := cfg.DeleteProfile(0); err == nil {
		t.Fatal("deleting the last profile succeeded")
	}
}

func TestDuplicateProfileIsIndependent(t *testing.T) {
	cfg := Default()
	profile := cfg.Active()
	profile.Keep.Targets = []string{"http://192.0.2.5/"}
	cfg.Profiles[0] = profile

	index, err := cfg.DuplicateProfile(0, "副本")
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	cfg.Profiles[index].Keep.Targets[0] = "http://192.0.2.6/"
	if cfg.Profiles[0].Keep.Targets[0] != "http://192.0.2.5/" {
		t.Fatal("profiles share their target slice")
	}
}

func TestProfileNamesAreUniqueOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `
[[profiles]]
name = "a"
[[profiles]]
name = "a"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Profiles) != 2 || cfg.Profiles[0].Name == cfg.Profiles[1].Name {
		t.Fatalf("duplicate names survived: %v", cfg.ProfileNames())
	}
}

func TestClientDataPathIsPerProfile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg := Default()
	path, err := cfg.ClientDataPathFor(cfg.Active())
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if filepath.Base(path) != "client_data.json" {
		t.Fatalf("default profile path = %q, want the v1 file name", path)
	}

	work := NewProfile("工作")
	first, err := cfg.ClientDataPathFor(work)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if first == path {
		t.Fatal("profiles share one client data file")
	}
	other, err := cfg.ClientDataPathFor(NewProfile("工作!"))
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if first == other {
		t.Fatalf("different names collapsed to %q", first)
	}
	if filepath.Dir(first) != filepath.Dir(path) {
		t.Fatalf("path %q is outside the config directory", first)
	}

	custom := NewProfile("custom")
	custom.Debug.ClientData = "/tmp/custom.json"
	got, err := cfg.ClientDataPathFor(custom)
	if err != nil || got != "/tmp/custom.json" {
		t.Fatalf("override = %q, %v", got, err)
	}
}

// A settings file outside the default directory owns its account material:
// --config must not read or overwrite the real account's stored ticket. The
// path follows the file that was loaded, including when that file does not
// exist yet.
func TestClientDataFollowsTheLoadedSettingsFile(t *testing.T) {
	defaultHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", defaultHome)

	dir := t.TempDir()
	path := filepath.Join(dir, "custom.toml")
	if err := SaveFile(path, Default()); err != nil {
		t.Fatalf("save: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	first, err := cfg.ClientDataPathFor(cfg.Active())
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if want := filepath.Join(dir, "client_data.json"); first != want {
		t.Fatalf("path = %q, want %q", first, want)
	}
	if strings.HasPrefix(first, defaultHome) {
		t.Fatalf("path = %q, the default directory was used", first)
	}

	if _, err := cfg.AddProfile("工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	work, err := cfg.ClientDataPathFor(cfg.Active())
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if filepath.Dir(work) != dir {
		t.Fatalf("profile path = %q, want it beside %q", work, path)
	}

	// A first launch with a custom path has no file yet; it still owns a
	// directory, not the default one.
	missing, err := LoadFile(filepath.Join(dir, "new.toml"))
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	fresh, err := missing.ClientDataPathFor(missing.Active())
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if want := filepath.Join(dir, "client_data.json"); fresh != want {
		t.Fatalf("fresh path = %q, want %q", fresh, want)
	}
}

func TestIntervalFallsBackOnGarbage(t *testing.T) {
	cfg := Default()
	cfg.Profiles[0].Keep.Interval = "not-a-duration"
	if got := cfg.KeepAliveInterval(); got != 90*time.Second {
		t.Fatalf("interval = %v, want the default", got)
	}

	cfg.Profiles[0].Keep.Interval = "90s"
	if got := cfg.KeepAliveInterval(); got != 90*time.Second {
		t.Fatalf("interval = %v, want 90s", got)
	}

	cfg.Profiles[0].Keep.Interval = "0s"
	if got := cfg.KeepAliveInterval(); got != 90*time.Second {
		t.Fatalf("zero interval = %v, want the default", got)
	}

	// Every probe is a real request, so a hand-edited fast interval is clamped
	// instead of hammering the gateway.
	cfg.Profiles[0].Keep.Interval = "1s"
	if got := cfg.KeepAliveInterval(); got != MinimumKeepAliveInterval {
		t.Fatalf("1s interval = %v, want %v", got, MinimumKeepAliveInterval)
	}
	cfg.Profiles[0].Keep.Interval = "30s"
	if got := cfg.KeepAliveInterval(); got != 30*time.Second {
		t.Fatalf("30s interval = %v, want it kept", got)
	}
}

func TestValidateRejectsUnusableSettings(t *testing.T) {
	valid := Default()
	valid.Profiles[0].Server.Address = "vpn.example.com"
	valid.Profiles[0].Auth.Username = "alice"
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}

	noServer := valid.Clone()
	noServer.Profiles[0].Server.Address = ""
	if err := noServer.Validate(); err == nil {
		t.Fatal("an empty server address was accepted")
	}

	badPort := valid.Clone()
	badPort.Profiles[0].Server.Port = 70000
	if err := badPort.Validate(); err == nil {
		t.Fatal("port 70000 was accepted")
	}

	noUser := valid.Clone()
	noUser.Profiles[0].Auth.Username = ""
	if err := noUser.Validate(); err == nil {
		t.Fatal("an empty username was accepted")
	}

	noListener := valid.Clone()
	noListener.Proxy = ProxyConfig{}
	if err := noListener.Validate(); err == nil {
		t.Fatal("disabling both listeners was accepted")
	}
}

func TestCloneDoesNotShareState(t *testing.T) {
	cfg := Default()
	clone := cfg.Clone()
	clone.Profiles[0].Name = "changed"
	clone.Profiles[0].Keep.Targets = append(clone.Profiles[0].Keep.Targets, "http://192.0.2.5/")
	clone.App.AutoConnect = true
	if cfg.Profiles[0].Name != DefaultProfileName || len(cfg.Profiles[0].Keep.Targets) != 0 || cfg.App.AutoConnect {
		t.Fatal("clone shares state with the original")
	}
}

// A bind outside the loopback interface would hand the tunnel to the
// machine's network.
func TestValidateBindKeepsTheProxyOnLoopback(t *testing.T) {
	for _, bind := range []string{"127.0.0.1:1080", "localhost:1080", "[::1]:1080", "127.0.0.2:65535"} {
		if err := ValidateBind(bind); err != nil {
			t.Errorf("ValidateBind(%q) = %v, want nil", bind, err)
		}
	}
	for _, bind := range []string{
		"0.0.0.0:1080",
		":1080",
		"192.0.2.10:1080",
		"[::]:1080",
		"vpn.example.com:1080",
		"127.0.0.1",
		"127.0.0.1:0",
		"127.0.0.1:65536",
		"127.0.0.1:socks",
	} {
		if err := ValidateBind(bind); err == nil {
			t.Errorf("ValidateBind(%q) accepted a non-loopback or malformed bind", bind)
		}
	}

	valid := Default()
	valid.Profiles[0].Server.Address = "vpn.example.com"
	valid.Profiles[0].Auth.Username = "alice"
	if err := valid.Validate(); err != nil {
		t.Fatalf("the default binds were rejected: %v", err)
	}

	exposed := valid.Clone()
	exposed.Proxy.HTTPBind = "0.0.0.0:1081"
	if err := exposed.Validate(); err == nil {
		t.Fatal("a non-loopback HTTP bind passed validation")
	}

	malformed := valid.Clone()
	malformed.Proxy.SOCKS5Bind = "127.0.0.1"
	if err := malformed.Validate(); err == nil {
		t.Fatal("a malformed SOCKS5 bind passed validation")
	}
}

// A hand-edited file with a typo must fail instead of silently running
// defaults: the file is the contract.
func TestLoadFileRejectsUnknownSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := "version = 2\n\n[proxy]\nsocks5_bind = \"127.0.0.1:1080\"\n" +
		"http_bind = \"127.0.0.1:1081\"\ntypo_bind = \"127.0.0.1:1082\"\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("an unknown setting was accepted")
	}
	if !strings.Contains(err.Error(), "proxy.typo_bind") {
		t.Fatalf("error = %v, want it to name the unknown key", err)
	}

	// Everything this build writes loads again.
	if err := SaveFile(path, Default()); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := LoadFile(path); err != nil {
		t.Fatalf("the file this build wrote was rejected: %v", err)
	}
}

// File names are the account material's identity. A case-insensitive
// filesystem cannot tell "Work" from "work", so the two profiles must not share
// a path, while a lowercase name keeps the plain file name it already had.
func TestProfileSlugSurvivesCaseInsensitiveFilesystems(t *testing.T) {
	cfg := Default()

	upper, err := cfg.ClientDataPathFor(Profile{Name: "Work"})
	if err != nil {
		t.Fatalf("upper path: %v", err)
	}
	lower, err := cfg.ClientDataPathFor(Profile{Name: "work"})
	if err != nil {
		t.Fatalf("lower path: %v", err)
	}
	if upper == lower || strings.EqualFold(upper, lower) {
		t.Fatalf("both profiles resolve to %q / %q", upper, lower)
	}
	if base := filepath.Base(lower); base != "client_data-work.json" {
		t.Fatalf("lowercase stem = %q, want the plain name", base)
	}
}

func TestInstallIdentityIsGeneratedOnce(t *testing.T) {
	cfg := Default()
	if !cfg.EnsureIdentity() {
		t.Fatal("a fresh identity reported no changes")
	}
	id := cfg.Identity.DeviceID
	if len(id) != 32 || strings.ToUpper(id) != id {
		t.Fatalf("device id %q is not 32 uppercase hex", id)
	}
	if cfg.EnsureIdentity() {
		t.Fatal("a second call replaced the stored id")
	}
	if cfg.Identity.DeviceID != id {
		t.Fatalf("device id = %q, want it stable", cfg.Identity.DeviceID)
	}
}

func TestIdentityHandEditsWin(t *testing.T) {
	cfg := Default()
	cfg.Identity.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	cfg.EnsureIdentity()
	if cfg.Identity.DeviceID != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("device id = %q, want the hand-edited value", cfg.Identity.DeviceID)
	}
	cfg.Identity.ClientVersion = "9.9.9.9"
	cfg.Identity.OSFamily = IdentityOSWindows
	cfg.EnsureIdentity()
	if cfg.Identity.ClientVersion != "9.9.9.9" || cfg.Identity.OSFamily != IdentityOSWindows {
		t.Fatalf("identity = %+v, want the hand-edited values", cfg.Identity)
	}
}

func TestIdentityRoundTripPersistsTheID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := Default()
	cfg.path = path
	cfg.EnsureIdentity()
	if err := SaveFile(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Identity.DeviceID != cfg.Identity.DeviceID {
		t.Fatalf("device id = %q, want %q", loaded.Identity.DeviceID, cfg.Identity.DeviceID)
	}
}

func TestValidateRejectsAMalformedDeviceID(t *testing.T) {
	for _, bad := range []string{"not-hex", "0123456789ABCDEF", "0123456789ABCDEF0123456789ABCDE"} {
		if err := (IdentityConfig{DeviceID: bad}).validate(); err == nil {
			t.Fatalf("device id %q was accepted", bad)
		}
	}
	if err := (IdentityConfig{DeviceID: "0123456789abcdef0123456789abcdef"}).validate(); err != nil {
		t.Fatalf("a lowercase hex id was refused: %v", err)
	}
	if err := (IdentityConfig{}).validate(); err != nil {
		t.Fatalf("an empty id was refused: %v", err)
	}
}

func TestEnsureIdentityAdoptsTheStoredProfileDevice(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.path = filepath.Join(dir, "config.toml")
	clientData, err := cfg.ClientDataPathFor(cfg.Active())
	if err != nil {
		t.Fatalf("client data path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(clientData), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(clientData, []byte(`{"device_id":"0123456789ABCDEF0123456789ABCDEF","cookies":[]}`), 0o600); err != nil {
		t.Fatalf("write client data: %v", err)
	}
	cfg.EnsureIdentity()
	if cfg.Identity.DeviceID != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("device id = %q, want the stored profile's", cfg.Identity.DeviceID)
	}
}

func TestUserAgentDefaultIsTruthful(t *testing.T) {
	id := IdentityConfig{}
	ua := id.UserAgentString()
	if ua != "goTrust (aTrust client; Linux "+archName()+")" {
		t.Fatalf("UA = %q, want the truthful default", ua)
	}
	if strings.Contains(ua, "aTrustTray") || strings.Contains(ua, "UOS") {
		t.Fatalf("UA %q is not the truthful default", ua)
	}
	id.ClientVersion = "1.2.3"
	if !strings.Contains(id.UserAgentString(), "goTrust/1.2.3") {
		t.Fatalf("UA %q does not carry the version", id.UserAgentString())
	}
	id.UserAgent = "custom-ua"
	if got := id.UserAgentString(); got != "custom-ua" {
		t.Fatalf("UA = %q, want the explicit override", got)
	}
}

func TestUserAgentPresetsAreExplicit(t *testing.T) {
	uos := IdentityConfig{OSFamily: IdentityOSUOS, OSArch: "x86_64"}
	if ua := uos.UserAgentString(); !strings.Contains(ua, "aTrustTray/"+defaultClientVersion) ||
		!strings.Contains(ua, "aTrustTray-Linux-Plat-UOS-x64") {
		t.Fatalf("UOS preset UA = %q", ua)
	}
	windows := IdentityConfig{OSFamily: IdentityOSWindows, OSArch: "x86_64"}
	if ua := windows.UserAgentString(); !strings.Contains(ua, "Windows NT 10.0") ||
		!strings.Contains(ua, "aTrustTray-Windows-Plat-Windows10-x64") {
		t.Fatalf("Windows preset UA = %q", ua)
	}
}

func TestEnsureIdentityFillsTheDerivedFields(t *testing.T) {
	cfg := Default()
	cfg.Identity.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	cfg.Identity.ClientVersion = ""
	cfg.Identity.OSFamily = ""
	cfg.Identity.OSArch = ""
	if !cfg.EnsureIdentity() {
		t.Fatal("an empty derived identity reported no change")
	}
	if cfg.Identity.OSFamily != IdentityOSLinux || cfg.Identity.OSArch == "" {
		t.Fatalf("identity = %+v", cfg.Identity)
	}
	if cfg.Identity.ClientVersion != "" {
		t.Fatalf("a client version was invented: %q", cfg.Identity.ClientVersion)
	}
	if cfg.Identity.ProcessName != "" || cfg.Identity.ProcessPath != "" {
		t.Fatalf("a process block was invented: %+v", cfg.Identity)
	}
}

func TestValidateRejectsAnUnknownIdentityFamily(t *testing.T) {
	if err := (IdentityConfig{OSFamily: "beos"}).validate(); err == nil {
		t.Fatal("an unknown os_family was accepted")
	}
	if err := (IdentityConfig{OSFamily: IdentityOSWindows}).validate(); err != nil {
		t.Fatalf("a known family was refused: %v", err)
	}
}
