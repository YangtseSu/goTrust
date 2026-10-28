# Task Plan: goTrust — interface redesign

## Goal

A fresh installation can be connected from the main window in one step, and
every screen afterwards shows the session as the primary information: the
connection state is the headline, each control has exactly one owner and one
place, the granted resources can be scanned and filtered, the settings form is
navigable in sections, and every semantic colour reaches 4.5:1 at caption size
in both appearances — all of it built from the pinned mygo 0.7.1 widgets, with no
new dependency and no change to `core/session`.

## Next Step

Phase 7: bring the login panels inside the window — the header stays, the
granted resource count stays as one line, the buttons get a hierarchy (提交
primary, 取消登录 left of the action row), and the captcha markers share the web
page's 26px marker and 13px number. Phase 6 is complete: the settings window is
six pages under a fixed profile bar, each page states when its edits take
effect, and the heading marks unsaved changes with 保存 / 放弃修改 at the foot.

## Current Phase

Phase 7: Login panels. Phase 6 is complete: `settingsTabBar` draws the six
pages with `ui.TabsBase` in a wrapped row, `SettingsState.Tab` chooses the page,
`Changed()` compares the folded draft with the loaded file, and
`SettingsActions.Discard` throws the draft away.

## Phases

Status values are `pending`, `in_progress`, `complete` only.

### Phase 1: First-run setup

Reported defect: on a fresh install, 连接 fails with the gateway/account error
and the only place to fix it is the settings window, which the user has to find
first.

- [x] `ProfileProblem(profile)` in `ui`: one Chinese sentence naming the first
      missing login field (address, port, username), and a table test that keeps
      it in step with `config.Profile.Validate`
- [x] `State.Setup` + `SetupDraft` (address, port, username, password,
      store-password, error) and `SetupDraft.Apply(*config.Config)`, mirroring
      `SettingsState.Apply`
- [x] `setupPanel` in the main window: 网关地址 / 端口 / 用户名 / 密码, the
      password optional with the reason stated, 保存并连接 (primary), 仅保存,
      更多设置… (opens the settings window)
- [x] `View` shows the panel instead of `controls` + `resources` whenever
      `State.Setup` is set; the tray's 连接 opens the window on the same panel
- [x] `internal/gui`: `startLogin` refuses with the Chinese reason and reveals
      the panel; `connect()` returns the same reason instead of English;
      `saveSetup` writes through `config.SaveFile` (0600) and then logs in
- [x] `refreshSetup` keeps the draft while the user types, rebuilds it when the
      active profile changed, and drops it once the profile is complete
- [x] Tests: `ui/setup_test.go` (panel appears while incomplete, complete
      profile shows 连接, 保存并连接 reaches the callback), an app test that the
      save lands on disk and in memory, and a `main-setup` render case
- **Status:** complete

### Phase 2: Localization (zh-CN + en-US)

goTrust speaks two languages today and claims one: the window and the documents
are Chinese, `gotrust-cli` and every library error are English. The worst case
is a single session that shows 已连接 in the window and `server address is empty`
in the footer. This phase makes the language a choice the user makes once.

The mechanism is mygo 0.7.1's own: a JSON catalog per language, generated typed
accessors, and one locale the whole app reads.

- [x] `mygo.json` with `main` (`./cmd/gotrust`) and
      `{"i18n": {"dir": "locales", "source": "zh"}}`, and
      `locales/zh.json` (the source: 302 messages) plus `locales/en.json`
- [x] `go tool mygo generate` writes `locales/catalog_gen.go`: typed accessors
      such as `locales.StatusOnline()`, each reading `i18n.Locale()`, so no view
      takes a locale parameter and `i18n.Register` makes zh and en the app's
      languages
- [x] mygo's own widget strings (the text field's 撤销/剪切/复制/粘贴 menu, the
      date picker) are English even inside the Chinese window because no
      language pack is linked; the accessors `mygo generate` writes import
      `_ "github.com/egoist/mygo/locales/zh-Hans"` themselves, so no
      hand-written import is needed
