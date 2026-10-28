# Progress Log

## Session: 2026-10-28

Plan: `.planning/2026-10-28-ui-redesign/`. Pinned with
`PLAN_ID=2026-10-28-ui-redesign`.

Started from a request for redesign advice. The audit was done first because the
advice had to be checkable: every window state was rendered to a PNG and the
semantic colours were measured, rather than judged from the source alone.

### Phase 1: First-run setup

- **Status:** complete
- Actions taken:
  - Traced the reported defect: 连接 → `manager.Login` →
    `config.Profile.Validate` (English) → `setStatusError(err.Error())` in the
    footer, with the settings window as the only editor (`findings.md`).
  - `ui/setup.go`: `ProfileProblem`, `SetupDraft` (`NewSetupDraft`,
    `Complete`, `Problem`, `Apply`), and `setupPanel`.
  - `ui/view.go`: `State.Setup`, `Actions.SaveSetup`, and the panel in the
    `View` switch after the login prompts; the connection controls and the
    resource list stay hidden while it is up.
  - `internal/gui/gui.go`: `startLogin` (连接 reveals the panel instead of
    failing), `connect` refuses in the window's words, `saveSetup` writes
    `config.SaveFile` then `applyConfig`, `setSetupError` for a rejected save,
    `refreshSetup` + `setupName`, a `refreshSetup` call in `applyConfig`, and one
    at startup so a fresh installation opens on the form.
  - Decided the title bar: `TitleBarHidden` for both windows with GTK's own
    buttons (`HideWindowButtons` false), a 34px strip padded by `c.TitleBar()`
    and dragged with `DragWindow` — recorded in the plan, implemented in Phase 4.
  - README's 第一次使用 rewritten: the form is in the main window, the password
    is optional, 设置 is for everything else.
- Files created/modified:
  - `ui/setup.go` (new)
  - `ui/view.go`
  - `internal/gui/gui.go`
  - `ui/setup_test.go` (new)
  - `ui/render_test.go` (`main-setup`, `main-setup-small`)
  - `internal/gui/gui_test.go` (three first-run tests)
  - `README.md`

### Phase 2: Localization (zh-CN + en-US)

- **Status:** complete
- Actions taken:
  - Counted the surfaces: 355 distinct Chinese literals in the GUI (421
    occurrences), 70 CJK lines in the web prompt pages, zero in `internal/cli`.
  - Chose mygo's own catalog system over a hand-rolled table, and read it out of
    the pinned module: JSON catalogs, generated accessors that read
    `i18n.Locale()`, `SetLocale` persistence, the `zh-Hans` widget pack.
  - `mygo.json` (`i18n.dir` = `locales`, source `zh`); 302 messages in
    `locales/zh.json` and `locales/en.json`, keyed by surface and with ICU
    placeholders for counts, statuses, ports and rule values.
  - `go tool mygo generate` writes `locales/catalog_gen.go`, which registers
    both languages and imports `zh-Hans`, so MyGo's own widget menus are
    Chinese in the Chinese window.
  - `ui/` and `internal/gui/`: every view, state message and status line now
    reads the catalog; `ProfileProblem`, `SettingsState.Apply`, the tray menu,
    the keep-alive summary and the tooltip included.
  - `core/prompt/web`: `Options.Language`, `text.go` with both languages, and
    the templates fed from it; an empty tag follows the POSIX locale
    environment, because `core/*` cannot import mygo.
  - Settings window: 语言 fieldset with 跟随系统/中文/English, bound to
    `SettingsState.LocaleIndex`; `SetLocale` runs on the next main-loop turn,
    outside the frame that saw the click.
  - `internal/guards/catalog_test.go`: CJK literals are forbidden in the
    production files of `ui/` and `internal/gui/`, and the two catalogs must
    carry the same keys.
  - `README.md` states both languages and the switch; `configuration.md` and
    `handoff.md` got English versions (the other two docs were already English).
