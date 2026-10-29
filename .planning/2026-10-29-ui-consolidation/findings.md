# Findings: one window, left navigation

Everything here is read from the repository or measured in the two local
mockups. The mockups are design references, not repository files:
`/tmp/gotrust-mockup.html` (the full layout) and
`/tmp/gotrust-status-variants.html` (the status/button study).

## Why the architecture changed

The current interface is two windows:

- `internal/gui/gui.go` owns `window` and `settingsWindow`; `openSettings`
  creates the second one, `hideSettings` discards its draft on close, and
  `invalidateSettings` invalidates it separately.
- `ui/settings.go` gives the settings window six tabs (`settingsTabs`), a
  `profileBar` on every page and a `connectionSection` that edits the profile
  name beside the account fields.
- The main window's `controls` (`ui/view.go`) duplicates 复制配置 and 打开日志
 目录 in its `⋯` menu; the tray repeats profile switching and settings.
- The first-run form is a separate `SetupDraft`/`setupPanel` in the main
  window (`ui/setup.go`), duplicating address/port/username/password.

The reported friction follows directly: settings needs two clicks, the home
page carries actions that belong to settings, profiles are the first thing in
the settings window, and the tray puts 显示窗口/设置 below the session and
profile controls.

## The approved layout

One window, 900×740 (min 720×560), one title strip, a 208 DIP sidebar and a
680 DIP content column.

```
仪表盘
────────
账户
代理
保活
启动
终端管理
日志
设置

...

多账户  账户 A
```

- The dashboard is the session page: status card with a 60 DIP play/stop
  button at the right of its first row, then
  `网关地址 host:port · 已保持 duration`, then the four metrics
  (已分配地址, SOCKS5, HTTP, 保活), then the resources card.
- The sidebar has no settings window and no tab bar. `设置` is a page.
- `多账户` sits at the bottom, subdued, with the live account as a tag.
- A global save bar appears at the bottom of the content only while the draft
  differs from the file. The draft survives hiding the window.

## The status/button study

Five combinations were rendered:

| Variant | Status carrier | Result |
|---|---|---|
| A | status chip + blue button | current; the chip and the button say different things |
| B | button colour + icon + phase text | chosen; one control carries state and action |
| C | separate shield icon + button | brand-like, but two icons compete |
| D | coloured ring around a neutral button | elegant, but a 3 px ring is thin and needs custom drawing |
| E | small dot + button | too weak for the primary status |

B's phases:

- offline: blue, play triangle, 连接;
- connecting: amber, spinner, 取消;
- online: green, stop square, 断开;
- reconnecting: amber, spinner, 取消, ladder/countdown in the meta line;
- failed: red, exclamation, 重试, detail in the meta line.

The icon, the phase text and the colour all encode the state, so colour is not
the only channel. `ui.Spinner` exists in mygo 0.7.2; the play/stop/exclamation
icons are new SVGs in `ui/icons.go`.

## Facts behind the page contents

- **Gateway input.** The store keeps `Server.Address` and `Server.Port`
  separately (`core/config/config.go`). The UI will hold one
  `host:port` buffer and parse on apply; the file schema stays. The accepted
  forms are `https://host:port`, `host:port`, `host` (443), bracketed IPv6 and
  a trailing `/`; other schemes and a port outside 1–65535 are errors.
- **Save password.** `config.NewProfile` currently leaves
  `AuthConfig.StorePass` false; the approved default for a new account is true.
  Existing profiles keep their stored value.
- **Version and license.** `internal/version.Version` is the release string;
  a source build says `devel` and the release workflow sets it with `-X`.
  The project URL is `github.com/YangtseSu/goTrust`; the repository is
  AGPL-3.0-or-later and the release copies `LICENSE` beside the binary.
- **Logs.** `core/logging.Logger.Path()` and `Dir()` expose the current file;
  the logger writes 0600 and redacts. A read-only tailer can poll the file
  without touching the logger. Five hundred lines and a one-second poll are
  enough for the page; rotation and truncation need a reopen.
