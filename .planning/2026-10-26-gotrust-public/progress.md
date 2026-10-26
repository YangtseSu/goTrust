# Progress Log

## Session: 2026-10-26

### Plan written

- **Status:** complete
- Actions taken:
  - measured the dependency closure of the current tree (22 packages, ~27.9k
    lines including tests) and the per-package split of what the app needs and
    what the fork carries for its CLI
  - confirmed with `go list -deps` that `config`, `session`, `proxy` and
    `logging` reach neither mygo nor purego, so the core/UI boundary already
    exists and only needs a guard
  - confirmed `GOWORK=off go build ./...` succeeds, so the pinned commit is
    buildable without the local workspace redirection
  - chose the import source: the published commit, never a working tree
  - rejected a separate protocol repository and a nested module, with reasons
    recorded in `findings.md`
- Files created/modified:
  - `.planning/2026-10-26-gotrust-public/task_plan.md`
  - `.planning/2026-10-26-gotrust-public/findings.md`
  - `.planning/2026-10-26-gotrust-public/progress.md`
  - `AGENTS.md` (plan pointer)

### Phase 1: Names and layout (mechanical)

- **Status:** complete
- **Started:** 2026-10-26
- Actions taken:
  - moved `config`, `session`, `proxy` and `logging` under `core/` as a pure
    import-path rewrite, then confirmed with
    `go list -deps ./core/...` that the guard is empty
  - lowercased the binary, the XDG config and log directories, the log file
    name, the desktop entry and both icons. The desktop `Name=`,
    the window and tray text, the client identity string and the reported
    process name keep `goTrust`
  - updated `README.md`, `docs/agent-ui-testing.md` and the packaging recipe
  - migrated `~/.config/goTrust` and `~/.local/state/goTrust` by hand while the
    app was stopped: six config files and one log file moved, permissions kept
  - removed a stale 19 MB build artifact from the repository root; the renamed
    ignore rule no longer covered it and `reuse lint` saw it as unlicensed
- Gates:
  - `go build ./...`, `go test ./...`, `go vet ./...` clean, `gofmt -l .` empty,
    `reuse lint` 57/57
  - `makepkg -fC` built `gotrust-git r67`; the payload is
    `/usr/bin/gotrust`, `gotrust.desktop` and both `gotrust` icons
  - nested headless UI smoke: ready line, the window renders with its `goTrust`
    heading, SIGTERM exits 0, no processes left behind
  - headless live login against the migrated config: session online, two
    granted resources, SOCKS5 and HTTP listeners up, keep-alive answered 200
- Follow-ups:
  - the installed package is still `gotrust-next-git`, which owns
    `/usr/bin/goTrust`; installing `gotrust-git` replaces both the binary and
    the desktop entry
  - the old `goTrust.log` in the state directory stays as history; new runs
    write `gotrust.log`
  - the reported tunnel process name still reads `goTrust` (see the plan's Key
    Questions)
- Files created/modified:
  - `core/**` (moved), `data/gotrust.desktop`, `data/icons/**`, `README.md`,
    `docs/agent-ui-testing.md`, `AGENTS.md`, `.gitignore`, `icon.go`,
    `core/config/config.go`, `core/logging/logging.go` and their tests

## Session: 2026-10-27

### Phase 2: Protocol layer in-tree

- **Status:** complete
- **Started:** 2026-10-27
- Actions taken:
  - took the tree from the module cache copy of the published commit
    (`zju-connect@v1.3.2-0.20261026141502-5caa3a1684a1`), not from a working
    checkout, and copied the dependency closure into `core/atrust/` at its
    original relative paths
  - rewrote the module prefix in 134 import lines and 13 application files
  - dropped `client/easyconnect` and the packet-stack branch that watched for
    its shutdown sentinel, `stack/tun`, the CLI entry point, `mobile/`, the
    service package's listeners, the packaging and documentation files
  - dropped the CLI's start-up hooks as well: they were unreachable from the
    application and one of them named operator-specific DNS suffixes. Their
    only consumer, the CLI's config type, went with them, which is what the
    plan's "`configs` once nothing kept needs it" resolved to. `hook_func`
    keeps the terminal hooks the stacks call
  - narrowed `service` to the DNS server and the keep-alive, so the package no
    longer carries the listeners `core/proxy` owns
  - deleted the `replace` directive, the fork's `go.sum` lines and `go.work`,
    and removed the sibling-path symlink from the PKGBUILD
  - annotated `core/atrust/**` in `REUSE.toml` (`AGPL-3.0-only`, with `MIT` for
    `internal/ping` and goTrust's own copyright for `PROVENANCE.md`) and added
    `LICENSES/AGPL-3.0-only.txt` and `LICENSES/MIT.txt`
  - wrote `core/atrust/PROVENANCE.md`: source repository, commit, imported and
    dropped packages, the edits made after the import, and the public index the
    client-version literal comes from
  - bounded text pass: third-party identifiers and comments naming another
    product became protocol descriptions; the tunnel
    process block defaults to the running binary; fixtures moved to the
    documentation address ranges; the challenge page title and the fatal-hook
    test's environment variable follow the project name
  - renamed the last fork-named import alias in `main.go` (`atrustlog`)
- Gates:
  - `diff -r` audit: 95 imported files differ from the source only by the
    module prefix, 25 by the documented pass and 35 source files were not
    imported; `PROVENANCE.md` is the only new file
  - `go build ./...`, `go test ./...`, `go vet ./...` clean, `gofmt -l .` empty
  - cross-builds for darwin/amd64, darwin/arm64, windows/amd64 and linux/arm64
    pass, so the platform files that stayed still compile
  - `go mod tidy` removed the fork require and replace, `gopsutil`,
    `shadowsocks2`, `utls`, `brotli`, `etree`, `klauspost/compress`,
    `go-bloom` and `x/crypto`, and promoted the layer's own direct
    dependencies (dns, gvisor, sing, sing-tun, go-cache, x/sys)
  - `reuse lint` 177/177, with `AGPL-3.0-only`, `AGPL-3.0-or-later` and `MIT`
    as the used licenses
  - live headless run: session online, two granted resources, SOCKS5 and HTTP
    listeners up, the granted web resource served through both (200, identical
    body), keep-alive answered 200, SIGTERM exit with no process left behind
  - `makepkg -fC` built `gotrust-git r70`; `namcap` reports only the
    warnings the recipe already documents, and the payload is unchanged
- Follow-ups:
  - the tunnel process block now defaults to the running binary, so a profile
    with an empty `process_name` reports `gotrust` where it used to report the
    source project's binary. The app names its own process anyway; the change
    only affects what an unset field sends
  - `internal/hook_func` still carries `Fatalf`'s exit-on-shutdown semantics, so
    Phase 3 re-evaluates the package with the CLI split
- Files created/modified:
  - `core/atrust/**` (imported), `core/session/*.go` and the two top-level
    entry files (import paths), `main.go` (alias), `go.mod`, `go.sum`,
    `REUSE.toml`, `LICENSES/AGPL-3.0-only.txt`, `LICENSES/MIT.txt`, `README.md`,
    `AGENTS.md`, `~/projects/arch-pkgs/gotrust-git/PKGBUILD`

### Phase 3: Front-end split

- **Status:** pending

### Phase 4: Web prompt for interactive steps

- **Status:** pending

### Phase 5: GUI refactor and publication

- **Status:** pending
