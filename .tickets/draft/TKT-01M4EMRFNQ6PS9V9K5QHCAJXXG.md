---
schema: 3
id: TKT-01M4EMRFNQ6PS9V9K5QHCAJXXG
title: Quiet Mesa's EGL warnings when the window opens under WSL2
type: bug
status: draft
status_reason: null
priority: low
due_on: null
labels:
  - desktop
assignees: []
milestone: null
parent: null
origin: null
dependencies: []
blocks_on: none
references: []
claim: null
archive: null
created_at: 2026-10-08T20:54:58Z
updated_at: 2026-10-08T20:54:58Z
created_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
updated_by:
  id: agent:claude/forgejo-parity-20261006
  name: ""
extensions: {}
---

## Description

On WSL2 (WSLg), the owner saw this on stderr at every launch of the window:

    libEGL warning: failed to get driver name for fd -1
    libEGL warning: MESA-LOADER: failed to retrieve device information
    MESA: error: ZINK: failed to choose pdev
    libEGL warning: egl: failed to create dri2 screen

WebKitGTK asks Mesa for a hardware GL context. WSLg offers no DRM device that
Mesa can open, and zink finds no Vulkan device, so Mesa falls back to software
rendering. The window still works; the lines are noise, but they read as
errors and they arrive before anything vuoro prints.

tuohi v0.1.0 sets none of the usual WebKitGTK GL variables. Its own CI sets
`LIBGL_ALWAYS_SOFTWARE`, `WEBKIT_DISABLE_DMABUF_RENDERER`, and
`WEBKIT_DISABLE_COMPOSITING_MODE` for its GUI jobs (tuohi
TKT-01M3HWWRRXTAR4T01SK79Z4BSM).

### Options

- Set `LIBGL_ALWAYS_SOFTWARE=1` before tuohi loads WebKit when vuoro runs
  under WSL (`WSL_DISTRO_NAME` is set, or `/proc/sys/fs/binfmt_misc/WSLInterop`
  exists), and only when the person has not set it. This stops Mesa trying
  hardware it cannot have. It gives up WSLg's d3d12 acceleration where that
  does work, so measure scrolling a long diff first.
- Set `WEBKIT_DISABLE_DMABUF_RENDERER=1` under WSL, which is the common fix
  for blank or flickering WebKitGTK windows there. It may not silence Mesa.
- Fix it in tuohi rather than vuoro, so every terva-sh application gets it.
  That needs a tuohi release and a deliberate pin bump.
- Leave it, and say in the README that the lines are harmless.

Check first whether the window is slow or blank on WSL, not only noisy.
