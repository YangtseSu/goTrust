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
