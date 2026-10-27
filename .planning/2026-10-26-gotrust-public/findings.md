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

## Dependency and dead-code review (2026-10-27)

Method: `go list -deps` on both entry points (all 20 `core/atrust` packages are
reachable, so no package is dead at that granularity), `go mod why -m` for every
suspicious module, and a caller search for every symbol proposed for deletion.

### The TUN-era subtree

`underlay` exists so a TUN device cannot capture the client's own underlay
traffic: it picks the interface that reaches the gateway, binds sockets to it,
re-detects when it changes, and can capture the result. goTrust installs no TUN,
and both call sites construct the dialer with an empty `Options`, which turns
`AutoDetect` off. Nothing in the tree can turn it on.

`go mod why` chains:

| Module | Pulled by |
|---|---|
| `mythologyli/sing-tun` | `underlay` (default-interface monitor) |
| `sagernet/sing` | `underlay` (`logger.NOP`) |
| `sagernet/netlink`, `sagernet/go-tun2socks`, `scjalliance/comshim`, `metacubex/gvisor`, `vishvananda/netns` | `sing-tun` |

Three package-level functions are the only remaining callers of the binding
machinery, and nothing calls them:

| Symbol | Why it looks used | Why it is not |
|---|---|---|
| `client.GetAuthInfoList` | the UI fetches auth methods | the UI goes through `session.DiscoverAuthMethods` → `(*auth.Session).GetAuthInfoList`; the package function builds its own dialer and takes no context |
| `client.SetTrusted` | the UI trusts a device | the UI goes through `Manager.SetTrusted` → `(*atrust.Client).SetTrusted(ctx, trusted)` on the live session |
| `newUnderlayDialer` | – | only the two functions above call it |

`ExcludeIP` exists only for interface selection, `internal/platformdialer` is
imported only by `underlay`, and `internal/keylog` only by the two wrappers. The
PCAP capture and the underlay DNS override are reachable only through the
wrappers' arguments, which no configuration exposes.

### DNS

The resolver is alive: `core/proxy` → `dial.Dial` → `resolver.Resolve`, then
domain resources (fake pool inside the stack), portal `dnsResource` entries,
cache, gateway DNS over the tunnel (UDP with a TCP fallback), and the secondary
resolver. The DNS *server* is not:

| Fact | Consequence |
|---|---|
| `Stack.SetupResolve` stores the server in a field the gvisor and tcptunnel stacks never read | the interface has no consumer |
| `CheckDnsHijack`/`HandleDnsMsg` were used only by `stack/tun`, which went with TUN mode | the server lost its reader |
| `ServeDNS` has no caller (upstream calls it from `main.go`) | nothing listens |
| `resolver.Resolve` asks for `ip4` only; AAAA falls to the secondary resolver | a v6-only internal name resolves outside the tunnel or fails |
| TTL fixed at 3600, no `custom_dns`, no listener bind, no TCP listener, single-answer A/AAAA only, and `NewDnsServer` pads its slice with nils before appending | gaps to weigh only if the listener is ever revived |

Upstream comparison: `EZ4Connect` is a Qt front-end that passes zju-connect's
own flags (`-remote-dns-server`, `-dns-ttl`, `-secondary-dns-server`,
`-local-dns-server`, `-dns-server-bind`, `-custom-dns`, `-disable-remote-dns`,
`-disable-keep-alive`, `-keep-alive-url`); it implements no DNS of its own.

Decision taken on 2026-10-27: delete the server code and claim nothing more. A
loopback listener needs a client that resolves names itself and then reaches the
proxy by IP; none exists. IPv6 is not a resolver-only change: `dial.Dial` and
`DialIPPort` route every IPv6 target to `dialDirectIP`, `ipresource` matches
through `To4()`, and the gvisor endpoint dials `interface{IPv4}`, so an AAAA
answer would leave the tunnel. Both stay out until a deployment needs them and
can be used to verify them.

Keep-alive: zju-connect defaults to a DNS lookup through the remote resolver
(`www.baidu.com`), which works only where the gateway counts lookups as
activity. goTrust deliberately sends real application traffic to a granted
target and keeps the DNS probe as a marked fallback that does not claim to hold
the session.

### Dependency freshness (2026-10-27)

`go list -m -u all`, against the 32 declared modules:

| Direct module | Pinned | Latest |
|---|---|---|
| `egoist/mygo` | v0.6.4 | v0.7.1 |
| `gvisor.dev/gvisor` | pseudo 2026-01-29 | pseudo 2026-10-27 |
| `miekg/dns` | v1.1.72 | v1.1.73 |
| `things-go/go-socks5` | v0.1.1 | v0.1.3 |
| `golang.org/x/sys` | v0.48.0 | v0.49.0 |
| `patrickmn/go-cache` | v2.1.0+incompatible | none: last tag, dormant |

Indirect modules with a newer minor: `purego`, `x/image`, `x/mod`, `x/net`,
`x/time`, `x/tools`, `x/exp`. The rest are already at their latest tag.
`sing-tun`, `sagernet/sing`, `netlink`, `go-tun2socks`, `comshim`,
`metacubex/gvisor` and `netns` disappear in Phase 6, and `netaddr` with its two
`go4.org` modules in Phase 8, so their versions are not a question.

Workflow actions are three majors behind: `checkout` v4 → v7, `setup-go`
v5 → v7, `upload-artifact` v4 → v7, `download-artifact` v4 → v8.

Cache replacement, checked 2026-10-27 (last release / last push):

| Module | Last release | Last push | Fit |
|---|---|---|---|
| `patrickmn/go-cache` | 2017-10-24 (v2.1.0) | 2023-11-20 | what is used now; dormant |
| `jellydator/ttlcache/v3` | v3.4.1, 2026-06-22 | 2026-10-10 | per-entry TTL, `NoTTL`, `Get`/`Set`/`Delete`; lazy expiry, no goroutine |
| `maypok86/otter/v2` | v2.4.0, 2026-10-19 | 2026-10-20 | W-TinyLFU with a capacity bound; more machinery than three lookups need |
| `dgraph-io/ristretto/v2` | v2.4.2, 2026-07-07 | 2026-09-21 | admission policy and tuning; same objection |
| `hashicorp/golang-lru/v2` | v2.0.7, 2023-09-29 | 2026-10-26 | expirable LRU; usable, older release than ttlcache |

The resolver needs get, set with the default TTL, set without expiry, and it
holds one entry per hostname. `ttlcache/v3` matches that directly; an in-tree
map with a TTL is the zero-dependency alternative if the module is not worth it.

### Address module

`inet.af/netaddr` is deprecated upstream in favour of `net/netip`. It appears as
`IPSet`/`IPSetBuilder`/`MustParseIP`/`MustParseIPPrefix` in
`client/atrust/{client,parse}.go`, `client/client.go` and one test stub, and
pulls `go4.org/intern` and `go4.org/unsafe/assume-no-moving-gc` with it.

### What this review did not remove

Earlier passes touched documentation, comments, one test-only guard and the CI
step that ran it. The login path, the auth-method discovery, the resolver, the
packet stack and the proxy are unchanged. The three symbols above are the only
code proposed for deletion, and their features stay on their method paths.

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
