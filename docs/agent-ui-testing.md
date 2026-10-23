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
position on each launch; do not reuse coordinates blindly.

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
XDG_CONFIG_HOME="$tmp/config" XDG_STATE_HOME="$tmp/state" ./goTrust \
  >"$tmp/stderr.log" 2>&1 &
pid=$!
# wait for the ready line
grep -q 'goTrust ready' "$tmp/state/goTrust/goTrust.log"
kill -TERM "$pid"
wait "$pid"               # expect 0
```

The packaged binary is `/usr/bin/goTrust`. The appindicator deprecation warning
on stderr is expected.

## mygo window rules

- Open and hide windows outside their own frame:
  `go func() { mygo.RunOnMain(fn) }()`.
- Creating a native window inside a frame can leave it without a GL context
  (`OpenGL 0.0 is too old`, `native UI shows nothing: no current context`).
- Hiding or destroying one inside its own action can crash GTK:
  `SIGSEGV` in `linux.(*Backend).Run` through a `purego` callback. That is a
  crash, not a clean exit; read stderr. Upstream: mygo PR #674 (close in
  frame), issue #680 (blank second window).
- The settings window hides on close; reopening reloads the saved draft.

## Autostart smoke

With a temporary config that sets `[app] autostart = true`, launch the app and
check `$XDG_CONFIG_HOME/autostart/*.desktop`. It should point at the binary and
pass `--mygo-opened-at-login`; SIGTERM should still exit 0.