- **Terminals.** `auth.TerminalInfo` carries `DeviceName`, `DeviceType`, `OS`,
  `OSVersion`, `Browser`, `OnlineStatus`, `LastLoginAddress`, `LastLoginIP`,
  `LastLoginTime`, `NetworkZoneList`, `DevDBID` and `ID`; `auth.Terminals`
  adds `SelfID`, `DeviceTrusted`, `CurrentTrustDeviceCount`,
  `MaxDeviceCount`, `DeviceTrustedByOne`. The current window shows only name,
  state, device string, location/IP and time; the approved page shows the
  user-facing fields and the counts.
- **Session facts.** `session.State` has `Phase`, `Detail`, `ClientIP`,
  `KeepAlive`, `Reconnect`, `Attention` and `Since`. The dashboard can build
  the phase, the gateway and the duration without new session state.
- **Traffic card.** `core/proxy` tracks connections in `Server.conns` but
  counts no bytes. A traffic card would need counters on `trackedConn` for
  SOCKS5/CONNECT and a counting writer for plain HTTP, plus a sampler. It is
  left optional; the mockup shows what it would look like.
- **Charts.** mygo 0.7.2 ships `ui/chart` (`Plot`, `Line`, `Area`, scales); a
  sparkline is feasible if Phase 7 is approved.

## Tray

Current `TrayMenu` order: status, separator, toggle, profiles, separator,
show, openApp, settings, separator, quit. The approved order is:

```
status
────────
打开仪表盘
打开默认应用
────────
连接/断开
────────
切换账户 ▸        (only when more than one account)
────────
退出
```

- `trayShow` becomes 打开仪表盘 / Open Dashboard.
- `trayProfiles` becomes 切换账户 / Switch Account; the group is omitted for a
  single account, including its separator.
- No 设置 item. The settings page is one click inside the window; the tray
  stays a session surface.
- Tray icon click shows and focuses the window and keeps the last page;
  打开仪表盘 switches to the dashboard. 打开默认应用 is disabled without a
  launch URL; 连接 is disabled while busy.

## Phase 1 facts

- A mygo Row centers its children vertically; the shell row and the sidebar's
  own row need `AlignItems(ui.Stretch)` or the sidebar and the page float in the
  middle of the window.
- The captcha is drawn at `DisplaySize` (up to 440×200) and maps clicks against
  that size. The declared 720 minimum leaves a 464 DIP content column, so the
  cap still fits; a narrower test window clipped the image and skewed the click
  mapping.
- The page headings repeat the sidebar labels by design; `tester.Click(label)`
  reaches the sidebar first because the sidebar is built before the content.
- The draft is a pointer in `State.Settings`; every page renders it in place,
  so hiding the window (the close handler only calls `Hide`) no longer runs
  code that discards it.

## Phase 2 facts

- `ParseGateway` (ui/account.go) returns `(address, port, problem)`. An empty
  text is not a problem: it is an account without a gateway, the port is 0 and
  the caller keeps the stored one. That keeps "save the proxy settings before
  you know the gateway" working. `https://` is the only scheme; a path or a
  trailing slash is cut; a bracketed IPv6 address may carry a port, and a bare
  address typed without brackets is accepted as the host with 443.
- `gatewayText` is the inverse: `host:port`, IPv6 bracketed, empty for a profile
  without an address. The field shows `vpn.example.com:443` rather than the
  file's two keys.
- `Apply` folds every profile's gateway, not only the selected one: the account
  page edits one profile at a time, and the old positional buffers only folded
  the selected profile, which would lose the first edit after a switch.
- `Changed` has to check the text separately: a gateway that does not parse
  folds to nothing, so the draft would look clean and the save bar (and the
  reason) would never appear.
- `config.NewProfile` sets `StorePass`, and `normalise` does not: a loaded
  profile keeps the value in its file, so an upgrade cannot silently start
  storing passwords for accounts that never opted in.