- [x] Keys named by surface, ICU placeholders instead of `fmt.Sprintf` for
      counts and durations (`{count} 项`, `{seconds} 秒后`), so a translated
      message can move its numbers
- [x] `ui/` and `internal/gui/` migrated: the views, the settings state's
      validation messages and every status line read the catalog;
      `phaseLabel`, `phaseDetail`, `KeepAliveSummary`, `TrayStatus` and
      `TrayMenu` included
- [x] `core/prompt/web` takes `Options.Language` and keeps its own two-language
      text (`text.go`), since `core/*` must not reach mygo; an empty tag
      follows `LC_ALL`/`LC_MESSAGES`/`LANG`
- [x] The settings window gets 跟随系统 / 中文 / English: a segmented control
      bound to `SettingsState.LocaleIndex` calls `SetLocale("", "zh", "en")` on
      the next main-loop turn; mygo persists the choice itself
- [x] `internal/guards/catalog_test.go`: no CJK string literal in the
      production files of `ui/` or `internal/gui/`, and both catalogs carry the
      same keys
- [x] Verify: `go tool mygo generate` is idempotent; `TestRenderEveryState`
      renders every state in both languages; the English layout is read at
      540×640, 440×400 and 600×760; a headless mutter session started with
      `LC_ALL=en_US.UTF-8` shows the English first-run window (its capture is
      byte-identical to the `LANG=en_US.UTF-8` one)
- [x] `README.md` states the two languages and the switch;
      `docs/configuration.md` and `docs/handoff.md` gained English versions
      (the other two documents were already English), and each pair links the
      other
- **Status:** complete

### Phase 3: Design tokens

- [x] Render every window state with `TestRenderEveryState` and read the images
- [x] Measure contrast of the semantic colours against both mygo themes
- [x] Inventory the mygo 0.7.1 widgets a redesign may use (`Tabs`, `MenuButton`,
      `SearchField`, `Icon`, `Toast`, `Tooltip`, `Badge`, `Segmented`)
- [x] Add the text-safe tone ink to `ui/theme.go` (`toneInk`, plus
      `chipSurface`, `contrastRatio`, `luminance`), derived per appearance from
      the theme's own fill instead of a fixed hex
- [x] Rework `statusChip`: semantic colour on the dot only, label in
      `Theme.Text` on the 12% tint, so the pill passes at caption size
- [x] Delete the hardcoded `FontSize(13)` (`ui/settings.go:529`) and
      `FontSize(10)` (`ui/view.go:295`); everything goes through `textSize`
- [x] Extend `ui/a11y_test.go` with a contrast assertion on the chip's colours
- **Status:** complete

### Phase 4: Window chrome and main window information architecture

The title bar becomes the window's own: mygo 0.7.1 hides the frame and keeps
the desktop's window controls, so the view draws a 34px strip under them.

- [x] Both windows: `TitleBarStyle: mygo.TitleBarHidden`, `TitleBarHeight: 34`;
      GTK keeps its own buttons (`HideWindowButtons` stays false, so the desktop
      decides the layout and the side)
- [x] One `titleBar(c, profile)` helper: the room `c.TitleBar()` reports,
      `DragWindow`, brand and the active profile at caption size muted, no
      interactive child inside the drag region
- [x] Header: the state as the headline (`statusChip` at title size, label in
      `Theme.Text`), one subline `host:port · 内网地址 · 已保持 N 分钟`; the
      duplicate phase line is gone and the profile name moved to the strip
- [x] Collapse nine buttons into `连接/断开` (primary), `复制配置 ▾`
      (`ui.MenuButton`: PAC, Clash 片段, Verge 脚本, 完整配置), and `⋯`
      (设置, 打开日志目录, 退出); the header row wraps at the minimum width
- [x] Footer: one line `SOCKS5 127.0.0.1:1080 · HTTP 127.0.0.1:1081 · 保活 N
      秒前`, with the PAC URL and the probe's own result behind `ui.Tooltip`;
      `State.Status` moves to `ui.Toast`, emitted once per `StatusSeq`
