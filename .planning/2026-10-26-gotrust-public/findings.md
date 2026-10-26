# Findings: goTrust restructure and publication

Evidence gathered before the plan was written. Numbers are measured on the tree
as it stood on 2026-10-26.

## Why the fork stops being a build dependency

| Measurement | Value |
|---|---|
| Upstream commits per month | 2026-08: 107, 2026-09: 5, 2026-10: 1 (last, 2026-10-08) |
| Our divergence from upstream | 84 commits, 65 files, `+5777/−693`, all linear on top of upstream |
| Commits that maintain a CLI this app never runs | 24 of 84 (README, CLI entry, config example, CI) |
| Commits that touch the packages we actually import | 47 of 84 |
| goTrust commits spent on pin bumps | 4 of 64 |

Reasons upstream cannot be the source of fixes: it is nearly idle, it has not
accepted our changes, and its remaining movement is on paths this app does not
exercise. Keeping the fork as a build dependency preserves a version boundary
that its only consumer crosses several times a day.

## What the dependency actually costs

The build is pinned through a `replace` to a fork commit. A local `go.work`
(untracked, therefore absent from any clone) redirects that module to a working
checkout, so local test runs and packaged builds execute different revisions
unless the pin is bumped by hand. `GOWORK=off go build ./...` passes, so the pin
itself is sound; the drift is between the two ways of building.

The fork's pinned tree passes `go build ./...` and `go test ./...` on the
packages we import, verified at the pinned commit.

## Import scope

Dependency closure: 22 packages, ~27.9k lines including 10.6k lines of tests.

| Package | Lines | Fate |
|---|---:|---|
| `client` | 178 | keep: resource types and the `Client` interface the stacks take |
| `client/atrust` | 9425 | keep |
| `client/atrust/auth` | 7161 | keep |
| `client/authchallenge` | 817 | keep |
| `dial` | 719 | keep |
| `resolve` | 895 | keep |
| `service` | 2629 | keep minus `shadowsocks.go` (264, third-party Apache-2.0 code) |
| `stack` | 18 | keep |
| `stack/gvisor` | 412 | keep |
| `stack/tcptunnel` | 86 | keep |
| `underlay` | 1251 | keep |
| `log` | 196 | keep |
| `internal/*` (7 packages) | 1869 | keep initially, re-evaluate after the CLI split |
| `client/easyconnect` | 2160 | drop |
| `stack/tun` | 1474 | drop: TUN is out of scope for this app |
| CLI: `main.go`, `init.go`, `configs` | 1238 | drop |
| `mobile` | 116 | drop |

`client/easyconnect` enters the closure because the packet stack matches one of
its sentinels. That sentinel belongs to another protocol and has no counterpart
on the aTrust path, so both the package and the branch go. The `Client`
interface stays as it is; only the second implementation disappears.

Rewrite cost if the module prefix changes: 229 import lines across 94 files in
the imported tree, and 32 lines across 13 files in the app.

## The cut that already exists

Measured with `go list -deps`: `config`, `session`, `proxy` and `logging` have
zero mygo and zero purego in their dependency graph. Only `ui/` (23 entries) and
the GUI entry point reach mygo. The core/UI boundary is therefore already real;
what is missing is a guard, not a design.

The app is 16k lines across 40 files (8.6k excluding tests), so the imported
tree will be roughly 60% of the repository. That is the price of owning the
protocol layer, and it is bounded by dropping the packages in the table above.

## Naming inventory

`goTrust` appears 91 times in Go sources, 24 times in `README.md`, 5 times in
`data/` and once in `go.mod`. Most Go hits are display strings, XDG paths, the
log file name and the client identity string; package names are already
lowercase.

Three classes, not one:

| Class | Example kinds | Rule |
|---|---|---|
| Project identity | module path, repository, README title | `goTrust` |
| Text a user reads | desktop `Name=`, window and tray text, dialogs | `goTrust` |
| Technical names | binaries, directories, XDG paths, log file, icon and desktop file names | lowercase |

The client identity string is a fourth case: it is transmitted to the gateway,
so it is behaviour, not a label. Leaving it unchanged keeps the bytes the
gateway sees identical to what it has already accepted.

## Client version literal

Current client versions are published at
`https://atrustcdn.sangfor.com/tools/clientVersion/index.html`. The version
literal the imported tests carry is a value available from that page, so it
stays as a configurable client version and the page is recorded in
`core/atrust/PROVENANCE.md` when the import lands.

## Options rejected

| Option | Why not |
|---|---|
| Keep the fork as the dependency | Every pin bump costs a commit, the local workspace overrides the pin, and the version boundary has a single consumer |
| A separate `atrust` repository | One Go consumer, co-evolving API; a second repository would add a tag-and-bump step to every cross-cutting change. Revisit only if a second program needs the layer |
| A nested module for `core/` | Reintroduces exactly the pinning cost this plan removes; the dependency guard on the front-end binaries is the real invariant |
| Reproducing another client's CLI | Its flags, TUN mode and shadowsocks service are ~3k lines of upkeep for functionality this project does not need |
| Keeping `--no-ui` after the CLI lands | Two headless implementations drift; one of them would be untested |

## Risks

| Risk | Mitigation |
|---|---|
| Phase 2 is the only high-risk phase: ~25k lines move into the tree | Review the import as a diff against the source tree, and treat the imported tests as the acceptance gate |
| Renaming paths breaks an installed copy | One documented directory move, and a package rebuild in the same step |
| A second front-end can fight the GUI over one profile's login state | Per-profile lock in `core`, taken by both front-ends |
| The web prompt opens a listener | Bind loopback only, single-use token, `no-store`, no external assets, tied to the login attempt's context |
