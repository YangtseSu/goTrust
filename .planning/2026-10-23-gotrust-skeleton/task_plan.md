# Task Plan: goTrust

## Goal

A cross-platform tray app in `~/projects/goTrust` that embeds the aTrust
protocol from the fork of `zju-connect`, logs in (password, click captcha,
one-time codes, external SSO), keeps the session alive, exposes SOCKS5 + HTTP
proxies, and shows the gateway's granted resources. The live gateway path is
verified. The remaining work turns the skeleton into a daily-use product:
settings and startup, auth methods, resource display, reliability, an Arch
package, and a UI polish pass.

## Next Step

Phase 19 (password change, forced and voluntary) is complete: the live gateway
forced it on a new account's first login, which uses the same `forced-change`
flow as the 90-day expiry. Phase 20 (keyring, backend decided at
  execution) is next.
Research is in `findings.md`. Phase 17 is closed apart from the manual
expiry→reconnect case; a live 断开 exposed a fatal packet-stack panic, fixed in
that fix (package `r44`).

## Current Phase

Phase 19 is complete. The live trigger was a new account's first login
(`forced-change`, code 75500401), the same flow the 90-day policy uses: the
change, the credential rewrite and the recovery after the portal closed the
pending chain all ran against the gateway.
Phase 17 is complete apart from the manual expiry→reconnect case, Phase 16 is
closed, Phase 8 (dae export) stays deferred, and Phase 18's double-click waits
on mygo.

## Phases

### Phase 1: License and layout decision

- [x] Confirm the license obligation created by linking `zju-connect` (AGPL-3.0-or-later)
- [x] Confirm the dependency form: compiled-in library, pinned to the fork's `dev` by `replace`
- [x] Confirm the package split: `config`, `session`, `proxy`, `ui`
- **Status:** complete

### Phase 2: Module scaffold

- [x] `go.mod` with the fork `replace` pinned to the tested pseudo-version
- [x] `LICENSE` (AGPL-3.0), `.gitignore`, `README.md`
- [x] `config` package: settings struct, XDG config path, load/save, validation
- **Status:** complete

### Phase 3: Session layer

- [x] `session.Manager`: state machine, event fan-out, `client_data` persistence
- [x] aTrust client assembly from public packages (underlay, atrust, gvisor/tcptunnel stack, resolver, dialer)
- [x] Challenge prompter interface so the UI supplies codes and callback URLs
- [x] Keep-alive loop over the session dialer with a configured target
- **Status:** complete

### Phase 4: Proxy layer

- [x] `proxy.Server` over an injected `DialFunc` (no dependency on `internal/`)
- [x] SOCKS5 listener with domain pass-through
- [x] HTTP proxy: CONNECT tunnel plus absolute-URI forwarding
- [x] Context-based shutdown for both listeners
- **Status:** complete

### Phase 5: UI shell

- [x] mygo app: single instance, tray, one window
- [x] Login form, status line, listener addresses, quit
- [x] `--no-ui` mode for headless runs
- **Status:** complete

### Phase 6: Publication

- [x] Privacy review of the tree, the diff, and the object store
- [x] Sanitisation of operator-specific addresses and service names
- [x] Unreachable blobs purged so the history has nothing to reveal later
- [x] Repository created private, `main` pushed
- **Status:** complete

### Phase 7: Verification

- [x] `go build ./...` and `go vet ./...` clean
- [x] Proxy integration test with a fake dialer and a local echo server
- [x] Launch the real binary, observe window and tray, then SIGTERM
- [x] SOCKS5 resolves hostnames through the tunnel resolver (regression test)
- **Status:** complete

### Phase 8: Transparent proxy via dae (deferred, last)

Deferred by the operator: nice-to-have, after the product phases below. Research
is in `findings.md`; dae stays a separate daemon.

- [ ] `--print-dae-config` emits `node`/`group`/`routing` for the configured listeners
- [ ] Intranet prefixes derived from the session's IP resources, not hardcoded
- [ ] The emitted rules always start with `pname(goTrust) -> must_direct`
- [ ] Intranet prefixes precede any private-range preset, and the reason is a comment in the output
- [ ] Tests: prefix derivation, loop rule present, ordering, an unconnected session still emits a usable template
- [ ] README section "Transparent proxy (optional)" with the daemon boundary and the DNS caveat
- **Status:** pending

### Phase 9: Live gateway verification (complete)

- [x] Real login through the window, including the gateway's click captcha
- [x] Proxied request over HTTP and SOCKS5 to the granted application
- [x] Keep-alive over the derived application target
- [x] Session restore from the stored ticket without a new captcha
- **Status:** complete
- **Note:** the account grants one network application, so the remote-DNS
  keep-alive fallback does not run on this gateway and stays untested.

### Phase 10: Click captcha (complete)

The gateway the operator uses asks for a click captcha. The reference
implementation is `EZ4Connect`'s `GraphCaptchaWindow`/`CaptchaCanvas`; the
protocol writes `{"coordinates":[[x,y],…],"width":W,"height":H}`.

