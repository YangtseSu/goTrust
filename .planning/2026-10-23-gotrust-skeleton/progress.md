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
  - the operator installed it with `pacman -U`; the installed package is
    `gotrust-git r32`
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

### Phase 12: Auth methods and second factors

- **Status:** complete (SMS/TOTP/SSO untested live; the verified account has no
  second factor and no SSO endpoint)
- Actions taken:
  - `session/authmethod.go`: `DiscoverAuthMethods` asks the gateway's
    `authConfig` for its method list on its own underlay dialer (the fork's
    `atrust.GetAuthInfoList` builds one internally and takes no context, so the
    exported `auth.NewSessionContext` + `Session.GetAuthInfoList` path is used
    instead); `ResolveAuthMethod` makes a stored pair authoritative only while
    the gateway still offers it and reports `ErrAuthMethodChoice` otherwise
  - login: discovery runs after validation, a nil method list (already logged
    in) leaves the stored-session restore to the client, a stale stored pair
    fails with the offered list, and `Options.ChooseAuth` asks the window; the
    chosen pair is persisted to the active profile
  - config: `auth_type` and `totp_secret` are wired into the login
    (`SetupOptions.TOTPSecret`); an empty `auth_type` means automatic
  - UI: `CodePrompt`, `ExternalLogin` and `AuthChoice` panels replace the
    connection controls, with `session.GenerateTOTPCode` for an opt-in TOTP
    secret and `session.ValidateCallbackURL` for CAS/OAuth2 pastes
  - settings: 认证方式 picker (自动 + discovered pairs + a "saved but not
    offered" entry), 获取认证方式 button that queries the draft's gateway off
    the UI thread, and per-profile clearing of a stale list
  - prompt lifecycle: every panel gets the same cancel/context release as the
    click captcha; 断开 releases all four through `cancelPrompts`
- Verification:
  - `go build`, `go vet`, `gofmt -l`, `go test ./...`, `go test -race . ./session ./ui`,
    `reuse lint` (36/36 files)
  - discovery against an `httptest` TLS gateway: mapping, dedupe, empty list,
    context cancellation; ResolveAuthMethod table; RFC 6238 TOTP vector
  - headless run against `127.0.0.1:1`: exits 1 after
    `discover the authentication methods: … connection refused`
  - real binary with a temp config: `goTrust ready`, settings window opened
    from the 设置 button, SIGTERM exit 0 (repeated with and without the
    settings window)
  - the four panels rendered through `ui.NewTester` and inspected (code, TOTP
    with the secret field, SSO, chooser)
- Notes:
  - the settings window was not clicked through the new picker on the real
    desktop; Phase 15 does the final UI smoke
  - primary SMS login is refused with a clear message because the settings
    window collects no phone number
  - `github.com/pquerna/otp` moved from the indirect block to the direct block
    by hand (`go mod tidy` cannot run offline here)

### Phase 13: Granted resources

- **Status:** complete (derivation, cache and the window verified by tests and a
  seeded cache; the live gateway was not re-run in this phase)
- Decisions:
  - the portal's app list names applications, so the fork now carries the name
    and launch hints instead of dropping them; alias/fork pin work stays in the
    fork, not in goTrust
  - one row per app, one per domain name, one per IP range (ports merged), one
    per DNS name; ordering is apps, domains, IPs, DNS, each sorted
  - URL suggestion only where a browser link is real: a single host and a
    tcp/all rule covering 443 (https) or 80 (http); ranges and UDP rules get no
    link
  - the cache is `resources.json` beside `config.toml` (0600, versioned) and
    holds addresses, never credentials. `--print-resources` reads it without a
    login; `--no-ui` prints what the login just fetched
- Actions taken:
  - fork `YangtseSu/zju-connect` dev `7958d6a` (pushed): `resourceApp` keeps
    `name`, the group `name`, `accessAddress`, `openModel.model` and
    `webRelativeDomainList`; `client.App` + `Client.Apps()` expose them, while
    the address rules still reference the app by ID. Tests cover the parse and
    the empty pre-parse answer. go.mod moved to
    `v1.3.2-0.20261023210742-7958d6a202ff`
  - `session/resources.go`: `Resources` (apps, IP/domain/DNS rules, derived
    `Entries`), `buildResources`, sorting, port/protocol rendering, URL
    suggestion, `FormatResources`, and the versioned snapshot with an atomic
    0600 write
  - `session.Login` retains the snapshot in `State.Resources` and writes the
    cache; `Logout` clears it. A failed fetch of one rule set is logged and
    leaves that set empty
  - main: `--print-resources`, stdout list after a `--no-ui` login, and the
    open/copy actions (`mygo.Shell.OpenExternal`, `mygo.Clipboard`)
  - main window: scrollable resource list below the connection controls, with
    per-row 打开/复制 and offline/empty hints
  - README: status and a "Granted resources" section
- Verification:
  - `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...` clean;
    the fork's `go test ./...` clean before the push
  - derivation: 7 rows from five IP rules (duplicate collapsed), three domain
    names, one DNS name; ordering, details and copy values asserted
  - URL suggestion: exact/covered 80/443, https first, UDP/no-port/no-host
    rejected; app access address, scheme-less value and web-domain fallback
  - cache: 0600, nested directory created, entries survive the round trip,
    version 99 refused, missing file an error
  - `--print-resources` with a seeded cache printed four rows and exited 0;
    without a cache it exited 1 with the path in the message
  - main window rendered headlessly at 520×560 with eight rows: apps, domains,
    IPs and DNS visible, the list clips and scrolls, buttons aligned; the PNG
    was inspected and deleted
  - real binary with a temporary `XDG_CONFIG_HOME`/`XDG_STATE_HOME`:
    `goTrust ready`, tray and window alive, SIGTERM exit 0
- Notes:
  - the live gateway was not re-run, so the portal's actual app list was not
    observed end to end; the parse is covered by the fork's fixture tests and
    the UI by seeded state
  - `--no-ui` needs a gateway that does not ask for a second factor; the print
    path after a successful login was therefore not exercised live

### Phase 14: Reliability

- **Status:** complete (unit-tested ladder and expiry path; the live gateway
  was not re-run)
- Decisions:
  - the fork's own recovery ladder (renew → upstream ticket restore →
    cooldown) runs first; goTrust's 1/2/4/8/16 s ladder starts only at
    `relogin-required`, so the two never race. The upstream restore attempt
    remains library behaviour, but it cannot cross a connection (the portal
    answers `75500307` once logged out); every goTrust attempt is a fresh
    login. The fork still keeps `AutoRelogin` off: a full login
    rotates the device token and drops other clients of the account
  - **a session ends with its connection.** The operator confirmed that both
    `atrust-quadlet` and `atrust-dev` tried the stored-sid-ticket restore and
    the portal rejected it once the session was gone. The recorded evidence:
    `ticketExchange` needs a live session, the
    ticket half answers `75500307 非法的随机校验码` while logged out (`:322`),
    and quadlet's restart "cannot restore a session the portal has
    already ended" (`plans/12-client-api-state-read.md:84`). Therefore no
    reconnect reuses a session: the ladder is a full login, auto-connect
    requires a saved password, and client data counts as device material only
  - the ladder is not a health probe. A single failed keep-alive is evidence,
    not a session loss; only the client's session state starts a reconnect
  - the state machine gained `PhaseReconnecting`; `State.Reconnect` carries the
    attempt, the cap and an absolute `NextAt`, and the countdown is rendered,
    never stored as a frozen number
- Actions taken:
  - fork `YangtseSu/zju-connect` dev:
    - the first attempt also adopted `DeviceID` from stored client data so
      `canRestoreByTicket` fired. That was wrong: it made a reconnect reuse the
      old session, which the portal refuses. The operator asked for the
      erroneous commit to disappear from history, so `dev` was rewritten (one
      commit replayed onto `7958d6a`, then a force push): the branch now holds
      only `1d0673b`, the feature commit, and no adoption code
    - `feat` `1d0673b`: `service.KeepAliveOptions.OnResult` reports every probe
      (HTTP target, status/error, timestamp) and the remote-DNS fallback;
      `SetupOptions.OnSessionState` reports each real
      `online`/`restoring`/`relogin-required` change; tests for both
  - go.mod re-pinned to `v1.3.2-0.20261023225112-1d0673b1c79b`; the fork tests
    (including the race-free suite) are green on the rewritten branch
  - `session/reconnect.go`: the schedule (`reconnectDelays`), `reconnectLadder`
    with injected wait/attempt/challenge hooks, `waitForReconnect` (one state
    per second so the countdown moves), `onSessionLost`, `hasStoredPassword`,
    `startReconnect`/`stopReconnect`, `expire`, `observingPrompter`
  - `session/session.go`: `PhaseReconnecting`, `State.Reconnect`,
    `State.KeepAlive`, `State.Attention`, `setTransition` (keeps `Since` across
    countdown ticks), `recordKeepAlive`, `teardown`, and a `Login`/
    `loginSession` split so the ladder owns its attempts
  - `ObserveSessionState` mirrors restoring/online/relogin-required and ignores
    a closed client's callbacks; keep-alive results are published without
    faking a phase change
  - `--no-ui` and the proxy are untouched: a dead session makes `Dial` return
    `ErrNotOnline` instead of serving through a tunnel that rejects everything
  - main: dropped the log-line keep-alive parser and the logging `OnLine` hook;
    the tray tooltip and the window read `state.Session.KeepAlive`; a failure or
    an attention state reveals the window; 断开 stops the ladder
  - UI: `重连中 (n/5，x 秒后)` in the phase line and the tray, `KeepAliveSummary`
    (HTTP answer, transport error, DNS fallback), the 保活 hint about
    `https://`, and 断开 available while reconnecting
  - settings label "断线后自动重连（最多 5 次）"
  - README "Reliability" section
