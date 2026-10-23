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
