// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	clientpkg "github.com/mythologyli/zju-connect/client"
)

// ResourceKind separates what a resource entry is: an application the portal
// offers, or an address rule through which it is reached.
type ResourceKind string

const (
	// ResourceApp is one application the portal lists for the account.
	ResourceApp ResourceKind = "app"
	// ResourceDomain is a domain rule, e.g. an intranet name and its ports.
	ResourceDomain ResourceKind = "domain"
	// ResourceIP is an IPv4 address or range with its ports.
	ResourceIP ResourceKind = "ip"
	// ResourceDNS is a granted name resolved through the tunnel's DNS.
	ResourceDNS ResourceKind = "dns"
)

// ResourceEntry is one row in the window and one line in the headless output.
// It is derived from Resources so the window never formats addresses itself.
type ResourceEntry struct {
	Kind   ResourceKind `json:"kind"`
	Title  string       `json:"title"`
	Detail string       `json:"detail,omitempty"`
	// URL is a browser link when the rule or application suggests one.
	URL string `json:"url,omitempty"`
	// Copy is what the 复制 button puts on the clipboard: the URL when there
	// is one, otherwise the title.
	Copy string `json:"copy"`
}

// App is one application the portal granted. The address rules reference it by
// AppID; the name and launch hint come from the portal's own list.
type App struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Group        string   `json:"group,omitempty"`
	LaunchURL    string   `json:"launch_url,omitempty"`
	LaunchMethod string   `json:"launch_method,omitempty"`
	WebDomains   []string `json:"web_domains,omitempty"`
}

// IPRule is one granted IPv4 address or range. It is a display and cache copy
// of client.IPResource: addresses are strings so the cached JSON stays
// readable.
type IPRule struct {
	From     string `json:"from"`
	To       string `json:"to"`
	PortMin  int    `json:"port_min"`
	PortMax  int    `json:"port_max"`
	Protocol string `json:"protocol"`
	AppID    string `json:"app_id,omitempty"`
}

// DomainRule is one granted domain with the ports it allows.
type DomainRule struct {
	Name     string `json:"name"`
	PortMin  int    `json:"port_min"`
	PortMax  int    `json:"port_max"`
	Protocol string `json:"protocol"`
	AppID    string `json:"app_id,omitempty"`
}

// DNSRule is one granted name and the addresses the gateway resolves it to.
type DNSRule struct {
	Name string   `json:"name"`
	IPs  []string `json:"ips"`
}

// Resources is what the gateway granted this account. It is retained in State
// after a login instead of being dropped once the data path is assembled.
type Resources struct {
	Apps    []App        `json:"apps,omitempty"`
	IPs     []IPRule     `json:"ips,omitempty"`
	Domains []DomainRule `json:"domains,omitempty"`
	DNS     []DNSRule    `json:"dns,omitempty"`
	// Entries is the display model. It is derived, so it is not cached: a
	// build that formats resources differently must not replay old rows.
	Entries []ResourceEntry `json:"-"`
}

// Empty reports whether the gateway granted nothing at all.
func (r Resources) Empty() bool {
	return len(r.Apps) == 0 && len(r.IPs) == 0 && len(r.Domains) == 0 && len(r.DNS) == 0
}

// Build refreshes the display entries from the raw rules.
func (r *Resources) Build() {
	r.Entries = deriveEntries(*r)
}