- [x] `ui.Captcha`: image, source size, picked points, display mapping
- [x] In-window panel: numbered markers, 撤销/清空/提交/取消, fields hidden while pending
- [x] `session.Options.Prompt` takes the login context so a cancel releases the handler
- [x] `uiPrompter` decodes the image, blocks the login goroutine, and returns the points
- [x] `--no-ui` keeps refusing with a message that names the window requirement
- [x] Tests: pixel mapping, panel clicks, submit payload, window/context cancel, bad image
- [x] README "Logging in" section
- [x] 登录域 field, applied by both the settings and the credentials path
- [x] Session and proxy lifetimes detached from the login-attempt context
- **Status:** complete

### Phase 11: Config v2, profiles, settings, startup and tray (complete)

Requirements 1, 2, 3, 5, 6, 7, and the foundation for 10. A dedicated settings
window keeps the main window about status, connection and resources; EZ4Connect
does the same. Settings are persisted as versioned TOML.

- [x] Config v2: `version`, `active_profile`, global `app`/`proxy`/`logging`
      sections and a `profiles[]` array; each profile owns server, auth, TOTP,
      keep-alive and debug settings. Proxy ports stay global so a switch does
      not move the listeners
- [x] No migration: v1 was a JSON file from an unreleased build, so a leftover
      `config.json` is ignored. Missing fields take defaults and the TOML file
      stays valid; a newer `version` is refused
- [x] Profiles: create, rename, duplicate and delete; one active session at a
      time; switching disconnects first, then reconnects when auto-connect is
      on; the active profile is visible in the window and the tray submenu
- [x] Logging: line log to `$XDG_STATE_HOME/goTrust/goTrust.log` (0600, 5 MiB
      × 3 rotation), stderr in `--no-ui`, `verbose` level from settings, and
      redaction of passwords, cookies, tokens and captcha data; bridge the
      upstream global `log` to the same writer; a settings button opens the
      log directory
- [x] Defaults: login domain `local`, keep-alive 90 s, SOCKS5 `127.0.0.1:1080`,
      HTTP `127.0.0.1:1081`
- [x] Settings window: connection (gateway, port, username, login domain,
      password/store), proxies (bind addresses and enable toggles), keep-alive
      (enabled, interval, manual targets), startup (autostart, auto-connect,
      start hidden, auto-reconnect), advanced (verbose, log directory, client
      data path)
- [x] Apply rules: proxy binds and keep-alive take effect on the next connect;
      credentials and domain on the next login; autostart and verbose apply
      immediately
- [x] Tray: disabled status line (`已连接 203.0.113.4`, `连接中`, `失败: …`),
      connect/disconnect toggle, profile submenu, show window, settings, quit;
      tooltip mirrors the phase and the last keep-alive result
- [x] Autostart at user login through `mygo.App.SetOpenAtLogin` (XDG autostart
      on Linux) and `WasOpenedAtLogin` to start hidden; a systemd unit is not
      the desktop equivalent and is not planned here
- [x] Auto-connect on `WhenReady` when enabled and a stored password or client
      data exists; reveal the window for a captcha/second factor and fail
      visibly when credentials are missing
- [x] Tests: TOML round-trip, profile switch/delete, log
      rotation/redaction, settings view and callbacks, tray menu per phase,
      autostart entry in a temp `XDG_CONFIG_HOME`, auto-connect decision
- **Status:** complete
- **Note:** the settings window was rendered and its callbacks tested; clicking
  it in the real window on Wayland belongs to the Phase 15 smoke. The real
  binary was launched with a temporary config and autostart entry.

### Phase 12: Auth methods and second factors

Requirements 3 and 4. The fork exposes `atrust.GetAuthInfoList` as a public
pre-login call, so method discovery needs no protocol change. aTrust matches
`authType` and `loginDomain` exactly.

- [x] `session.DiscoverAuthMethods(ctx, cfg)` over the gateway's `authConfig`
      (server, port, own underlay dialer, context cancellation); returns name,
      type, domain and URL. `atrust.GetAuthInfoList` itself builds a dialer and
      takes no context, so the exported session call is used instead
- [x] Settings: "获取认证方式" button and a list/dropdown; store the chosen
      `auth_type` and `login_domain`; keep `local` as the default domain
- [x] Resolution: auto mode uses the only offered method, else the saved one,
      else asks the user to choose; a stored method the gateway no longer offers
      fails with the list instead of a silent fallback
- [x] Code challenge dialog for SMS/TOTP/RADIUS: message, code input, and
      "跳过以后的二次认证" when the challenge allows it; optional TOTP secret
      (store only on opt-in) generates the code in-app and is passed upstream
- [x] External SSO (CAS/OAuth2): open `LoginURL` in the system browser and take
      the callback URL, validating path and ticket/code; a built-in webview is a
      later option, not a dependency
- [x] Cancel and login-context timeout release the handler, as the click
      captcha already does; 断开 releases all pending prompts
- [x] Tests: auth-info parsing/resolution, prompter dialogs and cancel, SSO
      callback validation; SMS/TOTP/SSO marked **untested live** (the verified
      account has no second factor) in the README and progress log
