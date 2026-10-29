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

### Phase 14: The settings draft on the main thread

- **Status:** complete
- mygo moved to v0.7.3 (`go.mod`, `go.sum`, `AGENTS.md`, `docs/design.md`,
  `docs/agent-ui-testing.md`); its UI package and `loop.go` match v0.7.2, so
  the audit's findings and phases 15-19 still hold.
- `app.runOnUI` is the seam: `Run` sets `mygo.RunOnMain`, `app.onUI` calls it,
  and tests install an inline call (`promptTestApp`). The fallback without the
  seam is `mygo.RunOnMain`, so a missed production assignment fails safe.
- `onSessionState` publishes `settings.Online` through the seam; the
  completions of `openSettingsPassword`, `refreshSettingsTerminals`,
  `setSettingsTrusted` and `submitSettingsPassword` do the same. The sweep
  found `storeChangedPassword` writing the draft's password from the login
  goroutine; it now publishes through the seam too. `discoverAuth` and
  `setLogLines` switched from calling `mygo.RunOnMain` directly to the seam.
- `setSettingsTrusted` now calls `refreshSettingsTerminals` on the main thread
  when the request succeeds, instead of starting it as a goroutine that would
  write the draft off-thread.
- The draft fields no longer take `a.mu` inside those closures; the pointer
  reads keep it, and `a.mu` still guards `a.view` and `a.cfg`.
- Tests: `TestSessionStatePublishesOnlineThroughTheUISeam`; the changed
  password test asserts its draft write goes through the seam. Before the
  change the suite's `go test -race ./internal/gui ./ui` stayed clean because
  no test overlapped a render with a session callback; the seam test pins the
  invariant instead.
- Real window: a test build under headless mutter with `auto_connect` and a
  temporary profile logged `goTrust ready`, started the login, published its
  failure through the seam and exited 0 on SIGTERM (the known Gtk-CRITICAL
  scale warning only).
