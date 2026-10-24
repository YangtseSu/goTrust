# Findings & Decisions

## Requirements

- New project at `~/projects/goTrust`, independent of any local checkout of a related project (no shared git history, no shared files).
- Skeleton only: must build and run, not yet a full product.
- aTrust only; no EasyConnect code.
- No TUN mode and no system-proxy manipulation (from the preceding design discussion).
- Managed by this planning skill.

## Research Findings

### License chain (verified from the checkouts)

- `zju-connect/LICENSE` is AGPL-3.0 with the FSF "or any later version" appendix → the grant is AGPL-3.0-or-later.
- No per-file license headers: `grep -rl 'Affero\|AGPL' --include=*.go` = 0 of 141 files, so the repository LICENSE governs everything.
- GPLv3 §13 permits combining a GPLv3 work with an AGPLv3 work, and the AGPL network clause then applies to the combination. The reverse (AGPL into a GPL-only project) is not permitted, and AGPL cannot be relicensed as GPL.
- Consequence: `goTrust` imports `zju-connect` packages and is therefore AGPL-3.0-or-later.

### Embeddable surface of the fork (verified by compiling an external module)

`/tmp/embed-spike`, module `embedspike`, `replace github.com/mythologyli/zju-connect => github.com/YangtseSu/zju-connect v1.3.1-0.20261020024545-4e07ac667f87`:

```go
d, _    := underlay.New(underlay.Options{AutoDetect: false})
c       := atrust.NewClient(atrust.ClientOptions{Session: ..., UnderlayDialer: d})
data, _ := c.SetupContext(ctx, atrust.SetupOptions{
              ServerAddress: ..., ServerPort: ...,
              ChallengeHandler: authchallenge.Handler,
              SaveClientData:   func([]byte) error,
           })
st, _   := gvisor.NewStack(c)                       // needs a live session
res     := resolve.NewResolver(st, "auto", "auto", 3600, domRes, dnsRes, true)
dialer  := dial.NewDialer(st, res, ipRes, false, "")
```

Observed: `gvisor.NewStack => false IP not available` without a live session (expected), `dial.NewDialer => true`, `service.ServeSocks5/ServeHTTP` addressable. No `internal/` package was needed.

### Shutdown gap

`service.ServeSocks5`, `ServeHTTP`, `ServeDNS`, `ServeShadowsocks` take no `context.Context`; they register cleanup on `internal/hook_func.RegisterTerminalFunc`, and `ExecTerminalFunc` lives in `internal/`, so an external module can neither drive nor observe their shutdown. Hence own listeners.

### Prompt injection point

`authchallenge.Handler` is a four-method interface (`HandleCodeChallenge`, `HandleTextCaptcha`, `HandleClickCaptcha`, `HandleExternalLogin`) and travels through `atrust.SetupOptions.ChallengeHandler`. That is the seam for GUI dialogs and the SSO webview.

### mygo facts used by the skeleton (v0.6.3, measured)

- Tray, `Secrets`, `SetOpenAtLogin`, `RequestSingleInstanceLock`, `ui.WebView`, `ui.NewTester` all work on Linux/Wayland.
- CJK renders correctly through `ui.NewTester` (`texts: ["深信服 aTrust 会话已建立" ...]`).
- `TrayOptions` field is `ToolTip` (not `Tooltip`).
- `WebView` navigation events hang off `vw.Page().OnDidNavigate`, not the view.
- `ui.NewTester` renders frames in memory with no window: usable for UI tests.
- Cross-compiles with `CGO_ENABLED=0` to `windows/amd64`, `darwin/arm64`, `linux/arm64`.

### Verified in the skeleton (built and run on this machine)

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l` | clean |
| `go test ./...` (config, proxy, session, ui) | pass |
| SOCKS5 end to end: greeting, CONNECT, byte echo | pass |
| HTTP CONNECT tunnel, byte echo | pass |
| HTTP absolute-URI forwarding through the proxy | pass |
| Direct hit on the proxy port answers 400 with an explanation | pass |
| A failed second bind closes the first listener | pass |
| Settings round-trip, 0600 mode, partial file, bad interval, validation | pass |
| UI renders idle/connecting/online/failed; buttons wired per phase | pass |
| Password field is masked (`ui.TextInput(...).Password()`) | pass |
| Binary launches on Wayland, tray created, window shown | pass |
| SIGTERM quits cleanly and removes the single-instance socket | pass |
| Cross-compile `windows/amd64`, `darwin/arm64`, `linux/arm64` with `CGO_ENABLED=0` | pass |
| Headless run against an unreachable gateway fails with the real reason and binds no port | pass |

Not verified: a live login, a proxy request through a real tunnel, and the keep-alive against a real gateway. The account on this machine belongs to the operator's production service, so the skeleton must not consume a login attempt.

### Transparent proxy: dae, as a daemon, not a library (researched 2026-10-23)

Question: implement eBPF transparent proxying by integrating
[daeuniverse/dae](https://github.com/daeuniverse/dae) or by running it?

| | dae | goTrust |
|---|---|---|
| Licence | AGPL-3.0 | AGPL-3.0 — same, so licensing is not the obstacle |
| Stars / release | 6.3k, v2.1.1, last push 2026-09-30 | — |
| Platform | Linux only; the official macOS path is a Lima VM; nothing for Windows | linux/darwin/windows |
| Kernel | ≥ 5.17 (`bpf_loop`), BTF, cgroup2, tc hooks, `CONFIG_NET_CLS_BPF` | none |
| Privilege | root, `CAP_BPF` + `CAP_NET_ADMIN` | no elevation |
| Shape | `cmd/run.go` daemon; `control/` is 200+ internal files plus BPF C and generated objects; no stable library API | 4 packages, 2.4k lines, own `main` |

Decision: **run dae as a separate systemd daemon; goTrust stays the node.**

Why not integrate:

1. Platform. eBPF at the tc hook has no Windows or macOS equivalent; linking dae would make the binary Linux-only or force build tags around the whole data path.
2. Privilege. Attaching BPF programs needs root. Integrating puts root inside the GUI process, which is exactly what the "no elevation" decision removed, and this time it cannot be removed.
3. Lifecycle. BPF maps, cgroup attachment and tc qdiscs need cleanup when the process dies. A GUI crash would leave hooks attached; a supervised daemon is the right owner.
4. API surface. `control/` has no embedding contract; `cmd/run.go` is the only assembly point. Embedding means forking a fast-moving AGPL project.
5. Duplication. Routing rules, DNS interception, SNI sniffing, node groups, health checks and full-cone NAT all already exist in dae.

Composition (verified against dae's `example.dae`):

```dae
node { atrust_node: 'socks5://127.0.0.1:1080' }

