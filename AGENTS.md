# AGENTS.md

goTrust: an AGPL-3.0-or-later aTrust tray client. Go 1.27, mygo UI,
zju-connect fork. No TUN, no system proxy, no elevation.

## Build and checks

```sh
go build ./...
go test ./...
go vet ./...
gofmt -l .
reuse lint
```

`reuse lint` must stay clean; every new Go file starts with the SPDX header.
The Arch package builds from `~/projects/arch-pkgs/gotrust-git` with
`makepkg -fC`.

## Layout

- `config/`: TOML v2, profiles, atomic 0600 writes.
- `session/`: aTrust client, state, prompter, per-profile client data.
- `proxy/`: SOCKS5 and HTTP listeners over an injected dialer.
- `ui/`: pure view functions and the testable `ui.NewTester` views.
- `logging/`: rotating, redacted line log.
- `main.go`: wiring, tray, settings window, startup.
- Plan and history: `.planning/2026-10-23-gotrust-skeleton/`.

## Rules

- Views are pure: no network, no global state; render them in tests.
- mygo quirk: create and hide windows outside their own frame
  (`go func() { mygo.RunOnMain(fn) }()`). Creating or hiding a native window
  inside a view frame can leave it without a GL context or segfault in GTK.
  See `docs/agent-ui-testing.md`.
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
  `--mirror`, or push a `devel/*` ref.
- `devel/next` in this repository and in the `zju-connect` fork is local-only.
  Treat it as generic new-feature development; its notes are not part of the
  public record.
- Never mention `devel/next`, its B-items, or their research in commits, code,
  comments, docs or tests that reach `main`. Describe released behaviour and
  interfaces only.
- Never merge `devel/next` into `main`: its ancestry predates the public
  rewrite. Move one B-item at a time (rewrite, rebase/cherry-pick, new phase).
- A B-item enters `main` only when all of these hold:
  1. its code builds against the published fork pin, with no local `replace`;
  2. its notes are rewritten to behaviour/interface level and moved into
     `task_plan.md` as the next numbered phase, without the B number;
  3. no local paths, real addresses, upstream artifact names or third-party
     identifiers remain;
  4. `go build ./...`, `go test ./...` and `reuse lint` pass, and the commits
     are signed.
