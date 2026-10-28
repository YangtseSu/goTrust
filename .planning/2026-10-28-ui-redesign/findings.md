# Findings: goTrust interface audit

Everything below is measured or read in this repository at the pinned versions
(mygo 0.7.1). Nothing here is a guess about how the app looks: each claim has a
file:line, a render, or a computed contrast ratio behind it.

## How the evidence was produced

```sh
GOTRUST_RENDER_DIR=/tmp/gotrust-ui go test ./ui -run TestRenderEveryState -count=1
```

`ui/render_test.go` writes every window state as a PNG at the real window sizes:
`main-*` at 540×640 (min 440×400), `settings-*` at 600×760 (min 560×440), and
`settings-full.png` at 600×1500 to show the whole form. The pre-redesign set is
kept in `/tmp/gotrust-ui-before`.

## Audit

| Symptom | Evidence | Consequence |
|---|---|---|
| State repeated three times | `statusChip` (`ui/theme.go:112`), the detail line from `phaseDetail` (`ui/view.go:213`), and the footer's listeners/keep-alive/`State.Status` (`ui/view.go:445`) | Nothing is the focus; the eye has no entry point |
| Nine equal-weight buttons | `controls` (`ui/view.go:321`): 连接/断开, 设置, 复制 PAC 地址, 退出, then Clash 片段/Verge 脚本/Clash 完整配置; see `main-online.png` | The primary action reads like an export button; 退出 sits next to 复制 |
| Resource list is unreadable | `resourceRow` (`ui/view.go:408`) puts two equal buttons on every row and leaves `ResourceEntry.Kind` inside the `Detail` string | With a real grant the window is a wall of buttons |
| Content disappears at the minimum size | `main-small.png` (440×400): the header, the resource heading with `5 项`, then nothing — every row is gone | The declared `MinWidth`/`MinHeight` are not usable |
| Settings is one 1450px column | `settings-full.png` (600×1500); `SettingsView` (`ui/settings.go:505`) wraps seven `Fieldset`s in one `Scroll` | Reaching 代理 or 保活 means scrolling past a screen and a half; no section navigation, no dirty marker |
| Login panels replace the window body | the `switch` in `View` (`ui/view.go:96-118`) swaps `controls` and `resources` for the panel | While answering a captcha the account, the gateway and the resources are all invisible |
| Captcha button row is a five-way choice | `captchaPanel` (`ui/view.go:284-315`): 撤销, 清空, 提交, 取消 in one row, 取消 unstyled beside 提交 | 取消 abandons the login and is the easiest control to mis-hit |
| Semantic colours fail AA | measured below | 已连接 green and the keep-alive warning are hard to read in the light appearance |
| The size scale contradicts itself | `ui/theme.go:19` documents four sizes; `ui/settings.go:529` hardcodes 13 and `ui/view.go:295` hardcodes 10 | Inconsistent weight at any DPI, and a desktop font setting does not move them |
| No keyboard path at all | `OnShortcut`/`Accelerator` appear nowhere under `ui/` or `internal/gui/` | The tray app is fully mouse-driven although mygo provides shortcuts and focus rings |
| Tray and window duplicate actions | `TrayMenu` (`ui/view.go:560+`) repeats 复制 PAC and three Clash forms that the window also renders | Two wordings for one action, two places to keep in sync |

## Contrast, measured

WCAG AA for body text is 4.5:1. The chip's label is `captionScale` × 14 ≈
11.9px, weight 600, so it needs the body threshold, not the large-text one. The
chip paints the label in the raw semantic colour on a 12% tint of that same
colour over the window background.

| Colour | mygo value | on background | on 12% tint |
|---|---|---|---|
| light Success | `#16a34a` | 3.30 | **2.89** |
| light Warning | `#d97706` | 3.19 | **2.81** |
| light Danger | `#dc2626` | 4.83 | 4.01 |
| light Accent | `#2563eb` | 5.17 | 4.37 |
| dark Success | `#22c55e` | 7.78 | 6.40 |
| dark Warning | `#f59e0b` | 8.25 | 6.71 |
| dark Danger | `#ef4444` | 4.71 | 4.19 |

