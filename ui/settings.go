// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/core/config"
	"github.com/YangtseSu/goTrust/core/session"
	buildversion "github.com/YangtseSu/goTrust/internal/version"
	"github.com/YangtseSu/goTrust/locales"
)

// SettingsState is the window's settings draft. It is a clone of the live
// configuration, edited by the pages for the life of the window: a page
// switch or hiding the window keeps the edits, and 保存, 放弃, a profile
// switch and a login that saves the draft reload it from the file.
//
// The text fields edit the draft directly, except the gateway and the
// keep-alive targets: the file stores an address and a port separately and the
// targets as a list, so those live in parallel slices indexed like
// Draft.Profiles.
type SettingsState struct {
	Draft    config.Config
	Selected int
	// NameText is the selected account's name, edited on 多账户. It is folded
	// into the draft when the draft is saved.
	NameText string
	// Gateways is one host:port per profile: what an administrator hands over
	// and what the field shows. ParseGateway splits it when the draft is saved.
	Gateways []string
	Targets  []string
	Status   string
	// StatusSeq increases with every SetStatus, so the window shows the same
	// text a second time as a second toast.
	StatusSeq uint64
	// StatusError marks the last status as a failure instead of a
	// confirmation.
	StatusError bool
	LogDir      string
	// LogPath is the file the 日志 page tails, empty when the app runs
	// without a log file. LogLines is the tail itself, oldest first, and the
	// gui's tailer replaces it as the file grows.
	LogPath  string
	LogLines []string
	// LogFilter narrows the visible tail to the lines containing it; the
	// empty string shows every line. LogAutoScroll keeps the pane on the
	// newest line until the user unticks it.
	LogFilter     string
	LogAutoScroll bool
	// logSeen is how many lines the last frame showed: the pane follows the
	// newest line only when the count changed.
	logSeen int
	// ClashFormat is the 代理 page's copy picker: which Clash/Mihomo rendering
	// 复制 writes to the clipboard.
	ClashFormat ClashFormat
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

	// baseline is the saved configuration the draft is compared against, so
	// the window can say that a save would change the file.
	baseline config.Config
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

// SettingsActions are the draft's callbacks.
type SettingsActions struct {
	Save func()
	// Discard throws the draft away and shows the saved settings again.
	Discard          func()
	NewProfile       func()
	DuplicateProfile func()
	DeleteProfile    func()
	SelectProfile    func(int)
	// SetAutostart changes the desktop's start-at-login entry immediately.
	SetAutostart func(bool)
	// SetVerbose changes the running log level immediately.
	SetVerbose func(bool)
	// ClearLogs truncates the log file and removes its rotated copies.
	ClearLogs func()
	OpenLogs  func()
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
	// CopyLogs puts the visible log lines on the clipboard.
	CopyLogs func(string)
	// OpenURL opens an http(s) page in the browser, for the 关于 links.
	OpenURL func(string)
	// SetLocale shows the app in the tag's language, "" to follow the desktop.
	SetLocale func(string)
}

// NewSettingsState copies cfg into an editable draft and selects the active
// profile.
func NewSettingsState(cfg config.Config, logDir string) *SettingsState {
	state := &SettingsState{
		Draft:         cfg.Clone(),
		LogDir:        logDir,
		ActiveAtOpen:  cfg.ActiveProfile,
		LogAutoScroll: true,
		ClashFormat:   ClashPlain,
	}
	state.baseline = cfg.Clone()
	state.Selected = state.Draft.ActiveIndex()
	if state.Selected < 0 {
		state.Selected = 0
	}
	state.refresh()
	if len(state.Draft.Profiles) > 0 {
		state.NameText = state.Draft.Profiles[state.Selected].Name
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
	for len(s.Gateways) < len(s.Draft.Profiles) {
		profile := s.Draft.Profiles[len(s.Gateways)]
		s.Gateways = append(s.Gateways, gatewayText(profile))
		s.Targets = append(s.Targets, strings.Join(profile.Keep.Targets, "\n"))
	}
	s.Gateways = s.Gateways[:len(s.Draft.Profiles)]
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
	s.baseline = cfg.Clone()
	s.ActiveAtOpen = cfg.ActiveProfile
	s.Discovering = false
	s.removedProfiles = nil
	s.renamed = nil
	s.Selected = s.Draft.ActiveIndex()
	if s.Selected < 0 {
		s.Selected = 0
	}
	s.Gateways = nil
	s.Targets = nil
	s.refresh()
	if len(s.Draft.Profiles) > 0 {
		s.NameText = s.Draft.Profiles[s.Selected].Name
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
	s.SetStatus(status, failed)
}

// SetStatus records the result of an action for the window's toast. The
// sequence makes the same text a second result rather than a silent no-op.
func (s *SettingsState) SetStatus(text string, failed bool) {
	s.Status = text
	s.StatusError = failed
	s.StatusSeq++
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

// targetKey names the gateway the draft points at, parsed from its text field:
// the same key gui.keyForTarget derives from a saved profile.
func (s *SettingsState) targetKey() string {
	s.refresh()
	if s.Selected < 0 || s.Selected >= len(s.Draft.Profiles) {
		return ""
	}
	address, port, problem := ParseGateway(s.Gateways[s.Selected])
	if problem != "" {
		return ""
	}
	return address + "|" + strconv.Itoa(port)
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
		s.SetStatus(locales.AuthAuto(), false)
		return
	}
	if index-1 >= len(s.Methods) || index < 0 {
		return
	}
	method := s.Methods[index-1]
	profile.Auth.AuthType = method.Type
	profile.Auth.Domain = method.Domain
	s.SetStatus(locales.AuthChosen(MethodLabel(method)), false)
}

// Target is the gateway the editor points at, parsed from its text field.
// Discovery has to use the draft: the user is usually editing a new gateway
// when asking what it offers.
func (s *SettingsState) Target() (config.Profile, error) {
	index, profile := s.selection()
	if profile == nil {
		return config.Profile{}, errors.New(locales.ErrNoProfiles())
	}
	address, port, problem := ParseGateway(s.Gateways[index])
	if problem != "" {
		return *profile, errors.New(problem)
	}
	if address == "" {
		return *profile, errors.New(locales.ProblemAddress())
	}
	target := *profile
	target.Server.Address = address
	target.Server.Port = port
	return target, nil
}

// AddProfile appends a blank profile and selects it.
func (s *SettingsState) AddProfile() {
	name := locales.ProfileBaseName()
	for suffix := 2; s.Draft.ProfileIndex(name) >= 0; suffix++ {
		name = locales.ProfileBaseNameN(suffix)
	}
	if _, err := s.Draft.AddProfile(name); err != nil {
		s.SetStatus(err.Error(), true)
		return
	}
	s.refresh()
	s.Select(s.Draft.ProfileIndex(name))
	s.SetStatus(locales.ProfileCreated(name), false)
}

// DuplicateProfile copies the selected profile.
func (s *SettingsState) DuplicateProfile() {
	s.refresh()
	index := s.Selected
	source := s.Draft.Profiles[index]
	// The copy shows what the editor shows; the draft's stored gateway and
	// targets can be older than the text fields. DuplicateProfile appends it,
	// so its buffers are written at the end after refresh.
	gateway, targets := s.Gateways[index], s.Targets[index]
	name := locales.ProfileCopyName(source.Name)
	for suffix := 2; s.Draft.ProfileIndex(name) >= 0; suffix++ {
		name = locales.ProfileCopyNameN(source.Name, suffix)
	}
	if _, err := s.Draft.DuplicateProfile(index, name); err != nil {
		s.SetStatus(err.Error(), true)
		return
	}
	s.refresh()
	copied := s.Draft.ProfileIndex(name)
	if copied >= 0 && copied < len(s.Gateways) {
		s.Gateways[copied] = gateway
		s.Targets[copied] = targets
	}
	s.Select(copied)
	s.SetStatus(locales.ProfileCopied(name), false)
}

// DeleteProfile removes the selected profile, unless it is the last one.
func (s *SettingsState) DeleteProfile() {
	s.refresh()
	index := s.Selected
	removed := s.Draft.Profiles[index]
	if err := s.Draft.DeleteProfile(index); err != nil {
		s.SetStatus(err.Error(), true)
		return
	}
	// The stored material is removed only after a save; until then the draft is
	// just an edit.
	s.removedProfiles = append(s.removedProfiles, removed)
	s.renamed = nil
	// The buffers are positional: truncating them would hand the deleted
	// profile's gateway and targets to the profile that moved up.
	s.Gateways = append(s.Gateways[:index], s.Gateways[index+1:]...)
	s.Targets = append(s.Targets[:index], s.Targets[index+1:]...)
	s.refresh()
	active := s.Draft.ActiveIndex()
	if active < 0 {
		active = 0
	}
	s.Select(active)
	s.SetStatus(locales.ProfileDeleted(), false)
}

// Apply folds the buffers into a configuration ready to validate and save. It
// deliberately allows an incomplete account: a fresh install has to be able to
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

	// Every profile's gateway is folded, not just the selected one: the user
	// edits one profile at a time, and a switch between two edits must not
	// throw the first one away.
	for profileIndex := range cfg.Profiles {
		address, port, problem := ParseGateway(s.Gateways[profileIndex])
		if problem != "" {
			return cfg, errors.New(problem)
		}
		cfg.Profiles[profileIndex].Server.Address = address
		if port > 0 {
			cfg.Profiles[profileIndex].Server.Port = port
		}
	}

	interval := strings.TrimSpace(cfg.Profiles[index].Keep.Interval)
	if interval == "" {
		interval = "90s"
	}
	duration, err := time.ParseDuration(interval)
	if err != nil || duration <= 0 {
		return cfg, errors.New(locales.ErrKeepAliveInvalid(config.MinimumKeepAliveInterval.String()))
	}
	if duration < config.MinimumKeepAliveInterval {
		return cfg, errors.New(locales.ErrKeepAliveShort(config.MinimumKeepAliveInterval.String()))
	}

	profile := &cfg.Profiles[index]
	previousName := profile.Name
	profile.Name = name
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

// folded returns what Apply would save, without the checks: the window marks
// unsaved edits even while one of them is still invalid, so the fields are
// folded the same way Apply folds them and nothing is rejected here.
func (s *SettingsState) folded() config.Config {
	s.refresh()
	cfg := s.Draft.Clone()
	index := s.Selected
	if index < 0 || index >= len(cfg.Profiles) {
		return cfg
	}
	profile := &cfg.Profiles[index]
	// The name is folded even when it is blank: an empty name is an edit, and
	// the window has to offer 保存 so the validation error can explain it.
	profile.Name = strings.TrimSpace(s.NameText)
	for profileIndex := range cfg.Profiles {
		address, port, problem := ParseGateway(s.Gateways[profileIndex])
		if problem != "" {
			continue
		}
		cfg.Profiles[profileIndex].Server.Address = address
		if port > 0 {
			cfg.Profiles[profileIndex].Server.Port = port
		}
	}
	profile.Auth.Username = strings.TrimSpace(profile.Auth.Username)
	profile.Auth.Domain = strings.TrimSpace(profile.Auth.Domain)
	profile.Auth.AuthType = strings.TrimSpace(profile.Auth.AuthType)
	profile.Keep.Interval = strings.TrimSpace(profile.Keep.Interval)
	profile.Keep.Targets = splitTargets(s.Targets[index])
	cfg.Proxy.SOCKS5Bind = strings.TrimSpace(cfg.Proxy.SOCKS5Bind)
	cfg.Proxy.HTTPBind = strings.TrimSpace(cfg.Proxy.HTTPBind)
	cfg.ActiveProfile = profile.Name
	cfg.Version = config.CurrentVersion
	return cfg
}

// Changed reports whether a save would change the settings file.
func (s *SettingsState) Changed() bool {
	if len(s.Draft.Profiles) == 0 {
		return false
	}
	if !reflect.DeepEqual(s.baseline, s.folded()) {
		return true
	}
	// A gateway field that does not parse is still an edit: the save bar has
	// to offer 保存 so Apply can say what is wrong with it.
	for index := range s.Draft.Profiles {
		if _, _, problem := ParseGateway(s.Gateways[index]); problem != "" {
			return true
		}
	}
	return false
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

// fieldWidth caps a form control: a value does not need the window's whole
// width, and an input stretched across it reads as a text area. The label
// column keeps the controls aligned, so the cap is one width for every field.
const fieldWidth = 360

// methodPickerWidth leaves room for 获取认证方式, which shares the row with the
// picker instead of sitting under the field.
const methodPickerWidth = 240

func connectionSection(c *ui.Context, state *SettingsState, actions SettingsActions, profile *config.Profile, index int) {
	formColumn(c, func() {
		formField(c, locales.FieldGateway(), func() {
			ui.TextInput(c, &state.Gateways[index]).Placeholder("vpn.example.com:443")
		})
		formField(c, locales.FieldUsername(), func() {
			ui.TextInput(c, &profile.Auth.Username)
		})
		// The password and 保存密码 are one row: the check box belongs to the
		// value beside it, and the hint about a gateway that asks later is the
		// placeholder it cannot be if it is a caption under the field.
		formField(c, locales.FieldPassword(), func() {
			ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
				ui.TextInput(c, &profile.Auth.Password).Password().Grow(1).Shrink(1).
					Placeholder(locales.PasswordPlaceholder())
				ui.Checkbox(c, &profile.Auth.StorePass, locales.StorePassword())
			})
		})
		formField(c, locales.AuthMethod(), func() {
			ui.Row(c).Wrap().AlignItems(ui.Center).Gap(8).Children(func() {
				options := methodOptions(state.Methods, profile.Auth.AuthType, profile.Auth.Domain)
				chosen := methodValue(state.Methods, profile.Auth.AuthType, profile.Auth.Domain)
				if ui.Select(c, &chosen, options).Grow(1).Changed() {
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
		formField(c, locales.LoginDomain(), func() {
			ui.TextInput(c, &profile.Auth.Domain).Placeholder("local")
		})
	})
}

// securitySection is the account's 安全 card: the voluntary password change
// and, when the login stored one, the TOTP secret.
func securitySection(c *ui.Context, state *SettingsState, actions Actions, profile *config.Profile) {
	theme := c.Theme()
	ui.Column(c).Gap(rowGap).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
				ui.Text(c, locales.ChangePassword()).FontSize(theme.FontSize)
				sub := locales.ChangePasswordDesc()
				if !state.Online {
					sub = locales.ResetNeedsConnection()
				} else if !state.CanChangePassword() {
					sub = locales.ResetOnlyActive()
				}
				caption(c, sub)
			})
			if ui.Button(c, locales.EditPassword()).Disabled(!state.CanChangePassword()).
				Clicked() && actions.Settings.OpenPassword != nil {
				actions.Settings.OpenPassword()
			}
		})
		if profile.Auth.TOTPSecret != "" {
			ui.Divider(c)
			ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
				ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
					ui.Text(c, locales.TotpSection()).FontSize(theme.FontSize)
					caption(c, locales.TotpStored())
				})
				if ui.Button(c, locales.CommonRemove()).Clicked() {
					profile.Auth.TOTPSecret = ""
					state.SetStatus(locales.TotpRemoved(), false)
				}
			})
		}
	})
}