- Files created/modified:
  - `mygo.json`, `locales/zh.json`, `locales/en.json`, `locales/catalog_gen.go`
  - `ui/{view,theme,setup,prompt,password,settings,render_test}.go`,
    `ui/locale_test.go`
  - `internal/gui/{gui,prompt}.go`
  - `core/prompt/web/{web,pages,challenges,page,text}.go`, `web_test.go`
  - `internal/guards/catalog_test.go`
  - `README.md`, `docs/configuration.en.md`, `docs/handoff.en.md`,
    `docs/configuration.md`, `docs/handoff.md`, `REUSE.toml`

### Phase 3: Design tokens

- **Status:** complete
- Actions taken:
  - Rendered every state before touching code and read the chip in light and
    dark; confirmed the audit's numbers: light Success on its 12% tint is
    2.89:1, Warning 2.81:1, Danger 4.01:1, Accent 4.37:1, all below the 4.5:1
    body threshold for an ~11.9px label.
  - `ui/theme.go`: `toneInk` derives the readable colour from the theme's own
    fill by mixing it toward black (light) or white (dark) in 5% steps until it
    clears `contrastFloor` (4.8:1, AA plus a margin for the step size) against
    `chipSurface` — the ink at 12% over the window. `contrastRatio` and
    `luminance` implement WCAG 2.1; no hex value is pinned, so a theme or
    desktop-accent change cannot reintroduce a failing colour.
  - Resulting ink, measured on the window and on the ink's own tint: light
    neutral #66666e (5.69 / 4.82), accent #2159d4 (6.10 / 5.10), success
    #0f7234 (6.04 / 5.06), warning #985304 (5.88 / 4.93), danger #bb2020
    (6.29 / 5.14); dark neutral #a1a1aa (6.91 / 5.66), accent #5895f7 (5.95 /
    5.02), success #22c55e (7.78 / 6.40), warning #f59e0b (8.25 / 6.71), danger
    #f26969 (5.91 / 5.02). Dark Success and Warning already passed, so they are
    unchanged.
  - `statusChip`: the dot and the 12% tint use the ink, the label reads in
    `Theme.Text` (13.10-15.01:1 on the tint), so the pill no longer depends on
    a fill carrying small text. `captionTone`, `detailLine`, the satisfied
    password rule and the footer's failure line now use `toneInk` too; the raw
    fill stays a surface only.
  - Removed both literal sizes: the settings window's no-profiles line keeps the
    text default, and the captcha marker's number goes through
    `textSize(captionScale)`.
  - `ui/a11y_test.go`: `TestToneInkPassesAA` asserts every tone's ink on its
    tint, on the window and the label on the tint at >= 4.5:1, in both
    appearances.
- Files created/modified:
  - `ui/theme.go`, `ui/view.go`, `ui/password.go`, `ui/settings.go`,
    `ui/a11y_test.go`

### Phase 4: Window chrome and main window information architecture

