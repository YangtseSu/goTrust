English | [中文](README.md)

# goTrust

goTrust is the desktop client for aTrust. It stays in the tray, signs in to a
Sangfor aTrust gateway and keeps the session alive, while serving SOCKS5 and
HTTP proxies on the loopback interface for browsers and other programs that
need the intranet.

It runs on Linux, macOS and Windows. The interface has Chinese and English,
follows the desktop language by default, and can be fixed to one in
Settings → Language. The scope is deliberately narrow:

* **aTrust only**, not EasyConnect.
* **No TUN device, no system proxy changes, no administrator rights.** It runs
  as the current user; a program that needs the intranet points its proxy at
  goTrust's local ports.
* **One gateway per account and one live session at a time.** Multiple gateways
  live in one settings file as separate accounts, and only one is connected at
  a time.

## Installation

Download the archive for your platform from
[GitHub Releases](https://github.com/YangtseSu/goTrust/releases):

| System | File |
|---|---|
| Linux x86_64 / aarch64 | `gotrust_<version>_linux_amd64.tar.gz`, `gotrust_<version>_linux_arm64.tar.gz` |
| macOS (Intel and Apple Silicon) | `gotrust_<version>_darwin_universal.tar.gz` |
| Windows x86_64 | `gotrust_<version>_windows_amd64.zip` |

The headless version is on the same page, named
`gotrust-cli_<version>_<system>_<arch>`.

Unpack it and run `gotrust` (`gotrust.exe` on Windows). On Linux the system
also needs GTK3, libayatana-appindicator and hicolor-icon-theme; macOS and
Windows use the system's own interface components and need no extra runtime.

Arch Linux users can also install from the AUR; both packages build from the
release tags:

```sh
yay -S gotrust        # tray app; brings GTK3 and appindicator
yay -S gotrust-cli    # headless; needs only glibc
```

The released packages are not code-signed: macOS warns about an unidentified
developer and Windows shows SmartScreen. To build it yourself, see
[`docs/development.md`](docs/development.md); the interface and interaction
rules are in [`docs/design.md`](docs/design.md).

## First use

1. Start goTrust. By default only the tray icon appears; open the window from
   the tray.
2. The first window is Dashboard. Connect moves to the Account page. Enter the
   Gateway (`host:port`; a bare host uses 443) and the username. The password
   may stay empty — the login asks for it when the gateway uses a second
   factor or single sign-on. Save password writes it to the settings file
   (mode 0600).
3. Back on Dashboard, click Connect. When the gateway asks for an image
   captcha, the page shows the image.
4. After the login Dashboard lists the resources this account can reach, and
   the proxy addresses appear in the metrics row.
5. The other items are in the left navigation: the Account's sign-in method
   (Load methods asks the gateway in advance which methods it supports), the
   Proxy's listener addresses, Keep-alive, the Startup switches, and the
   language in Settings.

The language follows the desktop by default: a Chinese desktop shows Chinese,
an English one English, and other languages fall back to Chinese. Settings →
Language offers Follow the system, 中文 and English; the choice applies at once
and is kept for the next start. gotrust-cli's terminal output and logs are
English; the browser login pages are English by default, and `--prompt-lang zh`
switches only those pages to Chinese. The environment is not read.

The window supports the keyboard (on macOS, Command instead of Ctrl):
`Ctrl+Enter` connects or disconnects, `/` jumps to the resource search. While a
login panel is open, `Esc` means Cancel login, and `Ctrl+Enter` does not end
the login being answered. Settings is a page in the left navigation, with no
shortcut.

## Using the proxy

Default listeners:

| Protocol | Address |
|---|---|
| SOCKS5 | `127.0.0.1:1080` |
| HTTP | `127.0.0.1:1081` |

Change the ports on the Proxy page; they take effect on the next connection.
Names are resolved inside the tunnel, so no system DNS change and no fake-IP is
needed.

Command-line examples:

```sh
curl --socks5-hostname 127.0.0.1:1080 http://intranet-address/
curl -x http://127.0.0.1:1081 http://intranet-address/
```

In a browser or another program set the SOCKS5 proxy to `127.0.0.1:1080` (tick
"proxy DNS"), or the HTTP proxy to `127.0.0.1:1081`.

Both listeners bind loopback only; an external address such as `0.0.0.0:1080`
is refused.

## Splitting traffic to a browser or Clash (optional)

goTrust already splits by the authorization: point a program at the proxy ports
above and it needs no rules. To send only the browser through goTrust, or to
coexist with Clash/Mihomo:

- **Browser / ZeroOmega**: use the live PAC URL, by default
  `http://127.0.0.1:1082/proxy.pac`; the GUI and `gotrust-cli --pac-url` both
  give it.
- **Clash / Mihomo**: `gotrust-cli --print-clash` (plain fragment),
  `--print-clash-script` (Clash Verge Rev global extension script), and
  `--print-clash-config` (full local configuration). The Proxy page has the
  same three options.
- **Without an external program**: point the program at SOCKS5/HTTP, for
  example `ALL_PROXY=socks5h://127.0.0.1:1080`.

The steps for ZeroOmega, Clash Verge Rev and FlClash, fake-IP, the UDP limits
and the FAQ are in [`docs/handoff.en.md`](docs/handoff.en.md)
([中文](docs/handoff.md)).

## What the login asks

* **Image captcha**: the window shows the image; click the characters in order
  and submit. Undo, Clear or Cancel; Cancel abandons this login.
* **One-time code** (SMS, TOTP, authenticator): the panel shows the gateway's
  prompt and an input. When the gateway allows it, later second factors can be
  skipped. TOTP can also save its secret, and goTrust then generates the code;
  the secret is stored only for an account with Save password ticked.
* **External login** (CAS / OAuth2): the system browser opens the gateway's
  login page; after the login, paste the address the browser stopped on back
  into the panel. A wrong address is reported in the panel.
* **Binding a device**: when the gateway asks to bind this device to the
  account, a confirmation panel appears; Allow binding continues the login,
  Cancel abandons it.
* **Changing the password**: when the gateway requires a change (for example
  after 90 days, or the first login after an administrator reset), the login
  stops and shows the change panel; enter the current and the new password
  under the gateway's rules.

While the session is online, Change password on the Account page changes it
voluntarily. After that, sessions of this account on other devices must sign in
again.

## Pages

The left navigation is the whole interface; one window at a time:

| Page | Purpose |
|---|---|
| Dashboard | The current session, the last minute's up/down traffic, four metrics and granted resources; Connect/Disconnect is the button at the top right of the session card, and the resources card header has the search |
| Account | The Connection and Security cards: Gateway, Username, Password and whether to save it, Sign-in method, Login domain; Change password and the saved second-factor secret |
| Proxy | The SOCKS5 and HTTP listener addresses, and copying the PAC address and the Clash/Mihomo configuration |
| Keep-alive | Whether to send keep-alive requests, the interval, manual targets; takes effect on the next connection |
| Startup | Start at login, Connect at startup, Start hidden, Reconnect after a drop (5 attempts); takes effect at the next start |
| Terminals | The portal's device list for this account; offline it shows a 0-device placeholder, with Refresh, Trust this device and Stop trusting this device |
| Logs | The live tail of the current log: filter, Copy, Open folder, Clear and Verbose log |
| Settings | Language and Local data (the Data file and the Device ID) |
| About | Icon, name and version, tagline, Project home and License |
| Accounts | New, Duplicate, rename and delete accounts; select a row to edit, Save switches |

Account, Proxy, Keep-alive, Startup and the other pages share one draft:
Unsaved changes appears at the bottom, Save writes the file, Discard restores
it; hiding the window keeps the draft. Switch account disconnects the current
session first. The interface language (Follow the system, 中文 or English) is
switched on the Settings page, applies at once and is remembered. The ☾/☀ button at the
top right of the title bar switches the light and dark theme for the current
run.

The full field reference is in
[`docs/configuration.en.md`](docs/configuration.en.md)
([中文](docs/configuration.md)).

Keep-alive sends a real HTTP request to the intranet every 90 seconds by
default. Manual targets take precedence over derived ones. Keep-alive targets
must be `http://` addresses: an `https://` request goes through a CONNECT tunnel
and does not count as application traffic, so it does not renew the session.

Automatic reconnect after a drop is off by default. When on, it retries after
1/2/4/8/16 seconds, up to 5 attempts; a captcha or second factor stops the
retries and shows the panel. Without a saved password it does not reconnect,
because every reconnect is a full login.

## Tray

The tray menu is: the current status (`Connected 203.0.113.4`, `Connecting`,
`Failed: …`), Open Dashboard, Open default app, Connect/Disconnect, Switch
account (only with more than one account), Quit. Hovering shows the same status
and the last keep-alive result. On Linux the tray icon is an AppIndicator: with
a menu set, a click opens the menu (GNOME's extension waits out the
double-click time and then opens it on a single click); when the library has
the `activate` signal, a double click in GNOME activates the window and keeps
the current page; older libayatana-appindicator3 releases have no such signal,
so only the menu works. Open Dashboard in the menu always shows the window and
selects Dashboard. A second launch of goTrust does not open a second instance:
the new process hands its command line to the running one and exits at once;
the running window is restored, shown and focused, still on the same page.

Start at login writes the system autostart entry. With Start hidden also on,
only the tray icon appears after login, with no window. Connect at startup
needs a saved password; without one, the window opens and explains why instead
of failing silently.

## Resources

After the login Dashboard lists what this account can reach: portal apps (with
groups and how to open them), domain rules, IP addresses and ranges, and DNS
name mappings. When a single host allows TCP 443 or 80, a `https://` or
`http://` link is offered; it can be opened or copied in one click. goTrust
opens only `http`/`https` links; other schemes the portal offers are never
handed to a local program.

The most recent resource list is cached beside the settings file as
`resources.json` (mode 0600); it holds intranet addresses only, no credentials.
`gotrust-cli --print-resources` prints that cache.

## Logs

Log locations:

| System | Path |
|---|---|
| Linux | `$XDG_STATE_HOME/gotrust/gotrust.log` (default `~/.local/state/gotrust/gotrust.log`) |
| macOS | `~/Library/Logs/gotrust` |
| Windows | `%LocalAppData%\gotrust\logs` |

The file mode is 0600, one file is 5 MiB, and three history files are kept.
Passwords, cookies, session tickets and captcha contents are redacted before
they reach disk. Open folder on the Logs page opens the directory; Clear
empties the current log and its rotations.

## Command line

`gotrust-cli` is the headless version for servers and remote hosts. It reads
the same settings, signs in and serves the same proxy ports. When a login step
needs a person, it serves a single-use page on `127.0.0.1` and prints the
address (`--print-url` only prints it and opens no browser); answer the
captcha, one-time code or callback address there.

```sh
gotrust-cli [--config file]                  # sign in and stay up until Ctrl-C or SIGTERM
gotrust-cli [--config file] --status         # whether this account is in use, and by which process
gotrust-cli [--config file] --print-resources
gotrust-cli --pac-url                        # the PAC URL while running
gotrust-cli --print-pac                      # print a PAC from the cached grant
gotrust-cli --print-clash                    # plain Clash/Mihomo fragment
gotrust-cli --print-clash-script             # Clash Verge Rev global extension script
gotrust-cli --print-clash-config             # full local Mihomo configuration
gotrust-cli --json                           # resources and status as JSON
gotrust-cli --prompt-lang zh                 # Chinese login pages (terminal output stays English)
gotrust-cli --version
```

| Exit code | Meaning |
|---:|---|
| 0 | Finished normally; with `--status`, a process is using the account |
| 1 | Settings, login, listener or session failure |
| 2 | Invalid command line |
| 3 | `--status` found no process using the account |

Only one process may use an account at a time: the tray and the command-line
version can run together with different accounts; with the same account the
later one is refused, so two processes never share one login's data.

## Requirements and known limits

* Linux needs GTK3, libayatana-appindicator and hicolor-icon-theme; macOS and
  Windows use the system's own interface components.
* The macOS and Windows builds pass the compiler checks, but nobody has run
  them on real hardware yet. Please open an issue if you hit a problem.
* SMS, TOTP, RADIUS and CAS/OAuth2 second factors are covered by unit tests
  only and have not been checked against a real gateway.
* The primary methods are password, CAS and OAuth2. A gateway that offers only
  SMS as the primary method cannot be used with goTrust.
* There is no TUN mode and no system proxy change. For a global proxy, point
  the program or the system proxy at the local listeners; see above for
  browsers and Clash.
* A session lives only for its connection: disconnecting and connecting again
  is a new login, and the saved login data cannot restore the old session.
  Automatic connect therefore needs a saved password.

## Credits

* [zju-connect](https://github.com/Mythologyli/zju-connect): the aTrust
  protocol implementation; this project's protocol layer was imported from it
  and is maintained here.
* [EZ4Connect](https://github.com/chenx-dust/EZ4Connect): an important reference
  for the interaction and interface design.
* [mygo](https://github.com/egoist/mygo): the Go desktop framework behind the
  tray and the windows.

## License

**AGPL-3.0-or-later**. The linked protocol layer (`core/atrust/`) is AGPL-3.0;
the binary is one combined work, so the released executables are licensed under
AGPL-3.0. If you run a modified goTrust as a network service, section 13 of the
AGPL requires you to offer its source to the service's users.
