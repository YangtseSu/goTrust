English | [中文](configuration.md)

# Configuration reference

goTrust stores its settings in `config.toml`, whose location depends on the system:

| System | Path |
|---|---|
| Linux | `~/.config/gotrust/config.toml` (honors `$XDG_CONFIG_HOME`) |
| macOS | `~/Library/Application Support/gotrust/config.toml` |
| Windows | `%AppData%\gotrust\config.toml` |

The file mode is 0600, because it may hold a password. The window's save bar writes this same file; editing it by hand also works, but the app reads it only at startup while it runs, so a save from the window replaces the file with its draft.

## Complete example

```toml
version = 4
active_profile = "work"

[[profiles]]
id = "…"
name = "work"
auto_reconnect = false

[profiles.server]
address = "vpn.example.com"
port = 443

[profiles.auth]
username = "alice"
auth_type = ""
domain = "local"
store_password = true
password = "…"

[profiles.app]
autostart = false
start_hidden = true
auto_connect = false

[profiles.proxy]
socks5_bind = "127.0.0.1:1080"
http_bind = "127.0.0.1:1081"

[profiles.handoff]
listen = "127.0.0.1:1082"

[profiles.logging]
verbose = false

[profiles.identity]
device_id = "…"
mac_addresses = ["…"]
os_family = "linux"

[profiles.keepalive]
enabled = true
interval = "90s"
targets = []

[[profiles]]
name = "personal"
# Every other block is omitted: an account takes the defaults it needs.
```

## One account is one complete configuration

Exactly one account is connected at a time, so every setting a session uses
belongs to a profile. The global section holds only the file's own framing.
Two accounts may therefore listen on different ports, log at different levels
and present different clients; the ones in use are always the active account's.

There are two ways to add an account (Settings → Profiles), and they differ in
what the gateway sees:

| Action | Device the portal records |
|---|---|
| New | A new device. The account gets its own `device_id` and hardware address on its first login, so the portal counts it as another machine |
| Duplicate | The device of the account it was copied from. Signing in to a second account from the same machine is one device to the portal, and the copy shares its `device_id` and hardware address |

Use New only for an account that really is another machine. Use Duplicate for a
second account on this one, then change the username and password. To turn a
copy into a separate device, edit `device_id` and `mac_addresses` by hand.

## Field descriptions

### Top level

| Field | Description |
|---|---|
| `version` | Configuration format version, currently 4. A file from version 3 or older is migrated on load: the shared settings are copied into every account. A version newer than the program is refused rather than rewritten |
| `active_profile` | Name of the account in use, matching the `name` of some `[[profiles]]` entry |

### `[[profiles]]`

Each `[[profiles]]` entry is one account, and carries everything that account
uses.

| Field | Default | Description |
|---|---|---|
| `id` | generated | Stable local identifier. Unchanged by renaming, never sent to the gateway, and used only to key local state |
| `name` | `默认` | Account name, also the name shown on the Accounts page and the stem of the account's login data file |
| `auto_reconnect` | `false` | Retry a lost session on the 1/2/4/8/16 s ladder, at most five times. Off by default: a full login rotates the device token and drops the account's other clients |
| `server.address` / `server.port` | — / `443` | Gateway address and port. The window has one Gateway field taking `host:port` |
| `auth.username` | — | Username |
| `auth.auth_type` / `auth.domain` | automatic / `local` | Authentication method and login domain. Empty means automatic: use the only method offered, and ask when several are |
| `auth.store_password` | `true` | Whether to write the password into the file |
| `auth.password` | — | The saved password; empty when `store_password = false` |
| `auth.totp_secret` | — | TOTP secret, written only when the user chooses to save it |
| `app.autostart` | `false` | Start when logging in to the system. Changing it on the Startup page updates the system autostart entry immediately |
| `app.start_hidden` | `true` | On startup show only the tray icon |
| `app.auto_connect` | `false` | Log in automatically after startup. Requires a saved password |
| `proxy.socks5_bind` / `proxy.http_bind` | `127.0.0.1:1080` / `127.0.0.1:1081` | Listen addresses; empty disables that listener. Loopback only, ports 1–65535, at least one enabled |
| `handoff.listen` | `127.0.0.1:1082` | Loopback listener serving `/proxy.pac`; empty disables it |
| `logging.verbose` | `false` | Emit protocol-level debug logs. The Logs page switch takes effect immediately |
| `keepalive.enabled` / `keepalive.interval` / `keepalive.targets` | on / `90s` / none | Application-traffic probes. Targets take precedence over the automatically derived ones (granted ports 80, 443 and 22, plus name-only grants) |
| `identity.device_id` | generated | 32-digit hexadecimal device identifier, seeded into this account's login data |
| `identity.mac_addresses` | generated | Hardware addresses the portal records for this account |
| `identity.client_version` | — | Client version reported in requests. Empty uses the default built into the program |
| `identity.os_family` | `linux` | `linux`, `uos` or `windows`. Selects the User-Agent wording; the latter two send the vocabulary the portal expects |
| `identity.os_arch` | host | Architecture name in the User-Agent |
| `identity.user_agent` | derived | Overrides the whole User-Agent |
| `identity.process_name` / `process_path` / `process_version` | client's own | Tunnel process block this account reports |

The PAC endpoint accepts only `GET` and `HEAD`, and regenerates the PAC for the
current session on every request without caching. While disconnected it returns
an all-`DIRECT` file. Both the proxy and PAC listeners can bind only to a
loopback address, and a change takes effect on the next start.

The window's Gateway field accepts `host:port`, `https://host:port`, a bare host (443), a pasted path or trailing slash, and a bracketed IPv6 address with or without a port. Other schemes, an empty host and ports outside 1–65535 are rejected. On save the field is split back into `server.address` and `server.port`.

`client_data-<name>.json` holds the account's cookies, device identity and session tickets, also with mode 0600. The path is derived from the account name and cannot be configured: two accounts sharing one file would overwrite each other's sign key and ticket. Renaming an account moves the file and deleting an account removes it, both only after settings are saved. Reset login data on the Settings page, or `gotrust-cli --reset-login-data`, removes it and makes the next login start from nothing: that is the recovery step for a stored record the portal no longer accepts.

## Compatibility

A file written by version 3 or earlier is migrated on load: the shared `[app]`, `[proxy]`, `[handoff]`, `[logging]` and `[identity]` settings are copied into every account, so the first launch after the upgrade listens where the applications already point and presents the device the portal already knows. The file is rewritten in the new shape on the next save. The old JSON format (`config.json`) is not migrated and is ignored if present, so it can be deleted. Unrecognized keys cause a refusal: a misspelled key should not silently fall back to a default.