- **Status:** complete
- **Note:** a primary SMS login is refused with a clear message because the
  settings window collects no phone number; a gateway whose only method is one
  starts with a password, CAS or OAuth2. The new panels were rendered headlessly
  and inspected; the settings picker was not clicked on the real desktop, which
  Phase 15 covers.

### Phase 13: Granted resources

Requirement 8, informed by `atrust-quadlet --apps`: the user must see what the
account may reach and which URL to open in a browser.

- [x] Retain IP, domain and DNS resources plus app IDs in `session.State`
      instead of discarding them after setup (`State.Resources`, cleared by
      Logout)
- [x] Check whether the resource model can carry app names and launch URLs;
      extend the fork's `resourceApp` if the fields are available, otherwise
      derive display entries from addresses/ports. The portal payload names
      each app, so the fork now parses `name`, group `name`, `accessAddress`,
      `openModel.model` and `webRelativeDomainList` into `client.App`, exposed
      by `Client.Apps()`; the pin moved to
      `v1.3.2-0.20261023210742-7958d6a202ff` (fork dev `7958d6a`)
- [x] Display model: domain `name[:port]`, IP/range with port range and protocol,
      DNS name→IP; apps with group and launch method. `http://`/`https://` is
      suggested for a single host whose rule covers 80/443, https first; app
      rows use the portal's access address or first web domain
- [x] Main window: scrollable resource list with open-in-browser and copy
      actions, plus gateway, tunnel IP and proxy addresses
- [x] Cache the last snapshot in the config directory at 0600 and print it in
      `--no-ui` runs; `--print-resources` prints the cached snapshot and exits
      without a login
- [x] Tests: derivation/formatting, URL suggestion, empty session, cache
      round-trip, app metadata; no real intranet address in the repository
- **Status:** complete

### Phase 14: Reliability (complete)

Inspired by EZ4Connect's reconnect ladder and the keepalive family's probe
discipline; not a new user-facing feature list, but the session must not lie.

- [x] Auto-reconnect option (default off): attempts after session loss with
      1/2/4/8/16 s backoff, at most 5; state and tray show
      `重连中 (n/5, next in Xs)`; `PhaseReconnecting` and `State.Reconnect`;
      the ladder runs only when the client's own renewal has failed, and every
      attempt is a full login
- [x] Stop the ladder when a captcha, one-time code or SSO callback is needed,
      and bring the window forward instead of retrying blindly: an observing
      prompter marks the attempt, and every prompt panel already reveals the
      window; the ladder never starts without a saved password, because a
      fresh login is the only way to a session
- [x] Expose `last target`, `last result`, `last at` in `session.State` by
      wrapping `service.KeepAlive` (a fork result hook) rather than duplicating
      its logic; manual targets first, derived targets next; `https://` targets
      are warned about (upstream validation) and the settings window says why;
      the 90 s default is unchanged. The fork's `KeepAliveOptions.OnResult`
      reports every attempt and the DNS fallback; the old log-line parser is
      gone
- [x] Surface session expiry and reauthentication: the fork reports
      `online`/`restoring`/`relogin-required` through
      `SetupOptions.OnSessionState`, goTrust mirrors it, and a lost session
      with auto-reconnect off becomes a visible `PhaseFailed` with attention
      instead of an online-looking dead tunnel
- [x] No session reuse across connections. A session ends with its connection:
      a stored sid ticket is not a login and cannot be exchanged after the
      portal ended the session. Auto-connect on startup and the reconnect
      ladder therefore require a **saved password**; client data alone is not
      enough. The operator confirmed that both `atrust-quadlet` and
      `atrust-dev` tried the restore path and the portal rejected it
      (`75500307` while logged out; `ticketExchange` needs a live session), so
      the fork commit that enabled it from stored client data was removed
      from `dev` (history rewritten; `1d0673b` is the only new commit)
- [x] Tests: backoff schedule and cap, target ordering, keep-alive result
      transitions, reconnect stops on a challenge / cancellation,
      session-loss state, no ladder without a password, foreign-client
      callbacks ignored
- **Status:** complete
- **Note:** the live gateway was not re-run; the ladder and the expiry path are
  covered by unit tests and the fork's own suite. The verify smoke launched the
  real binary with temporary config/state directories.

### Phase 15: UI polish

Requirement 10, after the functional windows exist.

- [x] One design system for every window: a four-step type scale derived from
      the theme's font size, a 24/16/8 spacing grid, and phase colours taken
      from the theme's semantic tokens instead of hard-coded hex values
- [x] Layout and typography pass over main, settings, captcha, code, SSO and
      chooser views; section headings, captions and a primary/secondary button
      hierarchy
- [x] Status chip and detail lines per phase; success, warning and failure
      colours, a spinner and disabled 连接 while work runs, 断开 kept usable as
      the cancel, and the tray toggle disabled only when it would start a race
- [x] Window sizing and scrolling: the main window shows the resource list
      with a pinned footer (`540×640`, minimum `440×400`); the settings window
      scrolls its groups and pins 保存/关闭 (`600×760`, minimum `560×440`) with
      a divider and the status line above the actions
- [x] Render every state (idle, connecting, online, reconnecting, failed, all
      four prompts, settings, light and dark, both minimum sizes) through
      `ui.NewTester` and inspect the images, then launch the real window on
      Wayland: CJK, the settings window opened by keyboard, the new tray icon