// buildResources snapshots a login's grant. A rule set the gateway does not
// send arrives as nil and simply contributes nothing: a session with only IP
// rules is still usable.
func buildResources(
	ips []clientpkg.IPResource,
	domains clientpkg.DomainResources,
	dns map[string][]net.IP,
	apps []clientpkg.App,
) Resources {
	var resources Resources

	for _, app := range apps {
		resources.Apps = append(resources.Apps, App{
			ID:           app.ID,
			Name:         app.Name,
			Group:        app.Group,
			LaunchURL:    app.LaunchURL,
			LaunchMethod: app.LaunchMethod,
			WebDomains:   append([]string(nil), app.WebDomains...),
		})
	}
	for _, rule := range ips {
		resources.IPs = append(resources.IPs, ipRule(rule))
	}
	for name, rules := range domains {
		for _, rule := range rules {
			resources.Domains = append(resources.Domains, DomainRule{
				Name:     name,
				PortMin:  rule.PortMin,
				PortMax:  rule.PortMax,
				Protocol: rule.Protocol,
				AppID:    rule.AppID,
			})
		}
	}
	for name, addresses := range dns {
		values := make([]string, 0, len(addresses))
		for _, ip := range addresses {
			if ip != nil {
				values = append(values, ip.String())
			}
		}
		resources.DNS = append(resources.DNS, DNSRule{Name: name, IPs: values})
	}

	resources.normalise()
	resources.Build()
	return resources
}

// ipRule converts one client rule. Client code may leave IPMax nil for a
// single address, which is what the window would otherwise print as a range.
func ipRule(rule clientpkg.IPResource) IPRule {
	from := ipString(rule.IPMin)
	to := ipString(rule.IPMax)
	if to == "" {
		to = from
	}
	return IPRule{
		From:     from,
		To:       to,
		PortMin:  rule.PortMin,
		PortMax:  rule.PortMax,
		Protocol: rule.Protocol,
		AppID:    rule.AppID,
	}
}

func ipString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
}

// normalise gives the snapshot a deterministic order before it is rendered,
// cached or compared in a test.
func (r *Resources) normalise() {
	sort.SliceStable(r.IPs, func(i, j int) bool {
		if r.IPs[i].From != r.IPs[j].From {
			return lessIP(r.IPs[i].From, r.IPs[j].From)
		}
		if r.IPs[i].To != r.IPs[j].To {
			return lessIP(r.IPs[i].To, r.IPs[j].To)
		}
		if r.IPs[i].PortMin != r.IPs[j].PortMin {
			return r.IPs[i].PortMin < r.IPs[j].PortMin
		}
		return r.IPs[i].Protocol < r.IPs[j].Protocol
	})
	sort.SliceStable(r.Domains, func(i, j int) bool {
		if r.Domains[i].Name != r.Domains[j].Name {
			return r.Domains[i].Name < r.Domains[j].Name
		}
		if r.Domains[i].PortMin != r.Domains[j].PortMin {
			return r.Domains[i].PortMin < r.Domains[j].PortMin
		}
		return r.Domains[i].Protocol < r.Domains[j].Protocol
	})
	sort.SliceStable(r.DNS, func(i, j int) bool { return r.DNS[i].Name < r.DNS[j].Name })
	sort.SliceStable(r.Apps, func(i, j int) bool {
		if r.Apps[i].Group != r.Apps[j].Group {
			return r.Apps[i].Group < r.Apps[j].Group
		}
		if r.Apps[i].Name != r.Apps[j].Name {
			return r.Apps[i].Name < r.Apps[j].Name
		}
		return r.Apps[i].ID < r.Apps[j].ID
	})
}

func lessIP(a, b string) bool {
	left, leftErr := netip.ParseAddr(a)
	right, rightErr := netip.ParseAddr(b)
	if leftErr == nil && rightErr == nil {
		return left.Compare(right) < 0
	}
	return a < b
}

