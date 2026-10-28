# Interface rules

The rules the tray window and the settings window follow, and the reasons they
exist. They are decisions, not preferences: a change that breaks one of them
needs a phase in the plan and an entry in its decisions table.

## The session is the headline

* The state is the first thing in the window: `statusChip` draws it at title
  size, with the semantic colour on the dot and the label in `Theme.Text`.
* One subline under it carries the endpoint, the gateway's address and the time
  held. A failure or the reconnect ladder replaces the timing, in the tone that
  reports it.
* The brand and the profile sit in the 34 DIP title strip, where the desktop's
  window controls already are. They are static; the state is not, so the state
  outranks them.
* The footer is one quiet line: the listeners and the last keep-alive. The PAC
  URL and the probe's own result are in its tooltip, not on the line.

## One primary action per screen

* Exactly one filled button per screen: 连接/断开, 保存, 提交, 保存并连接. Every
  other action is a plain button, a text button or a menu entry.
* 复制配置 owns the PAC and the three Clash renderings. The tray does not repeat
  them: one owner per action, one wording.
* A destructive or abandoning action is not a variant of the primary one. 取消登录
  sits on the left of the panel's foot (`panelActions`), 提交 on the right, and
  the two corrections 撤销 and 清空 are face-less text buttons between them.
* A row's actions go behind one menu (`Element.Menu` on the dots glyph), not two
  visible buttons per row.

## A login never needs a second window

* The first-run form lives in the main window, in the place of the connection
  controls (`setupPanel`). 连接 refuses in Chinese and reveals it; it never
  sends a new user to the settings window.
* Captcha, one-time code, SSO, device binding, password change and the method
  chooser are panels inside the window (`panelSlot`), never dialogs.
* A panel keeps the header, the state and the granted-resource count as one
  line above the footer. The list itself needs the height the question does.
* A panel is at most `panelMaxWidth` wide: a maximized window must not stretch
  three fields across the screen.

## Semantic colour is a mark, not a fill

* A tone is meaning, not a hex value (`tone`, `toneColor`, `toneInk`). The raw
  mygo fill is a Meter colour: it measures 2.8-4.4:1 as text on the chip tint,
  so it is never the text.
* Text that has to be read uses `Theme.Text` on the tint, or `toneInk` on the
  window. `toneInk` mixes the fill toward the far end of the appearance until it
  clears 4.8:1 against the chip's tint; `TestToneInkPassesAA` asserts it in both
  appearances, so a theme change cannot slip a failing colour back in.
* A healthy probe is not news: only a warning or a failure colours the footer.

## Four text sizes, one spacing grid

* Every size comes from `textSize` on the theme's own font size: heading, panel
  heading, body, caption. No view hardcodes a size; a desktop font setting moves
  all four together.
* Padding, gaps and control sizes are multiples of `Theme.Spacing`
  (`Theme.Space`), so a compact or roomy theme stays consistent.
* Form controls are capped (360 in the settings window), labels sit above their
  values, and a row whose label can outgrow the line wraps instead of clipping.

## A result is a toast, not a line that stays

* `statusToast` reports copy, save, profile and autostart outcomes once per
  sequence number. A success or failure is something that happened, not a
  permanent line under the form.
* The main window and the settings window share the helper; each has its own
  sequence (`State.StatusSeq`, `SettingsState.StatusSeq`).

## Keyboard first, with one owner per key

* `windowShortcuts` declares the window's keys at the window scope: Ctrl+Enter
  (connect/disconnect), Ctrl+, (settings), Esc (the panel's cancel) and /
  (the resource filter). macOS uses Command.
* A panel owns the login: Ctrl+Enter waits while one is up, and Esc is its
  cancel (`panelCancel`, in the order `View` shows the panels).
* / is registered only while the filter is on screen and unfocused. Once the
  field has the focus the key belongs to the text, and the focus request is
  dropped when the list leaves.

## One language at a time, from one catalog

* Every visible string comes from `locales.Catalog`, keyed by surface. No CJK
  literal lives in a production view; `internal/guards` fails the build if one
  appears.
* Counts and durations are ICU messages with placeholders, so a translation can
  move the number. `zh` is the source language and the fallback.
* `core/*` stays English: library errors are read in terminals and logs, and the
  UI boundary translates them.

## mygo owns the toolkit behaviour

* The windows use the pinned mygo fork: hidden title bars keep the desktop's
  window controls, and a hide from a window's own frame is queued by the toolkit.
* Views are pure functions of `ui.State`; they read no network and keep no global
  state. Widget state that belongs to one field lives in `ui.Local`, app state in
  the state struct.
* The render harness (`TestRenderEveryState`) is the acceptance instrument for a
  layout change; the tester (`ui.NewTester`) proves behaviour. The real-window
  checks and their limits are in `docs/agent-ui-testing.md`.
