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

## External proxy handoff: what the other programs need (researched 2026-10-28)

The demand behind the deferred dae phase: reach granted intranet resources, or
all traffic, through the tunnel without goTrust growing TUN mode, a system proxy
or elevation. The worked example is the old skeleton plan's phase 8, which only
emitted one dae config. The scope below is broader: one description of what the
tunnel can reach, rendered for the tools people actually run.

### What the data path already allows

| Fact | Where | Consequence for a handoff |
|---|---|---|
| `dial.NewDialer(…, alwaysUseVPN=false, …)` sends a granted destination through the tunnel and dials everything else directly from this host; `ErrACLDenied` exists because the gateway terminates the session on a non-granted destination | `core/session/session.go`, `core/atrust/dial/dialer.go` | A tool's "global" mode pointed at goTrust makes goTrust the splitter, not a full-tunnel VPN. The tool must not force everything into the tunnel; goTrust already refuses the part the gateway would punish |
| The listeners are SOCKS5 (CONNECT and UDP ASSOCIATE) and HTTP (CONNECT and forward); an ungranted UDP destination is dialled directly | `core/proxy/server.go` | TUN/global through goTrust carries TCP and granted UDP; the HTTP listener stays TCP-only, so PAC cannot carry UDP |
| Both listeners pass hostnames to the tunnel: SOCKS5 takes the name and resolves it with `proxy.Options.Resolver`; HTTP CONNECT forwards `host:port` unchanged | `core/proxy/server.go`, `core/session/session.go` | Name access works without touching the system resolver. Chromium sends the hostname to a SOCKS v5 proxy and resolves nothing locally; Firefox does the same with "proxy DNS when using SOCKS v5" |
| The resolver reaches the gateway DNS through the tunnel with its own `stack.DialUDP`/`DialTCP`, not through the proxy or the dialer's ACL | `core/atrust/resolve/resolver.go` | Nothing outside the process can use that path. dae's upstream schemes have no socks transport; sing-box can detour a DNS server through an outbound, but a query to the gateway DNS reaches the dialer as an ungranted destination and is sent direct, not through the tunnel |

Intranet *names* are therefore the hard part. Two mechanisms were considered:

1. fake-IP plus domain dialing in the tool (Clash/Mihomo fakeip + socks
   outbound): the tool hands the name to goTrust, which resolves it in the
   tunnel. No goTrust change; needs the tool's own DNS configured. Chosen.
2. a loopback DNS relay in goTrust that forwards raw queries through the tunnel
   to the gateway DNS (a byte relay, not a resolver). Not chosen: an address
   answer does not preserve a name-only grant, and the chosen targets do not
   need it.

### Config surfaces verified on 2026-10-28

- **dae 2.1.1** (installed here): `node { 'socks5://127.0.0.1:1080' }`,
  `group { … policy: fixed(0) }`, `routing { pname(gotrust) -> must_direct; … }`,
  `global { dial_mode: 'domain' }`. `domain(suffix: a, suffix: b)` and
  `dip(prefix, …)` are one rule each; rules are first-match, and the example
  config presets `dip(geoip:private) -> direct`, so intranet prefixes must come
  before it. `include { … }` exists, but an included file must stay under the
  entry config's directory, which the packaged service keeps root-owned in
  `/etc/dae`; goTrust cannot install its own fragment there.
- **sing-box / Throne**: Throne's `Custom (sing-box outbound)` profile takes one
  outbound (`{"type":"socks","server":"127.0.0.1","server_port":1080}`), its
  structured routing profile takes `ip:10.0.0.0/8` and `suffix:example.com`
  lines plus a default outbound in the UI, and `Custom (sing-box config)` runs a
  complete config. Throne rule-sets accept only `.srs` URLs, so a JSON rule-set
  URL helps sing-box itself, not Throne. sing-box route rules since 1.11 use
  `action`/`outbound`, `final` is the fallback, and a remote `rule_set` can use
  `format: source` (JSON) or binary.
