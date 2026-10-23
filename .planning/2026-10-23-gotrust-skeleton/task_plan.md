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

Phase 11: define the settings model and build the separate settings window
(connection, proxies, keep-alive, startup, advanced), then wire the tray
status/connect/settings items, autostart and auto-connect.

## Current Phase

Phase 11

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

### Phase 11: Config v2, profiles, settings, startup and tray

Requirements 1, 2, 3, 5, 6, 7, and the foundation for 10. A dedicated settings
window keeps the main window about status, connection and resources; EZ4Connect
does the same. Settings are persisted as versioned JSON.

- [ ] Config v2: `version`, `active_profile`, global `app`/`proxy`/`logging`
      sections and a `profiles[]` array; each profile owns server, auth, TOTP,
      keep-alive and debug settings. Proxy ports stay global so a switch does
      not move the listeners
- [ ] Migration: a v1 file becomes one profile (`默认`), preserving every value;
      missing fields take defaults and the file stays valid
- [ ] Profiles: create, rename, duplicate and delete; one active session at a
      time; switching disconnects first, then optionally auto-connects; the
      active profile is visible in the window and the tray submenu
- [ ] Logging: line log to `$XDG_STATE_HOME/goTrust/goTrust.log` (0600, 5 MiB
      × 3 rotation), stderr in `--no-ui`, `verbose` level from settings, and
      redaction of passwords, cookies, tokens and captcha data; bridge the
      upstream global `log` to the same writer; a settings button opens the
      log directory
- [ ] Defaults: login domain `local`, keep-alive 60 s, SOCKS5 `127.0.0.1:1080`,
      HTTP `127.0.0.1:1081`
- [ ] Settings window: connection (gateway, port, username, login domain,
      password/store), proxies (bind addresses and enable toggles), keep-alive
      (enabled, interval, manual targets), startup (autostart, auto-connect,
      start hidden, auto-reconnect), advanced (verbose, log directory, client
      data path)
- [ ] Apply rules: proxy binds and keep-alive take effect on the next connect;
      credentials and domain on the next login; cosmetic settings immediately
- [ ] Tray: disabled status line (`已连接 203.0.113.4`, `连接中`, `失败: …`),
      connect/disconnect toggle, profile submenu, show window, settings, quit;
      tooltip mirrors the phase and the last keep-alive result
- [ ] Autostart at user login through `mygo.App.SetOpenAtLogin` (XDG autostart
      on Linux) and `WasOpenedAtLogin` to start hidden; a systemd unit is not
      the desktop equivalent and is not planned here
- [ ] Auto-connect on `WhenReady` when enabled and a stored password or client
      data exists; reveal the window for a captcha/second factor and fail
      visibly when credentials are missing
- [ ] Tests: config v1→v2 migration and round-trip, profile switch/delete,
      log rotation/redaction, settings view and callbacks, tray menu per phase,
      autostart entry in a temp `XDG_CONFIG_HOME`, auto-connect decision
- **Status:** pending

### Phase 12: Auth methods and second factors

Requirements 3 and 4. The fork exposes `atrust.GetAuthInfoList` as a public
pre-login call, so method discovery needs no protocol change. aTrust matches
`authType` and `loginDomain` exactly.

- [ ] `session.DiscoverAuthMethods(ctx, cfg)` wrapping `atrust.GetAuthInfoList`
      (server, port, underlay settings); return name, type, domain, URL
- [ ] Settings: "获取认证方式" button and a list/dropdown; store the chosen
      `auth_type` and `login_domain`; keep `local` as the default domain
- [ ] Resolution: auto mode uses the only offered method, else the saved one,
      else asks the user to choose; a stored method the gateway no longer offers
      fails with the list instead of a silent fallback
- [ ] Code challenge dialog for SMS/TOTP/RADIUS: message, code input, and
      "跳过以后的二次认证" when the challenge allows it; optional TOTP secret
      (store only on opt-in) generates the code upstream
- [ ] External SSO (CAS/OAuth2): open `LoginURL` in the system browser and take
      the callback URL, validating path and ticket/code; a built-in webview is a
      later option, not a dependency
- [ ] Cancel and login-context timeout release the handler, as the click
      captcha already does
- [ ] Tests: auth-info parsing/resolution, prompter dialogs and cancel, SSO
      callback validation; mark SMS/TOTP/SSO **untested live** (the verified
      account has no second factor) in the README and progress log
- **Status:** pending

### Phase 13: Granted resources

Requirement 8, informed by `atrust-quadlet --apps`: the user must see what the
account may reach and which URL to open in a browser.

- [ ] Retain IP, domain and DNS resources plus app IDs in `session.State`
      instead of discarding them after setup
- [ ] Check whether the resource model can carry app names and launch URLs;
      extend the fork's `resourceApp` if the fields are available, otherwise
      derive display entries from addresses/ports
- [ ] Display model: domain `name[:port]`, IP/range with port range and protocol,
      DNS name→IP; suggest `http://`/`https://` for a single IP with port 80/443
- [ ] Main window: scrollable resource list with open-in-browser and copy
      actions, plus gateway, tunnel IP and proxy addresses
- [ ] Cache the last snapshot in the config directory at 0600 and print it in
      `--no-ui` runs; add `--print-resources` for headless use (Quadlet's
      `--apps` equivalent)
