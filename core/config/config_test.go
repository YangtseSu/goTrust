// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"fmt"
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
	if len(cfg.Profiles) != 1 || cfg.ActiveProfile != cfg.Profiles[0].ID {
		t.Fatalf("profiles = %v, active = %q", cfg.ProfileNames(), cfg.ActiveProfile)
	}
	socks5, httpBind := active(&cfg).ProxyAddresses()
	if socks5 == "" || httpBind == "" {
		t.Fatalf("defaults disable a listener: socks5=%q http=%q", socks5, httpBind)
	}
	profile := cfg.Active()
	if strings.TrimSpace(profile.ID) == "" {
		t.Fatal("the default profile has no id")
	}
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
	if _, err := want.DuplicateProfile(0, "工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	index := want.ProfileIndex("工作")
	want.Profiles[index].App.Autostart = true
	want.Profiles[index].App.AutoConnect = true
	want.Profiles[index].Log.Verbose = true
	want.Profiles[index].Proxy.HTTPBind = ""
	want.Profiles[index].Server.Address = "vpn.example.com"
	want.Profiles[index].Server.Port = 4430
	want.Profiles[index].Auth.Username = "alice"
	want.Profiles[index].Auth.Domain = "Radius"
	want.Profiles[index].Auth.StorePass = true
	want.Profiles[index].Auth.Password = `pa"ss\word`
	want.Profiles[index].Keep.SetEnabled(false)
	want.Profiles[index].Keep.Interval = "90s"
	want.Profiles[index].Keep.Targets = []string{"http://192.0.2.5/health"}

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
	// active_profile names the account by id, so a rename cannot move it.
	if !strings.Contains(string(raw), "active_profile = \""+want.ActiveProfile+"\"") {
		t.Fatalf("active profile not written:\n%s", raw)
	}

	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ActiveProfile != want.ActiveProfile || len(got.Profiles) != 2 {
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
	if got.Active().Proxy.HTTPBind != "" {
		t.Fatalf("http bind = %q, want empty (disabled)", got.Active().Proxy.HTTPBind)
	}
	if socks5, _ := got.Active().ProxyAddresses(); socks5 == "" {
		t.Fatal("socks bind was lost")
	}
	if !got.Active().Log.Verbose || !got.Active().App.Autostart || !got.Active().App.AutoConnect {
		t.Fatalf("account settings lost: %+v %+v", got.Active().App, got.Active().Log)
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
	socks5, httpBind := active(&cfg).ProxyAddresses()
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
	if cfg.ActiveProfile != cfg.Profiles[0].ID {
		t.Fatalf("active = %q, want the only profile's id", cfg.ActiveProfile)
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
	if _, err := cfg.DuplicateProfile(0, "工作"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if cfg.Active().Name != "工作" {
		t.Fatalf("active = %q, want the new profile", cfg.Active().Name)
	}
	if _, err := cfg.DuplicateProfile(0, "工作"); err == nil {
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
	if cfg.Active().Name != "工作 副本" {
		t.Fatalf("active = %q after duplicate", cfg.Active().Name)
	}

	if err := cfg.DeleteProfile(2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("profiles = %v", cfg.ProfileNames())
	}
	cfg.Profiles = cfg.Profiles[:1]
	cfg.ActiveProfile = cfg.Profiles[0].ID
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

// A new account stores its password (the settings file is 0600 and a stored
// password is what lets auto-connect work); a profile loaded from a file keeps
// the toggle the file holds, so an upgrade does not start writing credentials
// for an account that never asked.
func TestNewProfileStoresThePasswordByDefault(t *testing.T) {
	if !NewProfile("a").Auth.StorePass {
		t.Fatal("a new profile does not store its password")
	}

	path := filepath.Join(t.TempDir(), "config.toml")
	body := `version = 3

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
	if cfg.Active().Auth.StorePass {
		t.Fatal("a profile without store_password was rewritten to store it")
	}
}

func TestNewProfileHasAUniqueID(t *testing.T) {
	first := NewProfile("a")
	second := NewProfile("b")
	if first.ID == "" || second.ID == "" {
		t.Fatalf("profile ids = %q, %q", first.ID, second.ID)
	}
	if first.ID == second.ID {
		t.Fatalf("two profiles share the id %q", first.ID)
	}
}

func TestDuplicateProfileGetsANewID(t *testing.T) {
	cfg := Default()
	source := cfg.Active().ID
	index, err := cfg.DuplicateProfile(0, "副本")
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}
	if cfg.Profiles[index].ID == "" || cfg.Profiles[index].ID == source {
		t.Fatalf("duplicate id = %q, want a new one", cfg.Profiles[index].ID)
	}
}

func TestProfileIDSurvivesSaveLoadAndRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	id := cfg.Active().ID
	cfg.Profiles[0].Name = "工作"
	if err := SaveFile(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Profiles[0].ID != id {
		t.Fatalf("id = %q, want %q after a rename round-trip", loaded.Profiles[0].ID, id)
	}
	if loaded.Version != CurrentVersion {
		t.Fatalf("version = %d, want %d", loaded.Version, CurrentVersion)
	}
}

func TestLoadOrCreateFillsProfileIDsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `version = 2

[[profiles]]
name = "a"

[[profiles]]
name = "b"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	first, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	for index, profile := range first.Profiles {
		if strings.TrimSpace(profile.ID) == "" {
			t.Fatalf("profile %d has no id", index)
		}
	}
	if first.Profiles[0].ID == first.Profiles[1].ID {
		t.Fatalf("profiles share the id %q", first.Profiles[0].ID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), fmt.Sprintf("version = %d", CurrentVersion)) ||
		!strings.Contains(string(raw), first.Profiles[0].ID) {
		t.Fatalf("the generated ids were not persisted:\n%s", raw)
	}
	second, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate() error = %v", err)
	}
	if second.Profiles[0].ID != first.Profiles[0].ID ||
		second.Profiles[1].ID != first.Profiles[1].ID {
		t.Fatalf("ids changed between loads: %q/%q then %q/%q",
			first.Profiles[0].ID, first.Profiles[1].ID,
			second.Profiles[0].ID, second.Profiles[1].ID)
	}
}

func TestDuplicateProfileIDsAreBrokenOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `version = 2

[[profiles]]
id = "shared"
name = "a"

[[profiles]]
id = "shared"
name = "b"
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if cfg.Profiles[0].ID != "shared" {
		t.Fatalf("the first id = %q, want the hand-edited value", cfg.Profiles[0].ID)
	}
	if cfg.Profiles[1].ID == "shared" || cfg.Profiles[1].ID == "" {
		t.Fatalf("the duplicate id was kept: %q", cfg.Profiles[1].ID)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if count := strings.Count(string(raw), `id = "shared"`); count != 1 {
		t.Fatalf("the file carries the shared id %d times:\n%s", count, raw)
	}
}

func TestSaveFileWritesAnIDForAHandBuiltProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := Default()
	cfg.Profiles = []Profile{{Name: "a"}}
	if err := SaveFile(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if strings.TrimSpace(loaded.Profiles[0].ID) == "" {
		t.Fatal("a hand-built profile reached the file without an id")
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
	if base := filepath.Base(path); !strings.HasPrefix(base, "client_data-") {
		t.Fatalf("default profile path = %q, want an id-named file", path)
	}

	work := NewProfile("工作")
	first, err := cfg.ClientDataPathFor(work)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if first == path {
		t.Fatal("profiles share one client data file")
	}
	// Two accounts with one name are still two files: the id carries identity,
	// the name is only a label.
	twin := NewProfile("工作")
	other, err := cfg.ClientDataPathFor(twin)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if first == other {
		t.Fatalf("two accounts with one name collapsed to %q", first)
	}
	if filepath.Dir(first) != filepath.Dir(path) {
		t.Fatalf("path %q is outside the config directory", first)
	}

	// There is no override: two accounts resolving to one file would overwrite
	// each other's sign key and ticket.
	custom := NewProfile("custom")
	got, err := cfg.ClientDataPathFor(custom)
	if err != nil {
		t.Fatal(err)
	}
	if got == first {
		t.Fatalf("two accounts resolve to one file: %q", got)
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
	if got := filepath.Dir(first); got != dir {
		t.Fatalf("path = %q, want it beside %q", first, dir)
	}
	if strings.HasPrefix(first, defaultHome) {
		t.Fatalf("path = %q, the default directory was used", first)
	}

	if _, err := cfg.DuplicateProfile(0, "工作"); err != nil {
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
	if got := filepath.Dir(fresh); got != dir {
		t.Fatalf("fresh path = %q, want it beside %q", fresh, dir)
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
	noListener.ActiveProfileRef().Proxy = ProxyConfig{}
	if err := noListener.Validate(); err == nil {
		t.Fatal("disabling both listeners was accepted")
	}
}

func TestForDiskClearsUnstoredCredentials(t *testing.T) {
	cfg := Default()
	cfg.Profiles[0].Auth.StorePass = true
	cfg.Profiles[0].Auth.Password = "stored"
	cfg.Profiles[0].Auth.TOTPSecret = "stored-secret"
	if _, err := cfg.DuplicateProfile(0, "工作"); err != nil {
		t.Fatalf("add profile: %v", err)
	}
	cfg.Profiles[1].Auth.StorePass = false
	cfg.Profiles[1].Auth.Password = "memory-only"
	cfg.Profiles[1].Auth.TOTPSecret = "memory-only-secret"

	disk := cfg.ForDisk()
	if disk.Profiles[0].Auth.Password != "stored" || disk.Profiles[0].Auth.TOTPSecret != "stored-secret" {
		t.Fatal("a stored credential was cleared from the file")
	}
	if disk.Profiles[1].Auth.Password != "" || disk.Profiles[1].Auth.TOTPSecret != "" {
		t.Fatal("an unstored credential reached the file")
	}
	if cfg.Profiles[1].Auth.Password != "memory-only" || cfg.Profiles[1].Auth.TOTPSecret != "memory-only-secret" {
		t.Fatal("the in-memory credential was cleared")
	}
}

func TestCloneDoesNotShareState(t *testing.T) {
	cfg := Default()
	clone := cfg.Clone()
	clone.Profiles[0].Name = "changed"
	clone.Profiles[0].Keep.Targets = append(clone.Profiles[0].Keep.Targets, "http://192.0.2.5/")
	clone.Profiles[0].App.AutoConnect = true
	if cfg.Profiles[0].Name != DefaultProfileName || len(cfg.Profiles[0].Keep.Targets) != 0 || cfg.Profiles[0].App.AutoConnect {
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
	exposed.ActiveProfileRef().Proxy.HTTPBind = "0.0.0.0:1081"
	if err := exposed.Validate(); err == nil {
		t.Fatal("a non-loopback HTTP bind passed validation")
	}

	malformed := valid.Clone()
	malformed.ActiveProfileRef().Proxy.SOCKS5Bind = "127.0.0.1"
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

// File names are the account material's identity, and the id is what they
// carry. Two accounts whose names differ only in case are still two files, and
// the stem is lower case so a case-insensitive filesystem cannot merge them.
func TestClientDataPathFollowsTheID(t *testing.T) {
	cfg := Default()

	upper := Profile{ID: "ABCDEFGH", Name: "Work"}
	lower := Profile{ID: "IJKLMNOP", Name: "work"}
	upperPath, err := cfg.ClientDataPathFor(upper)
	if err != nil {
		t.Fatalf("upper path: %v", err)
	}
	lowerPath, err := cfg.ClientDataPathFor(lower)
	if err != nil {
		t.Fatalf("lower path: %v", err)
	}
	if strings.EqualFold(upperPath, lowerPath) {
		t.Fatalf("both accounts resolve to %q / %q", upperPath, lowerPath)
	}
	if base := filepath.Base(upperPath); base != "client_data-abcdefgh.json" {
		t.Fatalf("stem = %q, want the lower-case id", base)
	}
	// The name is not part of the path: a rename leaves the file alone.
	renamed := upper
	renamed.Name = "Personal"
	renamedPath, err := cfg.ClientDataPathFor(renamed)
	if err != nil {
		t.Fatalf("renamed path: %v", err)
	}
	if renamedPath != upperPath {
		t.Fatalf("a rename moved the file: %q -> %q", upperPath, renamedPath)
	}
}

func TestInstallIdentityIsGeneratedOnce(t *testing.T) {
	cfg := Default()
	id := active(&cfg).Ident.DeviceID
	if len(id) != 32 || strings.ToUpper(id) != id {
		t.Fatalf("device id %q is not 32 uppercase hex", id)
	}
	// A profile created in this run is complete, so loading it again is not a
	// change; EnsureIdentity only fills what an older file lacks.
	if cfg.EnsureIdentity() {
		t.Fatalf("a complete identity reported a change: %+v", cfg.Active().Ident)
	}
	if active(&cfg).Ident.DeviceID != id {
		t.Fatalf("device id = %q, want it stable", active(&cfg).Ident.DeviceID)
	}
}

func TestIdentityHandEditsWin(t *testing.T) {
	cfg := Default()
	active(&cfg).Ident.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	cfg.EnsureIdentity()
	if active(&cfg).Ident.DeviceID != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("device id = %q, want the hand-edited value", active(&cfg).Ident.DeviceID)
	}
	active(&cfg).Ident.ClientVersion = "9.9.9.9"
	active(&cfg).Ident.OSFamily = IdentityOSWindows
	cfg.EnsureIdentity()
	if active(&cfg).Ident.ClientVersion != "9.9.9.9" || active(&cfg).Ident.OSFamily != IdentityOSWindows {
		t.Fatalf("identity = %+v, want the hand-edited values", cfg.Active().Ident)
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
	if loaded.Active().Ident.DeviceID != cfg.Active().Ident.DeviceID {
		t.Fatalf("device id = %q, want %q", loaded.Active().Ident.DeviceID, cfg.Active().Ident.DeviceID)
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
	// An older file may predate the generated id; the stored session names the
	// device it already presented.
	active(&cfg).Ident.DeviceID = ""
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
	if active(&cfg).Ident.DeviceID != "0123456789ABCDEF0123456789ABCDEF" {
		t.Fatalf("device id = %q, want the stored profile's", active(&cfg).Ident.DeviceID)
	}
}

func TestUserAgentDefaultIsTruthful(t *testing.T) {
	id := IdentityConfig{}
	want := "goTrust (aTrust client; " + hostPlatformName() + " " + archName() + ")"
	if ua := id.UserAgentString(); ua != want {
		t.Fatalf("UA = %q, want %q", ua, want)
	}
	ua := id.UserAgentString()
	if strings.Contains(ua, "aTrustTray") || strings.Contains(ua, "UOS") ||
		strings.Contains(ua, "Chrome/") || strings.Contains(ua, "Electron/") {
		t.Fatalf("UA %q is not the truthful default", ua)
	}
	id.ClientVersion = "1.2.3"
	want = "goTrust/1.2.3 (aTrust client; " + hostPlatformName() + " " + archName() + ")"
	if got := id.UserAgentString(); got != want {
		t.Fatalf("UA = %q, want %q", got, want)
	}
	id.UserAgent = "custom-ua"
	if got := id.UserAgentString(); got != "custom-ua" {
		t.Fatalf("UA = %q, want the explicit override", got)
	}
}

// TestUserAgentIsTheSameForEveryFamily: the UA describes the host, so a preset
// family changes the protocol platform but not the UA.
func TestUserAgentIsTheSameForEveryFamily(t *testing.T) {
	want := "goTrust (aTrust client; " + hostPlatformName() + " " + archName() + ")"
	for _, family := range []string{IdentityOSLinux, IdentityOSWindows, IdentityOSMacOS, legacyIdentityOSUOS} {
		id := IdentityConfig{OSFamily: family, OSArch: "arm64"}
		if got := id.UserAgentString(); got != want {
			t.Fatalf("UA for %q = %q, want %q", family, got, want)
		}
	}
}

// TestHostFamilyFollowsTheHost: a fresh identity starts from the platform the
// build runs on, and every Linux distribution reports linux.
func TestHostFamilyFollowsTheHost(t *testing.T) {
	for goos, want := range map[string]string{
		"linux":   IdentityOSLinux,
		"freebsd": IdentityOSLinux,
		"windows": IdentityOSWindows,
		"darwin":  IdentityOSMacOS,
	} {
		if got := osFamilyFor(goos); got != want {
			t.Fatalf("osFamilyFor(%q) = %q, want %q", goos, got, want)
		}
	}
	if got := NewIdentity().OSFamily; got != hostOSFamily() {
		t.Fatalf("NewIdentity family = %q, want the host's %q", got, hostOSFamily())
	}
}

// TestPlatformNames: the platform= query keeps the vocabulary the portal
// expects, which differs from the tunnel process spelling for macOS.
func TestPlatformNames(t *testing.T) {
	for _, test := range []struct{ family, query, process string }{
		{IdentityOSLinux, "Linux", "Linux"},
		{IdentityOSWindows, "Windows", "Windows"},
		{IdentityOSMacOS, "macOS", "macOS"},
		{legacyIdentityOSUOS, "Linux", "Linux"},
	} {
		ident := IdentityConfig{OSFamily: test.family}
		if got := ident.PlatformName(); got != test.query {
			t.Fatalf("PlatformName(%q) = %q, want %q", test.family, got, test.query)
		}
		if got := ident.ProcessPlatformName(); got != test.process {
			t.Fatalf("ProcessPlatformName(%q) = %q, want %q", test.family, got, test.process)
		}
	}
	empty := IdentityConfig{}
	if got := empty.PlatformName(); got != (IdentityConfig{OSFamily: hostOSFamily()}).PlatformName() {
		t.Fatalf("an empty family reported %q, want the host family's platform", got)
	}
}

// TestNormalisedIdentityFamily: the stored vocabulary is resolved once, so a
// missing value takes the host's family and the retired uos value becomes
// linux before validation runs.
func TestNormalisedIdentityFamily(t *testing.T) {
	for _, test := range []struct{ stored, want string }{
		{"", hostOSFamily()},
		{legacyIdentityOSUOS, IdentityOSLinux},
		{" linux ", IdentityOSLinux},
		{IdentityOSWindows, IdentityOSWindows},
		{IdentityOSMacOS, IdentityOSMacOS},
		{"beos", "beos"},
	} {
		if got := normalisedIdentityFamily(test.stored); got != test.want {
			t.Fatalf("normalisedIdentityFamily(%q) = %q, want %q", test.stored, got, test.want)
		}
	}
}

func TestEnsureIdentityFillsTheDerivedFields(t *testing.T) {
	cfg := Default()
	active(&cfg).Ident.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	active(&cfg).Ident.ClientVersion = ""
	active(&cfg).Ident.OSFamily = ""
	active(&cfg).Ident.OSArch = ""
	if !cfg.EnsureIdentity() {
		t.Fatal("an empty derived identity reported no change")
	}
	if active(&cfg).Ident.OSFamily != hostOSFamily() || active(&cfg).Ident.OSArch == "" {
		t.Fatalf("identity = %+v", cfg.Active().Ident)
	}
	if active(&cfg).Ident.ClientVersion != "" {
		t.Fatalf("a client version was invented: %q", active(&cfg).Ident.ClientVersion)
	}
	if active(&cfg).Ident.ProcessName != "" || active(&cfg).Ident.ProcessPath != "" {
		t.Fatalf("a process block was invented: %+v", cfg.Active().Ident)
	}
}

func TestValidateRejectsAnUnknownIdentityFamily(t *testing.T) {
	if err := (IdentityConfig{OSFamily: "beos"}).validate(); err == nil {
		t.Fatal("an unknown os_family was accepted")
	}
	for _, family := range []string{IdentityOSLinux, IdentityOSWindows, IdentityOSMacOS} {
		if err := (IdentityConfig{OSFamily: family}).validate(); err != nil {
			t.Fatalf("a known family %q was refused: %v", family, err)
		}
	}
	if err := (IdentityConfig{OSFamily: legacyIdentityOSUOS}).validate(); err == nil {
		t.Fatal("the retired uos family was accepted")
	}
}

func TestLoadOrCreatePersistsTheIdentityOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")

	first, got, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	if got != path {
		t.Fatalf("path = %q, want %q", got, path)
	}
	if first.Active().Ident.DeviceID == "" {
		t.Fatal("the account's device id was not generated")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the settings file was not written: %v", err)
	}

	second, _, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("second LoadOrCreate() error = %v", err)
	}
	if second.Active().Ident.DeviceID != first.Active().Ident.DeviceID {
		t.Fatalf("device id = %q, want the stored %q", second.Active().Ident.DeviceID, first.Active().Ident.DeviceID)
	}
}

func TestHandoffDefaultsToLoopback(t *testing.T) {
	if got := Default().Active().Hand.Address(); got != defaultHandoffBind {
		t.Fatalf("handoff listen = %q, want %q", got, defaultHandoffBind)
	}
}

func TestHandoffCanBeDisabledExplicitly(t *testing.T) {
	cfg := Default()
	cfg.ActiveProfileRef().Hand.SetListen("")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := SaveFile(path, cfg); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if got := loaded.Active().Hand.Address(); got != "" {
		t.Fatalf("handoff listen = %q, want it disabled", got)
	}
}

func TestAnOlderFileGetsTheHandoffDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := "version = 2\nactive_profile = \"" + DefaultProfileName + "\"\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if got := loaded.Active().Hand.Address(); got != defaultHandoffBind {
		t.Fatalf("handoff listen = %q, want the default %q", got, defaultHandoffBind)
	}
}

func TestHandoffBindMustStayOnLoopback(t *testing.T) {
	cfg := Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	cfg.Profiles[0].Auth.Username = "alice"
	cfg.ActiveProfileRef().Hand.SetListen("0.0.0.0:1082")
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "handoff bind") {
		t.Fatalf("validate error = %v, want a handoff bind error", err)
	}
}

// TestEnsureIdentityLeavesTheAddressToTheMachine: the settings do not carry a
// hardware address. A blank entry means "report the machine's own cards", which
// is read at login, and a value the user typed is kept exactly as written.
func TestEnsureIdentityLeavesTheAddressToTheMachine(t *testing.T) {
	cfg := Default()
	active(&cfg).Ident.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	active(&cfg).Ident.MACAddresses = nil
	cfg.EnsureIdentity()
	if got := active(&cfg).Ident.MACAddresses; len(got) != 0 {
		t.Fatalf("an address was written into the settings: %v", got)
	}

	// A second pass is not a change: nothing is generated, so nothing is saved.
	if cfg.EnsureIdentity() {
		t.Fatalf("a blank address reported a change: %+v", cfg.Active().Ident)
	}

	cfg.ActiveProfileRef().Ident.MACAddresses = []string{"00:1a:2b:3c:4d:5e"}
	cfg.EnsureIdentity()
	if got := active(&cfg).Ident.MACAddresses; len(got) != 1 || got[0] != "00:1a:2b:3c:4d:5e" {
		t.Fatalf("a hand-set address was replaced: %v", got)
	}
}

// TestSaveKeepsTheEmptyAddressList: a new profile writes identity.mac_addresses
// as an empty list, which is how the file says the login reports the machine's
// own cards. Every save goes through ForDisk, which clones the settings; the
// clone has to keep the empty list rather than collapse it to a missing key.
func TestSaveKeepsTheEmptyAddressList(t *testing.T) {
	cfg := Default()
	active(&cfg).Ident.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	save := func(cfg Config) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := SaveFile(path, cfg.ForDisk()); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "mac_addresses = []") {
			t.Fatalf("the file does not show the empty address list:\n%s", raw)
		}
		return path
	}

	path := save(cfg)
	loaded, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := active(&loaded).Ident.MACAddresses; got == nil || len(got) != 0 {
		t.Fatalf("the empty address list did not survive the load: %#v", got)
	}

	// A reloaded empty list is still present, not missing.
	save(loaded)
}

// TestMigrationDropsAGeneratedAddress: a file from the previous version carries
// a locally administered address that no gateway ever recorded. Keeping it would
// keep reporting a device that does not exist, so the migration drops it and
// leaves the login free to report the machine's cards.
func TestMigrationDropsAGeneratedAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 4
active_profile = "a"

[[profiles]]
name = "a"
[profiles.identity]
device_id = "0123456789ABCDEF0123456789ABCDEF"
mac_addresses = ["02:00:5e:10:00:01"]

[[profiles]]
name = "b"
[profiles.identity]
device_id = "FEDCBA9876543210FEDCBA9876543210"
mac_addresses = ["00:1a:2b:3c:4d:5e"]
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("a v4 file was refused: %v", err)
	}
	if cfg.Version != CurrentVersion {
		t.Fatalf("version = %d", cfg.Version)
	}
	if got := cfg.Profiles[0].Ident.MACAddresses; len(got) != 0 {
		t.Fatalf("the generated address survived: %v", got)
	}
	if got := cfg.Profiles[1].Ident.MACAddresses; len(got) != 1 || got[0] != "00:1a:2b:3c:4d:5e" {
		t.Fatalf("the address the user typed was dropped: %v", got)
	}
}

// TestAccountsCarryTheirOwnIdentity: one file may hold accounts that present
// different protocol platforms and process blocks. The User-Agent stays the
// same because it describes the host.
func TestAccountsCarryTheirOwnIdentity(t *testing.T) {
	cfg := Default()
	cfg.ActiveProfileRef().Ident = IdentityConfig{
		DeviceID: "0123456789ABCDEF0123456789ABCDEF",
		MACAddresses: []string{
			"02:00:5e:10:00:01",
		},
		OSFamily: IdentityOSLinux,
		OSArch:   "x86_64",
	}
	if _, err := cfg.DuplicateProfile(0, "vendor"); err != nil {
		t.Fatal(err)
	}
	index := cfg.ProfileIndex("vendor")
	cfg.Profiles[index].Ident.OSFamily = IdentityOSMacOS
	cfg.Profiles[index].Ident.ProcessName = "google-chrome-stable"

	if got := cfg.Profiles[index].Ident.PlatformName(); got != "macOS" {
		t.Fatalf("vendor platform = %q, want macOS", got)
	}
	if got := cfg.Profiles[0].Ident.PlatformName(); got != "Linux" {
		t.Fatalf("other platform = %q, want Linux", got)
	}
	if got, want := cfg.Profiles[index].Ident.UserAgentString(), cfg.Profiles[0].Ident.UserAgentString(); got != want {
		t.Fatalf("vendor UA = %q, want the host UA %q", got, want)
	}
	if cfg.Profiles[0].Ident.ProcessName != "" {
		t.Fatalf("the other account inherited a process block: %q", cfg.Profiles[0].Ident.ProcessName)
	}

	// The copy starts as the same device, which is what a second account on one
	// host means.
	if got := cfg.Profiles[index].Ident.DeviceID; got != cfg.Profiles[0].Ident.DeviceID {
		t.Fatalf("copy device id = %q, want the original %q", got, cfg.Profiles[0].Ident.DeviceID)
	}
}

// TestAccountsCarryTheirOwnListeners: a listener port is part of a profile, so
// two accounts may listen on different ones.
func TestAccountsCarryTheirOwnListeners(t *testing.T) {
	cfg := Default()
	if _, err := cfg.DuplicateProfile(0, "second"); err != nil {
		t.Fatal(err)
	}
	index := cfg.ProfileIndex("second")
	cfg.Profiles[index].Proxy.SOCKS5Bind = "127.0.0.1:2080"
	cfg.Profiles[index].Proxy.HTTPBind = "127.0.0.1:2081"
	cfg.Profiles[index].Hand.SetListen("127.0.0.1:2082")
	cfg.Profiles[index].Log.Verbose = true
	cfg.Profiles[index].AutoReconnect = true

	if socks5, _ := cfg.Profiles[0].ProxyAddresses(); socks5 != defaultSocksBind {
		t.Fatalf("the original account moved: %q", socks5)
	}
	if got := cfg.Profiles[0].Hand.Address(); got != defaultHandoffBind {
		t.Fatalf("the original PAC endpoint moved: %q", got)
	}
	if cfg.Profiles[0].Log.Verbose || cfg.Profiles[0].AutoReconnect {
		t.Fatal("a per-account switch leaked into another account")
	}
	if socks5, httpBind := cfg.Profiles[index].ProxyAddresses(); socks5 != "127.0.0.1:2080" || httpBind != "127.0.0.1:2081" {
		t.Fatalf("copy listeners = %q %q", socks5, httpBind)
	}
}

func TestValidateRejectsAnUnknownAccountFamily(t *testing.T) {
	cfg := Default()
	cfg.Profiles[0].Server.Address = "vpn.example.com"
	cfg.Profiles[0].Auth.Username = "student"
	cfg.Profiles[0].Ident.OSFamily = "beos"
	if err := cfg.Validate(); err == nil {
		t.Fatal("an unknown account os_family was accepted")
	}
	cfg.Profiles[0].Ident.OSFamily = IdentityOSMacOS
	if err := cfg.Validate(); err != nil {
		t.Fatalf("a known account family was refused: %v", err)
	}
	cfg.Profiles[0].Ident.OSFamily = legacyIdentityOSUOS
	if err := cfg.Validate(); err == nil {
		t.Fatal("the retired uos family was accepted")
	}
}

// TestDuplicateKeepsTheDevice: copying an account must not mint a device, or the
// portal would see one machine as several. Changing the id by hand is how a user
// asks for a separate device.
func TestDuplicateKeepsTheDevice(t *testing.T) {
	cfg := Default()
	cfg.ActiveProfileRef().Ident.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	cfg.ActiveProfileRef().Ident.MACAddresses = []string{"02:00:5e:10:00:01"}
	if _, err := cfg.DuplicateProfile(0, "copy"); err != nil {
		t.Fatal(err)
	}
	index := cfg.ProfileIndex("copy")
	if cfg.Profiles[index].ID == cfg.Profiles[0].ID {
		t.Fatal("the copy reused the profile id")
	}
	if cfg.Profiles[index].Ident.DeviceID != cfg.Profiles[0].Ident.DeviceID {
		t.Fatalf("copy device id = %q, want %q", cfg.Profiles[index].Ident.DeviceID, cfg.Profiles[0].Ident.DeviceID)
	}
	if len(cfg.Profiles[index].Ident.MACAddresses) != 1 ||
		cfg.Profiles[index].Ident.MACAddresses[0] != cfg.Profiles[0].Ident.MACAddresses[0] {
		t.Fatalf("copy mac = %v", cfg.Profiles[index].Ident.MACAddresses)
	}
}

// active is the profile a session runs with: every setting lives there now.
func active(c *Config) *Profile { return c.ActiveProfileRef() }

// TestMigratesASharedSettingsFile: a file from before the split carried one
// identity, one set of listeners and one log level for the whole install. The
// migration stamps them into every account, so the first launch after the
// upgrade presents the device the portal already knows and listens where the
// applications already point.
func TestMigratesASharedSettingsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 3
active_profile = "work"

[app]
autostart = true
start_hidden = true
auto_connect = true
auto_reconnect = false

[proxy]
socks5_bind = "127.0.0.1:1180"
http_bind = "127.0.0.1:1181"

[handoff]
listen = "127.0.0.1:1182"

[logging]
verbose = true

[identity]
device_id = "0123456789ABCDEF0123456789ABCDEF"
mac_addresses = ["02:00:5e:10:00:01"]
os_family = "uos"
os_arch = "x86_64"

[[profiles]]
name = "home"
[profiles.server]
address = "vpn.example.com"
port = 443
[profiles.auth]
username = "alice"
[profiles.debug]
client_data_path = "/tmp/custom.json"

[[profiles]]
name = "work"
[profiles.server]
address = "vpn.example.com"
port = 443
[profiles.auth]
username = "bob"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("a v3 file was refused: %v", err)
	}
	// active_profile names the account by id; the migration has to point it at
	// the same account the file did, which was "work".
	if cfg.Version != CurrentVersion || cfg.Active().Name != "work" || len(cfg.Profiles) != 2 {
		t.Fatalf("migrated head = %d %q %d", cfg.Version, cfg.Active().Name, len(cfg.Profiles))
	}
	for _, profile := range cfg.Profiles {
		if profile.Ident.DeviceID != "0123456789ABCDEF0123456789ABCDEF" {
			t.Fatalf("%s device id = %q", profile.Name, profile.Ident.DeviceID)
		}
		if len(profile.Ident.MACAddresses) != 0 {
			t.Fatalf("%s mac = %v, want the generated placeholder dropped", profile.Name, profile.Ident.MACAddresses)
		}
		if profile.Ident.OSFamily != IdentityOSLinux {
			t.Fatalf("%s family = %q, want the retired uos value read as linux", profile.Name, profile.Ident.OSFamily)
		}
		if socks, httpBind := profile.ProxyAddresses(); socks != "127.0.0.1:1180" || httpBind != "127.0.0.1:1181" {
			t.Fatalf("%s listeners = %q %q", profile.Name, socks, httpBind)
		}
		if got := profile.Hand.Address(); got != "127.0.0.1:1182" {
			t.Fatalf("%s pac = %q", profile.Name, got)
		}
		if !profile.Log.Verbose || !profile.App.Autostart || !profile.App.AutoConnect || !profile.App.StartHidden {
			t.Fatalf("%s app/log = %+v %+v", profile.Name, profile.App, profile.Log)
		}
	}
}

// TestRetiredUOSFamilyLoadsAsLinux: a current-schema file may still carry the
// value an earlier build wrote. It is a Linux identity under a retired name, so
// the file loads and every later save writes linux.
func TestRetiredUOSFamilyLoadsAsLinux(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := fmt.Sprintf(`version = %d

[[profiles]]
name = "a"
[profiles.server]
address = "vpn.example.com"
[profiles.auth]
username = "alice"
[profiles.identity]
os_family = "uos"
`, CurrentVersion)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("a file with the retired value was refused: %v", err)
	}
	ident := cfg.Active().Ident
	if ident.OSFamily != IdentityOSLinux {
		t.Fatalf("family = %q, want linux", ident.OSFamily)
	}
	if ua := ident.UserAgentString(); !strings.Contains(ua, "goTrust") {
		t.Fatalf("UA = %q, want the truthful Linux wording", ua)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("the migrated file does not validate: %v", err)
	}
}

// A shared file with no hardware address leaves every account without one: the
// address is read from the machine at login, so there is nothing to share.
func TestMigrationLeavesTheAddressToTheMachine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	raw := `version = 3
[identity]
device_id = "0123456789ABCDEF0123456789ABCDEF"
os_family = "linux"

[[profiles]]
name = "a"
[[profiles]]
name = "b"
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range cfg.Profiles {
		if got := profile.Ident.MACAddresses; len(got) != 0 {
			t.Fatalf("%s mac = %v, want none", profile.Name, got)
		}
	}
}

// TestNewAccountIsASeparateDevice: a blank account is an unrelated one, so it
// presents a device of its own. Copying is what gives a second account on the
// same machine the same device.
func TestNewAccountIsASeparateDevice(t *testing.T) {
	cfg := Default()
	cfg.ActiveProfileRef().Ident.DeviceID = "0123456789ABCDEF0123456789ABCDEF"
	cfg.ActiveProfileRef().Proxy = ProxyConfig{SOCKS5Bind: "127.0.0.1:2080", HTTPBind: "127.0.0.1:2081"}
	cfg.EnsureIdentity()

	if _, err := cfg.NewProfileNamed("other"); err != nil {
		t.Fatal(err)
	}
	index := cfg.ProfileIndex("other")
	if index < 0 || cfg.Active().Name != "other" {
		t.Fatalf("profiles = %v active = %q", cfg.ProfileNames(), cfg.Active().Name)
	}
	blank := cfg.Profiles[index]
	if blank.Ident.DeviceID == "" || blank.Ident.DeviceID == cfg.Profiles[0].Ident.DeviceID {
		t.Fatalf("a new account reused the first account's device: %+v", blank.Ident)
	}
	if len(blank.Ident.DeviceID) != 32 || strings.ToUpper(blank.Ident.DeviceID) != blank.Ident.DeviceID {
		t.Fatalf("device id %q is not 32 uppercase hex", blank.Ident.DeviceID)
	}
	if len(blank.Ident.MACAddresses) != 0 {
		t.Fatalf("a new account came with an address: %v", blank.Ident.MACAddresses)
	}
	// The listeners carry over, so switching to the new account does not move
	// the ports an application already points at.
	if socks, httpBind := blank.ProxyAddresses(); socks != "127.0.0.1:2080" || httpBind != "127.0.0.1:2081" {
		t.Fatalf("listeners = %q %q, want the current account's", socks, httpBind)
	}

	// A later load keeps the device the account was created with.
	cfg.EnsureIdentity()
	minted := cfg.Profiles[cfg.ProfileIndex("other")].Ident
	if minted.DeviceID != blank.Ident.DeviceID {
		t.Fatalf("device id changed after load: %q -> %q", blank.Ident.DeviceID, minted.DeviceID)
	}
}

func TestNewAccountRefusesADuplicateName(t *testing.T) {
	cfg := Default()
	if _, err := cfg.NewProfileNamed(DefaultProfileName); err == nil {
		t.Fatal("a duplicate name was accepted")
	}
	if _, err := cfg.NewProfileNamed("  "); err == nil {
		t.Fatal("an empty name was accepted")
	}
}
