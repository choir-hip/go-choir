#!/usr/bin/env python3
"""The mark's projection is pure arithmetic, so its bound is checked numerically, not by eye.

Ports the projection in mobile/AutoradioLab/mark.contract (turn about y, tip about x, perspective, fit shrink, slab
thickness along the blade normal) and asserts that over a full turn at the pitch extremes, with the hop and sway of the
thinking state added, no projected point of any blade (front or back face) leaves LIMIT x the brand box's half size, and
that the rest pose (d = 0) is exactly the brand drawing. The constants below must equal the ones in mark.contract; the
script also checks the contract text still carries them. Run: python3 -I mobile/scripts/check-tetramark-projection.py"""
import math
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CONTRACT = (ROOT / "mobile/AutoradioLab/mark.contract").read_text()
TS = (ROOT / "frontend/src/lib/tetramark.ts").read_text()

D = 8.0          # eye distance in radii (pxy)
S = 0.7          # face slope (gen-tetramark.py)
C = 0.35         # apex depth (gen-tetramark.py)
THICK = 0.1      # slab thickness, along the blade normal (mark.contract `thick`)
FIT = 0.2       # shrink at full depth (mark.contract `fit`)
SC_THINK = 1.04  # thinking's largest scale (with the hop and sway)
SC_MAX = 1.1     # largest scale any other state applies (listening), without hop or sway
PITCH_MAX = 0.3  # largest |pitch| any state applies
HOP = 30         # thinking hop (up), brand-box units
SWAY = 12        # thinking sway
LIMIT = 1.15

for needle in (f"max({D:g} - rxz(", f"* {THICK:g}", f"1 - {FIT:g} * d"):
    if needle not in CONTRACT:
        sys.exit(f"mark.contract no longer carries `{needle}`; update this script and the contract together")

body = TS.split("TETRA_MARK_PATHS = [", 1)[1].split("];", 1)[0]
blades = [[((float(x) - 256) / 256, (float(y) - 256) / 256) for x, y in re.findall(r"[ML]\s*([-\d.]+)\s+([-\d.]+)", d)] for d in re.findall(r"'([^']+)'", body)]


def project(p, z, n, o, yaw, pitch, sc):
    d = min(1, 2 * (abs(math.sin(yaw / 2)) + abs(math.sin(pitch / 2))))
    x, y = p
    x, y, z = x - o * d * n[0], y - o * d * n[1], (z - o * n[2]) * d
    sa, ca, sb, cb = math.sin(yaw), math.cos(yaw), math.sin(pitch), math.cos(pitch)
    x1, z1 = x * ca + z * sa, z * ca - x * sa
    y2, z2 = y * cb - z1 * sb, y * sb + z1 * cb
    k = D / max(D - z2, 0.5 * D)
    f = sc * (1 - FIT * d)
    return 256 * x1 * k * f, 256 * y2 * k * f, d


def main():
    worst_x = worst_y = 0.0
    rest = 0.0
    for b in blades:
        cx = sum(p[0] for p in b) / len(b)
        cy = sum(p[1] for p in b) / len(b)
        m = math.hypot(cx, cy)
        ux, uy = cx / m, cy / m
        c = C
        nl = math.sqrt(S * S + 1)
        n = (S * ux / nl, S * uy / nl, 1 / nl)
        for p in b:
            z = c - S * (p[0] * ux + p[1] * uy)
            px, py, d = project(p, z, n, 0, 0, 0, 1)
            rest = max(rest, abs(px - 256 * p[0]), abs(py - 256 * p[1]))
            for deg in range(0, 361, 2):
                for pitch in (-PITCH_MAX, -PITCH_MAX / 2, 0, PITCH_MAX / 2, PITCH_MAX):
                    for o in (0, THICK):
                        px, py, _ = project(p, z, n, o, math.radians(deg), pitch, SC_THINK)
                        worst_x = max(worst_x, abs(px) + SWAY)
                        worst_y = max(worst_y, abs(py) + HOP)
                        px, py, _ = project(p, z, n, o, math.radians(deg), pitch, SC_MAX)
                        worst_x = max(worst_x, abs(px))
                        worst_y = max(worst_y, abs(py))
    lim = 256 * LIMIT
    print(f"rest pose error {rest:.6f} (must be 0); worst x {worst_x:.0f}, worst y {worst_y:.0f}, limit {lim:.0f}")
    if rest > 1e-9:
        sys.exit("FAIL: the rest pose is not the brand drawing")
    if max(worst_x, worst_y) > lim:
        sys.exit("FAIL: a projected point leaves the bound")
    print("ok: projection stays inside the bound at every angle, rest pose exact")


main()