- Checks: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`,
  `reuse lint`.

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

- **Status:** complete
- The dashboard is two cards on the window's own background: the session card
  (phase and facts left, 60 DIP power control right, a rule, then the four
  metrics) and the resources card (title, count, 打开默认应用 in the header).
  `card` is the shared bordered column; the chip, the subline, the footer with
  its tooltip and the dashboard copy menu are gone.
- `powerAppearance` maps each phase to the control's label, glyph and fill:
  连接 on accent, 取消 on amber with a spinner, 断开 on green with a stop
  square, 重试 on red with the exclamation. `powerControl` adds the hover (red
  for an ending action, `AccentHover` for a starting one) and the click
  (active phases log out; idle and failed log in). A busy start is disabled;
  a pending login keeps 取消.
- `sessionLine` is shared with the pre-login panels, so the phase stays above a
  question. The phase line is the reconnect ladder when it has started; the
  meta line is `网关地址 host:port`, the held time online, the failure detail
  in danger. `sessionFacts` replaced `sessionSummary`; `sessionEndpoint` and
  `sessionIntranet` are deleted.
- Metrics: 已分配地址 (`ClientIP`), SOCKS5 and HTTP from `State.Listeners`, 保活
  as the probe's age in its tone with `KeepAliveSummary` as the description. An
  absent value is `—`.
- Resources header: 打开默认应用 (disabled without `DefaultAppURL`). The copy
  menu, `controls`, `footer`, `footerContent` and `footerTip` are deleted;
  `CopyPAC`/`CopyClash` and `ClashFormat` stay on `Actions` for 代理 in phase 5.
- Tray: 未连接, separator, 打开仪表盘, 打开默认应用, separator, 连接/断开,
  optional separator + 切换账户, separator, 退出. `Actions.ShowWindow` is
  removed; the tray icon still calls `showWindow`, so it keeps the page.
  Catalog: `trayShow` → `trayDashboard`, `trayOpenApp` → `openDefaultApp`,
  `traySettings` removed; `retry`, `sessionGateway`, `metric*` and
  `keepAliveAgoValue` added; the generated accessors were rewritten.
- Tests: `TestPowerControlShowsThePhase` (label, glyph, fill, ending for all
  five phases), `TestButtonsCallTheRightAction` (重试 and 取消 included),
  `TestSessionMetricsShowTheSessionFacts` (values, dashes, description),
  `TestTrayMenuOrder` (one and two accounts), `TestTrayMenuOpensTheDashboard`,
  and the busy check in `TestTrayMenuOffersConnectWhenIdle`. The footer, chip
  and copy-menu tests were removed with their subject.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. `GOTRUST_RENDER_DIR` rendered every
  dashboard phase, the long grant at 900×740 and 720×560, dark mode and both
  languages; the images show the two cards, the metric values and the spinner.
  A throwaway pixel probe confirmed the hover fill (断开 green → `Danger`,
  连接 accent → `AccentHover`). The binary ran under headless mutter
  (`goTrust ready`, SIGTERM exit 0; the PAC port 1082 was already held by
  another process). Interactive clicks and tray behaviour remain phase 6.

### Phase 5: Page contents approved in the mockup

- **Status:** complete
- 代理 owns the copy actions again: the PAC address with a 复制 disabled while
  `State.PACURL` is empty, and a Clash/Mihomo select (Clash 片段, Verge 脚本,
  Clash 完整配置) with its own 复制. `SettingsState.ClashFormat` keeps the
  choice; the buttons read 复制 with 复制 PAC 地址 and 复制 Clash 配置 as
  accessible names.
- 日志 tails the current file: `internal/gui/logtail.go` polls once a second,
  reads only appended bytes, holds a partial line until its newline, restarts
  after a rotation or truncation, and keeps 500 lines. The hand-off to the page
  runs on the main loop, so the render never reads a slice being replaced. The
  page has 详细日志, 打开目录, a case-insensitive filter, 自动滚动, 复制 and a
  monospace pane with the file and 最后 500 行 under it. A level filter was
  planned but not shipped: the log lines carry no level, so it would filter
  nothing; 详细日志 is the level control and the search field is the filter.
- 设置 holds the language (moved off 启动), 本机数据 (the data file with the
  resolved default path as its placeholder, the device id and 打开配置文件)
  and 关于 (version, project home, license); both 关于 buttons open their pages
  through the same http/https guard as a granted resource. The client identity
  line and the verbose/log cards are gone.
- 启动 is the four toggles only.
- 终端管理 shows the portal's endpoint fields: type, OS, browser, last login,
  login address (LAN as 内网), network zones and device id, in a wrapping grid;
  the summary counts trusted endpoints against the device limit and names the
  self trust state.
- Catalog: 27 keys added, 25 dead ones removed; both catalogs hold 324 keys and
  the build carries no unused accessor.
- Tests: the proxy copy actions, the log filter/copy/verbose/open-dir, the
  settings sections and both links, the four-toggle 启动 page, the endpoint
  fields and the tailer (append, partial line, rotation, truncation, limit,
  long-file window); the shell, sidebar, language and trust tests follow the
  moved pages.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint` pass. The render pass wrote 98 PNGs across both
  languages, all five dashboard phases, every page at 900×740 and 720×560,
  dark mode, the save bar and the log page. The binary ran under headless
  mutter with the tailer and exited 0; the PAC port 1082 was already held by
  another process, as in phase 4. Interactive clicks and tray behaviour remain
  phase 6.

### Phase 6: Verification and documentation

