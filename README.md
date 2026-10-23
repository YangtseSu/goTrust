# goTrust

An aTrust client for Linux, macOS and Windows: a tray application that logs in
to a Sangfor aTrust gateway, keeps the session alive, and offers SOCKS5 and
HTTP proxy listeners on localhost.

It is deliberately narrow:

* **aTrust only.** No EasyConnect support.
* **No TUN device, no system proxy, no elevation.** The app runs entirely as
  the logged-in user. Applications reach the intranet by using the local proxy
  listeners (or a browser extension that points at them).
* **One gateway, one account.** Multi-account supervision is out of scope.

## Status

The session layer, the local proxy listeners, the tray window and the
in-window click captcha are implemented. Live verification against a real
gateway is the remaining step; see `.planning/` for the current phase.

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
```

Development needs Go 1.27 or later. The resulting binary needs no cgo.

## Configuration

Settings live in `~/.config/goTrust/config.json` on Linux
(`$XDG_CONFIG_HOME` honoured), `~/Library/Application Support/goTrust` on
macOS, and `%AppData%\goTrust` on Windows. The aTrust client data file sits
beside it and holds session material, so both are created with owner-only
permissions.

## Logging in

Fill in the gateway, port, username and password, then press 连接. If the
gateway asks for a click captcha, the window replaces the form with the image:
click the characters in the order the image asks for, then 提交. 撤销 removes
the last click, 清空 clears them all, and 取消 abandons the login.

One-time codes and external SSO logins are not implemented yet; a gateway that
requires one fails with a message instead of blocking. Text captchas only occur
with EasyConnect, which this app does not support. `--no-ui` cannot show a
captcha and reports that the window is required.

## Using it

Point a browser or an application at `127.0.0.1:1081` (HTTP) or
`127.0.0.1:1080` (SOCKS5). Hostnames are resolved inside the tunnel, so DNS
hijacking and fake-IP are unnecessary.
