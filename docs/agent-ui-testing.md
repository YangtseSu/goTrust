# UI testing on this workstation

Smoke tests for the native UI. The app is a tray application, and a crash looks
like an exit: always capture stderr.

Run the app as a **Wayland client of a headless mutter**. The operator's
desktop is a Wayland session, and the X11/GLX path hides Wayland-only failures
outright (see "Frame rules").

## Headless Wayland session

```sh
root=/tmp/gt-nested
mkdir -p "$root/runtime" "$root/config" "$root/state"
chmod 700 "$root/runtime"

dbus-run-session -- bash -c '
  root=/tmp/gt-nested
  export XDG_RUNTIME_DIR="$root/runtime"
  mutter --headless --no-x11 --wayland-display=wayland-gt \
    --virtual-monitor 1280x1024 &
  mpid=$!
  trap "kill $mpid 2>/dev/null" EXIT
  # the compositor is up once its socket exists
  for _ in $(seq 1 40); do [ -S "$root/runtime/wayland-gt" ] && break; sleep 0.1; done
  unset DISPLAY                       # or GDK picks X11
  export WAYLAND_DISPLAY=wayland-gt GDK_BACKEND=wayland
  XDG_CONFIG_HOME="$root/config" XDG_STATE_HOME="$root/state" \
    /usr/bin/gotrust --config "$root/config/config.toml"
'
```

- A Wayland client takes the EGL path the operator's GNOME session takes. The
  headless mutter needs a session bus, hence `dbus-run-session`; without one it
  aborts with `assertion failed: (session_bus)`.
- There is no screenshot tool for this display: mutter has no wlr-screencopy,
  so `grim` cannot see it. Check stderr and the exit code (a crash, a GL
  warning, `goTrust ready`, SIGTERM = 0). To look at the interface, render it:
  `GOTRUST_RENDER_DIR=/tmp/gotrust-ui go test ./ui -run TestRenderEveryState`.
- Temporary `XDG_CONFIG_HOME` and `XDG_STATE_HOME` keep the operator's config
  and credentials out of the test.
- `--config` is the same file the app reads. A profile with
  `auto_connect = true`, `store_password = true`, a password and a listener
  that accepts but never answers puts the window in `连接中`, which is how
  断开 during a login is exercised in a real window.
- Stop the app and the compositor when the check is done; nothing may outlive
  it.
- Tray and panel behaviour (icon, menu, double-click) still needs a real
  desktop; that check is the operator's, not a nested session's.

## The real desktop

The nested display cannot be screenshotted, so a picture of the app as the
operator sees it, and anything the panel or the tray owns, runs on the real
session with the tools below.

## GNOME hot corner

`(0, 0)` opens the GNOME Activities overview and steals focus. **Never move the
pointer to the top-left corner** and never click near it. Prefer keyboard
input; when a pointer is necessary, start from a window coordinate well away
from `(0, 0)`.

The desktop is 3840x2160 physical (2304x1296 logical, scale 1.6667) on this
machine. `ydotool mousemove --absolute` takes physical pixels here.

## ydotool

```sh
YDOTOOL_SOCKET=/tmp/ydotool.sock ydotoold --socket-path=/tmp/ydotool.sock &
export YDOTOOL_SOCKET=/tmp/ydotool.sock
```

Keycodes (Linux input): `1` Esc, `15` Tab, `28` Enter, `42` Left Shift,
`56` Left Alt, `62` F4.

```sh
ydotool key 15:1 15:0              # Tab
ydotool key 28:1 28:0              # Enter
ydotool key 42:1 15:1 15:0 42:0    # Shift+Tab
ydotool key 56:1 62:1 62:0 56:0    # Alt+F4
```

Prefer keyboard over the pointer: focus order is stable enough for buttons,
and it avoids the hot corner entirely. With the window focused, Esc cancels a
login panel; there is no second window to close.

Pointer input, only when needed:

```sh
ydotool mousemove --absolute 1629 1688
ydotool click 1
```

Find the window first: `flameshot full -p /tmp/shot.png`, crop with ImageMagick
or locate the light window rectangle with PIL. A window can be at a different
position on each launch; do not reuse coordinates blindly. `grim` cannot capture
this GNOME session (no wlr-screencopy); `flameshot full` works.

The window is a 900x740 page area, 952x792 including GTK's CSD shadow, about
1587x1320 physical at scale 1.6667, and it opens centred; the
1600x1700+1100+250 crop above contains it on this display. Crop coordinates map
to absolute physical pixels by adding the crop offset; the displayed image may be scaled, so multiply by
`1600/displayed_width` first.

## flameshot

```sh
flameshot full -p /tmp/shot.png
magick /tmp/shot.png -crop 1600x1700+1100+250 +repage /tmp/crop.png
```

Only inspect the cropped app area. Delete every screenshot after use.

## Driving the window without a pointer

GTK4 exports the window over AT-SPI2 (`pyatspi` is installed here), which is
how the one-window checks ran while screenshots were unavailable. A page tab,
button or menu entry is an accessible node with a `click` action; an entry
takes `setTextContents`. The application is named after the binary, so a test
build renamed away from `gotrust` gets its own single-instance lock and its
own tray item, and the installed app stays untouched.

