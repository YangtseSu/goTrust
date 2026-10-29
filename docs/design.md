# Interface rules

The rules the window follows, and the reasons they exist. They are decisions,
not preferences: a change that breaks one of them needs a phase in the plan and
an entry in its decisions table. The layout they describe is
[`docs/mockup.html`](mockup.html), the HTML the window is built against.

## One window, one page at a time

* The window is the app: a 900×740 page area by default, a 720×560 minimum.
  GTK counts the window's own shadow margin in the minimum hint, so the hint
  carries that margin on top of the page minimum (`windowShadowWidth`,
  `windowShadowHeight` in `internal/gui`). The sidebar is 208 DIPs wide and
  the page column stops at 680; a form control fills its card.
* The window surface is transparent and the content is clipped to a 12 DIP
  radius, so all four corners are round. GTK keeps the shadow, the resize
  edges and the desktop's window controls.
* The title strip is 44 DIPs and spans the window: the shield mark, `goTrust`
  and, at the right, the appearance switch and the window's own minimize,
  maximize and close controls. The frame's buttons are hidden
  (`HideWindowButtons`), so the strip draws them while GTK keeps the shadow,
  the rounded corners and the resize edges. A close hides the window to the
  tray; quitting stays in the tray menu.
* A card is the grouped face of one thing (the session, the resources, a
  settings group, the account's Connection or Security section, the account
  list): the mockup's surface colour, a 12 DIP radius and a 1 DIP border.
* The appearance switch forces the whole application, mygo's own widgets
  included, to the light or the dark theme (`mygo.Theme.SetSource`); the
  choice lasts for the run.
* The sidebar is the only navigation: Dashboard, a separator, Account, Proxy,
  Keep-alive, Startup, Terminals, Logs, Settings, About, then a spacer, a
  separator and Accounts at the bottom. The active page is tinted with the
  accent; the rows are tabs for assistive technology. The tab column is one Tab
  stop: Tab enters at the page on screen, the Up/Down arrows move among the
  rows, and Enter or Space activates the focused one.
* Accounts is quieter than the pages above it: a subdued row whose tag is the
  live account, so the account list does not read as one more step of the
  settings pages.
* One page is mounted at a time. A page scrolls when its content is taller
  than the window, the dashboard included, as the mockup's scrolling view
  does; the shell's bars stay in place.

## The session is the headline

* The dashboard is the session page: the phase as a chip (a tone-tinted pill,
  a dot in the phase's colour and the label in the theme's text colour) and
  the 60 DIP play/stop control at the right of the same row.
* The chip's colour is the phase: accent while connecting, warning while
  reconnecting, success online, danger failed, muted when idle. The dot
  repeats the colour, so colour is never the only channel; the label names
  the phase and the control carries the action.
* The control is the one primary action on the page: a play triangle and
  Connect when idle or failed (Retry), a spinner and Cancel while connecting or
  reconnecting, a stop square and Disconnect online. Its fill is the mockup's
  (#16a34a online, #d97706 waiting, #dc2626 failed, the accent when idle);
  the hover fill is red for an ending action and a darker accent for a
  starting one.
* One meta line under the chip: `Gateway host:port`, the time held when
  online, the failure detail in the danger tone, or the reconnect ladder
  (attempt and countdown) while it waits. A port is an identifier: it is
  printed as typed, never with a thousands separator.
* A four-cell metrics row follows (Assigned address, SOCKS5, HTTP,
  Keep-alive). A value the session does not have is `—`. The keep-alive cell
  carries the last probe's summary in its description, so the row stays one
  line.
* The traffic card sits between the session and the resources: Traffic, the
  newest rates as two legends, Down in the accent and Up in the success
  colour, Last 60 seconds at the right, and a 80 DIP sparkline of the last minute
  under them. The download is the accent area fading to nothing under its
  stroke, the upload the success line above it; both share one scale, so the
  taller line is the faster direction. The application samples the proxy's
  byte counters once a second into a 60-point ring; a new session starts a
  new minute, and a minute with no traffic draws a muted baseline instead.
* The resources card is the third card: its title, the count, Open default
  app and the search field in the header, then the granted list as part of the
  scrolling page. Open default app is disabled while the grant has no launch
  address; the filter appears as soon as there is a row, and / focuses
  it.
* The session line is shared with the pre-login panels, so a question never
  hides the phase it belongs to.

## One primary action per screen

* Exactly one filled button per screen: the play/stop control on the
  dashboard, Save in the save bar, Submit in a panel's foot. Every other action
  is a plain button, a text button or a menu entry.
* One owner per action. Proxy owns the PAC address and the three Clash/Mihomo
  renderings with their Copy buttons; Logs owns Open folder and Copy; the
  dashboard owns Open default app and nothing else. The tray repeats none of them.
* A destructive or abandoning action is not a variant of the primary one.
  Cancel login sits on the left of a panel's foot (`panelActions`), Submit on the
  right, and the corrections Undo and Clear are face-less text buttons between
  them.
* A resource row's actions go behind one menu (`Element.Menu` on the dots
  glyph), not two visible buttons per row.

## The tray

* The menu is the tray's one interface: the status line, Open Dashboard,
  Open default app, Connect/Disconnect, Switch account when there is more than
  one account, and Quit. Settings is not there.
* On Linux the icon is an AppIndicator. With a menu set, a click opens it
  (GNOME's extension waits out the double-click time and then opens it on a
  single click). Activation shows the window and keeps its page: GNOME's
  extension sends it on a double click, and only when
  libayatana-appindicator3 has the `activate` signal; older releases have
  none. Open Dashboard is therefore the item that always brings the window
  forward, and it selects the dashboard.

## A login never needs a second window

