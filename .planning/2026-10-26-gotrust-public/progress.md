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

### Phase 2: Protocol layer in-tree

- **Status:** pending

### Phase 3: Front-end split

- **Status:** pending

### Phase 4: Web prompt for interactive steps

- **Status:** pending

### Phase 5: GUI refactor and publication

- **Status:** pending
