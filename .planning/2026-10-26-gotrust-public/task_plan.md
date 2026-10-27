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

Phase 5 is complete: the repository is public at
<https://github.com/YangtseSu/goTrust>, CI is green, and
`makepkg -fC` built `gotrust-git` and `gotrust-cli-git` at `r80`
from the HTTPS source. Phase 20 of the app plan (keyring) is the next feature.

## Current Phase

Phase 5 is complete. `internal/gui` holds the tray app behind one `Run` entry
point, `cmd/gotrust` supplies main, and neither front-end imports the protocol
layer: `core/session` re-exports the challenge types the prompter interface
names and owns the debug switch. `internal/guards` kept its dependency guards, the PKGBUILD splits the tray app from the headless front-end and
clones over HTTPS, the repository is public, CI passed its five checks, and both packages were built at `r80` from the public source. Phase 4 is complete: `core/prompt/web`
serves every interactive login step as one page on `127.0.0.1` under a
single-use token, and the CLI opens or prints it. Phase 3 gave the CLI its own
binary and a per-profile lock; Phase 2 put the protocol layer in the tree;
Phase 1 moved the application packages under `core/`.
`../2026-10-23-gotrust-skeleton/` built the app.

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

- [x] `internal/cli`: config load, manager wiring, signal handling, exit codes,
      `--status` and `--json`
- [x] `cmd/gotrust-cli`: imports `core/*` only, never `ui/`, never mygo
- [x] A per-profile lock, so the GUI and the CLI cannot share one profile's
      login state or tunnel ports while still running different profiles
- [x] Delete `-no-ui` and `--print-resources` from the GUI binary once the CLI
      is at parity; one headless implementation only
- [x] Dependency guards in CI: no mygo or purego under `core/` or in
      `cmd/gotrust-cli`, and neither front-end imports the other
- **Gates:** headless login against the live gateway, proxies serve, exit codes
  distinguish success from failure, the guards report empty
- **Status:** complete. The CLI-split commit. `internal/cli` owns the command line
  (`--config`, `--status`, `--print-resources`, `--json`), the log bridge, the
  listeners and the exit codes 0/1/2/3; the login, the config load and the
  identity write-back come from `core`. `core/profilelock` takes an exclusive
  `flock(2)`/`LockFileEx` on a file beside the settings, keyed by the settings
  file and the profile name; both front-ends take it before a login and release
  it after the logout. `internal/guards` holds the dependency invariants as
  tests, so `go test ./...` fails on a toolkit import under `core/` or a
  front-end reaching the other. The live run logged in, served the granted web
  resource through both listeners (200, identical body), answered the
  keep-alive, refused a second instance with the holder's pid, and exited 0 on
  SIGTERM with the profile free again; `--status` returned 3 when free and 0
  when held. `makepkg -fC` built `r72` with both binaries, and the
  darwin/arm64, darwin/amd64, windows/amd64 and linux/arm64 cross-builds pass.
  `core/session/password.go` no longer names a removed flag, and the forced
  password change is refused with a front-end-neutral reason.

### Phase 4: Web prompt for interactive steps

- [x] `core/prompt/web`: one `127.0.0.1` page per challenge, a single-use token
      in the path, blocking on the login attempt's context
- [x] One-time codes, click captcha (image served as-is, natural-size
      coordinates), external SSO (link plus pasted callback URL), forced
      password change, device-bind confirmation, auth-method choice
- [x] Rules are rendered as hints only; validation stays in `core/session`
- [x] `--print-url` for hosts where no browser can be started
- [x] `httptest` coverage of every challenge type
- **Gates:** a live login that needs a click captcha completes through the page;
  no code, password or callback secret appears in the log
