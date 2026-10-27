# Task Plan: goTrust — self-contained core, two front-ends, public repository

## Goal

goTrust stops depending on an external fork at build time. The aTrust protocol
layer lives in this repository under `core/`, the mygo GUI and a headless CLI
are two independent front-ends over that core, and every commit is publishable
as it stands because the repository is going public.

Naming rule: the project and everything a user reads stays `goTrust`; binaries,
directories, XDG paths and packaging names are lowercase.

The scope up to Phase 9 was rearrangement and trimming of code that already
works. No new protocol capability was added, and the behaviour of the login path
stayed what the gateway has already accepted. Phase 10 is the first new scope:
it lets an external program use the tunnel, and it still adds no protocol
capability, no TUN device, no system proxy and no elevation.

## Next Step

Phase 10 is code complete. Local gates and the PAC/Mihomo smoke pass; the
live-login gate needs an operator run on a gateway that grants domain
resources. Phase 9 is complete: CI is green, the installed `r105` package
passed its live login, and Dependabot's security alerts are on with one grouped
update run a day per ecosystem.

## Current Phase

Phase 10 is code complete, narrowed to two targets: browsers and ZeroOmega
through a PAC served on loopback, and the Clash/Mihomo family (Clash Verge Rev,
FlClash) through a pasted rule fragment. No DNS listener is added: the chosen
targets send the name to the proxy and goTrust resolves it through the tunnel;
dae is out because it has no fake-IP and would force one.
Phase 9 is code complete. Every declared module is current: the patch/minor set,
mygo 0.7.1, gvisor's newest buildable `go` branch commit, and ttlcache in place
of the dormant cache. The workflows use the current action majors, Dependabot
has one grouped pull request a day per ecosystem, and the security alerts are
on. The push ran CI green and the installed package passed its live login.
Phase 8 is complete. The deprecated address module is gone: the `IPSet` accessor,
its builder and the interface method are deleted rather than reimplemented on
`net/netip`, because the set's only reader left with TUN mode and destination
matching already runs on `ipresource.Index`. `inet.af/netaddr` and the two
`go4.org` modules left the graph with it.
Phase 7 is complete. `service/dns.go` and `internal/zcdns` are deleted, the
`SetupResolve` method is gone from the stack interface and both implementations,
`core/session` no longer builds a DNS server, and `docs/development.md` records
the resolver's order and bounds. `miekg/dns` left the module graph with it.
Phase 6 is complete. The TUN-era subtree is gone: `underlay`,
`internal/platformdialer` and `internal/keylog` are deleted, the three one-shot
wrappers that were their only entry points are deleted, `client.UnderlayDialer`
is one method and `net.Dialer` satisfies it, and `go mod why -m` answers "main
module does not need" for `sing-tun`, `sagernet/sing`, `sagernet/netlink`,
`go-tun2socks`, `comshim`, `metacubex/gvisor` and `vishvananda/netns`.
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

After publication the imported layer was reviewed for what goTrust never runs.
The result is Phases 6 to 9: the TUN-era binding and monitoring subtree with its
seven modules, the DNS server code that lost its only reader when TUN mode was
dropped, the deprecated address module, and a refresh of every declared
dependency including the workflow actions. Everything else the review found is
in "Not doing" with the reason. Evidence is in `findings.md`.

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

### Phase 6: Remove the TUN-era dependency subtree

The imported layer still carries the parts that exist so a TUN device cannot
capture its own underlay traffic: socket binding per interface, default-interface
monitoring, DNS override for the underlay and a PCAP capture of it. goTrust
installs no TUN and constructs the dialer with zero options, so none of that
runs. The only entry points that could switch it on are three one-shot wrappers
that nothing in the tree calls. The login and auth-method discovery keep their
own paths through `core/session`.

- [x] `client/atrust/client.go`: delete the package-level `GetAuthInfoList`,
  `SetTrusted` and `newUnderlayDialer`, and the `ExcludeIP` call in the login
  path. The features themselves stay: they run through
  `(*auth.Session).GetAuthInfoList` and `(*Client).SetTrusted`.
- [x] `client/atrust/l3tunnel.go`: delete the `ExcludeIP` call on a virtual-IP
  update.
- [x] `client/client.go`: drop `ExcludeIP` from `UnderlayDialer`, which leaves
  `DialContext`; `net.Dialer` satisfies it.