group {
    atrust {
        filter: name(atrust_node)
        policy: fixed(0)
    }
}

routing {
    pname(goTrust) -> must_direct
    dip(10.0.0.0/8) -> atrust
    dip(172.16.0.0/12) -> atrust
    dip(geoip:private) -> direct
    fallback: direct
}
```

Three operational traps, all sourced from dae's own examples:

1. **Loop.** dae captures local process traffic, including goTrust's TLS to the gateway. Without `pname(goTrust) -> must_direct`, that traffic is routed back into goTrust. dae's README does the same for `NetworkManager` and its naiveproxy guide does it for `pname(naiveproxy)`.
2. **Rule order.** The example presets `dip(geoip:private) -> direct`. Intranet destinations are private addresses, so the intranet prefixes must be matched before that preset or they never reach the node.
3. **DNS.** dae derives domain→IP from DNS requests that pass through it. A gateway with split DNS needs the intranet zone routed in `dns { upstream { … } }`, or intranet names never resolve.

Machine readiness on this host: `dae` installed, kernel 7.3 (well above the 5.17 floor), `cgroup2fs`, BPF JIT enabled.

### SOCKS5 resolver bug found while researching the dae outbound (fixed)

`things-go/go-socks5` resolves the FQDN in `handleRequest` before calling the dialer, using `sf.resolver`, which defaults to the system resolver. goTrust did not pass one, so a SOCKS5 client's hostname was resolved locally: intranet names failed outright and public names would have connected outside the tunnel.

`resolve.Resolver.Resolve` already matches `socks5.NameResolver`, so the fix is to inject it. Verified by a test that asserts the injected resolver receives the hostname.

## Technical Decisions

| Decision | Rationale |
|----------|-----------|
| Module path `github.com/YangtseSu/goTrust` | Matches the account's other repositories; no path collision with upstream |
| `replace` pinned to a fork pseudo-version | Upstream `main` lacks the session/keep-alive fixes; the fork's `dev` is CI-green |
| Proxy takes `DialContextFunc`, not a `*dial.Dialer` | Testable with a fake dialer; avoids depending on a concrete type |
| Challenge handling behind a `Prompter` interface | UI supplies codes; headless mode can refuse with a clear error |
| Config in JSON under `os.UserConfigDir()/goTrust` | No cgo, no third-party config parser in the skeleton |
| `Options.Config` and `Options.Auth` are functions | The user edits the form after the manager exists; a login must use what is on screen |
| `mygo.App.Quit`, not `Exit` | `Exit` skips quit events, so the tray, the listeners and the single-instance lock outlive the process |
| Transparent proxy delegated to `dae`, not embedded | eBPF is Linux-only and needs root; a separate daemon keeps goTrust cross-platform and unprivileged |
| `proxy.Options.Resolver` injected from the session | The library would otherwise resolve SOCKS5 hostnames with the system resolver, outside the tunnel |
| `ui.TextInput`, not `ui.EditableText` | `EditableText` is an inline-rename control that paints its value; only `TextInput` honours `Password()` |
| Listeners own their `context` lifecycle | Removes the `hook_func` dependency that blocks external shutdown |

## Issues Encountered

| Issue | Resolution |
|-------|------------|
| `go get …@dev` fails: the default branch is `main` | Pin a pseudo-version through `replace` |
| Pseudo-version timestamp malformed (`20261020T02`) | Use the full 14 digits from the commit date |
| `service.Serve*` not shutdown-controllable | Write the listeners in `proxy` |
| A session-expiry callback was only a log line | Fork `1d0673b`: `SetupOptions.OnSessionState` publishes every real state change |
| Keep-alive results were parsed out of log text | Fork `1d0673b`: `KeepAliveOptions.OnResult` reports each probe and the DNS fallback |
| A stored sid ticket cannot revive a session after its connection ended | Tested in `atrust-quadlet` and `atrust-dev`: the exchange needs a live session (`75500307` while logged out). Do not treat client data as a login; a reconnect is a fresh login. The attempt to enable it from stored client data was removed from the fork history (force-pushed rewrite, `1d0673b`) |

## Resources

- Fork: `https://github.com/YangtseSu/zju-connect`, branch `dev`, commit `4e07ac667f8751ac229fbef72491eca47cbd0d16`
- Embeddable packages: `client`, `client/atrust`, `client/authchallenge`, `dial`, `resolve`, `stack/gvisor`, `underlay`, `service`
- mygo docs: `https://mygo.egoist.dev/`, `docs/` in `egoist/mygo`
- SOCKS5 library already used upstream: `github.com/things-go/go-socks5 v0.1.1`
- dae: `https://github.com/daeuniverse/dae`, config reference `example.dae`, `docs/en/configuration/routing.md`, `docs/en/proxy-protocols.md`
- dae prototype path: `node { 'socks5://host:port' }` plus a `group` with `filter: name(...)`

## Visual/Browser Findings

- Headless render of a CJK view (`ui.NewTester`) produced correct glyphs at 22px and body size; no tofu, no baseline drift.
- The goTrust window renders correctly at 520x560 with four `TextInput` fields, a masked password, four buttons, a divider and the listener list. `ui.NewTester` needs one extra `Frame()` before `Image()` for values typed into inputs to appear.

### Product-phase research (2026-10-24)

The operator's next scope: autostart, auto-connect, SMS/TOTP/SSO, auth-method
choice, defaults, settings, tray, resource display, Arch package, UI polish.
dae is deferred last.

