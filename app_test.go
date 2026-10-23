package main

import (
	"testing"

	"github.com/YangtseSu/goTrust/config"
)

// The form's domain must reach both the settings a login reads and the
// credentials it sends: a gateway matches the auth method by that pair.
func TestEditedDomainReachesTheLogin(t *testing.T) {
	cfg := config.Default()
	cfg.Auth.Domain = "stale"

	testApp := &app{cfg: cfg}
	testApp.view.Server = "vpn.example.com"
	testApp.view.Port = "443"
	testApp.view.Username = "alice"
	testApp.view.Domain = "  local  "
	testApp.view.Password = "secret"

	if got := testApp.currentConfig().Auth.Domain; got != "local" {
		t.Fatalf("settings domain = %q, want local", got)
	}
	if got := testApp.currentAuth().Domain; got != "local" {
		t.Fatalf("credential domain = %q, want local", got)
	}
}
