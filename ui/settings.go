// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	"github.com/YangtseSu/goTrust/locales"
)

// SettingsState is the settings window's draft. It is a clone of the live
// configuration: closing the window without saving discards the edits.
//
// The text fields edit the draft directly. Port and keep-alive targets need
// string buffers because the file stores an integer and a list, so those live
// in parallel slices indexed like Draft.Profiles.
type SettingsState struct {
	Draft    config.Config
	Selected int
	// SelectText is the profile dropdown's value. Select writes it, then the
	// view maps it back to Selected.
	SelectText string
	NameText   string
	Ports      []string
	Targets    []string
	Status     string
	// StatusError paints the status as a failure instead of a confirmation.
	StatusError bool
	LogDir      string
	Busy        bool
	// Online mirrors the live session: a voluntary password change goes through
	// the portal session, so it needs a connected client.
	Online bool
	// Password is the voluntary change panel, nil while the sections show.
	// PasswordBusy disables its buttons while the portal answers, and
	// PasswordStatus reports the outcome inside the panel.
	Password       *PasswordPrompt
	PasswordBusy   bool
	PasswordStatus string
	PasswordError  bool
	// TrustKnown and Trusted are the portal's answer about this device, read
	// through the live session. TrustStatus reports a query or change in
	// progress or the error it returned; TrustBusy disables the buttons.
	TrustKnown  bool
	Trusted     bool
	TrustBusy   bool
	TrustStatus string
	TrustError  bool
	// Terminals is the portal's 终端管理 answer: trusted and temporary
	// endpoints. TerminalsKnown distinguishes "not loaded yet" from both lists
	// being empty.
	Terminals      *session.Terminals
	TerminalsKnown bool
	// Methods are the gateway's offered authentication methods, filled by 获取
	// 认证方式. MethodsFrom is the gateway they belong to, so a reload or a
	// profile switch only drops them when the target really changed.
	Methods     []session.AuthMethod
	MethodsFrom string
	Discovering bool
	// ActiveAtOpen is the profile the saved settings point at. It differs from
	// the edited one after a dropdown choice, which is what the save hint
	// reports.
	ActiveAtOpen string
	// LocaleIndex is the language picker's entry: 0 follows the desktop, 1 is
	// Chinese, 2 is English. The application fills it from the choice mygo
	// holds, so the picker shows the language in effect.
	LocaleIndex int

	// removedProfiles are the profiles deleted from the draft since it was
	// loaded. Their stored client data is removed after a successful save, not
	// before: closing the window without saving must leave the account alone.
	removedProfiles []config.Profile
	// renamed is the selected profile's pending name change, if any. Apply
	// reports it so the caller can move the client data file once the settings
	// file names the new profile.
	renamed *ClientDataChange
}

// ClientDataChange is one stored account file that follows the draft. To is the
// profile's new name and settings; an empty To.Name means the profile was
// deleted. Deletions come before a rename in the list, because a rename may
// take the name of a profile deleted in the same save.
type ClientDataChange struct {
	From config.Profile
	To   config.Profile
}

// ClientDataChanges lists the account files the draft moves or removes. It is
// meaningful after Apply: the caller writes the settings file first and only
// then applies these, in order.
func (s *SettingsState) ClientDataChanges() []ClientDataChange {
	changes := make([]ClientDataChange, 0, len(s.removedProfiles)+1)
	for _, profile := range s.removedProfiles {
		changes = append(changes, ClientDataChange{From: profile})
	}
	if s.renamed != nil {
		changes = append(changes, *s.renamed)
	}
	return changes
}

// SettingsActions are the settings window's callbacks.
type SettingsActions struct {
	Save             func()
	Close            func()
	NewProfile       func()
	DuplicateProfile func()
	DeleteProfile    func()
	SelectProfile    func(int)
	// SetAutostart changes the desktop's start-at-login entry immediately.
	SetAutostart func(bool)
	// SetVerbose changes the running log level immediately.
	SetVerbose func(bool)
	OpenLogs   func()
	// DiscoverAuth asks the gateway in the draft for its authentication
	// methods; SelectAuth stores the chosen one in the draft.
	DiscoverAuth func()
	SelectAuth   func(int)
	// OpenPassword opens the voluntary password panel; SubmitPassword sends
	// the change through the live session; CancelPassword returns to the
	// sections.
	OpenPassword   func()
	SubmitPassword func()
	CancelPassword func()
	// RefreshTerminals reads both endpoint lists through the live session;
	// SetTrusted trusts or untrusts this device.
	RefreshTerminals func()
	SetTrusted       func(bool)
	// OpenConfig opens the settings file so the [identity] section can be
	// edited by hand; the window never writes it.
	OpenConfig func()
	// SetLocale shows the app in the tag's language, "" to follow the desktop.
	SetLocale func(string)
}

