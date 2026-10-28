#!/usr/bin/env python3
"""build_dmg.py — Build a beautified HiTool DMG using dmgbuild.

dmgbuild is the standard Python tool for this; it constructs the volume
layout entirely in Python via the ds_store + mac_alias libraries, never via
AppleScript.  This avoids the macOS TCC refusal that breaks any AppleScript
that touches Finder from an unprivileged CLI.

Usage:
  build_dmg.py \\
      --app /path/to/hitool.app \\
      --bg  /path/to/background.png \\
      --name "HiTool" \\
      --output /path/to/hitool.dmg \\
      [--icon-size 128] [--text-size 12] \\
      [--win-x 100 --win-y 100 --win-w 660 --win-h 400] \\
      [--app-x 200 --app-y 180 --apps-x 440 --apps-y 180]
"""

from __future__ import annotations

import argparse
import os
import shutil
import sys
import tempfile
from pathlib import Path

import dmgbuild


def build() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--app", required=True, help="Path to the .app bundle")
    p.add_argument("--bg", required=True, help="Path to the background PNG")
    p.add_argument("--name", required=True, help="DMG volume name")
    p.add_argument("--output", required=True, help="Path to the output .dmg")
    p.add_argument("--icon-size", type=int, default=128)
    p.add_argument("--text-size", type=int, default=12)
    p.add_argument("--win-x", type=int, default=100)
    p.add_argument("--win-y", type=int, default=100)
    p.add_argument("--win-w", type=int, default=660)
    p.add_argument("--win-h", type=int, default=400)
    p.add_argument("--app-x", type=int, default=200)
    p.add_argument("--app-y", type=int, default=180)
    p.add_argument("--apps-x", type=int, default=440)
    p.add_argument("--apps-y", type=int, default=180)
    args = p.parse_args()

    app_src = Path(args.app).resolve()
    bg_src = Path(args.bg).resolve()
    out_dmg = Path(args.output).resolve()

    if not app_src.is_dir():
        print(f"app bundle not found: {app_src}", file=sys.stderr)
        return 1
    if not bg_src.is_file():
        print(f"background image not found: {bg_src}", file=sys.stderr)
        return 1

    # dmgbuild copies everything listed in settings["files"] into the volume
    # root, and creates the volume-level "drag to Applications" affordance
    # from settings["symlinks"].  We stage the .app into a temporary folder
    # so that the whole build is read-only w.r.t. the source bundle.
    #
    # Using a symlink for Applications also dodges a bug in the mac_alias
    # library on macOS 14+, where Alias.for_file("/Applications") returns a
    # 64-bit CNID that overflows struct.pack's "I" (32-bit) format.
    staging = Path(tempfile.mkdtemp(prefix="dmg-stage-"))
    try:
        staged_app = staging / app_src.name
        shutil.copytree(app_src, staged_app, symlinks=True)

        settings = {
            "format": "UDZO",
            "compression_level": 9,
            "volume_name": args.name,
            "background": str(bg_src),
            "files": [str(staged_app)],
            "symlinks": {"Applications": "/Applications"},
            "icon_size": args.icon_size,
            "text_size": args.text_size,
            "window_rect": (
                (args.win_x, args.win_y),
                (args.win_x + args.win_w, args.win_y + args.win_h),
            ),
            "icon_locations": {
                app_src.name: (args.app_x, args.app_y),
                "Applications": (args.apps_x, args.apps_y),
            },
        }

        out_dmg.parent.mkdir(parents=True, exist_ok=True)
        if out_dmg.exists():
            out_dmg.unlink()

        dmgbuild.build_dmg(
            filename=str(out_dmg),
            volume_name=args.name,
            settings=settings,
        )

        print(f"wrote {out_dmg} ({out_dmg.stat().st_size} bytes)")
        return 0
    finally:
        shutil.rmtree(staging, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(build())
