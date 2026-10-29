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

Start Phase 14: move the settings draft's writes to the main thread. It is the
one real bug of the mygo usage audit; Phases 15-19 follow in the order below.

## Current Phase

Phase 14 pending; Phases 15-19 planned after it.

## Decisions already made

- **Window:** one window, 900×740, minimum 720×560; 208 DIP sidebar; page
  content capped at 680; the title strip spans the window.
- **Navigation:** 仪表盘; separator; 账户, 代理, 保活, 启动, 终端管理, 日志,
  设置; a spacer; separator; 多账户 at the bottom.
- **Dashboard:** status card with a tone-tinted phase chip (dot + label) at
  the left of its first row and a 60 DIP stateful play/stop button at the
  right; one meta line `网关地址 <host:port> · 已保持 <duration>`; a four-cell
  metrics row (已分配地址, SOCKS5, HTTP, 保活); resources card with
  打开默认应用 in its header; no footer and no copy/overflow menu. The chip is
  a mark, not a control: the button alone is the session control. This
  reverses the earlier "no separate chip" decision after review; the tinted
  background and the leading dot stay.
- **Window minimum:** 720×560 is the page area. GTK applies the minimum hint to
  the whole surface, its CSD shadow included, and on Wayland neither `Bounds`
  nor `ContentBounds` reports that margin, so the hint carries a measured 52×52
  shadow (`windowShadowWidth`/`windowShadowHeight` in `internal/gui`).
- **Gateway port:** printed as text, never through ICU's `{port, number}`,
  which groups a five-digit port as `38,265`.
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
- **No memory display.** The traffic card (Phase 7) is built: the proxy's
  last minute of up/down throughput, between the session and the resources.
- **mygo usage audit (2026-10-29).** All 116 pages under the mygo docs were
  read and every used API checked against the pinned v0.7.2. The settings
  draft is main-thread state: background results are delivered through
  `RunOnMain`, not written under `a.mu` while the frame reads them. The theme
  starts from `*c.Theme()`, so the desktop's text scale survives. The sidebar
  is a real tab list (`FocusGroup`). Icon-only controls carry a tooltip and
  the row action is a menu button. A second launch restores the window. The
  tray keeps its menu on click, and the docs say so.
- **Design reference:** `docs/mockup.html`, the layout the views follow. The
  status/button study that chose the stateful control was
  `/tmp/gotrust-status-variants.html` (variant B); its outcome is recorded in
  this plan and in `docs/design.md`.

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

- [x] Add `powerControl` to the dashboard: a 60 DIP round button at the right
      of the status card's first row. The button is the action and the status:
      - offline: accent blue, play triangle, label 连接;
      - connecting: amber, spinner, label 取消;
      - online: green, stop square, label 断开;
      - reconnecting: amber, spinner, label 取消;
      - failed: red, exclamation, label 重试.
      Hover is red for a session-ending action and a darker accent for a
      starting one. A busy login disables a new start but keeps cancel
      available.
- [x] Replace the chip and the old `subline` with the phase text and the meta
      line `网关地址 <host:port> · 已保持 <duration>`; reconnecting shows the
      ladder position and countdown; failed shows the detail. The gateway comes
      from the live profile, the duration from `State.Session.Since`.
- [x] Keep the metrics row (已分配地址, SOCKS5, HTTP, 保活), using `—` for
      values that are not available; the keepalive value carries the summary in
      its description. Delete `footer`, `footerContent` and `footerTip`.
- [x] Move 打开默认应用 into the resources card header; delete the dashboard
      copy menu and the `⋯` menu. 复制配置 and PAC live on 代理; 打开日志目录
      lives on 日志; quit lives in the tray.
- [x] Rebuild `TrayMenu`: status line, separator, 打开仪表盘, 打开默认应用,
      separator, 连接/断开, optional separator + 切换账户 submenu, separator,
      退出. Build the separators conditionally so a single-account menu has no
      empty group. No 设置 item.
- [x] Rename `trayShow` to 打开仪表盘 / Open Dashboard and `trayProfiles` to
      切换账户 / Switch Account; remove the unused `traySettings` message.
- [x] Tray icon click shows and focuses the window and keeps the last page;
      打开仪表盘 switches to the dashboard. 打开默认应用 stays disabled
      without a launch URL; 连接 is disabled while busy.