- **ZeroOmega / SwitchyOmega**: a PAC profile takes a URL or a raw script; the
  PAC compiler emits `isInNet` only for literal-IP hosts and hostname matching
  for names (`omega-pac/src/conditions.coffee`). PAC has no port matching, which
  is harmless: goTrust enforces the granted port itself and sends the rest
  direct.
- **PAC**: `FindProxyForURL` returns `PROXY host:port` or `SOCKS5 host:port`;
  the script is fetched directly, not through the proxy it selects.

### What a handoff needs to describe

| Item | Source in the tree |
|---|---|
| SOCKS5 and HTTP endpoints | `config.Config.ProxyAddresses()` |
| Granted prefixes | `Resources.IPs` (`from`/`to` ranges, not prefixes) merged to the smallest CIDR set |
| Granted names | `Resources.Domains` keys (suffix-matched on label boundaries) and `Resources.DNS` names; wildcard `*.` becomes a suffix. `App.WebDomains` is app metadata and never a rule |
| Ports and protocols | `Resources.IPs`/`Domains`; only informational, goTrust's dialer is the authority |
| GoTrust's own process name | `gotrust`, for the loop guard every tool needs |

### Risks the design has to carry

| Risk | Mitigation |
|---|---|
| Tool captures goTrust's own TLS to the gateway and loops it back | Every rendering starts with a process rule to `direct`/`must_direct`; a test asserts it |
| Intranet prefixes precede the private-range preset | Renderer emits the prefixes before any `geoip:private`/`ip_is_private` line, with the reason in a comment; test asserts order |
| Intranet names do not resolve in the tool | The name must reach the proxy; a tool that resolves locally only matches IP grants. Fake-IP is the documented mechanism for TUN; PAC matches only names present in the grant |
| UDP silently disappears in TUN mode | Documented per target: PAC and the HTTP listener are TCP-only, SOCKS5 carries UDP |
| A generated Clash fragment goes stale when the grant changes | The PAC is served live; the Clash fragment is regenerated after a login that changes the grant |

## How the reference clients hand traffic to other programs (researched 2026-10-28)

### zju-connect: TUN inside the client, DNS hijack, fake-IP

| Option | What it does | Why it exists |
|---|---|---|
| `-tun-mode`, `-add-route` | Creates a TUN device, takes root, installs the granted routes | Full-tunnel use without a second program |
| `-dns-hijack` | Intercepts UDP/53 inside its own TUN and answers from the resolver (`service/dns.go`, `HandleDnsMsg`) | An application inside the TUN resolves names through the tunnel |
| `-fake-ip` | The resolver hands out addresses from `198.18.0.0/16`; the TUN stack maps the address back to the name before matching | aTrust grants can be domain-only; a real answer's address is not necessarily a granted address. The README states that with aTrust, TUN traffic by name is unreliable without hijack/fake-IP |
| `-dns-server-bind` | A standalone DNS listener (UDP, resolver-backed, A/AAAA only) | Other programs can point at the tunnel's resolver without using the TUN |
| `-remote-dns-server auto`, `-secondary-dns-server`, `-local-dns-server` | The gateway's own DNS through the tunnel, a fallback, and the resolver used to reach the gateway's name | Gateways may or may not issue DNS servers; the local one is for finding the VPN server itself |
| `-custom-dns`, `-custom-proxy-domain` | Manual name→address overrides and extra proxied names | Deployments whose grants are incomplete or whose split DNS is wrong |
| `-proxy-all` | Forces every destination into the tunnel | Dangerous with a gateway that terminates the session on a non-granted destination; the OpenWrt notes pair it with an external splitter |

