# Task Plan: goTrust — self-contained core, two front-ends, public repository

## Goal

goTrust stops depending on an external fork at build time. The aTrust protocol
layer lives in this repository under `core/`, the mygo GUI and a headless CLI
are two independent front-ends over that core, and every commit is publishable
as it stands because the repository is going public.

Naming rule: the project and everything a user reads stays `goTrust`; binaries,
directories, XDG paths and packaging names are lowercase.

The scope is rearrangement and trimming of code that already works. No new
protocol capability is added, and the behaviour of the login path stays what
the gateway has already accepted.

## Next Step

Phase 3: split the front-ends. `internal/cli` takes the config load, the
manager wiring, the signal handling and the exit codes; `cmd/gotrust-cli`
imports `core/*` only. The per-profile lock lands with it, and the dependency
guards then report empty.

## Current Phase

Phase 2 is complete: the protocol layer lives under `core/atrust/`, the fork
dependency, the `replace` directive and `go.work` are gone, and the imported
tests run in this repository. Phase 1 moved the application packages under
`core/`, lowercased the technical names and migrated the installed directories;
`../2026-10-23-gotrust-skeleton/` built the app, this plan moves it behind a
stable core and makes the repository publishable.

## Phases

### Phase 1: Names and layout (mechanical)

- [x] Inventory every case-sensitive name: module path, binary, XDG directories,
      log file, desktop entry, icons, PKGBUILD fields and the client identity
      string
- [x] Keep `goTrust` for the module path and repository name, the README title,
      the desktop entry's `Name=` and every window/tray/string a user reads
- [x] Lowercase the rest: `gotrust` binary, `~/.config/gotrust`,
      `~/.local/state/gotrust/gotrust.log`, `gotrust.desktop`, `gotrust.svg`,
      `gotrust-symbolic.svg`, the PKGBUILD install target
- [x] Leave the client identity string alone: it is sent to the gateway and is
      not a display name, so changing it is a behaviour change, not a rename
- [x] Move `config`, `session`, `proxy` and `logging` under `core/`; leave `ui/`
      and the GUI entry point at the top level
- [x] Migrate the config and log directories once, by hand; no migration code
      in the binary
- **Gates:** `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`,
  `reuse lint`, the `gotrust-git` package builds, a login works after the
  directory move, UI smoke per `docs/agent-ui-testing.md`
- **Status:** complete. The core-move and lowercase-names commits. The `cmd/`
  directories belong to Phase 3, where both entry points are created in one step.

### Phase 2: Protocol layer in-tree

- [x] Take the published `zju-connect` tree at commit `5caa3a1` (module cache or
      `git archive`), not a working tree
- [x] Import the dependency closure into `core/atrust/`, keeping the source's
      relative layout so the import can be reviewed as a diff against the
      source tree
- [x] Drop `client/easyconnect`, together with the packet stack's
      vendor-specific shutdown branch it exists for; the stack keeps taking the
      `client.Client` interface unchanged
- [x] Drop the CLI (`main.go`, `init.go`), `mobile/`, `stack/tun/`,
      `service/shadowsocks.go`, the READMEs, the Dockerfile, the compose file
      and the plist. `configs` went with them: its only consumer was the CLI's
      start-up hooks, which left in the same step
- [x] Split `service` so the keep-alive and DNS pieces stand alone: the package
      keeps `dns.go`, `keep_alive.go` and `keepalive_target.go`, and the SOCKS5,
      HTTP, forwarding and shadowsocks listeners went with the CLI that ran
      them. `internal/hook_func` keeps the terminal hooks the stacks call; its
      start-up hooks went with `configs` because they named operator-specific
      DNS suffixes
- [x] Delete the `replace` directive, the fork's `go.sum` lines, `go.work` and
      the PKGBUILD symlink step
- [x] Keep `LICENSES/` intact, annotate `core/atrust/**` in `REUSE.toml` with
      the upstream copyright (`AGPL-3.0-only`, plus `MIT` for
      `internal/ping`), and write `core/atrust/PROVENANCE.md`: source, commit,
      imported packages, dropped packages
- [x] Bounded text pass over the imported tree: third-party identifiers and
      comments naming another product become protocol descriptions; the tunnel process block defaults to the running
      binary; test fixtures use the documentation address ranges. Algorithms,
      constants and test vectors are untouched
- **Gates:** `diff -r` between the imported directory and the source tree shows
  only the import-path rewrite and the dropped files; the imported tests are
  green; the `go mod tidy` delta is exactly the expected subset; a live login,
  proxy and keep-alive run; `makepkg -fC`