- [x] Tests: each phase's button icon, colour, label and action; the tray order
      with one and two accounts; the page switch from 打开仪表盘; the tray icon
      click keeps the page.
- **Status:** complete.

Notes:

- The dashboard is two cards on the window's own background (`card`, a
  bordered column): the session card and the resources card. The status card's
  first row is the phase and its facts on the left and the 60 DIP control on
  the right; a rule, then the four-cell metrics row. The resources card's
  header is the title, the count, a spacer and 打开默认应用, disabled while the
  grant has no launch address.
- `powerAppearance` maps the phase to the control's label, glyph and fill;
  `powerControl` owns the hover (red for 断开/取消, `AccentHover` for
  连接/重试) and the click (active phases call `Logout`, idle and failed call
  `Login`). The two waiting phases use mygo's `Spinner` instead of a glyph;
  `busyLine` now belongs to the save bar alone, and the dashboard button is
  simply disabled while a start would race.
- `sessionLine` is shared by the dashboard and the pre-login panel, so a
  question still keeps the phase above it. `phaseText` is the phase label, or
  the reconnect ladder when the ladder has started; `sessionFacts` builds the
  meta line from `SessionGateway` and the held time (failure detail in the
  danger tone, reconnect in the warning tone). The assigned address left the
  sentence for the metrics row, so `sessionIntranet` and `sessionEndpoint` are
  gone.
- The keep-alive metric shows the age (`keepAliveAgoValue`, seconds) in the
  probe's tone, with `KeepAliveSummary` as the value's description. A metric
  with no value shows `—`.
- 打开默认应用 now uses the window-level `openDefaultApp` key (the tray shared
  it); the tray's `trayShow` became `trayDashboard` and `traySettings` is gone.
  The copy actions (`CopyPAC`, `CopyClash`) and `ClashFormat` stay on `Actions`
  for 代理 in phase 5 but no page shows them until then; their catalog strings
  stay for phase 5's sweep too, together with `keepAliveAgo`, `proxyDisabled`,
  `proxyListening`, `pacURL` and the already-unused `settingsAction`,
  `moreActions`, `openLogsMenu`, `windowSettingsTitle`.
- `Actions.ShowWindow` is gone: the tray icon's click path was always
  `tray.OnClick(a.showWindow)`, and 打开仪表盘 now goes through
  `ShowPage(PageDashboard)`.