- [x] Raise the main window's `MinHeight` to 480 and re-check the 440×400-era
      states in the new chrome (440×480)
- **Status:** complete

### Phase 5: Resource list

- [x] Group entries by `ResourceKind` with per-group counts; the `Kind` stops
      living inside `Detail` text
- [x] Row: 18px kind icon (`ui.Icon` with an inline SVG, no emoji), title,
      right-aligned detail, `⋯` row menu; 打开/复制 leave the always-visible row
- [x] `ui.SearchField` appears once a group holds more than eight entries
- [x] Fix the 440×400 collapse: `Scroll` gets a `MinHeight` so the list keeps at
      least three rows at the declared minimum
- **Status:** complete

Details: `ui/icons.go` holds the five glyphs (application grid, globe, stacked
hosts, name book, unknown kind) plus the three dots of a row's menu, all stroked
at mygo's own 24-grid weight. `groupResources(entries, query)` is a pure
function of the grant and the filter; it fixes the order 应用 / 域名 / IP 地址 /
DNS, puts a kind it does not know under 其他, and drops empty groups, so a
heading counts only rows that are on screen. The filter matches the title or
the detail, case-insensitively, and lives in the field's own `ui.Local` state
rather than in `State`: no other part of the window reads it, and a new login
must not inherit a stale query. It is one field over the whole list, shown when
any kind exceeds eight rows, so the threshold is about the length of a group
rather than about the field's position in the window. A row's menu is the dots
glyph with `Element.Menu`, not `ui.MenuButton`: the arrowed button is right in
the header, but one per row would put a border and an arrow beside every entry
and outshout the rows. `ui.Scroll` now has `MinHeight` of three rows at the
theme's size (18px icon line plus its 6px padding each side), so the list stops
collapsing to its heading when the header and footer have taken the window.

### Phase 6: Settings window

The form was one 1450px column: reaching 代理 meant scrolling past a screen and
a half, the effect of every setting sat in one caption above the form, and
nothing said whether the window held unsaved edits.

- [x] Six pages — 连接 / 代理 / 保活 / 启动 / 高级 / 终端 — as mygo's tab list
      (`TabsBase` with the widget's own look), so the choice, the arrow keys and
      the tab roles stay the toolkit's; the row wraps because six English labels
      do not fit the 560px minimum
- [x] The profile picker with 新建/复制/删除 moves from a fieldset inside the
      form to a fixed bar above the tabs
- [x] Each page opens with its effect (`下次登录生效`, `下次连接生效`,
      `下次启动生效`, `详细日志立即生效；客户端数据下次登录生效`,
      `随连接实时读取`) instead of one `settingsHint` paragraph; that key is gone
- [x] Controls are capped at `fieldWidth` (360), the method picker that shares
      its row with 获取认证方式 at 240, and every row whose labels can outgrow
      the line wraps
- [x] `SettingsState.Changed()` folds the draft the way `Apply` folds it and
      compares it with the loaded file; the heading marks 未保存的修改 and the
      bottom bar offers 保存 (primary) and 放弃修改, both disabled while nothing
      differs
- [x] `SettingsActions.Discard` reloads the draft and reports 已放弃修改; 关闭
      left the bottom bar, because the window's own close control already hides
      and discards
- [x] 终端管理 is its own page (终端), no longer between 保活 and 高级
- **Status:** complete

Details: `settingsTabBar` is `ui.Tabs` with one difference: the row wraps.
`ui.Tabs` lays its labels out in a single `Shrink(0)` row, so at 560 DIP the
English labels already ran 14px past the content box at the default size, and
larger text would have pushed 终端 off the window. `TabsBase` exposes the list
and keeps the choice, the arrow keys and the tab roles; the wrapped rows keep
the widget's look (12px tab padding, a 2px accent underline under the choice, a
6px focus ring). `settingsPage` switches on `SettingsState.Tab`, so a page that
is not shown is not in the tree at all, and the tab names the page, which is why
the page draws only its effect line instead of repeating the title. `folded()`
is `Apply` without the checks: it trims the same fields, parses the port through
the same range, splits the same targets and sets the active profile, which is
why choosing another profile in the picker marks the window unsaved — saving is
what switches. `Changed()` compares two `Clone`s with `reflect.DeepEqual`, so a
field added to `config.Config` later cannot silently escape the marker. 语言
moved to the bottom of 启动: it is the only app-level page, and 高级 holds
diagnostics and the device identity.