// NewSettingsState copies cfg into an editable draft and selects the active
// profile.
func NewSettingsState(cfg config.Config, logDir string) *SettingsState {
	state := &SettingsState{Draft: cfg.Clone(), LogDir: logDir, ActiveAtOpen: cfg.ActiveProfile}
	state.Selected = state.Draft.ActiveIndex()
	if state.Selected < 0 {
		state.Selected = 0
	}
	state.refresh()
	if len(state.Draft.Profiles) > 0 {
		state.NameText = state.Draft.Profiles[state.Selected].Name
		state.SelectText = state.NameText
	}
	return state
}

// refresh keeps the parallel buffers the same length as the profile list.
func (s *SettingsState) refresh() {
	if len(s.Draft.Profiles) == 0 {
		s.Draft.Profiles = append(s.Draft.Profiles, config.NewProfile(config.DefaultProfileName))
	}
	if s.Selected < 0 || s.Selected >= len(s.Draft.Profiles) {
		s.Selected = 0
	}
	for len(s.Ports) < len(s.Draft.Profiles) {
		profile := s.Draft.Profiles[len(s.Ports)]
		s.Ports = append(s.Ports, strconv.Itoa(profile.Server.Port))
		s.Targets = append(s.Targets, strings.Join(profile.Keep.Targets, "\n"))
	}
	s.Ports = s.Ports[:len(s.Draft.Profiles)]
	s.Targets = s.Targets[:len(s.Draft.Profiles)]
}

// selection returns the index and the profile the editor shows, or nil after
// refresh when nothing is selectable. A frame reads it again after a profile
// button ran, because 新建/复制/删除 change the list from inside that frame.
func (s *SettingsState) selection() (int, *config.Profile) {
	s.refresh()
	if s.Selected < 0 || s.Selected >= len(s.Draft.Profiles) {
		return -1, nil
	}
	return s.Selected, &s.Draft.Profiles[s.Selected]
}

// CanChangePassword reports whether the editor's profile owns the live session:
// the portal changes the password of the account it is logged in with, not of
// whichever profile the draft shows.
func (s *SettingsState) CanChangePassword() bool {
	if !s.Online {
		return false
	}
	profile := s.SelectedProfile()
	return profile != nil && profile.Name == s.Draft.ActiveProfile
}

// SelectedProfile returns the profile the editor shows, or nil when nothing is
// selectable.
func (s *SettingsState) SelectedProfile() *config.Profile {
	_, profile := s.selection()
	return profile
}

// Reload resets the draft from cfg, after a save or a tray-driven profile
// switch.
func (s *SettingsState) Reload(cfg config.Config) {
	s.Draft = cfg.Clone()
	s.ActiveAtOpen = cfg.ActiveProfile
	s.Discovering = false
	s.removedProfiles = nil
	s.renamed = nil
	s.Selected = s.Draft.ActiveIndex()
	if s.Selected < 0 {
		s.Selected = 0
	}
	s.Ports = nil
	s.Targets = nil
	s.refresh()
	if len(s.Draft.Profiles) > 0 {
		s.NameText = s.Draft.Profiles[s.Selected].Name
		s.SelectText = s.NameText
	}
	s.dropStaleMethods()
}

// Select edits the profile at index. It reports whether the selection changed;
// the name fields are refreshed even when it did not, because the selected
// profile may be a different one after a delete.
func (s *SettingsState) Select(index int) bool {
	if index < 0 || index >= len(s.Draft.Profiles) {
		return false
	}
	changed := index != s.Selected
	s.Selected = index
	s.NameText = s.Draft.Profiles[index].Name
	s.SelectText = s.NameText
	// The name field now shows another profile's name, so any rename typed for
	// the previous selection is gone.
	s.renamed = nil
	if changed {
		s.dropStaleMethods()
	}
	return changed
}

// SetMethods stores the result of a discovery. target is the gateway the list
// was read from; the caller fills Status with the outcome, and failed marks a
// discovery that did not produce a list so the window paints it as an error.
func (s *SettingsState) SetMethods(methods []session.AuthMethod, target, status string, failed bool) {
	s.Methods = methods
	s.MethodsFrom = target
	s.Discovering = false
	s.Status = status
	s.StatusError = failed
}