- [x] `underlay/`: delete the package (binding, detection, retry, PCAP, local
  DNS). `core/session` builds `net.Dialer` at its two call sites and drops the
  dialer field and the two `Close` calls.
- [x] Delete `internal/platformdialer` and `internal/keylog`. The
  `TLSKeyLogWriter` option stays: it is an `io.Writer` and needs no package.
- [x] Tests: the `underlay` tests go with the package; `client_options_test.go`
  and `tls_test.go` use `net.Dialer`; the two dialer fakes lose `ExcludeIP`;
  `underlay_api_test.go` asserts `*net.Dialer` instead.
- [x] `go mod tidy`; `go mod why -m` must answer "main module does not need" for
  `mythologyli/sing-tun`, `sagernet/sing`, `sagernet/netlink`,
  `sagernet/go-tun2socks`, `scjalliance/comshim`, `metacubex/gvisor` and
  `vishvananda/netns`.
- [x] `core/atrust/PROVENANCE.md`: add the dropped packages and the reason.
- **Gates:** build, vet, gofmt, `go test ./...`, `reuse lint`, darwin/windows/
  arm64 cross-builds, one live login with both proxies serving, and a name
  resolved through the tunnel (the dial path is shared by login, discovery and
  keep-alive).
- **Status:** complete. The local gates ran: build, vet, gofmt,
  `go test ./...`, `go test -race` on `core/session` and `core/atrust/client`,
  `reuse lint` 185/185, and both binaries cross-built for darwin/amd64,
  darwin/arm64, windows/amd64, windows/arm64 and linux/arm64. The packaged build
  `gotrust-git r95.gcd088a6` was installed from the local checkout and its live
  login passed, which is the gateway-facing half of the gate; the package built
  with `namcap` reporting only the four documented warnings, and its binary
  carries no `sing-tun` module.
- **Risk:** this is the first change to the dial path since the import. The
  imported tests plus one live login are the acceptance gate; the module list is
  the proof the dependency subtree really left.

### Phase 7: Delete the dead DNS server code

The resolver stays; the server around it goes. The only reader of
`zcdns.LocalServer` was `stack/tun`, dropped with TUN mode, and `ServeDNS` has
no caller. Nothing listens today, so nothing can regress.

- [x] Delete `core/atrust/service/dns.go`, `core/atrust/internal/zcdns`, the
  `SetupResolve` method in both stacks, and the `NewDnsServer` call in
  `core/session`.
- [x] Keep the resolver as the only DNS path and write its bounds into
  `docs/development.md`: the gateway's server over the tunnel with a TCP
  fallback, the portal's own name entries, a 3600 s cache, the gateway's second
  server or the system resolver as the fallback, and IPv4 only.
- **Gates:** build, vet, `go test ./...`, `reuse lint`, and one live login that
  resolves an internal name through the proxy.
- **Status:** code complete. `miekg/dns` left with the server, so the module is
  gone from `go.mod` together with the `x/mod`, `x/sync` and `x/tools` it
  pulled in. Local gates ran: build, vet, gofmt, `go test ./...`,
  `reuse lint` 183/183, and both binaries cross-built for the five targets. The
  live login gate needs a gateway.

### Phase 8: Remove the deprecated address module

`inet.af/netaddr` is deprecated upstream in favour of `net/netip`. Its `IPSet`
answered "is this destination inside the VPN's address set" for the TUN stack;
after TUN mode was dropped the set had no reader, so the accessor, its builder
and the interface method go rather than being reimplemented on `net/netip`.
Destination matching already runs on `ipresource.Index`, which folds the same
`IPMin`/`IPMax` entries and additionally applies the port and protocol rules.

- [x] Delete `IPSet` from `core/atrust/client.Client`, the `ipSet` field and the
  method, the builder in `parseResource` (prefixes, ranges, single addresses and
  the node-address exclusion), and the session test stub's method.
- [x] `go mod tidy` drops `inet.af/netaddr`, `go4.org/intern` and
  `go4.org/unsafe/assume-no-moving-gc`.
- **Gates:** resolver and resource-parsing tests, one live login, and the
  granted-resource list rendering the same entries.
- **Status:** code complete. `go mod why -m` answers "main module does not need"
  for all three modules. Local gates ran: build, vet, gofmt, `go test ./...`,
  `reuse lint` 183/183, and both binaries cross-built for the five targets. The
  live login gate needs a gateway.
