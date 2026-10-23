// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clientpkg "github.com/mythologyli/zju-connect/client"

	"github.com/YangtseSu/goTrust/config"
)

func grant() ([]clientpkg.IPResource, clientpkg.DomainResources, map[string][]net.IP) {
	ips := []clientpkg.IPResource{
		{IPMin: net.ParseIP("192.0.2.10"), IPMax: net.ParseIP("192.0.2.10"), PortMin: 443, PortMax: 443, Protocol: "tcp", AppID: "wiki"},
		{IPMin: net.ParseIP("192.0.2.10"), IPMax: net.ParseIP("192.0.2.10"), PortMin: 22, PortMax: 22, Protocol: "tcp", AppID: "shell"},
		{IPMin: net.ParseIP("192.0.2.10"), IPMax: net.ParseIP("192.0.2.10"), PortMin: 22, PortMax: 22, Protocol: "tcp", AppID: "shell"},
		{IPMin: net.ParseIP("192.0.2.0"), IPMax: net.ParseIP("192.0.2.255"), PortMin: 80, PortMax: 443, Protocol: "tcp", AppID: "range"},
		{IPMin: net.ParseIP("198.51.100.7"), PortMin: 53, PortMax: 53, Protocol: "udp", AppID: "dns"},
	}
	domains := clientpkg.DomainResources{
		"intranet.example.com": {{PortMin: 443, PortMax: 443, Protocol: "tcp", AppID: "wiki"}},
		"files.example.com":    {{PortMin: 21, PortMax: 21, Protocol: "tcp", AppID: "ftp"}},
		"dns.example.com":      {{PortMin: 53, PortMax: 53, Protocol: "udp", AppID: "dns"}},
	}
	dns := map[string][]net.IP{
		"wiki.example.com": {net.ParseIP("192.0.2.10")},
	}
	return ips, domains, dns
}

func TestBuildResourcesDerivesDisplayEntries(t *testing.T) {
	ips, domains, dns := grant()
	resources := buildResources(ips, domains, dns, nil)
	resources.Build()

	if resources.Empty() {
		t.Fatal("resources are empty, want the granted rules")
	}
	if len(resources.IPs) != 5 {
		t.Fatalf("IP rules = %d, want the five granted rules", len(resources.IPs))
	}
	if len(resources.Entries) != 7 {
		t.Fatalf("entries = %d, want 7: %#v", len(resources.Entries), resources.Entries)
	}

	// Domains come before addresses; within a kind the order is stable.
	wantTitles := []string{
		"dns.example.com",
		"files.example.com",
		"intranet.example.com",
		"192.0.2.0 – 192.0.2.255",
		"192.0.2.10",
		"198.51.100.7",
		"wiki.example.com",
	}
	for index, want := range wantTitles {
		if resources.Entries[index].Title != want {
			t.Fatalf("entry %d title = %q, want %q", index, resources.Entries[index].Title, want)
		}
	}

	byTitle := make(map[string]ResourceEntry, len(resources.Entries))
	for _, entry := range resources.Entries {
		byTitle[entry.Title] = entry
	}

	intranet := byTitle["intranet.example.com"]
	if intranet.Kind != ResourceDomain || intranet.Detail != "tcp 443" || intranet.URL != "https://intranet.example.com" {
		t.Fatalf("intranet entry = %#v, want a tcp 443 domain with https", intranet)
	}
	if got := byTitle["dns.example.com"].URL; got != "" {
		t.Fatalf("UDP-only domain URL = %q, want none", got)
	}
	if got := byTitle["files.example.com"].URL; got != "" {
		t.Fatalf("FTP domain URL = %q, want none", got)
	}

	single := byTitle["192.0.2.10"]
	if single.Kind != ResourceIP || single.Detail != "tcp 22, tcp 443" {
		t.Fatalf("single IP entry = %#v, want both ports once", single)
	}
	if single.URL != "https://192.0.2.10" {
		t.Fatalf("single IP URL = %q, want https", single.URL)
	}
	if got := byTitle["192.0.2.0 – 192.0.2.255"].URL; got != "" {
		t.Fatalf("range URL = %q, want none", got)
	}
	if got := byTitle["198.51.100.7"].Detail; got != "udp 53" {
		t.Fatalf("UDP rule detail = %q", got)
	}

	dnsEntry := byTitle["wiki.example.com"]
	if dnsEntry.Kind != ResourceDNS || dnsEntry.Detail != "→ 192.0.2.10" || dnsEntry.URL != "" {
		t.Fatalf("DNS entry = %#v", dnsEntry)
	}
	if dnsEntry.Copy != "wiki.example.com" {
		t.Fatalf("DNS copy value = %q", dnsEntry.Copy)
	}
}

