# Provenance of `core/atrust`

This directory is the aTrust protocol layer goTrust runs. It was imported as a
whole from the `zju-connect` project so the build no longer depends on an
external module: the layer changes with the application that consumes it.

## Source

| | |
|---|---|
| Repository | <https://github.com/YangtseSu/zju-connect> |
| Branch | `dev` |
| Commit | `5caa3a1684a1b4b83c2337884084054a425d3ef3` |
| Module path there | `github.com/mythologyli/zju-connect` |
| Module path here | `github.com/YangtseSu/goTrust/core/atrust` |
| Upstream of that fork | <https://github.com/Mythologyli/zju-connect> |

The tree was taken from that commit, not from a working copy, and the paths
below it were kept as they are, so the import can be read as a diff against the
source tree. Only the module prefix in the import lines changed.

## Licence

The imported code is AGPL-3.0 under the source project's `LICENSE`. Its files
carry no license headers of their own, so `REUSE.toml` annotates the directory;
see the `core/atrust/**` annotation. `internal/ping` is the MIT-licensed ping
implementation by Cloverstd (2017) that the imported layer carries, annotated
separately.

## Imported packages

- `client`, `client/atrust`, `client/atrust/auth`, `client/authchallenge`
- `dial`, `resolve`, `log`, `service`, `underlay`
- `stack`, `stack/gvisor`, `stack/tcptunnel`
- `internal/hook_func`, `internal/ippool`, `internal/ipresource`,
  `internal/keylog`, `internal/ping`, `internal/platformdialer`,
  `internal/zcdns`, `internal/zctcpip`

## Dropped packages

| Package | Why |
|---|---|
| `client/easyconnect` | A second protocol this application does not speak. The packet stack matched one of its sentinels in its write path; the `client.Client` interface the stack takes is unchanged. |
| `stack/tun` | TUN mode is out of scope: goTrust runs unprivileged and proxies over loopback. |
| `main.go`, `init.go`, `configs` | The source project's CLI entry point, its flag parsing and its config type. goTrust wires the layer from `core/session`. |
| `service` listeners | The SOCKS5, HTTP, TCP/UDP forwarding and shadowsocks servers. `core/proxy` owns the listeners, and the imported `service` package keeps only the DNS server and the keep-alive. |
| `internal/hook_func` initial hooks | The start-up checks belonged to the CLI, and one of them named operator-specific DNS suffixes. The package keeps the terminal hooks the stacks call. |
| `client/authchallenge` terminal handler | The stdin handler and the temporary browser page it started duplicated what the front-ends own. Every interactive step now comes from `core/prompt/web`, and a caller without a prompt gets `UnsupportedHandler` instead of a read from a terminal that may not exist. |
| `mobile`, `docs`, `Dockerfile`, `docker-compose.yml`, the property list, the READMEs | Packaging and documentation of the source project. |
| `go.mod`, `go.sum` | goTrust is one module; the layer's dependencies are recorded in the root `go.mod`. |

## Changes made to the imported source

Beyond the module prefix and the drops above:

- Comments and identifiers that named another product were rewritten to
  describe the protocol behaviour instead. No algorithm, constant, field or test
  vector changed, so the imported tests keep their meaning.
- The tunnel process block defaults to the running binary's own name and path
  instead of the source project's binary. A caller that names a process, which
  is what `core/session` does, is unaffected.
- Test fixtures use the documentation ranges (`192.0.2.0/24`,
  `198.51.100.0/24`, `203.0.113.0/24`, `example.com`) rather than the hosts the
  source project happened to use.
- `auth.Session.Login` answers a challenge without a handler with
  `authchallenge.UnsupportedHandler`, which fails with the reason. The source
  fell back to its terminal handler; goTrust's front-ends always supply their
  own prompt.

## Client version literals

The default user agent in `client/atrust/auth` carries an aTrust client version.
The current client versions are published at
<https://atrustcdn.sangfor.com/tools/clientVersion/index.html>, so the literal is
a published value. It is only a default: the login
takes the user agent from its options, and goTrust always passes its own.