- **Status:** complete. The import commit and the protocol-layer docs commit.
  95 imported files differ from the source only by the module prefix, 25 by the
  documented pass, and 35 source files were not imported. `go mod tidy` removed
  the fork, `gopsutil`, `shadowsocks`, `utls`, `brotli`, `etree`,
  `klauspost/compress`, `go-bloom` and `x/crypto`, and promoted the layer's own
  direct dependencies. The live run logged in, served the granted web
  resource through both proxies (200, same body over SOCKS5 and HTTP) and
  answered the keep-alive with 200;
  `makepkg -fC` built `r70`, and the cross-builds for darwin and
  windows pass.

### Phase 3: Front-end split

- [ ] `internal/cli`: config load, manager wiring, signal handling, exit codes,
      `--status` and `--json`
- [ ] `cmd/gotrust-cli`: imports `core/*` only, never `ui/`, never mygo
- [ ] A per-profile lock, so the GUI and the CLI cannot share one profile's
      login state or tunnel ports while still running different profiles
- [ ] Delete `-no-ui` and `--print-resources` from the GUI binary once the CLI
      is at parity; one headless implementation only
- [ ] Dependency guards in CI: no mygo or purego under `core/` or in
      `cmd/gotrust-cli`, and neither front-end imports the other
- **Gates:** headless login against the live gateway, proxies serve, exit codes
  distinguish success from failure, the guards report empty

### Phase 4: Web prompt for interactive steps

- [ ] `core/prompt/web`: one `127.0.0.1` page per challenge, a single-use token
      in the path, blocking on the login attempt's context
- [ ] One-time codes, click captcha (image served as-is, natural-size
      coordinates), external SSO (link plus pasted callback URL), forced
      password change, device-bind confirmation, auth-method choice
- [ ] Rules are rendered as hints only; validation stays in `core/session`
- [ ] `--print-url` for hosts where no browser can be started
- [ ] `httptest` coverage of every challenge type
- **Gates:** a live login that needs a click captcha completes through the page;
  no code, password or callback secret appears in the log

### Phase 5: GUI refactor and publication

- [ ] `app` becomes `internal/gui`: windows, tray, event loop and state mapping;
      every business call goes through `core/session`
- [ ] Publication: flip the repository to public and switch the PKGBUILD source
      from the local checkout to `git+https://`
## Key Questions

- Should the CLI be its own binary or a mode of the GUI binary? Own binary: the
  GUI binary loads GTK, so it cannot serve a headless host, and two headless
  implementations would drift.
- Lock granularity? Per profile. A single config-directory lock would forbid
  running the GUI on one account and the CLI on another.
- Where does the lock live on Windows and macOS? Behind a small `core` package
  with one implementation per platform, so no front-end owns it.
- Does the process name reported on the tunnel block follow the binary name? It
  stays `goTrust` for now: it is a wire identity, and Phase 1 changes no bytes
  the gateway sees.

## Decisions Made

| Decision | Why |
|----------|-----|
| One Go module, `core/` as a directory convention | A nested module would bring back the version boundary this plan removes; the guard is the front-ends' dependency graphs, which is what "runs without mygo" actually means |
| Import only the published commit | Unpublished protocol work stays out of the tree; anything not published today is redesigned when it is needed rather than carried over |
| Preserve the source's relative layout in `core/atrust/` | The import stays reviewable as a diff; a package rename is a separate commit if it ever earns one |
| Keep the fork published and frozen | It carries the PR record and the last complete CLI; freezing means the packaging pins a commit that no longer moves |
| CLI is a front-end, not a compatible tool | No TUN, no shadowsocks, no flag compatibility with any other client; it reuses `core/config`, `core/session` and `core/proxy` |
| Web page for interactive steps | A click captcha needs an image and coordinates, and an SSO login needs a URL and a pasted answer; a terminal cannot render either, and the page works over SSH forwarding and in tests |
| Third-party version literal in imported fixtures | Client versions are published on a public index, so the value is a published fact; keep it as a configurable client version and record the source page in `PROVENANCE.md` |
| Delete `--no-ui` after the CLI lands | Two headless paths diverge; the GUI binary should only do GUI |

## Errors Encountered

None yet.

## Notes

- Every commit must be publishable at the moment it is made: no third-party
  identifier names, library names or version strings, no wording that sources a
  value from another product, private paths, real gateway
  addresses or client data. Categories are listed in `findings.md`.
- The imported tests are the acceptance gate for Phase 2. They are the only
  proof that the move did not change behaviour, because the login path itself
  is the same code the gateway already accepted.
- `reuse lint` must stay clean. New protocol files are annotated through
  `REUSE.toml` rather than by editing borrowed files.
- No dead weight: the fork dependency, the `go.work` file, the PKGBUILD symlink
  step and `--no-ui` all disappear in the plan rather than living beside their
  replacements.
