// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"strconv"
	"time"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/profilelock"
	"github.com/YangtseSu/goTrust/core/session"
)

// liveGrant is what a completed login reports: whose grant it is, through which
// gateway, and the grant itself.
type liveGrant struct {
	Profile   string            `json:"profile"`
	Gateway   string            `json:"gateway"`
	TunnelIP  string            `json:"tunnel_ip,omitempty"`
	Resources session.Resources `json:"resources"`
}

// statusReport is the machine-readable form of --status. The grant is the last
// cached one: a separate process cannot read the live session's state, and a
// grant written by the running login is the closest honest answer.
type statusReport struct {
	Profile string                    `json:"profile"`
	InUse   bool                      `json:"in_use"`
	PID     int                       `json:"pid,omitempty"`
	Grant   *session.ResourceSnapshot `json:"grant,omitempty"`
}

// reportStatus answers whether an instance holds the profile. It is a probe of
// the same lock the front-ends take, so it reports a login that is starting or
// shutting down as in use, which is exactly what a second instance would find.
func reportStatus(stdout, stderr io.Writer, profile, configPath string, asJSON bool) int {
	pid, held, err := profilelock.Probe(configPath, profile)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return ExitFailure
	}

	snapshot, grantErr := session.ReadResourceCache(config.ResourcesPath(configPath))
	cached := grantErr == nil
	if grantErr != nil && !errors.Is(grantErr, fs.ErrNotExist) {
		// A cache this build cannot read says nothing about the live lock, so
		// report it and let the state line stay authoritative.
		fmt.Fprintf(stderr, "grant: %v\n", grantErr)
	}

	if asJSON {
		report := statusReport{Profile: profile, InUse: held, PID: pid}
		if cached {
			report.Grant = &snapshot
		}
		writeJSON(stdout, report)
	} else {
		switch {
		case held && pid > 0:
			fmt.Fprintf(stdout, "profile %s: in use by process %d\n", profile, pid)
		case held:
			fmt.Fprintf(stdout, "profile %s: in use\n", profile)
		default:
			fmt.Fprintf(stdout, "profile %s: not in use\n", profile)
		}
		if cached {
			writeSnapshotText(stdout, snapshot)
		} else {
			fmt.Fprintln(stdout, "no cached grant")
		}
	}

	if held {
		return ExitOK
	}
	return ExitNotRunning
}

// reportCachedGrant prints the snapshot the last login wrote, without a login.
func reportCachedGrant(stdout, stderr io.Writer, configPath string, asJSON bool) int {
	snapshot, err := session.ReadResourceCache(config.ResourcesPath(configPath))
	if err != nil {
		fmt.Fprintf(stderr, "grant: %v\n", err)
		return ExitFailure
	}
	if asJSON {
		writeJSON(stdout, snapshot)
	} else {
		writeSnapshotText(stdout, snapshot)
	}
	return ExitOK
}

// writeSnapshotText says whose grant this is before the rows: a cached snapshot
// may be days old, and --print-resources runs without a login.
func writeSnapshotText(w io.Writer, snapshot session.ResourceSnapshot) {
	if !snapshot.SavedAt.IsZero() {
		fmt.Fprintf(w, "saved %s by profile %s via %s",
			snapshot.SavedAt.Format(time.RFC3339), snapshot.Profile, snapshot.Gateway)
		if snapshot.TunnelIP != "" {
			fmt.Fprintf(w, ", tunnel %s", snapshot.TunnelIP)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, session.FormatResources(snapshot.Resources))
}

func printGrant(w io.Writer, grant liveGrant, asJSON bool) {
	if asJSON {
		writeJSON(w, grant)
		return
	}
	fmt.Fprintln(w, session.FormatResources(grant.Resources))
}

// gatewayAddress names the gateway the way the cached snapshot does.
func gatewayAddress(cfg config.Config) string {
	profile := cfg.Active()
	return net.JoinHostPort(profile.ServerAddress(), strconv.Itoa(profile.Server.Port))
}

func writeJSON(w io.Writer, value any) {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	// A closed stdout (a pipe that was read and dropped) must not end the
	// session: the log still carries everything the operator needs.
	_ = encoder.Encode(value)
}