- A page switch is `doAction` on the `page tab` named 仪表盘, 账户, 代理, ….
  The page heading, the 状态 chip (`notification`, `状态：未连接`) and the
  power control (连接, 取消, 断开, 重试) are what to assert afterwards.
- The tray menu is the app's own `com.canonical.dbusmenu` on its unique bus
  name, at `/org/ayatana/NotificationItem/<name>_<pid>_1/Menu`:
  `GetLayout` lists the items, `Event(id, "clicked", …)` activates one.
  打开仪表盘 shows and focuses the window; 连接 starts the tray login. The
  shell's own icon exposes no Action, so the icon click and the panel stay the
  operator's check.
- Wayland refuses focus stealing: a background launch cannot focus itself
  even through `window.Focus()`. `Alt+Tab` with a synthetic keyboard focuses
  it, after which the window shortcuts (Ctrl+Enter) arrive. A synthetic
  keyboard or pointer must declare every key with `UI_SET_KEYBIT` and every
  axis with `UI_SET_ABSBIT`; events for undeclared codes are dropped by the
  kernel. `ydotool` is not installed here; `/dev/uinput` is world-writable, so
  the same events can be written directly.
- The window minimum is a surface size: GTK's hint counts the CSD shadow, and
  on Wayland neither `Bounds` nor `ContentBounds` reports that margin. A page
  minimum of 720x560 needs a 772x612 hint (`windowShadowWidth` and
  `windowShadowHeight` in `internal/gui`); a test build opened at 600x400 is
  clamped to a 720x560 page, which is how the compensation was checked.

## Launch and quit

A clean start, the ready line, and a clean stop. The packaged binary is
`/usr/bin/gotrust`; the appindicator deprecation warning on stderr is expected.

```sh
tmp=$(mktemp -d)
mkdir -p "$tmp/config" "$tmp/state"
XDG_CONFIG_HOME="$tmp/config" XDG_STATE_HOME="$tmp/state" ./gotrust \
  >"$tmp/stderr.log" 2>&1 &
pid=$!
grep -q 'goTrust ready' "$tmp/state/gotrust/gotrust.log"
kill -TERM "$pid"
wait "$pid"               # expect 0
```

## mygo window rules

Measured on 0.6.3, 0.6.4, 0.7.1 and 0.7.2:

- `NewWindow` inside a frame is safe from 0.6.4 on. In 0.6.3 it made the new
  window's GtkGLArea fail (`OpenGL 0.0 is too old`, then `native UI shows
  nothing: gl: no current context`) and the window stayed blank: 0.6.4's
  `gl.MakeAreaCurrent` before an in-memory present fixed that half (issue #680's
  logs).
- `Hide` of a window from **its own frame** crashed GTK on Wayland up to 0.7.1:
  SIGSEGV `addr=0x40` in `gdk_cairo_draw_from_gl` → `gdk_gl_context_make_current`
  → `wl_proxy_get_version` with a NULL proxy, because the hide unmapped the
  window before GTK painted its GtkGLArea. 0.7.2 carries the fix
  (`linux-hide-in-frame`): it queues that hide in an idle below `Close`'s
  priority, as mygo already defers a destroy during a frame (PR #674), so the
  app hides and shows windows from any frame again.
- Hiding one window from another window's frame, and `Show`, were always fine.
- If mygo is ever moved back to a release without the fix, hiding a window from
  its own frame must go through `go func() { mygo.RunOnMain(fn) }()` again. The
  one window hides on close and keeps its settings draft; reopening it from
  the tray shows the same page and edits.

## Frame rules

A window's build runs inside mygo's frame, so an action that runs while the
view builds runs there too. What each operation does from there, measured in
the headless Wayland session and on the operator's GNOME session, with a second
window carrying a `Scroll`. The last column is 0.7.2, which carries the
`linux-hide-in-frame` fix:

| Operation, from | Where | 0.6.3 | 0.6.4 | 0.7.1 | 0.7.2 |
|---|---|---|---|---|---|
| `NewWindow` | a frame | blank window, `no current context` | renders | renders | renders |
| `Close` | a frame | destroys at once, no crash in this run | deferred by mygo | deferred by mygo | deferred by mygo |
| `Hide` | the window's own frame | SIGSEGV | SIGSEGV | SIGSEGV | hides after the frame |
| `Hide` | another window's frame | fine | fine | fine | fine |
| `Show` | a frame | fine | fine | fine | fine |

To repeat it: one window's build calls `NewWindow` once for a second window,
and a timer posts `Update` to ask that window to call `Hide` from its own build;
swap the calls for `go func() { mygo.RunOnMain(fn) }()` to see them pass. A
crash prints `SIGSEGV ... addr=0x40` and
`signal arrived during cgo execution`; `gdb -batch -ex run -ex bt` lands in
`gdk_cairo_draw_from_gl`, so the fault is GTK painting a GtkGLArea whose Wayland
context the hide released, not a Go frame.

## Autostart smoke

With a temporary config that sets `[app] autostart = true`, launch the app and
check `$XDG_CONFIG_HOME/autostart/*.desktop`. It should point at the binary and
pass `--mygo-opened-at-login`; SIGTERM should still exit 0.