// dropStaleMethods forgets a list that belongs to another gateway.
func (s *SettingsState) dropStaleMethods() {
	if s.MethodsFrom == "" {
		s.Methods = nil
		return
	}
	if s.MethodsFrom != s.targetKey() {
		s.Methods = nil
		s.MethodsFrom = ""
	}
}

// targetKey names the gateway the draft points at, port buffer included. Two
// gateways at different ports are different targets even at one address.
func (s *SettingsState) targetKey() string {
	s.refresh()
	if s.Selected < 0 || s.Selected >= len(s.Draft.Profiles) {
		return ""
	}
	profile := s.Draft.Profiles[s.Selected]
	return targetKey(profile.ServerAddress(), s.Ports[s.Selected])
}

// targetKey is the key discovery records and the draft checks: the address
// without surrounding spaces and the port as a number, so " 0443 " names the
// same gateway as "443". main.keyForTarget writes the same form.
func targetKey(address, port string) string {
	address = strings.TrimSpace(address)
	port = strings.TrimSpace(port)
	if number, err := strconv.Atoi(port); err == nil {
		port = strconv.Itoa(number)
	}
	return address + "|" + port
}

// SelectAuth stores the method at index in the draft: 0 is automatic and the
// indexes after it are the discovered methods. The trailing "saved but not
// offered" entry changes nothing.
func (s *SettingsState) SelectAuth(index int) {
	s.refresh()
	// The caller renders the picker from the list, but an address typed earlier
	// in the same frame can already belong to another gateway.
	s.dropStaleMethods()
	profile := &s.Draft.Profiles[s.Selected]
	if index == 0 {
		profile.Auth.AuthType = ""
		s.Status = locales.AuthAuto()
		s.StatusError = false
		return
	}
	if index-1 >= len(s.Methods) || index < 0 {
		return
	}
	method := s.Methods[index-1]
	profile.Auth.AuthType = method.Type
	profile.Auth.Domain = method.Domain
	s.Status = locales.AuthChosen(MethodLabel(method))
	s.StatusError = false
}

// Target is the gateway the editor points at, with the port from its text
// field. Discovery has to use the draft: the user is usually editing a new
// gateway when asking what it offers.
func (s *SettingsState) Target() (config.Profile, error) {
	s.refresh()
	profile := s.Draft.Profiles[s.Selected]
	port, err := strconv.Atoi(strings.TrimSpace(s.Ports[s.Selected]))
	if err != nil || port < 1 || port > 65535 {
		return profile, errors.New(locales.ErrPort())
	}
	profile.Server.Port = port
	profile.Server.Address = strings.TrimSpace(profile.Server.Address)
	if profile.ServerAddress() == "" {
		return profile, errors.New(locales.ErrAddress())
	}
	return profile, nil
}

// AddProfile appends a blank profile and selects it.
func (s *SettingsState) AddProfile() {
	name := locales.ProfileBaseName()
	for suffix := 2; s.Draft.ProfileIndex(name) >= 0; suffix++ {
		name = locales.ProfileBaseNameN(suffix)
	}
	if _, err := s.Draft.AddProfile(name); err != nil {
		s.Status = err.Error()
		s.StatusError = true
		return
	}
	s.refresh()
	s.Select(s.Draft.ProfileIndex(name))
	s.Status = locales.ProfileCreated(name)
	s.StatusError = false
}

// DuplicateProfile copies the selected profile.
func (s *SettingsState) DuplicateProfile() {
	s.refresh()
	index := s.Selected
	source := s.Draft.Profiles[index]
	// The copy shows what the editor shows; the draft's stored port and
	// targets can be older than the text fields. DuplicateProfile appends it,
	// so its buffers are written at the end after refresh.
	port, targets := s.Ports[index], s.Targets[index]
	name := locales.ProfileCopyName(source.Name)
	for suffix := 2; s.Draft.ProfileIndex(name) >= 0; suffix++ {
		name = locales.ProfileCopyNameN(source.Name, suffix)
	}
	if _, err := s.Draft.DuplicateProfile(index, name); err != nil {
		s.Status = err.Error()
		s.StatusError = true
		return
	}
	s.refresh()
	copied := s.Draft.ProfileIndex(name)
	if copied >= 0 && copied < len(s.Ports) {
		s.Ports[copied] = port
		s.Targets[copied] = targets
	}
	s.Select(copied)
	s.Status = locales.ProfileCopied(name)
	s.StatusError = false
}

