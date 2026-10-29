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

## Mockup caveats

- The mockup uses a 900×740 window; the real mygo window draws its own 34 DIP
  title strip. The sidebar starts below that strip.
- The traffic card in the mockup is illustrative only; it is not part of the
  approved phases until Phase 7 is confirmed.
- The mockup's search, filters and theme switch are HTML-only; the app uses
  mygo widgets and the desktop theme.
