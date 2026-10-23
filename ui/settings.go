// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/YangtseSu/goTrust/config"
	"github.com/YangtseSu/goTrust/session"
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

// Reload resets the draft from cfg, after a save or a tray-driven profile
// switch.
func (s *SettingsState) Reload(cfg config.Config) {
	s.Draft = cfg.Clone()
	s.ActiveAtOpen = cfg.ActiveProfile
	s.Discovering = false
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
// without surrounding spaces and the port as typed.
func targetKey(address, port string) string {
	return strings.TrimSpace(address) + "|" + strings.TrimSpace(port)
}

// SelectAuth stores the method at index in the draft: 0 is automatic and the
// indexes after it are the discovered methods. The trailing "saved but not
// offered" entry changes nothing.
func (s *SettingsState) SelectAuth(index int) {
	s.refresh()
	profile := &s.Draft.Profiles[s.Selected]
	if index == 0 {
		profile.Auth.AuthType = ""
		s.Status = "认证方式：自动"
		s.StatusError = false
		return
	}
	if index-1 >= len(s.Methods) || index < 0 {
		return
	}
	method := s.Methods[index-1]
	profile.Auth.AuthType = method.Type
	profile.Auth.Domain = method.Domain
	s.Status = "认证方式：" + MethodLabel(method)
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
		return profile, errors.New("端口必须是 1-65535 之间的数字")
	}
	profile.Server.Port = port
	profile.Server.Address = strings.TrimSpace(profile.Server.Address)
	if profile.ServerAddress() == "" {
		return profile, errors.New("请先填写网关地址")
	}
	return profile, nil
}

// AddProfile appends a blank profile and selects it.
func (s *SettingsState) AddProfile() {
	name := "配置"
	for suffix := 2; s.Draft.ProfileIndex(name) >= 0; suffix++ {
		name = fmt.Sprintf("配置 %d", suffix)
	}
	if _, err := s.Draft.AddProfile(name); err != nil {
		s.Status = err.Error()
		s.StatusError = true
		return
	}
	s.refresh()
	s.Select(s.Draft.ProfileIndex(name))
	s.Status = "已新建配置 " + name + "；保存后生效"
	s.StatusError = false
}

// DuplicateProfile copies the selected profile.
func (s *SettingsState) DuplicateProfile() {
	s.refresh()
	source := s.Draft.Profiles[s.Selected]
	name := source.Name + " 副本"
	for suffix := 2; s.Draft.ProfileIndex(name) >= 0; suffix++ {
		name = fmt.Sprintf("%s 副本 %d", source.Name, suffix)
	}
	if _, err := s.Draft.DuplicateProfile(s.Selected, name); err != nil {
		s.Status = err.Error()
		s.StatusError = true
		return
	}
	s.refresh()
	s.Select(s.Draft.ProfileIndex(name))
	s.Status = "已复制为 " + name + "；保存后生效"
	s.StatusError = false
}

