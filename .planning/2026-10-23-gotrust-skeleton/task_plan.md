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

Phase 17: every P0 and P1 item is fixed with regression tests, including P1-17
in the fork and its pin bump. The queue now holds the itemised P2 batch and
then the verification pass (race suite, real UI smoke, `reuse lint`). Phase 16
is closed: the package is installed from `pacman -U`. Phase 8 (dae export)
stays deferred.

## Current Phase

Phase 17 (review remediation; every P0 and P1 item is done, the P2 batch is
running from the proxy item). Phase 18's tray menu item is done; its
double-click/upstream
item is blocked on mygo.

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
  - [ ] TOTP: clear the stored secret with 保存密码 and offer a removal action
  - [ ] persistence: atomic temp+rename `writeClientData`, `Sync` config and
        resource caches, reject unknown TOML keys, make `profileSlug`
        collision-safe on case-insensitive filesystems
  - [ ] session: no `PhaseFailed: context canceled` after 断开; keep
        `Resources`/`ClientIP`/`KeepAlive` during a ladder attempt; publish
        `set`/`setTransition`/`recordKeepAlive` under the lock snapshot;
        refuse `Login` during a client-owned restore without overwriting the
        client; verify the proxy-epoch guard covers a server bound after
        `disconnect`; a cancelled login must not paint an English red error
  - [ ] keep-alive: lower interval bound in the settings view
  - [ ] dead code and metadata: drop `Config.SelectProfile/RenameProfile/
        ClientDataPath`, `Logger.Writer/Verbose`, `cliPrompter.logger`,
        `ReconnectState.Reason`, redundant `Attention` (whatever the fixes
        above leave unused); `--print-resources` prints the snapshot metadata;
        document or remove the inherited `InsecureSkipVerify`
- [ ] Verification: `go test -race ./...`, real UI smoke (断开 during login,
      设置 delete, session expiry then reconnect), `reuse lint`
- **Status:** pending
- **Note:** Phase 16 is closed (the package is installed with `pacman -U`).
  P1-17 needed a fork commit and a pin bump; every other item above is
  goTrust-local.

### Phase 18: Tray interaction and the appindicator backend (2026-10-24)

- [x] 打开默认应用: the tray opens the URL the gateway returned for the first
      granted application (the same value the resource row shows) with the
      desktop's default browser; the item is disabled when the grant has no
      URL
- [ ] Double-click opens the main window: blocked by mygo v0.6.3, whose Linux
      backend never reports clicks and lets the panel take the primary click
      for the menu; needs an upstream activation backend, not a fake
- [ ] Upstream mygo issue/PR: activation callbacks plus a
      `libayatana-appindicator-glib` backend (GMenu/GAction, `activate_event`),
      keeping the GTK3 backend as the fallback; draft is in `findings.md`
- [ ] After mygo ships it: bump the pin, add the `-glib` package dependency,
      re-run the native tray smoke; keep the legacy dependency for the
      fallback
- **Status:** in_progress
- **Note:** the `libayatana-appindicator-WARNING` line comes from the legacy
  backend. The `.so` is not a drop-in: GMenu vs GtkMenu, and no
  `app_indicator_set_icon_full`. Do not swap the package dependency alone.

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