// deriveEntries turns the granted rules into the rows the window and the
// headless output share.
func deriveEntries(resources Resources) []ResourceEntry {
	entries := make([]ResourceEntry, 0, len(resources.Apps)+len(resources.Domains)+len(resources.IPs)+len(resources.DNS))

	for _, app := range resources.Apps {
		title := strings.TrimSpace(app.Name)
		if title == "" {
			title = strings.TrimSpace(app.ID)
		}
		if title == "" {
			continue
		}
		url := launchURL(app)
		detail := strings.TrimSpace(app.Group)
		if method := strings.TrimSpace(app.LaunchMethod); method != "" {
			if detail != "" {
				detail += " · "
			}
			detail += method
		}
		entries = append(entries, ResourceEntry{
			Kind:   ResourceApp,
			Title:  title,
			Detail: detail,
			URL:    url,
			Copy:   firstNonEmpty(url, title),
		})
	}

	// One row per domain, with all of its rules on the detail line.
	byDomain := make(map[string][]DomainRule)
	domainNames := make([]string, 0, len(resources.Domains))
	for _, rule := range resources.Domains {
		if _, seen := byDomain[rule.Name]; !seen {
			domainNames = append(domainNames, rule.Name)
		}
		byDomain[rule.Name] = append(byDomain[rule.Name], rule)
	}
	sort.Strings(domainNames)
	for _, name := range domainNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		rules := byDomain[name]
		portRules := make([]portRule, 0, len(rules))
		for _, rule := range rules {
			portRules = append(portRules, portRule{rule.Protocol, rule.PortMin, rule.PortMax})
		}
		entries = append(entries, ResourceEntry{
			Kind:   ResourceDomain,
			Title:  name,
			Detail: describeRules(portRules),
			URL:    suggestURL(name, portRules),
			Copy:   name,
		})
	}

	// One row per address range, merging the ports granted to it.
	type rangeKey struct{ from, to string }
	byRange := make(map[rangeKey][]IPRule)
	rangeOrder := make([]rangeKey, 0, len(resources.IPs))
	for _, rule := range resources.IPs {
		key := rangeKey{rule.From, rule.To}
		if _, seen := byRange[key]; !seen {
			rangeOrder = append(rangeOrder, key)
		}
		byRange[key] = append(byRange[key], rule)
	}
	sort.SliceStable(rangeOrder, func(i, j int) bool {
		if rangeOrder[i].from != rangeOrder[j].from {
			return lessIP(rangeOrder[i].from, rangeOrder[j].from)
		}
		return lessIP(rangeOrder[i].to, rangeOrder[j].to)
	})
	for _, key := range rangeOrder {
		if strings.TrimSpace(key.from) == "" {
			continue
		}
		rules := byRange[key]
		portRules := make([]portRule, 0, len(rules))
		for _, rule := range rules {
			portRules = append(portRules, portRule{rule.Protocol, rule.PortMin, rule.PortMax})
		}
		title := key.from
		if key.to != "" && key.to != key.from {
			title = key.from + " – " + key.to
		}
		url := ""
		if key.from == key.to {
			url = suggestURL(key.from, portRules)
		}
		entries = append(entries, ResourceEntry{
			Kind:   ResourceIP,
			Title:  title,
			Detail: describeRules(portRules),
			URL:    url,
			Copy:   title,
		})
	}

	for _, rule := range resources.DNS {
		if strings.TrimSpace(rule.Name) == "" {
			continue
		}
		detail := strings.Join(rule.IPs, ", ")
		if detail == "" {
			detail = "no address"
		}
		entries = append(entries, ResourceEntry{
			Kind:   ResourceDNS,
			Title:  rule.Name,
			Detail: "→ " + detail,
			Copy:   rule.Name,
		})
	}

	return entries
}

// portRule is the part of a rule the URL suggestion needs.
type portRule struct {
	Protocol string
	PortMin  int
	PortMax  int
}