func TestBuildResourcesEmpty(t *testing.T) {
	resources := buildResources(nil, nil, nil, nil)
	if !resources.Empty() || len(resources.Entries) != 0 {
		t.Fatalf("empty grant produced %#v", resources)
	}
	if got := FormatResources(resources); !strings.Contains(got, "granted no resources") {
		t.Fatalf("empty format = %q", got)
	}
}

func TestBuildResourcesRetainsPortalApps(t *testing.T) {
	apps := []clientpkg.App{{
		ID: "wiki", Name: "Intranet Wiki", Group: "Portal",
		LaunchURL: "https://wiki.example.com", LaunchMethod: "web",
		WebDomains: []string{"wiki.example.com"},
	}}
	resources := buildResources(nil, nil, nil, apps)

	if len(resources.Apps) != 1 {
		t.Fatalf("apps = %#v, want the portal entry", resources.Apps)
	}
	app := resources.Apps[0]
	if app.Name != "Intranet Wiki" || app.Group != "Portal" || app.LaunchURL != "https://wiki.example.com" {
		t.Fatalf("app = %#v", app)
	}
	if len(resources.Entries) != 1 || resources.Entries[0].Kind != ResourceApp {
		t.Fatalf("entries = %#v, want one app row", resources.Entries)
	}
	if got := resources.Entries[0].URL; got != "https://wiki.example.com" {
		t.Fatalf("app row URL = %q", got)
	}
}

