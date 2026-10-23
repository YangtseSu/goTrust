# goTrust

An aTrust client for Linux, macOS and Windows: a tray application that logs in
to a Sangfor aTrust gateway, keeps the session alive, and offers SOCKS5 and
HTTP proxy listeners on localhost.

It is deliberately narrow:

* **aTrust only.** No EasyConnect support.
* **No TUN device, no system proxy, no elevation.** The app runs entirely as
  the logged-in user. Applications reach the intranet by using the local proxy
  listeners (or a browser extension that points at them).
* **One gateway per profile, one live session.** Multiple accounts are profiles
  in one settings file; only one is connected at a time.

## Status

The session layer, the local proxy listeners, the tray, the in-window click
captcha, the settings window, profiles, file logging and autostart are
implemented. The protocol path was verified against a real gateway in Phase 9;
the settings/profile/startup paths are verified by tests and a headless smoke
run. A final live pass over the settings window is the remaining step; see
`.planning/` for the current phase.

## Why a new project rather than a fork

The protocol implementation comes from
[`zju-connect`](https://github.com/Mythologyli/zju-connect), which already
speaks aTrust, including the session-lifecycle and keep-alive fixes this app
needs. goTrust imports those packages and supplies its own wiring, because the
upstream `main` is a CLI: it parses flags, reads a TOML config and exits on
signals, none of which a tray app wants.

The dependency is pinned to a fork (`YangtseSu/zju-connect`, branch `dev`)
through a `replace` directive, so the protocol layer can move independently of
the application.

## Licence

**AGPL-3.0-or-later.** goTrust links `zju-connect`, which is AGPL-3.0; a
statically linked Go binary is a single combined work, so the combination is
AGPL. GPLv3 §13 permits combining a GPL work with an AGPL one, never the
reverse, so a copyleft-only licence is not available here.

If you run a modified goTrust as a network service, AGPL §13 requires you to
offer its source to the users of that service.

## Build

```sh
go build -o goTrust .
go test ./...
reuse lint
```

Development needs Go 1.27 or later. The resulting binary needs no cgo. The tree
is REUSE-compliant (AGPL-3.0-or-later); `reuse lint` must stay clean.

## Configuration

Settings live in `config.toml` under the user config directory:
`~/.config/goTrust/config.toml` on Linux (`$XDG_CONFIG_HOME` honoured),
`~/Library/Application Support/goTrust` on macOS, and `%AppData%\goTrust` on
Windows. The file is written with owner-only permissions, because it can hold a
password.

The format is versioned TOML. Global sections hold the application, the proxy
listeners and logging; each `[[profiles]]` entry owns one account's gateway,
credentials, keep-alive and client-data path. Proxy ports stay global so that
switching accounts does not move the addresses other applications use.

```toml
version = 2
active_profile = "默认"

[app]
autostart = false
start_hidden = true
auto_connect = false
auto_reconnect = false

[proxy]
socks5_bind = "127.0.0.1:1080"
http_bind = "127.0.0.1:1081"

[logging]
verbose = false

[[profiles]]
name = "默认"

[profiles.server]
address = "vpn.example.com"
port = 443

[profiles.auth]
username = "alice"
domain = "local"
store_password = true
password = "…"

[profiles.keepalive]
enabled = true
interval = "60s"
targets = []

[profiles.debug]
client_data_path = ""
```

An empty `socks5_bind` or `http_bind` disables that listener; at least one must
remain. `client_data_path` is per profile and defaults to
`client_data-<profile>.json` beside the settings (`默认` keeps the original
`client_data.json`), so one profile's stored ticket never restores another's
session. The client data file holds cookies and the sid ticket and is 0600 as
well.

A JSON `config.json` from the unreleased v1 layout is ignored; there is no
migration path. Delete it if it is still present. A file whose `version` is
newer than the build is refused rather than rewritten.

## Settings window

设置 opens a separate window; the main window stays a status and connection
page. The groups are:

* **配置** — choose, create, copy, rename or delete a profile. Saving makes the
  selected profile active; switching while online disconnects first, then
  reconnects when 自动连接 is on.
* **连接** — gateway, port, username, login domain, password and whether to
  store it. These take effect on the next login.
* **代理** — SOCKS5 and HTTP bind addresses. These take effect on the next
  connect.
* **保活** — enable, interval and manual targets, one per line. Also next
  connect.
* **启动** — start at login, auto-connect, start hidden and auto-reconnect
  (stored now; the reconnect ladder arrives in a later phase).
* **高级** — verbose logging, the log directory with an open button, and the
  per-profile client-data path.

The settings file has no JSON parser and no schema surprises: an omitted field
takes its default.

## Logging in

Fill in the gateway, port, username, login domain and password in 设置, save,
then press 连接. The login domain must match what the gateway's `authConfig`
offers, for example `local` or `Radius`; the upstream client does not guess it.
If the gateway asks for a click captcha, the window replaces the form with the
image: click the characters in the order the image asks for, then 提交. 撤销
removes the last click, 清空 clears them all, and 取消 abandons the login.

One-time codes and external SSO logins are not implemented yet; a gateway that
requires one fails with a message instead of blocking. Text captchas only occur
with EasyConnect, which this app does not support. `--no-ui` cannot show a
captcha and reports that the window is required.

## Tray and startup

The tray menu shows a disabled status line (`已连接 203.0.113.4`, `连接中`,
`失败: …`), the connect/disconnect toggle, a profile submenu, 显示窗口, 设置…
and 退出. Its tooltip repeats the status and the last keep-alive result.

登录系统时启动 writes the desktop's start-at-login entry
(`App.SetOpenAtLogin`, an XDG autostart file on Linux) and the app reconciles it
at every launch. `start_hidden` opens only the tray; a launch caused by the
autostart entry is hidden as well. With 启动后自动连接, the app logs in when a
stored password or a stored session exists; otherwise it shows the window with
the reason instead of failing silently. A captcha or a missing credential
always reveals the window.

## Logs

The application writes a line log to `$XDG_STATE_HOME/goTrust/goTrust.log`
(`~/.local/state/goTrust/goTrust.log` by default; `~/Library/Logs/goTrust` on
macOS, `%LocalAppData%\goTrust\logs` on Windows). The file is 0600, rotates at
5 MiB and keeps three older files. Passwords, cookies, session tokens and
captcha data are redacted before a line reaches the disk. The upstream aTrust
client logs through Go's standard logger, which is bridged to the same writer.

`--no-ui` mirrors the log to stderr, which is where a headless run's operator
looks. 高级 → 打开 opens the log directory from the settings window.

## Using it

Point a browser or an application at `127.0.0.1:1081` (HTTP) or
`127.0.0.1:1080` (SOCKS5). Hostnames are resolved inside the tunnel, so DNS
hijacking and fake-IP are unnecessary.