- Tests: `TestPowerControlShowsThePhase` covers the label/glyph/fill/ending of
  all five phases, `TestButtonsCallTheRightAction` the click per phase,
  `TestSessionMetricsShowTheSessionFacts` the row and its description,
  `TestTrayMenuOrder` the order with one and two accounts,
  `TestTrayMenuOpensTheDashboard` the page switch,
  `TestTrayMenuOffersConnectWhenIdle` the busy disable. The footer, chip and
  copy-menu tests are gone with what they tested.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` rendered the five
  dashboard phases, the long grant at 900×740 and 720×560, dark mode and both
  languages; the two-card layout, the four metrics and the spinner were checked
  on the images. A throwaway pixel probe confirmed the hover fill (断开 turns
  `Danger`, 连接 turns `AccentHover`). The binary ran under headless mutter
  (`goTrust ready`, PAC port 1082 already held by another process, SIGTERM
  exit 0). Interactive clicks and tray behaviour remain phase 6.

### Phase 5: Page contents approved in the mockup

- [x] **代理**: keep the two listener fields; add a copy card with the PAC
      address (复制, disabled when empty) and a Clash/Mihomo row (format picker
      for Clash, Clash Script, Clash configuration; 复制). The wording is
      复制代理配置. The dashboard no longer owns either action.
- [x] **日志**: a new page with a read-only tail of the current log. The gui
      owns a tailer that reads `logging.Logger.Path()`, polls once a second,
      keeps the last 500 lines and stops on quit; it reopens after rotation or
      truncation. The page has a level filter, a search field, 自动滚动, 复制,
      打开目录 and the 详细日志 switch. The switch applies through `SetVerbose`
      and is part of the saved settings. No credentials are rendered; the log
      layer already redacts them.
- [x] **设置**: language (moved out of 启动), 本机数据 (the data file field with
      the resolved default path as its placeholder, the device ID and 打开配置
      文件), and 关于 (版本, 项目主页, 许可证). The version comes from
      `internal/version.Version`; the project and license buttons open
      `github.com/YangtseSu/goTrust` and its `LICENSE` through the browser.
      Remove the log, memory and verbose cards from this page.
- [x] **终端管理**: show the portal's user-facing fields per endpoint — name,
      current/online/offline, client/browser type, OS and version, browser,
      last login time, login address (LAN as 内网) and IP, network zones, and
      device ID. Add the summary (trusted count and limit, temporary count,
      self trust state) and keep the trust/untrust action.
- [x] **启动**: keep only 开机启动, 自动连接, 启动时隐藏窗口 and 断线自动重连;
      language moves to 设置.
- [x] Add the locale keys for the new labels and remove the strings that no
      page uses; keep zh as the source and regenerate the catalog.
- [x] Render cases: dashboard online/offline/connecting/reconnecting/failed,
      every page at 900×740 and at the minimum, light and dark, plus the save
      bar and the log page.
- **Status:** complete.

Notes:

- 代理: `proxyCopySection` follows the listener form with the PAC address and
  a 复制 that is disabled while `State.PACURL` is empty, then a
  Clash/Mihomo select (Clash 片段, Verge 脚本, Clash 完整配置) and its 复制.
  `SettingsState.ClashFormat` keeps the choice; `clashLabel`/`clashFormat` map
  between the picker text and the `ClashFormat` the action takes. Both buttons
  read 复制, so their accessible labels are 复制 PAC 地址 and 复制 Clash 配置;
  the section heading is `copyConfig`, now 复制代理配置.
- 日志: `internal/gui/logtail.go` owns the tail. `logTail.run` polls once a
  second, reads only what was appended, holds a partial line until its
  newline, seeks to the last 256 KiB on the first read of a longer file and
  drops that window's partial first line, and restarts on a rotation
  (`os.SameFile`) or a shrunk file. It keeps `ui.LogTailLines` (500) lines and
  hands out a copy; `setLogLines` applies it through `mygo.RunOnMain`, so the
  page never reads a slice the tailer is replacing. `SettingsState.LogPath`
  is the file, `LogLines` the tail, `LogFilter` and `LogAutoScroll` the page
  state. The page has 详细日志 (the saved `SetVerbose` switch), 打开目录, a
  search field, 自动滚动, 复制 (disabled with no visible line) and a
  monospace pane; the footer names the file and 最后 500 行.
- The page has no level filter. `core/logging` writes the standard
  `log.LstdFlags` prefix and the protocol layer adds no level, so a
  INFO/WARN/ERROR picker would classify nothing; 详细日志 is the only level
  control, and a filtered tail is the search field. This is the one planned
  item not shipped as written.
- Auto-scroll reveals the last line only when the visible count changed, so a
  user can read further up while the switch stays on; the next new line
  returns to the end.
- 设置: language, 本机数据 and 关于. The data file placeholder is
  `ClientDataPathFor` through `shortPath` (`~` for the home directory); the
  device id sits beside 打开配置文件. 关于 shows `internal/version.Version`
  (`v` prefixed unless `devel`), `github.com/YangtseSu/goTrust` and
  AGPL-3.0-or-later · © 2026 Yangtse Su; both buttons open their page through
  `Actions.OpenURL`, which reuses the http/https guard of a resource.
  `identitySummary`, 客户端标识, the log directory and 详细日志 left 设置.
- 启动 is the four toggles only; the language picker moved to 设置.
- 终端管理: the summary reads 授信终端 n/limit · 临时终端 m · 本机已信任 or
  未信任 (the limit is omitted when the portal sends 0). Each list is a card
  headed by `n 台`; an endpoint is its name and 当前设备/在线/离线, then a
  grid of 类型, 系统, 浏览器, 最近登录, 登录地址 (LAN as 内网（ip）), 网络区域
  and 设备 ID (`DevDBID`, `ID` as the fallback). A field the portal omitted is
  an em dash. Three cells fit the 900 window, two the 720 minimum.
- Catalog: 27 keys added, 25 dead ones removed (the old section titles, the
  chip/footer strings, `quit`, `pacURL`, `terminalCount`, `browserSuffix` and
  the identity keys); both catalogs hold the same 324 keys and no generated
  accessor is unused.
- Tests: `TestProxyPageCopiesTheProxyConfig`,
  `TestLogPageFiltersCopiesAndToggles`, `TestSettingsPageShowsLocalDataAndAbout`,
  `TestShortPathKeepsTheHomeShort`, the tailer tests (append, partial line,
  rotation, truncation, limit, window) and `TestOpenURLAllowsOnlyHTTP`; the
  shell, sidebar, language and trust tests follow the moved pages and the
  portal fields.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` wrote 98 images:
  both languages, the five dashboard phases, every page at 900×740, 720×560
  and in dark mode, plus the save bar and the log page. The binary ran under
  headless mutter and exited 0 with the tailer running (PAC port 1082 was
  already held by another process, as in phase 4). Interactive clicks remain
  phase 6.