### Phase 7: Login panels

- [ ] Panels render inside the window with the header intact, instead of
      replacing `controls` and `resources` wholesale; the granted resource count
      stays visible as one line
- [ ] Button hierarchy: 提交 primary, 撤销/清空 as text buttons, 取消 left of
      the action row and labelled 取消登录
- [ ] Captcha markers share the web page's 26px marker and 13px number, and the
      number uses `captionScale` rather than a literal
- **Status:** pending

### Phase 8: Keyboard, feedback, tray

- [ ] `c.OnShortcut`: `Ctrl+Enter` connect/disconnect, `Ctrl+,` settings, `Esc`
      cancel the current panel, `/` focus the resource search
- [ ] Tray menu stops duplicating the window's copy actions; one owner per
      action, one wording
- [ ] `ui.Toast` reports copy/save results instead of a persistent footer line
- **Status:** pending

### Phase 9: Verification and docs

- [ ] Re-render every state (`GOTRUST_RENDER_DIR=… go test ./ui -run
      TestRenderEveryState`) and compare against `/tmp/gotrust-ui-before`
- [ ] `CGO_ENABLED=0 go test ./...`, `go vet ./...`, `go tool mygo vet .`,
      `gofmt -l .`, `reuse lint`
- [ ] Run the tray app on the operator's Wayland session: drag by the new title
      bar strip, double-click maximize, close-to-tray, and the first-run panel
      from an empty config
- [ ] Record the design rules (state first, one primary action, text-safe tones,
      four sizes, first run never sends the user to a second window) in `docs/`
- **Status:** pending

## Not doing

* No multi-step onboarding wizard. First run asks for what a login needs, in the
  window that was already open; everything else waits in 更多设置.
* No "test this account" button in the first-run panel: the login itself is the
  test, and a second network path would duplicate the login's errors.
* No custom light/dark switch: the app follows the desktop appearance through
  mygo's `ThemeModule`.
* No icon package: `ui.Icon` takes an inline SVG, so the kind glyphs are five
  small paths in the tree, not a dependency.
* No change to `core/session`, `core/config` or the protocol layer. Validation
  messages stay English there; the UI boundary translates.
* No second settings surface: the tray keeps 设置… and the first-run panel links
  to it.
* No third UI language, no runtime translation service, and no per-window
  language switch; the locale is one value for the process.
* No translation of the imported protocol layer's log lines: they are English
  already, and they are diagnostics rather than interface.
* No animated transitions. mygo 0.7.1 has no transition API for these widgets,
  and `docs/agent-ui-testing.md` makes a new window frame the risky place to
  experiment.

## Key Questions

1. What does a first run have to ask for? — Gateway address, port, username.
   The password stays optional because a gateway that uses second-factor or
   unified authentication never asks for one until the login flow does.
2. Where does the first-run form live? — The main window, in the slot the
   connection controls occupy. The settings window is the editor for a config
   that already exists, which is why sending a new user there was wrong.
3. Who decides "this profile is not usable yet"? — One function,
   `ui.ProfileProblem`, called by the view (to show the panel) and by the
   connect path (to refuse in Chinese). `config.Profile.Validate` stays the
   authority; a test keeps the two lists identical.
4. Does the hidden title bar keep the desktop's window buttons? — Yes.
   `TitleBarHidden` keeps GTK's own buttons; `HideWindowButtons` would make the
   view draw close/minimize/maximize, which is more surface and more ways to
   differ from the desktop.
5. Does the chip keep a tinted background at all? — Keep the tint, put the label
   in `Theme.Text`: the colour then reads at a glance without the text having to
   pass on a 12% surface.
