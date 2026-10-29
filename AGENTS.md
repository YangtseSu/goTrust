# AGENTS.md

goTrust: an AGPL-3.0-or-later aTrust tray client. Go 1.27, mygo UI, the aTrust
protocol layer in the tree under `core/atrust/`. No TUN, no system proxy, no
elevation.

## Build and checks

```sh
go build ./...
go test ./...
go vet ./...
gofmt -l .
reuse lint
```

Two binaries: `gotrust` (`cmd/gotrust`, the tray app) and `gotrust-cli`
(`cmd/gotrust-cli`, the headless front-end). `go test ./...` includes
`internal/guards`, which fails if a core package reaches mygo or purego, if
either front-end reaches the other or `ui/`, if a front-end imports the
protocol layer instead of `core/session`.

`reuse lint` must stay clean; every new Go file starts with the SPDX header,
except in the imported tree, which is annotated in `REUSE.toml` instead of being
edited.
The Arch packages live in `github.com/YangtseSu/arch-pkgs` (`gotrust-git/`);
build them with `makepkg -fC` from that directory. The AUR release packages
(`gotrust`, `gotrust-cli`) are separate clones in `~/aur`; the update steps for
each release are in `docs/development.md`.

## Layout

- `core/`: the application packages. Nothing here imports mygo or purego, so a
  second front-end can be built against them without the GUI toolkit.
- `core/atrust/`: the aTrust protocol layer, imported from zju-connect and owned
  here now. `core/atrust/PROVENANCE.md` records the source commit and what was
  left out; do not edit the imported files for style, and keep the relative
  layout so a diff against the source still reads.
- `core/config/`: TOML v2, profiles, atomic 0600 writes.
- `core/session/`: aTrust client, state, prompter, per-profile client data.
- `core/prompt/web/`: the interactive login steps as single-use pages on
  `127.0.0.1`; the CLI's prompter, no toolkit.
- `core/proxy/`: SOCKS5 and HTTP listeners over an injected dialer.
- `core/logging/`: rotating, redacted line log.
- `core/profilelock/`: the per-profile instance lock both front-ends take.
- `internal/cli/`: the headless front-end; imports `core/*` only.
- `internal/gui/`: the tray front-end: windows, tray, event loop, prompt
  panels. It imports `core/session` and never the protocol layer.
- `cmd/gotrust-cli/`: the CLI's entry point.
- `cmd/gotrust/`: the tray app's entry point.
- `ui/`: pure view functions and the testable `ui.NewTester` views.
- `data/`: the desktop entry and icons, plus the Go file that embeds the tray
  PNG.
- Plan and history: `.planning/2026-10-23-gotrust-skeleton/` (the app) and
  `.planning/2026-10-26-gotrust-public/` (the restructure to a self-contained
  core and a public repository).

## Rules

- Views are pure: no network, no global state; render them in tests.
- A prompt page lives on the loopback interface only, under a single-use token,
  and never logs what was typed into it.
- mygo is upstream `github.com/egoist/mygo` v0.7.3, where a window created,
  shown, hidden or closed inside a frame is safe, `Hide` included (the
  `linux-hide-in-frame` fix for issue #680). If mygo moves to a release without
  it, hide a window from outside its own frame again through
  `go func() { mygo.RunOnMain(fn) }()` (0.7.1 and earlier crash in
  `gdk_cairo_draw_from_gl` on Wayland). See `docs/agent-ui-testing.md`.
- Never put real gateways, accounts, intranet addresses or client data in code,
  tests, commits or screenshots. Use `192.0.2.0/24`, `203.0.113.0/24`,
  `example.com`.
- `config.toml` and `client_data*.json` are gitignored and 0600.
- Do not push without explicit approval. Do not rerun checks a report already
  answered.
- UI smoke tests on this machine: `docs/agent-ui-testing.md`. Never move the
  pointer to the top-left corner (GNOME Activities).

## Branch policy

- Only `main` is published: `git push origin main`. Never `git push --all`,
  `--mirror`, or push any other ref.
- Work that is not ready for the public record stays on a local branch and is
  never pushed. Its notes are not part of the public tree.
- Work enters `main` one item at a time, rewritten to describe released
  behaviour and interfaces. Never merge a local development branch into `main`:
  its ancestry predates the public rewrite, so move one item at a time (rewrite,
  rebase/cherry-pick, new phase).
- An item enters `main` only when all of these hold:
  1. its code builds against the in-tree protocol layer, with no local
     `replace`;
  2. its notes are rewritten to behaviour/interface level and moved into
     `task_plan.md` as the next numbered phase;
  3. no local paths, real addresses, upstream artifact names or third-party
     identifiers remain;
  4. `go build ./...`, `go test ./...` and `reuse lint` pass, and the commits
     are signed.