The dark appearance is fine; the light one is not. `Success` and `Warning` are
Meter colours (`Theme` documents them as Meter levels) that the redesign uses as
text, which is the root cause — not a bad hex value.

Light-appearance candidates that pass on the tint, verified with the same
computation:

| Candidate | on background | on 12% tint |
|---|---|---|
| `#166534` (success text) | 7.13 | 5.95 |
| `#92400e` (warning text) | 7.09 | 5.88 |
| `#b91c1c` (danger text) | 6.47 | 5.29 |
| `#1d4ed8` (accent text) | 6.70 | 5.57 |

Dark-appearance candidates: `#4ade80` (8.00 on tint), `#fbbf24` (8.29),
`#f87171` (5.39).

### Resolved in Phase 3

The candidates above were a survey; the shipped tokens are derived, not pinned.
`toneInk` (`ui/theme.go`) takes the theme's own fill and mixes it toward black
(light) or white (dark) in 5% steps until it clears 4.8:1 against the chip's
tint — the ink at 12% over the window. Measured after the change:

| Tone | light ink | on window / tint | dark ink | on window / tint |
|---|---|---|---|---|
| neutral | `#66666e` | 5.69 / 4.82 | `#a1a1aa` | 6.91 / 5.66 |
| accent | `#2159d4` | 6.10 / 5.10 | `#5895f7` | 5.95 / 5.02 |
| success | `#0f7234` | 6.04 / 5.06 | `#22c55e` | 7.78 / 6.40 |
| warning | `#985304` | 5.88 / 4.93 | `#f59e0b` | 8.25 / 6.71 |
| danger | `#bb2020` | 6.29 / 5.14 | `#f26969` | 5.91 / 5.02 |

The dot and the tint use the ink; the label itself is `Theme.Text` on that tint
(13.10-15.01:1). `TestToneInkPassesAA` asserts all three pairs in both
appearances, so a mygo palette change or a desktop accent cannot slip a failing
colour back in.

## mygo 0.7.1 surface the redesign can use

Read from `~/go/pkg/mod/github.com/egoist/mygo@v0.7.1/ui/api_gen.go`:

| API | Signature | Use in the redesign |
|---|---|---|
| `MenuButton` | `MenuButton(c, label, func(m *Menu))` | `复制配置 ▾` and the `⋯` overflow |
| `SearchField` | `SearchField(c, query *string)` | filtering the resource list |
| `Tabs` / `TabsBase` | `Tabs(c, selected *int, labels ...string)` | the six settings pages |
| `Icon` | `Icon(c, s *SVG)` | five kind glyphs, inline SVG, no emoji |
| `Toast` / `ToastViewportBase` | `ToastBase(c, t)` | transient copy/save results |
| `Tooltip` | `TooltipBase(c, anchor, fn)` | listener and probe detail behind one footer line |
| `Badge` | `Badge(c, text)` | the `下次登录生效` markers |
| `Segmented` | `Segmented(c, selected *int, labels ...string)` | available if a two-way view switch is ever needed |

The app follows the desktop's light/dark appearance through mygo's
`ThemeModule`; there is no app-level `SetTheme` call, and the redesign does not
add one. Any new colour must therefore be derived for both appearances.

## Layout: Fill is not Grow

Measured while adding the title strip: `Element.Fill()` sets height to 100% of
the parent, so a content column under a 34px strip was 480 tall in a 480 window
and ended 34px past the bottom: the footer and the pinned action rows fell off
the screen at every size. `Grow(1).Shrink(1)` sizes the column to what the strip
leaves. Also, mygo's default flex-shrink is 0 (`layout.go` shrinks only items
whose `shrink` is set), so the list column, every `Scroll` and the panel columns
declare `Shrink(1)`; otherwise they keep their content height and push the
footer out.

## Constraints confirmed in the tree

* `internal/guards` forbids `core/*` reaching mygo and either front-end reaching
  the other; every change here is in `ui/` plus, where a callback is needed,
  `internal/gui/`.
