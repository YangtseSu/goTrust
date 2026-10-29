# Task Plan: goTrust — one-window interface

## Goal

One resizable window with a left navigation. The dashboard is the session
control; the other pages are 账户, 代理, 保活, 启动, 终端管理, 日志 and 设置,
with 多账户 demoted to the bottom of the sidebar. The settings window and the
first-run setup panel are gone: 账户 is the first-run form and the only editor
of the account. Profiles are low-prominence; the tray is ordered by frequency
and no longer offers 设置. The dashboard's play/stop control is the only session
control, and its state is carried by the button's colour and icon, not by a
separate chip.

## Next Step

Phase 4: the stateful play/stop control and the tray. 账户 is the first-run
form, 多账户 is the bottom page with rows, and the tray's account submenu is
the only switcher left.

## Current Phase

Phase 3 complete; Phase 4 pending.

## Decisions already made

- **Window:** one window, 900×740, minimum 720×560; 208 DIP sidebar; page
  content capped at 680; the title strip spans the window.
- **Navigation:** 仪表盘; separator; 账户, 代理, 保活, 启动, 终端管理, 日志,
  设置; a spacer; separator; 多账户 at the bottom.
- **Dashboard:** status card with a 60 DIP stateful play/stop button at the
  right of its first row; text left: phase, then
  `网关地址 <host:port> · 已保持 <duration>`; a four-cell metrics row
  (已分配地址, SOCKS5, HTTP, 保活); resources card with 打开默认应用 in its
  header; no footer and no copy/overflow menu.
- **Account order:** 网关 (one field), 用户名, 密码 + 保存密码 on one row,
  认证方式, 登录域名, 修改密码, TOTP. No 名称 field on this page.
- **Profiles:** the 多账户 page owns select/rename/new/duplicate/delete; the
  sidebar bottom item shows the active account; the tray account submenu only
  appears with more than one account.
- **Tray:** status line; 打开仪表盘; 打开默认应用; 连接/断开; 切换账户
  (conditional); 退出. No 设置 item.
- **Page contents:** 代理 owns the listeners, the PAC address and the Clash/
  Mihomo copy controls; 日志 is a live tail; 设置 owns language, local data and
  the About card; 终端管理 shows every user-facing portal field; 启动 has only
  the four toggles.
- **About:** version from `internal/version`, project URL
  `github.com/YangtseSu/goTrust`, license AGPL-3.0-or-later, copyright.
- **No memory display.** The traffic card is a separate, optional phase.
- **Design references (not repository files):** `/tmp/gotrust-mockup.html` and
  `/tmp/gotrust-status-variants.html`; the agreed card is variant B.

## Phases

Status values are `pending`, `in_progress`, `complete` only.

### Phase 1: Single-window shell and page state

The two-window architecture is the root of the navigation complaints: settings
live in a second window with six tabs, while the main window duplicates a few
of its actions. This phase turns both into one window with a page state; the
existing forms keep working inside their pages.

- [x] Add `ui.Page` and its constants (`PageDashboard`, `PageAccount`,
      `PageProxy`, `PageKeepAlive`, `PageStartup`, `PageTerminals`, `PageLogs`,
      `PageSettings`, `PageProfiles`), `State.Page` and `State.Settings`;
      default to the dashboard.
- [x] Add `Actions.ShowPage func(Page)`; the sidebar and the tray use it, and
      the gui stores the page in the view state and invalidates the one window.
- [x] Rewrite `ui.View` as the shell: title bar, sidebar, content, global
      status toast and save bar. `navItem` draws the icon and label; the active
      page uses the accent tint. Icons for the pages go in `ui/icons.go`.
- [x] Split `ui/settings.go`: keep `SettingsState` and the section builders;
      delete `SettingsView`, `settingsBody`, `settingsTabs`, `settingsTabBar`,
      `settingsPage`, `profileBar` and `pageEffect`. Render the sections as
      page functions called by the shell.
- [x] Remove `SettingsState.Tab` and `SettingsActions.Close`: the page is
      `State.Page`, and closing the window is hiding it. Update every test
      helper that set `Tab` or called `Close`.
- [x] Move the save/discard controls out of the settings window footer into a
      shell-level bar at the bottom of the content; it appears only while
      `SettingsState.Changed()`. 保存 applies, 放弃 reloads.
- [x] `SettingsState` becomes a long-lived draft: created at startup, kept
      while the window is hidden, reloaded by 保存, 放弃, a profile switch and
      an external configuration change. The old "reopening discards" rule is
      gone.