- **Status:** complete. `core/prompt/web` binds a listener on `127.0.0.1` per
  question and serves one page under 128 bits of token randomness; the page
  answers once, rejects a request whose `Host` is not a loopback name, and
  closes with the attempt's context. The CLI uses it for the prompter, the
  authentication-method chooser, the forced password change and the device
  binding, prints the URL to stderr, and only tries the browser without
  `--print-url`; a changed password reaches the retry's credential and the file
  through `config.ForDisk`. The imported text handler and its temporary browser
  page are gone, and a caller with no prompt now gets `UnsupportedHandler`.
  The live login was asked for a click captcha on a scratch configuration; the
  page served the image, the posted points entered the protocol, and the login
  reached online after the gateway accepted the answer. The log held no token,
  coordinate, code or callback value (only `prompt: <url>` went to stderr, never
  to the file), `go test ./...` covers every page, and `reuse lint` is clean.
  The page is not a new mechanism: the imported terminal handler already
  started a loopback server and opened a browser for the click captcha whenever
  no captcha file path was configured, which was goTrust's case. This phase
  moved that page out of the protocol layer, extended it to every other
  challenge, and added the single-use token, the host check, the attempt
  context, no disk copy and per-type tests. Its stdin paths and the file branch
  are recorded in `PROVENANCE.md` as dropped.

### Phase 5: GUI refactor and publication

- [x] `app` becomes `internal/gui`: windows, tray, event loop and state mapping;
      every business call goes through `core/session`. The two exceptions were
      imports, not calls: `core/atrust/client/authchallenge` for the prompter
      types (now re-exported by `core/session`) and `core/atrust/log` for the
      debug switch (now `session.SetProtocolDebug`). A guard checks the direct
      imports of both front-ends, which the transitive graph cannot express
      because `core/session` itself is built on the protocol layer
- [x] Publication: flip the repository to public and switch the PKGBUILD source
      from the local checkout to `git+https://`. `main` was pushed
      and the repository is public; `makepkg -fC` built
      `gotrust-git` and `gotrust-cli-git` at `r80` from the HTTPS
      source; both payloads were inspected and both binaries started
- **Gates:** UI smoke per `docs/agent-ui-testing.md`, CI green. The local gates ran:
  build, vet, gofmt, `go test ./...` (with `-race` on the moved packages),
  `reuse lint` 193/193, four cross-builds, and the nested headless smoke
  (window rendered, ready line, SIGTERM exit 0). The pushed commit ran CI
  green (build, vet, gofmt, tests with the guards, the
  cross-builds, `reuse lint`); `makepkg -fC` built both packages from the
  HTTPS source, the payloads were inspected, and both packaged binaries were
  run (the GUI wrote its ready line and exited 0 on SIGTERM,
  `gotrust-cli --status` returned 3). `namcap` keeps the documented warnings
- **Status:** complete

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
| Web page for interactive steps | The imported layer already served a local page for the click captcha by default: its terminal handler started a loopback server and opened a browser unless a captcha file path was configured, and goTrust never configured one. The plan generalizes that page to every challenge and hardens it. A terminal cannot render an image, a click coordinate or an SSO redirect, and the page works over SSH forwarding and in tests |
| Third-party version literal in imported fixtures | Client versions are published on a public index, so the value is a published fact; keep it as a configurable client version and record the source page in `PROVENANCE.md` |
| Delete `--no-ui` after the CLI lands | Two headless paths diverge; the GUI binary should only do GUI |
| Lock by (settings file, profile), file beside the settings | One profile owns one client data file and one tunnel identity; a config-wide lock would forbid the tray app on one account and the CLI on another, and `--config` keeps a scratch file apart |
| Try-lock instead of a pid file | `flock`/`LockFileEx` is released by the kernel when the process ends, so a crash cannot leave a stale claim; the holder's pid is written for the refusal message |
| `--status` exits 3 when no instance holds the profile | A supervisor needs the distinction, and `systemctl is-active` uses the same code |
| One listener per question, not one per attempt | A page owns its listener and token and closes when it answers, so a finished or abandoned question cannot be reloaded and no prompt state outlives the wait; the cost is a fresh ephemeral port per question, which a person never sees |
| A page only accepts loopback `Host` names | A hostile page cannot reach a prompt through DNS rebinding, and an SSH port forward that keeps `127.0.0.1`/`localhost` still works |
| Password rules are hints in the page | The portal re-checks every rule; a local verdict on the policy database, the history or keyboard runs would refuse a password the portal accepts |
| Answers never reach the log, the URL does | A code, a password or a callback ticket is a credential; the URL is how a host without a display finds the page, and it dies with the question |
| A forced change updates the in-memory credential before the file | The retry runs with the new password even when the settings file cannot be written; the file still follows `ForDisk`, so an unstored password stays in memory only |

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