// DeleteProfile removes the selected profile, unless it is the last one.
func (s *SettingsState) DeleteProfile() {
	s.refresh()
	index := s.Selected
	removed := s.Draft.Profiles[index]
	if err := s.Draft.DeleteProfile(index); err != nil {
		s.Status = err.Error()
		s.StatusError = true
		return
	}
	// The stored material is removed only after a save; until then the draft is
	// just an edit.
	s.removedProfiles = append(s.removedProfiles, removed)
	s.renamed = nil
	// The buffers are positional: truncating them would hand the deleted
	// profile's port and targets to the profile that moved up.
	s.Ports = append(s.Ports[:index], s.Ports[index+1:]...)
	s.Targets = append(s.Targets[:index], s.Targets[index+1:]...)
	s.refresh()
	active := s.Draft.ActiveIndex()
	if active < 0 {
		active = 0
	}
	s.Select(active)
	s.Status = locales.ProfileDeleted()
	s.StatusError = false
}

// Apply folds the buffers into a configuration ready to validate and save. It
// deliberately allows an incomplete account: the first run has to be able to
// store the proxy ports before the user knows the gateway address. Login
// validates the rest.
func (s *SettingsState) Apply() (config.Config, error) {
	s.refresh()
	cfg := s.Draft.Clone()
	index := s.Selected
	if index < 0 || index >= len(cfg.Profiles) {
		return cfg, errors.New(locales.ErrNoProfiles())
	}

	name := strings.TrimSpace(s.NameText)
	if name == "" {
		return cfg, errors.New(locales.ErrProfileNameEmpty())
	}
	if existing := cfg.ProfileIndex(name); existing >= 0 && existing != index {
		return cfg, errors.New(locales.ErrProfileNameExists(name))
	}

	port, err := strconv.Atoi(strings.TrimSpace(s.Ports[index]))
	if err != nil || port < 1 || port > 65535 {
		return cfg, errors.New(locales.ErrPort())
	}

	interval := strings.TrimSpace(cfg.Profiles[index].Keep.Interval)
	if interval == "" {
		interval = "90s"
	}
	duration, err := time.ParseDuration(interval)
	if err != nil || duration <= 0 {
		return cfg, errors.New(locales.ErrKeepAliveInvalid())
	}
	if duration < config.MinimumKeepAliveInterval {
		return cfg, errors.New(locales.ErrKeepAliveShort(config.MinimumKeepAliveInterval.String()))
	}

	profile := &cfg.Profiles[index]
	previousName := profile.Name
	profile.Name = name
	profile.Server.Address = strings.TrimSpace(profile.Server.Address)
	profile.Server.Port = port
	profile.Auth.Username = strings.TrimSpace(profile.Auth.Username)
	profile.Auth.Domain = strings.TrimSpace(profile.Auth.Domain)
	profile.Auth.AuthType = strings.TrimSpace(profile.Auth.AuthType)
	profile.Keep.Interval = interval
	profile.Keep.Targets = splitTargets(s.Targets[index])

	cfg.Proxy.SOCKS5Bind = strings.TrimSpace(cfg.Proxy.SOCKS5Bind)
	cfg.Proxy.HTTPBind = strings.TrimSpace(cfg.Proxy.HTTPBind)
	socks5, httpBind := cfg.ProxyAddresses()
	if socks5 == "" && httpBind == "" {
		return cfg, errors.New(locales.ErrProxyBoth())
	}
	// Check the binds here as well: the same rule runs before a login, but a
	// typo should not wait for a successful connection to show up.
	for _, bind := range []string{socks5, httpBind} {
		if bind == "" {
			continue
		}
		if err := config.ValidateBind(bind); err != nil {
			return cfg, errors.New(locales.ErrBind(bind, err.Error()))
		}
	}

	cfg.ActiveProfile = name
	cfg.Version = config.CurrentVersion

	// A renamed profile's client data file has to follow it. The saved draft
	// names the new profile already, so this is safe to report here; the caller
	// applies it only after the settings file is written.
	s.renamed = nil
	if previousName != name {
		s.renamed = &ClientDataChange{From: s.Draft.Profiles[index], To: cfg.Profiles[index]}
	}
	return cfg, nil
}

