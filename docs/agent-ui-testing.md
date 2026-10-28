# UI testing on this workstation

Manual smoke tests for the native UI. The app is a tray application, and a
crash looks like an exit: always capture stderr.

## Safety

- Run with temporary `XDG_CONFIG_HOME` and `XDG_STATE_HOME` unless the test
  specifically needs the operator's profile. Their real config holds
  credentials.
- Screenshots of the desktop may contain private data. Keep them in `/tmp`,
  crop to the app, and delete them when done. Never commit or paste one.
- Do not leave the app, `ydotoold`, or injected input running after the test.
- Confirm the desktop is idle before injecting input. The operator may keep
  using the machine: a window can open or rise between the screenshot and the
  click, and a stale coordinate then hits the wrong application. Take the
  screenshot and click in the same step, and stop the smoke if the layout
  changed.

## Headless session (preferred)

Most of the native smoke can run without touching the operator's desktop: a
headless mutter plus its own Xwayland gives an isolated display whose windows,
screenshots and injected clicks stay inside it. Nothing appears on the real
screen, and the capture cannot contain the operator's data.

```sh
root=/tmp/gt-nested
mkdir -p "$root/runtime" "$root/config" "$root/state"
chmod 700 "$root/runtime"
XDG_RUNTIME_DIR="$root/runtime" mutter --headless --no-x11 \
  --wayland-display=wayland-gt --virtual-monitor 1280x1024 &
XDG_RUNTIME_DIR="$root/runtime" WAYLAND_DISPLAY=wayland-gt Xwayland :99 &
sleep 2
DISPLAY=:99 GDK_BACKEND=x11 XDG_CONFIG_HOME="$root/config" \
  XDG_STATE_HOME="$root/state" ./gotrust --config "$root/config/config.toml" &
```

- Capture with `import -display :99 -window root "$root/shot.png"`: only the
  nested display is in the image. `grim` and `flameshot` are not involved.
- Click and move with XTEST through python-xlib:
  `from Xlib.ext import xtest; xtest.fake_input(d, X.ButtonPress, 1)` after a
  `MotionNotify` warp; coordinates are the ones in the capture.
- There is no window manager: windows take their requested size, and the
  Xwayland screen starts at 640x480, which clips a 600x760 settings window.
  Enlarge it on :99 (the "against an Xwayland server" warning is expected;
  `--newmode` may report `BadName` yet still work):
  `xrandr --newmode "1024x768" ...`, `xrandr --addmode XWAYLAND0 1024x768`,
  `xrandr --output XWAYLAND0 --mode 1024x768`.
- The same `--config` rules apply. A profile with `auto_connect = true`,
  `store_password = true`, a password and a listener that accepts but never
  answers puts the window in `连接中`, which is how 断开 during a login is
  exercised in a real window.
- Kill the app, `Xwayland :99` and the headless mutter when done. The real
  desktop is still needed for tray/panel behaviour (icon, menu, double-click).

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
and it avoids the hot corner entirely. With the settings window focused,
`Shift+Tab` then `Enter` reaches and activates 关闭.

Pointer input, only when needed:

```sh
ydotool mousemove --absolute 1629 1688
ydotool click 1
```

Find the window first: `flameshot full -p /tmp/shot.png`, crop with ImageMagick
or locate the light window rectangle with PIL. A window can be at a different
position on each launch; do not reuse coordinates blindly. `grim` cannot capture
this GNOME session (no wlr-screencopy); `flameshot full` works.

The main window is 540x640 logical, about 900x1067 physical at scale 1.6667
plus its title bar, and it opens centred; the 1600x1700+1100+250 crop above
contains it on this display. Crop coordinates map to absolute physical pixels
by adding the crop offset; the displayed image may be scaled, so multiply by
`1600/displayed_width` first.

## flameshot

```sh
flameshot full -p /tmp/shot.png
magick /tmp/shot.png -crop 1600x1700+1100+250 +repage /tmp/crop.png
```

Only inspect the cropped app area. Delete every screenshot after use.

## Launch and quit

```sh
tmp=$(mktemp -d)
mkdir -p "$tmp/config" "$tmp/state"
XDG_CONFIG_HOME="$tmp/config" XDG_STATE_HOME="$tmp/state" ./gotrust \
  >"$tmp/stderr.log" 2>&1 &
pid=$!
# wait for the ready line
grep -q 'goTrust ready' "$tmp/state/gotrust/gotrust.log"
kill -TERM "$pid"
wait "$pid"               # expect 0
```

The packaged binary is `/usr/bin/gotrust`. The appindicator deprecation warning
on stderr is expected.

## mygo window rules

Measured on 0.6.3, 0.6.4 and 0.7.1 in the nested X11 session and on the
operator's GNOME Wayland session, with the repro in "Frame rules" below:

- `NewWindow` inside a frame is safe from 0.6.4 on. In 0.6.3 it made the new
  window's GtkGLArea fail (`OpenGL 0.0 is too old`, then `native UI shows
  nothing: gl: no current context`) and the window stayed blank: 0.6.4's
  `gl.MakeAreaCurrent` before an in-memory present fixed that half (issue #680's
  logs).
- `Hide` of a window from **its own frame** still crashes GTK on Wayland:
  SIGSEGV `addr=0x40` in `gdk_cairo_draw_from_gl` → `gdk_gl_context_make_current`
  → `wl_proxy_get_version` with a NULL proxy. The same on 0.6.4 and 0.7.1;
  upstream defers `Close` during a frame (PR #674) but not `Hide`. Hiding one
  window from another window's frame is fine, and so is `Show`.
- So only the hide hop is required: hide through
  `go func() { mygo.RunOnMain(fn) }()`. Creating a window in the frame that asked
  for it is fine, and `Close` there is deferred by mygo itself.
- The headless X11 session below does **not** reproduce the crash: the GL
  context works there, so the frame rules cannot be validated by this smoke
  alone. Run the repro on the operator's session instead.
- The settings window hides on close; reopening reloads the saved draft.
- The headless mutter needs a session bus: without
  `DBUS_SESSION_BUS_ADDRESS` it aborts with
  `assertion failed: (session_bus)`. Start a private one with
  `eval $(dbus-launch --sh-syntax)` or wrap the smoke in
  `dbus-run-session`.

## Frame rules

A window's build runs inside mygo's frame, so an action that runs while the
view builds runs there too. What each operation does from there, measured on
the operator's GNOME Wayland session with a second window carrying a `Scroll`:

| Operation, from | Where | 0.6.3 | 0.6.4 | 0.7.1 |
|---|---|---|---|---|
| `NewWindow` | a frame | blank window, `no current context` | renders | renders |
| `Close` | a frame | destroys at once, no crash in this run | deferred by mygo | deferred by mygo |
| `Hide` | the window's own frame | SIGSEGV | SIGSEGV | SIGSEGV |
| `Hide` | another window's frame | fine | fine | fine |
| `Show` | a frame | fine | fine | fine |

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
