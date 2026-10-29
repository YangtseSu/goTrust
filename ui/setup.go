// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/locales"
)

// ProfileProblem names the first field a login needs and the profile does not
// have. An empty result means the profile can be used for a login.
//
// config.Profile.Validate is the authority and speaks English, because
// core/config is a library boundary; the window shows this sentence instead,
// and the connect path refuses with it. TestProfileProblemMatchesValidate keeps
// the two lists identical.
func ProfileProblem(profile config.Profile) string {
	switch {
	case profile.ServerAddress() == "":
		return locales.ProblemAddress()
	case profile.Server.Port <= 0 || profile.Server.Port > 65535:
		return locales.ProblemPort()
	case profile.Auth.Username == "":
		return locales.ProblemUsername()
	default:
		return ""
	}
}

// SetupDraft is the first run: the least a login needs, edited inside the
// window. The pages edit the rest of the configuration; a fresh install only
// has to answer what a login needs.
type SetupDraft struct {
	// Profile is the name the draft was built from, so a profile switch
	// rebuilds the fields instead of writing them onto another account.
	Profile string

	Address  string
	Port     string
	Username string
	// Password may stay empty: a gateway that uses a second factor or unified
	// authentication asks during the login, not here.
	Password  string
	StorePass bool

	// Error is the reason the last save did not happen.
	Error string
}

// NewSetupDraft fills the form from a profile, so a half-filled configuration
// is edited in place rather than retyped.
func NewSetupDraft(profile config.Profile) *SetupDraft {
	port := profile.Server.Port
	if port <= 0 {
		port = 443
	}
	name := profile.Name
	if name == "" {
		name = config.DefaultProfileName
	}
	return &SetupDraft{
		Profile:   name,
		Address:   profile.Server.Address,
		Port:      strconv.Itoa(port),
		Username:  profile.Auth.Username,
		Password:  profile.Auth.Password,
		StorePass: profile.Auth.StorePass && profile.Auth.Password != "",
	}
}

// Complete reports whether the form has what a login needs: an address, a valid
// port and a username. The password is not part of it.
func (d *SetupDraft) Complete() bool {
	port, err := strconv.Atoi(strings.TrimSpace(d.Port))
	if err != nil || port <= 0 || port > 65535 {
		return false
	}
	return strings.TrimSpace(d.Address) != "" && strings.TrimSpace(d.Username) != ""
}

// Problem names what the form is still missing, in the same words the connect
// path uses.
func (d *SetupDraft) Problem() string {
	port, err := strconv.Atoi(strings.TrimSpace(d.Port))
	switch {
	case strings.TrimSpace(d.Address) == "":
		return locales.ProblemAddress()
	case err != nil || port <= 0 || port > 65535:
		return locales.ProblemPort()
	case strings.TrimSpace(d.Username) == "":
		return locales.ProblemUsername()
	default:
		return ""
	}
}

// Apply writes the form into cfg's active profile. A password the user asked to
// keep is stored and marked; otherwise the file keeps none, exactly like the
// account page's 保存密码.
func (d *SetupDraft) Apply(cfg *config.Config) error {
	index := cfg.ProfileIndex(cfg.ActiveProfile)
	if index < 0 {
		return errors.New(locales.ProfileMissing(cfg.ActiveProfile))
	}
	name := cfg.Profiles[index].Name
	if name == "" {
		name = config.DefaultProfileName
	}
	if d.Profile != "" && d.Profile != name {
		return errors.New(locales.ProfileChanged())
	}
	if problem := d.Problem(); problem != "" {
		return errors.New(problem)
	}
	port, _ := strconv.Atoi(strings.TrimSpace(d.Port))

	profile := cfg.Profiles[index]
	profile.Server.Address = strings.TrimSpace(d.Address)
	profile.Server.Port = port
	profile.Auth.Username = strings.TrimSpace(d.Username)
	if d.StorePass && d.Password != "" {
		profile.Auth.Password = d.Password
		profile.Auth.StorePass = true
	} else {
		profile.Auth.Password = ""
		profile.Auth.StorePass = false
	}
	cfg.Profiles[index] = profile
	cfg.ActiveProfile = name
	cfg.Version = config.CurrentVersion
	return nil
}

// setupPanel is the first-run form. It replaces the connection controls while
// the active profile cannot be used, so 连接 never ends in an error that names
// a field the window does not offer.
func setupPanel(c *ui.Context, state *State, actions Actions) {
	draft := state.Setup
	busy := state.Busy

	ui.Column(c).Grow(1).Shrink(1).Gap(sectionGap).Children(func() {
		ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
			ui.Column(c).Gap(sectionGap).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					panelHeading(c, locales.SetupTitle())
					caption(c, locales.SetupHint())
				})

				ui.Column(c).Gap(rowGap).Children(func() {
					ui.Field(c, locales.FieldGateway(), func() {
						ui.TextInput(c, &draft.Address).Placeholder("vpn.example.com")
					})
					ui.Field(c, locales.FieldPort(), func() {
						ui.TextInput(c, &draft.Port).Placeholder("443")
					})
					ui.Field(c, locales.FieldUsername(), func() {
						ui.TextInput(c, &draft.Username)
					})
					ui.Field(c, locales.FieldPassword(), func() {
						ui.TextInput(c, &draft.Password).Password().Placeholder(locales.SetupPasswordPlaceholder())
						caption(c, locales.SetupPasswordHint())
					})
					ui.Checkbox(c, &draft.StorePass, locales.SetupStorePassword())
					if draft.Error != "" {
						captionTone(c, draft.Error, toneDanger)
					}
				})
			})
		})

		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			if ui.PrimaryButton(c, locales.SetupConnect()).
				Disabled(busy || !draft.Complete()).Clicked() && actions.SaveSetup != nil {
				actions.SaveSetup(*draft, true)
			}
			if ui.Button(c, locales.SetupSaveOnly()).
				Disabled(busy || !draft.Complete()).Clicked() && actions.SaveSetup != nil {
				actions.SaveSetup(*draft, false)
			}
			if busy {
				busyLine(c)
			}
		})
	})
}