// DeleteProfile removes the selected profile, unless it is the last one.
func (s *SettingsState) DeleteProfile() {
	s.refresh()
	if err := s.Draft.DeleteProfile(s.Selected); err != nil {
		s.Status = err.Error()
		s.StatusError = true
		return
	}
	s.refresh()
	active := s.Draft.ActiveIndex()
	if active < 0 {
		active = 0
	}
	s.Select(active)
	s.Status = "已删除配置；保存后生效"
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
		return cfg, errors.New("没有可用的配置")
	}

	name := strings.TrimSpace(s.NameText)
	if name == "" {
		return cfg, errors.New("配置名称不能为空")
	}
	if existing := cfg.ProfileIndex(name); existing >= 0 && existing != index {
		return cfg, fmt.Errorf("配置名称 %q 已存在", name)
	}

	port, err := strconv.Atoi(strings.TrimSpace(s.Ports[index]))
	if err != nil || port < 1 || port > 65535 {
		return cfg, errors.New("端口必须是 1-65535 之间的数字")
	}

	interval := strings.TrimSpace(cfg.Profiles[index].Keep.Interval)
	if interval == "" {
		interval = "90s"
	}
	if duration, err := time.ParseDuration(interval); err != nil || duration <= 0 {
		return cfg, errors.New("保活间隔不是有效时长，例如 90s")
	}

	profile := &cfg.Profiles[index]
	profile.Name = name
	profile.Server.Address = strings.TrimSpace(profile.Server.Address)
	profile.Server.Port = port
	profile.Auth.Username = strings.TrimSpace(profile.Auth.Username)
	profile.Auth.Domain = strings.TrimSpace(profile.Auth.Domain)
	profile.Auth.AuthType = strings.TrimSpace(profile.Auth.AuthType)
	profile.Keep.Interval = interval
	profile.Keep.Targets = splitTargets(s.Targets[index])

	if socks5, httpBind := cfg.ProxyAddresses(); socks5 == "" && httpBind == "" {
		return cfg, errors.New("SOCKS5 和 HTTP 监听至少启用一个")
	}

	cfg.ActiveProfile = name
	cfg.Version = config.CurrentVersion
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
		index := state.Selected
		if index < 0 || index >= len(state.Draft.Profiles) {
			ui.Text(c, "没有可用的配置").FontSize(13)
			return
		}
		profile := &state.Draft.Profiles[index]
		names := state.Draft.ProfileNames()
		active := state.ActiveAtOpen
		enabled := profile.Keep.IsEnabled()

		ui.Column(c).Fill().Padding(windowPadding).Gap(sectionGap).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				windowHeading(c, "设置")
				caption(c, "保存后：连接信息在下次登录生效，代理端口与保活在下次连接生效。")
				if active != "" && active != profile.Name {
					captionTone(c, "保存后将切换到配置 "+profile.Name, toneWarning)
				}
			})

			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Gap(sectionGap).Children(func() {
					ui.Form(c, func() {
						profileSection(c, state, actions, names)
						connectionSection(c, state, actions, profile, index)
						proxySection(c, state)
						keepAliveSection(c, state, profile, &enabled, index)
						startupSection(c, state, actions)
						advancedSection(c, state, profile, actions)
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
				if ui.PrimaryButton(c, "保存").Clicked() && actions.Save != nil {
					actions.Save()
				}
				if ui.Button(c, "关闭").Clicked() && actions.Close != nil {
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
	ui.Fieldset(c, "配置", func() {
		ui.Field(c, "当前配置", func() {
			if ui.Select(c, &state.SelectText, names).Changed() {
				if chosen := indexOf(names, state.SelectText); chosen >= 0 && actions.SelectProfile != nil {
					actions.SelectProfile(chosen)
				}
			}
		})
		ui.Field(c, "名称", func() {
			ui.TextInput(c, &state.NameText)
		})
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "新建").Clicked() && actions.NewProfile != nil {
				actions.NewProfile()
			}
			if ui.Button(c, "复制").Clicked() && actions.DuplicateProfile != nil {
				actions.DuplicateProfile()
			}
			if ui.Button(c, "删除").Disabled(len(names) == 1).Clicked() && actions.DeleteProfile != nil {
				actions.DeleteProfile()
			}
		})
	})
}

func connectionSection(c *ui.Context, state *SettingsState, actions SettingsActions, profile *config.Profile, index int) {
	ui.Fieldset(c, "连接", func() {
		ui.Field(c, "网关地址", func() {
			ui.TextInput(c, &profile.Server.Address).Placeholder("vpn.example.com")
		})
		ui.Field(c, "端口", func() {
			ui.TextInput(c, &state.Ports[index]).Placeholder("443")
		})
		ui.Field(c, "用户名", func() {
			ui.TextInput(c, &profile.Auth.Username)
		})
		ui.Field(c, "认证方式", func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				options := methodOptions(state.Methods, profile.Auth.AuthType, profile.Auth.Domain)
				chosen := methodValue(state.Methods, profile.Auth.AuthType, profile.Auth.Domain)
				if ui.Select(c, &chosen, options).Changed() {
					if picked := indexOf(options, chosen); picked >= 0 && actions.SelectAuth != nil {
						actions.SelectAuth(picked)
					}
				}
				if ui.Button(c, "获取认证方式").Disabled(state.Discovering).Clicked() && actions.DiscoverAuth != nil {
					actions.DiscoverAuth()
				}
			})
			caption(c, authMethodHint(state))
		})
		ui.Field(c, "登录域", func() {
			ui.TextInput(c, &profile.Auth.Domain).Placeholder("local")
		})
		ui.Field(c, "密码", func() {
			ui.TextInput(c, &profile.Auth.Password).Password()
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &profile.Auth.StorePass, "保存密码（文件权限 0600）")
		})
	})
}

