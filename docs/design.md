# Interface rules

The rules the window follows, and the reasons they exist. They are decisions,
not preferences: a change that breaks one of them needs a phase in the plan and
an entry in its decisions table.

## One window, one page at a time

* The window is the app: a 900×740 page area by default, a 720×560 minimum.
  GTK counts the window's own shadow margin in the minimum hint, so the hint
  carries that margin on top of the page minimum (`windowShadowWidth`,
  `windowShadowHeight` in `internal/gui`). The sidebar is 208 DIPs wide, the
  page column stops at 680, and every form control stops at 360.
* The title strip spans the window: the brand and the live account
  (`goTrust · 账户 默认`). The desktop's own window controls live in it; a
  close hides the window to the tray, and quitting stays in the tray menu.
* The sidebar is the only navigation: 仪表盘, a separator, 账户, 代理, 保活,
  启动, 终端管理, 日志, 设置, then a spacer, a separator and 多账户 at the
  bottom. The active page is tinted with the accent; the rows are tabs for
  assistive technology, and Enter or Space activates them.
* 多账户 is quieter than the pages above it: a subdued row whose tag is the
  live account, so the account list does not read as one more step of the
  settings pages.
* One page is mounted at a time. A page of forms scrolls when the window is
  shorter than the form; the shell's bars stay in place.

## The session is the headline

* The dashboard is the session page: the phase as a chip (a tone-tinted pill,
  a dot in the phase's colour and the label in the theme's text colour) and
  the 60 DIP play/stop control at the right of the same row.
* The chip's colour is the phase: accent while connecting, warning while
  reconnecting, success online, danger failed, muted when idle. The dot
  repeats the colour, so colour is never the only channel; the label names
  the phase and the control carries the action.
* The control is the one primary action on the page: a play triangle and
  连接 when idle or failed (重试), a spinner and 取消 while connecting or
  reconnecting, a stop square and 断开 online. The hover fill is red for an
  ending action and a darker accent for a starting one.
* One meta line under the chip: `网关地址 host:port`, the time held when
  online, the failure detail in the danger tone, or the reconnect ladder
  (attempt and countdown) while it waits. A port is an identifier: it is
  printed as typed, never with a thousands separator.
* A four-cell metrics row follows (已分配地址, SOCKS5, HTTP, 保活). A value the
  session does not have is `—`. The keep-alive cell carries the last probe's
  summary in its description, so the row stays one line.
* The resources card is the second card: its title, the count and
  打开默认应用 in the header, then the granted list. 打开默认应用 is disabled
  while the grant has no launch address.
* The session line is shared with the pre-login panels, so a question never
  hides the phase it belongs to.

## One primary action per screen

* Exactly one filled button per screen: the play/stop control on the
  dashboard, 保存 in the save bar, 提交 in a panel's foot. Every other action
  is a plain button, a text button or a menu entry.
* One owner per action. 代理 owns the PAC address and the three Clash/Mihomo
  renderings with their 复制 buttons; 日志 owns 打开目录 and 复制; the dashboard
  owns 打开默认应用 and nothing else. The tray repeats none of them.
* A destructive or abandoning action is not a variant of the primary one.
  取消登录 sits on the left of a panel's foot (`panelActions`), 提交 on the
  right, and the corrections 撤销 and 清空 are face-less text buttons between
  them.
* A resource row's actions go behind one menu (`Element.Menu` on the dots
  glyph), not two visible buttons per row.

## A login never needs a second window

* Captcha, one-time code, SSO, device binding, the method chooser and the
  forced password change are panels inside the window (`panelBody`), never
  dialogs and never windows of their own.
* A panel owns the content wherever the sidebar points, and the sidebar dims
  and disables while it is up. Esc is its cancel; the selected page returns
  when the panel closes.
* 账户 is the first-run form and the only editor of the account. A fresh
  install opens on the dashboard; 连接 selects 账户 with the reason when the
  saved account is incomplete.