- **Status:** complete
- Checks: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` and
  `reuse lint` pass.
- Real window: a test build renamed `gt-a11y` (its own single-instance lock and
  tray item; the installed app stayed untouched), a temporary config and
  ports, the live GNOME Wayland session. AT-SPI drove the interface: every
  page tab opened its heading; the 账户 gateway edit raised the save bar, 保存
  wrote `address = "vpn2.example.com"` / `port = 8443` and the toast; a second
  edit survived 关闭 and the tray's 打开仪表盘, and 放弃修改 restored the stored
  value. 重试 started a login and 取消 ended it (连接中 → 未连接, toast 已取消);
  the tray's 连接 started the saved-config login the same way. A throwaway
  local HTTPS gateway answered `authConfig` with two methods: the choice panel
  appeared, 密码登录 was answered, and the failure was reported (状态：失败)
  after the fake returned 500 for `/public/manifest`.
- The 720×560 minimum: GTK applies the hint to the surface (a 560 hint gave a
  508 page), and mygo's `Bounds`/`ContentBounds` report no margin on Wayland.
  `internal/gui` now adds the measured 52 px CSD shadow to the page minimum; a
  probe build opened at 600×400 was clamped to a 720×560 page.
- Fixes found by the pass: the gateway port was ICU-formatted (`38,265` for
  38265) and is now a string (`TestGatewayPortIsNotGrouped`); the chip was
  restored per the review (`phaseText` draws the tinted pill and its dot).
- Not exercised: online 断开 (no live gateway) and the tray icon's own click
  and panel; the shell's icon exposes no action, and the DBusMenu path used
  above is what a script reaches. Both remain the operator's tray check.
- Docs: `docs/design.md` rewritten (one window, chip, save bar, minimum);
  `docs/configuration.md` and `.en.md` (gateway field, page wording);
  `docs/agent-ui-testing.md` (AT-SPI, DBusMenu, focus, shadow); README
  first-run, page table, tray and log references.
- Render: `GOTRUST_RENDER_DIR` wrote 98 images after the chip change; the five
  dashboard phases, dark mode, the panel and 720×560 were checked.

### Phase 13: Copy and structure parity audit

- **Status:** complete
- Compared every page, the title strip and the sidebar with the mockup.
- Fixed: 已授权应用 and muted group counts; 搜索应用或地址 in the accessible
  filter name; 网关; the password placeholder; 自动 without a permanent hint;
  复制配置, SOCKS5/HTTP, Clash / Clash Script / Clash 配置, and no proxy
  caption (the two-empty rule is an error line); the keep-alive divider;
  当前设备/在线 chips and the four-column fact grid; 名称; danger 删除;
  设备标识; 取消信任本机; 有未保存修改 / 放弃; the check box's baseline; the
  dead `fieldWidth`; the English `{# device}` plural.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .`, `reuse lint`; 102 renders at 900/720, light/dark, zh/en.
- Decisions: the traffic card stays deferred (Phase 7); the log stays a
  plain-text tail; the toolkit metrics stay; the startup labels keep their
  accurate wording.

### Phase 12: Captcha centering and About tagline

- **Status:** complete
- The captcha image is centered in its card; the click mapping is unchanged.
- The About hero is taller (56 DIP padding) and adds the tagline
  "aTrust 桌面客户端 · 本地 SOCKS5 / HTTP 代理" under the name and version.
- Verification: full checks and the About/captcha renders.

### Phase 11: Panel design pass and About room

- **Status:** complete
- About: the hero is 72 DIP with vertical padding, then the link rows.
- Panels: `panelCard` wraps the captcha, one-time code, SSO, method chooser,
  device binding, forced password change and the voluntary password change;
  each uses the labels-above fields and the foot inside the card.
- Captcha: max width 400 so the card fits the minimum window.
- Verification: full checks and the panel/About renders.

### Phase 10: Copy and About cleanup

- **Status:** complete
- 保活: removed both hint captions; `errKeepAliveInvalid` takes the minimum
  and the invalid/short messages name 10s, never 90s; a unitless or short
  value is covered by `TestKeepAliveIntervalValidation`.
- 终端管理: the 随连接实时读取 caption is gone; the heading refresh and the
  offline cards remain.
- 设置: the language hint is gone; the page is language + local data only.
- About: a separate page below 设置 with its own info glyph; inside, no 关于
  heading, a centered shield and `goTrust <version>`, then the project and
  license rows.
- Verification: full checks, renders including the new 关于 page.

### Phase 9: Mockup corrections after the trial

- **Status:** complete
- Forms: `formColumn`/`formField` put every page label above its control.
- 日志: the toolbar, pane and footer are one card; the head (详细日志, 清除,
  打开目录) stays above it.
- 代理: the PAC and Clash/Mihomo rows fill the card with their 复制 at the
  right; the listeners keep the filled form.
- 终端管理: 刷新 moved to the page heading; the summary and both cards are
  drawn before a session with 0 counts and the offline placeholder; the
  refresh is disabled offline.
- 设置: the language segment keeps its content width; 关于 centers the shield
  and `goTrust <version>` above the project/license rows.
- Titlebar: 44 DIP, `goTrust` only, and its own minimize/maximize/close via
  `HideWindowButtons`; verified in the real window (maximize/restore,
  close/reopen via the tray, minimize and return).
- Verification: full checks, renders at both sizes and appearances, and the
  offline terminal test.

### Phase 8: Mockup parity

- **Status:** complete
- Title bar: the shield mark, the brand and the appearance switch. The switch
  calls `mygo.Theme.SetSource`; `themeFor` paints mygo's widgets with the
  mockup's palette.
- Dashboard: the resource search moved into the card header, one line at
  900 and wrapping only when it must; it exists as soon as there is a row.
- 账户 is two cards (连接, 安全); 设置's cards and rows fill their width; 启动,
  保活 and 日志 use switch rows; 日志 gained 清除; 多账户 lost its footer hint;
  项目主页/许可证 are link rows with the arrow and no button.
- The dark palette is the mockup's (`#1e2025` content, `#23262c` cards,
  `#1a1c21` sidebar, `#77a8fb` accent, `#5cc98a` success ...); the power
  control uses the fixed `#16a34a/#d97706/#dc2626` fills.