// splitTargets parses the keep-alive text area: one target per line, and
// commas accepted because the v1 setting used them.
func splitTargets(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ','
	})
	var targets []string
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		target := strings.TrimSpace(field)
		if target == "" || seen[target] {
			continue
		}
		seen[target] = true
		targets = append(targets, target)
	}
	return targets
}

// SettingsView renders the settings window. The state is edited in place, so
// the caller keeps one SettingsState for the window's lifetime.
func SettingsView(state *SettingsState, actions SettingsActions) func(c *ui.Context) {
	return func(c *ui.Context) {
		state.refresh()
		if state.Password != nil {
			ui.Column(c).Fill().Padding(windowPadding).Gap(sectionGap).Children(func() {
				ui.Scroll(c).Grow(1).Children(func() {
					settingsPasswordPanel(c, state, actions)
				})
				ui.Divider(c)
				ui.Row(c).AlignItems(ui.Center).Justify(ui.End).Gap(rowGap).Children(func() {
					if ui.Button(c, locales.CommonClose()).Disabled(state.PasswordBusy).Clicked() && actions.Close != nil {
						actions.Close()
					}
				})
			})
			return
		}
		// A discovered list belongs to the gateway it was read from, and the
		// gateway is edited in this window. Drop it as soon as the address or
		// the port names something else, before the picker can store another
		// gateway's method.
		state.dropStaleMethods()
		index, profile := state.selection()
		if profile == nil {
			ui.Text(c, locales.ErrNoProfiles())
			return
		}
		names := state.Draft.ProfileNames()
		active := state.ActiveAtOpen
		enabled := profile.Keep.IsEnabled()

		ui.Column(c).Fill().Padding(windowPadding).Gap(sectionGap).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				windowHeading(c, locales.SettingsTitle())
				caption(c, locales.SettingsHint())
				if active != "" && active != profile.Name {
					captionTone(c, locales.SettingsSwitchHint(profile.Name), toneWarning)
				}
			})

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(sectionGap).Children(func() {
					ui.Form(c, func() {
						profileSection(c, state, actions, names)
						// 新建/复制/删除 change the list and the selection
						// from inside this frame. Re-read them before the
						// sections that index the profile buffers, or the
						// rest of the frame works on a profile that is gone.
						index, profile = state.selection()
						if profile == nil {
							return
						}
						enabled = profile.Keep.IsEnabled()
						connectionSection(c, state, actions, profile, index)
						proxySection(c, state)
						keepAliveSection(c, state, profile, &enabled, index)
						startupSection(c, state, actions)
						// The live terminal list is the portal's own view of
						// this account, so it sits above the advanced fields.
						terminalSection(c, state, actions)
						advancedSection(c, state, profile, actions)
						languageSection(c, state, actions)
					})
				})
			})

			ui.Divider(c)
			if state.Status != "" {
				line := ui.Text(c, state.Status).FontSize(c.Theme().FontSize)
				if state.StatusError {
					line.TextColor(c.Theme().Danger)
				} else {
					line.TextColor(c.Theme().TextMuted)
				}
			}
			ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
				if ui.PrimaryButton(c, locales.CommonSave()).Clicked() && actions.Save != nil {
					actions.Save()
				}
				if ui.Button(c, locales.CommonClose()).Clicked() && actions.Close != nil {
					actions.Close()
				}
				if state.Busy {
					busyLine(c)
				}
			})
		})
	}
}

func profileSection(c *ui.Context, state *SettingsState, actions SettingsActions, names []string) {
	ui.Fieldset(c, locales.ProfileSection(), func() {
		ui.Field(c, locales.CurrentProfile(), func() {
			if ui.Select(c, &state.SelectText, names).Changed() {
				if chosen := indexOf(names, state.SelectText); chosen >= 0 && actions.SelectProfile != nil {
					actions.SelectProfile(chosen)
				}
			}
		})
		ui.Field(c, locales.NameField(), func() {
			ui.TextInput(c, &state.NameText)
		})
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, locales.ProfileNew()).Clicked() && actions.NewProfile != nil {
				actions.NewProfile()
			}
			if ui.Button(c, locales.ProfileDuplicate()).Clicked() && actions.DuplicateProfile != nil {
				actions.DuplicateProfile()
			}
			if ui.Button(c, locales.ProfileDelete()).Disabled(len(names) == 1).Clicked() && actions.DeleteProfile != nil {
				actions.DeleteProfile()
			}
		})
	})
}