func proxySection(c *ui.Context, state *SettingsState) {
	caption(c, locales.SettingsEffectConnect())
	formColumn(c, func() {
		formField(c, locales.Socks5Listen(), func() {
			ui.TextInput(c, &state.Draft.Proxy.SOCKS5Bind).Placeholder("127.0.0.1:1080")
		})
		formField(c, locales.HttpListen(), func() {
			ui.TextInput(c, &state.Draft.Proxy.HTTPBind).Placeholder("127.0.0.1:1081")
		})
		caption(c, locales.ProxyHint())
	})
}

func keepAliveSection(c *ui.Context, state *SettingsState, profile *config.Profile, enabled *bool, index int) {
	card(c).Gap(rowGap).Children(func() {
		if switchRow(c, enabled, locales.KeepAliveEnable()) {
			profile.Keep.SetEnabled(*enabled)
		}
		ui.Divider(c)
		formColumn(c, func() {
			formField(c, locales.KeepAliveInterval(), func() {
				ui.TextInput(c, &profile.Keep.Interval).Placeholder("90s")
			})
			formField(c, locales.KeepAliveTargets(), func() {
				ui.TextArea(c, &state.Targets[index]).Placeholder(locales.KeepAliveTargetsPlaceholder())
			})
		})
	})
}

func startupSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	card(c).Gap(rowGap).Children(func() {
		if switchRow(c, &state.Draft.App.Autostart, locales.Autostart()) && actions.SetAutostart != nil {
			actions.SetAutostart(state.Draft.App.Autostart)
		}
		ui.Divider(c)
		switchRow(c, &state.Draft.App.AutoConnect, locales.AutoConnect())
		ui.Divider(c)
		switchRow(c, &state.Draft.App.StartHidden, locales.StartHidden())
		ui.Divider(c)
		switchRow(c, &state.Draft.App.AutoReconnect, locales.AutoReconnect())
	})
}

func localDataSection(c *ui.Context, state *SettingsState, profile *config.Profile, actions SettingsActions) {
	card(c).Gap(rowGap).Children(func() {
		sectionHeading(c, locales.LocalData())
		formColumn(c, func() {
			formField(c, locales.ClientData(), func() {
				input := ui.TextInput(c, &profile.Debug.ClientData)
				if resolved, err := state.Draft.ClientDataPathFor(*profile); err == nil {
					input.Placeholder(shortPath(resolved))
				}
			})
		})
		ui.Divider(c)
		deviceID := state.Draft.Identity.DeviceID
		if deviceID == "" {
			deviceID = locales.NotGenerated()
		}
		rowLine(c, locales.DeviceID(), deviceID, func() {
			if ui.Button(c, locales.OpenConfig()).Description(locales.OpenConfigDesc()).Clicked() && actions.OpenConfig != nil {
				actions.OpenConfig()
			}
		})
	})
}

