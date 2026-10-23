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

## Visual/Browser Findings

- Headless render of a CJK view (`ui.NewTester`) produced correct glyphs at 22px and body size; no tofu, no baseline drift.
- The goTrust window renders correctly at 520x560 with four `TextInput` fields, a masked password, four buttons, a divider and the listener list. `ui.NewTester` needs one extra `Frame()` before `Image()` for values typed into inputs to appear.