- [x] Accessibility: headings carry `RoleHeading`/`Level`, the phase chip is a
      named status, repeated resource buttons have descriptions naming their
      row, every field is wrapped in `Field` (label association) and Tab reaches
      every action in reading order (tested)
- [x] New icon: a scalable shield-and-arrow mark drawn at 16/32/128 and
      inspected, a symbolic variant, a 64×64 tray PNG embedded in the binary,
      and the desktop entry and icons shipped from `data/` instead of the
      package's `$srcdir`
- **Status:** complete
- **Note:** the native tray menu was not captured on the desktop: ydotool's
  absolute pointer could not reliably hit panel icons here, and blind clicks on
  the operator's screen were stopped. The menu's items, toggles and status
  lines are covered by `TestTrayMenu*`; the icon itself was seen and stayed in
  the panel.

### Phase 16: REUSE compliance and Arch package `gotrust-git`

Requirements 4 and 9. REUSE first, because the package ships the license and
the repository is public. Then a self-use VCS package; it does not go to AUR.

- [x] `LICENSES/AGPL-3.0-or-later.txt`, `REUSE.toml`, and SPDX headers in every
      Go file; annotate docs, `go.mod`, `go.sum`, `.gitignore` and `.planning/`
- [x] `reuse lint` clean (28/28 files); documented in README
- [x] Create `~/projects/arch-pkgs/gotrust-git/` with `PKGBUILD`, `REUSE.toml`,
      `goTrust.desktop` and an icon
- [x] Source: local `git+file://$HOME/projects/goTrust#branch=main`, because
      makepkg runs git with `GIT_CONFIG_GLOBAL=/dev/null` and cannot use the
      private GitHub credential helper; the GitHub URL is a one-line switch
      once the repository is public or the helper is made makepkg-safe
- [x] Install layout: `/usr/bin/goTrust`, the desktop entry, the two icons and
      the AGPL license; runtime deps `gtk3`, `libayatana-appindicator`,
      `hicolor-icon-theme` (WebKitGTK is lazy and stays out)
- [x] `check()` runs `go test ./...`; `pkgver()` uses a rev-count/hash fallback;
      build is `-trimpath`, PIE, FULL RELRO/BIND_NOW via the external linker
- [x] `makepkg -fC` rebuilt `gotrust-git r18` after Phase 15; the
      desktop entry and both scalable icons now come from `data/` in the
      checkout instead of the package `$srcdir`. `namcap` keeps only the
      intentional purego/dlopen and libresolv warnings; the extracted binary
      starts, writes `goTrust ready`, and exits 0 on SIGTERM; the installed
      desktop entry passes `desktop-file-validate` (one category hint)
- [x] Install with `pacman -U`; the installed package is `gotrust-git
      r32`
- **Status:** complete

### Phase 17: Code review remediation (2026-10-24)

Full-tree review of the tree reviewed then (config/logging, session, proxy,
UI/wiring,
cross-cutting). Evidence, reproductions and upstream references are in
`findings.md` under "Code review 2026-10-24". Three P0 defects block release.
Fix order is deliberate; each batch is one commit and starts from a failing
test that reproduces the defect.

- [x] P0-3 + P1-11, settings window: deleting the selected profile panics
      mid-frame, and deleting a middle profile shifts Ports/Targets onto the
      next account. The frame re-reads the selection after the profile buttons,
      and the positional buffers are spliced on delete and carried into a copy
- [x] P0-2, proxy lifetime: close the old listeners before binding; a failed
      session must not leave ports held so manual reconnect can never work.
      `restartProxy` releases first, terminal failures (`PhaseFailed` with
      `Attention`) release the listeners, and a server that finishes binding
      after a release closes itself
- [x] P0-1, UI deadlock: never join `Manager.Logout` from the UI thread while
      the login goroutine may still need the main loop. `disconnect` releases
      prompts, the attempt and the listeners synchronously, then joins the
      session on another goroutine; a shared pending join keeps a second 断开
      from racing it; `connect` waits for the join before a new login;
      `disconnectNow` joins synchronously only in `--no-ui` and after
      `App.Run`
- [x] P1-4/P1-5, tunnel DNS and stack: resolve `auto` through
      `client.DNSServer()`/`DNSServers()`, choose gVisor when an IP exists.
      The packet stack carries UDP, so a TCP-only gateway still falls back to
      the TCP tunnel and remote DNS is off when the gateway names no server
- [x] P1-6/P1-7, reconnect: hand the proxy a live resolver, allow `Dial` while
      the client is restoring a session. `Manager.Resolve` reads the resolver
      per request; `Dial` refuses only when the ladder has torn the tunnel down

Open work, in this order (each still one commit, each starting from a failing
test except where noted):

- [x] P1-8/P1-9, client data: derive the path from the loaded settings file, so
      `--config` moves it; deleting or renaming a profile moves or removes
      `client_data-*.json` with it. The settings draft records the change and
      the app applies it only after a successful save; a custom
      `client_data_path` is never touched
