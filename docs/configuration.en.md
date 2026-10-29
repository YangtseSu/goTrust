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
version = 3
active_profile = "默认"

[app]
autostart = false
start_hidden = true
auto_connect = false
auto_reconnect = false

[proxy]
socks5_bind = "127.0.0.1:1080"
http_bind = "127.0.0.1:1081"

[handoff]
listen = "127.0.0.1:1082"

[logging]
verbose = false

[identity]
device_id = "…"

[[profiles]]
id = "…"
name = "默认"

[profiles.server]
address = "vpn.example.com"
port = 443

[profiles.auth]
username = "alice"
auth_type = ""
domain = "local"
store_password = true
password = "…"

[profiles.keepalive]
enabled = true
interval = "90s"
targets = []

[profiles.debug]
client_data_path = ""
```

## Field descriptions

### Top level

| Field | Description |
|---|---|
| `version` | Configuration format version, currently 3. A version newer than the program is refused rather than rewritten in the old format |
| `active_profile` | Name of the configuration in use, matching the `name` of some `[[profiles]]` entry |

### `[app]`

| Field | Default | Description |
|---|---|---|
| `autostart` | `false` | Start when logging in to the system. Changing it on the 启动 page immediately updates the system autostart entry |
| `start_hidden` | `true` | On startup show only the tray icon, without opening a window |
| `auto_connect` | `false` | Log in automatically after startup. Requires a saved password |
| `auto_reconnect` | `false` | Reconnect automatically after the session drops: 1/2/4/8/16 seconds, up to 5 attempts |

### `[proxy]`

| Field | Default | Description |
|---|---|---|
| `socks5_bind` | `127.0.0.1:1080` | SOCKS5 listen address; empty disables this listener |
| `http_bind` | `127.0.0.1:1081` | HTTP proxy listen address; empty disables this listener |

Both listeners can bind only to loopback addresses (`127.0.0.1`, `::1`, `localhost`), with ports 1–65535. An address such as `0.0.0.0:1080` is rejected before login, and saving shows the same rule. Keep at least one listener.

### `[handoff]`

| Field | Default | Description |
|---|---|---|
| `listen` | `127.0.0.1:1082` | Loopback listener serving `/proxy.pac`; empty disables the endpoint |

The endpoint accepts only `GET` and `HEAD`, and regenerates the PAC for the current session on every request without caching. While disconnected it returns an all-`DIRECT` file. Like the proxy listeners it can bind only to a loopback address, and changes take effect on the next start. When the tray version and the command-line version run at the same time (with different configurations), both occupy `127.0.0.1:1082` by default; the one that starts later fails to bind and logs it, so change the port or leave it empty to disable it.

### `[logging]`

| Field | Default | Description |
|---|---|---|
| `verbose` | `false` | Emit protocol-level debug logs. The 详细日志 switch on the 日志 page takes effect immediately and writes here |

### `[identity]`

Usually there is no need to edit this by hand; 设置 → 本机数据 shows the device identifier read-only.

| Field | Description |
|---|---|
| `device_id` | 32-digit hexadecimal device identifier. Generated on first start and afterwards injected into every configuration's login data, so switching accounts is not seen by the portal as a new device. Changing it by hand to something the portal cannot read is rejected at startup |
| `client_version` | Client version reported in requests. Empty uses the default built into the program |
| `os_family` | `linux` (default), `uos`, `windows`. Determines the User-Agent wording; the latter two use the values expected by the portal |
| `os_arch` | Architecture name in the User-Agent |
| `user_agent` | Overrides the whole User-Agent directly |
| `process_name` / `process_path` / `process_version` | Tunnel process block. Empty keeps the client's own values |

### `[[profiles]]`

Each configuration corresponds to one account.

| Field | Description |
|---|---|
| `id` | Stable identifier of the configuration. Generated on first read and unchanged by renaming; it is used only locally and not sent to the gateway. Copying it into another configuration replaces it with a new value |
| `name` | Configuration name, also the name shown on the 多账户 page |
| `server.address` / `server.port` | Gateway address and port. The window has one 网关 field taking `host:port`; a bare host uses 443 |
| `auth.username` | Username |
| `auth.auth_type` / `auth.domain` | Authentication method and login domain. Empty means automatic: use the only method the gateway offers, and when several are offered, ask and remember the answer |
| `auth.store_password` | Whether to write the password into the configuration file |
| `auth.password` | The saved password; empty when `store_password = false` |
| `auth.totp_secret` | TOTP secret. Written only when the user explicitly chooses to save it in the one-time code panel; turning off password storage clears it as well |
| `keepalive.enabled` | Whether to send keepalive requests |
| `keepalive.interval` | Keepalive interval, `90s` by default, minimum 10 s |
| `keepalive.targets` | Manual keepalive targets, one per line, taking precedence over automatically derived targets |
| `debug.client_data_path` | Custom login data file path. When empty, `client_data-<配置名>.json` in the configuration directory is used |

The window's 网关 field accepts `host:port`, `https://host:port`, a bare host (443), a pasted path or trailing slash, and a bracketed IPv6 address with or without a port. Other schemes, an empty host and ports outside 1–65535 are rejected. On save the field is split back into `server.address` and `server.port`; the file format does not change.

`client_data*.json` holds cookies and session tickets, also with mode 0600. Renaming a configuration moves it and deleting a configuration removes it, both only after settings are saved.

## Compatibility

The old JSON format (`config.json`) is not migrated and is ignored if present, so it can be deleted. Files with version 2 gain an `id` for each configuration on first read and are written back as version 3; `id` is a local identifier and is never sent to the gateway. Unrecognized keys in the configuration cause a refusal: a misspelled key should not silently fall back to a default.