### Phase 6: Verification and documentation

- [x] `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`,
      `reuse lint`.
- [x] Run the real window at 900×740 and at 720×560: switch every page, edit
      and save the gateway, hide and reopen with a dirty draft, connect and
      disconnect from the button and the tray, and answer at least one login
      panel. Follow `docs/agent-ui-testing.md`.
- [x] Rewrite `docs/design.md` for the one-window/left-navigation rules and the
      stateful button. Update `docs/configuration.md` for the accepted gateway
      input. Update `docs/agent-ui-testing.md` and the README first-run notes
      that still mention the settings window or the setup panel.
- **Status:** complete.

Notes:

- The checks pass: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint`.
- Real window: a test build renamed `gt-a11y` (so it gets its own
  single-instance lock and tray item), a temporary config and ports, the live
  GNOME Wayland session. Its accessibility tree drove every page, the save
  bar, the gateway edit and save, and the dirty draft across a hide (关闭 hides
  the window; the tray's 打开仪表盘 reopens it on the same draft). The power
  control and the tray (DBusMenu `Event`) both started a login and 取消 ended
  it. A throwaway local HTTPS gateway offering two methods produced the choice
  panel; 密码登录 was answered in the window, and the failed login reported its
  reason. The 720×560 page was verified with a probe build opened at 600×400:
  the compensated hint clamped it to a 720×560 page.
- Not exercised: a real online session (no gateway was available), so 断开
  while online and the tray's 切换账户 variants rest on the tests and the
  render pass; the tray icon's own click and panel remain the operator's
  check. The accessibility route is described in `docs/agent-ui-testing.md`.
- Two fixes came out of the pass. The gateway port was printed through ICU
  `{port, number}`, so a five-digit port read `38,265`; it is a string now, with
  `TestGatewayPortIsNotGrouped`. The minimum hint is a surface size on
  Wayland, and neither `Bounds` nor `ContentBounds` reports the CSD margin, so
  `internal/gui` now adds the measured 52 px shadow to the page minimum;
  the 600×400 probe confirmed the 720×560 page.
- The chip is back by request: `phaseText` draws the phase as a tone-tinted
  pill with a leading dot. `GOTRUST_RENDER_DIR` rendered all 98 states after
  the change; the five phases, dark mode, the panel and 720×560 were checked
  on the images.
- Docs: `docs/design.md` rewritten for one window, the chip and the save bar;
  `docs/configuration.md` and its English copy describe the gateway field and
  the page-based wording; `docs/agent-ui-testing.md` documents the AT-SPI and
  DBusMenu paths, the focus rule and the shadow margin; the README's first-run,
  page and tray text follow the one window.

### Phase 7 (optional): Traffic card

The mockup shows a Clash Verge-style traffic card; it was deferred with the
7/8/11/13 work and built on request, exactly as the mockup draws it.

- [x] Add byte counters to `core/proxy` (`trackedConn` reads and writes for
      SOCKS5 and CONNECT; a counting reader for the plain-HTTP request body and
      a counting writer for its response), expose a snapshot, and sample it
      once a second in the gui into a 60-point ring.
- [x] Add the dashboard card with the current up/down rates and two line series;
      use the mygo chart primitives or a small custom sparkline.
- [x] Tests for the counters and the ring; smoke the card in the real window.
- **Status:** complete.

Notes:

- The counters are `atomic.Uint64` on `Server`, not fields under `mu`: every
  proxied byte passes them, and `mu` guards the listener list that `Close` and
  `Listeners` use. `trackedConn.Read` counts up and `Write` counts down, which
  is the same client-side direction for SOCKS5 and CONNECT; the CONNECT
  pipeline buffer is counted explicitly because the HTTP server read it before
  the hijack. Plain HTTP counts the request body (a counting reader, which
  forwards `Close`) and the response body (a counting writer). `Traffic()`
  returns both totals; a closed server reports its last ones.
- The counting writer has to implement `Unwrap() http.ResponseWriter`:
  embedding `http.ResponseWriter` promotes only Header/Write/WriteHeader, so
  `http.NewResponseController` found no `Flush` and the streamed-response test
  failed. The wrapper now exposes the writer and the flush reaches the
  connection.
- `internal/gui/traffic.go` owns the sampler: one reading a second, a 60-point
  ring, and `TrafficState` published under `a.mu` for the window. A different
  server starts a new minute; a reading smaller than the one before does the
  same, because an address reused inside one sample looks like the same server
  with reset counters. A repeated reading of no session publishes nothing, so
  an idle window is not repainted every second.
- `ui/traffic.go` is the mockup's card: 流量, the 下行/上行 legends with their
  7 DIP dots and the newest rates, 最近 60 秒 at the right, and an 80 DIP
  sparkline whose download is the accent area (22% to 0) under its stroke and
  whose upload is the success line. Both lines share one scale and sit in a
  rect inset by 3 DIP so a flat line is not clipped. `trafficRate` prints
  binary units (B/s, KB/s, MB/s, GB/s, TB/s) with one decimal above a
  kilobyte.
- The dashboard now scrolls as the mockup's `.view-wrap` does; with three cards
  it is taller than a 720×560 window. The resource list lost its own `Scroll`
  and the three-row minimum (`resourceListMinHeight`, `resourceListMinRows`):
  the page is the one scroll area. `TestDashboardScrollsAtTheMinimumSize`
  replaces the old three-rows test, and the row-menu test renders at 900×740.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .`, `go tool mygo vet .` and `reuse lint` pass. `GOTRUST_RENDER_DIR`
  wrote 102 images; the dashboard at 900×740 and 720×560, light and dark, both
  languages, was checked on the images. The binary ran under headless mutter
  for five samples (`goTrust ready`, no GL errors, SIGTERM exit 0). No live
  gateway was available, so the live rates rest on the counter and sampler
  tests; the card's build and the sampler's idle loop were the real-window
  check.

