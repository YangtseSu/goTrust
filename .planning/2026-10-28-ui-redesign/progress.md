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

- **Status:** complete
- Actions taken:
  - `ui/icons.go`: five kind glyphs (application grid, globe, stacked hosts,
    name book, unknown kind) and the three-dot row menu, all inline SVG parsed
    once at package init, stroked at mygo's own 24-grid weight.
  - `ui/view.go`: `resources` draws one group per `ResourceKind` in the fixed
    order 应用 / 域名 / IP 地址 / DNS, unknown kinds under 其他; headings read
    `应用 · 12 项` and count what the filter leaves. `groupResources` and
    `matchesResource` are pure and tested apart from the widget tree.
  - A row is one line: kind glyph, title, detail at the right, and a dots menu
    holding 打开 (disabled without a URL) and 复制. The visible 打开/复制
    buttons are gone, so each row owns one control instead of three.
  - `ui.SearchField` appears when any kind holds more than
    `resourceSearchAt` (8) rows; it lives in the field's own `ui.Local` state
    and matches title or detail case-insensitively. An empty match says
    没有匹配的资源.
  - `ui.Scroll` gained `MinHeight(resourceListMinHeight)`: three rows at the
    theme's size, so the 440-wide list no longer collapses to its heading.
  - Catalog: `resourceGroupApp`, `resourceGroupDomain`, `resourceGroupAddress`,
    `resourceGroupDNS`, `resourceGroupOther`, `resourceGroupCount`,
    `resourceSearch`, `resourceSearchEmpty`, `resourceMenu` in both languages;
    `resourceOpen`/`resourceCopy`, which only labelled the removed buttons, are
    gone. `go tool mygo generate` regenerated the accessors.
  - Render harness: `save` now calls `tester.SetLocale(lang)`, so MyGo's own
    widget strings are in the language of the images; without it the Chinese
    window drew the search placeholder in English.
  - Tests: grouping and counts, an unknown kind, the row menu reaching both
    actions and 打开 being disabled without a URL, the filter appearing at the
    ninth row and filtering 9+1 rows, and three rows plus the footer at
    440×480.