// describeRules renders protocol and ports without repeating a rule that the
// gateway listed twice.
func describeRules(rules []portRule) string {
	seen := make(map[string]bool)
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		protocol := rule.Protocol
		if protocol == "all" {
			protocol = "tcp/udp"
		}
		part := protocol + " " + portRange(rule.PortMin, rule.PortMax)
		if seen[part] {
			continue
		}
		seen[part] = true
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func portRange(min, max int) string {
	if min == max {
		return strconv.Itoa(min)
	}
	return strconv.Itoa(min) + "-" + strconv.Itoa(max)
}

// suggestURL offers a browser link when the rule really implies one: an exact
// host (not a range) and a granted TCP port that a browser speaks, 443 before
// 80. The default port is left out so the link is the one a user would type.
func suggestURL(host string, rules []portRule) string {
	if host == "" {
		return ""
	}
	for _, candidate := range []struct {
		port   int
		scheme string
	}{{443, "https"}, {80, "http"}} {
		for _, rule := range rules {
			if rule.Protocol != "tcp" && rule.Protocol != "all" {
				continue
			}
			if rule.PortMin <= candidate.port && candidate.port <= rule.PortMax {
				return candidate.scheme + "://" + host
			}
		}
	}
	return ""
}

// launchURL prefers the portal's access address, then the first web address it
// lists. A value without a scheme gets https, which is what the portal's web
// applications use.
func launchURL(app App) string {
	for _, candidate := range append([]string{app.LaunchURL}, app.WebDomains...) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, "://") {
			return candidate
		}
		return "https://" + candidate
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// DefaultAppURL is the launch address of the first portal application that
// has one, in the order the window lists them. A grant with no application
// metadata falls back to the first host rule that has a browser link, so a
// gateway that lists only address rules still gives the tray something to
// open. An empty string means nothing on this grant is a browser destination.
func (r Resources) DefaultAppURL() string {
	for _, entry := range r.Entries {
		if entry.Kind == ResourceApp && entry.URL != "" {
			return entry.URL
		}
	}
	for _, entry := range r.Entries {
		if entry.URL != "" {
			return entry.URL
		}
	}
	return ""
}

// FormatResources renders the snapshot for a terminal: one line per entry, in
// the same order the window shows them.
func FormatResources(resources Resources) string {
	entries := resources.Entries
	if len(entries) == 0 {
		return "the gateway granted no resources to this account"
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, fmt.Sprintf("%d resource(s):", len(entries)))
	for _, entry := range entries {
		line := fmt.Sprintf("  %-7s %s", entry.Kind, entry.Title)
		if entry.Detail != "" {
			line += " | " + entry.Detail
		}
		if entry.URL != "" {
			line += " | " + entry.URL
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// resourceCacheVersion guards the cached snapshot. A newer build may add
// fields; an older one must not silently treat them as absent.
const resourceCacheVersion = 1

// ResourceSnapshot is the last grant, plus enough context to tell a user whose
// grant it was when --print-resources runs without a login.
type ResourceSnapshot struct {
	Version   int       `json:"version"`
	SavedAt   time.Time `json:"saved_at"`
	Profile   string    `json:"profile"`
	Gateway   string    `json:"gateway"`
	TunnelIP  string    `json:"tunnel_ip,omitempty"`
	Resources Resources `json:"resources"`
}

// ReadResourceCache loads the snapshot the last login wrote. A missing file is
// reported as a normal error: --print-resources has nothing to print yet.
func ReadResourceCache(path string) (ResourceSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ResourceSnapshot{}, fmt.Errorf("read %s: %w", path, err)
	}
	var snapshot ResourceSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return ResourceSnapshot{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if snapshot.Version > resourceCacheVersion {
		return ResourceSnapshot{}, fmt.Errorf(
			"%s uses resource cache version %d, which this build does not understand (latest %d)",
			path, snapshot.Version, resourceCacheVersion)
	}
	snapshot.Resources.normalise()
	snapshot.Resources.Build()
	return snapshot, nil
}

// WriteResourceCache stores the snapshot beside the settings. The file lists
// intranet addresses, so it is 0600 like the client data, and the rename keeps
// a crash from leaving a half-written file that the next --print-resources
// would parse.
func WriteResourceCache(path string, snapshot ResourceSnapshot) error {
	if path == "" {
		return errors.New("no resource cache path")
	}
	snapshot.Version = resourceCacheVersion
	snapshot.Resources.Build()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode resources: %w", err)
	}
	encoded = append(encoded, '\n')

	temp, err := os.CreateTemp(dir, ".resources-*.tmp")
	if err != nil {
		return fmt.Errorf("create the resource cache: %w", err)
	}
	name := temp.Name()
	defer func() { _ = os.Remove(name) }()

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("restrict the resource cache: %w", err)
	}
	if _, err := temp.Write(encoded); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write the resource cache: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("sync the resource cache: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close the resource cache: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("replace the resource cache: %w", err)
	}
	return nil
}
