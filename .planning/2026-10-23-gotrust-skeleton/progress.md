# Progress Log

## Session: 2026-10-23

### Phase 1: License and layout decision

- **Status:** complete
- **Started:** 2026-10-23
- Actions taken:
  - Read `zju-connect/LICENSE` and the FSF appendix; confirmed AGPL-3.0-or-later.
  - Verified GPLv3 §13 permits GPL + AGPL combination in one direction only.
  - Read upstream `service.*` signatures and confirmed the absence of a context-based shutdown.
  - Compiled an external module against the fork to prove the datapath is assemblable from public packages.
- Files created/modified:
  - none in this repository yet

### Phase 2: Module scaffold

- **Status:** complete
- Actions taken:
  - fetched the AGPL-3.0 text into `LICENSE`
  - added `go.mod` with the fork `replace`, `.gitignore`, `README.md`
  - wrote `config`: defaults, XDG path, atomic save with 0600, validation, interval fallback
- Files created/modified:
  - `go.mod`, `LICENSE`, `.gitignore`, `README.md`
  - `config/config.go`, `config/config_test.go`

### Phase 3: Session layer

- **Status:** complete
- Actions taken:
  - assembled underlay → aTrust client → stack → resolver → dialer from public packages only
  - added `Phase`/`State`, transition fan-out, `client_data` read/write at 0600
  - put credentials and settings behind functions so a login uses what the form shows
  - wired the upstream keep-alive with named and derived targets
- Files created/modified:
  - `session/session.go`, `session/session_test.go`

### Phase 4: Proxy layer

- **Status:** complete
- Actions taken:
  - wrote `proxy.Server` over an injected `DialFunc`, with SOCKS5 and an HTTP proxy
  - implemented CONNECT tunneling and absolute-URI forwarding, dropping hop-by-hop headers
  - gave both listeners context-based shutdown and a Close that is safe twice
  - made a failed second bind close the first listener
- Files created/modified:
  - `proxy/server.go`, `proxy/server_test.go`

### Phase 5: UI shell

- **Status:** complete
- Actions taken:
  - built the mygo app: single instance, tray with a state-dependent menu, one window
  - wrote the view as a pure function of state, with `ui.TextInput` fields and a masked password
  - added `--no-ui` for headless runs and a prompter that refuses instead of blocking
  - switched teardown to `Quit` + `OnQuit` so the lock and listeners are released
- Files created/modified:
  - `main.go`, `ui/view.go`, `ui/view_test.go`, `ui_atomic.go`, `icon.go`

### Phase 6: Publication

- **Status:** complete
- Actions taken:
  - privacy review of the whole tree and of every added line
  - replaced an operator-specific intranet address in tests with a documentation address
  - removed a local service unit name and a sibling checkout path from the planning notes
  - purged the unreachable blobs that the pre-sanitisation `git add` had left in the object store
  - created the repository private and pushed `main`
- Files created/modified:
  - none beyond the sanitisation edits already listed

### Phase 7: Verification

- **Status:** in_progress
- Actions taken:
  - `go build`, `go vet`, `gofmt` clean; all four test packages pass
  - proxy tests cover SOCKS5, HTTP CONNECT, forwarding, direct hit, double Close, bind failure
  - launched the binary: tray appears, window opens, SIGTERM removes the single-instance socket
  - cross-compiled to windows/amd64, darwin/arm64, linux/arm64 with `CGO_ENABLED=0`
- Remaining:
  - a live login, a proxied request and the keep-alive against a real gateway

### Transparent proxy research

- **Status:** complete (recorded, implementation deferred to Phase 8)
- Actions taken:
  - compared dae and goTrust on licence, platform, kernel, privilege and API shape
  - read `example.dae`, `proxy-protocols.md`, `how-it-works.md` and the kernel requirements
  - confirmed dae's socks5 outbound syntax and the three configuration traps
  - found and fixed the SOCKS5 resolver bug while reading the outbound path
- Files created/modified:
  - `proxy/server.go`, `proxy/server_test.go`, `main.go` (resolver injection)
  - `.planning/2026-10-23-gotrust-skeleton/findings.md` (research record)

### Click captcha