- [x] `internal/gui`: delete `settingsWindow`, `openSettings`, `hideSettings`,
      `closeSettings` and their close handlers; one window at 900×740 (min
      720×560); `showWindow` shows and focuses without changing the page;
      `showPage` selects the page, shows and focuses.
- [x] Remove `Actions.Settings` and the Ctrl+, shortcut. The window's `⋯`
      menu is not added back; quit stays in the tray.
- [x] While a pre-login panel owns the flow (`panelOpen`), the content shows
      the panel regardless of the selected page, and the sidebar is disabled.
      Esc keeps cancelling the panel.
- [x] Tests: page switching through the sidebar; one window; the draft survives
      a hide; the save bar appears and clears; a11y roles for the nav.
- **Status:** complete.

Notes:

- The first-run panel lost its `更多设置…` button with `Actions.Settings`; a
  fresh install reaches the other pages after phase 2 makes 账户 the first-run
  form. The panel still owns the content and disables the sidebar according to
  the plan.
- `多账户` exists as a transitional picker (select, 新建, 复制, 删除) so profile
  management does not disappear between this phase and phase 3, which replaces
  it with the account rows.
- The tray's `显示窗口` item still calls `ShowWindow`; phase 4 renames it to
  `打开仪表盘` and points it at `ShowPage(PageDashboard)`.
- The dashboard keeps its copy menu until phase 4 moves PAC and Clash to 代理.

### Phase 2: Account page, gateway field and password row

- [x] Delete the first-run panel: `State.Setup`, `SetupDraft`,
      `NewSetupDraft`, `setupPanel`, `Actions.SaveSetup`, `saveSetup`,
      `setSetupError`, `refreshSetup`, and `ui/setup_test.go`. The 账户 page is
      the first-run form; `ProfileProblem` stays as the shared validation
      sentence.
- [x] Replace `SettingsState.Ports` with `Gateways []string`: one text field
      per profile holding `host:port`. A parser accepts
      `https://host:port`, `host:port`, `host` (port 443), a trailing `/`, and
      bracketed IPv6; it rejects other schemes, an empty host and a port outside
      1–65535 in the window's words.
- [x] Update `refresh`, `Apply`, `folded`, `Target`, `targetKey`,
      `DuplicateProfile` and `DeleteProfile` to the gateway buffer. `Apply`
      writes the parsed address and port into the profile exactly as the file
      stores them; the configuration schema does not change.
- [x] Order the 账户 form: 网关, 用户名, 密码 + 保存密码, 认证方式 (+ 获取认证
      方式), 登录域名, 修改密码, TOTP when stored. Remove the 名称 field.
- [x] Password row: the input grows, 保存密码 sits on the same line; the label
      is just 密码 and the hint about a second factor moves into the password
      placeholder.
- [x] `config.NewProfile` defaults `Auth.StorePass` to true; existing profiles
      keep their stored value. The form shows the profile's value.
- [x] Login from the dashboard saves a dirty draft first; a validation failure
      selects the 账户 page, keeps the draft and reports the reason as a toast.
      A login from the tray keeps using the saved configuration.
- [x] Keep the `ActiveAtOpen` warning on the 账户 page when the selected draft
      profile differs from the live account.
- [x] Tests: gateway parsing table (scheme, bare host, IPv6, bad port, empty);
      field order; the store-password default; save-then-login.
- **Status:** complete.

Notes:

- The parser lives in `ui/account.go` with `ProfileProblem` and `gatewayText`,
  the field text. `fieldWidth` still caps the controls; the gateways of every
  profile are folded on apply, so switching between two edits cannot lose the
  first one.
- A gateway that does not parse marks the draft as changed even though nothing
  can be folded: the save bar has to offer 保存 so the reason is reachable. An
  empty field stays an account without a gateway, so proxy and startup settings
  can be saved before the gateway is known.
- The window's 连接 saves the draft first (`saveDraftForLogin`); the tray has
  its own `TrayLogin`, which uses the saved configuration. `writeDraft` is the
  one save path for 保存 and for the login's first step.
- A fresh install still opens on the dashboard; 连接 on an incomplete profile
  selects 账户 and shows the window's sentence. The 用户-facing name editor
  waits for phase 3.
- The render set replaced the three `main-setup` images with `account-fresh`
  and `account-fresh-small`. The old `errPort`, `errAddress`, `fieldPort` and
  `setup*` catalog entries are gone; `passwordPlaceholder` and
  `problemGatewayScheme` are new.