func connectionSection(c *ui.Context, state *SettingsState, actions SettingsActions, profile *config.Profile, index int) {
	ui.Fieldset(c, locales.ConnectionSection(), func() {
		ui.Field(c, locales.FieldGateway(), func() {
			ui.TextInput(c, &profile.Server.Address).Placeholder("vpn.example.com")
		})
		ui.Field(c, locales.FieldPort(), func() {
			ui.TextInput(c, &state.Ports[index]).Placeholder("443")
		})
		ui.Field(c, locales.FieldUsername(), func() {
			ui.TextInput(c, &profile.Auth.Username)
		})
		ui.Field(c, locales.AuthMethod(), func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				options := methodOptions(state.Methods, profile.Auth.AuthType, profile.Auth.Domain)
				chosen := methodValue(state.Methods, profile.Auth.AuthType, profile.Auth.Domain)
				if ui.Select(c, &chosen, options).Changed() {
					if picked := indexOf(options, chosen); picked >= 0 && actions.SelectAuth != nil {
						actions.SelectAuth(picked)
					}
				}
				if ui.Button(c, locales.DiscoverAuth()).Disabled(state.Discovering).Clicked() && actions.DiscoverAuth != nil {
					actions.DiscoverAuth()
				}
			})
			caption(c, authMethodHint(state))
		})
		ui.Field(c, locales.LoginDomain(), func() {
			ui.TextInput(c, &profile.Auth.Domain).Placeholder("local")
		})
		ui.Field(c, locales.FieldPassword(), func() {
			ui.TextInput(c, &profile.Auth.Password).Password()
		})
		ui.Field(c, locales.PasswordSecurity(), func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if ui.Button(c, locales.ResetPassword()).Disabled(!state.CanChangePassword()).Clicked() && actions.OpenPassword != nil {
					actions.OpenPassword()
				}
				if !state.Online {
					caption(c, locales.ResetNeedsConnection())
				} else if !state.CanChangePassword() {
					caption(c, locales.ResetOnlyActive())
				}
			})
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &profile.Auth.StorePass, locales.StorePassword())
		})
		if profile.Auth.TOTPSecret != "" {
			ui.Field(c, locales.TotpSection(), func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					caption(c, locales.TotpStored())
					if ui.Button(c, locales.CommonRemove()).Clicked() {
						profile.Auth.TOTPSecret = ""
						state.Status = locales.TotpRemoved()
						state.StatusError = false
					}
				})
			})
		}
	})
}

func proxySection(c *ui.Context, state *SettingsState) {
	ui.Fieldset(c, locales.ProxySection(), func() {
		ui.Field(c, locales.Socks5Listen(), func() {
			ui.TextInput(c, &state.Draft.Proxy.SOCKS5Bind).Placeholder("127.0.0.1:1080")
		})
		ui.Field(c, locales.HttpListen(), func() {
			ui.TextInput(c, &state.Draft.Proxy.HTTPBind).Placeholder("127.0.0.1:1081")
		})
		caption(c, locales.ProxyHint())
	})
}

func keepAliveSection(c *ui.Context, state *SettingsState, profile *config.Profile, enabled *bool, index int) {
	ui.Fieldset(c, locales.KeepAliveSection(), func() {
		ui.Field(c, "", func() {
			if ui.Checkbox(c, enabled, locales.KeepAliveEnable()).Changed() {
				profile.Keep.SetEnabled(*enabled)
			}
		})
		ui.Field(c, locales.KeepAliveInterval(), func() {
			ui.TextInput(c, &profile.Keep.Interval).Placeholder("90s")
			caption(c, locales.KeepAliveIntervalHint())
		})
		ui.Field(c, locales.KeepAliveTargets(), func() {
			ui.TextArea(c, &state.Targets[index]).Placeholder(locales.KeepAliveTargetsPlaceholder())
			caption(c, locales.KeepAliveTargetsHint())
		})
	})
}

func startupSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	ui.Fieldset(c, locales.StartupSection(), func() {
		ui.Field(c, "", func() {
			if ui.Checkbox(c, &state.Draft.App.Autostart, locales.Autostart()).Changed() && actions.SetAutostart != nil {
				actions.SetAutostart(state.Draft.App.Autostart)
			}
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &state.Draft.App.AutoConnect, locales.AutoConnect())
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &state.Draft.App.StartHidden, locales.StartHidden())
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &state.Draft.App.AutoReconnect, locales.AutoReconnect())
		})
	})
}