* Views are pure functions of `State`; a redesign may add fields to `ui.State`
  but must not read the network from a view.
* `.agents/` is gitignored, so the installed `mygo-maintenance` skill and the
  `ui-ux-pro-max` skill never enter `reuse lint`.
* Window creation and hiding must stay outside their own frame
  (`docs/agent-ui-testing.md`); a redesign that adds a window would inherit that
  rule, which is one reason the login panels stay inline.

## First-run defect, traced through the code

Reported: on a fresh install 连接 says the gateway and account are missing, and
the only way to supply them is the settings window.

| Step | Where | What happens |
|---|---|---|
| 连接 | `Actions.Login` (`internal/gui/gui.go:376`) | `a.async(a.connect)` |
| login | `app.connect` (`gui.go:876`) → `a.manager.Login(ctx)` | no precondition check of its own |
| validation | `core/session/session.go:297` → `config.Profile.Validate` (`core/config/config.go:809`) | `errors.New("server address is empty")`, `"username is empty"`, port range |
| display | `async`'s failure branch (`gui.go:869`) → `a.setStatusError(err.Error())` | the English string lands in the footer, red |
| fix | `SettingsView` (`ui/settings.go:505`), reached only by 设置 (`gui.go:379`) or the tray's 设置… | the fields exist nowhere else |

Three separate problems in that path:

1. The window already knows the profile is incomplete — `profileSummary`
   (`ui/view.go:171`) renders `尚未填写网关` — but offers no way to fix it.
2. The validation text is English, because `core/config` is a library boundary
   with English errors and the GUI prints them raw. The auto-connect path
   (`gui.go:1525-1535`) already writes its own Chinese reasons, so the wording
   exists; only the manual path leaks.
3. The fix lives in a 1450px window with six unrelated sections, for three
   fields.

## Hidden title bar, mygo 0.7.1

`WindowOptions` (`window.go:90-135` in the pinned module):

| Field | Meaning |
|---|---|
| `TitleBarStyle` | `TitleBarDefault`, `TitleBarHidden`, `TitleBarHiddenInset` |
| `TitleBarHidden` | hides the frame, keeps the window controls, which sit over the view: the traffic lights on macOS, minimize/maximize/close at the top corner on Linux and Windows |
| `TitleBarHeight` | height of the strip the page draws; zero means Windows 11's 32 or the desktop header bar's height on Linux |
| `HideWindowButtons` | hides the controls too and keeps the shadow, the rounded corners and the resizing edges; the view then draws its own buttons |
| `Frameless` | removes the chrome entirely, including the resizing edges on Windows |

The view side:

* `c.TitleBar()` returns `TitleBar{Height, Left, Right}`, zero in other windows
  and in full screen. The documented pattern (`ui/context.go:357`) is
  `ui.Row(c).Height(max(bar.Height, 32)).Padding(0, bar.Right, 0, bar.Left).DragWindow()`.
* `Element.DragWindow()` sets `flagDragWindow`; `ui/input.go:365` starts a window
  drag only when the pressed target carries no other interactive flag, so a
  button inside the strip still receives its click while the empty space drags.
* On Linux the controls are GTK's own and the desktop's button layout decides
  which appear and on which side — `Left`/`Right` absorb that, which is why the
  strip pads by them instead of assuming a corner.

## First-run panel: what the render showed

* At 540×640 the form fits without scrolling: heading, one caption, four fields
  with their labels above the input (`ui.Field` stacks), the 保存密码 check box,
  then the action row. The header above it still says `配置 默认 ｜ 尚未填写网关`,
  which Phase 4 replaces with the state as the headline.
* At 440×400 the fields scroll under a pinned action row, because `setupPanel`
  uses the same `Scroll` + fixed row shape as `passwordPanel`. Phase 4's
  `MinHeight` change should make this the exception rather than the rule.
* `ui.Tester.Click` on a disabled button is a no-op, which is what makes the
  "an incomplete form is never submitted" assertion possible without inspecting
  the widget state.
* The footer still prints `代理端口尚未开启` under the form. It is correct and
  noise at the same time; Phase 4 collapses the footer to one line.

