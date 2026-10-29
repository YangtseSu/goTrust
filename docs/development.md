# Development

Everything here is for people working on the code. Users want the README, the
[configuration reference](configuration.md) and the [UI smoke recipe](agent-ui-testing.md).

## Status

The session layer, local proxies, tray, in-window prompts, the settings window,
profiles, logging, autostart, the granted-resource list and the `gotrust-cli`
front-end are implemented. The protocol path was verified against a real
gateway; the settings, profile, startup and resource paths are covered by tests
and headless smoke runs. The current phase and its history live in `.planning/`.

## Build and checks

```sh
go build ./...
go test ./...
go vet ./...
gofmt -l .
reuse lint
```

Go 1.27 or later. Neither binary needs cgo: the tray app loads GTK through
purego at runtime on Linux, and macOS/Windows use mygo's native backends. The
tree is REUSE-compliant (AGPL-3.0-or-later, with the imported layer annotated in
`REUSE.toml`); `reuse lint` must stay clean.

Two binaries:

```sh
go build -o gotrust ./cmd/gotrust         # tray app
go build -o gotrust-cli ./cmd/gotrust-cli # headless front-end
```

`go test ./...` includes `internal/guards`, which fails if a core package
reaches mygo or purego, if either front-end reaches the other or `ui/`, if a
front-end imports the protocol layer instead of `core/session`.

## Layout

- `core/`: application packages; nothing here imports mygo or purego.
  - `core/atrust/`: the aTrust protocol layer, imported from `zju-connect`.
    `core/atrust/PROVENANCE.md` records the source commit and what was left
    out. Do not edit the imported files for style, and keep the relative layout
    so a diff against the source still reads.
  - `core/config/`, `core/session/`, `core/proxy/`, `core/logging/`,
    `core/profilelock/`, `core/prompt/web/`: settings, login/session, listeners,
    log file, per-profile lock, and the CLI's loopback prompt pages.
- `internal/gui/`: the tray front-end (windows, tray, event loop, prompts).
- `internal/cli/`: the headless front-end.
- `cmd/gotrust/`, `cmd/gotrust-cli/`: entry points.
- `ui/`: pure view functions and the testable `ui.NewTester` views.
- `data/`: desktop entry, icons, and the Go file that embeds the tray PNG.

## Why the protocol layer is in the tree

