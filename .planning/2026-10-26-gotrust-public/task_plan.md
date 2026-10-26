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

No phase has started. Begin with Phase 1, the name inventory and the `core/`
move, because every later phase writes imports to their final home and a second
pass over the same files would double the review.

## Current Phase

None started. `../2026-10-23-gotrust-skeleton/` built the app and closed with a
working tray client; this plan moves that app behind a stable core and makes
the repository publishable.

## Phases

### Phase 1: Names and layout (mechanical)

- [ ] Inventory every case-sensitive name: module path, binary, XDG directories,
      log file, desktop entry, icons, PKGBUILD fields and the client identity
      string
- [ ] Keep `goTrust` for the module path and repository name, the README title,
      the desktop entry's `Name=` and every window/tray/string a user reads
- [ ] Lowercase the rest: `gotrust` and `gotrust-cli` binaries, `cmd/`
      directories, `~/.config/gotrust`, `~/.local/state/gotrust/gotrust.log`,
      `gotrust.desktop`, `gotrust.svg`, `gotrust-symbolic.svg`, the PKGBUILD
      install target
- [ ] Leave the client identity string alone: it is sent to the gateway and is
      not a display name, so changing it is a behaviour change, not a rename
- [ ] Move `config`, `session`, `proxy` and `logging` under `core/`; leave `ui/`
      and the GUI entry point at the top level
- [ ] Migrate the config and log directories once, by hand; no migration code
      in the binary
- **Gates:** `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .`,
  `reuse lint`, the `gotrust-git` package builds, a login works after the
  directory move, UI smoke per `docs/agent-ui-testing.md`

### Phase 2: Protocol layer in-tree

- [ ] Take the published `zju-connect` tree at commit `5caa3a1` (module cache or
      `git archive`), not a working tree
- [ ] Import the dependency closure into `core/atrust/`, keeping the source's
      relative layout so the import can be reviewed as a diff against the
      source tree
- [ ] Drop `client/easyconnect`, together with the packet stack's
      vendor-specific shutdown branch it exists for; the stack keeps taking the
      `client.Client` interface unchanged
- [ ] Drop the CLI (`main.go`, `init.go`, and `configs` once nothing kept needs
      it), `mobile/`, `stack/tun/`, `service/shadowsocks.go`, the READMEs, the
      Dockerfile, the compose file and the plist
- [ ] Split `service` so the keep-alive and DNS pieces stand alone; keep
      `internal/hook_func` and `internal/ping` for now and re-evaluate them in
      Phase 3, when the exit-on-shutdown semantics leave the tree
- [ ] Delete the `replace` directive, the fork's `go.sum` lines, `go.work` and
      the PKGBUILD symlink step
- [ ] Keep `LICENSES/` intact, annotate `core/atrust/**` in `REUSE.toml` with
      the upstream copyright, and write `core/atrust/PROVENANCE.md`: source,
      commit, imported packages, dropped packages
- [ ] Bounded text pass over the imported tree: third-party identifier names,
      library names, version strings and provenance wording become protocol
      descriptions. Algorithms, constants and test vectors are untouched
- **Gates:** `diff -r` between the imported directory and the source tree shows
  only the import-path rewrite and the dropped files; the imported tests are
  green; the `go mod tidy` delta is exactly the expected subset; a live login,
  proxy and keep-alive run; `makepkg -fC`

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
