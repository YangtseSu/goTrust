# Progress Log

## Session: 2026-10-29

Plan: `.planning/2026-10-29-ui-consolidation/`. Pinned with
`PLAN_ID=2026-10-29-ui-consolidation`.

Started from a review of the two-window interface. The design was worked out
against two local HTML mockups before any Go change:

- `/tmp/gotrust-mockup.html` — the final layout: left navigation, dashboard,
  account, proxy, keep-alive, startup, terminals, logs, settings and
  multi-account pages, light and dark.
- `/tmp/gotrust-status-variants.html` — five status/button combinations; the
  stateful button (B) was chosen, then refined so the gateway address is
  labelled and the status text is one line under the phase.

The review settled these points:

- one window, no settings window, no tab bar;
- dashboard's play/stop button is the primary control and carries the phase by
  colour and icon;
- account order is gateway, username, password + store-password, then the
  remaining login options;
- profiles move to a bottom sidebar page; no profile bar or name on 账户;
- 代理 owns the PAC and the Clash/Mihomo copy controls;
- 日志 is a page with a live tail, not a directory button;
- 设置 owns language, local data and the About card;
- 终端管理 shows the full portal fields;
- the tray is status, 打开仪表盘, 打开默认应用, 连接/断开, 切换账户 (only
  with more than one account), 退出 — no 设置;
- no memory display; the traffic card is optional.

### Phase 1: Single-window shell and page state

- **Status:** complete
- One window, one draft. `ui.View` is the shell: title strip, a 208 DIP
  sidebar, a 680-capped page column, the status toasts and a save bar that
  appears only while `SettingsState.Changed()`. The pages are 仪表盘, 账户,
  代理, 保活, 启动, 终端管理, 日志, 设置 and 多账户; the nav items are tabs
  with Enter/Space activation, and they dim and disable while a pre-login
  panel owns the flow.
- `SettingsState` is created in `Run` once the log directory is known and is
  kept for the life of the process; `State.Settings` points at it. 保存,
  放弃, a tray profile switch and a setup save reload it in place.
- `internal/gui` has one window at 900×740 (min 720×560). `settingsWindow`,
  `openSettings`, `hideSettings`, `closeSettings`, `resetSettingsDraft` and
  the Ctrl+, shortcut are gone; `showWindow` keeps the page and `showPage`
  selects one. The tray's 设置 item is gone; the rest of the tray waits for
  phase 4.
- The settings pages are page functions: account, proxy, keep-alive, startup,
  terminals, logs (the log directory until the live tail lands), settings (the
  advanced fields) and the transitional profiles picker. The log directory
  moved off 设置 to 日志.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` rendered every
  state; the shell was checked in both languages, in dark mode and at the
  720×560 minimum. The real binary ran under headless mutter (`goTrust ready`,
  clean SIGTERM exit; the Gtk-CRITICAL scale-factor warning also appears on the
  pre-change binary and belongs to mygo/AppIndicator).

### Phase 2: Account page, gateway field and password row

- **Status:** complete
- The first-run panel is gone: `ui/setup.go` and `ui/setup_test.go` are
  deleted, and `State.Setup`, `Actions.SaveSetup`, `refreshSetup`, `saveSetup`
  and `setSetupError` with them. `ProfileProblem` moved to `ui/account.go`.
- The account page is the first-run form: 网关 (`host:port`, one field), 用户名,
  密码 with 保存密码 on the same row, 认证方式 + 获取认证方式, 登录域, 密码安全
  and the stored TOTP. 名称 and 端口 are no longer on the page;
  `SettingsState.NameText` stays for the phase 3 profile editor.
- `SettingsState.Ports` became `Gateways []string`; `ParseGateway` accepts
  `host:port`, `https://host:port`, a bare host (443), a trailing path and
  IPv6 in brackets, and refuses other schemes, an empty host and ports outside
  1–65535. `Apply` folds every profile's gateway, `folded` ignores text that
  does not parse, and `Changed` marks such text as an edit so 保存 can explain
  it. The file schema is unchanged.