The DNS server there is resolver-backed: each question calls `resolver.Resolve`, which applies the
gateway's domain rules, the server-issued name→address map, the remote DNS through the tunnel and
the secondary resolver; it answers A and AAAA only, `dns.NewRR` with TTL 0. It is not a forwarder,
which is why it still works on gateways that issue no DNS server, and why public names go to the
system resolver instead of the corporate one.

### EZ4Connect: no TUN, two documented ways to combine with Clash/Mihomo

`docs/ADVANCED_USAGE.md`:

1. The client as the main proxy: system proxy on, and a "direct proxy" sends everything the client
   does not tunnel to Clash. This mirrors goTrust's split dialer.
2. Clash/Mihomo as the main proxy: a SOCKS5 proxy entry pointing at the client, a proxy group, and
   rules: auth/VPN host `DIRECT`, the campus suffix and a private CIDR to the group, and
   `PROCESS-NAME` of the client and its core before the suffix rule. With Clash TUN, it adds
   `fake-ip-filter` for the campus suffix, because a fake address defeats the `IP-CIDR` rule.
   `route-exclude-address` must not contain the campus prefix.

The rule sets there are written by hand and are deployment-specific (one campus suffix, `10/8`).
Notably the same rule shape is repeated in another GUI's README, so this is the de facto pattern:
process exclusion first, name rules next, address rules last, and the client does the final split.

### What this means for goTrust

- The external-tool direction is sound and already the documented practice; goTrust can generate
  the rules from the live grant instead of asking the user to copy a campus suffix and `10/8`.
- Fake-IP is the mechanism that preserves a *name* across a tool's TUN. Without it, the tool hands
  an address to goTrust and a domain-only grant cannot match. Generated configurations should
  therefore (a) route the grant's domain suffixes to the goTrust outbound and (b) tell the user to
  keep those names out of the tool's fake-IP filter, or better, not filter them at all.
- A loopback DNS server is the established shape for a tool that resolves locally, but it answers
  with an address, and an address loses a name-only grant. It is therefore not part of the chosen
  scope: both chosen targets deliver the name to the proxy, and goTrust resolves it through the
  tunnel.
- zju-connect's fake-IP warning confirms the crux: an aTrust grant that names a domain is matched
  by name at the gateway (`AddrPretend` in `tcptunnel.go`), so the tool must not pre-resolve.
- A generated rule set must never be derived from `App.WebDomains` alone: that field is the
  portal's launch hint, not a grant. Only `AddressList` entries become resources.

## aTrust deployments: what varies (researched 2026-10-28)

Evidence: the in-tree parser (`core/atrust/client/atrust/parse.go`), the reference clients' options
and issue tracker, and the operator's own gateway logs.

| Dimension | What varies | Consequence for goTrust and the handoff |
|---|---|---|
| Server version | aTrust 2.5.10 and 2.5.16 seen in the field; login replies differ in optional blocks | Parse defensively; every optional block already degrades to "not available" |
| Login | Domains `Radius`, `local`, `openldap`, …; `auth/psw`, `auth/cas`, `auth/smsCheckCode`, TOTP; click captcha, SMS, external SSO; device binding; forced password change | Nothing to do with routing; the handoff must not assume a login shape |
| Access model | Only apps whose `AccessModel` is `L3VPN` produce address rules; other models are skipped | A portal app can appear in the list yet be unreachable through the tunnel. The handoff snapshot must come from parsed rules, never from app names or `WebDomains` |
| Address shape | Single address, CIDR, `from-to` range, domain, port or port range, protocol `tcp`/`udp`/`all` | Prefixes must be computed from ranges; domains are label-suffix rules; ports decide what the gateway tunnels and what goTrust sends direct |
| Address family | IPv4 only in the parser; IPv6 entries are logged and skipped | Handoff rules are IPv4 |
| Public addresses | A ZTNA deployment grants domains that resolve to public addresses (institutional access), and the login reply carries the name→address map | Never assume private ranges in generated rules; it is correct to route a public address through goTrust |
| Name→address map | `AddressList[].IP` adds both a DNS rule and an IP rule | For such deployments the tool can resolve the name anywhere; the address is granted |
| `AddrPretend` | Whether the TCP tunnel reports the name or the already-resolved address to the gateway | A tool that pre-resolves can lose a name-only grant; keep names in the tool |
| `EnableTCPPrefL3` | Per app: TCP prefers the L3 tunnel instead of the per-connection TCP tunnel | Transport choice inside goTrust; invisible to the handoff |
| DNS issue | The login reply may carry zero, one or two DNS servers (`DNSOption`, `DNSOptionV2`) | The chosen handoff does not need them: the browser or Clash sends the name to the proxy and goTrust resolves it through the tunnel. A tool that resolves locally only matches IP grants |
| Session lifetime | An idle aTrust session on the operator's gateway expires in ~10 min; only real application traffic extends it; renewal is not traffic | A TUN user with no traffic still needs goTrust's keep-alive; document it |
| Nodes | Multiple node groups, WAN/LAN addresses, `{{sdpcHost}}`, default port 441, best-node refresh | Only affects goTrust's own connection, not the handoff |
| ACL | A non-granted destination forced into the tunnel ends the session (gateway SHUTDOWN) | Global mode through goTrust is a split, not a full tunnel; do not set anything equivalent to `proxy_all` |