func aboutPage(c *ui.Context, actions SettingsActions) {
	theme := c.Theme()
	p := paletteOf(c)
	pageScroll(c, func() {
		card(c).Gap(rowGap).Children(func() {
			ui.Column(c).AlignItems(ui.Center).Gap(theme.Space(2.5)).
				PaddingY(theme.Space(14)).Children(func() {
				ui.Icon(c, iconBrand).Size(72, 72).TextColor(p.accent)
				ui.Text(c, "goTrust "+releaseVersion()).FontSize(theme.FontSize + 3).
					FontWeight(600).TextColor(theme.Text)
				ui.Text(c, locales.AboutTagline()).FontSize(theme.FontSize).
					TextColor(p.muted)
			})
			ui.Divider(c)
			linkRow(c, locales.ProjectHome(), projectHost, func() {
				if actions.OpenURL != nil {
					actions.OpenURL(projectURL)
				}
			})
			ui.Divider(c)
			linkRow(c, locales.LicenseLabel(), licenseLine, func() {
				if actions.OpenURL != nil {
					actions.OpenURL(licenseURL)
				}
			})
		})
	})
}

// The project's public addresses. The host is what the window shows; the
// rows open the https pages.
const (
	projectHost = "github.com/YangtseSu/goTrust"
	projectURL  = "https://" + projectHost
	licenseURL  = projectURL + "/blob/main/LICENSE"
	licenseLine = "AGPL-3.0-or-later · © 2026 Yangtse Su"
)

