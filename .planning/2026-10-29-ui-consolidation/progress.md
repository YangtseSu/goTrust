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

- **Status:** pending

### Phase 3: Demote profiles

- **Status:** pending

### Phase 4: Stateful play/stop control and tray

- **Status:** pending

### Phase 5: Page contents approved in the mockup

- **Status:** pending

### Phase 6: Verification and documentation

- **Status:** pending

### Phase 7 (optional): Traffic card

- **Status:** pending (optional; confirm before starting)