- **Status:** complete
- Actions taken:
  - read the reference implementation in `EZ4Connect`
    (`GraphCaptchaWindow`, `CaptchaCanvas`) and the upstream
    `authchallenge` flow: aTrust sends an image and expects
    `{"coordinates":[[x,y],…],"width":W,"height":H}`
  - added `ui.Captcha` with the source image, its pixel size and the picked
    points; display size keeps the aspect ratio and clicks map back to image
    pixels
  - drew the panel in the main window: numbered markers, 撤销/清空/提交/取消,
    and the login fields are hidden while the prompt is up
  - changed `session.Options.Prompt` to take the login context, so cancelling
    the login releases a handler that is waiting for the user
  - added `uiPrompter` in `prompt.go`: decodes the image, publishes the panel,
    blocks the login goroutine on a buffered channel, and converts the points
    to `authchallenge.ClickCaptchaResponse`
  - `--no-ui` still refuses the click captcha, but now says the window is
    required
  - raised the async deadline to 5 minutes so the captcha wait fits
  - rendered the panel with the mygo test renderer and verified the numbered
    markers and button row visually; launched the binary and stopped it with
    SIGTERM
- Files created/modified:
  - `ui/captcha.go`, `ui/view.go`, `ui/captcha_test.go`
  - `session/session.go` (`Prompt` signature)
  - `main.go`, `prompt.go`, `prompt_test.go`
  - `README.md`, this plan

### Live gateway verification

- **Status:** complete
- Account: the operator supplied a disposable management account; the only
  second factor was the click captcha.
- Actions taken:
  - reached the gateway and got the click captcha; the operator solved it in
    the real window and the login completed (IP `203.0.113.4`)
  - verified the derived keep-alive target (`http://192.0.2.10/`, the one
    granted network application) and both proxies against it: HTTP and SOCKS5
    each returned 200 with the intranet page
  - restarted the app; the stored sid ticket restored the session without a
    new login or captcha
  - found the gateway needs login domain `local`; an empty domain fails with
    `auth type/login domain combination not found`
  - found two lifetime bugs that only a real session exposes:
    - `session.Login` derived the session from the login-attempt context, which
      the app's async helper cancels on return; the keep-alive saw one round
      and stopped. The session now uses `context.WithoutCancel`.
    - `app.connect` passed the same attempt context to the proxy server; the
      SOCKS5 listener closed right after login while the HTTP listener stayed
      (no context shutdown). The app now owns a proxy context and cancels it on
      disconnect; both listeners honour their context.
  - added the 登录域 field; `currentConfig` and `currentAuth` both apply it
- Notes:
  - the remote-DNS keep-alive fallback is not reachable on this gateway: the
    account is granted one network application, so the derived HTTP target is
    the path that runs. The fallback stays untested; it is explicitly a
    degraded mode in the upstream code.
  - the session context fix is verified in the live logs: repeated
    `KeepAlive: OK status=200` rounds at the 15 s interval (10+ over about two
    minutes), interleaved with session refreshes.

## Session: 2026-10-24

### Product roadmap planning

- **Status:** complete
- Actions taken:
  - reviewed `EZ4Connect` for auth-method discovery, one-time-code and SSO
    dialogs, settings keys, tray, autostart and the reconnect ladder
  - reviewed `zju-connect` for the public `GetAuthInfoList`, challenge types,
    TOTP secret generation, SSO callback validation, resource structs and
    keep-alive derivation
  - reviewed `atrust-quadlet` for `--apps` publication, status/probe discipline,
    TOTP handling and diagnostics
  - reviewed `atrust-keepalive` for real-HTTP probes, 90 s cadence, target
    discovery, failure surfacing and systemd user units
  - checked mygo's `SetOpenAtLogin`/`WasOpenedAtLogin` and tray menu fields
  - checked the `~/projects/arch-pkgs` PKGBUILD conventions
  - recorded the new phases 11-16 in `task_plan.md`; dae stays deferred last
- Files created/modified:
  - `.planning/2026-10-23-gotrust-skeleton/task_plan.md`
  - `.planning/2026-10-23-gotrust-skeleton/findings.md`
  - `.planning/2026-10-23-gotrust-skeleton/progress.md`
- No application code changed in this session.

### Logging, config shape, profiles, REUSE and the package

