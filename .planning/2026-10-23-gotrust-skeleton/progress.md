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

## Error Log

| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-10-23 | `go get github.com/mythologyli/zju-connect@dev`: unknown revision | 1 | `main` is the default branch; used `replace` with a fork pseudo-version |
| 2026-10-23 | pseudo-version `v1.3.1-0.20261020T02-…` rejected | 2 | timestamp needs 14 digits; stripped the separators first |

## 5-Question Reboot Check

| Question | Answer |
|----------|--------|
| Where am I? | Phase 7, verification (live-gateway check outstanding) |
| Where am I going? | live login, proxied request, keep-alive |
| What's the goal? | a buildable, runnable goTrust skeleton |
| What have I learned? | see `findings.md` (license chain, embeddable surface, shutdown gap, mygo API traps) |
| What have I done? | four packages written and tested; binary built, launched and shut down cleanly |
