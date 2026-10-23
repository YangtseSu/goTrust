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
| Where am I? | Roadmap updated: phases 11-16 planned, dae deferred last |
| Where am I going? | Phase 11, settings/startup/tray |
| What's the goal? | a buildable, runnable goTrust skeleton |
| What have I learned? | see `findings.md` (license chain, embeddable surface, shutdown gap, mygo API traps) |
| What have I done? | four packages written and tested; binary built, launched and shut down cleanly |