- **Status:** complete
- Actions taken:
  - Both windows: `TitleBarStyle: mygo.TitleBarHidden` with `TitleBarHeight:
    gotrustui.TitleBarHeight` (34); GTK keeps its own buttons. The main window's
    `MinHeight` is 480.
  - `ui.view`: `titleBar(c, profile)` draws the strip in the room `c.TitleBar()`
    reports, pads by `Left`/`Right`, drags with `DragWindow`, and shows
    `goTrust · 配置 默认` at caption size muted; it holds no control.
  - Header: `statusChip` is now the headline (title size, dot and tint in the
    tone ink, label in `Theme.Text`); one subline joins the endpoint, `内网地址`,
    and `已保持 N 分钟`, or the reconnect ladder / failure reason in its tone.
    `phaseDetail`, `profileSummary` and `detailLine` are gone.
  - Controls: `连接/断开` (primary), `复制配置 ▾` (`ui.MenuButton`: PAC, Clash
    片段, Verge 脚本, Clash 完整配置), `⋯` (设置, 打开日志目录, 退出). The row
    wraps so the overflow stays reachable at 440 DIP. The first-run panel hides
    the header's 连接 and the subline: the panel already owns both.
  - Footer: one line from `core/proxy`'s new `Listeners()` (`SOCKS5 … · HTTP …
    · 保活 N 秒前`), coloured only by a failing probe; the PAC URL and
    `KeepAliveSummary` sit in a `ui.TooltipBase` tip, and the anchor carries them
    as its description. `State.Status` is shown once as `ui.Toast`, gated by the
    new `State.StatusSeq` the app increments in `setStatus`/`setStatusError`.
  - Layout: the content column is `Grow(1).Shrink(1)` (not `Fill`, which is
    100% of the parent and overflowed by the strip's height); every scroll and
    the panel columns shrink, so the footer and pinned action rows stay on
    screen at the minimum size.
  - `ui.Actions.OpenLogs` wired to `app.openLogs` for the overflow menu.
  - `core/proxy`: `Addresses() []string` became `Listeners() []Listener`
    (`Protocol`, `Address`), with `boundListener` recording the protocol at
    bind; the CLI logs and the app's state follow.
- Files created/modified:
  - `ui/{view,theme,settings,password,setup}.go`, `ui/render_test.go`,
    `ui/view_test.go`, `ui/a11y_test.go`
  - `core/proxy/server.go`, `core/proxy/server_test.go`
  - `internal/gui/{gui,gui_test}.go`, `internal/cli/cli.go`
  - `locales/{zh,en}.json`, `locales/catalog_gen.go`

### Phase 5: Resource list

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 6: Settings window

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 7: Login panels

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 8: Keyboard, feedback, tray

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 9: Verification and docs

- **Status:** pending
- Actions taken: none
- Files created/modified: none

## Test Results

| Test | Input | Expected | Actual | Status |
|---|---|---|---|---|
| Render every state | `GOTRUST_RENDER_DIR=/tmp/gotrust-ui go test ./ui -run TestRenderEveryState -count=1` | every PNG, suite ok | 20 files, `ok … 0.309s` | pass |
| Render sizes at the declared minimum | `main-small.png`, `settings-small.png` | content visible at 440×400 and 560×440 | `main-small.png` shows the resource heading with zero rows | **fail — Phase 5 fixes** |
| First-run form at the minimum size | `main-setup-small.png` (440×400) | the form scrolls, the action row stays | fields scroll under a pinned 保存并连接 / 仅保存 row | pass |
| Contrast of the chip colours | `go test ./ui -run TestToneInkPassesAA` | ≥ 4.5:1 at ~11.9px in both appearances | light ink 5.06-6.29 on the window, 4.82-5.14 on the tint; dark 5.02-8.25 / 5.02-6.71; labels 13.10-15.01 | pass |
| Literal font sizes gone | `grep -n 'FontSize(1[0-9])' ui/*.go` | none | none | pass |
| Renders after the token change | `GOTRUST_RENDER_DIR=/tmp/gotrust-ui go test ./ui -run TestRenderEveryState -count=1` | 40 PNGs, chip legible in every phase | 40 files, `ok … 0.653s`; chip read in light online/idle/reconnecting/failed, dark online and the captcha marker | pass |
| First-run view tests | `go test ./ui -run 'TestProfileProblem\|TestSetup\|TestCompleteProfile'` | pass | `ok` | pass |
| First-run app tests | `go test ./internal/gui -run TestFirstRun` | pass | `ok` | pass |
| Whole suite | `go test ./...` | pass | all packages ok | pass |
| Header, menus and footer | `go test ./ui -run 'TestListenersAndKeepAliveAreListed\|TestSettingsButtonOpensTheWindow\|TestClashButtonsCopyEachFormat\|TestActionResultsShowAsToasts\|TestMainWindowTabOrder'` | menu paths reach the actions, tooltip holds the probe | pass | pass |
| Minimum size, 440×480 | `main-small.png`, `main-small-en.png`, `main-setup-small.png`, `main-password-small.png` | footer on screen, `⋯` reachable, form scrolls | footer intact, header wraps to two rows in English, form scrolls under its pinned row | pass |
| Window chrome in a real session | headless mutter + Xwayland, empty config, `import -window root` | strip above the content, no crash | strip shows `goTrust · 配置 默认`, window alive, no stderr beyond the appindicator deprecation notice | pass |
| Static checks | `go vet ./...`, `gofmt -l .`, `reuse lint`, `go tool mygo vet .` | clean | no output; reuse 198/198 | pass |
| CGO-free build of the UI packages | `CGO_ENABLED=0 go test ./ui ./internal/gui` | pass | `ok` | pass |

| Both-language renders | `GOTRUST_RENDER_DIR=/tmp/gotrust-ui-i18n go test ./ui -run TestRenderEveryState -count=1` | 40 PNGs, suite ok | 40 files, `ok … 0.654s` | pass |
| English first-run window | headless mutter + Xwayland, `LC_ALL=en_US.UTF-8 ./gotrust --config …` | English labels, no crash | English form captured; identical to the `LANG` run | pass |
| Catalog imports MyGo's widgets | `grep zh-Hans locales/catalog_gen.go` | one blank import | `_ "github.com/egoist/mygo/locales/zh-Hans"` | pass |
| Guard: no literals in views | `go test ./internal/guards` | no CJK literal, equal key sets | `ok` | pass |
| Locale switch | `go test ./ui -run TestSettingsLanguagePickerCallsSetLocale` | one `SetLocale("en")` | pass | pass |
| Web pages in English | `go test ./core/prompt/web -run 'TestPagesSpeakEnglish\|TestResolveLanguage'` | English page, success page, env fallback | pass | pass |

## Error Log

| Timestamp | Error | Attempt | Resolution |
|---|---|---|---|
| 2026-10-28 | `GET api.github.com/repos/egoist/mygo/contents/...` → 403 | 1 | Used the codeload tarball for the skill and the pinned module cache for the API surface |
| 2026-10-28 | `tar --strip-components=6` left the path as `skills/mygo-maintenance/`, so the first copy nested it one level deep | 1 | Recreated `.agents/skills/mygo-maintenance/` from the extracted contents and removed the stray tree |
| 2026-10-28 | `Segmented(...).OnClick` never fired: the tester click lands on the segment, not the track the handler was attached to | 1 | Bound `SettingsState.LocaleIndex` directly and read `.Changed()`, the same pattern as the profile picker |
| 2026-10-28 | `grep '"[^"]*\p{Han}[^"]*"'` matched middle dots; the catalog guard uses a Go AST and `unicode.Han` instead | 1 | Guard parses non-test files of `ui/` and `internal/gui/` |
| 2026-10-28 | Converting accessor names: `TOTPSection`, `SOCKS5Listen`, `PACURL` are generated as `TotpSection`, `Socks5Listen`, `PacURL` | 1 | Build errors named each; call sites fixed |
| 2026-10-28 | `settings-full` at 600×1500 still clips after 详细日志: the scroll content is taller than the view; the language field lies below the fold | 1 | Inspected the language field through a 600×2200 tester instead of a PNG |
| 2026-10-28 | The password hint rendered under the 保存密码 checkbox and read as if it explained the checkbox | 1 | Moved the caption inside the 密码 field, above the checkbox |
| 2026-10-28 | With the strip added, the footer and the pinned action rows fell below the window at every size | 1 | The content column used `Fill()`, which is 100% of the parent and ignored the strip above it; `Grow(1).Shrink(1)` sizes it to the remaining room |
| 2026-10-28 | The resource list still pushed the footer out at 440×480 | 1 | Nothing shrank: mygo's default is shrink 0. The list column, its `Scroll`, the panel columns and the settings body now shrink |
| 2026-10-28 | English at 440 DIP clipped the `⋯` button | 1 | The header row wraps; the actions take a second line only when they do not fit beside the headline |
| 2026-10-28 | The first-run form scrolled at 540×640 once the header grew | 1 | Dropped the subline (it repeated the panel) and the header's 连接 (the panel owns it); the form fits again |