6. Should `State.Status` disappear from the model? — No. The window stops
   painting it permanently; `ui.Toast` still needs the same string.
7. Which languages, and does everything get one? — zh-CN and en-US. No third
   language, no runtime translation, and the headless CLI keeps its English
   output because its text is read in terminals and logs.
8. When does localization happen? — Before the visual phases. With generated
   accessors in place a redesign edits one catalog line per string instead of
   hunting literals, so the catalog lowers the cost of Phases 3-8.
9. Who owns the language? — mygo. `App.Locale()` resolves the desktop's
   languages, `App.SetLocale` stores an override for the next launches, and the
   generated accessors read it; goTrust adds no locale plumbing of its own.

## Decisions Made

| Decision | Rationale |
|---|---|
| First run is edited in the main window, not the settings dialog | The reported defect: 连接 told the user to configure something and the only way to do it was another window |
| Password is optional in the first-run panel, with the reason on screen | CAS/OAuth2 and second-factor gateways have no stored password; requiring one would dead-end those accounts |
| `ProfileProblem` owns both "show the form" and "why the connect refused" | Two copies of the precondition drift; the user must never see `server address is empty` |
| Adopt `TitleBarHidden` for both windows | Removes the double title (frame title + in-view brand), gives back ~30px, and keeps GTK's buttons and the desktop's button layout |
| Keep `HideWindowButtons` false | The view would have to draw and wire close/minimize/maximize; GTK already does, correctly, on the operator's Wayland session |
| State becomes the window headline, brand drops to 12px | With the frame hidden the brand cannot hide in the title bar, but it must not outrank the state |
| Nine buttons become primary + `复制配置` menu + `⋯` overflow | Measured against the render: every control had equal weight, so the primary action had none |
| Resources group by kind instead of a flat list | `ResourceEntry.Kind` already exists and apps/domains/addresses are read differently; grouping costs no new data |
| Semantic colours split into fill and text variants | mygo's `Success`/`Warning` are Meter colours, measured 3.30:1 and 3.19:1 on white, 2.89/2.81 on the chip's tint. `toneInk` mixes the fill toward the far end of the appearance until it clears 4.8:1 against that tint, so a theme change or the desktop's own accent cannot reintroduce a failing colour |
| Login panels stay in the window instead of a second dialog | A prompt asks about a specific account and gateway; the header must stay visible, and a new window frame is the documented crash risk |
| The title strip names the profile | The subline's job is the session; the account is static, so it belongs where it does not compete, and a fresh install still shows which profile is being filled |
| The header row wraps instead of clipping | At 440 DIP the English labels of three controls plus the headline exceed the width; a wrapped second row keeps `⋯` reachable, where a clipped row would hide settings and quit |
| The first-run panel hides the header's 连接 and the subline | The panel owns the connect action and says what is missing; two controls for one intent, and a line repeating its heading, only cost the form height |
| `listeners()` in `core/proxy` reports the protocol | The footer names SOCKS5 and HTTP; deriving the protocol from order would tie the window to the order the server binds in |
| Contrast becomes a test, not a review note | The failure is numeric and cheap to assert from the tokens the views paint with |
| One filter above the list, shown when any kind exceeds eight rows | The threshold is about how long a group is, not where the field sits; a field per group would add a control to every heading and filter by kind the heading already names |
| The filter lives in the field's `ui.Local` state, not in `State` | Nothing else reads the query, `State` is copied per frame by the app, and a new login must not inherit a stale filter |
| A row's menu is `Element.Menu` on the dots glyph, not `ui.MenuButton` | The arrowed button belongs in the header; one per row puts a border and an arrow beside every entry and outshouts the rows |
| A kind the build does not know is listed under 其他 | A newer gateway adds rows instead of losing them, and the window still has one place for each entry |
| The settings pages are `TabsBase` in a wrapped row, not `Tabs` | Six English labels are 526px at the 560px minimum and wider with larger text; `Tabs` lays them out in one `Shrink(0)` row and cannot wrap, while `TabsBase` keeps the choice, the arrow keys and the tab roles and costs ten lines for the same look |
| The tab names the page, so the page draws only its effect line | A heading repeating the tab label 20px under it says the same thing twice; the effect is the one line that changes meaning from page to page |
| 保存 and 放弃修改 are disabled while the draft matches the file | A greyed primary button is the clearest "a save would write nothing"; it also keeps the keyboard order honest, because a disabled control takes no focus |
| The bottom bar dropped 关闭 | The window's own close control already hides and discards the draft; two controls for one intent |
| 语言 sits at the bottom of 启动 | It is the only app-level page; 高级 is diagnostics and the device identity, and the language is not a saved setting (mygo stores it) |
| The 名称 field stays on 连接, not in the fixed bar | The bar has room for the picker and three actions at 560px; a name input beside them would wrap in English and cost the form a row |
| Every page's controls are capped at 360 | A value does not need the window's width; the label column already aligns them, so the cap is one number |
| Unsaved is computed from the file, not from keystrokes | Comparing the folded draft with the loaded configuration catches a profile switch, a delete and a checkbox, and cannot be fooled by typing a value the file already holds |
| The list keeps three rows as its `MinHeight` | The window exists for the resources; a list that collapses to a heading is the 440-wide defect this phase closes |
| Localize to zh-CN + en-US | The product already mixes them: Chinese GUI and docs, English CLI and library errors. "Chinese only" is not true today |
| Use mygo's catalog and generated accessors, not a hand-rolled table | The toolkit already ships catalog parsing, ICU formatting, locale resolution, a persisted override and the widget strings; a second mechanism would have to be kept in step with it |
| Source language is `zh`, not `en` | The window's text is Chinese today; making English the source would rewrite every message before a single translation exists |
| Keys named by surface with ICU placeholders | A key survives a copy edit, and `{count}`/`{seconds}` let a translation reorder numbers instead of freezing `fmt.Sprintf` layout into the sentence |
| Locale follows the desktop, switchable in the settings window | mygo resolves `LANGUAGE`/`LC_ALL`/`LC_MESSAGES`/`LANG` and persists the user's own choice; a settings key would duplicate both |
| The catalog lands before the redesign phases | Phases 4-8 rewrite most of the 355 GUI strings; with the catalog in place each rewrite is one line instead of a literal hunt |
| The headless CLI stays English | Its output is read in terminals and logs, which are already English |
| The login pages carry their own two-language text | `core/prompt/web` cannot import mygo's catalogs, and `internal/guards` forbids it; a 60-field table is smaller than moving the package out of core |
| The language picker binds `SettingsState.LocaleIndex` | A local recomputed per frame loses the click before `Changed` reports it; the profile picker uses the same pattern |
| `SetLocale` runs on the next main-loop turn | A language change redraws every window, and the click happens inside a frame |
| The CJK guard covers production view files, not tests | A test may use Chinese as fixture data (truncation, names); a view may not |
| Only two documents needed translating | `agent-ui-testing.md` and `development.md` were already English |

## Errors Encountered

| Error | Attempt | Resolution |
|---|---|---|
| `curl https://api.github.com/repos/egoist/mygo/contents/…` returned 403 | 1 | Read the pinned module source at `~/go/pkg/mod/github.com/egoist/mygo@v0.7.1` and fetched the skill from the codeload tarball instead |
| `act press` on the ⋯ menu button hung the app's main loop, so the smoke-test client never got its next answer | 1 | The overflow menu is a GTK popup with a loop of its own, which the automation protocol cannot drive; the run opens the settings window from the first-run panel's 更多设置… instead, a plain button |

## Notes

* The skill `mygo-maintenance` is installed at `.agents/skills/mygo-maintenance/`
  (gitignored): it carries the lifetime, identity and threading rules these
  changes have to respect, plus `go tool mygo vet .` and `ui.NewTester` for
  verification.
* The render harness is the acceptance instrument for every phase: images
  before in `/tmp/gotrust-ui-before`, images after in `/tmp/gotrust-ui`.