### Phase 3: Demote profiles

- [x] Remove `profileBar` and the profile name from the 账户 page.
- [x] Add the 多账户 page: one row per account (name, gateway, active mark),
      the name field for the selected draft profile, and 新建/复制/删除. The
      draft selection stays inside `SettingsState`; saving applies the rename
      and moves the client data file through the existing `ClientDataChanges`.
- [x] Add the subdued 多账户 item at the bottom of the sidebar with the live
      account name as a tag.
- [x] Tray: replace the 配置 submenu with 切换账户 and render it only when the
      configuration has more than one account; radio marks the active one.
- [x] Rename the user-facing profile wording to 账户: `ProfileName`,
      `CurrentProfile`, `NameField`, the default profile name and the title
      strip. The `config.Profile` type and file keys stay.
- [x] Tests: 账户 has no name/profile bar; a single-account tray has no account
      item; a rename still moves the client data; the bottom item tags the live
      account.
- **Status:** complete.

Notes:

- `profileBar` was already deleted with the two-window shell, and the account
  page lost its 名称 field in phase 2; the render `account-fresh` is the proof
  that 账户 is now only the login form.
- 多账户 is a radio group of rows: mygo's `Avatar` (initials), the account name,
  its gateway from the draft buffer (`尚未填写网关` when empty) and `当前` on the
  live account (`SettingsState.ActiveAtOpen`). The accent tint marks the draft's
  selection; a click only moves that selection, and 保存 is what switches the
  live account and moves the client data. 新建/复制/删除 sit under the name
  field, outside the form's label column.
- The dropdown is gone: `SettingsState.SelectText` and `profilePickerWidth` are
  deleted. `NameText` remains the single rename buffer, so a name typed for one
  account is discarded when another row is selected (documented on `Select`).
- The sidebar row is a subdued variant of `navItem` (`navRow` with `subdued`):
  it keeps the muted ink while its page is on screen and is the only row with a
  tag. The tag is capped at 76 DIP so a long account name ellipsizes instead of
  squeezing 多账户; the title strip is single-lined for the same reason.
- Tray: the account submenu is `切换账户` and exists only with more than one
  account; its items are radios checked on `ActiveProfile` (IDs `accounts` and
  `account:<name>`). Phase 4 owns the rest of the tray order.
- Wording: every user-facing 配置 that meant a profile is now 账户, including
  the auto-connect and TOTP hints that still pointed at 设置. New accounts are
  named 账户 1, 账户 2, … (`profileBaseName`/`profileBaseNameN`), and the toast
  reads 已新建 {name}. `config.DefaultProfileName` stays `默认`: it is a stored
  value and the name the v1 `client_data.json` maps to. The `config.Profile`
  type and the TOML keys are unchanged.
