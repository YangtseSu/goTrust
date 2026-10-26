// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/profilelock"
	"github.com/YangtseSu/goTrust/core/session"
)

// testSettings writes a complete settings file whose gateway refuses
// connections, and returns it with the file it lives in.
func testSettings(t *testing.T) (config.Config, string) {
	t.Helper()
	cfg := config.Default()
	cfg.Profiles[0].Server.Address = "127.0.0.1"
	cfg.Profiles[0].Server.Port = 1
	cfg.Profiles[0].Auth.Username = "alice"

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := config.SaveFile(path, cfg); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	return cfg, path
}

// seedGrant caches a grant the way a login would, so the reporting commands
// have something to print without a gateway.
func seedGrant(t *testing.T, configPath string) session.ResourceSnapshot {
	t.Helper()
	snapshot := session.ResourceSnapshot{
		SavedAt:  time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC),
		Profile:  config.DefaultProfileName,
		Gateway:  "vpn.example.com:443",
		TunnelIP: "192.0.2.10",
		Resources: session.Resources{
			IPs: []session.IPRule{{
				From:     "192.0.2.20",
				To:       "192.0.2.20",
				PortMin:  443,
				PortMax:  443,
				Protocol: "tcp",
			}},
		},
	}
	if err := session.WriteResourceCache(config.ResourcesPath(configPath), snapshot); err != nil {
		t.Fatalf("seed the grant: %v", err)
	}
	return snapshot
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestStatusReportsAFreeProfile(t *testing.T) {
	_, path := testSettings(t)

	code, stdout, stderr := run(t, "-config", path, "-status")
	if code != ExitNotRunning {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, ExitNotRunning, stderr)
	}
	if !strings.Contains(stdout, "not in use") {
		t.Fatalf("stdout = %q, want it to say the profile is free", stdout)
	}
	if !strings.Contains(stdout, "no cached grant") {
		t.Fatalf("stdout = %q, want it to say no grant is cached", stdout)
	}
}

func TestStatusReportsTheHolder(t *testing.T) {
	cfg, path := testSettings(t)

	lock, err := profilelock.Acquire(path, cfg.ActiveProfile)
	if err != nil {
		t.Fatalf("hold the profile: %v", err)
	}
	defer func() { _ = lock.Release() }()

	code, stdout, stderr := run(t, "-config", path, "-status")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if !strings.Contains(stdout, "in use by process") {
		t.Fatalf("stdout = %q, want it to name the holding process", stdout)
	}
}

func TestStatusPrintsTheCachedGrant(t *testing.T) {
	_, path := testSettings(t)
	seedGrant(t, path)

	code, stdout, _ := run(t, "-config", path, "-status")
	if code != ExitNotRunning {
		t.Fatalf("exit code = %d, want %d", code, ExitNotRunning)
	}
	for _, want := range []string{"192.0.2.20", "vpn.example.com:443"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestStatusJSON(t *testing.T) {
	cfg, path := testSettings(t)
	seedGrant(t, path)

	lock, err := profilelock.Acquire(path, cfg.ActiveProfile)
	if err != nil {
		t.Fatalf("hold the profile: %v", err)
	}
	defer func() { _ = lock.Release() }()

	code, stdout, stderr := run(t, "-config", path, "-status", "-json")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	var report statusReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("decode the status: %v (stdout %q)", err, stdout)
	}
	if !report.InUse || report.PID <= 0 {
		t.Fatalf("report = %+v, want a held profile with a process id", report)
	}
	if report.Profile != cfg.ActiveProfile {
		t.Fatalf("profile = %q, want %q", report.Profile, cfg.ActiveProfile)
	}
	if report.Grant == nil || report.Grant.Gateway != "vpn.example.com:443" {
		t.Fatalf("grant = %+v, want the cached one", report.Grant)
	}
}

func TestPrintResourcesUsesTheCache(t *testing.T) {
	_, path := testSettings(t)
	seedGrant(t, path)

	code, stdout, stderr := run(t, "-config", path, "-print-resources")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	for _, want := range []string{"saved 2026-10-27T12:00:00Z", "by profile", "vpn.example.com:443", "192.0.2.20"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestPrintResourcesJSON(t *testing.T) {
	_, path := testSettings(t)
	seedGrant(t, path)

	code, stdout, stderr := run(t, "-config", path, "-print-resources", "-json")
	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	var snapshot session.ResourceSnapshot
	if err := json.Unmarshal([]byte(stdout), &snapshot); err != nil {
		t.Fatalf("decode the snapshot: %v (stdout %q)", err, stdout)
	}
	if snapshot.Version != 1 || snapshot.Profile != config.DefaultProfileName {
		t.Fatalf("snapshot = %+v, want version 1 and the default profile", snapshot)
	}
	if len(snapshot.Resources.IPs) != 1 || snapshot.Resources.IPs[0].From != "192.0.2.20" {
		t.Fatalf("resources = %+v, want the seeded address rule", snapshot.Resources)
	}
}

func TestPrintResourcesWithoutACacheFails(t *testing.T) {
	_, path := testSettings(t)

	code, _, stderr := run(t, "-config", path, "-print-resources")
	if code != ExitFailure {
		t.Fatalf("exit code = %d, want %d", code, ExitFailure)
	}
	if !strings.Contains(stderr, "grant:") {
		t.Fatalf("stderr = %q, want the cache error", stderr)
	}
}

func TestUsageErrors(t *testing.T) {
	_, path := testSettings(t)

	cases := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"-config", path, "-nope"}},
		{"two commands", []string{"-config", path, "-status", "-print-resources"}},
		{"stray argument", []string{"-config", path, "extra"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if code, _, _ := run(t, testCase.args...); code != ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, ExitUsage)
			}
		})
	}
}