// releaseVersion is the built release with its tag prefix, or the source
// build's own word.
func releaseVersion() string {
	value := strings.TrimSpace(buildversion.Version)
	switch {
	case value == "":
		return "devel"
	case value == "devel" || strings.HasPrefix(value, "v"):
		return value
	default:
		return "v" + value
	}
}

// shortPath replaces the user's home directory with "~", so a long default
// path reads as the one in the documentation.
func shortPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// languageSection switches the language of the whole app. The picker binds a
// field of the state, not a local, so the click survives into the pass that
// reports Changed.
func languageSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	card(c).Gap(rowGap).Children(func() {
		sectionHeading(c, locales.Language())
		if ui.Segmented(c, &state.LocaleIndex,
			locales.LanguageSystem(),
			i18n.Autonym("zh"),
			i18n.Autonym("en"),
		).AlignSelf(ui.Start).Changed() && actions.SetLocale != nil {
			actions.SetLocale(localeTag(state.LocaleIndex))
		}
	})
}

// rowLine is one titled row of a card: the title and an optional sub line on
// the left, the value or the action on the right.
func rowLine(c *ui.Context, title, sub string, right func()) {
	theme := c.Theme()
	ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(theme.FontSize)
			if sub != "" {
				caption(c, sub).SingleLine()
			}
		})
		if right != nil {
			right()
		}
	})
}

// linkRow is a row that opens a page in the browser. The whole row is the
// target: the mockup has no separate 打开 button.
func linkRow(c *ui.Context, title, sub string, open func()) {
	theme := c.Theme()
	p := paletteOf(c)
	row := ui.ButtonBase(c).AlignItems(ui.Center).Padding(theme.Space(1.5), 0).
		Radius(theme.Radius).Gap(rowGap).Label(title)
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		ui.Column(c).Grow(1).Shrink(1).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(theme.FontSize)
			if sub != "" {
				caption(c, sub).SingleLine()
			}
		})
		ui.Icon(c, iconArrowUpRight).Size(16, 16).Shrink(0).TextColor(p.muted)
	})
	if row.Clicked() && open != nil {
		open()
	}
}