## Language survey (2026-10-28)

Question: what language does goTrust speak today, and should it be one language?

### Measured surfaces

| Surface | Language | Evidence |
|---|---|---|
| Tray app (`ui/`, `internal/gui`) | Chinese | 355 distinct Chinese string literals, 421 occurrences, in non-test Go |
| Web prompt pages (`core/prompt/web`) | Chinese | 70 CJK lines across `pages.go` and `challenges.go`, plus the inline templates |
| Headless CLI (`internal/cli`) | **English** | zero CJK in the package; every message is `fmt.Fprintf` English |
| `core/config`, `core/session` errors | English | library boundary; the first-run path used to surface them raw |
| `README.md` and `docs/*.md` | Chinese | README states 界面为中文 |

The product does not have one language. It has Chinese wherever a person looks
at a window or a document, and English wherever a person looks at a terminal or
an error from the library. The same user meets both in one session: the window
says 已连接 while a failed action prints `server address is empty`.

The first-run fix already forced the first boundary translation
(`ProfileProblem`). That is the shape of the whole problem: a string crossing
from `core/*` into the window has no owner.

### Cost, measured

* GUI: 355 distinct literals (`ui/settings.go` 105, `ui/view.go` 64,
  `internal/gui/gui.go` 53, `ui/password.go` 43, `ui/prompt.go` 31,
  `ui/setup.go` 20, `ui/theme.go` 3).
* Web prompt pages: 70 CJK lines.
* Docs: `README.md` plus six files under `docs/`.
* CLI: nothing to do; it is English and stays English.

### Decision

Two locales, zh-CN and en-US, chosen from the desktop environment and
overridable in the settings file. The catalog is keyed by the Chinese message
(as gettext keys by the source string), so a translation is one line next to its
source text, a missing translation falls back to Chinese instead of showing a
key, and a test can prove there is no Chinese literal outside the catalog.

Because most of those 355 strings are rewritten by Phases 3-7, the catalog goes
in first: after it exists, a redesign edits one catalog line per string instead
of hunting literals, and the translation survives.

## mygo 0.7.1 localization stack

Read from the pinned module, because Phase 2 uses it instead of a second
mechanism:

| Piece | Location | Behaviour |
|---|---|---|
| `i18n.Catalog` | `i18n/catalog.go` | One ARB-style JSON per language, `<tag>.json`, ICU MessageFormat; the source language's file holds every message and the metadata keys (`@@locale`, `@key`); a translation is read on first use and kept |
| `mygo generate` | `cmd/mygo` | With `i18n` in `mygo.json` (`{"dir": "locales", "source": "zh"}`), it validates the catalogs and writes typed accessors into the catalog directory as a Go package; without a frontend it writes no TypeScript |
| Generated accessors | `internal/i18ngen/testdata/app/catalog_gen.go` | `locales.Title()`, `locales.FilesSelected(count)`, each reading `i18n.Locale()`, plus `locales.In(tag)` for an explicit language; `init` calls `i18n.NewCatalog` and `i18n.Register`, which makes the catalog's languages the app's |
| `App.Locale` / `SetLocale` / `OnLocaleChanged` | `locale.go` | The desktop's languages on Linux are `LANGUAGE`, then `LC_ALL`, `LC_MESSAGES`, `LANG`; `SetLocale` shows a chosen tag at once and stores it (PathUserData), `""` follows the desktop again, and `MYGO_LOCALE` overrides one run |
| Language packs | `locales/{ja,zh-Hans,zh-Hant}` | mygo's own widget strings, which `ui` asks through `l10n.In(lang)`: the text field's 撤销/剪切/复制/粘贴 menu, the date picker, the token field |

Two consequences:

* goTrust links no pack today, so mygo's own menus are **English inside the
  Chinese window** — a second instance of the same defect, and one import fixes
  it.
* `internal/guards` judges the whole `go list -deps` graph, so `core/prompt/web`
  cannot import the generated `locales` package. The web pages take the language
  tag and keep their own 35 lines of text.