func TestSuggestURL(t *testing.T) {
	cases := []struct {
		name  string
		host  string
		rules []portRule
		want  string
	}{
		{name: "https exact", host: "192.0.2.10", rules: []portRule{{"tcp", 443, 443}}, want: "https://192.0.2.10"},
		{name: "http exact", host: "192.0.2.10", rules: []portRule{{"tcp", 80, 80}}, want: "http://192.0.2.10"},
		{name: "https wins", host: "host.example", rules: []portRule{{"tcp", 80, 80}, {"tcp", 443, 443}}, want: "https://host.example"},
		{name: "range covers", host: "host.example", rules: []portRule{{"tcp", 80, 443}}, want: "https://host.example"},
		{name: "all protocol", host: "host.example", rules: []portRule{{"all", 443, 443}}, want: "https://host.example"},
		{name: "udp only", host: "host.example", rules: []portRule{{"udp", 443, 443}}, want: ""},
		{name: "no browser port", host: "host.example", rules: []portRule{{"tcp", 22, 22}}, want: ""},
		{name: "no host", host: "", rules: []portRule{{"tcp", 443, 443}}, want: ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := suggestURL(test.host, test.rules); got != test.want {
				t.Fatalf("suggestURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestLaunchURLPrefersThePortalAccessAddress(t *testing.T) {
	cases := []struct {
		name string
		app  App
		want string
	}{
		{name: "access address", app: App{LaunchURL: "https://portal.example/app"}, want: "https://portal.example/app"},
		{name: "scheme added", app: App{LaunchURL: "portal.example/app"}, want: "https://portal.example/app"},
		{name: "web domain fallback", app: App{WebDomains: []string{"wiki.example.com"}}, want: "https://wiki.example.com"},
		{name: "nothing", app: App{Name: "ssh"}, want: ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := launchURL(test.app); got != test.want {
				t.Fatalf("launchURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAppEntriesCarryTheLaunchEntry(t *testing.T) {
	resources := Resources{Apps: []App{
		{ID: "a", Name: "Wiki", Group: "Portal", LaunchURL: "https://wiki.example.com", LaunchMethod: "web"},
		{ID: "b", Group: "Portal"},
	}}
	resources.Build()

	if len(resources.Entries) != 2 {
		t.Fatalf("entries = %#v", resources.Entries)
	}
	first := resources.Entries[0]
	if first.Kind != ResourceApp || first.Title != "Wiki" || first.URL != "https://wiki.example.com" {
		t.Fatalf("app entry = %#v", first)
	}
	if first.Detail != "Portal · web" {
		t.Fatalf("app detail = %q", first.Detail)
	}
	if first.Copy != "https://wiki.example.com" {
		t.Fatalf("app copy = %q", first.Copy)
	}
	if got := resources.Entries[1]; got.Title != "b" || got.URL != "" || got.Copy != "b" {
		t.Fatalf("fallback app entry = %#v", got)
	}
}

func TestFormatResources(t *testing.T) {
	resources := Resources{Entries: []ResourceEntry{
		{Kind: ResourceApp, Title: "Wiki", Detail: "Portal", URL: "https://wiki.example.com"},
		{Kind: ResourceIP, Title: "192.0.2.10", Detail: "tcp 443"},
	}}
	got := FormatResources(resources)
	want := "2 resource(s):\n" +
		"  app     Wiki | Portal | https://wiki.example.com\n" +
		"  ip      192.0.2.10 | tcp 443"
	if got != want {
		t.Fatalf("FormatResources() =\n%s\nwant\n%s", got, want)
	}
}

func TestResourceCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "resources.json")

	ips, domains, dns := grant()
	snapshot := ResourceSnapshot{
		SavedAt:  time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC),
		Profile:  "默认",
		Gateway:  "vpn.example.com:443",
		TunnelIP: "192.0.2.10",
		Resources: buildResources(ips, domains, dns, []clientpkg.App{{
			ID: "wiki", Name: "Wiki", Group: "Portal",
			LaunchURL: "https://wiki.example.com", LaunchMethod: "web",
		}}),
	}

	if err := WriteResourceCache(path, snapshot); err != nil {
		t.Fatalf("WriteResourceCache() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat cache: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("cache mode = %o, want 600", mode)
	}

	loaded, err := ReadResourceCache(path)
	if err != nil {
		t.Fatalf("ReadResourceCache() error = %v", err)
	}
	if loaded.Version != resourceCacheVersion || loaded.Profile != snapshot.Profile || loaded.Gateway != snapshot.Gateway {
		t.Fatalf("loaded snapshot = %#v", loaded)
	}
	if loaded.Version != 1 {
		t.Fatalf("version = %d, want 1", loaded.Version)
	}
	if len(loaded.Resources.Entries) != len(snapshot.Resources.Entries) {
		t.Fatalf("entries = %d, want %d", len(loaded.Resources.Entries), len(snapshot.Resources.Entries))
	}
	for index, want := range snapshot.Resources.Entries {
		if loaded.Resources.Entries[index] != want {
			t.Fatalf("entry %d = %#v, want %#v", index, loaded.Resources.Entries[index], want)
		}
	}
	if loaded.SavedAt.IsZero() {
		t.Fatal("the saved time was lost")
	}
}

func TestResourceCacheRefusesANewerVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"resources":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadResourceCache(path); err == nil {
		t.Fatal("a newer cache version was accepted")
	}
}

func TestResourceCacheMissingFile(t *testing.T) {
	if _, err := ReadResourceCache(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("a missing cache was accepted")
	}
}

func TestCacheResourcesWritesTheSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resources.json")
	manager := New(Options{ResourceCachePath: path})
	ips, domains, dns := grant()
	manager.cacheResources(
		config.NewProfile("默认"),
		"vpn.example.com:443",
		"192.0.2.10",
		buildResources(ips, domains, dns, nil),
	)

	snapshot, err := ReadResourceCache(path)
	if err != nil {
		t.Fatalf("ReadResourceCache() error = %v", err)
	}
	if snapshot.Profile != "默认" || snapshot.Gateway != "vpn.example.com:443" || snapshot.TunnelIP != "192.0.2.10" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if len(snapshot.Resources.Entries) == 0 {
		t.Fatal("the cached snapshot has no entries")
	}
}

func TestCacheResourcesWithoutAPathIsSilent(t *testing.T) {
	manager := New(Options{})
	manager.cacheResources(config.NewProfile("默认"), "vpn.example.com:443", "", Resources{})
}