- **Risk:** the only changed behaviour is that no set is accumulated during
  parsing; nothing read it, so no decision can change.

### Phase 9: Refresh every declared dependency

Run after Phase 6, so the modules that disappear with the TUN subtree are not
chased first.

| Module | Before | After |
|---|---|---|
| `egoist/mygo` | v0.6.4 | v0.7.1 |
| `gvisor.dev/gvisor` | pseudo 2026-01-29 | pseudo 2026-10-27, `a68437cfc713` |
| `things-go/go-socks5` | v0.1.1 | v0.1.3 |
| `golang.org/x/sys` | v0.48.0 | v0.49.0 |
| `ebitengine/purego` | v0.11.1 | v0.11.2 |
| `golang.org/x/image`, `x/net`, `x/time`, `x/exp` | — | one minor newer each |
| `BurntSushi/toml`, `pquerna/otp`, `go-text`, `btree`, `barcode` | — | already latest |
| `patrickmn/go-cache` | v2.1.0+incompatible | replaced by `jellydator/ttlcache/v3` v3.4.1 |

- [x] `go get` for the patch/minor set, `go mod tidy`, `go mod verify`. The
  `go` directive stays at 1.27.3; no dependency demanded a newer one.
- [x] `mygo` 0.6.4 → 0.7.1. The release notes list no fix for either window
  hazard, and 0.7.1's source still defers `Close` but not `Hide`; the workarounds
  stay. The headless smoke rendered the main window and opened the settings
  window without a GL-context error, and SIGTERM exited 0.
- [x] `gvisor.dev/gvisor` moves to the newest commit that builds. The 09:48
  merge ships `pkg/tcpip/stack/bridge_test.go` as package `bridge_test`, which
  the Go loader rejects when `gonet` imports `stack`; the 09:58 merge removes
  that file. The packet stack tests pass, and one live login still proves the
  L3 path.
- [x] Actions: `checkout` v4 → v7, `setup-go` v5 → v7, `upload-artifact`
  v4 → v7, `download-artifact` v4 → v8. The inputs this repository uses are
  unchanged. CI run 40521378071 passed on `3b5d923` with the new majors.
- [x] Replace `patrickmn/go-cache`: the resolver's three calls map to
  `github.com/jellydator/ttlcache/v3`, with touch-on-hit disabled so a lookup
  does not extend the answer's life and `NoTTL` for the permanent entry. A zero
  TTL keeps entries forever, as before. `getDNSCache` / `setDNSCache` /
  `SetPermanentDNS` keep their shape, and their tests cover the TTL, the
  permanent entry and the zero-TTL case.
- [x] `.github/dependabot.yml`: daily, `gomod` and `github-actions`, one grouped
  PR per ecosystem, open-PR limits 3 and 2. Dependabot security alerts were
  enabled through the repository API (the endpoint answered 404 before and 204
  after). The first update run succeeded and opened no PR because every module
  is current.
- **Conclusion on PRs:** enable version-update PRs, not alerts alone. By now
  the fork modules are gone, so the remaining modules are normal releases
  Dependabot can move. A bot PR proves build, vet, tests and the cross-builds;
  it cannot prove the live login or the UI smoke, so merging stays manual and
  the open-PR limit stays low. Alerts stay on as well: they cover
  pseudo-versions (the gvisor module) and transitive modules, where no PR is
  possible.
- [x] Recorded: Dependabot does not bump pseudo-versions, so
  `gvisor.dev/gvisor` stays a manual bump; check it when this phase is revisited.
- **Commits:** the patch/minor set, the cache replacement, `mygo`, `gvisor`, the
  workflows and Dependabot each have their own commit, so a regression names its
  cause.
- **Gates:** build, vet, gofmt, `go test ./...`, `go test -race` on the touched
  packages, `reuse lint` 185/185, cross-builds, the UI smoke, the green CI run
  and the live login passed.
- **Status:** complete. The installed `gotrust-git r105.gfd3499a-1` and
  `gotrust-cli-git r105.gfd3499a-1` passed their live login; their binaries
  carry mygo 0.7.1, ttlcache and the pinned gvisor, and no `go-cache` or
  `netaddr`.