The protocol implementation comes from
[`zju-connect`](https://github.com/Mythologyli/zju-connect), which already
speaks aTrust, including the session-lifecycle and keep-alive fixes this app
needs. It is imported once at a recorded commit and kept here, so a protocol
change and its consumer land in one commit instead of a pin bump per change.
`core/atrust/PROVENANCE.md` names the source, the commit and the dropped
packages; the imported tests are the acceptance gate for the import.

## Release

A release is one signed tag on `main`:

```sh
git tag -s v0.1.0 -m "goTrust 0.1.0"
git push origin v0.1.0
```

`.github/workflows/release.yml` repeats the checks on the tagged commit, builds
the archives, and creates the GitHub release with generated notes:

| asset | build |
|---|---|
| `gotrust_<ver>_linux_{amd64,arm64}.tar.gz` | binary, desktop entry, icons, licence |
| `gotrust_<ver>_darwin_universal.tar.gz` | amd64 and arm64 joined with `lipo` |
| `gotrust_<ver>_windows_amd64.zip` | GUI subsystem, so no console window |
| `gotrust-cli_<ver>_<os>_<arch>.*` | the headless binary, console subsystem |
| `checksums.txt` | sha256 of every archive |

The version a binary reports is injected at link time into
`internal/version.Version`; a source checkout says `devel`, and
`gotrust --version` / `gotrust-cli --version` print it.

Nothing is code-signed. macOS and Windows builds pass the cross-build matrix
but have never been run on those systems, so a signature would only hide that.
macOS users get Gatekeeper's unknown-developer refusal, Windows users get
SmartScreen's prompt; the release notes say so and nothing more.

### AUR

Two release packages are published from local clones in `~/aur`, one per AUR
repository:

| package | repository | installs |
|---|---|---|
| `gotrust` | `ssh://aur@aur.archlinux.org/gotrust.git` | tray app, desktop entry, icons |
| `gotrust-cli` | `ssh://aur@aur.archlinux.org/gotrust-cli.git` | headless binary |

Both build the `v<ver>` tarball and inject that version at link time. They are
release packages, not VCS packages: the `-git` pair stays in `arch-pkgs` for the
operator's own use, and the AUR packages conflict with it.

On every release, after the tag is pushed:

```sh
# in each of ~/aur/gotrust and ~/aur/gotrust-cli
sed -i "s/^pkgver=.*/pkgver=<ver>/" PKGBUILD
updpkgsums                        # refetches the tarball and rewrites sha256sums
makepkg --printsrcinfo > .SRCINFO
makepkg -f                        # builds, runs check(), packages
namcap PKGBUILD && namcap <pkgname>-<ver>-*.pkg.tar.zst
git commit -am "upgpkg: <pkgname> <ver>-1"
git push
```

`updpkgsums` downloads the new tag, so the release has to exist first. The only
namcap warnings expected are the two documented ones: libresolv from the
external linker, and GTK/appindicator because purego loads them at runtime.

## Packaging

The Arch recipe lives outside this repository, in
`github.com/YangtseSu/arch-pkgs` (`gotrust-git`): one checkout builds
`gotrust-git` (tray app; `gtk3`, `libayatana-appindicator`,
`hicolor-icon-theme`) and `gotrust-cli-git` (headless; glibc only), both with
`-trimpath`, PIE, FULL RELRO and the version injected. `makepkg -fC` from that
directory, then `namcap` on both packages: the only warnings are libresolv from
the external linker and the two dlopen-only GUI dependencies.

The desktop entry and icons come from `data/`, so a package installs exactly
what the repository holds. The tray PNG is rendered from the scalable SVG; after
editing the SVG:

```sh
go generate ./...   # needs rsvg-convert
```

## Session and keep-alive behaviour

A session belongs to its connection and cannot be reused. A stored sid ticket
is device material, not a login: after Disconnect, or after the portal ends the
session, the next connection logs in again. The gateway rejects a ticket
exchange while logged out, so Connect at startup requires a saved password.

The keep-alive sends a real HTTP request: a `https://` target is tunnelled with
CONNECT and does not count as application traffic, so it does not renew the
session. Manual targets are probed before derived candidates, and the interval
has a 10 s floor.

## Name resolution

goTrust runs no DNS listener: nothing it starts answers on port 53, and names
are resolved only for the lookups the proxies, the keep-alive and the login
make. `core/atrust/resolve` is the single resolver, and its order is:

1. the cached answer for the host — the first address returned, held for
   3600 s and swept 7200 s after it was written;
2. the portal's own name entries, rotating across the addresses it listed; a
   host that matches a granted domain resource also gets an address from
   `198.18.0.0/16`, which the dial path maps back to that resource;
3. the gateway's DNS server over the tunnel — UDP first, a TCP retry after the
   fallback delay, and TCP kept as the preference for a while after a UDP
   answer was lost;
4. the gateway's second server when the portal named one, otherwise the system
   resolver. Step 4 is the whole path when the gateway names no usable server.

Step 3 asks for IPv4 only, so an AAAA name is answered by step 4. The proxy
starts a connection only once the resolver has returned an address.

## Handoff to external programs

`core/handoff` turns one grant into a PAC and a Clash/Mihomo rule fragment. It
holds no socket: `handoff.Server` serves `GET /proxy.pac` from a `Source` the
front-end supplies, and `gotrust-cli` renders the same snapshot on demand.

- The snapshot merges `IPRule` ranges into IPv4 prefixes, collects the suffix
  set from `Resources.Domains` and `Resources.DNS`, and ignores ports: the
  dialer already enforces them and sends an ungranted port direct.
- `App.WebDomains` is a portal launch hint and never becomes a rule.
- PAC tests literal IPv4 against `isInNet` and names against `dnsDomainIs`; a
  hostname is never handed to `isInNet`, because that would invoke the browser's
  resolver. PAC has no UDP notion.
- Three Clash renderings share one rule set: `Clash` uses the plain
  `proxies`/`proxy-groups`/`rules` keys, `ClashScript` is a Clash Verge Rev
  Script that prepends the same proxy, group and rules to the runtime config
  (Verge's Merge replaces lists and does not implement `prepend-*`), and
  `ClashConfig` is a standalone config with a `MATCH,DIRECT` fallback for a
  local profile. All start with `PROCESS-NAME` for both binaries and with the
  gateway host, then the granted suffixes and prefixes; none adds a
  private-range preset.
- Both listeners are loopback only. The PAC server starts before the first login
  and serves a direct-only script while no data path is usable.
- No DNS listener exists. The browser or Clash sends the name to the proxy and
  the resolver answers it through the tunnel; a tool that resolves first only
  matches IP grants.

## Password change requests

Both password changes reuse the login password's encryption and the shared
request parameters, so the session's own key and counters are what the portal
expects. A forced change goes through the portal's forced-change endpoint; the
voluntary reset asks the portal's policy before it submits. Refreshing the
login configuration before encrypting replaces that state and is rejected.

## CLI prompt pages

`gotrust-cli` asks every interactive question through `core/prompt/web`: one
`127.0.0.1` page per question, under a fresh single-use token, served only while
the login waits. The URL goes to stderr (and, with `--print-url`, nowhere else);
the page is not logged, accepts one answer, rejects a request whose `Host` is
not a loopback name, loads no external assets, and closes with the login
attempt's context. A page works over SSH port forwarding.