func proxySection(c *ui.Context, state *SettingsState) {
	ui.Fieldset(c, "代理", func() {
		ui.Field(c, "SOCKS5 监听", func() {
			ui.TextInput(c, &state.Draft.Proxy.SOCKS5Bind).Placeholder("127.0.0.1:1080")
		})
		ui.Field(c, "HTTP 监听", func() {
			ui.TextInput(c, &state.Draft.Proxy.HTTPBind).Placeholder("127.0.0.1:1081")
		})
		caption(c, "留空即关闭该监听；两者不能同时留空。")
	})
}

func keepAliveSection(c *ui.Context, state *SettingsState, profile *config.Profile, enabled *bool, index int) {
	ui.Fieldset(c, "保活", func() {
		ui.Field(c, "", func() {
			if ui.Checkbox(c, enabled, "启用保活").Changed() {
				profile.Keep.SetEnabled(*enabled)
			}
		})
		ui.Field(c, "间隔", func() {
			ui.TextInput(c, &profile.Keep.Interval).Placeholder("90s")
		})
		ui.Field(c, "手动目标", func() {
			ui.TextArea(c, &state.Targets[index]).Placeholder("http://192.0.2.10/，每行一个")
			caption(c, "只探测 http:// 目标：https 经 CONNECT 隧道，不算应用流量，不续期会话。")
		})
	})
}

func startupSection(c *ui.Context, state *SettingsState, actions SettingsActions) {
	ui.Fieldset(c, "启动", func() {
		ui.Field(c, "", func() {
			if ui.Checkbox(c, &state.Draft.App.Autostart, "登录系统时启动").Changed() && actions.SetAutostart != nil {
				actions.SetAutostart(state.Draft.App.Autostart)
			}
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &state.Draft.App.AutoConnect, "启动后自动连接")
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &state.Draft.App.StartHidden, "启动时隐藏窗口")
		})
		ui.Field(c, "", func() {
			ui.Checkbox(c, &state.Draft.App.AutoReconnect, "断线后自动重连（最多 5 次）")
		})
	})
}

func advancedSection(c *ui.Context, state *SettingsState, profile *config.Profile, actions SettingsActions) {
	ui.Fieldset(c, "高级", func() {
		ui.Field(c, "", func() {
			if ui.Checkbox(c, &state.Draft.Logging.Verbose, "详细日志").Changed() && actions.SetVerbose != nil {
				actions.SetVerbose(state.Draft.Logging.Verbose)
			}
		})
		ui.Field(c, "日志目录", func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, state.LogDir).FontSize(c.Theme().FontSize)
				if ui.Button(c, "打开").Description("打开日志目录").Clicked() && actions.OpenLogs != nil {
					actions.OpenLogs()
				}
			})
		})
		ui.Field(c, "客户端数据", func() {
			ui.TextInput(c, &profile.Debug.ClientData).Placeholder("留空则按配置名存放")
		})
	})
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
const authAutoLabel = "自动（网关唯一方式）"

// methodOptions builds the picker's entries. A saved pair the gateway did not
// list is kept as a trailing entry so the select never shows a blank value; it
// is informational, not selectable.
func methodOptions(methods []session.AuthMethod, authType, domain string) []string {
	options := []string{authAutoLabel}
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
		return authAutoLabel
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
	return fmt.Sprintf("已保存：%s · %s（网关未提供）", authType, domain)
}

// authMethodHint explains what the picker currently means for the login.
func authMethodHint(state *SettingsState) string {
	switch {
	case state.Discovering:
		return "正在向网关查询认证方式…"
	case len(state.Methods) == 0:
		return "尚未查询。自动模式使用网关提供的唯一方式；多种方式时登录会询问。"
	default:
		return fmt.Sprintf("网关提供 %d 种认证方式。固定一种可避免每次登录询问。", len(state.Methods))
	}
}