* A panel is at most `panelMaxWidth` (560) wide: a maximized window must not
  stretch three fields across the screen.

## Semantic colour is a mark, not a fill

* A tone is meaning, not a hex value (`tone`, `toneColor`, `toneInk`). The raw
  mygo fill is a Meter colour: it measures 2.8-4.4:1 as text on the chip tint,
  so it is never the text.
* Text that has to be read uses `Theme.Text` on the tint, or `toneInk` on the
  window. `toneInk` mixes the fill toward the far end of the appearance until
  it clears 4.8:1 against the chip's tint; `TestToneInkPassesAA` asserts it in
  both appearances, so a theme change cannot slip a failing colour back in.
* The phase's fill appears on the chip (its tint and its dot) and on the power
  control, nowhere else as a value.
* A healthy probe is not news: only a warning or a failure colours the
  metrics row.

## Four text sizes, one spacing grid

* Every size comes from `textSize` on the theme's own font size: heading,
  panel heading, body, caption. No view hardcodes a size; a desktop font
  setting moves all four together.
* Padding, gaps and control sizes are multiples of `Theme.Spacing`
  (`Theme.Space`), so a compact or roomy theme stays consistent. The shell
  adds the fixed 24 DIP window padding, a 16 DIP gap between groups and an
  8 DIP gap inside one group.
* A row whose label can outgrow the line wraps instead of clipping; a long
  account name ellipsizes in the sidebar tag instead of squeezing 多账户.

## A draft, a save bar and a toast

* The settings draft is created at startup and kept for the life of the
  process. Hiding the window, switching pages and opening the tray keep the
  edits; a profile switch, 保存, 放弃修改 and a login that saves the draft
  are what reload it.
* The save bar appears at the bottom of the content only while the draft
  differs from the file: 未保存的修改, 放弃修改, 保存. Applying it writes the
  file, moves or removes the stored client data the draft renamed or deleted,
  and reloads the draft from the result.
* A result is a toast once, not a line that stays: `statusToast` reports
  copy, save, profile and autostart outcomes once per sequence. The session
  state and the settings draft each have their own sequence
  (`State.StatusSeq`, `SettingsState.StatusSeq`).

## Keyboard first, with one owner per key

* `windowShortcuts` declares the window's keys at the window scope: Ctrl+Enter
  (connect/disconnect) and / (the resource filter). macOS uses Command.
* A panel owns the login: Ctrl+Enter waits while one is up, and Esc is its
  cancel (`panelCancel`, in the order `View` shows the panels).
* / is registered only while the filter is on screen and unfocused. Once the
  field has the focus the key belongs to the text, and the focus request is
  dropped when the list leaves.
* 设置 is a sidebar page, not a shortcut. Ctrl+, and the window's `⋯` menu
  were removed with the second window; quit lives in the tray.

## One language at a time, from one catalog

* Every visible string comes from `locales.Catalog`, keyed by surface. No CJK
  literal lives in a production view; `internal/guards` fails the build if one
  appears.
* Counts and durations are ICU messages with placeholders, so a translation can
  move the number. `zh` is the source language and the fallback.
* `core/*` stays English: library errors are read in terminals and logs, and the
  UI boundary translates them.

## mygo owns the toolkit behaviour

* The window uses mygo 0.7.2: a hidden title bar keeps the desktop's window
  controls, and a hide from the window's own frame is queued by the toolkit.
* Views are pure functions of `ui.State`; they read no network and keep no
  global state. Widget state that belongs to one field lives in `ui.Local`, app
  state in the state struct. `SettingsState` is the one long-lived draft the
  views render in place.
* The render harness (`TestRenderEveryState`) is the acceptance instrument for a
  layout change; the tester (`ui.NewTester`) proves behaviour. The real-window
  checks and their limits are in `docs/agent-ui-testing.md`.