#### EZ4Connect (Qt UI around the core)

- Auth-method discovery: a settings/guide button runs the core as
  `-protocol atrust -server … -port … -auth-info` and parses the JSON list into
  a picker; the chosen `authType`/`loginDomain` are stored
  (`presentation/dialogs/authinfowindow/authinfowindow.cpp:105-113`,
  `configurationguidedialog.cpp:608-621`). Exact-match failure is surfaced as
  "认证方式/登录域不可用" (`connectionuicontroller.cpp:499`).
- Settings keys: `ZJUConnect/AuthType` (default `psw`),
  `ZJUConnect/LoginDomain`, SMS phone/URL, TOTP secret, HTTP port, keep-alive
  URL/mode, auto start/silent start/connect after start, auto reconnect and its
  initial delay (`presentation/dialogs/settingwindow/settingwindow.ui`).
- Settings live in a separate window; a configuration guide covers first run.
- Reconnect: `Common/AutoReconnect` (default false) and
  `Common/ReconnectTime` (default 1 s); a 1/2/4/8/16 s ladder, max 5, and a
  `Reconnecting` state with attempt/delay text (`connectionsession.cpp:8-53`,
  `connectionsessionstate.*`, `mainwindow.cpp:651-670`).
- Tray: connect, profile submenu, show, quit; tooltip carries the session and
  last keep-alive state (`mainwindow.cpp:697-739`, `622-648`).
- One-time codes: SMS dialog with "跳过以后的二次认证" (prefix `$` to the
  core), RADIUS dialog with the same checkbox, TOTP input
  (`authdialogcoordinator.cpp:230-285`). SSO uses a Qt WebEngine window that
  intercepts the redirect to the gateway callback and rejects it
  (`ssologinwebview.cpp:50-80`).
- Resources: no per-app list; the main window only says "连接后即可访问网络
  资源". Quadlet's `--apps` is the better model.

#### zju-connect (protocol)

- `atrust.GetAuthInfoList(server, port, bindInterface, autoDetect, localDNS,
  tlsKeyLog)` is a public pre-login call; it builds its own underlay/session
  (`client/atrust/client.go:263-288`). goTrust can use it without a fork patch.
- Auth matching is exact on `AuthType` + `LoginDomain`
  (`client/atrust/auth/auth.go:438-448`); `NewLoginMethod` supports
  `auth/psw`, `auth/cas`, `auth/httpsOauth2`, `auth/smsCheckCode`
  (`login_method.go:16-56`).
- Code challenges: `CodeSMS`, `CodeTOTP`, `CodeRadius`; TOTP can be generated
  from `LoginOptions.TOTPSecret` instead of prompting
  (`client/atrust/auth/token.go:22-33`); SMS/RADIUS prompt with
  `CanSkipSecondaryAuth` (`request.go:553-566`, `custom_sms.go:15-29`).
- External login: CAS and OAuth2 call `HandleExternalLogin` with the login URL
  and validate the pasted callback path and ticket/code
  (`client/atrust/auth/cas.go:55-90`, `https_oauth2.go:60-90`).
- Resources: `IPResource{IPMin,IPMax,PortMin,PortMax,Protocol,AppID,NodeGroupID,
  EnableTCPPrefL3}`, `DomainResources` by name, `DNSResource` name→IP
  (`client/client.go:24-45`). The parser keeps no app name or launch URL
  (`client/atrust/parse.go:26-100`); raw JSON may have more, to be verified.
- Keep-alive: `DeriveKeepAliveTargets(ipResources)` produces an HTTP URL from a
  granted app; named targets are merged first (`service/keep_alive.go`), and
  the remote-DNS fallback explicitly does not count as application traffic.
- Session data persists cookies/sign key/SID ticket; a restart can restore
  without a login (`client/atrust/auth/auth.go:58-72`). The client logs
  `aTrust session requires reauthentication` and refreshes the SID itself.

#### atrust-quadlet (supervised deployment)

- Headless supervisor with podman/Quadlet; state machine logs in, probes the
  data plane and recovers the tunnel without a human when it can
  (`README.md`, `atrustd/__main__.py`).
- Publishes the account's applications after login: name, launch URL, launch
  method and server address, as `apps.json` and `atrustd --apps`
  (`atrustd/__main__.py:234-249`, `atrustd/apps.py`); this is exactly the
  "what can I open" surface goTrust needs.
- Probe discipline: a real HTTP request through the tunnel, a named target or
  route discovery, and a 90 s watch interval; `CONNECT`-only and HTTPS do not
  renew the session (README "What you get", `probe.py`).
- TOTP secret is supported for printing a code (`atrustd --totp`) but is never
  submitted automatically; a human completes VNC.
- Diagnostics: `--status`, `--once`, `--login-probe`, `--apps`, `--bundle`;
  state/evidence files are 0600 and contain no credentials by construction.

#### atrust-keepalive (host deployment)

- One real HTTP request per 90 s through the tunnel interface, bound with
  `SO_BINDTODEVICE` so a withdrawn route cannot leak the probe
  (`atrust-keepalive-linux/atrust-keepalive.sh:6-30`).
- Status comes from the client's loopback API, cross-checked by the probe;
  session/tunnel events wake the next cycle early; target is discovered from
  the tunnel's host routes or configured explicitly (`README.md`).
- Failure is a first-class state: OFFLINE/ONLINE transitions, consecutive
  failures, a warning when no real traffic is sent, and a ticket-based tunnel
  rebuild before asking a human.
- systemd `--user` for the session bus and notifications; journal logging.

#### mygo (v0.6.3)

- `App.SetOpenAtLogin` / `OpenAtLogin` / `WasOpenedAtLogin` are the platform
  autostart path; on Linux it writes
  `$XDG_CONFIG_HOME/autostart/<id>.desktop` with `--mygo-opened-at-login`
  (`login.go:36-78`, `internal/linux/login.go:20-50`).
