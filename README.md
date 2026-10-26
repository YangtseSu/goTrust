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
captcha, the settings window, profiles, file logging, autostart and the granted
resource list are implemented. The protocol path was verified against a real
gateway in Phase 9; the settings/profile/startup/resource paths are verified by
tests and headless smoke runs. A final live pass over the settings window is the
remaining step; see `.planning/` for the current phase.

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
go build -o gotrust .
go test ./...
reuse lint
```

Development needs Go 1.27 or later. The resulting binary needs no cgo. The tree
is REUSE-compliant (AGPL-3.0-or-later); `reuse lint` must stay clean.

## Configuration

Settings live in `config.toml` under the user config directory:
`~/.config/gotrust/config.toml` on Linux (`$XDG_CONFIG_HOME` honoured),
`~/Library/Application Support/gotrust` on macOS, and `%AppData%\gotrust` on
Windows. The file is written with owner-only permissions, because it can hold a
password.

The format is versioned TOML. Global sections hold the application, the proxy
listeners and logging; each `[[profiles]]` entry owns one account's gateway,
credentials, keep-alive and client-data path. Proxy ports stay global so that
switching accounts does not move the addresses other applications use.

Both listeners bind to the loopback interface only: `127.0.0.1`, `::1` or
`localhost`, with a port between 1 and 65535.
A wider bind such as `0.0.0.0:1080` is refused before a login, and the settings
window reports the same rule while saving.

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
# Empty means automatic: the only offered method, or a chooser when several
# are offered. A stored pair must match what the gateway still offers.
auth_type = ""
domain = "local"
store_password = true
password = "…"
# Written only when the user opts in from a TOTP challenge, and
# cleared with the password when store_password is false.
# totp_secret = "…"

[profiles.keepalive]
enabled = true
interval = "90s"
targets = []

[profiles.debug]
client_data_path = ""
```

An empty `socks5_bind` or `http_bind` disables that listener; at least one must
remain. `client_data_path` is per profile and defaults to
`client_data-<profile>.json` beside the settings (`默认` keeps the original
`client_data.json`), so one profile's stored ticket never restores another's
session. `--config` moves the settings file, and the account material follows
it: a temporary configuration never reads or overwrites the real account's
files. The client data file holds cookies and the sid ticket and is 0600 as
well. A profile rename moves its file and a profile delete removes it, both
only once the settings are saved.

A JSON `config.json` from the unreleased v1 layout is ignored; there is no
migration path. Delete it if it is still present. A file whose `version` is
newer than the build is refused rather than rewritten, and a key this build does
not know is refused too: a typo must not silently fall back to a default.

## Settings window

设置 opens a separate window; the main window stays a status and connection
page. The groups are:

* **配置** — choose, create, copy, rename or delete a profile. Saving makes the
  selected profile active; switching while online disconnects first, then
  reconnects when 自动连接 is on.
* **连接** — gateway, port, username, authentication method, login domain,
  password and whether to store it. These take effect on the next login.
  **获取认证方式** asks the gateway what it offers; the picker then stores the
  `authType`/`loginDomain` pair. 自动 uses the only offered method, asks when
  several are offered, and saves the answer. A stored pair the gateway no
  longer offers fails with the list instead of switching methods silently.
  A stored TOTP secret is a second credential: it is shown here with an 移除
  action, follows the store-password box, and the box's removal clears it from
  the file on the next save.
* **代理** — SOCKS5 and HTTP bind addresses. These take effect on the next
  connect.
* **保活** — enable, interval (default 90 s) and manual targets, one per line.
  Also next connect.
* **启动** — start at login, auto-connect, start hidden and auto-reconnect
  (1/2/4/8/16 s, five attempts at most).
* **终端管理** — the portal's own list of endpoints: trusted devices and the
  temporary login history, read through the live session. It shows the counts,
  both lists and the current device, offers 刷新 and 信任此设备/取消信任, and
  reads the lists whenever the window opens. Before a login it says so instead
  of showing an empty list.
