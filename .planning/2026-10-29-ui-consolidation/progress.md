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

- **Status:** pending
- Not started. The shell, page state and single-window lifecycle are the first
  cut because every later page depends on them.

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
