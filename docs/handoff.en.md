English | [中文](handoff.md)

# External program integration: PAC, ZeroOmega, Clash and Mihomo

goTrust does not create a TUN interface and does not change the system proxy. It only provides local SOCKS5/HTTP proxies and splits traffic according to the authorization: authorized destinations go into the tunnel, everything else is connected directly from the local machine. So any program that points at the proxy port is already "granted resources via goTrust, everything else direct" and needs no rules.

You only need the PAC and rule snippets in this document when you want "only the browser goes through goTrust", or when coexisting with another proxy (Clash/Mihomo).

## Prerequisites

1. goTrust is logged in; the PAC and the rules come from that authorization.
2. The proxy listens by default on `127.0.0.1:1080` (SOCKS5) and `127.0.0.1:1081` (HTTP); both can be changed in the settings.
3. The tray and command-line versions use the same configuration; when using `--config`, add the same argument to the commands below.
4. PAC/rules are an authorization snapshot. After the authorization changes, the PAC follows the live session automatically; the Clash snippet and the full configuration are static and must be regenerated.

## PAC

While running, the PAC is served by default at `http://127.0.0.1:1082/proxy.pac`:

- GUI: tray or main window "Copy PAC URL".
- CLI: `gotrust-cli --pac-url` prints the URL; `gotrust-cli --print-pac` prints the script from the last login cache.
- In the settings file, `[handoff] listen = ""` disables the endpoint; a changed port takes effect on the next start.

ZeroOmega:

1. Create a new **PAC profile** and enter the address above as its URL.
2. Switch the browser to that profile.
3. The system proxy can also be set to this PAC URL directly.

Matching rules:

- a literal IPv4 address hit on an authorized prefix → proxy;
- a domain equal to or under an authorized suffix → proxy (the browser hands the domain to the proxy, goTrust resolves it inside the tunnel);
- `localhost`, loopback addresses, the gateway itself → direct;
- everything else → direct.

When disconnected, the PAC returns `DIRECT` for everything, so the browser does not hang while goTrust reconnects.

Boundaries:

- PAC does not cover UDP; QUIC/WebRTC bypass the proxy.
- PAC only knows the names that appear in the authorization (`Resources.Domains` and the domain→IP mapping delivered by the gateway). If a resource is authorized only by IP and you access it by domain, the PAC does not know which IP that name maps to; point the browser globally at goTrust (or at Clash's fake-IP) and let goTrust resolve inside the tunnel.
- PAC returns an HTTP proxy (CONNECT with a domain); it falls back to SOCKS5 when the HTTP listener is off.

## Clash / Mihomo

Three outputs, pick one per client; no key renaming needed:

| Command | Usage |
|---|---|
| `gotrust-cli --print-clash` | Plain snippet: paste into `proxies`, `proxy-groups`, `rules` of the Mihomo/Clash configuration |
| `gotrust-cli --print-clash-script` | Global extension script (Script) for Clash Verge Rev |
| `gotrust-cli --print-clash-config` | Full local Mihomo configuration: importable as a local profile in any client (contains only goTrust) |

The GUI tray "Copy Clash fragment" submenu and the main window's `Clash fragment` / `Verge script` / `Full Clash config` buttons are equivalent to the three items above and use the live authorization, not the cache.

All three outputs share one set of rules, in this order:

1. goTrust processes (`gotrust` / `gotrust-cli`, including `.exe`) → `DIRECT`;
2. the gateway address → `DIRECT`;
3. authorized domains → `goTrust-routes`;
4. authorized IP ranges → `goTrust-routes` (`no-resolve`).

The first two prevent TUN from sending goTrust's own underlying TLS back into the proxy; do not delete them. There are no preset private ranges such as `10/8`: an authorization may be a public address, or it may sit inside a private range.

### Plain snippet (Mihomo core / Clash configuration)

Merge the output into the main configuration:

- `proxies` contains `goTrust` (SOCKS5, `udp: true`);
- `proxy-groups` contains `goTrust-routes` (`select`, members `goTrust`, `DIRECT`);
- `rules` must be placed before the original configuration's `MATCH`.

Subscription updates overwrite the main configuration; for long-term use, prefer the Merge below or a standalone configuration.

### Clash Verge Rev global extension script (Script)

**Do not use a Merge profile**: Verge Rev's Merge deep-merges the configuration, list keys are replaced as a whole, and the current version does not implement prepend keys such as `prepend-rules` / `prepend-proxies`. After putting the snippet into Merge, these keys stay in the running configuration verbatim and are ignored by Mihomo, `rules:` remains as in the subscription, and the rules do not take effect.

The correct approach is a Script:

1. Run `gotrust-cli --print-clash-script`.
2. Verge Rev → "Profiles/Subscriptions" page → **Script** under the global extension configuration → paste, save and enable 🔥.
3. The script prepends the `goTrust` proxy, the `goTrust-routes` group and the rules to every profile; subscription updates are unaffected.
4. Using each profile's own Script has the same effect but applies only to that profile; re-enable it once after editing.

The script first removes old proxies/groups/rules with the same names, then prepends them back, so running it repeatedly does not create duplicates. The rule order is still process/gateway direct → authorized domains → authorized IPs.

Alternative: each Verge Rev profile has three files, "extension rules / extension proxies / extension proxy groups", in `prepend:` / `append:` / `delete:` list format; splitting the `--print-clash` output into these three `prepend` sections also works and needs no script engine.

### Full configuration (local profile import)

`gotrust-cli --print-clash-config` outputs a runnable Mihomo configuration (`mixed-port: 7890`, `mode: rule`, ending with `MATCH,DIRECT`) that can be imported directly as a local profile. It contains only goTrust, not your original subscription nodes; it suits the "goTrust is the only exit" setup.

### FlClash

FlClash's **standard override can only append rules, it cannot add proxies or proxy groups**, so there is no YAML entry point where "pasting as-is adds nodes":

- To use goTrust standalone: import the `--print-clash-config` output as a local profile;
- To coexist with a subscription: manually add the SOCKS5 `127.0.0.1:1080` proxy in FlClash's custom configuration, or use its script override. Verge's global extension script only applies to Verge Rev.

### TUN and fake-IP

- With TUN: enable fake-IP and **do not put authorized domains into `fake-ip-filter`**. Once filtered, the domain becomes a real IP, `DOMAIN-SUFFIX` rules may not match, and domain-only authorizations stop working.
- System proxy only, no TUN: fake-IP is not needed.
- Do not also point the browser globally at goTrust: either let the browser go through Clash, or let the browser connect to goTrust directly and let Clash manage the other programs, to avoid an extra hop.

### Verification

- Access an intranet address and confirm on Clash's "Connections" page that a `DOMAIN-SUFFIX,...` or `IP-CIDR,...,goTrust-routes` rule matches and the proxy chain is `goTrust`.
- Access the public internet and confirm the original rules still match.
- After disconnecting goTrust, intranet access should fail rather than silently going direct.

### FAQ

| Symptom | Cause and fix |
|---|---|
| Intranet access does not hit goTrust | The destination is not in this authorization; log in again and regenerate. Or you access it by domain while the authorization only has IPs and TUN has no fake-IP |
| The rules have no effect at all | The snippet was put into a Merge profile (Verge does not implement `prepend-*`); use a Script or the "extension rules/proxies/groups" instead |
| No `goTrust` proxy after startup | The snippet port differs from goTrust's actual SOCKS5 port; change `port` in `proxies` |
| Name conflict with an existing entry | Rename your existing `goTrust` / `goTrust-routes` first |
| Rules disappear after a subscription update | The main configuration was edited directly; use the global extension script or a standalone local profile |
| Under TUN, goTrust itself cannot reach the gateway | The leading `PROCESS-NAME` and gateway direct rules in the snippet were deleted; keep them |
| Want an automatic bypass when goTrust is down | Change the group type from `select` to `fallback`, keeping the member order `goTrust`, `DIRECT` |

## Without an external program

```sh
ALL_PROXY=socks5h://127.0.0.1:1080 curl https://intranet.example/
HTTPS_PROXY=http://127.0.0.1:1081 git clone https://intranet.example/repo
ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:1080 %h %p" user@intranet-host
```

A systemd service can set `Environment=ALL_PROXY=socks5h://127.0.0.1:1080`. If a program does not honor these variables, use proxychains-ng to route only its TCP traffic through goTrust.

## Limitations

- IPv4 resources and rules only.
- The HTTP proxy is TCP only; UDP is available only over SOCKS5.
- No DNS listener: names are handed to the proxy by the browser/Clash and goTrust resolves them inside the tunnel. When a tool resolves a name itself and connects by IP, only IP authorizations can match.
- PAC and Clash rules do not change goTrust's routing: unauthorized destinations are always connected directly from the local machine by goTrust.
- When the tray and command-line versions run simultaneously (with different configurations), both claim `127.0.0.1:1082` for PAC by default; the one started later fails to bind and logs it, so change the port or leave it empty to disable it.