- **Status:** complete for the package and REUSE; Phase 11 implementation pending
- Decisions:
  - logging: yes. Line log under `$XDG_STATE_HOME/goTrust/goTrust.log`
    (0600, 5 MiB × 3 rotation), stderr for `--no-ui`, a verbose level, secret
    redaction, and a settings shortcut to the directory
  - config: JSON stays, versioned as v2. `active_profile`, global
    `app`/`proxy`/`logging`, and `profiles[]`; a profile owns server, auth,
    TOTP and keep-alive. Proxy ports stay global so a switch does not move the
    listeners. v1 migrates to one `默认` profile
  - profiles: yes, one active session. Create/rename/duplicate/delete, a tray
    submenu, and a switch disconnects first and optionally reconnects
  - REUSE: AGPL-3.0-or-later, SPDX headers, `LICENSES/`, `REUSE.toml`;
    `reuse lint` is the gate
  - package: self-use `gotrust-git`, not AUR. makepkg runs git with
    `GIT_CONFIG_GLOBAL=/dev/null`, so the private GitHub HTTPS clone cannot use
    the credential helper; the package builds from the local checkout and the
    GitHub URL is the one-line switch later
- Actions taken:
  - answered the five design questions and folded the decisions into
    `task_plan.md` phases 11 and 16
  - added `LICENSES/AGPL-3.0-or-later.txt`, `REUSE.toml` and SPDX headers to
    every Go file; `reuse lint` reports 24/24 files
  - created `~/projects/arch-pkgs/gotrust-git/` with `PKGBUILD`, `REUSE.toml`,
    `goTrust.desktop`, 256×256 and 32×32 icons
  - built `gotrust-git r7` from the local `main`; `namcap` only
    reports the intentional dlopen/external-linker warnings; extracted the
    package and launched the binary (ready line, SIGTERM exit 0)
  - installed nothing: the host has no passwordless sudo; `pacman -U` is left
    for the operator
- Privacy review before the push:
  - rewrote the three local commits so the intermediate history no longer
    contains the live tunnel/intranet addresses; the outgoing diff, tree and
    full history have no credentials, gateway address, account, absolute home
    path or analysis-process wording
  - pushed `main` to `https://github.com/YangtseSu/goTrust.git`
  - the REUSE/package commits follow in a second push after the operator's
    explicit approval

### Phase 11: Config v2, profiles, settings, startup and tray

- **Status:** complete
- Decisions:
  - config is TOML (`BurntSushi/toml`), version 2: global `app`/`proxy`/
    `logging`, `[[profiles]]` with server/auth/keep-alive/debug. Proxy ports
    stay global
  - no v1 migration: v1 was never released and the only `config.json` found on
    this machine holds defaults. The JSON file is ignored; a newer `version`
    is refused. `.gitignore` now covers `config.toml`
  - client data is per profile (`client_data-<slug>.json`; `默认` keeps
    `client_data.json`), so one ticket cannot restore another account
  - the last keep-alive result arrives by observing the upstream `KeepAlive:`
    log lines on the shared writer; the fork hook is Phase 14's job
  - keep-alive default is 90 s, not 60 s (operator preference)