- [x] P1-10, config race: `Clone()` before mutating in `storeAuthChoice`,
      `storeTOTPSecret` and every prompt handler, and leave the in-memory
      config untouched when `SaveFile` fails. `mutableConfig` is the one copy
      the handlers edit; `persistFlag` and `selectProfile` only write scalar
      fields on their value copy, so they need no clone
- [x] P1-12/P1-16, settings freshness: a frame drops a discovered method list
      whose gateway no longer matches the draft, and the picker refuses it
      before storing; the port is canonicalised so a padded value keeps its
      list. The hide epoch is claimed on the main thread in the same step as
      the hide, and `openSettings` bumps it there, so a reopen cannot be
      swallowed by a queued hide
- [x] P1-13/P1-14, logging and CONNECT: the whole value of a
      credential-bearing header is masked (cookie families, bearer/basic values
      and `map[…]` wrappers included), and pipelined CONNECT bytes are drained
      from the hijacked buffer instead of read until EOF
- [x] P1-15, log rotation: a line whose file is nil retries `openLocked`, so a
      failed rotation no longer silences the file for the rest of the process.
      The same path hid a panic: the 256-byte truncation floor sliced past a
      shorter line whenever `MaxBytes` was under 512; the floor now applies
      only when the line is longer
- [x] P1-17, restore confirmation on the packet stack: the fork's gVisor
      `DialTCP` reports a successful packet-stack connection through the same
      optional observer the TCP tunnel uses (fork `5e185ef`,
      `v1.3.2-0.20261024055516-5e185ef9fdce`). easyconnect is unaffected, and
      the TCP tunnel still reports its own handshake. Live acceptance (a real
      restore leaving `正在恢复会话`) stays a manual smoke item because this
      account's gateway was not exercised
- [ ] P2 batch, itemised from `findings.md`:
  - [x] proxy: track hijacked CONNECT/SOCKS5 connections and close them in
        `Server.Close`; forward no hop-by-hop request headers
        (`Proxy-Authorization` included); half-close both directions; flush
        streamed responses; cache the `forward` Transport. Every new test was
        run against the pre-fix tree first (six failures: four behavioural, two
        deadline timeouts)
  - [x] proxy/config input: reject or warn on a non-loopback bind, syntax-check
        binds in the settings window, allowlist `openResource` schemes
        (`http`/`https` only). The bind rule is one exported
        `config.ValidateBind`, run by `Config.Validate` before a login and by
        the settings save; the real binary refuses an exposed bind and exits 1
  - [x] TOTP: clear the stored secret with 保存密码 and offer a removal action;
        the prompt refuses to store one for a profile that stores no password,
        and 保存密码和 TOTP 密钥 is the toggle's label
  - [x] persistence: atomic temp+rename `writeClientData`, `Sync` config and
        resource caches, reject unknown TOML keys, make `profileSlug`
        collision-safe on case-insensitive filesystems (an uppercase letter now
        takes a hash suffix, so `Work` and `work` never share a file)
  - [x] session: no `PhaseFailed: context canceled` after 断开; keep
        `Resources`/`ClientIP`/`KeepAlive` during a ladder attempt; publish
        `set`/`setTransition`/`recordKeepAlive` under the lock snapshot;
        refuse `Login` during a client-owned restore without overwriting the
        client; verify the proxy-epoch guard covers a server bound after
        `disconnect`; a cancelled login must not paint an English red error.
        A real `--no-ui` run cancelled against a stalling gateway logged no
        `session failed` line; `installProxy` is the extracted guard with a
        test that installs nothing after a release
  - [x] keep-alive: lower interval bound in the settings view (10 s; a
        hand-edited smaller value is clamped, the settings save rejects it)
  - [x] dead code and metadata: dropped `Config.SelectProfile/RenameProfile/
        ClientDataPath`, `Logger.Writer`, `cliPrompter.logger` and
        `ReconnectState.Reason`; `Attention` is a bool because only its presence
        was ever used; `--print-resources` prints the snapshot metadata;
        the login channel's certificate check is documented at its call site
- [x] Verification, automated: `go test -race ./...`, `go vet ./...`,
      `gofmt -l .`, `reuse lint` (47/47), and a real-binary lifecycle run with a
      temporary config (`goTrust ready`, expected appindicator warning, SIGTERM
      exit 0)
- [x] Verification, native window: a headless mutter plus its own Xwayland
      (`docs/agent-ui-testing.md`, "Headless session") drove the real GTK
      window without touching the desktop. 断开 during a login returned the
      window to `未连接` with `已取消` and no failure in the log; 设置 → 新建 →
      删除 removed the draft profile with no panic, the file kept its one
      profile, and reopening reloaded `默认`; 关闭 hid the settings window and
      the question mark (退出) quit cleanly. `reconnect` after expiry still
      needs the real gateway and stays manual (Phase 14)
- **Status:** complete
- **Note:** the headless recipe is now the preferred native smoke; the real
  desktop is needed only for tray/panel behaviour
- **Note:** Phase 16 is closed (the package is installed with `pacman -U`).
  P1-17 needed a fork commit and a pin bump; every other item above is
  goTrust-local.

### Phase 18: Tray interaction and the appindicator backend (2026-10-24)