- Tray `MenuItem` supports `Disabled`, `Checked`, `Submenu`, `ToolTip`; enough
  for a status line, a connect/disconnect toggle, settings and quit
  (`menu.go:89-110`).
- Multiple windows are ordinary (`NewWindow`); the settings window can reuse
  the same view helpers and `Invalidate` model.

#### Arch packaging (`~/projects/arch-pkgs`)

- Sibling packages are VCS packages with a comment header, SPDX, `_branch`,
  `source=(git+…)`, `prepare()` running `go mod download`, `pkgver()` from
  `git describe`, `build()` with `-trimpath`, `check()` running the suite, and
  `package()` installing binary, desktop entry, icon and license
  (`zju-connect-git/PKGBUILD`, `ez4connect-git/PKGBUILD`).
- A GUI package installs a `.desktop` and an icon; runtime deps are resolved by
  `ldd`/`namcap`, not guessed (`ez4connect-git/PKGBUILD`).
- `gotrust-git` can build from a local `git+file://` source until the repository
  is public; the source URL is the only release-time change.

#### Auth methods and second factors (verified 2026-10-24)

- `atrust.GetAuthInfoList(serverAddress, serverPort, bindInterface,
  autoDetectInterface, localDNSServer, debugTLSLogFile)` is public but builds
  its own underlay dialer and takes no `context.Context`, so a cancelled login
  cannot stop it. `auth.NewSessionContext(ctx, host, keyLogWriter,
  dialContext...)` + `Session.GetAuthInfoList()` is the exported path that can:
  the request context is linked to the session lifetime.
- The host format matters: the client omits the port at 443
  (`client/atrust/client.go:492`), so discovery has to use the same form or the
  session cookie and certificate are checked against a different name.
- An empty `authServerInfoList` is a valid `authConfig` answer (gateway thinks
  the client is already logged in); it must not fail the login, because the
  client then restores its stored session.
- aTrust matches `authType` + `loginDomain` as an exact pair. Offering
  `auth/psw` at `local` and `Radius` means two methods, not one.
- `auth.NewLoginMethod` also accepts `auth/smsCheckCode`, but its
  `LoginMethodOptions.Phone` has no source in this app; a primary SMS method is
  therefore refused with a message instead of sending an empty phone.
- `SetupOptions.TOTPSecret` lets the client generate TOTP codes without the
  handler (`auth/token.go:22`); storing a secret changes the next login, while
  the current challenge still needs a code (or can be answered locally with
  `totp.GenerateCode`).
- CAS/OAuth2 callbacks are validated by the client after the handler returns
  (`validateCASCallbackURL`, `validateHTTPSOauth2CallbackURL`): path plus
  `ticket`/`code`. Validating in the panel keeps a wrong paste there instead of
  failing the whole login.
- `mygo.Shell.OpenExternal(url)` is the system-browser call; it is routed
  through the main thread. A failed open is not fatal: the URL and a retry
  button remain in the panel.

#### Granted resources (verified 2026-10-24)

- The portal's resource answer carries the app list the client's own "App
  Details" panel shows:
  `data.appList.data.appInfo[] = { name, apps[] }`, and each app has `id`,
  `name`, `accessAddress`, `openModel.model`, `webRelativeDomainList[]` and
  `addressList[]` (`{protocol, port, host, ip[]}`).
- Routing needs only the address rules (`IPResource`, `DomainResource`,
  `DNSResource`). The display layer needs the app fields, which the fork used
  to drop; `client.App` + `Client.Apps()` now keep the name, group, access
  address, open model and web domains, linked to the rules by `AppID`.
- An application whose `accessModel` is not `L3VPN` still belongs in the app
  list even though it contributes no rules; goTrust lists it with no address
  row. `accessAddress` wins over the first `webRelativeDomainList` entry as a
  launch hint, and a value without a scheme is prefixed with `https://`.
- A browser link is only honest for a single host with a tcp/all rule covering
  443 (https) or 80 (http); a range or a UDP-only rule gets none. The
  keep-alive's stricter rule (port 80 only, because the probe is plain HTTP) is
  a different question and stays in `service`.
- The snapshot is display data: `resources.json` beside `config.toml`, 0600,
  versioned, holding addresses and names but no credentials. `--print-resources`
  reads it without a login; `--no-ui` prints the snapshot it just fetched.

## UI polish and desktop integration (Phase 15)

- **The tray icon is a tile, not a glyph.** A 16px panel icon needs one bold
  shape and one colour; a shield with a thick arrow survives, a keyhole reads as
  a lock and a chevron reads as a letter. Draw at 128, rasterize 16/32 and look
  before writing the SVG. A 64px PNG in the binary covers HiDPI panels; the
  package needs only the scalable SVG.
- **Panel icons move.** GNOME right-aligns the status area, so a changing
  network-speed label shifts every appindicator by tens of pixels. A fixed crop
  or a remembered click coordinate is wrong a second later; locate the icon in
  the same screenshot you act on, or accept that clicks miss.
- **ydotool absolute pointer input is unreliable on this GNOME session.** Moves
  start from the top-left, which trips the Activities hot corner, and clicks
  aimed at the panel landed on the window below (the power button did not open
  the system menu). Prefer keyboard; when the pointer is unavoidable, park it on
  a window first, press Esc afterwards, and never trust a click you did not see
  land.
- **A click is not the only way a menu is proven.** The tray menu's items and
  callbacks are a pure function of state (`TrayMenu`), so a tester covers them;
  the native popup only needs a visual check that the panel shows the icon.
- **mygo cannot be asked for a unique tray icon name.** It writes `icon-1.png`
  into a per-process temporary directory and sets the icon by that name; two
  instances of an app share the name across different theme paths. Nothing broke
  in one instance, but an extension that caches icons by name can resolve the
  wrong path after a process exits.

## Code review 2026-10-24 (the tree reviewed then)

Scope: every Go file, its tests, `README.md`, `data/` and the packaging assets.
Four independent read-only slices (config/logging, session, proxy, UI/wiring)
plus cross-cutting race/lifecycle checks. The tree was clean before and after;
no code was changed by the review.

### Baseline

