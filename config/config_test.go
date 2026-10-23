// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultHasBothListeners(t *testing.T) {
	cfg := Default()
	socks5, httpBind := cfg.ProxyAddresses()
	if socks5 == "" || httpBind == "" {
		t.Fatalf("defaults disable a listener: socks5=%q http=%q", socks5, httpBind)
	}
	if cfg.Server.Port != 443 {
		t.Fatalf("default port = %d, want 443", cfg.Server.Port)
	}
}

func TestMissingFileYieldsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server.Port != 443 || cfg.Proxy.SOCKS5Bind == "" {
		t.Fatalf("missing file did not yield defaults: %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")

	want := Default()
	want.Server.Address = "vpn.example.com"
	want.Server.Port = 4430
	want.Auth.Username = "alice"
	want.Auth.StorePass = true
	want.Auth.Password = "secret"
	want.Keep.Targets = []string{"http://192.0.2.5/health"}
	want.Proxy.HTTPBind = ""

	if err := SaveFile(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Server.Address != want.Server.Address || got.Server.Port != want.Server.Port {
		t.Fatalf("server = %+v, want %+v", got.Server, want.Server)
	}
	if got.Auth.Password != want.Auth.Password {
		t.Fatalf("password round-trip lost the value")
	}
	if len(got.Keep.Targets) != 1 || got.Keep.Targets[0] != want.Keep.Targets[0] {
		t.Fatalf("targets = %v, want %v", got.Keep.Targets, want.Keep.Targets)
	}
	if got.Proxy.HTTPBind != "" {
		t.Fatalf("http bind = %q, want empty (disabled)", got.Proxy.HTTPBind)
	}
	if socks5, _ := got.ProxyAddresses(); socks5 == "" {
		t.Fatal("socks bind was lost")
	}
}

func TestSaveRestrictsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
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

// A hand-edited file that omits a listener must keep the other one working
// rather than silently disabling both.
func TestPartialFileKeepsListeners(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"server":{"address":"vpn.example.com"}}`), 0o600); err != nil {
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
	if cfg.Server.Port != 443 {
		t.Fatalf("port = %d, want the default 443", cfg.Server.Port)
	}
}

func TestIntervalFallsBackOnGarbage(t *testing.T) {
	cfg := Default()

	cfg.Keep.Interval = "not-a-duration"
	if got := cfg.KeepAliveInterval(); got != 60*time.Second {
		t.Fatalf("interval = %v, want the default", got)
	}

	cfg.Keep.Interval = "90s"
	if got := cfg.KeepAliveInterval(); got != 90*time.Second {
		t.Fatalf("interval = %v, want 90s", got)
	}

	cfg.Keep.Interval = "0s"
	if got := cfg.KeepAliveInterval(); got != 60*time.Second {
		t.Fatalf("zero interval = %v, want the default", got)
	}
}

func TestValidateRejectsUnusableSettings(t *testing.T) {
	valid := Default()
	valid.Server.Address = "vpn.example.com"
	valid.Auth.Username = "alice"
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}

	noServer := valid
	noServer.Server.Address = ""
	if err := noServer.Validate(); err == nil {
		t.Fatal("an empty server address was accepted")
	}

	badPort := valid
	badPort.Server.Port = 70000
	if err := badPort.Validate(); err == nil {
		t.Fatal("port 70000 was accepted")
	}

	noUser := valid
	noUser.Auth.Username = ""
	if err := noUser.Validate(); err == nil {
		t.Fatal("an empty username was accepted")
	}

	noListener := valid
	noListener.Proxy = ProxyConfig{}
	if err := noListener.Validate(); err == nil {
		t.Fatal("disabling both listeners was accepted")
	}
}

func TestClientDataPathHonoursOverride(t *testing.T) {
	cfg := Default()
	cfg.Debug.ClientData = "/tmp/custom-client-data.json"

	got, err := cfg.ClientDataPath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if got != "/tmp/custom-client-data.json" {
		t.Fatalf("path = %q, want the override", got)
	}
}