// switchLabel is a switch beside its label, as the log page's 详细日志 and
// the mockup's toolbar draw it. The whole label is the target.
func switchLabel(c *ui.Context, on *bool, label string) bool {
	theme := c.Theme()
	p := paletteOf(c)
	row := ui.ButtonBase(c).AlignItems(ui.Center).Gap(theme.Space(2)).
		Padding(theme.Space(0.5), theme.Space(1)).Radius(theme.Radius).
		Role(ui.RoleSwitch).Label(label).Checked(*on)
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		switchTrack(c, *on)
		ui.Text(c, label).FontSize(theme.FontSize)
	})
	if !row.Clicked() {
		return false
	}
	*on = !*on
	return true
}

// switchRow is a titled row with a switch at its right end; the row is the
// control, as the mockup's label wraps its switch. It reports whether the
// value changed this frame.
func switchRow(c *ui.Context, on *bool, label string) bool {
	theme := c.Theme()
	p := paletteOf(c)
	row := ui.ButtonBase(c).AlignItems(ui.Center).Gap(rowGap).
		Padding(theme.Space(0.5), 0).Radius(theme.Radius).
		Role(ui.RoleSwitch).Label(label).Checked(*on)
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		ui.Text(c, label).FontSize(theme.FontSize).Grow(1)
		switchTrack(c, *on)
	})
	if !row.Clicked() {
		return false
	}
	*on = !*on
	return true
}

// switchTrack draws the mockup's switch: a 34x20 pill with a 16px knob at
// the left when off and at the right when on.
func switchTrack(c *ui.Context, on bool) {
	p := paletteOf(c)
	track := ui.Box(c).Width(34).Height(20).Radius(999).Shrink(0).Padding(2)
	if on {
		track.Background(p.accent)
	} else {
		track.Background(p.switchOff)
	}
	track.Children(func() {
		ui.Row(c).Fill().Children(func() {
			if on {
				ui.Box(c).Grow(1)
			}
			ui.Box(c).Size(16, 16).Radius(8).Background(ui.RGB(255, 255, 255))
		})
	})
}

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
// endpoints separately from the temporary login history. The summary counts
// both lists against the account's device limit, and the current device is
// marked by selfId, exactly as the portal does.
func terminalSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	theme := c.Theme()
	loaded := state.Online && state.TerminalsKnown && state.Terminals != nil
	var terminals *session.Terminals
	if loaded {
		terminals = state.Terminals
	}

	ui.Row(c).Wrap().AlignItems(ui.Center).Gap(rowGap).Children(func() {
		if loaded {
			ui.Text(c, terminalSummary(terminals)).FontSize(theme.FontSize)
		} else {
			ui.Text(c, locales.TerminalCounts(0, 0)).FontSize(theme.FontSize)
		}
		if state.TrustKnown {
			label := locales.TerminalSelfUntrusted()
			if state.Trusted {
				label = locales.TerminalSelfTrusted()
			}
			caption(c, "· "+label)
		}
		ui.Box(c).Grow(1)
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

	var trusted, temporary []session.Terminal
	var selfID string
	if loaded {
		trusted, temporary, selfID = terminals.Trusted, terminals.Temporary, terminals.SelfID
	}
	terminalList(c, locales.TrustedTerminals(), trusted, selfID, loaded)
	terminalList(c, locales.TemporaryTerminals(), temporary, selfID, loaded)

	if state.TrustStatus != "" {
		line := caption(c, state.TrustStatus)
		if state.TrustError {
			line.TextColor(toneInk(c.Theme(), toneDanger))
		}
	}
}

// terminalSummary counts the two lists, with the device limit when the portal
// reports one: trust devices are limited per account, temporary logins are not.
func terminalSummary(terminals *session.Terminals) string {
	if terminals.MaxDeviceCount > 0 {
		return locales.TerminalCountsLimit(len(terminals.Trusted), terminals.MaxDeviceCount, len(terminals.Temporary))
	}
	return locales.TerminalCounts(len(terminals.Trusted), len(terminals.Temporary))
}

// terminalFactWidth is one cell of the facts grid: three cells fit the page at
// its default width, two at the minimum.
const terminalFactWidth float32 = 180

func terminalList(c *ui.Context, title string, terminals []session.Terminal, selfID string, loaded bool) {
	card(c).Gap(c.Theme().Space(2)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Children(func() {
			sectionHeading(c, title)
			caption(c, locales.TerminalCountTitle(len(terminals)))
		})
		if len(terminals) == 0 {
			if loaded {
				caption(c, locales.TerminalNone(title))
			} else {
				caption(c, locales.TerminalsOffline())
			}
			return
		}
		for index, terminal := range terminals {
			if index > 0 {
				ui.Divider(c)
			}
			terminalRow(c, terminal, selfID)
		}
	})
}