- `go build ./...`, `go vet ./...`, `go test ./...`, `go test -race ./...`,
  `gofmt -l .`: clean.
- `reuse lint`: compliant (47/47 files).
- `CGO_ENABLED=0 go build` and `GOOS=windows GOARCH=amd64` build: pass.
- The green suite does not exercise the paths below; the reproduced failures
  needed targeted tests, which were run from temporary files and removed.

### P0 — release blockers

1. **UI freeze: `disconnect` joins the login on the UI thread.**
   `main.go:614-642` calls `Manager.Logout` (`session/session.go:364`), which
   waits for `m.login` held by `loginSession` (`session/session.go:173`). The
   login goroutine, woken by cancellation, publishes through
   `onSessionState` (`main.go:562`) → `refreshTray` → `tray.SetMenu` → mygo
   `onMain`, which blocks until the main thread drains the loop — but the main
   thread is waiting on the mutex. Permanent hang; same on quit. Not
   reproduced natively (needs a real login); the lock order is deterministic in
   the code. Fix by keeping the join off the UI thread or posting tray/window
   updates without blocking.
2. **Proxies are never released on session failure.**
   Only `disconnect` closes `a.proxy` (`main.go:626-640`). `expire` and an
   exhausted ladder leave both listeners bound, the failed view offers only
   连接 (`ui/view.go:291-297`), and `connect` → `startProxy` then fails with
   `address already in use` on every retry until the process exits.
3. **Settings delete crashes the process (reproduced).**
   `ui/settings.go:350-355` snapshots `index`, the 删除 action runs inside the
   frame and shrinks `Ports`/`Targets`, then `:438`/`:493` index the old value:
   `panic: index out of range [1] with length 1`. Reproduced with
   `ui.NewTester`: 新建 → 删除. Unrecovered panic on the GTK main thread.

### P1 — severe

4. **Tunnel DNS is built with the literal `"auto"`** (`session/session.go:317`).
   `resolve.NewResolver` turns it into `net.ParseIP("auto") == nil` →
   `0.0.0.0:53`, and the secondary into `JoinHostPort("auto","53")`
   (upstream `resolve/resolver.go:388-418`). Hostnames that miss
   `dnsResource` cannot resolve through SOCKS5. Upstream `main.go:357-386`
   resolves `auto` via `client.DNSServer()`/`DNSServers()` first.
5. **`newStack` can never pick gVisor** (`session/session.go:423-443`):
   the fork's `CanUseTCPTunnel()` returns `true` unconditionally, so the
   `IP()`/gVisor branch is dead. The TCP tunnel's `DialUDP` is
   `not implemented`, so SOCKS5 UDP and remote DNS are unavailable, contrary
   to upstream's no-TUN default.
6. **SOCKS5 keeps the resolver of the session it started with**
   (`main.go:586-596`); after a successful auto-reconnect `m.resolver` points
   at the new stack while the proxy still holds the closed one.
7. **`Dial` refuses while `PhaseReconnecting`** (`session/session.go:401-408`),
   but a restored session stays in `SessionRestoring` until real tunnel
   traffic confirms it (upstream `session_recovery.go:81-89`). With keep-alive
   off or without targets nothing ever confirms; the proxy is stuck.
8. **`--config` does not move `client_data`** (`config/config.go:454-467`):
   the path always comes from the default `Dir()`, so a temporary settings
   file reads and overwrites the real account's ticket. README:117 promises
   otherwise.
9. **Deleting/renaming a profile leaves `client_data-*.json` behind**
   (`config/config.go:402-418`, path at `:454-467`; inline rename at
   `ui/settings.go:307`). Credentials outlive the account and a same-named
   profile inherits them; a rename loses the material instead.
10. **`currentConfig` returns a shallow copy** (`main.go:897`), and
    `storeAuthChoice`/`storeTOTPSecret` (`prompt.go:497`, `:324`) mutate the
    shared `Profiles` array without the lock. Reproduced with
    `go test -race` (`Config.Active` vs `storeAuthChoice`). A failed
    `SaveFile` also leaves the in-memory config mutated.
11. **Deleting a middle profile misaligns the parallel buffers**
    (`ui/settings.go:87-100`, `:257-276`): reproduced with 默认/B(8443)/C(9443),
    delete B, save C → port 8443, and C inherits B's keep-alive targets.
12. **Discovered auth methods survive a gateway edit**
    (`ui/settings.go:152-161`; `dropStaleMethods` runs only on reload/select),
    so the picker can store the old gateway's method for the new one.
13. **Redaction leaks all but the first cookie and `map[Authorization:…]`**
    (`logging/logging.go:272-281`). Reproduced:
    `Cookie: theme=dark; session=SECRET123` keeps `SECRET123`;
    `map[Authorization:[Bearer tok123]` keeps `tok123`.
14. **Pipelined CONNECT bytes hang the tunnel (reproduced).**
    `proxy/server.go:258-262` uses `io.Copy(target, buffered)`, which selects
    `(*bufio.Reader).WriteTo` and keeps reading the client until EOF before the
    target→client direction starts. A client that writes CONNECT plus payload
    in one go receives 200 and then nothing (3 s, 0 bytes).
15. **One failed rotation disables file logging for the process**
    (`logging/logging.go:206-246`): after `openLocked` fails, `size=0` and
    `file=nil`, so the rotation path never retries; the comment claims it does.
16. **Settings hide epoch is check-then-act** (`main.go:477-497`): a reopen
    between the check and `window.Hide()` lets the queued hide swallow the
    freshly shown window.

### P2 — medium, and nits

- Ladder attempt after 断开 can publish `PhaseFailed: context canceled` over
  Idle (`session/session.go:173`, `reconnect.go:231`); check `ctx` on entry.
- Every connecting state wipes `Resources`/`ClientIP`/`KeepAlive`
  (`session/session.go:195,228,252,286`), so the grant vanishes during a
  ladder attempt.
- `set`/`setTransition`/`recordKeepAlive` emit after unlocking
  (`session/session.go:570-596`): an old snapshot can overwrite a newer phase.