func advancedSection(c *ui.Context, state *SettingsState, profile *config.Profile, actions SettingsActions) {
	ui.Fieldset(c, locales.AdvancedSection(), func() {
		ui.Field(c, "", func() {
			if ui.Checkbox(c, &state.Draft.Logging.Verbose, locales.VerboseLog()).Changed() && actions.SetVerbose != nil {
				actions.SetVerbose(state.Draft.Logging.Verbose)
			}
		})
		ui.Field(c, locales.LogDir(), func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, state.LogDir).FontSize(c.Theme().FontSize)
				if ui.Button(c, locales.CommonOpen()).Description(locales.OpenLogsDesc()).Clicked() && actions.OpenLogs != nil {
					actions.OpenLogs()
				}
			})
		})
		ui.Field(c, locales.ClientData(), func() {
			ui.TextInput(c, &profile.Debug.ClientData).Placeholder(locales.ClientDataPlaceholder())
		})
		// The device identity is hand-edited only: a wrong id is a support
		// problem, so the window shows the values in effect and opens the file.
		ui.Field(c, locales.DeviceID(), func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				deviceID := state.Draft.Identity.DeviceID
				if deviceID == "" {
					deviceID = locales.NotGenerated()
				}
				ui.Text(c, deviceID).FontSize(c.Theme().FontSize).TextColor(c.Theme().TextMuted)
				if ui.Button(c, locales.OpenConfig()).Description(locales.OpenConfigDesc()).Clicked() && actions.OpenConfig != nil {
					actions.OpenConfig()
				}
			})
		})
		ui.Field(c, locales.ClientIdentity(), func() {
			ui.Text(c, identitySummary(state.Draft.Identity)).FontSize(c.Theme().FontSize).
				TextColor(c.Theme().TextMuted)
		})
	})
}

// languageSection switches the language of the whole app. The picker binds a
// field of the state, not a local, so the click survives into the pass that
// reports Changed.
func languageSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	ui.Fieldset(c, locales.Language(), func() {
		if ui.Segmented(c, &state.LocaleIndex,
			locales.LanguageSystem(),
			i18n.Autonym("zh"),
			i18n.Autonym("en"),
		).Changed() && actions.SetLocale != nil {
			actions.SetLocale(localeTag(state.LocaleIndex))
		}
		caption(c, locales.LanguageHint())
	})
}

// localeTag maps a picker index to the tag SetLocale takes.
func localeTag(index int) string {
	switch index {
	case 1:
		return "zh"
	case 2:
		return "en"
	default:
		return ""
	}
}

// identitySummary is the one-line description of the identity in effect: the
// values that change what the portal is told, without the long id or the UA.
func identitySummary(identity config.IdentityConfig) string {
	osName := "Linux"
	switch identity.OSFamily {
	case config.IdentityOSWindows:
		osName = "Windows"
	case config.IdentityOSUOS:
		osName = "UOS"
	}
	parts := make([]string, 0, 3)
	add := func(value string) {
		if strings.TrimSpace(value) != "" {
			parts = append(parts, value)
		}
	}
	add(osName)
	add(identity.ClientVersion)
	add(identity.ProcessName)
	return strings.Join(parts, " · ")
}

func indexOf(values []string, value string) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	return -1
}

// authAutoLabel is the first entry of the method picker: let the gateway's
// list decide.
func authAutoLabel() string { return locales.AuthAutoLabel() }

// methodOptions builds the picker's entries. A saved pair the gateway did not
// list is kept as a trailing entry so the select never shows a blank value; it
// is informational, not selectable.
func methodOptions(methods []session.AuthMethod, authType, domain string) []string {
	options := []string{authAutoLabel()}
	for _, method := range methods {
		options = append(options, MethodLabel(method))
	}
	if saved := savedMethodLabel(methods, authType, domain); saved != "" {
		options = append(options, saved)
	}
	return options
}

// methodValue is the picker's current value for a profile.
func methodValue(methods []session.AuthMethod, authType, domain string) string {
	if authType == "" {
		return authAutoLabel()
	}
	if saved := savedMethodLabel(methods, authType, domain); saved != "" {
		return saved
	}
	return MethodLabel(session.AuthMethod{Type: authType, Domain: domain})
}

// savedMethodLabel reports the entry that names a stored pair the discovered
// list does not contain, or "" when the pair is offered or nothing is stored.
func savedMethodLabel(methods []session.AuthMethod, authType, domain string) string {
	if authType == "" {
		return ""
	}
	for _, method := range methods {
		if method.Matches(authType, domain) {
			return ""
		}
	}
	return locales.SavedMethod(authType, domain)
}