// terminalRow is one endpoint: its name and state, then the fields the portal's
// own page shows, so a support question can be answered from this window.
func terminalRow(c *ui.Context, terminal session.Terminal, selfID string) {
	theme := c.Theme()
	name := strings.TrimSpace(terminal.DeviceName)
	if name == "" {
		name = locales.UnknownDevice()
	}
	ui.Column(c).Gap(theme.Space(1.5)).PaddingY(theme.Space(1)).Children(func() {
		ui.Row(c).Wrap().AlignItems(ui.Center).Gap(rowGap).Children(func() {
			ui.Text(c, name).FontSize(theme.FontSize).FontWeight(600)
			ui.Text(c, terminalState(terminal, selfID)).FontSize(textSize(c, captionScale)).
				TextColor(theme.TextMuted)
		})
		terminalFactsGrid(c, terminal)
	})
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

// terminalFact is one labelled field of an endpoint.
type terminalFact struct {
	label string
	value string
}

// terminalFacts are the portal's user-facing fields, in the order its own page
// reads. A value the portal does not report is a dash, never an empty cell.
func terminalFacts(terminal session.Terminal) []terminalFact {
	return []terminalFact{
		{locales.TerminalType(), terminalKind(terminal)},
		{locales.TerminalOS(), orDash(strings.TrimSpace(terminal.OS + " " + terminal.OSVersion))},
		{locales.Browser(), orDash(terminal.Browser)},
		{locales.TerminalLastLogin(), orDash(terminal.LastLoginTime)},
		{locales.TerminalLoginAddress(), terminalAddress(terminal)},
		{locales.TerminalZones(), orDash(strings.Join(terminal.NetworkZoneList, " / "))},
		{locales.DeviceID(), terminalDeviceID(terminal)},
	}
}

// terminalFactsGrid lays the fields out in wrapping cells.
func terminalFactsGrid(c *ui.Context, terminal session.Terminal) {
	ui.Row(c).Wrap().AlignItems(ui.Start).Gap(c.Theme().Space(2)).Children(func() {
		for _, fact := range terminalFacts(terminal) {
			ui.Column(c).Gap(2).Width(terminalFactWidth).Shrink(0).Children(func() {
				caption(c, fact.label)
				ui.Text(c, fact.value).FontSize(c.Theme().FontSize).MaxLines(2)
			})
		}
	})
}

// terminalKind names the portal's client-or-browser values; an unknown value is
// shown as the portal sent it.
func terminalKind(terminal session.Terminal) string {
	switch terminal.DeviceType {
	case "browser":
		return locales.Browser()
	case "":
		return "—"
	case "client":
		return locales.TerminalClient()
	default:
		return terminal.DeviceType
	}
}

// terminalAddress is the login location with the address beside it, and 内网
// for the portal's "LAN".
func terminalAddress(terminal session.Terminal) string {
	location := strings.TrimSpace(terminal.LastLoginAddress)
	if location == "LAN" {
		location = locales.Intranet()
	}
	if ip := strings.TrimSpace(terminal.LastLoginIP); ip != "" {
		if location == "" {
			return ip
		}
		return location + "（" + ip + "）"
	}
	return orDash(location)
}

// terminalDeviceID prefers the portal's device database id: it is the value its
// own page shows, and the row id is the fallback.
func terminalDeviceID(terminal session.Terminal) string {
	if id := strings.TrimSpace(terminal.DevDBID); id != "" {
		return id
	}
	return orDash(terminal.ID)
}

// orDash keeps a cell readable when the portal left the field out.
func orDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}