- Tests added: the row list shows names, gateways and the live mark and selects
  on a click; a single-account tray has no account item; the sidebar tag is the
  live account name and lives inside the sidebar; `TestViewRendersEveryPhase`
  now looks for the account name instead of the word 配置.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` rendered
  `settings-accounts` in both languages plus the 720×560 page; a throwaway test
  checked that a 14-character account name ellipsizes in the tag without
  squeezing the label. Interactive clicks in the real window remain phase 6.

### Phase 4: Stateful play/stop control and tray

- [ ] Add `powerControl` to the dashboard: a 60 DIP round button at the right
      of the status card's first row. The button is the action and the status:
      - offline: accent blue, play triangle, label 连接;
      - connecting: amber, spinner, label 取消;
      - online: green, stop square, label 断开;
      - reconnecting: amber, spinner, label 取消;
      - failed: red, exclamation, label 重试.
      Hover is red for a session-ending action and a darker accent for a
      starting one. A busy login disables a new start but keeps cancel
      available.
- [ ] Replace the chip and the old `subline` with the phase text and the meta
      line `网关地址 <host:port> · 已保持 <duration>`; reconnecting shows the
      ladder position and countdown; failed shows the detail. The gateway comes
      from the live profile, the duration from `State.Session.Since`.
- [ ] Keep the metrics row (已分配地址, SOCKS5, HTTP, 保活), using `—` for
      values that are not available; the keepalive value carries the summary in
      its description. Delete `footer`, `footerContent` and `footerTip`.
- [ ] Move 打开默认应用 into the resources card header; delete the dashboard
      copy menu and the `⋯` menu. 复制配置 and PAC live on 代理; 打开日志目录
      lives on 日志; quit lives in the tray.
- [ ] Rebuild `TrayMenu`: status line, separator, 打开仪表盘, 打开默认应用,
      separator, 连接/断开, optional separator + 切换账户 submenu, separator,
      退出. Build the separators conditionally so a single-account menu has no
      empty group. No 设置 item.
- [ ] Rename `trayShow` to 打开仪表盘 / Open Dashboard and `trayProfiles` to
      切换账户 / Switch Account; remove the unused `traySettings` message.
- [ ] Tray icon click shows and focuses the window and keeps the last page;
      打开仪表盘 switches to the dashboard. 打开默认应用 stays disabled
      without a launch URL; 连接 is disabled while busy.
- [ ] Tests: each phase's button icon, colour, label and action; the tray order
      with one and two accounts; the page switch from 打开仪表盘; the tray icon
      click keeps the page.
- **Status:** pending.

### Phase 5: Page contents approved in the mockup

- [ ] **代理**: keep the two listener fields; add a copy card with the PAC
      address (复制, disabled when empty) and a Clash/Mihomo row (format picker
      for Clash, Clash Script, Clash configuration; 复制). The wording is
      复制代理配置. The dashboard no longer owns either action.
- [ ] **日志**: a new page with a read-only tail of the current log. The gui
      owns a tailer that reads `logging.Logger.Path()`, polls once a second,
      keeps the last 500 lines and stops on quit; it reopens after rotation or
      truncation. The page has a level filter, a search field, 自动滚动, 复制,
      打开目录 and the 详细日志 switch. The switch applies through `SetVerbose`
      and is part of the saved settings. No credentials are rendered; the log
      layer already redacts them.
- [ ] **设置**: language (moved out of 启动), 本机数据 (the data file field with
      the resolved default path as its placeholder, the device ID and 打开配置
      文件), and 关于 (版本, 项目主页, 许可证). The version comes from
      `internal/version.Version`; the project and license buttons open
      `github.com/YangtseSu/goTrust` and its `LICENSE` through the browser.
      Remove the log, memory and verbose cards from this page.
- [ ] **终端管理**: show the portal's user-facing fields per endpoint — name,
      current/online/offline, client/browser type, OS and version, browser,
      last login time, login address (LAN as 内网) and IP, network zones, and
      device ID. Add the summary (trusted count and limit, temporary count,
      self trust state) and keep the trust/untrust action.
- [ ] **启动**: keep only 开机启动, 自动连接, 启动时隐藏窗口 and 断线自动重连;
      language moves to 设置.
- [ ] Add the locale keys for the new labels and remove the strings that no
      page uses; keep zh as the source and regenerate the catalog.
- [ ] Render cases: dashboard online/offline/connecting/reconnecting/failed,
      every page at 900×740 and at the minimum, light and dark, plus the save
      bar and the log page.
- **Status:** pending.

### Phase 6: Verification and documentation

- [ ] `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`,
      `reuse lint`.
- [ ] Run the real window at 900×740 and at 720×560: switch every page, edit
      and save the gateway, hide and reopen with a dirty draft, connect and
      disconnect from the button and the tray, and answer at least one login
      panel. Follow `docs/agent-ui-testing.md`.
- [ ] Rewrite `docs/design.md` for the one-window/left-navigation rules and the
      stateful button. Update `docs/configuration.md` for the accepted gateway
      input. Update `docs/agent-ui-testing.md` and the README first-run notes
      that still mention the settings window or the setup panel.
- **Status:** pending.

### Phase 7 (optional): Traffic card

The mockup shows a Clash Verge-style traffic card; it is not part of the
approved 7/8/11/13 work and is not started until asked for.

- [ ] Add byte counters to `core/proxy` (`trackedConn` reads and writes for
      SOCKS5 and CONNECT; a counting writer for plain HTTP), expose a snapshot,
      and sample it once a second in the gui into a 60-point ring.
- [ ] Add the dashboard card with the current up/down rates and two line series;
      use the mygo chart primitives or a small custom sparkline.
- [ ] Tests for the counters and the ring; smoke the card in the real window.
- **Status:** pending (optional; confirm before starting).

## Out of scope

- A memory-usage display or chart.
- A connections-count chart.
- A 设置 item in the tray.
- Changing the `core/atrust` protocol layer; only the proxy counters above
  touch `core/` and only in the optional phase.
- Reworking the resource row's two-click menu; the current row menu stays.