- Verification:
  - `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`,
    `go test -race . ./session ./ui`, `reuse lint` (40/40) clean
  - the fork's `go test ./...` clean before the push
  - schedule, cap, challenge stop and cancellation are asserted on
    `reconnectLadder` with no clock; session-loss states (including "no ladder
    without a saved password"), foreign callbacks, keep-alive transitions and
    target ordering on the manager; auto-connect now needs a saved password
  - the reconnect state and the keep-alive summaries rendered through
    `ui.NewTester`; the tray status asserted
  - a headless binary smoke (`--no-ui`, temp XDG, gateway `127.0.0.1:1`)
    exited 1 with the discovery error and wrote the 0600 log
- Notes:
  - the live gateway was not re-run: the verified account is in use by the
    operator's running instance, and mygo's single-instance lock refuses a
    second window. The window states are covered by the headless renderer
  - a transient network drop that never produces an aTrust "session invalid"
    answer only shows up as failed keep-alive probes; it does not start the
    ladder. That is deliberate (a retry is a full login), but a future phase
    could offer probe-based recovery

### Phase 15: UI polish

- **Status:** complete (all states rendered and inspected; the native tray menu
  was not captured, see Notes)
- Decisions:
  - one design system in `ui/theme.go`: a four-step type scale derived from the
    theme's `FontSize` (title 1.5x, panel 1.15x, body 1x, caption 0.85x), a
    24/16/8 spacing grid, and phase colours from the theme's semantic tokens.
    No hard-coded hex values in the views; light and dark both follow the
    desktop
  - the phase is a coloured chip (dot + tinted pill) next to the title; the
    detail line is muted for idle/connecting/online and takes the warning or
    danger colour only when the session is reconnecting or has failed
  - busy is a spinner plus 处理中… and a disabled 连接; 断开 stays enabled
    because it is the only cancel, and the tray toggle is disabled only when it
    would start a second action (idle phase)
  - the settings window pins its actions above a divider with the status line
    above them, so a long form never hides why a save failed
  - the mark is a blue tile with a white shield and a downward arrow: drawn at
    128, rasterized to 16/32 and inspected before choosing it; a symbolic
    variant is a filled shield with the arrow cut out, which stays legible at
    16px
- Actions taken:
  - `ui/theme.go`: type scale, spacing constants, `tone` (neutral/accent/
    success/warning/danger), `phaseTone`, `toneColor`, `windowHeading`,
    `panelHeading`, `sectionHeading`, `caption`, `captionTone`, `statusChip`,
    `busyLine`
  - `ui/view.go`: header with the status chip and a non-duplicated detail line;
    captcha panel with the theme's danger colour for markers; controls with the
    busy state; resource list with a heading and count, hairline dividers and
    per-row descriptions on 打开/复制; footer with tone-coloured keep-alive and
    an error-coloured status; `phaseActive` shared by the window and the tray
  - `ui/settings.go`, `ui/prompt.go`: shared headings, muted captions, action
    rows aligned to the end, Enter submits the code and callback fields, and
    the SSO URL is constrained to two lines with an ellipsis so the open button
    never leaves the window; checkboxes sit on the form's control column
  - `ui.SettingsState.StatusError` and `ui.State.StatusError`; `setStatusError`
    in `main.go` for failed actions, `setStatus` clears the flag and refreshes
    the tray
  - window sizes and minimums: main 540×640 (min 440×400), settings 600×760
    (min 560×440)
  - accessibility: `RoleHeading`+`Level` on window and panel headings, a named
    `RoleStatus` chip, `Description` on repeated resource and log buttons,
    `Label` on the captcha image and spinner; tab order tested in the main
    window and up to 保存/关闭 in the settings form
  - icons: `data/icons/hicolor/scalable/apps/goTrust.svg` and the symbolic
    variant; `data/icons/tray-64.png` generated from the SVG and embedded with
    `go:embed`; `data/goTrust.desktop`; `go:generate` runs `rsvg-convert`
  - package: `PKGBUILD` no longer carries a desktop entry and PNGs in
    `$srcdir`; it installs the desktop entry and both SVGs from the checkout
  - render harness: `ui/render_test.go` writes every state, dark/light and both
    minimum sizes to `GOTRUST_RENDER_DIR` and is skipped otherwise
- Verification:
  - `go build ./...`, `go vet ./...`, `gofmt -l .`, `go test ./...`,
    `reuse lint` (47/47) clean
  - all images inspected: header chip and hierarchy, resource rows, captcha
    markers, code/SSO/chooser panels, settings groups, light and dark, 440×400
    and 560×440 minimum sizes
  - real binary with temporary `XDG_*` directories: window with CJK, the
    settings window opened by keyboard (Tab Tab Enter), the new tray icon in
    the GNOME panel at 16px, SIGTERM exit 0 and no process left
  - `makepkg -fC` rebuilt `gotrust-git r18`; the package now holds
    `/usr/bin/goTrust`, the desktop entry, both scalable icons and the licence;
    `desktop-file-validate` passes (one category hint), the extracted binary
    starts and exits 0, and `go generate ./...` regenerates the tray PNG with no
    diff
  - icon checked at 16, 32 and 128 before the code was written; the panel icon
    was identified by zooming a screenshot (it is a blue tile, white shield,
    blue arrow)
- Notes:
  - the native tray menu was not opened on the desktop: ydotool's absolute
    pointer did not land on panel icons in this session (clicking the power
    icon focused the window below), and blind clicking on the operator's
    desktop was stopped. `TestTrayMenu*` covers the items, the toggle and the
    profile submenu; the icon was seen and stayed
  - the GNOME appindicator extension logged
    `gtk_widget_get_scale_factor: assertion 'GTK_IS_WIDGET (widget)' failed`
    when it was disabled and re-enabled while the app ran. Harmless for the
    session; a mygo/libayatana lifecycle issue, not a goTrust change
  - a 64px tray PNG is used for HiDPI panels; the package installs only the
    scalable SVGs, which the icon theme rasterizes at the requested size

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
| Resolve auth method | one method / stored pair / stale pair / several | automatic / exact / list error / choice | as expected | pass |
| Discover auth methods | fake TLS gateway, duplicate and typeless entries | three mapped, deduped | as expected | pass |
| Discover empty list | `isLogin: 1` answer | empty, no error | empty | pass |
| Discover cancellation | cancel mid-request | error, no list | cancelled | pass |
| Auth host | 443, 8443, IPv6 | port omitted / joined | as expected | pass |
| Primary login guard | `auth/smsCheckCode` | refused without a phone number | refused | pass |
| Callback validation | CAS/OAuth2 good and bad pastes | ticket/code/path/https enforced | as expected | pass |
| TOTP generation | RFC 6238 vector, bad secret | 287082; error | as expected | pass |
| Code prompt submit | SMS with skip | code and skip reach the client | as expected | pass |
| TOTP secret opt-in | secret entered, store ticked | 6-digit code, 0600 file updated | as expected | pass |
| TOTP bad secret | `not base32!` | panel stays, status explains | as expected | pass |
| Code prompt cancel | window and login context | handler returns, panel clears | both | pass |
| SSO prompt | bad paste then valid CAS callback | bad kept open, valid returns | as expected | pass |
| Auth chooser | two methods, pick the second | returned and persisted to TOML | as expected | pass |
| Auth chooser cancel | login context ends | returns `context.Canceled` | as expected | pass |
| Settings auth picker | select pair, trailing entry, auto | pair stored, trailing no-op, auto clears | as expected | pass |
| Settings discovery target | draft address/port, bad port | trimmed / 8443 / rejected | as expected | pass |
| Settings auth view | two methods | picker, button, hint, callback | rendered | pass |
| New panels render | code, TOTP, SSO, chooser 520×560 | all texts and buttons | rendered | pass |
| Discovery failure smoke | `--no-ui`, gateway `127.0.0.1:1` | exit 1 with the reason | as expected | pass |
| Reconnect schedule | `reconnectDelay` 1…5 and out-of-range | 1/2/4/8/16 s, clamped | as expected | pass |
| Reconnect ladder cap | always-failing attempt, password stored | 5 attempts, exhausted | 5 | pass |
| Reconnect ladder challenge | challenge after attempt 2 | stops at 2 | 2 | pass |
| No ladder without a password | auto-reconnect on, no saved password | failed + attention, no ladder | as expected | pass |
| Auto-connect needs a password | stored client data, no password | not connected, reason names the password | as expected | pass |
| Reconnect ladder cancelled | cancelled context | no attempt runs | 0 | pass |
| Session loss, no auto-reconnect | `relogin-required` | failed + attention, `Dial` → `ErrNotOnline` | as expected | pass |
| Session loss, no material | auto-reconnect on, empty profile | failed, no ladder | as expected | pass |
| Restore transitions | `restoring` then `online` | reconnecting → online, IP kept | as expected | pass |
| Foreign client callback | stale client reports loss | live state untouched | ignored | pass |
| Keep-alive transitions | OK then transport error | fields replaced, phase kept | as expected | pass |
| Target ordering | named + derived port-80 grant | named first, derived second | as expected | pass |
| Tray reconnect status | attempt 2/5, 4 s left | `重连中 (2/5，4 秒后)` | as expected | pass |
| Keep-alive summary | OK / error / 503 / DNS / none | formatted per case | as expected | pass |
| Reconnect render | 520×560, ladder 2/5 | label and countdown present | rendered | pass |
| Headless reliability smoke | `--no-ui`, temp XDG, gateway `127.0.0.1:1` | exit 1 with the reason, 0600 log | as expected | pass |
| Fork session observer | online repeated, then relogin-required | one callback per change | as expected | pass |
| Fork keep-alive results | 500 then 200 | two results, timestamps increasing | as expected | pass |
| Fork fallback result | zero resolver | one labelled failure | as expected | pass |
| Fork after the rewrite | module cache at `1d0673b` | hooks present, no adoption symbol | as expected | pass |
| Race detector (reliability) | `go test -race . ./session ./ui` | clean | clean | pass |
| REUSE after the new files | `reuse lint` | 40/40 | 40/40 | pass |
| Phase tone mapping | idle/connecting/online/reconnecting/failed | neutral/accent/success/warning/danger | as expected | pass |
| Busy disables connect | Busy with idle phase, click 连接 | no login, 处理中… visible | as expected | pass |
| Busy keeps disconnect | Busy while connecting, click 断开 | logout fires | as expected | pass |
| Main tab order | Tab from the top | 连接 → 设置 → 退出 | as expected | pass |
| Settings tab order | Tab up to the actions | 保存 then 关闭 | as expected | pass |
| Status tone | failure flag set and cleared | text present both ways | as expected | pass |
| State renders | every phase, prompts, settings, light/dark, min sizes | PNGs written and inspected | as expected | pass |
| Icon at panel size | 16/32 px, GNOME panel | shield and arrow legible | as expected | pass |
| Native window smoke | real binary, temp XDG | CJK, settings by keyboard, exit 0 | as expected | pass |
| Package after Phase 15 | `makepkg -fC`, `pacman -Qlp` | binary, desktop, two SVGs, licence | r18, all present | pass |
| Desktop entry | `desktop-file-validate` | valid, hint only | valid | pass |
| Packaged binary | extracted `usr/bin/goTrust` | ready line, exit 0 | as expected | pass |
| Icon regeneration | `go generate ./...` | same PNG, no diff | no diff | pass |

| Live login with captcha | real gateway, operator clicked the image | online with an IP | online, `203.0.113.4` | pass |
| Live HTTP proxy | `curl -x 127.0.0.1:11081 http://192.0.2.10/` | 200 from the intranet | 200, 507 bytes | pass |
| Live SOCKS5 proxy | `curl --socks5-hostname 127.0.0.1:11080 …` | 200 from the intranet | 200, 507 bytes | pass |
| Session restore | restart with stored client data | online without a captcha | restored | pass |
| Live keep-alive | one derived application target, 15 s | repeated OK rounds | 10+ × `KeepAlive: OK status=200` | pass |
| Listener lifetime | 35 s after login | both ports still bound | both bound | pass |
| Login domain | gateway offers `local` | login accepted | accepted | pass |
| Listener context test | cancel the session context | both listeners close | both close | pass |

### Phase 17: review remediation (plan recorded)

- **Status:** pending
- **Started:** 2026-10-24 (review), fixes not started
- Actions taken:
  - reviewed every Go file, test, `README.md` and packaging asset in the tree reviewed
    then, with four independent read-only slices plus cross-cutting checks
  - confirmed the requested gates stay green: build, vet, test, `-race`,
    `gofmt`, `reuse lint`, no-cgo and windows/amd64 builds
  - reproduced four defects with temporary targeted tests, then removed them:
    settings delete panic, Ports/Targets drift after a middle delete, CONNECT
    pipelining hang against the real proxy, and the Cookie/Authorization
    redaction leak; reproduced the `currentConfig` data race under `-race`
  - verified the DNS/stack findings against the pinned fork source
    (`CanUseTCPTunnel` always true, `DialUDP` not implemented, `"auto"` passed
    to concrete-address resolver dialers)
  - recorded three P0, 13 P1, the P2 batch and the test gaps in `findings.md`;
    added the fix order to `task_plan.md` as Phase 17
- Results:
  | Check | Result |
  |---|---|
  | Build/vet/test/race/gofmt | clean |
  | `reuse lint` | 47/47 compliant |
  | `CGO_ENABLED=0`, windows/amd64 | pass |
  | Working tree after the review | clean, no code changed |
  | Settings delete through `SettingsView` | panic: index out of range [1] with length 1 |
  | Delete middle profile, save next | port 8443 instead of 9443 |
  | CONNECT + payload in one write | 200 then no bytes for 3 s |
  | `Cookie: theme=dark; session=SECRET123` | `SECRET123` survives redaction |
  | `currentConfig` vs `storeAuthChoice` under `-race` | data race reported |
- Files created/modified:
  - `.planning/2026-10-23-gotrust-skeleton/task_plan.md` (Phase 17, Next Step)
  - `.planning/2026-10-23-gotrust-skeleton/findings.md` (review evidence)
  - `.planning/2026-10-23-gotrust-skeleton/progress.md` (this entry)
  - no source files
- Next: P0-3 + P1-11 in one change with regression tests, then P0-2, then P0-1.

### Phase 17: settings delete panic and buffer alignment (P0-3, P1-11)

- **Status:** done (one commit); P0-2 and P0-1 remain
- Actions taken:
  - wrote three failing tests first, driving 新建/复制/删除 through
    `SettingsView` with the real state and the real callbacks: the mid-frame
    delete panicked with `index out of range [1] with length 1`, a middle
    delete left C with B's port `8443`, and a copy showed B's stored targets
    instead of the edited ones
  - `SettingsState.selection()` re-resolves the index and profile after
    refresh; `SettingsView` calls it again after `profileSection`, so the rest
    of the frame never indexes a profile the buttons just removed
  - `DeleteProfile` splices both positional buffers at the deleted index
    before refreshing, instead of truncating the tail onto its neighbour
  - `DuplicateProfile` writes the editor's port and target text onto the
    appended copy, so the copy matches what the user sees
- Results:
  | Check | Result |
  |---|---|
  | `go test ./ui` before the fix | panic, 8443 on C, wrong copy targets |
  | `go test ./ui` after the fix | pass |
  | `go build ./...`, `go vet ./...`, `gofmt -l .` | clean |
  | `go test ./...`, `go test -race ./ui` | pass |
  | Settings view frame after delete | renders the status line, no panic |
- Files created/modified: `ui/settings.go`, `ui/settings_test.go`, planning
  files
- Next: P0-2 (release proxy listeners on session failure), then P0-1.

### Phase 17: proxy lifetime on failure and reconnect (P0-2)

- **Status:** done (one commit); P0-1 remains
- Actions taken:
  - wrote the failing tests first against the real sockets: a restart on the
    server's own port failed with `bind: address already in use`, and a
    terminal `PhaseFailed` (`Attention` set, as `expire` and the exhausted
    ladder publish) left the port bound
  - extracted `restartProxy` (used by `connect`) so the replacement is one
    path: it calls `releaseProxy` before `startProxy`, closing the old
    listeners instead of binding under them
  - `releaseProxy` takes the server and cancel under the lock, clears
    `view.Addresses`, then cancels and closes; `disconnect` uses it instead of
    its own copy
  - `onSessionState` releases on a terminal failure only. Interim ladder
    failures carry no `Attention` and keep the listeners, because a retry that
    succeeds reuses them and the app never restarts the proxy from inside the
    ladder
  - `proxyEpoch` makes a server that finishes binding after a release close
    itself rather than install its ports for a session that is gone
  - `startProxy` now installs the server and its cancel atomically with the
    epoch check
- Results:
  | Check | Result |
  |---|---|
  | Restart on the same port, before the fix | `bind: address already in use` |
  | Terminal failure, before the fix | port still bound |
  | Restart, terminal failure, interim failure after the fix | pass |
  | `go test ./...`, `go test -race .` | pass |
  | `go vet ./...`, `gofmt -l .` | clean |
- Files created/modified: `main.go`, `app_test.go`, planning files
- Next: P0-1 (never join `Manager.Logout` on the UI thread while a login may
  need the main loop).

### Phase 17: disconnect never joins the login on the UI thread (P0-1)

- **Status:** done (one commit); the P1 batches remain
- Actions taken:
  - wrote the failing test first: `disconnect` called the session join
    synchronously, so it did not return while the join was held open; the
    frame would be waiting on `Manager.Logout` while the cancelled login
    publishes through `onSessionState → onMain`
  - `disconnect` now releases the prompts, the login attempt and the proxy
    listeners synchronously, then runs `Manager.Logout` on another goroutine;
    the UI thread returns before the join
  - `beginLogout` shares one pending join between concurrent stops, so two
    `Logout` calls cannot race over the session's login lock; its channel
    closes when the join finished
  - `connect` waits for the pending join before `Manager.Login`; otherwise the
    old `Logout` would tear down the session the new login just built
    (`selectProfile`/`reconcileProfileSwitch` disconnect and connect in
    sequence)
  - `disconnectNow` joins synchronously in `--no-ui` and after `App.Run`,
    where `onMain` drops its callbacks and nothing can deadlock
  - `logout` is a test seam next to `browser`, so the regression test can hold
    the join open
- Results:
  | Check | Result |
  |---|---|
  | `disconnect` with the join held open, before the fix | returned only after the join; test failed |
  | Same test after the fix | returns immediately |
  | Pending join shared, login waits, cancel path | pass |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .` | clean |
- Files created/modified: `main.go`, `app_test.go`, planning files
- Next: P1-4/P1-5 (tunnel DNS `auto`, gVisor when the gateway assigns an IP).

### Phase 17: tunnel DNS and the packet stack (P1-4, P1-5)

- **Status:** done (one commit); P1-6/P1-7 remain
- Actions taken:
  - reproduced the stack defect at HEAD in a throwaway worktree: a real
    `atrust.Client` reports `CanUseTCPTunnel() = true`, so `newStack` always
    returned `*tcptunnel.Stack` and its `IP()`/gVisor branch was dead. The DNS
    defect was traced to the literal `"auto"`: `net.ParseIP("auto")` is nil, so
    the old resolver dialled `:53`, the unspecified address
  - `newStack` is a `Manager` method over `clientpkg.Client` now. It opens the
    gVisor netstack whenever the gateway assigned an address; that stack
    carries UDP, which remote DNS needs. A TCP-only gateway takes the TCP
    tunnel, and a packet stack that fails to come up is logged and leaves the
    TCP path instead of failing the login
  - `tunnelDNS` reads `client.DNSServer()`/`DNSServers()`: an unusable primary
    turns remote DNS off instead of dialling 0.0.0.0, and a missing second
    server stays empty so the resolver keeps the system configuration instead
    of a hard-coded public resolver. The resolver and `service.NewDnsServer`
    now receive those addresses
  - README: names the portal did not map go to the gateway's server over the
    packet stack; a TCP-only gateway uses the system resolver for them
- Results:
  | Check | Result |
  |---|---|
  | HEAD `newStack` with a real client (worktree) | `CanUseTCPTunnel=true`, returned `*tcptunnel.Stack`; repro test failed |
  | After the fix, with and without an assigned address | `*gvisor.Stack` / `*tcptunnel.Stack` |
  | `net.ParseIP("auto")` / `UDPAddr.String()` | `<nil>` / `:53` (unspecified), the old resolver target |
  | `go test ./...`, `go test -race ./session ./proxy ./ui` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: no live gateway login this session; the choice, the DNS inputs and the
  TCP-only fallback are covered by unit tests, plus the fork's own stack tests.
- Files created/modified: `session/session.go`, `session/session_test.go`,
  `README.md`, planning files
- Next: P1-6/P1-7 (hand the proxy a live resolver, allow `Dial` while the
  client is restoring a session).

### Phase 17: reconnect resolver handoff and dialling while restoring (P1-6, P1-7)

- **Status:** done (one commit); P1-8/P1-9 next
- Actions taken:
  - reproduced P1-7 on HEAD in a throwaway worktree: with `PhaseReconnecting`
    and a live dialer, `Dial` returned `the session is not online`, so the
    repro test failed
  - `Dial` now refuses only without a dialer or outside online/reconnecting. A
    client-owned renewal or restore keeps the data path, and a restored session
    is confirmed by traffic, so refusing there stopped the only confirmation.
    The ladder tears the client down before its first attempt, so its
    `PhaseReconnecting` has no dialer and still refuses
  - `Manager.Resolve` resolves through the resolver read per call and replaces
    `Resolver()`; `startProxy` hands `a.manager` to the SOCKS5 listener, so an
    auto-reconnect no longer leaves it on the dead stack's resolver
  - `Manager.dialer` is the `tunnelDialer` interface (the same `Dial` shape the
    keep-alive already takes), which makes the refusal and the restore case
    testable
- Results:
  | Check | Result |
  |---|---|
  | HEAD: `Dial` in `PhaseReconnecting` with a dialer | `the session is not online`; repro failed |
  | After: restoring with a dialer / ladder without one | serves / `ErrNotOnline` |
  | Resolver swapped after a reconnect | the second session's answer |
  | `go test ./...`, `go test -race ./session ./proxy` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: no live gateway. `ObserveTunnelTraffic` is called only by the TCP
  tunnel, so with the gVisor stack the phase can stay `正在恢复会话` after the
  tunnel works; scheduled as P1-17 (last P1 before the P2 batch, needs a fork
  hook and a pin bump).
- Files created/modified: `session/session.go`, `session/session_test.go`,
  `main.go`, planning files
- Next: P1-8/P1-9 (`--config` must move the client data, a profile delete or
  rename must take its file with it).

### Phase 17: queue itemised, P1-17 scheduled

- **Status:** schedule updated (planning files only)
- The follow-up found while fixing P1-5 (the packet stack never reports
  `ObserveTunnelTraffic`, so a restored session keeps saying
  `正在恢复会话`) was recorded but not queued. It is now **P1-17**, the last P1
  item before the P2 batch, with the fork hook as the primary fix, a pin bump,
  and an acceptance test that the first tunnelled connection publishes `online`.
- The P2 batch is itemised with every finding from the review and an acceptance
  line each: proxy connection tracking and hop-by-hop headers, half-close and
  flush, bind validation, scheme allowlist, TOTP removal, atomic client data and
  cache sync, TOML unknown keys, slug collisions, ladder/dial races,
  stale-snapshot publishes, keep-alive bounds, dead code and
  `--print-resources` metadata.
- Order recorded in `task_plan.md`: P1-8/P1-9 → P1-10 → P1-12/P1-16 →
  P1-13/P1-14 → P1-15 → P1-17 → P2 → verification. Only P1-17 depends on the
  fork; the rest are goTrust-local.

### Phase 17: account material follows the settings and the profile (P1-8, P1-9)

- **Status:** done (one commit); P1-10 next
- Actions taken:
  - reproduced both defects at HEAD in a throwaway worktree: `--config` on a
    temporary path still resolved client data to
    `$XDG_CONFIG_HOME/goTrust/client_data.json`, and a saved profile rename
    left the old `client_data-*.json` behind
  - `Config` now carries the settings file it was loaded from (unexported, not
    encoded). `ClientDataPathFor` uses that file's directory and falls back to
    `Dir()` only for a configuration that was never loaded; every copy
    (`Clone`, the settings draft, `cfgForDisk`) carries it
  - `SettingsState` records a deleted profile and the pending rename;
    `ClientDataChanges` reports them in apply order (removals first, then the
    rename). The rename is detected in `Apply` from the draft's old name and is
    cleared by `Select`/`Reload`, so a discarded draft changes nothing
  - `saveSettings` applies the changes only after `SaveFile` succeeded: remove
    the deleted profile's file, rename the file to the new slug (replacing a
    stale file with that name). A custom `client_data_path` belongs to the
    user, so it is never moved or deleted
  - README and the `--config` help state the rule
- Results:
  | Check | Result |
  |---|---|
  | HEAD: `--config` on a custom path | client data resolved to the default directory; repro failed |
  | HEAD: saved rename | old file remains; repro failed |
  | After: rename / delete / unsaved draft | moved / removed / untouched |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Files created/modified: `config/config.go`, `config/config_test.go`,
  `ui/settings.go`, `ui/settings_test.go`, `main.go`, `app_test.go`, `README.md`,
  planning files
- Next: P1-10 (clone the config before a prompt handler mutates it).

### Phase 17: prompt handlers edit a private config copy (P1-10)

- **Status:** done (one commit); P1-12/P1-16 next
- Actions taken:
  - reproduced both symptoms at HEAD in a throwaway worktree:
    `storeAuthChoice` against an unwritable settings path left `auth_type` set
    in memory, and `go test -race` reported a data race between the handler's
    write through the shared `Profiles` array and `Active()` on the reader
    goroutine
  - `currentConfig` copies the struct but the slice header shares the backing
    array; `mutableConfig` returns a `Clone()` and is now the only source for
    the two handlers that edit a profile
  - because the edit lives in the clone, `applyConfig` runs only after a
    successful `SaveFile`, so a failed save leaves the running settings intact
  - audited the other `currentConfig` callers: `persistFlag` and
    `selectProfile` write only top-level scalar fields on their value copy, and
    `cfgForDisk` already clones; no other shared-array writes exist outside
    `config`/`ui`
- Results:
  | Check | Result |
  |---|---|
  | HEAD: failed save after `storeAuthChoice` | `auth_type = auth/psw` in memory; repro failed |
  | HEAD: handler vs concurrent `Active()` | `WARNING: DATA RACE` |
  | After: both tests | pass, race-clean |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Files created/modified: `main.go`, `prompt.go`, `prompt_test.go`, planning
  files
- Next: P1-12/P1-16 (stale auth methods across a gateway edit, the hide epoch).

### Phase 17: stale auth methods and the settings hide epoch (P1-12, P1-16)

- **Status:** done (one commit); P1-13/P1-14 next
- Actions taken:
  - reproduced P1-12 at HEAD in a throwaway worktree: after editing the port,
    a rendered frame kept the list discovered for the old gateway, so the
    picker could pin a pair the new gateway never offered
  - `SettingsView` drops a discovered list at the top of every frame when the
    draft no longer names the gateway it came from; `SelectAuth` drops it again
    before reading the index, because the same frame may have edited the
    address
  - `targetKey` canonicalises the port numerically to match `main.keyForTarget`,
    so ` 0443 ` still names the same gateway instead of dropping a valid list
  - two existing picker tests seeded a list under a key that did not match
    their state; they now set the address and use `state.targetKey()`, which is
    the contract: a list belongs to the gateway the draft names
  - P1-16: the epoch check moved into the main-thread callback that hides the
    window (`claimSettingsHide`), with `clearSettingsDraftAfterHide` for the
    draft; `openSettings` and `hideSettings` share `bumpSettingsEpoch`. A
    reopen also runs on the main thread, so it either invalidates the queued
    hide or happens after it
- Results:
  | Check | Result |
  |---|---|
  | HEAD: port edited, frame rendered | list kept with `old.example.com\|443`; repro failed |
  | After: edited / unchanged / padded port | dropped / kept / kept |
  | Epoch claim: current vs reopened | claimed and cleared / stale, new draft kept |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: P1-16 cannot be reproduced headlessly (it needs the native window and
  its event loop). The regression pins the claim and clear invariants; the
  ordering argument is in the function comment and rests on `openSettings`
  running only through `mygo.RunOnMain`.
- Files created/modified: `ui/settings.go`, `ui/settings_test.go`, `main.go`,
  `app_test.go`, planning files
- Next: P1-13/P1-14 (whole-value redaction, pipelined CONNECT bytes).

### Phase 17: whole-header redaction and pipelined CONNECT (P1-13, P1-14)

- **Status:** done (one commit); P1-15 next
- Actions taken:
  - reproduced both defects at HEAD in a throwaway worktree: `Redact` turned
    `Cookie: theme=dark; session=SECRET123` into `Cookie: "***"; session=SECRET123`
    and left `tok123` in `map[Authorization:[Bearer tok123] …]`; a CONNECT with
    its payload in one write received 200 and then nothing until the deadline
  - `credentialHeader` replaces the header-level patterns and masks the whole
    value: a bracketed or quoted value is masked to its close, anything else to
    the end of the line. It covers `Cookie`, `Set-Cookie`,
    `Authorization`/`Proxy-Authorization`, optional quotes around the key, and a
    leading `bearer`/`basic` scheme
  - `secretField` no longer lists the cookie keys (one pass, no double mask) and
    keeps the field-level JSON/TOML/query rules; the existing cookie test now
    expects the whole value masked
  - `tunnel` drains the hijacked `bufio.Reader` with `ReadFull` and writes those
    bytes to the target before the raw copies, instead of `io.Copy(target,
    buffered)`, whose `WriteTo` read the client until EOF and starved the
    return direction
- Results:
  | Check | Result |
  |---|---|
  | HEAD: cookie family / map authorization | `SECRET123` and `tok123` leaked; repro failed |
  | HEAD: CONNECT+payload in one write | 200, then no echoed bytes until the deadline |
  | After: redaction cases | whole header masked; ordinary lines unchanged |
  | After: pipelined CONNECT | the payload is echoed |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: masking runs to the end of the line for an unwrapped value, so a line
  that puts another field after a cookie loses that field too. That is the
  deliberate trade: a missed credential is worse than a lost diagnostic.
- Files created/modified: `logging/logging.go`, `logging/logging_test.go`,
  `proxy/server.go`, `proxy/server_test.go`, planning files
- Next: P1-15 (reopen the log after a failed rotation).

### Phase 17: rotation recovery and the truncation panic (P1-15)

- **Status:** done (one commit); the queue now starts with P1-17 (fork)
- Actions taken:
  - reproduced P1-15 at HEAD in a throwaway worktree: after a rotation whose
    rename and reopen failed, the next lines went nowhere and the file never
    came back
  - `emitLocked` retries `openLocked` whenever the file is nil, before the
    size check; the rotation path puts a failed reopen there instead of
    silently keeping `size = 0`, which its own guard then never retried
  - writing the test with a small `MaxBytes` exposed a latent panic:
    `redacted[:256]` on a shorter line whenever `MaxBytes` is under 512. The
    256-byte floor is now a minimum length for the cut, not a fixed slice
    index, and a line shorter than the floor is left whole. Recorded here
    rather than scheduled because it is fixed in this commit
- Results:
  | Check | Result |
  |---|---|
  | HEAD: rotation fails, directory restored, next line | file never recreated; repro failed |
  | HEAD: `MaxBytes` 64 with a 75-byte line | `slice bounds out of range [:256]` |
  | After: recovery / truncation cases | line written; long cut, short intact |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Files created/modified: `logging/logging.go`, `logging/logging_test.go`,
  planning files
- Next: P1-17 (fork: gVisor `DialTCP` reports tunnel traffic; pin bump).

### Phase 17: packet-stack restore confirmation (P1-17, fork)

- **Status:** done (fork commit pushed, pin bumped); next is the P2 batch
- Actions taken:
  - added `trafficObserver` and `reportTunnelTraffic` to the fork's
    `stack/gvisor`, and `DialTCP` reports a successful `gonet` connection; the
    TCP-tunnel shortcut keeps reporting its own handshake, and `easyconnect`
    does not implement the observer
  - fork tests: the adapter reaches an observer, ignores a client without one,
    and the compile-time assertion `var _ trafficObserver = (*atrust.Client)(nil)`
    keeps the aTrust client wired to it; the fork's full suite passes
  - committed `5e185ef` on `dev`, ran the privacy review (diff and message hold
    code and comments only), pushed to `origin` after approval
  - `go.mod` require + replace moved to
    `v1.3.2-0.20261024055516-5e185ef9fdce`; `go mod tidy` refreshed `go.sum`
- Results:
  | Check | Result |
  |---|---|
  | Fork `go build ./...`, `go test ./...` | pass |
  | Fork push | `1d0673b..5e185ef dev -> dev` |
  | goTrust build, `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .` | clean |
  | `reuse lint` on the committed tree (clean worktree of HEAD) | 47/47 compliant |
  | `reuse lint` in the working tree | fails on the new untracked `.agents/skills/mygo-maintenance/SKILL.md` (not ours; left untouched) |
- Note: the live acceptance (a real restore leaving `正在恢复会话` on its own)
  needs the gateway and is recorded as a manual smoke item; the packet-stack
  dial itself is not reachable in a unit test without a gVisor peer.
- Files created/modified: fork `stack/gvisor/dial.go`, `dial_test.go`;
  `go.mod`, `go.sum`, planning files
- Next: the itemised P2 batch.

### Phase 17: proxy connection lifetime and the HTTP proxy contract (P2)

- **Status:** done (one commit); the rest of the P2 batch remains
- Actions taken:
  - wrote the six regression tests first and ran them in a throwaway worktree at
    HEAD: the two `Close` tests waited out the read deadline instead of seeing
    a dropped connection, the origin still saw `Proxy-Authorization` and the
    `Connection`-listed header, the streamed response never left the proxy, the
    half-closed CONNECT answered nothing (`EOF`), and two forward requests
    dialled the upstream twice
  - `Server` tracks every proxied connection: `trackedListener` registers each
    accepted SOCKS5 connection and `tunnel` registers the hijacked CONNECT
    connection; `trackedConn` untracks on its own close. `Close` stops the
    listeners, then closes the tracked connections, then shuts the HTTP server
    down, because `http.Server.Shutdown` ignores hijacked connections
  - a connection that surfaces after `Close` is refused by `track` and closed
    at once, so nothing installs itself into a dead server
  - `forward` strips hop-by-hop request headers, including every name listed in
    `Connection`, so neither the client's proxy credentials nor a hop token
    reach the origin (`stripHopByHop`)
  - both tunnel directions end with `CloseWrite` and the handler waits for both,
    so a client that half-closes after its request still receives the answer
  - `stream` writes and flushes each body chunk, so an event stream or a long
    poll arrives while the origin keeps it open
  - one `http.Transport` per server for plain-HTTP forwarding, with
    `CloseIdleConnections` on `Close`; a per-request Transport dialled the
    upstream again for every request
- Results:
  | Check | Result |
  |---|---|
  | Six new tests at HEAD (throwaway worktree) | all six failed as reviewed |
  | Same tests after the fix | pass |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean (47/47 compliant) |
- Note: the handler now waits for both tunnel directions; a peer that never
  closes keeps its handler alive until `Close`, which now really drops it.
  Half-close is best-effort: a connection without `CloseWrite` is left alone.
- Files created/modified: `proxy/server.go`, `proxy/server_test.go`, planning
  files
- Next: P2 input validation (non-loopback bind, settings bind syntax,
  `openResource` scheme allowlist).

### Phase 17: proxy binds and browser links are validated (P2)

- **Status:** done (one commit); the rest of the P2 batch remains
- Actions taken:
  - wrote the checks first and ran them in a throwaway worktree at HEAD: the
    pre-fix `Config.Validate` accepted `0.0.0.0:1081` and `127.0.0.1`, the
    settings save accepted `0.0.0.0:1081` and a bind with no port, and
    `openResource("file:///etc/passwd")` reached the browser
  - `config.ValidateBind` is the one rule: host:port, port 1-65535, and a host
    that is `localhost`, an IPv4/IPv6 loopback address; empty host (every
    interface), a name and a non-loopback address are refused with a reason.
    A wider bind would hand the tunnel to the machine's network
  - `Config.Validate` applies it to each enabled listener after the
    both-disabled check, so a hand-edited file fails before a login
  - `SettingsState.Apply` trims the binds and runs the same rule, so the
    failure appears while saving instead of after a successful connection
  - `app.openResource` parses the URL first and only forwards `http`/`https`
    with a host to the desktop handler; a portal-supplied `file:`, `smb:` or
    custom scheme is reported in the status line and logged, not opened
  - README: the loopback rule under Configuration, the link rule under Granted
    resources
- Results:
  | Check | Result |
  |---|---|
  | Pre-fix: `Config.Validate`, settings save, `openResource(file:)` | all three accepted; tests failed |
  | After: loopback forms (127.0.0.1, 127.0.0.2, [::1], localhost) | accepted |
  | After: 0.0.0.0, :1080, [::], 192.0.2.10, a name, a bad port | refused with a reason |
  | Real binary, `--no-ui`, `socks5_bind = "0.0.0.0:1080"` | `SOCKS5 proxy bind: host "0.0.0.0" is not a loopback address`, exit 1, no dial |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean (47/47 compliant) |
- Files created/modified: `config/config.go`, `config/config_test.go`,
  `ui/settings.go`, `ui/settings_test.go`, `main.go`, `app_test.go`,
  `README.md`, planning files
- Next: P2 TOTP (a stored secret is not cleared with 保存密码 and has no removal
  action).

### Phase 17: stored TOTP secret follows the password toggle (P2)

- **Status:** done (one commit); the rest of the P2 batch remains
- Actions taken:
  - wrote the checks first and ran them in a throwaway worktree: the pre-fix
    `cfgForDisk` left an unstored profile's TOTP secret in the file,
    `storeTOTPSecret` wrote a secret for a profile that stores no password, and
    the settings window had no way to remove a stored secret
  - `cfgForDisk` clears `TOTPSecret` together with `Password` when
    `store_password` is false; the in-memory configuration keeps both for the
    run, as it already did for the password
  - `storeTOTPSecret` refuses without the password toggle and says why in the
    status line and the log, because the same save would drop the secret again
  - the connection section shows 二次认证密钥 / 已保存 with a 移除 action once a
    secret exists; the removal edits the draft and takes effect on 保存, like
    every other field
  - the store checkbox now reads 保存密码和 TOTP 密钥（文件权限 0600）
  - README: the settings and TOTP paragraphs, and the TOML comment
- Results:
  | Check | Result |
  |---|---|
  | Pre-fix: disk copy, prompt store, settings removal | secret kept / written / no action; tests failed |
  | After: unstored profile, store without toggle, stored profile | cleared / refused with a note / written |
  | After: 移除 then 保存 | draft and applied config have no secret; the row is gone |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: `TestAFailedPromptSaveLeavesTheSettingsAlone` and
  `TestTOTPChallengeStoresTheSecret` now tick `store_password` first, which is
  the real precondition for writing the secret; their original intent (a failed
  save leaves memory untouched) is unchanged.
- Files created/modified: `main.go`, `prompt.go`, `prompt_test.go`, `app_test.go`,
  `ui/settings.go`, `ui/settings_test.go`, `README.md`, planning files
- Next: P2 persistence (atomic client data, `Sync` on config and cache
  writes, unknown TOML keys, slug collisions).

### Phase 17: durable settings, strict parsing and unique client-data names (P2)

- **Status:** done (one commit); the rest of the P2 batch remains
- Actions taken:
  - wrote the observable checks first and ran them in a throwaway worktree:
    `LoadFile` silently accepted `proxy.typo_bind`, and `Work` and `work`
    resolved to the same path once case is folded
  - `writeClientData` now writes a 0600 temporary file beside the target,
    `Sync`s it and renames it over the ticket; the old `os.WriteFile` truncated
    in place, so a crash could leave half a ticket that the next login would
    read as account material. The final `Chmod` is unnecessary now: the rename
    brings the temporary file's mode with it
  - `SaveFile` and `WriteResourceCache` `Sync` before the rename as well; both
    already used temp+rename
  - `LoadFile` switches to `toml.Decode` and refuses any `MetaData.Undecoded()`
    key, naming it (`proxy.typo_bind`); a newer `version` was already refused
  - `profileSlug`: an uppercase letter marks the stem as rewritten, so the file
    name carries the existing FNV suffix. `Work` and `work` now differ on a
    case-insensitive filesystem; a lowercase name keeps its plain file name
  - the real `~/.config/goTrust/config.toml` was loaded with the new parser
    (`--print-resources`, exit 0), so the strict decode does not reject the
    file this build already writes
- Results:
  | Check | Result |
  |---|---|
  | Pre-fix: unknown key / `Work` vs `work` | accepted / same path (case-folded); tests failed |
  | After: unknown key / file this build wrote | refused with the key / loads |
  | After: `Work` / `work` / lowercase stem | distinct paths / plain name kept |
  | After: client-data rewrite | no temporary file left, new content, 0600 |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: the `Sync` calls and the temp+rename in `writeClientData` are structural
  durability guarantees; no test can kill the process between write and rename
  deterministically, so they are covered by the observable side effects (no
  temporary file, atomic replacement) instead of a failing-before test. A
  profile whose name contains an uppercase letter gets a new client-data file
  name once; the old file is device material, not a credential, and a fresh
  login recreates it.
- Files created/modified: `config/config.go`, `config/config_test.go`,
  `session/session.go`, `session/session_test.go`, `session/resources.go`,
  `README.md`, planning files
- Next: P2 session (no `PhaseFailed: context canceled` after 断开, resources
  kept during the ladder, snapshot-published transitions, login during a
  restore, cancelled-login status).

### Phase 17: cancelled logins, ladder snapshots and publish order (P2)

- **Status:** done (one commit); the rest of the P2 batch remains
- Actions taken:
  - wrote the checks first and ran them in a throwaway worktree at HEAD: a login
    cancelled at method discovery published `connecting … failed`, the first
    connecting state of an attempt carried an empty grant, a second login was
    accepted during a client-owned restore (and failed later on an unrelated
    config error), a newer state overtook one whose delivery was still in
    progress, and a stopped attempt wrote `log in: context canceled` into the
    status line
  - `loginSession` returns at once on an already-cancelled context, and the
    failure helper publishes only when `ctx.Err() == nil`: 断开 asked for the
    stop, and its idle state must not be overwritten by a red failure
  - the four connecting steps go through `connectingState`, which carries
    `Resources`, `ClientIP` and `KeepAlive` from the previous snapshot, so a
    reconnect attempt no longer flashes an empty resource list
  - `Login` refuses a `PhaseReconnecting` state that still owns a client: that
    is the client's own restore, and a second login would replace and leak the
    client. The ladder still logs in, because it tore the client down first
    (`hasClient`)
  - `set`, `setTransition` and `recordKeepAlive` update and publish under one
    `publishMu`, so an older snapshot cannot be emitted after a newer one. The
    callback runs outside `m.mu`; it must not call back into the manager
    synchronously, which the app's handler does not do
  - `app.async` reports `已取消` when the attempt was stopped, even for an
    error that does not wrap `context.Canceled`
  - the proxy install step is now `installProxy(server, cancel, epoch)`; the
    guard compares the epoch under the app lock and closes a server that bound
    after the release
- Results:
  | Check | Result |
  |---|---|
  | Pre-fix: cancelled login / grant / restore / ordering / status | failed in all five |
  | After: same tests | pass |
  | Real `--no-ui` cancelled against a stalling gateway | no `session failed` line in the log, exit 1 with `context canceled` on the CLI |
  | `installProxy` after `releaseProxy` | refuses, port free, view empty |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Note: the proxy-epoch guard already existed (P0-2); the batch asked for a
  verification, so the test pins the decision and the extraction adds no
  behaviour. The restore refusal has no live gateway: its regression uses a
  zero-value client in the state.
- Files created/modified: `session/session.go`, `session/session_test.go`,
  `main.go`, `app_test.go`, planning files
- Next: P2 keep-alive interval bound and dead-code/metadata cleanup.

### Phase 17: keep-alive floor, dead code and snapshot metadata (P2)

- **Status:** done (one commit); the P2 batch is complete, the verification
  pass remains
- Actions taken:
  - wrote the keep-alive checks first and ran them in a throwaway worktree: a
    hand-edited `1s` interval reached the probe loop, and the settings save
    accepted `5s`
  - `config.MinimumKeepAliveInterval` is 10 s. `KeepAliveInterval` clamps a
    positive value below it instead of reverting to the 90 s default, and the
    settings save rejects one with a message naming the floor; the view says
    `至少 10 秒；默认 90 秒`
  - dead code removed after checking each symbol: `Config.SelectProfile`
    (the app switches with `ActiveProfile` plus `SaveFile`), `RenameProfile`
    (the settings draft renames and `Apply` checks duplicates),
    `ClientDataPath` (callers use `ClientDataPathFor`), `Logger.Writer`,
    `cliPrompter.logger` (never read) and `ReconnectState.Reason` (the reason
    is already `Detail`)
  - `State.Attention` is now a bool: only its presence was used, by the app to
    release the listeners and reveal the window; `failureState` takes the flag
    and the explanatory text that was never displayed is gone
  - the tests that pinned the removed API were deleted or moved to the
    remaining one (`ClientDataPathFor(cfg.Active())`, a direct
    `ActiveProfile` assignment in the tray tests)
  - `--print-resources` starts with `saved <RFC3339> by profile <name> via
    <gateway>, tunnel <ip>`, so a days-old cache is attributable
  - the login channel's certificate check is documented at its call site;
    the tunnel authenticates at the aTrust layer, and the proxy bind stays on
    loopback
- Results:
  | Check | Result |
  |---|---|
  | Pre-fix: `1s` interval reaches the loop / settings accept `5s` | both failed |
  | After: clamp / floor accepted / 30 s kept | 10 s / pass / 30 s |
  | `--print-resources` with a temp cache | header, then the rows, exit 0 |
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean |
- Files created/modified: `config/config.go`, `config/config_test.go`,
  `ui/settings.go`, `ui/settings_test.go`, `session/session.go`,
  `session/reconnect.go`, `session/reconnect_test.go`, `logging/logging.go`,
  `prompt.go`, `main.go`, `app_test.go`, `ui/view_test.go`, `README.md`,
  planning files
- Next: the Phase 17 verification pass (`go test -race`, the real UI smoke for
  断开 during a login, settings delete, session expiry then reconnect, and
  `reuse lint` on the committed tree).

### Phase 17: verification pass

- **Status:** automated part complete; the native-window click-through stays
  manual
- Actions taken:
  - `go test ./...` and `go test -race ./...`: all six packages pass
  - `go vet ./...`, `gofmt -l .`, `reuse lint`: clean, 47/47 files compliant
  - real binary with a temporary `XDG_CONFIG_HOME`/`XDG_STATE_HOME`: the log
    records `goTrust ready`, stderr holds only the expected
    `libayatana-appindicator is deprecated` warning (no GL/GTK error), and
    SIGTERM exits 0. A second run against a stalling gateway, cancelled with
    SIGTERM, logged no `session failed` line
  - first attempt on the live desktop: the guide's crop located the window and
    its `连接中` state, but the desktop changed between the screenshot and the
    click (other applications rose), so the click was unconfirmable and blind
    retries were stopped
  - second attempt in a headless session, which is now the documented path:
    `mutter --headless --no-x11 --wayland-display=wayland-gt
    --virtual-monitor 1280x1024`, its own `Xwayland :99`, the app with
    `DISPLAY=:99 GDK_BACKEND=x11`, capture with `import -display :99 -window
    root`, input through python-xlib XTEST. Nothing appears on the operator's
    screen and the capture holds only the nested display
  - the window rendered `连接中` / `127.0.0.1:<stall>` /
    `discovering the authentication methods…` / 断开·设置·退出 and the spinner.
    Clicking 断开 returned it to `未连接` with the status `已取消`, the log had no
    `session failed`, and the process stayed up — the P2 cancellation fix in a
    real GTK window
  - 设置 opened beside the main window (no GL error), 新建 added a draft
    profile, 删除 removed it without a panic, the settings file still held one
    profile (the delete was a draft edit), 关闭 hid the window, reopening
    reloaded `默认` with 删除 disabled, and 退出 quit the process with only the
    expected appindicator warning on stderr
  - the Xwayland screen starts at 640x480 and clipped the 600x760 settings
    window; `xrandr --newmode/--addmode/--output XWAYLAND0 --mode 1024x768` on
    :99 made the pinned footer reachable
  - all processes (app, `Xwayland :99`, headless mutter, the stalling listener,
    `ydotoold`) were stopped and `/tmp/gt-nested` deleted, screenshots included.
    Earlier live-desktop crops had included an unrelated notes window with
    account text; they were deleted at once and never committed or pasted
  - every process (`goTrust-ui`, the stalling listener, `ydotoold`) was stopped
    and `/tmp/gt-ui` removed
- Results:
  | Check | Result |
  |---|---|
  | `go test ./...`, `go test -race ./...` | pass |
  | `go vet ./...`, `gofmt -l .`, `reuse lint` | clean / clean / 47/47 |
  | Real binary: ready, stderr, SIGTERM | `goTrust ready`, appindicator warning only, exit 0 |
  | Cancelled login against a stalling gateway | no `session failed`, CLI reports the cancellation |
  | Native window: 断开 during login / 设置 delete / expiry then reconnect | manual; the handlers are covered by `TestACancelledLoginPublishesNoFailure`, `TestAStoppedAttemptReportsCancelled`, the P0-3 settings-delete tests and the ladder tests |
- Note: the expiry-then-reconnect path needs the real gateway and was already
  recorded as a manual item in Phase 14.
- Files created/modified: planning files only
- Next: Phase 8 stays deferred; Phase 18's double-click waits on a mygo Linux
  activation backend (draft in `findings.md`).

### 2026-10-24: research and the 断开 panic

- **Status:** complete (research); one fix shipped
- Actions taken:
  - **断开 exited the app.** The journal of the installed run shows
    `panic: EOF` in `stack/gvisor.(*Stack).Run` (`stack.go:244`), goroutine
    created by `loginSession` (`session/session.go:354`), at the second the
    proxy closed. The fork's read loop panics unless an `internal/hook_func`
    terminal flag is set, which an external module cannot reach. Fixed locally:
    `runPacketStack` recovers, logs `the packet stack stopped: …` and lets the
    keep-alive observe the dead tunnel; regression
    `TestAPacketStackPanicDoesNotKillTheApp`; rebuilt as `r44`.
    Follow-up for the fork: return instead of panic, and surface a read error
    while online.
  - **Keyring.** The branch is EZ4Connect's local-only
    `feat/credential-keychain`, not in the fork and not on GitHub.
    It is app-side: libsecret `secret-tool` with `service`/`scope`/`key`
    attributes on Linux, Keychain/CredMan elsewhere, settings file as fallback,
    migration on load, scope = profile id or `--config` path, keys `Password`,
    `TOTPSecret`, `CertPassword`. Even there the Go core receives the secret on
    argv/env. goTrust's choke points: `cfgForDisk`, the login snapshot, the
    profile move/erase hooks.
  - **Password change.** Endpoints verified:
    `POST forced-change` (form-urlencoded, `skip:true`),
    `POST reset-path` (`{oldPassword,newPassword[,
    account-field]}`), `POST portal/public/resetPsw` (forgot flow),
    `GET portal/user/policy-object`, `POST portal/public/policy-probe`.
    The forced signal is `code 0` + `next-step:"forced-change"` with
    `change-code` 75500402/75500404 and the `policy-object` rule object; the
    fork fails with "unsupported next authentication service". EZ4Connect and
    atrust-quadlet implement nothing here (the latter hands over to VNC).
- Files created/modified: `session/session.go`, `session/session_test.go`
  (that fix); `findings.md`, `task_plan.md`, this log
- Next: the operator picks a phase; the open phases are itemised in `task_plan.md`.

### Phase 18: tray interaction

- **Status:** in_progress (menu item done; double-click blocked on mygo)
- Actions taken:
  - restored 显示窗口 after the first attempt wrongly duplicated it, and added
    打开默认应用: first granted application URL (fallback: first host rule with
    a browser link) through `Actions.OpenResource`, i.e. the desktop's default
    browser; disabled while the grant has no URL
  - added `session.Resources.DefaultAppURL()` and tray tests for the URL choice
    and the disabled state
  - traced the double-click request to mygo v0.6.3: its Linux backend installs
    an AppIndicator menu and never calls `Clicked`/`RightClicked`; only darwin
    and windows do, so the panel owns the primary click and a double-click is
    not observable
  - verified `libayatana-appindicator-glib` is not a drop-in (GMenu vs
    GtkMenu, no `set_icon_full`, no GTK3) and drafted the upstream issue/PR
- Results:
  | Check | Result |
  |---|---|
  | `go test ./ui ./session` | pass (new tray tests included) |
  | `go vet ./...`, `gofmt -l .` | clean |
  | Native tray menu | not clicked; items are a pure function and tested |
  | Double-click | not implemented: mygo exposes no Linux click event |
  | glib backend | requires a new mygo backend, not a .so swap |
- Files created/modified:
  - `ui/view.go` (tray menu item), `ui/view_test.go` (two tests),
    `session/resources.go` (`DefaultAppURL`), `README.md`
  - planning files
- Next: file the upstream mygo issue/PR when the operator approves (draft in
  `findings.md`); Phase 17 P0 fixes remain the release blocker.

### Phase 19: password change and forced reset

- **Status:** complete
- Actions taken:
  - the reset flow uses these request shapes
    (recorded in `findings.md`) (recorded in `findings.md`): `forced-change` is
    form-urlencoded with RSA `oldPassword`/`newPassword` and optional
    `account-field`/`skip=true`; `reset-path` is JSON; `policy-probe` takes
    the plaintext; the account field is required whenever the page shows it
  - fork (`fcff928`, pushed to `YangtseSu/zju-connect` `dev`): the password
    answer keeps the forced-change payload, `LoginOptions` gains
    `PasswordChangeHandler` and `OnPasswordChanged`, and the chain answers
    `forced-change` in place (change or skip), continuing with the answer's
    `next-step` and skipping `reportEnv` only when the portal issued no
    ticket. A nil handler yields `*PasswordChangeRequiredError`, and the
    session outlives setup so `ChangePassword` can finish it. Added
    `ResetPassword` (security endpoint), `PasswordStrategy`, `Rules`/
    `Violations`, and `CheckWeakPassword`
  - live attempts: `r47` failed on `change-password` with 75500000 (bare
    passwords, no shared query); the next package added the
    `password-payload` suffix, the shared query and an extra `authConfig`
    refresh, and failed with 75500001 (the refresh replaced the login's
    key/nonce and CSRF), and finally (3) the live gateway names the change in
    the auth-check answer, which we parsed for next-step only, losing
    `change-code`/message/strategy and the account field. Final fix: capture
    the payload from both answers, use the login answer's key/nonce from the
    session, no extra `authConfig`; unit tests cover the live
    psw → auth-check(change-password) → change-password order. After the change the portal
    keeps the account online and refuses the pending `auth-check` with
    75500006; the fork now signals `auth.ErrReauthRequired` and the manager
    retries the login once with the new credential (proven by the operator's
    manual re-login in the same log). The in-app reset then failed at the
    policy probe, which the portal also encrypts; fork `3477534` sends
    the nonce-encrypted value, keeps the renewal answer from clearing the
    CSRF/key/nonce, and logs the three reset-path requests
  - goTrust: `session.Options.ChangePassword`/`OnPasswordChanged`, the forced
    panel in the login flow (message, account, rules, prompt, skip, cancel,
    context release), the voluntary panel in the settings window (offered only
    for the profile that owns the live session; weak check first), and
    `storeChangedPassword` rewriting the profile credential (disk only when
    `store_password` is on). The voluntary entry is labelled 重置密码 under
    连接 → 密码安全 and its panel follows the reset dialog:
    旧密码/新密码/确认密码, 密码规则, 确定/取消, and the
    "密码重置后，其他终端已在线账号需重新登录" note
  - `--no-ui` gets a clear refusal: the real binary against a fake gateway
    reached the demand and exited with "the gateway requires a password
    change, which needs the window"
- Results:
  | Check | Result |
  |---|---|
  | fork `go test ./...`, `go vet` | pass |
  | `go test -race ./...` (goTrust, pinned `fcff928`) | pass |
  | `reuse lint` | 52/52 compliant |
  | Forced panel, rendered | `ui.NewTester` PNG (light), inspected |
  | Forced panel, native | headless mutter + Xwayland, real GTK window, the gateway's message and fields visible |
  | `--no-ui` refusal | real binary + fake gateway: clear message, exit 1 |
  | Live forced change, first package (`r47`) | failed `75500000 (Parameter error or missing occurred)` |
  | Causes / fixes | (1) missing shared `shared-params` and the login-style `<password>_nonce` encryption; (2) the extra `authConfig` before encrypting replaced the login's key/nonce and CSRF → `75500001 (authentication timed out)`; (3) the live gateway announces the change in the **auth-check** answer, which was parsed for next-step only, so the challenge reached the window with code 0 and no `account-field` for `75500402`. Final: read the payload from auth-check too, use the password answer's key/nonce and CSRF, no extra `authConfig` |
  | Live forced change, third package (`r49`) | change succeeded at 23:03:25 and the credential was saved; the follow-up `auth-check` got `75500006` ("account already logged in") |
  | Fix for the continuation | the change leaves the account online and closes the pending chain; the fork now returns `auth.ErrReauthRequired` and the manager retries the login once with the saved credential (the operator's manual re-login with the new password succeeded at 23:03:49) |
  | Live trigger | a new account's first login (`change-code 75500401`); the same `forced-change` flow the 90-day policy uses, so the live evidence covers both |
  | Live forced change after the re-login fix | accepted by the operator |
  | In-app 重置密码 (`r50`) | failed before the reset request: `/public/policy-probe` rejected the plaintext password with "Parameter error or missing occurred" |
  | Fix | fork `3477534`: the weak probe sends `<password>_nonce`; a renewal answer without CSRF/key/nonce no longer clears them; reset/weak/strategy calls log at info. Pin `v1.3.2-0.20261024152456-347753428ab2`, package `r51` |
  | In-app 重置密码 after the fix | phase closed by the operator; the probe fix is in `r51` |
  | Live 90-day expiry | not triggered; operator test pending |
- Files created/modified:
  - fork: `client/atrust/auth/password_change.go` (new),
    `password_change_test.go` (new), `auth.go`, `psw.go`,
    `client/atrust/client.go`
  - goTrust: `session/password.go` (new), `session/password_test.go` (new),
    `session/session.go`, `ui/password.go` (new), `ui/password_test.go` (new),
    `ui/settings.go`, `ui/view.go`, `ui/render_test.go`, `prompt.go`,
    `password_test.go` (new), `main.go`, `go.mod`, `go.sum`, `README.md`
  - planning files
- Next: live-test the forced change when the gateway first demands it;
  Phase 20 (keyring) remains.

### Phase 21: 终端管理与设备信任

- **Status:** complete
- Actions taken:
  - fork (`2e39e25`, on the fork's `dev`): `queryDevice` keeps both status
    lists and the counts the portal's 终端管理 page shows; `Client.`
    `QueryTerminals` reads `status=trust` and `status=untrust` in one call and
    `Client.SetTrusted` reads `selfId` first, so the trust call goes to this
    device; `LoginOptions`/`SetupOptions` gain `DeviceBindHandler`, and
    `auth/bindAuthDevice` asks it before answering. An unset handler keeps the
    automatic answer, so the CLI is unchanged
  - goTrust: `session.Terminals`/`Terminal`/`DeviceBindChallenge` aliases,
    `Manager.Terminals`/`SetTrusted`, `Options.ConfirmDeviceBind`, and the
    login wrapper that marks the attempt as needing a person
  - settings 终端管理 group above 高级: counts, both lists, 刷新,
    信任此设备/取消信任, the current device marked by `selfId`; the window reads
    the lists when it opens and re-reads after a trust change
  - 绑定此设备 panel in the main window; 取消 aborts the login with the user's
    answer
  - the lists are read through the live session, so a disconnected window only
    says 连接后可读取终端列表
- Verification:
  | Check | Result |
  |-------|--------|
  | fork `go build ./...`, `go test ./client/atrust/...` | green |
  | goTrust `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` | green |
  | `reuse lint` | compliant |
  | UI: 终端管理 renders the portal answer, marks the current device, hides the actions while offline | `ui.NewTester` test |
  | UI: 绑定此设备 reaches 允许绑定 and 取消 | `ui.NewTester` test |
  | Live: lists, trust toggle, binding prompt | accepted by the operator |
- Files created/modified:
  - fork: `client/atrust/auth/device.go`, `device_test.go` (new),
    `enhanced.go`, `enhanced_test.go`, `auth.go`, `client/atrust/client.go`
  - goTrust: `session/session.go`, `prompt.go`, `main.go`, `ui/prompt.go`,
    `ui/prompt_test.go`, `ui/settings.go`, `ui/settings_test.go`, `ui/view.go`,
    `README.md`, planning files
  - `go.mod`/`go.sum`: fork pin moves to the commit above
- Next: Phase 20 (keyring) remains open.

## Error Log

| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-10-23 | `go get github.com/mythologyli/zju-connect@dev`: unknown revision | 1 | `main` is the default branch; used `replace` with a fork pseudo-version |
| 2026-10-23 | pseudo-version `v1.3.1-0.20261020T02-…` rejected | 2 | timestamp needs 14 digits; stripped the separators first |
| 2026-10-24 | `go mod tidy` on the fresh fork commit: goproxy.cn sumdb lookup 404 | 1 | a just-pushed pseudo-version is not in the checksum database; fetched it directly with `GOPRIVATE=github.com/YangtseSu/zju-connect GOSUMDB=off` and the hashes were computed locally |

## 5-Question Reboot Check

| Question | Answer |
|----------|--------|
| Where am I? | Phase 21 is complete: the settings window reads the portal's 终端管理 lists and changes this device's trust, the main window confirms a device binding, the live acceptance passed, and `r56` carries both |
| Where am I going? | Keyring storage (Phase 20) is next. Phase 8 (dae) stays deferred and Phase 18's double-click waits on mygo. Phase 16 is closed: `gotrust-git` is installed |
| What's the goal? | a daily-use aTrust tray client |
| What have I learned? | see `findings.md` (license chain, embeddable surface, shutdown gap, mygo API traps, auth lookup and second factors); the client's own recovery ladder is the only trustworthy session-loss signal; the 2026-10-24 review adds the lock-order, listener-lifetime and resolver/stack defects |
| What have I done? | four packages, the polished windows, the reliability state machine, the fork hooks, the new icon/desktop files and the package, the forced/voluntary password change, and the portal terminal list with device trust; a full review with five reproduced defects, recorded as Phase 17 |