- The window is transparent and the root column clips its children to a
  12 DIP radius; render corners are alpha and the real window starts cleanly.
- `logging.Logger.Clear` truncates and removes rotations;
  `TestClearTruncatesAndRemovesRotations` covers it.
- Verification: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` and `reuse lint`; 98 renders; the real window's toggle and
  clear were exercised.

### Phase 7 (optional): Traffic card

- **Status:** complete
- `core/proxy` counts the bytes the client side moves: `trackedConn.Read` is
  upload, `Write` is download (SOCKS5 and CONNECT share that shape), the
  CONNECT pipeline buffer is counted where the hijack reads it, and plain HTTP
  counts its request and response bodies. `Server.Traffic()` reads two
  atomics.
- `internal/gui/traffic.go`: a one-second sampler into a 60-point ring,
  published as `ui.TrafficState`. A new server, or a reading smaller than the
  previous one (an address reused inside one sample), starts a new minute; an
  idle app publishes no new point.
- The dashboard card is the mockup's: 流量, the 下行/上行 legends with 7 DIP
  dots and the newest rates, 最近 60 秒, and an 80 DIP sparkline with the
  download's accent area fading to nothing and the upload's success line, both
  on one scale.
- The dashboard scrolls like the mockup's view: the resource list lost its own
  scroll and three-row minimum. The .view-wrap scroll is the page scroll.
- The counting writer needs `Unwrap()`: embedding `http.ResponseWriter` does
  not promote `Flush`, and the streamed-response test caught the missing
  flush.
- Verification: full checks, 102 renders (both languages, 900 and 720, light
  and dark), five seconds of the real binary under headless mutter with a
  clean exit. No live gateway was available; live rates rest on the counter
  and sampler tests.

## Session: mygo usage audit (2026-10-29)

Read all 116 pages under the mygo checkout's `docs/` (about 24k lines)
in five parallel readers, then checked every API the app uses against the
pinned v0.7.2 source. The checkout is v0.7.3-11; `FocusGroup`,
`OnSecondInstance`, `Tooltip`, `SwitchBase`, `Menu`, `chart` and `Handle` all
exist in v0.7.2, so the findings hold for the pin.

Verdict: the architecture follows the documented model (views are functions
of state, no stored build values, `mygo vet` clean, guarded state plus
`Invalidate`, the tray hide-on-close pattern, the drawn title bar, i18n and
drawing). One real data race and five smaller deviations were found; they are
Phases 14-19.

- The race: `SettingsState` fields are written from session and network
  goroutines under `a.mu`, while the frame reads them without it. Reproduced
  with `-race` (read at `ui/pages.go:261`).
- `themeFor` rebuilds from `ui.LightTheme()`/`DarkTheme()`, dropping the
  desktop text scale and high contrast that `Theme.follow` applies.
- The sidebar declares `RoleTabList`/`RoleTab` without `FocusGroup`.
- Icon-only controls lack a tooltip; the resource row's menu glyph stays
  `RoleImage`.
- No `OnSecondInstance`, so a second launch does not show the window.
- The README says a tray click opens the window; on Linux a click opens the
  menu and activation (a double click) is `OnClick`.
- Not planned: reusing chart paths (60 points at 1 Hz), the 64×64 tray icon,
  and moving the view's idempotent model syncs out of the build.