### Phase 8: Mockup parity

The packaged build, compared with `/tmp/gotrust-mockup.html`, missed eleven
details. This phase adds them.

- [x] Title bar: the shield mark left of the brand.
- [x] Title bar: the light/dark switch at the right (`mygo.Theme.SetSource`).
- [x] Dashboard: the resource card's search sits in its header as soon as the
      card holds a row.
- [x] 账户: two cards, 连接 and 安全.
- [x] 设置: rows fill their card instead of stopping at a narrow column.
- [x] 启动, 保活 and 日志: switch rows, the whole label the target.
- [x] 日志: 清除 empties the file and its rotations.
- [x] 多账户: drop the footer hint the mockup does not have.
- [x] 设置: 项目主页 and 许可证 are clickable rows, not rows with a button.
- [x] Palette: the mockup's colours in both appearances, and the power
      control's fixed fills.
- [x] Window: transparent surface, content clipped to a 12 DIP radius.
- **Status:** complete.

Notes:

- `themeFor` rebuilds mygo's theme from the mockup's palette (`paletteFor`),
  so inputs, switches, selects and scrollbars follow the views. The title bar
  switch forces mygo's source and lasts for the run; it is not a config field.
- The window is created with `Transparent: true`; the view sets the frame root
  transparent and paints one column with `.Radius(12).Border(1, …).Clip()`, so
  the four corners are round while GTK keeps its shadow, resize edges and
  window controls. The render PNGs carry transparent corners; the real window
  starts without GL errors.
