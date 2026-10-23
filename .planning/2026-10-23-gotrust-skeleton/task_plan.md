# Task Plan: goTrust skeleton

## Goal

A buildable, runnable Go desktop app skeleton in `~/projects/goTrust` that starts a mygo window and tray, manages one aTrust session through the fork of `zju-connect` as a library, and exposes SOCKS5 + HTTP proxy listeners over that session's dialer.

## Next Step

Phase 8: add `--print-dae-config`, emitting a dae `node`/`group`/`routing` fragment for the current listeners and the session's intranet prefixes, with tests for the loop rule and the rule ordering.

## Current Phase

Phase 8

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

### Phase 8: Transparent proxy via dae (next)

Research is in `findings.md`; dae stays a separate daemon.

- [ ] `--print-dae-config` emits `node`/`group`/`routing` for the configured listeners
- [ ] Intranet prefixes derived from the session's IP resources, not hardcoded
- [ ] The emitted rules always start with `pname(goTrust) -> must_direct`
- [ ] Intranet prefixes precede any private-range preset, and the reason is a comment in the output
- [ ] Tests: prefix derivation, loop rule present, ordering, an unconnected session still emits a usable template
- [ ] README section "Transparent proxy (optional)" with the daemon boundary and the DNS caveat
- **Status:** pending

### Phase 9: Live gateway verification (blocked)

- [ ] Real login, proxied request and keep-alive against a gateway
- **Status:** pending
- **Blocked on:** an account that may be consumed; the only account on this host belongs to the operator's production service

## Key Questions

1. Does mygo's `ui` package expose a text input suitable for username/password? (answer: `ui.Field` wraps a child view; check for a bound-input constructor before writing the form)
2. Does the fork's `dev` branch carry the session-recovery fixes needed for a usable state machine? (answer: yes, 72 commits ahead, CI green)
3. Can `service.ServeSocks5` be used? (answer: no — it has no context and registers cleanup through `internal/hook_func`; own listeners instead)

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| New repository, not a fork of `zju-connect` | The protocol fixes stay usable as a library; the app rewrites the wiring anyway, and `go get` beats rebasing 46 files |
| Compile-time dependency on the fork, not a child process | Single Go binary is the whole point; process-boundary would keep the Qt/GPL split |
| AGPL-3.0-or-later for goTrust | Static linking with AGPL code makes the combination AGPL; GPLv3 §13 only permits the combination in the AGPL direction |
| Own SOCKS5/HTTP listeners instead of `service.Serve*` | Those take no `context.Context` and shut down through `internal/hook_func`, which an external module cannot import |
| No TUN, no system proxy | Removes root elevation, wintun, and three platform backends; keeps the app a plain user process |

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

- The production aTrust account is owned by the operator's production service; this skeleton must never attempt a real login during development.
- `internal/` packages of `zju-connect` are unreachable from here by design. Only use `client`, `client/atrust`, `client/authchallenge`, `dial`, `resolve`, `service`, `stack/gvisor`, `underlay`.