// authMethodHint explains what the picker currently means for the login.
func authMethodHint(state *SettingsState) string {
	switch {
	case state.Discovering:
		return locales.MethodsDiscovering()
	case len(state.Methods) == 0:
		return locales.MethodsUnknown()
	default:
		return locales.MethodsHint(len(state.Methods))
	}
}

// terminalSection is the live 终端管理 page: the portal lists trusted
// endpoints separately from the temporary login history. The current device is
// marked by selfId, exactly as the portal does.
func terminalSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	ui.Fieldset(c, locales.TerminalsSection(), func() {
		if !state.Online {
			caption(c, locales.TerminalsOffline())
			return
		}

		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			if state.TerminalsKnown && state.Terminals != nil {
				ui.Text(c, locales.TerminalCounts(len(state.Terminals.Trusted), len(state.Terminals.Temporary))).
					FontSize(c.Theme().FontSize)
			} else {
				ui.Text(c, locales.TerminalsNotLoaded()).FontSize(c.Theme().FontSize)
			}
			if ui.Button(c, locales.CommonRefresh()).Description(locales.TerminalsRefreshDesc()).
				Disabled(state.TrustBusy).Clicked() && actions.RefreshTerminals != nil {
				actions.RefreshTerminals()
			}
			if state.TrustKnown {
				label := locales.Trust()
				if state.Trusted {
					label = locales.Untrust()
				}
				if ui.Button(c, label).Disabled(state.TrustBusy).Clicked() && actions.SetTrusted != nil {
					actions.SetTrusted(!state.Trusted)
				}
			}
		})

		if state.TerminalsKnown && state.Terminals != nil {
			terminalList(c, locales.TrustedTerminals(), state.Terminals.Trusted, state.Terminals.SelfID)
			terminalList(c, locales.TemporaryTerminals(), state.Terminals.Temporary, state.Terminals.SelfID)
		}

		if state.TrustStatus != "" {
			line := caption(c, state.TrustStatus)
			if state.TrustError {
				line.TextColor(c.Theme().Danger)
			}
		}
	})
}

func terminalList(c *ui.Context, title string, terminals []session.Terminal, selfID string) {
	if len(terminals) == 0 {
		caption(c, locales.TerminalNone(title))
		return
	}
	caption(c, locales.TerminalCount(title, len(terminals)))
	for _, terminal := range terminals {
		ui.Column(c).Gap(2).PaddingY(4).Children(func() {
			name := strings.TrimSpace(terminal.DeviceName)
			if name == "" {
				name = locales.UnknownDevice()
			}
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, name).FontSize(c.Theme().FontSize).FontWeight(600)
				ui.Text(c, terminalState(terminal, selfID)).FontSize(textSize(c, captionScale)).
					TextColor(c.Theme().TextMuted)
			})
			details := terminalDetails(terminal)
			if details != "" {
				caption(c, details)
			}
		})
	}
}

// terminalState uses selfId first, as the portal marks the current device even
// when the historical row says it is offline.
func terminalState(terminal session.Terminal, selfID string) string {
	if terminal.ID != "" && terminal.ID == selfID {
		return locales.CurrentDevice()
	}
	if terminal.OnlineStatus {
		return locales.Online()
	}
	return locales.Offline()
}

func terminalDetails(terminal session.Terminal) string {
	parts := make([]string, 0, 3)
	if device := terminalDevice(terminal); device != "" {
		parts = append(parts, device)
	}
	location := strings.TrimSpace(terminal.LastLoginAddress)
	if location == "LAN" {
		location = locales.Intranet()
	}
	if location != "" {
		if terminal.LastLoginIP != "" {
			location += "（" + terminal.LastLoginIP + "）"
		}
		parts = append(parts, location)
	}
	if terminal.LastLoginTime != "" {
		parts = append(parts, terminal.LastLoginTime)
	}
	return strings.Join(parts, " · ")
}

func terminalDevice(terminal session.Terminal) string {
	if terminal.DeviceType == "browser" {
		if terminal.Browser != "" {
			return locales.BrowserSuffix(terminal.Browser)
		}
		return locales.Browser()
	}
	if terminal.OS != "" {
		return strings.TrimSpace(terminal.OS + " " + terminal.OSVersion)
	}
	return terminal.DeviceType
}