- Files created/modified:
  - `ui/icons.go` (new)
  - `ui/view.go`
  - `ui/view_test.go`
  - `ui/render_test.go`
  - `locales/zh.json`, `locales/en.json`, `locales/catalog_gen.go`
  - `core/session/resources.go` (the `Copy` field's comment)
  - `.planning/2026-10-28-ui-redesign/task_plan.md`

### Phase 6: Settings window

- **Status:** complete
- Actions taken:
  - Rendered the window before the change: one 1450px form, seven fieldsets,
    one caption at the top saying when all of it takes effect, no dirty state.
  - `SettingsState.Tab` and six page constants; `settingsTabBar` draws the
    pages with `ui.TabsBase` in a wrapped row (same look as `ui.Tabs`: 12px tab
    padding, a 2px accent underline under the choice, the widget's focus ring),
    because the six English labels overflow the 560px minimum at the default
    text size and more so with larger text.
  - `profileBar` moves the picker and 新建/复制/删除 out of the form into a
    fixed row above the tabs; the 名称 field stays on 连接.
  - `settingsPage` switches on the tab; each page draws its effect as one line
    (`settingsEffectLogin`, `settingsEffectConnect`, `settingsEffectStartup`,
    `settingsEffectAdvanced`, `settingsEffectLive`) and one `ui.Form` of fields
    capped at `fieldWidth` (360), with the method picker at 240 beside
   获取认证方式. Rows whose label and action can outgrow the line wrap.
  - `settingsHint` is gone; `settingsUnsaved`, `settingsDiscard`,
    `settingsDiscarded`, the five effect lines and `terminalsTab` are new;
    `profileSection` and `terminalsSection` are gone. The English
    `passwordSecurity` said "Password", the same as the field above it, and now
    says "Password security".
  - `SettingsState.Changed()` compares the folded draft with the loaded file:
    `folded()` is `Apply` without the checks. `SettingsActions.Discard` reloads
    the draft; the bottom bar's 关闭 is gone, and 保存/放弃修改 disable while
    nothing differs.
  - 终端管理 is its own page; 语言 moved to the bottom of 启动.
  - Tests: `Changed()` semantics, the profile bar on every page, save/discard
    following the draft, the tab order reaching 保存 then 放弃修改 (with a
    draft edit, because a disabled control takes no focus), the language
    picker and the terminal page through their tabs, and one render per page
    plus a 560px-wide page.
- Files created/modified:
  - `ui/settings.go`, `ui/settings_test.go`, `ui/render_test.go`,
    `ui/a11y_test.go`, `ui/locale_test.go`
  - `internal/gui/gui.go` (`Discard`)
  - `locales/zh.json`, `locales/en.json`, `locales/catalog_gen.go`
  - `README.md`

### Phase 7: Login panels

- **Status:** complete
- Actions taken:
  - `panelSlot` wraps every pre-login panel (choice, captcha, code, SSO,
    device bind, password, first run) in the window body: the header keeps the
    state and its controls, and `grantedLine` pins the count of granted
    resources to the bottom of the panel, just above the footer. A session with
    no grants draws no line.
  - `panelActions` is one shared foot for the panels: 取消登录 on the left,
    through `textButton`, and the panel's own answers right-aligned with the
    primary last. The choice panel passes no answers, so its 取消登录 stays on
    the left of an otherwise empty row.
  - `textButton` (`ui/theme.go`) is `ui.ButtonBase` with the theme's padding,
    radius and a hover tint; the label is body text, so 撤销 and 清空 read as
    corrections rather than as two more buttons.
  - `panelActions` draws 取消登录 even when no cancel callback is wired and
    disables it instead: the render harness passes `Actions{}`, and the row it
    writes should be the row the app draws.
  - The captcha marker is 26px (`captchaMarkerSize`), the web prompt page's
    disc, with its number at `captionScale`. 取消 left the picked-count row,
    which is now its own caption, and 撤销/清空 became text buttons.
  - 取消登录 is a new catalog key (`cancelLogin`); `resourcesGranted` carries the
    count line's plural. `go tool mygo generate` rewrote the accessors.
  - Tests: the code panel keeps 连接中 and 已获授权 3 项资源 while its question is
    up; 取消登录 sits left of 提交, and in the captcha panel left of 撤销, which
    is left of 提交. The existing panel tests now click 取消登录; the settings
    password panel keeps 取消 because it is not a login.
  - One new render, `main-code-granted`, where a code challenge meets an
    account with five rows, so the count line is visible above the footer.
- Files created/modified:
  - `ui/view.go`, `ui/prompt.go`, `ui/password.go`, `ui/theme.go`
  - `ui/prompt_test.go`, `ui/captcha_test.go`, `ui/password_test.go`,
    `ui/render_test.go`
  - `locales/zh.json`, `locales/en.json`, `locales/catalog_gen.go`

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
| Render sizes at the declared minimum | `main-small.png`, `settings-small.png` | content visible at 440×400 and 560×440 | `main-small.png` showed the resource heading with zero rows; Phase 5 closed it with `main-resources-small.png` at 440×480, which shows five rows and the footer | pass |
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
| Grouped resource list | `go test ./ui -run 'TestResourceListRendersAndActs\|TestResourceListGroupsUnknownKinds'` | headings with counts, row menu reaches 打开 and 复制, 打开 disabled without a URL | pass; the unknown kind lands under 其他 | pass |
| Filter threshold and filtering | `go test ./ui -run TestResourceSearch` | no field at eight rows, a field at nine, typing narrows rows and headings, empty match explained | pass | pass |
| Three rows at the minimum | `go test ./ui -run TestResourceListKeepsThreeRowsAtTheMinimumSize` (440×480, 20 rows) | three rows and the footer on screen | pass | pass |
| Rendered long grant | `main-resources.png`, `main-resources-small.png`, `main-resources-en.png` | groups, glyphs, counts, filter, quiet row menus | 9 rows at 540×640, 5 at 440×480 with the footer intact; the Chinese placeholder reads 搜索 and the English one Search | pass |
| Renders in both themes | `main-online.png`, `main-online-dark.png` | glyphs and counts legible in light and dark | pass | pass |
| Whole suite after Phase 5 | `go test ./...`, `go vet ./...`, `gofmt -l .`, `go tool mygo vet .`, `reuse lint` | clean | all packages ok; 208/208 files with license information | pass |
| CGO-free UI build after Phase 5 | `CGO_ENABLED=0 go test ./ui ./internal/gui ./internal/cli` | pass | `ok` | pass |
| Every settings page rendered | `GOTRUST_RENDER_DIR=/tmp/gotrust-ui-phase6 go test ./ui -run TestRenderEveryState -count=1` | six pages, the unsaved state and a 560px-wide page, both languages | 56 PNGs; the tab row wraps to two lines in English at 560, the Chinese one stays on one line | pass |
| The unsaved marker follows the file | `go test ./ui -run 'TestSettingsChanged\|TestSettingsSaveAndDiscard'` | clean draft, padded values and the language stay clean; a port, a picker switch and a delete are marked | pass | pass |
| Save and discard in the window | `go test ./ui -run 'TestSettingsViewRendersAndCallsActions\|TestSettingsProfileBarStaysOnEveryPage'` | pages are exclusive, 新建 enables 保存, 放弃修改 reaches its action | pass | pass |
| The actions stay reachable with Tab | `go test ./ui -run TestSettingsTabOrderReachesTheActions` (with an edited draft) | Tab reaches 保存 then 放弃修改 | pass; a disabled control takes no focus, so the test edits first | pass |
| The real window, six pages | headless mutter (Wayland) + mygo's automation plugin: 更多设置… → the six tabs → port 8443 | six tabs in order, every page drawn, marker on after the edit | `tabs ['连接', '代理', '保活', '启动', '高级', '终端']`, 10 captures, app exit 0 | pass |
| 放弃修改 and 保存 in the real window | same run | discard restores 443 and clears the marker; save writes the file and clears the marker | `port after discard: 443`; `config.toml` then reads `port = 8443` | pass |
| A panel keeps the header and the grant | `go test ./ui -run TestLoginPanelKeepsHeaderAndGrantedCount` | 连接中 and 已获授权 3 项资源 both on screen | pass | pass |
| Action hierarchy | `go test ./ui -run TestPanelActionsPutCancelAwayFromSubmit` | 取消登录 left of 提交; captcha order 取消登录 < 撤销 < 提交 | pass | pass |
| Every state rendered | `GOTRUST_RENDER_DIR=/tmp/gotrust-ui-phase7 go test ./ui -run TestRenderEveryState -count=1` | every PNG, both languages, panel rows with 取消登录 | 58 PNGs; `main-code-granted` shows the count line above the footer | pass |
| Panel layouts read | `main-captcha.png`, `main-code-granted-en.png`, `main-sso-en.png`, `main-choice.png`, `main-password-small.png` | cancel left, plain 撤销/清空, 26px markers, grant line | the captcha row reads 取消登录 … 撤销 清空 提交; the count line sits above the divider | pass |
| Whole suite after Phase 7 | `go test ./...`, `go vet ./...`, `gofmt -l .`, `go tool mygo vet .`, `reuse lint` | clean | all packages ok; 208/208 files with license information | pass |
| CGO-free UI build after Phase 7 | `CGO_ENABLED=0 go test ./ui ./internal/gui ./internal/cli` | pass | `ok` | pass |

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
| 2026-10-28 | The Chinese render of the new filter drew its placeholder as "Search" | 1 | The renders set only `MYGO_LOCALE`, which the app's catalog reads; `Tester.lang`, which MyGo's own widget strings read, stayed empty. `save` now calls `tester.SetLocale(lang)`; the placeholder reads 搜索 |
| 2026-10-28 | `ui.MenuButton` per row put a bordered button with a dropdown arrow beside every entry | 1 | `Element.Menu` on a three-dot glyph: same menu, one icon, no arrow |
| 2026-10-28 | `resourceListMinHeight` mixed an untyped const with `Theme.FontSize` and did not compile | 1 | `line := float32(resourceIconSize)` before comparing |
| 2026-10-28 | The row-menu test could not click a second `⋯`: `Tester.Click` takes the first element with that text, which is the header's overflow | 1 | Each row menu carries `Label("<title> 的操作")`, its accessible name, which is also what the test clicks |
| 2026-10-29 | The smoke-test client hung: `act press` on the header's ⋯ opened a GTK popup with a loop of its own, and the app stopped answering the socket | 1 | Opened the settings window from the first-run panel's 更多设置… (a plain button) instead; an open context menu is out of the protocol's reach |
| 2026-10-29 | `panelActions` omitted 取消登录 when the callback was nil, so the render harness's `Actions{}` drew a row the app never draws | 1 | The control is always drawn and disabled without a callback, like the header's primary button, whose presence does not depend on its wiring |
