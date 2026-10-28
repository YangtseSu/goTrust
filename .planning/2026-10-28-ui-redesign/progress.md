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

- **Status:** pending
- Actions taken:
  - Counted the surfaces: 355 distinct Chinese literals in the GUI (421
    occurrences), 70 CJK lines in the web prompt pages, zero in `internal/cli`.
  - Recorded that the product already speaks two languages: Chinese in the
    windows and documents, English in the CLI and every library error.
  - Chose mygo's own catalog system over a hand-rolled table, and read it out of
    the pinned module: JSON catalogs, generated accessors that read
    `i18n.Locale()`, `SetLocale` persistence, the `zh-Hans` widget pack.
  - Found the second instance of the same defect: mygo's own widget menus are
    English inside the Chinese window because no language pack is linked.
  - Rewrote the phase around that mechanism: source `zh`, `mygo.json`,
    `locales/{zh,en}.json`, generated accessors, the settings switch, the web
    pages carrying their own text, and a guard against new CJK literals.
- Files created/modified:
  - `.planning/2026-10-28-ui-redesign/task_plan.md`
  - `.planning/2026-10-28-ui-redesign/findings.md`

### Phase 3: Design tokens

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 4: Window chrome and main window information architecture

- **Status:** pending
- Actions taken: none
- Files created/modified: none

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
| Contrast of the chip colours | computed from the mygo palette | ≥ 4.5:1 at ~11.9px | light Success 2.89, Warning 2.81, Danger 4.01 | **fail — Phase 3 fixes** |
| First-run view tests | `go test ./ui -run 'TestProfileProblem\|TestSetup\|TestCompleteProfile'` | pass | `ok` | pass |
| First-run app tests | `go test ./internal/gui -run TestFirstRun` | pass | `ok` | pass |
| Whole suite | `go test ./...` | pass | all packages ok | pass |
| Static checks | `go vet ./...`, `gofmt -l .`, `reuse lint`, `go tool mygo vet .` | clean | no output; reuse 198/198 | pass |
| CGO-free build of the UI packages | `CGO_ENABLED=0 go test ./ui ./internal/gui` | pass | `ok` | pass |

## Error Log

| Timestamp | Error | Attempt | Resolution |
|---|---|---|---|
| 2026-10-28 | `GET api.github.com/repos/egoist/mygo/contents/...` → 403 | 1 | Used the codeload tarball for the skill and the pinned module cache for the API surface |
| 2026-10-28 | `tar --strip-components=6` left the path as `skills/mygo-maintenance/`, so the first copy nested it one level deep | 1 | Recreated `.agents/skills/mygo-maintenance/` from the extracted contents and removed the stray tree |
| 2026-10-28 | The password hint rendered under the 保存密码 checkbox and read as if it explained the checkbox | 1 | Moved the caption inside the 密码 field, above the checkbox |