- `proxy.Server.Close` does not close hijacked CONNECT or SOCKS5 connections
  and tracks no connections (`proxy/server.go:115-144`).
- Request hop-by-hop headers are forwarded to the origin, including
  `Proxy-Authorization` (`proxy/server.go:273-283`). Half-close truncates the
  other direction (`:264-267`); streamed responses are not flushed (`:302`).
- A non-loopback bind is neither rejected nor warned about
  (`proxy/server.go:148-155`, `config/config.go:475`).
- Proxy binds are not syntax-checked in the settings window
  (`ui/settings.go:317`); the error only appears after a successful login.
- `openResource` accepts any scheme; a portal `file:`/`smb:` URL reaches
  `xdg-open` (`main.go:408`, `session/resources.go:410`).
- A stored TOTP secret is not cleared with 保存密码 and has no UI to remove it
  (`main.go:905-913`, `prompt.go:318-326`).
- `writeClientData` truncates in place (`session/session.go:665`); config and
  resource caches lack `Sync` (`config/config.go:239-277`).
- Unknown TOML keys are ignored (`config/config.go:216`); `profileSlug`
  collides on case-insensitive filesystems (`:585`); keep-alive has no lower
  interval bound (`ui/settings.go:299`).
- `Login` is accepted during a client-owned restore and then overwrites the
  old client without closing it (`session/session.go:164-181`).
- `connect` can leak a server bound after `disconnect` cleared `a.proxy`
  (`main.go:544-558`); a cancelled login still paints an English red error
  (`main.go:519-526`).
- Dead code: `proxy.Server.socks`, `Config.SelectProfile/RenameProfile/
  ClientDataPath`, `Logger.Writer/Verbose`, `cliPrompter.logger`,
  `ReconnectState.Reason`, redundant `Attention`; `--print-resources` drops
  the snapshot metadata; `forward` builds a Transport per request.
- Inherited from upstream: the login channel's certificate check is disabled
  (`client/atrust/auth/auth.go:145`); the tunnel authenticates at the aTrust layer.

### Test gaps

- No app-level test for `disconnect` or proxy rebind after failure; P0-1/P0-2
  are invisible to the suite. P1-6/P1-7 now have unit tests (`Manager.Resolve`
  follows a swapped resolver; `Dial` serves a restoring session that still has
  a dialer and refuses once the ladder has none).
- Settings tests count callbacks and call state methods directly; none drives
  新建/删除 through `SettingsView` with the real state, so P0-3/P1-11 pass.
- Proxy tests read the 200 before sending the payload, missing P1-14; no
  `Dial`-error, half-close, hop-by-hop or post-`Close` tunnel test.
- Logging tests do not assert rotated content, failed-rotation recovery,
  truncation, or concurrent writers.
- `TestClientDataPathIsPerProfile` (`config/config_test.go:312`) encoded the
  default-directory behaviour of P1-8 instead of catching it; that behaviour is
  still correct for a configuration that was never loaded, and
  `TestClientDataFollowsTheLoadedSettingsFile` now covers the loaded case.

### After the P1-4/P1-5 fix (2026-10-24)

- The TCP-tunnel decision is now `IP()`: `CanUseTCPTunnel()` is unconditional in
  the atrust client, so it cannot select a stack. The gVisor stack is the
  normal path and the TCP tunnel is the TCP-only fallback.
- **New upstream gap caused by that:** `ObserveTunnelTraffic` was called only
  by `client/atrust/tcptunnel.go:620` after a tunnel handshake. The gVisor
  stack had no equivalent, and `restoreFromTicket`/`relogin` deliberately leave
  `SessionRestoring` until that hook fires, so with gVisor a restored session
  could keep goTrust in `PhaseReconnecting` after the tunnel was working.
  **Fixed in fork `5e185ef`** (pushed to `origin/dev`, pin
  `v1.3.2-0.20261024055516-5e185ef9fdce`): the gVisor `DialTCP` reports a
  successful packet-stack connection through the same optional observer;
  `easyconnect` does not implement it. Keep-alive confirmation was considered
  as a fallback and rejected because probes can be off.
- `Manager.Resolve` reads the resolver per call, so the SOCKS5 listener follows
  an auto-reconnect instead of holding the dead stack's resolver.

## Tray interaction and the appindicator backend (2026-10-24)

Requested: double-click opens the main window; the menu opens a server-returned
application URL with the default browser.

- **打开默认应用 is implemented.** `ui.TrayMenu` calls
  `session.Resources.DefaultAppURL()` — the first `ResourceApp` entry with a
  URL, then the first host rule with a browser link — and hands it to
  `Actions.OpenResource`, the same path the resource row uses
  (`mygo.Shell.OpenExternal`). The item is disabled while the grant has no URL.
  Tests: `TestTrayMenuOpensTheDefaultApp`,
  `TestTrayMenuDefaultAppIsDisabledWithoutAURL`.
- **Double-click is not implementable with the pinned mygo v0.6.3 on Linux.**
  Its backend creates an AppIndicator and installs the menu
  (`internal/linux/menu.go:455-500`); `TrayHandler.Clicked/RightClicked` are
  only called on darwin and windows (`internal/darwin/menu.go:296-298`,
  `internal/windows/tray.go:301-305`), and `Tray.OnClick` is documented as
  "clicked and it has no menu" (`modules.go:779`). The panel gives the primary
  click to the menu, so the app never sees a double-click. A real fix is an
  upstream activation backend; two synthetic Activate events would not be a
  double-click.
- **`libayatana-appindicator-glib` is not a drop-in.** Its
  `app_indicator_set_menu` takes `GMenu*`
  (`/usr/include/libayatana-appindicator-glib/ayatana-appindicator.h:102`),
  while mygo builds a `GtkMenu`; it offers `app_indicator_set_icon`, not
  `_set_icon_full`, adds `app_indicator_set_actions` and an `activate_event`
  class signal, and links no GTK3. Supporting it is a new backend
  (GMenu + GSimpleAction), not a package swap. The deprecation warning is
  emitted by the legacy library itself.