- [ ] Tests: derivation/formatting, URL suggestion, empty session, cache
      round-trip; no real intranet address in the repository
- **Status:** pending

### Phase 14: Reliability

Inspired by EZ4Connect's reconnect ladder and the keepalive family's probe
discipline; not a new user-facing feature list, but the session must not lie.

- [ ] Auto-reconnect option (default off): attempts after session loss with
      1/2/4/8/16 s backoff, at most 5; state and tray show
      `重连中 (n/5, next in Xs)`
- [ ] Stop the ladder when a captcha, one-time code or SSO callback is needed,
      and bring the window forward instead of retrying blindly
- [ ] Expose `last target`, `last result`, `last at` in `session.State` by
      wrapping `service.KeepAlive` (a fork result hook) rather than duplicating
      its logic; manual targets first, derived targets next; reject or warn
      about `https://` targets because only a real HTTP request counts; keep
      the 60 s default
- [ ] Surface session expiry and reauthentication: ticket restore first,
      automatic login only when credentials are stored, otherwise a clear
      tray/window state
- [ ] Tests: backoff schedule and cap, target ordering, keep-alive result
      transitions, reconnect stops on a challenge
- **Status:** pending

### Phase 15: UI polish

Requirement 10, after the functional windows exist.

- [ ] Layout and typography pass over main, settings, captcha, code and SSO
      views; consistent spacing, section headings and button hierarchy
- [ ] Status colours and detail lines for idle/connecting/online/failed/
      reconnecting; a visible busy state and disabled controls while work runs
- [ ] Window sizing and scrolling: main window with the resource list; a
      settings window sized for its groups, scrollable when the screen is
      short
- [ ] Render every state with `ui.NewTester` and inspect the images; launch the
      real window on Wayland for a final smoke, including CJK and the tray menu
- [ ] Accessibility labels and a sane tab order for every input and button
- **Status:** pending

### Phase 16: REUSE compliance and Arch package `gotrust-git`

Requirements 4 and 9. REUSE first, because the package ships the license and
the repository is public. Then a self-use VCS package; it does not go to AUR.

- [x] `LICENSES/AGPL-3.0-or-later.txt`, `REUSE.toml`, and SPDX headers in every
      Go file; annotate docs, `go.mod`, `go.sum`, `.gitignore` and `.planning/`
- [x] `reuse lint` clean (24/24 files); documented in README
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
- [x] `makepkg -f` built `gotrust-git r7`; `namcap` only reports the
      intentional dlopen/external-linker warnings; the extracted binary starts
      and exits 0 on SIGTERM
- [ ] Install with `pacman -U` after the operator supplies the sudo password;
      the package is built and tested from its extracted tree
- **Status:** in_progress

## Key Questions

1. Can the resource model carry app names and launch URLs, or only the
   address/port rules the fork parses? (Phase 13: extend the parser if the
   fields exist, otherwise derive display URLs)
2. What signal does the embedded client give when the session needs a relogin?
   The upstream logs `aTrust session requires reauthentication`; if no callback
   exists, Phase 14 needs a small fork hook or dial-failure detection.
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
| Config is versioned JSON with profiles | The existing file is JSON and atomic/0600; adding `version` and `profiles[]` avoids a second format while allowing multiple accounts. Proxy ports stay global so listeners do not move on a switch |
| Multiple profiles, one live session | The operator asked for account switching; one session at a time keeps the device token and proxy listeners unambiguous. A switch disconnects first |
| Logging is a rotating local file | A tray app has no terminal; a 0600 file under the state directory plus a settings shortcut makes failures diagnosable, while stderr/journal stays for `--no-ui`. Secrets are redacted |
| REUSE is a gate | The repository is public and AGPL; SPDX headers, `LICENSES/` and `reuse lint` make the licence machine-checkable and are required by the package |
| Arch package builds from GitHub now | The operator approved the push and wants the package now; it is self-use, not AUR, so private-repo credentials during `makepkg` are acceptable |
| Auth method: auto by default, manual override | `atrust.GetAuthInfoList` is public and works pre-login; one offered method is unambiguous, several need the saved pair or a chooser, and the protocol requires an exact `authType`+`loginDomain` match |
| Login domain defaults to `local` | The verified gateway offers `local`; the field stays editable because other gateways use `Radius` or a tenant name |
| Autostart means XDG login autostart | `mygo.App.SetOpenAtLogin` is the platform path for a tray app; a systemd unit is not the desktop equivalent and is not planned here |
| Auto-connect on startup, auto-reconnect separate and off by default | "自动连接" is a startup behavior; reconnecting after expiry is a different risk and gets its own setting (EZ4Connect defaults it off too) |
| Keep-alive is a real HTTP request | CONNECT-only and HTTPS targets do not renew the session (tested in atrust-keepalive and atrust-quadlet); manual targets come first, derived targets next |
| Resource list derives URLs when names are absent | The fork keeps address/port rules and app IDs, not necessarily names; showing `http://IP[:port]` is useful even without a portal label |
| Arch package builds from the local checkout first | The repository is private until publication; `gotrust-git` stays self-use and switches its source URL at release |
| dae remains last | The operator deferred transparent-proxy export; it is nice-to-have after the product phases |

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