- The window's 连接 and the tray's 连接 are different actions. `saveDraftForLogin`
  writes the draft when `Changed()` (through `writeDraft`, the same path 保存
  uses) and selects 账户 on a rejection; `startSavedLogin` is the saved-only
  path the tray calls. An incomplete saved profile from either path selects
  `账户` and reports `ProfileProblem`.
- The password row is `Row{ TextInput.Grow(1).Shrink(1), Checkbox }` capped at
  `fieldWidth`; the placeholder is the second-factor hint, so the row needs no
  caption of its own.

## Mockup caveats

- The mockup uses a 900×740 window; the real mygo window draws its own 34 DIP
  title strip. The sidebar starts below that strip.
- The traffic card in the mockup is illustrative only; it is not part of the
  approved phases until Phase 7 is confirmed.
- The mockup's search, filters and theme switch are HTML-only; the app uses
  mygo widgets and the desktop theme.

## Phase 5 facts

- **No level in a log line.** `core/logging` decorates every line with the
  standard logger's `log.LstdFlags` and the message; the protocol layer adds no
  level, and `SetVerbose`/`SetProtocolDebug` decide whether the debug lines
  exist at all. A 全部/信息/警告/错误 filter therefore has nothing to classify;
  the page uses the search field as its filter and 详细日志 as the level
  control. A real level filter needs a level source in `core/logging` first.
- **SearchField's placeholder is fixed.** `ui.SearchField` builds its own
  input with the catalog's generic 搜索/ Search placeholder and ignores a
  later `.Placeholder`; the filter's name is its accessible label
  (`Filter the log`), which is what a screen reader announces.
- **The Tester cannot see an input's placeholder.** `Texts()` lists labels and
  drawn strings, not placeholder text, so the data-file default path is checked
  on the rendered image and `shortPath` is unit-tested instead.
- **Rotation and truncation are two signals.** `os.SameFile` separates the
  rotated file from its replacement even when the new file is already longer
  than the old offset; a smaller size is the truncation signal. A truncation
  that regrows past the old offset between two polls is not distinguishable
  from an append, but the logger rotates by rename, so only a manual truncation
  can hit it.
- **The tail hands over on the main loop.** Rendering and the assignment of
  `SettingsState.LogLines` both run through `mygo.RunOnMain`, so the page never
  reads a slice header the tailer is replacing.
- **A closed Select shows its choice.** The Clash format picker draws the
  chosen option's text, so the copy test can assert the format without opening
  the menu.
- **The summary line needs an explicit separator.** The trusted/temporary
  counts are one locale string; the self-trust state is a second node, so the
  page prefixes it with `· ` rather than relying on the row gap.

## Phase 6 facts

- **The minimum hint is a surface size.** GTK 4 applies the geometry hint to
  the whole CSD surface on Wayland: a 560 hint clamped the frame to 560 and
  left a 508 page. The shadow measured 26 px on the sides and 23/29 top and
  bottom (952×792 surface for a 900×740 page). mygo's `Bounds` and
  `ContentBounds` report no margin on Wayland (`frameExtents()` returns zero),
  so a page minimum needs the shadow on top of the hint: 720×560 → 772×612,
  verified with a 600×400 probe.
- **Focus is not a property a background launch controls.** `gtk_window_present`
  from a process started outside the session does not activate the window;
  GNOME's focus stealing prevention leaves the keys with the shell. Alt+Tab
  from a synthetic keyboard focuses it, after which Ctrl+Enter and Alt+F8
  arrive.
- **Synthetic input needs declared codes.** The kernel drops key events for
  codes not set with `UI_SET_KEYBIT` (the first probe declared only Alt/F8, so
  Ctrl+Enter and Tab vanished). A mouse device with REL axes is classified as
  a mouse and its ABS axes move nothing; a tablet is rejected without
  `ABS_X`/`ABS_Y` resolution (`libinput bug: missing tablet capabilities`).