- [x] 打开默认应用: the tray opens the URL the gateway returned for the first
      granted application (the same value the resource row shows) with the
      desktop's default browser; the item is disabled when the grant has no
      URL
- [x] Activating the tray icon opens the main window: mygo v0.6.4 answers the
      panel's `Activate` with `Tray.OnClick` (upstream PR #706, carried from
      #705), and the tray already registers the handler
- [x] Upstream mygo PR: the activation half landed as PR #705/#706
- [ ] Upstream glib backend: v0.6.4 still binds `libayatana-appindicator3` and
      the deprecation warning stays. A `libayatana-appindicator-glib` backend
      (GMenu/GAction, `activate_event`) keeps the legacy one as the fallback
      when a release ships it
- **Status:** complete (activation and menu); the glib backend is upstream work
- **Note:** mygo 0.6.4 defers only a destroy in frame (PR #674), not `Hide`, and
  issue #680 is still open, so the create/hide hops stay in `main.go`.
- **Note:** the `libayatana-appindicator-WARNING` line comes from the legacy
  backend. The `.so` is not a drop-in: GMenu vs GtkMenu, and no
  `app_indicator_set_icon_full`. Do not swap the package dependency alone.
  With a click listener the panel activates the icon (double click in GNOME);
  the menu opens on the secondary click.

### Phase 19: Password change and forced reset (full)

Done completely, live-tested when the gateway first forces it (the 90-day
policy). Endpoints and the signal are in `findings.md`.

- [x] Fork: typed signal for `next-step:"forced-change"`
      (`change-code`, `change message`, `policy-object`, `username`,
      `domain`, `pwdRulePrompt`), `Session.ChangePassword(ctx, old, new)` over
      `POST reset-path`,
      `Session.SkipPasswordChange(ctx)` (`skip:true`, `remainedSkipTimes`), the
      forced change over `POST forced-change`
      (form-urlencoded), `Session.PasswordStrategy(ctx)` over
      `portal/user/policy-object`, and `Session.CheckWeakPassword` over
      `portal/public/policy-probe`. `LoginOptions.PasswordChangeHandler`
      answers the demand inside the login chain (the reset form's shape:
      `oldPassword`/`newPassword`, optional `account-field`, `skip`), a nil
      handler yields `*PasswordChangeRequiredError`, and the chain continues
      after the change
- [x] Panel in the login flow: old password, new password twice, rules rendered
      from `policy-object`, the account field for `75500402`, 跳过 when
      `75500404`, `pwdRulePrompt` text; wired like the code/captcha prompts and
      released by 断开/cancel
- [x] Voluntary reset while online: `POST reset-path`
      (`{oldPassword,newPassword}`) from the settings window, policy
      check via `portal/public/policy-probe`; only the profile that owns the
      live session is offered
- [x] After success: rewrite the stored credential (config today, keyring after
      Phase 20) and continue; the portal is expected to end other terminals'
      sessions. A forced change continues the same login; the online reset keeps
      the tunnel and warns that other terminals must log in again
- [x] Tests: typed signal parsing, form/JSON bodies and encryption, skip,
      rules/violations, panel validation and cancel, stored credential updated,
      headless refusal is clear
- [x] Live test: the gateway forced the change on a new account's first login
      (same `forced-change` flow as the 90-day expiry); the change succeeded,
      the credential was rewritten, and the portal closed the pending chain
      with `75500006`, which the automatic re-login handles
- **Status:** complete
- **Note:** the reset form uses these shapes: `forced-change` is form-urlencoded
  with RSA-encrypted
  `oldPassword`/`newPassword` (no anti-replay suffix) and optional
  `account-field`/`skip=true`; `reset-path` is JSON with the same pair;
  `policy-probe` takes `{password}`; `policy-object` returns the rule object. The
  voluntary entry is 设置 → 连接 → 密码安全 → 重置密码 and its panel follows the
  重置密码 dialog (旧密码/新密码/确认密码,
  密码规则, 确定/取消, the "其他终端已在线账号需重新登录" note). The panel was
  rendered through `ui.NewTester` (light) and the forced-flow real-window smoke
  waits on a gateway that demands the change.
- **Note (live attempt):** the first package (`r47`) reached a gateway
  that demanded the change and got `75500000 (Parameter error or missing
  occurred)`. The reset form refreshes `authConfig` before encrypting and
  sends `<password>_nonce`; every request also carries
  shared-params. Fork `7f399b5` added both; a later fix
  removed the authConfig refresh (the second live attempt got `75500001`),
  because the session's own key/nonce from the password answer are what the
  portal expects.
- **Note (live completion and re-login):** the next package changed the password
  successfully and saved it, but the portal then answered the follow-up
  `auth-check` with `75500006 (account already logged in)` because the change
  leaves the account online and closes the pending chain. A fresh login with
  the new password works (the operator's manual retry in the same log), so the
  fork returns `auth.ErrReauthRequired` and the manager retries once with the
  saved credential.
- **Note (in-app reset):** the settings dialog failed before its reset request:
  `portal/public/policy-probe` also expects the RSA
  `<password>_nonce` value, and the probe sent plaintext. Fork
  `3477534` fixes the probe, keeps a renewal answer from clearing the session's
  CSRF/key/nonce, and logs the reset path; the package was rebuilt (`r51`).
- **Note (first login is the same flow):** the live demand came from a new
  account's first login (`change-code 75500401`), not from the 90-day policy,
  but the portal routes both through `next-step:"forced-change"` and the
  same reset page; only the account-rename (`75500402`) and skip (`75500404`)
  flags vary. The live evidence therefore covers the expiry case as well.

### Phase 20: Keyring credential storage (second; backend chosen at execution)

The reference branch is EZ4Connect's local-only `feat/credential-keychain`
(libsecret `secret-tool`, attributes `service`/`scope`/`key`; macOS
Keychain and Windows Credential Manager; settings file as the fallback). The
zju-connect fork has no keyring branch. Details and the goTrust choke points are
in `findings.md`.

- [ ] Choose the backend then: `go-keyring` (D-Bus Secret Service, no CGO) or
      `secret-tool` (mirrors EZ4Connect, adds `libsecret` to depends)
- [ ] Stable scope per profile (a new id, or the client-data move/erase hooks);
      the `--config` path must be part of the scope
- [ ] Read at the login snapshot; write/erase through `cfgForDisk` and the
      profile actions; erase on `store_password=false`, delete and reset
- [ ] Migrate an existing plaintext `password`/`totp_secret` into the store,
      dropping the file copy only after a successful write
- [ ] Headless contract without a session bus or prompt agent; locked-collection
      behaviour and its UI message
- [ ] Tests with a fake store: migration, fallback, locked/absent, rename and
      delete

### Phase 21: 终端管理与设备信任 (complete)

The portal's 终端管理 page is the account's own view of its endpoints: the
devices it trusts and the temporary login history. goTrust reads both lists
through the live session, so the settings window shows what the portal sees
without a second login.

- [x] read `queryDevice` twice (`status=trust` and `status=untrust`) and keep
      both lists plus `selfId`/`deviceTrusted` on the client
- [x] settings 终端管理 group: counts, both lists, 刷新,
      信任此设备/取消信任, the current device marked by `selfId`
- [x] confirm `auth/bindAuthDevice` in the window: 允许绑定 continues the
      login, 取消 aborts it, and a `--no-ui` run keeps the automatic answer
- [x] tests: the section renders the portal answer, marks the current device
      and hides its actions while offline; the bind panel reaches both answers
- [x] live acceptance: the operator read the lists against the gateway,
      toggled trust and took a login through the binding prompt

### Phase 22: Install identity and endpoint reporting

The portal lists one row per login unless the client presents a stable device
identity. goTrust keeps one identity per installation, in `[identity]`, and
seeds it into every profile before the login.

- [x] `[identity] device_id`: generated once (32 uppercase hex), persisted
      0600 beside the settings, adopted from an existing profile's stored
      device id instead of minting a second device, refused at startup when
      hand-edited into a form the portal cannot read
- [x] seeded into each profile's client data before the login, so switching
      accounts does not look like a second device
- [x] settings 高级 shows the id read-only and opens the file for hand edits
- [ ] live acceptance of the seeded id: log in, check 终端管理 keeps one row
- [x] UA and tunnel process block from the identity: the client library takes
      a per-session UA and a process block, and `[identity]` supplies both; an
      empty process block keeps the client's own
- [ ] native endpoint reporting: the host/OS/MAC fields the portal shows for a
      native registration, behind a switch that stays off while the gateway
      rejects the native session

## Key Questions

1. Can the resource model carry app names and launch URLs, or only the
   address/port rules the fork parses? **Answered in Phase 13:** the portal's
   app list carries `name`, group `name`, `accessAddress`, `openModel.model` and
   `webRelativeDomainList`; the fork parses them into `client.App` and exposes
   `Apps()`. Address rules remain the routing model.
2. What signal does the embedded client give when the session needs a
   relogin? **Answered in Phase 14:** the fork now publishes
   `online`/`restoring`/`relogin-required` through
   `SetupOptions.OnSessionState`, so goTrust mirrors the client's own renewal,
   ticket-restore and cooldown ladder instead of parsing log lines. The fork's
   `KeepAliveOptions.OnResult` does the same for probes.
3. For external SSO, is a system browser plus a pasted callback URL enough, or
   does a gateway hand back a cookie that only a webview can complete? Start
   with the browser/paste path and add a webview only on evidence.
4. Which runtime libraries does the packaged binary actually load (GTK,
   WebKitGTK, libayatana-appindicator)? Resolve with `ldd` and `namcap` in
   Phase 16, not by guessing.

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| New repository, not a fork of `zju-connect` | The protocol fixes stay usable as a library; the app rewrites the wiring anyway, and `go get` beats rebasing 46 files |
| Compile-time dependency on the fork, not a child process | Single Go binary is the whole point; process-boundary would keep the Qt/GPL split |
| AGPL-3.0-or-later for goTrust | Static linking with AGPL code makes the combination AGPL; GPLv3 §13 only permits the combination in the AGPL direction |
| Own SOCKS5/HTTP listeners instead of `service.Serve*` | Those take no `context.Context` and shut down through `internal/hook_func`, which an external module cannot import |
| No TUN, no system proxy | Removes root elevation, wintun, and three platform backends; keeps the app a plain user process |
| Settings in a dedicated window | The main window stays a status/connection page, as EZ4Connect does; the settings surface is large (connection, proxies, keep-alive, startup, advanced) |
| Config is versioned TOML with profiles | The operator prefers TOML, and `BurntSushi/toml` writes a readable, hand-editable file with `[[profiles]]`. Atomic save and 0600 stay. Proxy ports stay global so listeners do not move on a switch |
| No v1→v2 migration | v1 was never released and the only existing `config.json` on the operator's machine holds defaults; a migration path would preserve nothing. The old file is ignored, and a newer `version` is refused so a future build cannot silently drop fields |
| Multiple profiles, one live session | The operator asked for account switching; one session at a time keeps the device token and proxy listeners unambiguous. A switch disconnects first |
| Logging is a rotating local file | A tray app has no terminal; a 0600 file under the state directory plus a settings shortcut makes failures diagnosable, while stderr/journal stays for `--no-ui`. Secrets are redacted |
| REUSE is a gate | The repository is public and AGPL; SPDX headers, `LICENSES/` and `reuse lint` make the licence machine-checkable and are required by the package |
| Arch package builds from GitHub now | The operator approved the push and wants the package now; it is self-use, not AUR, so private-repo credentials during `makepkg` are acceptable |
| Auth method: auto by default, manual override | `atrust.GetAuthInfoList` is public and works pre-login; one offered method is unambiguous, several need the saved pair or a chooser, and the protocol requires an exact `authType`+`loginDomain` match |
| Login domain defaults to `local` | The verified gateway offers `local`; the field stays editable because other gateways use `Radius` or a tenant name |
| Autostart means XDG login autostart | `mygo.App.SetOpenAtLogin` is the platform path for a tray app; a systemd unit is not the desktop equivalent and is not planned here |
| Auto-connect on startup, auto-reconnect separate and off by default | "自动连接" is a startup behavior; reconnecting after expiry is a different risk and gets its own setting (EZ4Connect defaults it off too) |
| A session cannot outlive its connection; no ticket restore | Tested in `atrust-quadlet` and `atrust-dev`: the stored sid ticket only exchanges while the portal still has a live session (`75500307` while logged out), so a reconnect must log in again. Stored client data is device material, not a credential |
| Keep-alive is a real HTTP request | CONNECT-only and HTTPS targets do not renew the session (tested in atrust-keepalive and atrust-quadlet); manual targets come first, derived targets next |
| Resource list derives URLs when names are absent | The fork keeps address/port rules and app IDs, not necessarily names; showing `http://IP[:port]` is useful even without a portal label |
| Arch package builds from the local checkout first | The repository is private until publication; `gotrust-git` stays self-use and switches its source URL at release |
| dae remains last | The operator deferred transparent-proxy export; it is nice-to-have after the product phases |
| Review remediation before new features, P0 first | In the tree reviewed then the UI can freeze, manual reconnect can never rebind, and deleting a profile crashes the process; no feature can be trusted until those are gone. Each fix gets a regression test |

## Errors Encountered

| Error | Attempt | Resolution |
|-------|---------|------------|
| `go get github.com/mythologyli/zju-connect@dev` → unknown revision | 1 | The default branch is `main`; selected the fork's commit and used a pseudo-version via `replace` |
| Pseudo-version built from `cut -c1-14` produced `20261020T02` | 2 | The git timestamp needs 14 digits: `tr -dc '0-9'` before truncating |
| `ui.EditableText` rendered the password in clear | 1 | It is an inline-rename control, not an input; `ui.TextInput` is the input, and `.Password()` masks it |
| `mygo.MenuItem.Click` assigned a `func()` | 1 | The field is `func(*MenuItem, *Window)`; wrapped the callbacks |
| `ui.Form(c)` missing an argument | 1 | `Form` takes the children builder: `Form(c, func(){…})` |
| `App.Exit` left a stale single-instance socket | 1 | `Exit` skips quit events; switched to `Quit` and hung teardown on `OnQuit` |
| `phaseDetail` printed `2562047h…` for a zero `Since` | 1 | Guarded the zero timestamp before formatting the duration |
| `Manager` kept the startup config after the form changed | 1 | `Options.Config` became a function, like `Options.Auth` |

## Notes

- The verified gateway account is disposable and has no second factor beyond the
  click captcha; SMS/TOTP/SSO can be implemented but not live-tested. Never put
  the account, password, gateway address, intranet addresses or client data in
  the repository or a package.
- `internal/` packages of `zju-connect` are unreachable from here by design.
  Only use `client`, `client/atrust`, `client/authchallenge`, `dial`, `resolve`,
  `service`, `stack/gvisor`, `underlay`.
- Reference projects: `EZ4Connect` (Qt UI around the core), `zju-connect`
  (protocol), `atrust-quadlet` (supervised deployment, `--apps`,
  `--status`), `atrust-keepalive` (90 s real-HTTP probe, target discovery,
  failure surfacing). Their reusable lessons are recorded in `findings.md`.