- Actions taken:
  - rewrote `config` for TOML v2, profile CRUD, `Clone`, per-profile client
    data paths, defaults (domain `local`), validation and atomic 0600 writes
  - added `logging`: 0600 line log under `$XDG_STATE_HOME/goTrust`, 5 MiB × 3
    rotation, redaction of passwords/cookies/tokens/coordinates, stderr mirror,
    `OnLine` hook, and the stdlib `log` bridge for the aTrust client
  - `session` now snapshots the active profile per login instead of reading a
    flat config; `Options.Auth` is gone, and client data is read/written per
    profile. Keep-alive enablement, interval and targets come from the profile
  - main window reduced to status, profile summary, connect, settings and quit;
    captcha panel unchanged
  - new settings window (`ui/settings.go`): profile picker plus create, copy,
    rename and delete; connection, proxies, keep-alive, startup and advanced
    groups. The draft is a clone, so closing discards edits; `Apply` validates
    names, ports, interval and listeners but allows an incomplete account
  - tray menu: disabled status line, connect/disconnect, profile radio
    submenu, show window, settings, quit, tooltip with the keep-alive result
  - startup: autostart reconciled with `SetOpenAtLogin`, `WasOpenedAtLogin`
    hides the window, auto-connect decision with a visible reason, captcha
    reveals the window, and a failed phase reveals it too
  - `async` now registers its cancel, so 断开 cancels a login that is waiting
    on the network or a captcha
  - the settings window is created and hidden outside the frame it was
    triggered from. Opening it from the 设置 button inside a frame made the
    second GtkGLArea fail ("OpenGL 0.0 is too old", "native UI shows
    nothing"), and hiding it from the 关闭 action segfaulted inside GTK
    through mygo's purego callbacks. `Settings` posts `openSettings` through
    `RunOnMain`, and `hideSettings` posts the hide after the frame with an
    epoch so a reopen is not hidden by a stale request. Reopening reloads the
    saved draft
  - README rewritten for TOML, profiles, settings, logs, tray and startup
- Verification:
  - config/logging/session/ui/app tests; `go vet`, `gofmt`, `go test -race`
  - headless run with temp `XDG_*`: created the 0600 log, failed with
    `server address is empty`, exit 1
  - real binary with a temp `XDG_CONFIG_HOME` and `autostart = true`:
    `goTrust ready`, XDG autostart entry written, SIGTERM exit 0, no process
    left
  - auto-connect without a saved password: the log records the decision reason
  - main and settings windows rendered headlessly and inspected; the settings
    window scrolls its groups and keeps 保存/关闭 visible
- Notes:
  - `go mod tidy` cannot run offline here (test deps of deps are not cached);
    the TOML requirement was moved to the direct block by hand
  - the live gateway was not re-run: the protocol path was verified in Phase 9
    and this phase changes wiring, not the aTrust calls

## Test Results

| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| External module compiles against fork | `go build` in `/tmp/embed-spike` | compiles, no `internal/` import | compiled; ran to "IP not available" | pass |
| mygo tray + window + webview + secrets | spike binary on Wayland | all four report OK | all four OK | pass |
| CJK render | `ui.NewTester` 520x300 | glyphs present | texts correct, PNG rendered | pass |
| SOCKS5 tunnel | greeting + CONNECT + echo | bytes return | `hello` echoed | pass |
| HTTP CONNECT | CONNECT + echo | bytes return | `tunnelled` echoed | pass |
| HTTP forwarding | `http.Client` with the proxy | body returns | `intranet ok` | pass |
| Direct hit | `GET /` on the proxy port | 400 | 400 | pass |
| Bind failure cleanup | HTTP bind on an occupied port | no listeners left | none left | pass |
| Config round-trip | save then load | equal values, 0600 | equal, 0600 | pass |
| UI phases | four phases rendered | label per phase | labels correct | pass |
| Button wiring | click per phase | right callback | right callback | pass |
| Password masking | `hunter2` in the field | not readable | rendered as dots | pass |
| Graceful quit | SIGTERM | exits, socket removed | exits, socket removed | pass |
| Cross-compile | 3 targets, `CGO_ENABLED=0` | builds | builds | pass |
| SOCKS5 resolver injection | CONNECT with a hostname | injected resolver asked | asked with the hostname | pass |
| Captcha pixel mapping | 100×50 image drawn at 400×200, centre click | `{50 25}`; edges rejected | correct | pass |
| Captcha panel | two pre-picked points | count, numbered markers, click adds a point | all present; click mapped to `{25 25}` | pass |
| Captcha buttons | 撤销/清空/提交/取消 | each action once; empty submit disabled | correct | pass |
| Captcha submit payload | two points through `uiPrompter` | size 80×40 and points in order | correct | pass |
| Captcha cancellation | window cancel and login-context cancel | handler returns error, panel cleared | both | pass |
| Captcha bad image | empty and non-image bytes | error, no panel | both | pass |
| Captcha panel render | mygo test renderer, 520×560 | numbered markers and buttons visible | rendered | pass |
| Race detector | `go test -race . ./ui ./session` | no data race | clean | pass |
| Binary after the change | launch, 6 s, SIGTERM | ready line, exit 0 | exit 0 | pass |
| REUSE compliance | `reuse lint` | 24/24 files | clean | pass |
| Package build | `makepkg -f` from the local checkout | package with binary, desktop, icons, license | `gotrust-git r7` | pass |
| Package hardening | `readelf` on the extracted binary | PIE, FULL RELRO, BIND_NOW | all present | pass |
| namcap | `namcap` PKGBUILD/package | only documented warnings | dlopen deps and unused libresolv only | pass |
| Packaged binary | extracted `usr/bin/goTrust`, 6 s, SIGTERM | ready line, exit 0 | exit 0 | pass |
| Config TOML round-trip | profiles, secrets, disabled listener, escaping | equal values, 0600 | equal, 0600 | pass |
| Config defaults | partial TOML, omitted keep-alive | 443, `local`, enabled, 90 s | as expected | pass |
| Keep-alive explicit false | `enabled = false` | stays off | stays off | pass |
| Config newer version | `version = 99` | refused | refused | pass |
| v1 JSON ignored | leftover `config.json` | defaults, no `config.toml` | as expected | pass |
| Profile operations | add/copy/rename/select/delete | active follows, last protected | as expected | pass |
| Client data per profile | `默认` and two other names | distinct files, no traversal | distinct | pass |
| Log redaction | password/sid/cookie/coordinates | secrets absent | absent | pass |
| Log rotation | 400-byte limit, 2 backups | bounded files, last line kept | as expected | pass |
| Log file mode | state dir | 0600 file, 0700 dir | as expected | pass |
| Settings apply | edits, dedup targets, disabled proxy | normalised profile | as expected | pass |
| Settings validation | name/port/interval/listeners | rejected with status | rejected | pass |
| Settings view | all groups, password masked, callbacks | rendered; save/new/toggles fire | as expected | pass |
| Settings profile hint | new profile selected | switch hint shown | shown | pass |
| Tray menu | online with two profiles | status disabled, checked profile, clicks | as expected | pass |
| Auto-connect decision | option off, no gateway/user/secret, stored password/session | connect only with proof | as expected | pass |
| Keep-alive parse | OK, error, DNS and ordinary lines | status line per case | as expected | pass |
| Disk config | stored vs unstored password | only stored written | as expected | pass |
| Headless smoke | temp XDG, no gateway | 0600 log, exit 1 with reason | as expected | pass |
| Real UI + autostart | temp XDG, `autostart = true` | ready, entry written, exit 0 | as expected | pass |
| Auto-connect visible failure | no saved password | reason logged and window shown | logged | pass |
| Settings window render | 560×900 headless | groups, 保存/关闭 visible | rendered | pass |
| Settings save pipeline | edit draft, save | TOML written, selected profile active | as expected | pass |
| Unstored password | save with store off | disk empty, memory keeps it | as expected | pass |
| Rejected save | port 0 | status set, file and live config untouched | as expected | pass |
| Client data directory | nested path on first login | 0700 dir, 0600 file | as expected | pass |
| Graceful quit after hide-on-close | SIGTERM, hidden window | exit 0, no process | as expected | pass |
| Settings native close | ydotool Alt+F4 on the settings window | window hides, app alive | alive; before the fix: SIGSEGV | pass |
| Settings 关闭 button | ydotool keyboard activation | window hides, app alive | alive; before the fix: SIGSEGV | pass |
| Settings open from 按钮 | RunOnMain-deferred open, real config | no GL errors, window shows | as expected | pass |
| Quit with settings open | SIGTERM | exit 0, no crash | exit 0 | pass |
| Keep-alive default | fresh profile | 90 s | 90 s | pass |

| Live login with captcha | real gateway, operator clicked the image | online with an IP | online, `203.0.113.4` | pass |
| Live HTTP proxy | `curl -x 127.0.0.1:11081 http://192.0.2.10/` | 200 from the intranet | 200, 507 bytes | pass |
| Live SOCKS5 proxy | `curl --socks5-hostname 127.0.0.1:11080 …` | 200 from the intranet | 200, 507 bytes | pass |
| Session restore | restart with stored client data | online without a captcha | restored | pass |
| Live keep-alive | one derived application target, 15 s | repeated OK rounds | 10+ × `KeepAlive: OK status=200` | pass |
| Listener lifetime | 35 s after login | both ports still bound | both bound | pass |
| Login domain | gateway offers `local` | login accepted | accepted | pass |
| Listener context test | cancel the session context | both listeners close | both close | pass |

## Error Log

| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-10-23 | `go get github.com/mythologyli/zju-connect@dev`: unknown revision | 1 | `main` is the default branch; used `replace` with a fork pseudo-version |
| 2026-10-23 | pseudo-version `v1.3.1-0.20261020T02-…` rejected | 2 | timestamp needs 14 digits; stripped the separators first |

## 5-Question Reboot Check

| Question | Answer |
|----------|--------|
| Where am I? | Phase 11 complete: TOML v2, profiles, settings window, logging, tray, autostart, auto-connect |
| Where am I going? | Phase 12, auth methods and second factors |
| What's the goal? | a buildable, runnable goTrust skeleton |
| What have I learned? | see `findings.md` (license chain, embeddable surface, shutdown gap, mygo API traps) |
| What have I done? | four packages written and tested; binary built, launched and shut down cleanly |