- **The tray menu is reachable without the shell.** libayatana-appindicator
  exports `com.canonical.dbusmenu` at
  `/org/ayatana/NotificationItem/<name>_<pid>_1/Menu` on the app's unique bus
  name; `GetLayout` and `Event(id, "clicked", …)` drive it. The shell's own
  indicator button exposes no Action.
- **ICU groups a port.** `{port, number}` turns 38265 into `38,265`; a port is
  an identifier and is passed as a string. `TestGatewayPortIsNotGrouped` pins
  it.

## Phase 8 facts

- **The appearance switch is a mygo source.** `mygo.Theme.SetSource` accepts
  `ThemeLight`/`ThemeDark`/`ThemeSystem` and changes the whole application,
  including GTK and mygo's own palette. The app still has to call
  `Context.SetTheme(themeFor(dark))` to paint the mockup's colours.
- **Transparent + Clip gives four rounded corners.** GTK rounds a CSD window's
  top only. A window created with `Transparent: true`, a frame root set to
  `ui.Transparent` and one column with `.Radius(12).Border(…).Clip()` leaves
  the corners transparent while GTK keeps the shadow and the window controls.
  The render PNG's corner pixels are `srgba(0,0,0,0)`.
- **A tester click finds the label first.** With a separate text label and a
  `ui.Switch` sharing a name, `Tester.Click` clicks the text and nothing
  happens. A row-level button (role switch, checked state, drawn 34×20 track)
  is both testable and what the mockup's label-wrapped switch does.
- **The mockup's power fills are not its tone colours.** The semantic colours
  are text marks (`#15803d`/`#5cc98a`), while the power button uses fixed
  `#16a34a`, `#d97706` and `#dc2626` in both appearances.
- **The chip's mix survives the palette change.** `toneInk` still walks the
  fill toward the far end until it clears 4.8:1 on the chip's tint; the new
  accent/success/warning/danger values keep `TestToneInkPassesAA` green.

## Phase 9 facts

- **`HideWindowButtons` keeps the frame, not its buttons.** A window with
  `TitleBarStyle: TitleBarHidden` and `HideWindowButtons: true` keeps GTK's
  shadow, rounded corners and resize edges, and reports no title-bar insets;
  the view draws its own 44 DIP strip and calls `Minimize`,
  `Maximize`/`Unmaximize` and `Close`. Close still runs `OnClose`, which hides
  the window.
- **Labels above need a container, not `ui.Field`.** `ui.Field` is a label
  column; `formColumn`/`formField` stack a muted caption over the control, as
  the mockup draws the forms.
- **The frame reports the content extents.** With the frame buttons hidden the
  accessible frame is the 900x740 page, not 952x792; maximize took it to the
  work area and restore brought it back.
- **A content-sized segment needs `AlignSelf(Start)`.** In a card column a
  `ui.Segmented` stretches unless it is told to sit at its own width.
- **Offline terminal cards are a state, not a different page.** Drawing the
  summary and both cards with zero counts keeps the page's shape; the refresh
  is disabled until a session exists.

## Phase 10 facts

- **A validation message states the constraint, not the default.** The
  invalid-interval message now takes `MinimumKeepAliveInterval` and reads
  "最短允许 10s"; the 90s default is a placeholder, not a rule.
- **A page below 设置 is a new Page.** `PageAbout` shifts `PageProfiles`'
  value but no state is persisted by number; `pageLabel` reuses 关于 for the
  sidebar and `aboutPage` draws no heading, so the page body never repeats it.

## Phase 11 facts

- **Panels fit the card only after a cap.** The captcha image was 440 DIPs
  wide; a 16 DIP card padding in a 464 DIP content column left 432, so the
  cap is 400. The card keeps the image inside at the minimum window.
- **The forced password panel keeps its scroll.** The card grows and shrinks
  inside the panel slot, the body scrolls, and the foot stays at the card's
  bottom.