* **高级** — verbose logging, the log directory with an open button, the
  per-profile client-data path, and the install identity with 打开配置文件.
  `[identity]` holds a device id, the OS family, an optional client version and
  an optional process name/path/version for the TCP destination request. The
  L3 environment reports goTrust's own binary instead of a preset one. The id is generated once and seeded
  into every profile's client data before a login, so switching accounts does
  not look like a second device. The default UA states what the client is
  (`goTrust (aTrust client; Linux x86_64)`); `os_family = "uos"` or `"windows"`
  sends the vocabulary those platforms use, and `user_agent` overrides the field outright. An empty process
  block keeps the client's own, and the file is edited by hand: a device id the
  portal cannot read is refused at startup.

The settings file has no JSON parser and no schema surprises: an omitted field
takes its default. `[identity]` holds the device identity; the rest of the file
is the profiles, the listeners and the app behaviour. 关闭 hides the window and discards the draft; reopening it
loads the saved settings again.

## Logging in

Fill in the gateway, port and username in 设置, save, then press 连接. The
authentication method is discovered before the login: 自动 uses the only method
the gateway offers, a stored pair is used as long as the gateway still offers
it, and several offered methods open a chooser whose answer is remembered.
`login_domain` in the file stays `local` by default; the settings window writes
the domain that belongs to the chosen method.

If the gateway asks for a click captcha, the window replaces the form with the
image: click the characters in the order the image asks for, then 提交. 撤销
removes the last click, 清空 clears them all, and 取消 abandons the login.

A one-time code (SMS, TOTP or RADIUS) opens a panel with the gateway's message
and a code field; 跳过以后的二次认证 is offered only when the gateway allows it.
A TOTP challenge also offers to store the Base32 secret: once saved (0600, in
the settings file), goTrust generates the code itself and the panel does not
appear again. The secret is stored only for a profile whose store-password box
is ticked; clearing that box removes the secret from the file with the next
save, and 连接 → 移除 deletes it without waiting for a new challenge.

An external login (CAS or OAuth2) opens the gateway's login page in the system
browser and asks for the callback URL the browser ends on. The paste is checked
against the portal's path and its `ticket` (CAS) or `code` (OAuth2) before it is
sent, so a wrong window fails in the panel with a reason.

If the gateway asks to bind this installation to the account, a 绑定此设备 panel
names the device and waits for 允许绑定 or 取消. Allowing continues the login and
puts the device in 终端管理; cancelling abandons the login instead of answering
for the user. `--no-ui` keeps the automatic answer, as the CLI has always sent.

Every prompt belongs to the login attempt: 断开, the attempt's five-minute
deadline or a cancelled login releases it instead of leaving a dialog open.
`--no-ui` cannot ask any of these questions and reports which one the gateway
wanted.

**Not tested live:** the verified account has no second factor beyond the click
captcha and no SSO endpoint, so the SMS, TOTP, RADIUS and CAS/OAuth2 paths are
covered by unit tests only. The primary method must be password, CAS or OAuth2;
a gateway whose only method is a primary SMS login is not supported because
goTrust does not collect a phone number.

## Resetting the password

A gateway with a password policy (the common 90-day expiry, and the first login
after an administrator set one) makes the login stop instead of finishing: the
window shows a 重置密码 panel with the gateway's message, the account, the
gateway's rule list and its own prompt. 旧密码, 新密码 and 确认密码 are required;
a gateway that also asks for a new account name adds that field, and one that
allows postponing the change adds 跳过本次修改. 取消 abandons the login like any
other prompt.

The rules the client can evaluate (length, character classes, the account name,
repeated characters) are checked before the panel closes, so a typo does not
cost a login attempt; the rest are the portal's to answer. The new password is
encrypted the same way as the login password and travels through the session the
login already opened. After the gateway accepts the change, the new password is
written into the profile (the settings file is 0600; with 保存密码 off it stays
in memory only) and the login continues in the same session.

While online, 设置 → 连接 → 密码安全 → 重置密码 opens the
重置密码 dialog: 旧密码, 新密码, 确认密码, the 密码规则 list, and 确定/取消.

## Tray and startup