- Switch rows are one target: the row is the control and the track is drawn
  (34×20, 16 px knob). `ui.Switch` could not be reached by the tester's label
  lookup because its label text is a separate element; the row-level control
  also matches the mockup's label-wrapped switch.
- 日志's 清除 calls `logging.Logger.Clear`, which truncates under the logger's
  mutex and removes the rotations; the tailer restarts on the shrink and the
  page's tail is cleared at once. `TestClearTruncatesAndRemovesRotations`
  covers the file.
- The palette also moved the power fills to the mockup's `#16a34a`, `#d97706`
  and `#dc2626`; the chip keeps the semantic tints.
- Tests: the tab order now starts with the appearance switch, the account
  form asserts 连接/安全, the password button reads 修改…, the keep-alive
  test clicks the switch row, the About test clicks the rows, and the resource
  search test asserts the filter with any entry and none when empty.
  `go test ./...`, `go vet ./...`, `gofmt -l .` and `reuse lint` pass.
- Renders: 98 images again; the light and dark dashboard, account, settings,
  startup, logs and profiles were checked, including the 720×560 variants.
- Real window: the appearance switch flips its label (切换到浅色主题 →
  切换到深色主题), 清除 left the log at 0 bytes, and the account page reported
  the 连接/安全 headings.

### Phase 9: Mockup corrections after the trial

The packaged build still differed from the mockup in eight places. This phase
fixes them.

- [x] Every page form puts its labels above the controls.
- [x] 日志's toolbar, pane and footer are one card, so the right border closes.
- [x] 代理's PAC and Clash/Mihomo rows fill the card, their 复制 at the right.
- [x] 终端管理's 刷新 sits in the page heading, above the trust row.
- [x] 终端管理 shows the 0-count summary and both cards before a session.
- [x] 设置's language segment keeps its content width.
- [x] 关于 shows the shield and `goTrust <version>` centered, then the two
      link rows.
- [x] The title strip is 44 DIP, names only `goTrust`, and draws the
      minimize, maximize and close controls itself.
- **Status:** complete.

Notes:

- The title strip uses `HideWindowButtons`, so GTK hides its buttons while
  keeping the shadow, the rounded corners and the resize edges. The three
  controls call `Minimize`, `Maximize`/`Unmaximize` and `Close`; close goes
  through `OnClose`, which hides the window. The real window maximized to the
  work area and restored, closed and reopened from the tray, and minimized
  and came back through 打开仪表盘.
- `formColumn` and `formField` stack a muted caption over the control; the
  account, proxy, keep-alive, settings and account-name fields use them.
- 日志's card wraps the toolbar, the monospace pane and the footer; the header
  (详细日志, 清除, 打开目录) stays above it.
- 代理's copy section is two rows with the action at the right edge, not a
  form of fields.
- 终端管理 always draws its summary and two cards; offline the summary reads
  the 0 counts and each card holds the offline placeholder, with the refresh
  disabled. `TestTerminalsPageShowsEmptyCardsOffline` covers it.
- The language segment is aligned to the start so it sizes to its labels.
- 关于 centers the shield (48 DIP) and `goTrust <version>` and keeps the
  project/license rows; the version row is gone.