func TestHelpIsSuccess(t *testing.T) {
	if code, _, _ := run(t, "-h"); code != ExitOK {
		t.Fatalf("exit code for -h = %d, want %d", code, ExitOK)
	}
}

func TestInvalidSettingsFailBeforeALogin(t *testing.T) {
	cfg, path := testSettings(t)
	cfg.Profiles[0].Auth.Username = ""
	if err := config.SaveFile(path, cfg); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	code, _, stderr := run(t, "-config", path)
	if code != ExitFailure {
		t.Fatalf("exit code = %d, want %d", code, ExitFailure)
	}
	if !strings.Contains(stderr, "settings:") || !strings.Contains(stderr, "username") {
		t.Fatalf("stderr = %q, want the missing username", stderr)
	}
}

func TestFailedLoginReleasesTheProfile(t *testing.T) {
	// The log file belongs to the user's state directory; keep the test out of
	// it.
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg, path := testSettings(t)

	code, _, stderr := run(t, "-config", path)
	if code != ExitFailure {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, ExitFailure, stderr)
	}
	if !strings.Contains(stderr, "connect:") {
		t.Fatalf("stderr = %q, want the failure the gateway dial produced", stderr)
	}
	if _, held, err := profilelock.Probe(path, cfg.ActiveProfile); err != nil || held {
		t.Fatalf("the profile is still held after a failed login: held=%v err=%v", held, err)
	}
}

func TestFailedLoginLeavesAFreeProfileForTheNextRun(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, path := testSettings(t)

	if code, _, _ := run(t, "-config", path); code != ExitFailure {
		t.Fatalf("first run exit code = %d, want %d", code, ExitFailure)
	}
	if code, _, _ := run(t, "-config", path, "-status"); code != ExitNotRunning {
		t.Fatalf("status exit code = %d, want %d", code, ExitNotRunning)
	}
}

// A forced change has to reach the retry's credential even when the file
// cannot be written, and the file must respect a profile that stores no
// password.
func TestSaveChangedPassword(t *testing.T) {
	var logged []string
	logf := func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	}

	cfg := config.Default()
	cfg.Profiles[0].Auth.StorePass = false
	cfg.Profiles[0].Auth.Password = "old"
	path := filepath.Join(t.TempDir(), "config.toml")

	saveChangedPassword(&cfg, path, cfg.ActiveProfile, "new", logf)
	if cfg.Profiles[0].Auth.Password != "new" {
		t.Fatalf("in-memory password = %q, want the changed one", cfg.Profiles[0].Auth.Password)
	}
	loaded, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("load the saved settings: %v", err)
	}
	if loaded.Profiles[0].Auth.Password != "" {
		t.Fatalf("file password = %q, want it cleared for a profile that stores none", loaded.Profiles[0].Auth.Password)
	}

	cfg.Profiles[0].Auth.StorePass = true
	saveChangedPassword(&cfg, path, cfg.ActiveProfile, "changed", logf)
	loaded, err = config.LoadFile(path)
	if err != nil {
		t.Fatalf("load the saved settings: %v", err)
	}
	if loaded.Profiles[0].Auth.Password != "changed" {
		t.Fatalf("file password = %q, want the changed one", loaded.Profiles[0].Auth.Password)
	}

	// A file that cannot be written must not cost the retry its credential.
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("create the blocker: %v", err)
	}
	saveChangedPassword(&cfg, filepath.Join(blocker, "config.toml"), cfg.ActiveProfile, "retry", logf)
	if cfg.Profiles[0].Auth.Password != "retry" {
		t.Fatalf("in-memory password = %q, want it kept after a failed save", cfg.Profiles[0].Auth.Password)
	}
	if !strings.Contains(strings.Join(logged, "\n"), "save the changed password") {
		t.Fatalf("log = %q, want the save failure", logged)
	}
}

// The live run's --json output is a machine contract: a deployment reads the
// grant from it instead of scraping the text rows.
func TestLiveGrantJSON(t *testing.T) {
	grant := liveGrant{
		Profile:  config.DefaultProfileName,
		Gateway:  "vpn.example.com:443",
		TunnelIP: "192.0.2.10",
		Resources: session.Resources{
			IPs: []session.IPRule{{
				From:     "192.0.2.20",
				To:       "192.0.2.20",
				PortMin:  443,
				PortMax:  443,
				Protocol: "tcp",
			}},
		},
	}

	var out bytes.Buffer
	printGrant(&out, grant, true)

	var decoded struct {
		Profile   string `json:"profile"`
		Gateway   string `json:"gateway"`
		TunnelIP  string `json:"tunnel_ip"`
		Resources struct {
			IPs []struct {
				From     string `json:"from"`
				Protocol string `json:"protocol"`
			} `json:"ips"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode the grant: %v (output %q)", err, out.String())
	}
	if decoded.Profile != config.DefaultProfileName || decoded.Gateway != "vpn.example.com:443" || decoded.TunnelIP != "192.0.2.10" {
		t.Fatalf("grant = %+v", decoded)
	}
	if len(decoded.Resources.IPs) != 1 || decoded.Resources.IPs[0].From != "192.0.2.20" || decoded.Resources.IPs[0].Protocol != "tcp" {
		t.Fatalf("resources = %+v", decoded.Resources)
	}
}