- `config.NewProfile` now defaults `store_password` to true; a loaded profile
  keeps its own value, so an upgrade does not start writing credentials for an
  account that never opted in. The box is 保存密码 (the file is 0600); the
  second-factor hint is the password placeholder.
- 连接 from the window saves a dirty draft through `writeDraft` first and, on a
  validation failure, selects 账户 and keeps the edits. The tray uses
  `TrayLogin`, which logs in with the saved configuration. An incomplete saved
  profile from either entry point selects 账户 with the window's sentence.
- Catalog: `passwordPlaceholder` and `problemGatewayScheme` added;
  `setup*`, `errPort`, `errAddress` and `fieldPort` removed;
  `storePassword` is now 保存密码. `go tool mygo generate` rewrote the
  accessors.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` rendered
  `account-fresh`, `account-fresh-small` and the English/dark pages; the
  password row and the truncated placeholder were checked at both widths. The
  real binary ran under headless mutter from a fresh config: the new file
  carries `store_password = true`, the log says
  `goTrust ready; profile 默认`, and SIGTERM exits 0. Interactive clicks in the
  real window remain phase 6.

### Phase 3: Demote profiles

- **Status:** complete
- 账户 is only the login form: `profileBar` went with the two-window shell, the
  名称 field went in phase 2, and the page has no picker. `account-fresh` shows
  it.
- 多账户 is a radio group of rows: `ui.Avatar` initials, the account name, its
  gateway from the draft buffer (`尚未填写网关` when empty), and `当前` on the
  live account (`SettingsState.ActiveAtOpen`). The accent tint marks the draft's
  selection; a click calls the settings' `SelectProfile`, which only moves that
  selection. The name field edits `NameText`, and 新建/复制/删除 sit under it,
  outside the form's label column. Saving still applies the rename and moves the
  client data through `ClientDataChanges`.
- The dropdown is gone with `SettingsState.SelectText` and
  `profilePickerWidth`. `NameText` is still the single rename buffer, so a name
  typed for one account is discarded when another row is selected (as the old
  dropdown did; documented on `Select`).
- The sidebar's bottom row is `navAccount`: `navRow` with `subdued`, which keeps
  the muted ink while its page is on screen. Its tag is capped at 76 DIP, so a
  long account name ellipsizes instead of squeezing 多账户; the title strip is
  single-lined for the same reason.
- Tray: the submenu is 切换账户 (`traySwitchAccount`) and appears only with more
  than one account; items are radios checked on `ActiveProfile` (IDs `accounts`
  and `account:<name>`). The full tray order stays phase 4.
- Wording: every user-facing 配置 that meant a profile is 账户, including the
  auto-connect and TOTP hints that still pointed at 设置. New accounts are named
  账户 1, 账户 2, … and the toast reads 已新建 {name}. `config.DefaultProfileName`
  stays `默认` (stored value, and the name the v1 `client_data.json` maps to);
  the `config.Profile` type and the TOML keys are unchanged. `CurrentProfile`
  became `profileCurrent` (当前), `NameField` is 账户名称.
- Tests: `TestAccountsPageRowsMarkTheLiveAccountAndSelect` (names, gateways,
  当前, click selects), `TestTrayMenuHidesTheAccountSubmenuForOneAccount`,
  `TestSidebarTagsTheLiveAccount` (the bare name, inside the sidebar), and
  `TestViewRendersEveryPhase` now looks for the account name instead of 配置.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` rendered
  `settings-accounts` in both languages and at 720×560; a throwaway test
  checked a 14-character account name in the sidebar. Interactive clicks in the
  real window remain phase 6.

### Phase 4: Stateful play/stop control and tray

- **Status:** pending

### Phase 5: Page contents approved in the mockup

- **Status:** pending

### Phase 6: Verification and documentation

- **Status:** pending

### Phase 7 (optional): Traffic card

- **Status:** pending (optional; confirm before starting)