* Captcha, one-time code, SSO, device binding, the method chooser and the
  forced password change are panels inside the window (`panelBody`), never
  dialogs and never windows of their own.
* A panel is a card with the page's fields: the heading, the controls with
  their labels above, and the foot inside the card (Cancel login on the left, the
  answer on the right). The captcha image is capped at 400 DIPs and centered
  in the card, so it still fits the 720 DIP window.
* A panel owns the content wherever the sidebar points, and the sidebar dims
  and disables while it is up. Esc is its cancel; the selected page returns
  when the panel closes.
* Account is the first-run form and the only editor of the account. A fresh
  install opens on the dashboard; Connect selects Account with the reason when the
  saved account is incomplete.
* A panel is at most `panelMaxWidth` (560) wide: a maximized window must not
  stretch three fields across the screen.

## The palette is the mockup's

* Light: content `#fbfbfc`, cards and controls `#ffffff`, sidebar
  `#f1f3f6`, hover `#eceef2`, border `#e2e5ea`, text `#1f2328`, muted
  `#6b7280`, accent `#2563eb`, success `#15803d`, warning `#b45309`, danger
  `#b42318`.
* Dark: content `#1e2025`, cards and controls `#23262c`, sidebar
  `#1a1c21`, hover `#2b2f37`, border `#32363f`, text `#e7e9ee`, muted
  `#9aa1ad`, accent `#77a8fb`, success `#5cc98a`, warning `#e5a54b`, danger
  `#f0736a`.
* `themeFor` rebuilds mygo's theme from these colours, so inputs, switches,
  selects and scrollbars sit in the same palette as the views. A change to
  one colour moves the whole application with it.

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
* A field stacks its label above the control (`formColumn`, `formField`), as
  the mockup draws every form; the label is muted and caption-sized.
* A row whose label can outgrow the line wraps instead of clipping; a long
  account name ellipsizes in the sidebar tag instead of squeezing Accounts.
* A switch row is one target: the whole label toggles, as the mockup's label
  wraps its switch. The track is the mockup's 34×20 pill, accent when on and
  `switchOff` when off; the Startup, Keep-alive and Verbose log switches use
  it.
* Settings' rows fill their card: Data file, Device ID and the language segment.
  The language segment is content-sized, not stretched, and carries no hint
  line. About is its own page below Settings: centered shield and
  `goTrust <version>`, then Project home and License as rows, not rows with a
  button: the row itself opens the page, with the arrow at its right. The
  page has no heading of its own.
* Keep-alive's fields carry no hint lines. A malformed interval, or one shorter
  than the minimum, is refused with the minimum (10s), never with the 90s
  default: `errKeepAliveInvalid`/`errKeepAliveShort` name the shortest
  accepted cadence. Proxy carries no caption: the listener fields say
  nothing until both are empty, when the rule appears as an error line.
* Logs' block (toolbar, pane and footer) is one card, so its border closes
  on every side; its header has the Verbose log switch, Clear and Open
  folder, and Clear truncates the file and drops its rotations. Proxy's PAC and
  Clash/Mihomo rows carry their Copy at the card's right edge. Terminals keeps
  its summary line, its Refresh in the page heading and its two cards before a
  session exists, with 0 counts and the offline placeholder; it has no
  caption of its own. An endpoint row carries the current-device and
  online/offline chips and a four-column fact grid (two at the minimum
  window width). The account list keeps no footer hint.

## A draft, a save bar and a toast

* The settings draft is created at startup and kept for the life of the
  process. Hiding the window, switching pages and opening the tray keep the
  edits; a profile switch, Save, Discard and a login that saves the draft
  are what reload it.
* The save bar appears at the bottom of the content only while the draft
  differs from the file: Unsaved changes, Discard, Save. Applying it writes the
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
* Settings is a sidebar page, not a shortcut. Ctrl+, and the window's `⋯` menu
  were removed with the second window; quit lives in the tray.

## One language at a time, from one catalog

* Every visible string comes from `locales.Catalog`, keyed by surface. No CJK
  literal lives in a production view; `internal/guards` fails the build if one
  appears. The guard also refuses an empty message, an English message that
  still carries Chinese, and placeholders that name different arguments.
* Counts and durations are ICU messages with placeholders, so a translation can
  move the number. `zh` is the source language and the fallback.
* The window is bilingual. The headless front-end's terminal output and logs
  are English; the login pages it serves are English by default and follow
  `--prompt-lang zh` alone, never the environment. `core/*` stays English too,
  so library errors and the session detail are read as written in terminals
  and logs.

## mygo owns the toolkit behaviour

* The window uses mygo 0.7.3: a hidden title bar keeps the desktop's window
  controls, a hide from the window's own frame is queued by the toolkit, and
  the transparent surface with `Clip` gives the four rounded corners. The
  mockup's palette is applied to a copy of the frame's theme, so the desktop's
  text scale and high-contrast adjustments survive it.
* Views are pure functions of `ui.State`; they read no network and keep no
  global state. Widget state that belongs to one field lives in `ui.Local`, app
  state in the state struct. `SettingsState` is the one long-lived draft the
  views render in place; its fields are written on the toolkit's main thread
  only (background results publish through `app.runOnUI`), because the frame
  reads them without a lock.
* Icon-only controls name themselves: the window controls, the appearance
  switch, the power control and a resource row's menu glyph carry their label
  as the accessible name and show it as a tooltip on hover or keyboard focus.
  The row glyph is a menu button to assistive technology.
* The render harness (`TestRenderEveryState`) is the acceptance instrument for a
  layout change; the tester (`ui.NewTester`) proves behaviour. The real-window
  checks and their limits are in `docs/agent-ui-testing.md`.