### Upstream issue/PR draft (to file, not filed yet)

Title: Linux tray: expose activation events and add a
`libayatana-appindicator-glib` backend

- v0.6.3 binds `libayatana-appindicator3.so.1` and a GtkMenu; the Linux tray
  never calls `TrayHandler.Clicked`/`RightClicked`, so an app cannot implement
  "click the icon to open the window" or a primary-action menu item.
- `libayatana-appindicator` is deprecated upstream; distributions ship
  `libayatana-appindicator-glib`, which exposes `activate_event` and
  `scroll_event` and uses `GMenu` + `GSimpleActionGroup` with no GTK3.
- Proposal: add a glib backend selected at load time, with the current GtkMenu
  backend as the fallback; map `activate_event` → `Clicked` and a configured
  action → `RightClicked`/secondary activate; keep `NewTray`'s behavior and
  the platform interface unchanged.
- goTrust use case: a tray menu whose first action opens the main window, and a
  double-click/activation that does the same without opening the menu.

## Research 2026-10-24: keyring, password reset, disconnect exit

Asked by the operator. Every claim below is from a local tree, the pinned module
cache, the portal's responses, or the journal. Live gateway behaviour
that would need a call is marked [[INFERENCE]] or "missing".

### 1. Keychain storage for credentials

- **The branch is in EZ4Connect, not in the zju-connect fork.** Local-only
  `feat/credential-keychain` in a local
  EZ4Connect worktree ("feat: 凭据改由系统密钥环保存，
  配置文件只留账号名"). It is not on GitHub (`YangtseSu/EZ4Connect` has 8 heads,
  none of them keychain/secret; a commit search finds nothing). The zju-connect
  fork has no such branch locally or on GitHub, and the pinned module cache has
  no keyring code.
- **The branch is app-side only.** `application/secretstore.h` (one secret per
  `(scope, key)`), `application/credentialstore.{h,cpp}` (migration, fallback,
  per-profile copies) and `infrastructure/secrets/platformsecretstore.cpp`.
  Linux uses libsecret's **`secret-tool` CLI** with the secret on **stdin** and
  the attributes `service=EZ4Connect`, `scope`, `key`; macOS uses
  `SecKeychain*`, Windows the Credential Manager. `scope` is `default`, a
  profile id, or the absolute `--config-path`; `key` is `Password`,
  `TOTPSecret` or `CertPassword`. `available()` is just "secret-tool exists" on
  Linux. A missing/unavailable store keeps the old value in the settings file
  as the documented fallback; locked-collection behaviour was never tested.
  Even on that branch the Go core still receives the secret as `-password` /
  `ZJU_CONNECT_PASSWORD` argv/env, so the keyring only protects storage at rest.
- **goTrust today** keeps `password` and `totp_secret` in `config.toml` (0600),
  the cookies/sign key/sid ticket in `client_data*.json` (0600, temp+fsync+
  rename) and redacts every log line. `cfgForDisk` (`main.go:1158-1170`) is the
  single choke point every save goes through; `AuthConfig` already has
  `omitempty` on both secrets (`config/config.go:113-129`); `store_password`
  gates both (`prompt.go:325-330`, `main.go:1165-1168`).
- **A Go integration** has three shapes: `github.com/zalando/go-keyring`
  (D-Bus Secret Service on Linux, Keychain/CredMan elsewhere, no CGO, no new
  runtime package; a prompt for a locked collection blocks on a prompt agent),
  the `secret-tool` CLI (mirrors EZ4Connect, adds a `libsecret` runtime
  dependency), or a direct `godbus` Secret Service client (already in the
  module cache). Whatever the backend, goTrust has **no stable profile id**:
  profiles are keyed by a mutable name and files use `profileSlug(name)`
  (`config/config.go:608-648`), with the rename move already implemented
  (`main.go:948-975`). The keyring item therefore needs either a new stable id
  or the same move/erase hooks as the client-data file, and the `--config`
  override must be in the scope or a throwaway config reads the real account's
  secret.
- **Open decisions:** stable profile id; headless (`--no-ui`) contract when no
  session bus/prompt agent exists; locked keyring fallback; erase on
  `store_password=false`, on reset, and on profile delete; one item per profile
  or one per `(scope,key)`.

### 2. Password reset and the 90-day forced change

- **The signal is already in the login response and the fork throws it away.**
  The portal answers the password call with `code 0` and
  `data.next-step:"forced-change"` plus `change-code`,
  `change message`, `policy-object`, `username`, `domain` and `pwdRulePrompt`.
  `change-code 75500402` shows the account field,
  `75500404` allows skipping (the skip answer carries
  `data.remainedSkipTimes`). The fork's dispatcher has no case for it and fails
  with `unsupported next authentication service: forced-change`
  (`auth/auth.go:353-354`); no expiry code exists in its `codes.go`.
- **Endpoints the reset flow uses:**
  `POST forced-change` (forced change; **form-urlencoded**;
  `skip:true` supported), `POST reset-path` (logged-in
  个人信息 → 密码安全 → 重置密码; `{oldPassword,newPassword[,account-field]}`),
  `POST portal/public/resetPsw` (forgot-password reset after the SMS
  verification), `GET portal/user/policy-object` (rules) and
  `POST portal/public/policy-probe` (policy database). The dialog's
  client-side fields are `oldPassword`, `newPwd`, `confirmPwd`; both passwords
  are RSA-encrypted before sending, with the same RSA
  scheme the login uses (`psw.go:57-62`). The rule object carries
  `enablePswLength`, `enablePswCombination`, `enablePswNotContainName`,
  `enablePswNotEqualHistory`, `notBelongToWeakDb`, `effectiveTime` and
  `enablePswExpire` — the 90-day policy is `effectiveTime`+`enablePswExpire`.
- **Comparable clients:** EZ4Connect has no reset dialog and cannot have one (the
  Go core has no such call); atrust-quadlet deliberately hands a forced change
  to a VNC session. Neither implements it.
