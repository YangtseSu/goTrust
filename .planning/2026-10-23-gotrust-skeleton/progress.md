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
  tunnel works; recorded in `findings.md` as the next follow-up (a fork hook or
  confirming on a successful keep-alive probe).
- Files created/modified: `session/session.go`, `session/session_test.go`,
  `main.go`, planning files
- Next: P1-8/P1-9 (`--config` must move the client data, a profile delete or
  rename must take its file with it).

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

## Error Log

| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-10-23 | `go get github.com/mythologyli/zju-connect@dev`: unknown revision | 1 | `main` is the default branch; used `replace` with a fork pseudo-version |
| 2026-10-23 | pseudo-version `v1.3.1-0.20261020T02-…` rejected | 2 | timestamp needs 14 digits; stripped the separators first |

## 5-Question Reboot Check

| Question | Answer |
|----------|--------|
| Where am I? | Phase 17 recorded (review remediation); P0 and P1-4/P1-5/P1-6/P1-7 landed; Phase 16 `pacman -U` still awaits sudo |
| Where am I going? | Remaining P1 batches: client-data path/rename, config race, settings freshness, redaction and CONNECT, rotation; then the P2 batch; Phase 8 (dae) last |
| What's the goal? | a daily-use aTrust tray client |
| What have I learned? | see `findings.md` (license chain, embeddable surface, shutdown gap, mygo API traps, auth lookup and second factors); the client's own recovery ladder is the only trustworthy session-loss signal; the 2026-10-24 review adds the lock-order, listener-lifetime and resolver/stack defects |
| What have I done? | four packages, the polished windows, the reliability state machine, the fork hooks, the new icon/desktop files and the package; a full review with five reproduced defects, recorded as Phase 17 |