- **Risk:** the gvisor module is a branch commit, not a release. If the stack
  breaks later, pin the newest commit that passes and record why here.

### Phase 10: Browser and Clash-family handoff

Goal: a browser (through ZeroOmega or the system proxy) sends only the granted
intranet destinations to goTrust, and a Clash/Mihomo user can do the same or
route everything, without goTrust owning a TUN device, the system proxy or
elevation. Two targets only; anything else is a later phase.

Targets:

* **Browsers and ZeroOmega through a PAC served on loopback.** Covers the
  operator's own case (resources addressed by IP) and intranet names, because
  the browser sends the name to the proxy and goTrust resolves it inside the
  tunnel.
* **Clash/Mihomo and the GUIs that consume their config (Clash Verge Rev,
  FlClash) through a rule fragment pasted into an existing config.** One format
  serves the whole family.

Not in this phase: dae, sing-box, Throne, Xray/v2ray, a snapshot JSON endpoint,
a scope toggle, extra rule lists. Xray needs a whole JSON document, its core has
no TUN or system proxy, and a front-end owns the config anyway; sing-box/Throne
are a third dialect; dae is the one target that would force a DNS service (it
has no fake-IP and cannot reach the gateway resolver), so it stays out until
someone needs it. Clash/Mihomo answer the same TUN question with fake-IP and no
new listener.

Design constraints:

* Routing is per grant: granted destinations go through the tunnel, everything
  else is dialled directly. A tool's global mode sends everything to goTrust and
  goTrust splits; nothing forces a non-granted destination into the tunnel.
* Names must reach goTrust as names. HTTP CONNECT and SOCKS5 do that; a TUN tool
  needs fake-IP. A name-only grant is lost when the tool resolves first.
* No DNS listener. For both targets the name arrives at the proxy and goTrust's
  existing resolver answers it through the tunnel. A DNS server would only help
  a tool that resolves locally, which is the case this phase declines to paper
  over.
* The data path is IPv4; rules and PAC matching are IPv4 only.

**Snapshot**

- [x] `core/handoff`: `Snapshot{Generated, Profile, Gateway, TunnelIP, Online,
  SOCKS5, HTTP, Prefixes []netip.Prefix, Domains []string}` built from
  `config.Config` and `session.State`.
- [x] Prefixes: merge `IPRule.From`/`To` into the smallest IPv4 prefix set,
  dedupe and sort. Ports are ignored: goTrust enforces them and dials an
  ungranted port directly.
- [x] Domains: label-suffix set from `Resources.Domains` and `Resources.DNS`
  names, `*.` removed, lower-cased, deduped. `App.WebDomains` never becomes a
  rule.
- [x] `Snapshot.PAC()` and `Snapshot.Clash()` are pure and tested.

**PAC endpoint**

- [x] `handoff.listen` (default `127.0.0.1:1082`, empty disables) serves
  `GET /proxy.pac`; GET only, `no-store`, loopback only.
- [x] PAC: a literal IPv4 inside a granted prefix, or a host equal to or under a
  granted suffix, returns the HTTP proxy (`PROXY`) when it is configured, or
  the SOCKS5 proxy (`SOCKS5`) when only SOCKS5 is enabled; `localhost`, loopback
  and the gateway host return `DIRECT`; everything else `DIRECT`. Never
  `isInNet` on a hostname.
- [x] Generated per request from the live session state; disconnected serves a
  DIRECT-only script so the browser keeps working while goTrust reconnects.
- [x] The server runs for the life of each front-end, before the first login.
- [x] CLI: `--pac-url` prints the URL and `--print-pac` prints the script; both
  read the cached grant and the settings, no login.
- [x] GUI: a tray item "复制 PAC 地址" and the same action in the main window.

**Clash/Mihomo fragment**

- [x] `--print-clash` renders a paste-in fragment, `--print-clash-script` a
  Clash Verge Rev Script that prepends the same proxy, group and rules, and
  `--print-clash-config` a standalone config with a `MATCH` fallback. Each has a
  `socks5` proxy entry (`udp: true`), a selector group `goTrust-routes`, and
  rules in this order:
  `PROCESS-NAME,gotrust` and `gotrust-cli` (plus the `.exe` spellings) →
  `DIRECT`, the gateway host → `DIRECT` (`IP-CIDR,<ip>/32` when the profile
  address is an IP, `DOMAIN,<host>` when it is a name), granted
  `DOMAIN-SUFFIX` → the group, granted `IP-CIDR,…,no-resolve` → the group.