- Checks: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` and
  `reuse lint`; renders at 900 and 720 in both appearances.

### Phase 10: Copy and About cleanup

- [x] 保活's interval and target hints are gone; a bad or short interval is
      refused with the 10s minimum, without the 90s example.
- [x] 终端管理 loses its 随连接实时读取 caption.
- [x] 设置's language section loses its hint.
- [x] 关于 is its own page below 设置, with no 关于 heading inside.
- **Status:** complete.

Notes:

- `errKeepAliveInvalid` now takes the minimum (`config.MinimumKeepAliveInterval`)
  and both messages name the 10s floor: a malformed value reads
  "保活间隔不是有效时长，最短允许 10s". `TestKeepAliveIntervalValidation`
  covers 3s and a unitless 3.
- `PageAbout` sits between 设置 and 多账户; `pageLabel` uses the existing
  关于 string, the sidebar has its own info glyph, and `aboutPage` draws the
  centered shield and name over the card and the two link rows with no
  heading. The settings page keeps language and local data only.
- The render set gained `about` (900) and `page-about-*` (720, dark); the
  terminal page's 720 render shows the heading refresh and no caption.

### Phase 11: Panel design pass and About room

- [x] 关于's centered block takes more vertical room: a 72 DIP shield and
      padded name/version.
- [x] Every remaining panel follows the page design: captcha, code, SSO,
      choice, device binding and both password changes use the card face and
      the labels-above fields.
- [x] The captcha image is capped at 400 DIPs, so its card still fits the
      720 DIP window.
- **Status:** complete.

Notes:

- `panelCard` gives a login panel the page card's surface, border and radius;
  each panel keeps its heading, its fields (`formColumn`/`formField`) and its
  foot inside the card. The forced password change keeps its scrolling body
  with the foot pinned at the card's bottom.
- The About hero is 72 DIP with 40 DIP above and below; the two link rows
  follow under a divider.
- Renders: `main-captcha`, `main-code`, `main-choice`, `main-sso`,
  `main-password`, `settings-password` and `about` were checked at 900, plus
  the small and dark page sets.

### Phase 12: Captcha centering and About tagline

- [x] The captcha image is centered in its card.
- [x] The About hero takes more vertical room and adds a one-line tagline
      under the name and version.
- **Status:** complete.

Notes:

- The captcha box uses `AlignSelf(ui.Center)`; the click markers stay in the
  image's own coordinates.
- The About hero's padding rose to 56 DIP above and below, and the tagline
  (`aboutTagline`) sits under `goTrust <version>` in the muted ink. The
  project and license rows follow unchanged.

### Phase 13: Copy and structure parity audit

Every page, the title strip and the sidebar were compared again with the
mockup; the copy and structure gaps it found are fixed.

- [x] 仪表盘's resource card is 已授权应用, its group headings are a muted
      `group count`, and the filter reads 搜索应用或地址.
- [x] 账户's gateway label is 网关, the password placeholder is the mockup's
      sentence, 自动 is the automatic entry, and the form carries no permanent
      auth hint (only the discovering line).
- [x] 代理: 复制配置, SOCKS5/HTTP labels, Clash / Clash Script / Clash 配置,
      no caption; the two-empty rule appears as an error line only then.
- [x] 保活: no divider between the switch row and its fields.
- [x] 终端管理: 当前设备 and 在线/离线 chips, a four-column fact grid (two at
      720), and 取消信任本机.
- [x] 多账户: 名称; the delete action keeps the danger ink.
- [x] 设置: 设备标识.
- [x] Save bar: 有未保存修改, 放弃, 保存.
- [x] Password row: the check box sits on the input's baseline.
- [x] Removed the dead `fieldWidth`; the English device count is pluralized.
- **Status:** complete.

Notes:

- `terminalChip` draws the mockup's pill with `successWeak`/`surface2`; the
  current device and its online state are two chips, so a self row that the
  portal lists offline still says 离线.
- The render set at 900 and 720, light and dark, both languages, checked the
  changed pages; the resource group headings, chips, four-column grid, red
  delete, compact Clash picker and the copy card's single divider were
  checked on the images.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. The tests pin the new copy.
- Open, not changed: the traffic card (Phase 7); the log level picker and the
  level column (the logger writes no level); the toolkit's 14 px body, 32 px
  inputs and 6 px control radius against the mockup's 13/36/8; the startup
  labels stay 登录系统时启动 / 启动后自动连接 / 断线后自动重连（最多 5 次）
  because they describe the actual behaviour; the resource filter's visible
  placeholder stays mygo's 搜索 because `SearchField` fixes it (the accessible
  name is 搜索应用或地址).
- Decisions after the audit: traffic card stays deferred (Phase 7); the log
  keeps its plain-text tail; the toolkit metrics stay; the startup labels
  stay the accurate ones.

### Phase 14: The settings draft lives on the main thread

The audit found a data race. The GUI publishes `SettingsState` fields from
session and network goroutines under `a.mu`, while the frame reads the same
fields without it: `openWindow` copies `State` under `RLock`, then the view
reads the shared `*SettingsState`. `go test -race` reproduces it with a read
at `ui/pages.go:261` (`settings.Online`); the writers are `onSessionState`
(`internal/gui/gui.go:1099`) and the completions of `refreshSettingsTerminals`,
`setSettingsTrusted` and `submitSettingsPassword`.

- [ ] Give `app` a `runOnUI func(func())` seam, defaulting to
      `mygo.RunOnMain`; tests replace it with an inline call. (The seam is
      required: the GUI tests call `onSessionState` without `App.Run`, where
      `RunOnMain` would wait for a loop that never starts.)
- [ ] Publish `settings.Online` through the seam in `onSessionState`.
- [ ] Publish the three async completions through the seam, as `discoverAuth`
      already does; the busy flags they set before the goroutine stay on the
      main thread.
- [ ] Sweep for other `SettingsState` writes from goroutines; `setLogLines`
      and `discoverAuth` are already correct.
- [ ] Test: `onSessionState` runs its settings write through the injected
      seam; `go test -race ./internal/gui ./ui` is clean.
- **Status:** pending.

Notes:

- Once every writer is on the main thread, `a.mu` no longer guards the draft's
  fields; it stays for `a.view`, `a.cfg` and the two pointers.
- The seam's default is `mygo.RunOnMain` (fail-safe: a missed injection blocks
  in a test instead of hiding the race).

### Phase 15: The theme keeps the desktop's preferences

`themeFor` rebuilds from `ui.LightTheme()`/`DarkTheme()`, which bypasses
`Theme.follow`: the desktop's text scale and high-contrast adjustments are
lost. The docs' recipe is `t := *c.Theme()` and then override.

- [ ] `themeFor(c)` copies `*c.Theme()`, overrides the palette colours and
      keeps `FontSize`; under `c.Preferences().HighContrast`, mix
      `Border`/`TextMuted` toward `Text` as `follow` does.
- [ ] `View` calls `c.SetTheme(themeFor(c))`; `paletteFor(t.Dark)` still
      provides the app's own surfaces.
- [ ] Test: `ui.NewTester` with `SetPreferences(ui.Preferences{TextScale:
      1.5})` renders larger text; a unit test asserts the copied `FontSize`.
- **Status:** pending.

### Phase 16: The sidebar is a real tab list

`RoleTabList`/`RoleTab` promise a tab list, but every row is its own Tab stop
and the arrows do not move; mygo's own `Tabs` wraps the list in `FocusGroup`
(v0.7.2 `ui/base.go`).

- [ ] Add `.FocusGroup(ui.Vertical)` to the sidebar's tab column.
- [ ] Update `TestMainWindowTabOrder`: Tab enters the group at the selected
      tab and the next Tab leaves it; add a test that Down moves to the next
      tab and Enter activates it.
- **Status:** pending.

### Phase 17: Icon-only controls speak for themselves

- [ ] `.Tooltip(label)` on the window controls, the appearance switch, the
      power control and the resource row's menu glyph.
- [ ] The row glyph takes `.Role(ui.RoleMenuButton)`: `Element.Menu` promotes
      only `RoleAuto`/`RoleButton`, so it stays an image today.
- [ ] Test: Tab reaches the row menu and Enter opens it; the renders keep
      their look.
- **Status:** pending.

### Phase 18: A second launch brings the window forward

- [ ] Register `mygo.App.OnSecondInstance` before `App.Run`: restore, show and
      focus the window, keeping the page.
- [ ] Smoke: two launches under one headless session; the second exits and the
      first stays.
- **Status:** pending.

### Phase 19: The tray documentation matches Linux

- [ ] README and `docs/design.md`: a tray with a menu opens it on click;
      activation shows the window (a double click under AppIndicator), and
      `打开仪表盘` is the menu item that always works.
- **Status:** pending.

## Out of scope

- A memory-usage display or chart.
- A connections-count chart.
- A 设置 item in the tray.
- Changing the `core/atrust` protocol layer; only the proxy counters above
  touch `core/` and only in the optional phase.
- Reworking the resource row's two-click menu; the current row menu stays.
- Reusing chart paths and slices in the traffic card: 60 points once a second
  do not justify `MinMax`/`LineInto`/`StrokePolyline`.
- The 64×64 tray icon: the panel scales it.
- Moving the view's idempotent model syncs (`refresh`, `dropStaleMethods`,
  `logSeen`) out of the build; they do no I/O.