The tray menu shows a disabled status line (`已连接 203.0.113.4`, `连接中`,
`失败: …`), the connect/disconnect toggle, a profile submenu, 显示窗口,
打开默认应用 (the first granted application's URL, opened in the default
browser), 设置… and 退出. Its tooltip repeats the status and the last
keep-alive result. Activating the icon (a double-click in GNOME's panel) shows
the main window; the menu stays on the panel's secondary click.

登录系统时启动 writes the desktop's start-at-login entry
(`App.SetOpenAtLogin`, an XDG autostart file on Linux) and the app reconciles it
at every launch. `start_hidden` opens only the tray; a launch caused by the
autostart entry is hidden as well. With 启动后自动连接, the app logs in when a
saved password exists; otherwise it shows the window with the reason instead of
failing silently. A captcha or a missing credential always reveals the window.

### Desktop integration

`data/gotrust.desktop` and the two hicolor icons (`gotrust.svg`,
`gotrust-symbolic.svg`) ship with the source tree and are installed by the
package, so the application menu, the dock and the tray show the same mark: a
blue tile with a white shield and a downward arrow. The tray gets a PNG embedded
in the binary (`data/icons/tray-64.png`, 64×64 for HiDPI panels) rendered from
the scalable SVG, so a packaged binary works without an icon directory beside
it. After editing the SVG, regenerate the PNG and rebuild:

```sh
go generate ./...   # needs rsvg-convert
go build -o gotrust .
```

## Reliability

With 断线后自动重连 enabled (off by default), goTrust retries a lost session with
a fresh login on the 1/2/4/8/16 s ladder, five attempts at most. The window and
the tray show `重连中 (2/5，4 秒后)` while it waits; 断开 stops the ladder. It
runs only when the gateway says the session is gone — the client has already
tried a renewal — and it stops at a captcha, one-time code or SSO callback: the
prompt appears in the window instead of being retried blindly. Without a saved
password the ladder never starts, and an expired session becomes a visible
failure (`会话已过期，需要重新登录`) that brings the window forward.

A session belongs to its connection and cannot be reused. A stored sid ticket
is device material, not a login: after 断开, or after the portal ends the
session, the next connection logs in again. Both atrust-quadlet and
zju-connect tried to revive a session from the stored ticket, and the portal
ended it anyway; the attempt only delayed the login. For the same reason
启动后自动连接 requires a saved password — stored client data alone is not
enough.

The main window and the tray tooltip show the last keep-alive probe: target,
HTTP status, transport error, or the DNS fallback. A `https://` target is
tunnelled with CONNECT and does not count as application traffic, so it does
not renew the session; use a plain `http://` address inside the VPN. Manual
targets are probed first, derived candidates after them. The interval has a
10 s floor because every probe is a real request; 90 s is the default.

## Logs

The application writes a line log to `$XDG_STATE_HOME/gotrust/gotrust.log`
(`~/.local/state/gotrust/gotrust.log` by default; `~/Library/Logs/gotrust` on
macOS, `%LocalAppData%\gotrust\logs` on Windows). The file is 0600, rotates at
5 MiB and keeps three older files. Passwords, cookies, session tokens and
captcha data are redacted before a line reaches the disk. The upstream aTrust
client logs through Go's standard logger, which is bridged to the same writer.

`--no-ui` mirrors the log to stderr, which is where a headless run's operator
looks. 高级 → 打开 opens the log directory from the settings window.

## Granted resources

After a login the main window lists what the account may reach. Portal
applications come first, with the group and launch method the portal reports and
an 打开 button when it gives a launch address; then domain rules, IPv4 addresses
and ranges with their protocols and ports, and DNS name→IP mappings. A single
host that allows TCP 443 (or 80) gets an `https://` (`http://`) link, and 复制
puts the URL or the address on the clipboard. Only `http` and `https` links are
opened, so a portal-supplied `file:` or custom scheme cannot launch a local
application.

The last snapshot is cached at `resources.json` beside `config.toml` (0600). It
holds intranet addresses but no credentials. `--print-resources` prints that
snapshot and exits without logging in, starting with when it was saved, for
which profile and through which gateway:

```sh
gotrust --print-resources
```

A `--no-ui` run prints the list it just fetched to stdout after connecting, so a
headless deployment can publish what the account was granted.

## Using it

Point a browser or an application at `127.0.0.1:1081` (HTTP) or
`127.0.0.1:1080` (SOCKS5). Hostnames are resolved inside the tunnel, so DNS
hijacking and fake-IP are unnecessary. Names the portal did not map go to the
DNS server the gateway named, over the packet stack; a TCP-only gateway has no
UDP path and uses the system resolver for them instead.