- [x] The fragment assumes the user's own fallback (usually `DIRECT`) and adds
  no private-range preset: a grant may be public, and a grant may sit inside a
  private range.
- [x] Docs: where to paste, the `fake-ip` requirement for TUN mode, and “do not
  put the granted suffixes in `fake-ip-filter`”.

**Docs and gates**

- [x] README: ZeroOmega PAC profile (URL), system proxy (PAC URL), Clash Verge
  Rev/FlClash (fragment + fake-IP), and a limits table: IPv4 only; the HTTP
  proxy is TCP-only and PAC cannot carry UDP; ungranted destinations go direct;
  a name-only grant needs the name to survive; the endpoint is loopback.
- [x] README also documents the zero-artifact usage: point one program at the
  existing SOCKS5/HTTP proxy (`ALL_PROXY`/`HTTPS_PROXY`, `curl --proxy`, an SSH
  `ProxyCommand`, a systemd `Environment=`). goTrust already splits by grant, so
  a consumer needs no rules at all; the PAC and the Clash fragment exist for a
  browser-only scope or for coexistence with another proxy that owns the default
  route.
- [x] README states the PAC boundary: it matches only names present in the grant
  (`Resources.Domains` and the gateway's name→IP map). An IP-only grant reached
  by a name needs the global proxy mode (browser profile or environment), or
  Clash fake-IP; PAC will not resolve an unknown name to test its prefix.
- [x] Tests: prefix merge (ranges, adjacency, duplicates), domain
  normalisation, PAC matching for suffix, literal IP, loopback and gateway,
  Clash rule order and `no-resolve`, offline PAC, bind failure, config
  validation.
- [x] Smoke: the PAC evaluated in Bun against eight representative URLs
  (suffix, literal prefix, gateway, loopback, non-matching); the live
  `/proxy.pac` served from `gotrust-cli` on a loopback port; the Clash fragment
  accepted by `mihomo -t`. A live login that grants domain resources still needs
  a gateway run.
- **Status:** code complete. gofmt, vet, `go test ./...` and `reuse lint`
  192/192 pass. The PAC, the live endpoint and the Clash fragment were exercised
  as above. The live-login gate (a grant with domain resources, a fetched PAC,
  and a proxied request) waits for an operator run.
- **Risk:** a static Clash fragment goes stale when the grant changes; the PAC
  does not. Regenerate the fragment after a login that changes the grant.
- **Risk:** PAC cannot carry UDP, so QUIC and WebRTC bypass the proxy; the docs
  say so rather than pretending the browser is fully covered.

## Not doing

Items that would add code nobody has asked for, or that this machine cannot
verify.

| Item | Why not |
|---|---|
| Local DNS listener (`ServeDNS` on `127.0.0.1`) | Not needed for the chosen targets: the browser or Clash sends the name to the proxy and goTrust resolves it through the tunnel. A DNS answer is an address, so it would not help a name-only grant anyway. dae would need one; that is why dae is not a target |
| A `--resolve` subcommand | Convenience, not a requirement |
| Remote AAAA lookups | The resolver asks `ip4` only, but the data plane is IPv4 throughout: `dial.Dial` and `DialIPPort` send every IPv6 target to `dialDirectIP`, `ipresource` folds addresses with `To4()` into a uint32 trie, and the gvisor endpoint dials `header.IPv4ProtocolNumber`. An AAAA answer would resolve an internal name and then connect around the tunnel. Revisit when a deployment grants IPv6 resources and the path can be exercised end to end |
| `dns_ttl` / `custom_dns` configuration | The TTL is a constant and the portal supplies the name entries; no deployment has asked for either knob |
| Resolving only granted names through the gateway DNS | Needs a campus split-horizon setup to measure, which this machine does not have |
| System-keyring credential storage | The settings file is already 0600 and stores a password only when the user ticks 保存密码; a keyring would only cover backups, sync and copied files, while `client_data*.json` keeps comparable material in the same directory. A Linux-only backend would leave macOS and Windows on the file even though their binaries are cross-built and never run on hardware. The design and the reopen conditions are in `../2026-10-23-gotrust-skeleton/findings.md` |

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