- **Feasibility:** yes, with one fork addition and one measurement. The fork
  must expose the change call and the forced-change signal (the session's
  `csrfToken`, `pubKey`, `antiReplayRand` and `request` are unexported), e.g.
  `Session.ChangePassword(ctx, old, new)` plus a typed
  `ErrPasswordChangeRequired{Code, Message, Strategy, Username, Domain, Prompt}`
  and a skip call. goTrust then needs a panel (old, new, confirm, rules from
  `policy-object`, skip when allowed), wired like the existing code/captcha
  prompts, and it must rewrite the stored credential afterwards or every
  reconnect fails. Missing evidence: one round-trip on an expired
  account (the exact body/encoding of `forced-change`, whether a ticket is
  already issued, what success returns, whether other sessions are cut).
- **Safe immediate step, no guessing:** recognise the
  `forced-change` next-service error and report "网关要求修改密码" instead of a
  generic login failure.

### 3. 断开 exited the process

- **Root cause, from the journal of the 14:41 run**
  (`journalctl --user`, app started from the desktop):
  `panic: EOF` in `stack/gvisor.(*Stack).Run` at `stack.go:244`, goroutine
  created by `session.(*Manager).loginSession` at `session/session.go:354`, at
  the same second the proxy logged its close. 断开 cancels the session, the
  client closes the L3 connection, the read loop gets EOF, and the fork panics
  because its "shutdown is terminal" flag lives in an `internal/hook_func`
  package an external module cannot reach (`stack/gvisor/stack.go:238-249`).
  A panic in that goroutine ends the whole tray app.
- **Fixed in one commit** (package `r44`): the stack runs
  through `Manager.runPacketStack`, which recovers, logs
  `the packet stack stopped: …` and lets the keep-alive/session state observe
  the dead tunnel. Regression test `TestAPacketStackPanicDoesNotKillTheApp`.
- **Follow-up for the fork:** `Run` should return (or expose a stop/terminal
  hook) on `net.ErrClosed`/EOF instead of panicking; a read error while *online*
  is currently silent to the session state, so the keep-alive failure is the
  only signal. The existing `fix/core-exit-cleanup` branch already replaced
  panics with `hook_func.Fatalf` in the service listeners but not in this read
  loop.

### Proposed phases

1. **Keyring** — secret storage with `go-keyring` (D-Bus) or `secret-tool`,
   a stable profile scope, migration of existing plaintext, erase on
   `store_password=false`/delete/reset, headless fallback, tests with a fake
   store.
2. **Password change** — first the fork API and the typed forced-change
   signal, then the panel and the credential rewrite; blocked on one live
   measurement.

### Operator decisions on the two phases (2026-10-24)

- **Order:** password change (full, live-tested when the gateway first
  forces it), then keyring with the backend chosen at execution.
- **Password change:** implement the forced `forced-change` flow, the skip
  path, the rule rendering, and the voluntary `reset-path` reset; the
  live 90-day test waits until the gateway triggers it.


### Verified password request shapes (Phase 19, 2026-10-24)

The reset flow uses these request shapes:

- **Forced reset page** fields: `oldPassword`, `newPwd`, `confirmPwd`, plus
  `account-field` when `change-code` is `75500402` (also shown when the change
  may be skipped, `75500404`). It hides the rule list for a non-local `domain`
  unless `pwdRulePrompt` is present.
- **Submit**: `POST forced-change`,
  `Content-Type: application/x-www-form-urlencoded`, body
  `oldPassword=<rsa>&newPassword=<rsa>[&account-field=<name>]`. The page encrypts
  both with `sa()` (`RSA RSA`, hex, 245-char chunks) and **does not
  append the anti-replay nonce** the login password uses.
- **Skip**: same endpoint, body `skip=true`; the answer's
  `data.remainedSkipTimes` feeds "剩余跳过机会 N 次".
- **Voluntary reset**: `POST reset-path`, JSON
  `{oldPassword:<rsa>,newPassword:<rsa>}` (the reset form adds no
  `account-field`); the note is "密码重置后，其他终端已在线账号需重新登录".
- **Weak password**: `POST
  portal/public/policy-probe`, JSON `{password:<plaintext>}`; the form treats
  `code == 0` as acceptable and a non-zero code with `validate == false` as
  "密码不能属于弱密码库".
- **Rules**: `GET portal/user/policy-object` returns `{code,
  data:{enablePswLength:{enable,value}, enablePswCombination:{enable,
  value:{enableAllLetter, enableUpperAndLowerLetter, enableNumber,
  enableSpecialLetter}}, enablePswNotContainName, enablePswNotEqualHistory:
  {enable,value}, notContainContChar:{enable,value}, notContainKeyboardContChar:
  {enable,value}, notBelongToWeakDb, effectiveTime, enablePswExpire,
  changeFirstLogin, changeMangerResetPwd}}`. The default object the voluntary
  page uses is `{changeFirstLogin:"1", effectiveTime:"90", enablePswExpire:"1",
  enablePswLength:{enable:true,value:"8"}, enablePswCombination:{enable:true,
  value:{enableAllLetter:true,enableUpperAndLowerLetter:false,
  enableNumber:true,enableSpecialLetter:true}}, enablePswNotContainName:true,
  enablePswNotEqualHistory:{enable:true,value:"3"},
  notContainContChar:{enable:true,value:"3"},
  notContainKeyboardContChar:{enable:true,value:"3"}, notBelongToWeakDb:1,
  changeMangerResetPwd:1}`.
- **Repeated characters**: the rule regex is `(\w|\W)\1{n-1}`, i.e. value
  `3` rejects `"aaa"` (exactly three repeats, not four).
- **GoTrust implementation**: the fork answers the demand in place
  (`auth.LoginOptions.PasswordChangeHandler`), continues the chain with the
  answer's `next-step` (default `auth/auth-check`), and reports the new
  password through `OnPasswordChanged` only after `code == 0`; the chain skips
  `reportEnv` when the portal issued no ticket with the change. The voluntary
  path is `Client.ChangePassword` / `PasswordStrategy` / `CheckWeakPassword`,
  gated on the live session.
