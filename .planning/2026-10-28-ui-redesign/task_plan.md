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

Phase 2: create `locales/` — `mygo.json` with the `i18n` block, `locales/zh.json`
as the source catalog holding every message, `locales/en.json`, and a first
`go tool mygo generate` run. Migrate `ui/setup.go` as the proving surface, since
it is the smallest view that has both static text and formatting.

## Current Phase

Phase 2: Localization. Phase 1 is complete: a fresh install opens on the
first-run form, 保存并连接 writes the settings file and starts the login, and an
incomplete profile never surfaces the library's English validation message. The
mechanism for this phase is settled (mygo's catalogs); no catalog file exists
yet.

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

- [ ] `mygo.json` with `{"i18n": {"dir": "locales", "source": "zh"}}`, and
      `locales/zh.json` (the source: every message) plus `locales/en.json`
- [ ] `go tool mygo generate` writes `locales/catalog_gen.go`: typed accessors
      such as `locales.StatusOnline()`, each reading `i18n.Locale()`, so no view
      takes a locale parameter and `i18n.Register` makes zh and en the app's
      languages
- [ ] Import `_ "github.com/egoist/mygo/locales/zh-Hans"` in the tray app: mygo's
      own widget strings (the text field's 撤销/剪切/复制/粘贴 menu, the date
      picker) are English today even inside the Chinese window, because no
      language pack is linked
- [ ] Keys named by surface, ICU placeholders instead of `fmt.Sprintf` for
      counts and durations (`{count} 项`, `{seconds} 秒后`), so a translated
      message can move its numbers
- [ ] Migrate `ui/` and `internal/gui/` (355 distinct literals): no Chinese or
      English literal left in a view; `phaseLabel`, `phaseDetail`,
      `KeepAliveSummary`, `TrayStatus` and `TrayMenu` say theirs through the
      catalog too
- [ ] `core/prompt/web` takes the language tag and keeps its own two-language
      text for its 35 lines: `core/*` must not reach mygo, and `internal/guards`
      checks the whole `go list -deps` graph
- [ ] The settings window gets 跟随系统 / 中文 / English, calling
      `mygo.App.SetLocale("", "zh", "en")`; mygo persists the choice itself
      (PathUserData), so no new key in `config.toml`
- [ ] `internal/guards` or a new test: no CJK literal outside `locales/*.json`,
      and `locales/en.json` carries every key of the source
- [ ] Verify: `go tool mygo generate` is idempotent; `TestRenderEveryState`
      gains a locale switch and renders every state in both languages; the
      English layout is checked at 440×400 and 600×760, where labels grow;
      `LC_ALL=en_US.UTF-8 ./gotrust` shows English with no rebuild
- [ ] `README.md` states the two languages and the switch; `docs/*.md` get
      English versions
- **Status:** pending

### Phase 3: Design tokens

- [x] Render every window state with `TestRenderEveryState` and read the images
- [x] Measure contrast of the semantic colours against both mygo themes
- [x] Inventory the mygo 0.7.1 widgets a redesign may use (`Tabs`, `MenuButton`,
      `SearchField`, `Icon`, `Toast`, `Tooltip`, `Badge`, `Segmented`)
- [ ] Add text-safe semantic colours (`successText`, `warningText`,
      `dangerText`) to `ui/theme.go`, derived per appearance
- [ ] Rework `statusChip`: semantic colour on the dot only, label in
      `Theme.Text` on the 12% tint, so the pill passes at caption size
- [ ] Delete the hardcoded `FontSize(13)` (`ui/settings.go:529`) and
      `FontSize(10)` (`ui/view.go:295`); everything goes through `textSize`
- [ ] Extend `ui/a11y_test.go` with a contrast assertion on the chip's colours
- **Status:** pending

### Phase 4: Window chrome and main window information architecture

The title bar becomes the window's own: mygo 0.7.1 hides the frame and keeps
the desktop's window controls, so the view draws a 34px strip under them.

- [ ] Both windows: `TitleBarStyle: mygo.TitleBarHidden`, `TitleBarHeight: 34`;
      GTK keeps its own buttons (`HideWindowButtons` stays false, so the desktop
      decides the layout and the side)
- [ ] One `titleBar(c)` helper: the room `c.TitleBar()` reports, `DragWindow`,
      brand at 12px muted, no interactive child inside the drag region
- [ ] Header: state as the headline (22px, `Theme.Text`), one subline
      `host:port · 内网地址 · 已保持 N 分钟`; drop the duplicate phase line and
      the chip's repeated label
- [ ] Collapse nine buttons into `连接/断开` (primary), `复制配置 ▾`
      (`ui.MenuButton`: PAC, Clash 片段, Verge 脚本, 完整配置), and `⋯`
      (设置, 打开日志目录, 退出)
- [ ] Footer: one line `SOCKS5 … · HTTP … · 保活 N 秒前`, addresses and probe
      detail behind `ui.Tooltip`; transient `State.Status` moves to `ui.Toast`
- [ ] Raise the main window's `MinHeight` to 480 and re-check 440×400-era states
      in the new chrome
- **Status:** pending

### Phase 5: Resource list

- [ ] Group entries by `ResourceKind` with per-group counts; the `Kind` stops
      living inside `Detail` text
- [ ] Row: 18px kind icon (`ui.Icon` with an inline SVG, no emoji), title,
      right-aligned detail, `⋯` row menu; 打开/复制 leave the always-visible row
- [ ] `ui.SearchField` appears once a group holds more than eight entries
- [ ] Fix the 440×400 collapse: `Scroll` gets a `MinHeight` so the list keeps at
      least three rows at the declared minimum
- **Status:** pending

### Phase 6: Settings window

- [ ] `ui.Tabs` pages: 连接 / 代理 / 保活 / 启动 / 高级 / 终端; the profile
      picker and 新建/复制/删除 move to a fixed top bar
- [ ] Each section heading carries its effect (`下次登录生效`,
      `下次连接生效`) instead of one long caption above the form
- [ ] Inputs get `MaxWidth(360)`, label column stays one width
- [ ] Unsaved changes marked in the heading; the bottom bar offers 保存 and
      放弃修改
- [ ] 终端管理 becomes its own page; it no longer sits between 保活 and 高级
- **Status:** pending

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
| Semantic colours split into fill and text variants | mygo's `Success`/`Warning` are Meter colours; measured 3.30:1 and 3.19:1 on white, 2.89/2.81 on the chip's tint |
| Login panels stay in the window instead of a second dialog | A prompt asks about a specific account and gateway; the header must stay visible, and a new window frame is the documented crash risk |
| Contrast becomes a test, not a review note | The failure is numeric and cheap to assert from the rendered pixels |
| Localize to zh-CN + en-US | The product already mixes them: Chinese GUI and docs, English CLI and library errors. "Chinese only" is not true today |
| Use mygo's catalog and generated accessors, not a hand-rolled table | The toolkit already ships catalog parsing, ICU formatting, locale resolution, a persisted override and the widget strings; a second mechanism would have to be kept in step with it |
| Source language is `zh`, not `en` | The window's text is Chinese today; making English the source would rewrite every message before a single translation exists |
| Keys named by surface with ICU placeholders | A key survives a copy edit, and `{count}`/`{seconds}` let a translation reorder numbers instead of freezing `fmt.Sprintf` layout into the sentence |
| Locale follows the desktop, switchable in the settings window | mygo resolves `LANGUAGE`/`LC_ALL`/`LC_MESSAGES`/`LANG` and persists the user's own choice; a settings key would duplicate both |
| The catalog lands before the redesign phases | Phases 4-8 rewrite most of the 355 GUI strings; with the catalog in place each rewrite is one line instead of a literal hunt |
| The headless CLI stays English | Its output is read in terminals and logs, which are already English |

## Errors Encountered

| Error | Attempt | Resolution |
|---|---|---|
| `curl https://api.github.com/repos/egoist/mygo/contents/…` returned 403 | 1 | Read the pinned module source at `~/go/pkg/mod/github.com/egoist/mygo@v0.7.1` and fetched the skill from the codeload tarball instead |

## Notes

* The skill `mygo-maintenance` is installed at `.agents/skills/mygo-maintenance/`
  (gitignored): it carries the lifetime, identity and threading rules these
  changes have to respect, plus `go tool mygo vet .` and `ui.NewTester` for
  verification.
* The render harness is the acceptance instrument for every phase: images
  before in `/tmp/gotrust-ui-before`, images after in `/tmp/gotrust-ui`.
