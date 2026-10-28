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
    and dragged with `DragWindow` — recorded in the plan, implemented in Phase 3.
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

### Phase 2: Design tokens

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 3: Window chrome and main window information architecture

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 4: Resource list

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 5: Settings window

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 6: Login panels

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 7: Keyboard, feedback, tray

- **Status:** pending
- Actions taken: none
- Files created/modified: none

### Phase 8: Verification and docs

- **Status:** pending
- Actions taken: none
- Files created/modified: none

## Test Results

| Test | Input | Expected | Actual | Status |
|---|---|---|---|---|
| Render every state | `GOTRUST_RENDER_DIR=/tmp/gotrust-ui go test ./ui -run TestRenderEveryState -count=1` | every PNG, suite ok | 20 files, `ok … 0.309s` | pass |
| Render sizes at the declared minimum | `main-small.png`, `settings-small.png` | content visible at 440×400 and 560×440 | `main-small.png` shows the resource heading with zero rows | **fail — Phase 4 fixes** |
| First-run form at the minimum size | `main-setup-small.png` (440×400) | the form scrolls, the action row stays | fields scroll under a pinned 保存并连接 / 仅保存 row | pass |
| Contrast of the chip colours | computed from the mygo palette | ≥ 4.5:1 at ~11.9px | light Success 2.89, Warning 2.81, Danger 4.01 | **fail — Phase 2 fixes** |
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

### Language survey, folded into Phase 9

- **Status:** complete (survey only; the work is Phase 9)
- Actions taken:
  - Counted goTrust's own surfaces: 355 distinct Chinese literals in the GUI
    (421 occurrences), 70 CJK lines in the web prompt pages, zero in
    `internal/cli`, which is English.
  - Checked the protocol upstream: bilingual README and docs, English CLI
    (`zju-connect -h`), English logs.
  - Checked the Qt GUI built on that core (EZ4Connect): Chinese README, docs and
    walkthrough; no translations directory in its tree listing.
  - Inspected the installed official client (2.5.16.41): two UI languages,
    zh-CN and en-US — 122 pairs in its main-process language service, a
    2651-entry Chinese→English renderer dictionary, `web/locale/{zh-CN,en-US}`.
    Its Electron `locales/*.pak` are Chromium's, not the product's.
  - Decided: localize to zh-CN + en-US, catalog with stable keys, locale from
    the environment with a settings override, CLI stays English, work lands
    after the redesign phases.
- Files created/modified:
  - `.planning/2026-10-28-ui-redesign/task_plan.md` (Phase 9, decisions, key
    questions, not-doing)
  - `.planning/2026-10-28-ui-redesign/findings.md` (language survey)