### Correction to an earlier assumption in this file

`things-go/go-socks5` v0.1.1+ implements UDP ASSOCIATE, and the injected dial is called with
`network == "udp"`. A scratch test against `core/proxy` proved the relay works end to end: the
SOCKS5 handshake, the associate reply, the datagram header, the injected dial and the echo all
behaved. So UDP through goTrust is available on the SOCKS5 listener (HTTP stays TCP-only), and a
tool's TUN can carry UDP to granted UDP resources; an ungranted UDP destination is dialled direct
from this host, which is what the reference clients also do and what their issue tracker calls
intended behaviour.

## Handoff scope decision (2026-10-28)

Chosen targets, and only these:

1. **Browsers and ZeroOmega through a PAC served on loopback.** The PAC is the
   only live artifact: a stable URL, generated per request from the current
   grant, so a reconnect or a new grant needs no paste.
2. **Clash/Mihomo and the GUIs that consume their config (Clash Verge Rev,
   FlClash) through a pasted rule fragment.** One format covers the family; TUN
   mode, global mode and fake-IP are the tool's own.

Why not the rest:

* **dae** has no fake-IP and its DNS upstream has no socks scheme, so an
  intranet name it must resolve would require goTrust to add a DNS listener.
  That is the only reason dae would cost more than a text renderer, and it is
  not worth a new service while Clash covers TUN.
* **sing-box and Throne** would be a third and fourth format for the same
  consumer group, with no capability the two chosen targets lack.
* The snapshot JSON endpoint, the scope toggle and the extra rule lists had no
  consumer of their own.

DNS handling, which is the only non-obvious part:

* A browser request and a Clash request carry the hostname to the HTTP/SOCKS5
  proxy, and goTrust's resolver answers it through the tunnel. Intranet names
  therefore need no local DNS server.
* Clash in TUN mode needs fake-IP so the name survives until the connection
  reaches the proxy; the granted suffixes must not be placed in
  `fake-ip-filter`.
* A tool that resolves first and sends an address matches only the IP grants.
  That is documented as a boundary, not worked around with a DNS server.

Artifacts:

* One snapshot: proxy endpoints, merged IPv4 prefixes, name suffixes, the
  gateway host, and the online flag.
* `handoff.listen` serves `GET /proxy.pac`; the CLI prints the URL, the script,
  and the Clash fragment from the cached grant.
* The GUI copies the PAC URL from the tray and the main window.
* Rules never use a private-range preset and never encode ports; the grant is
  the authority and goTrust enforces it.
* Loopback only; no TUN device, no system proxy, no DNAT rule, no elevation.
