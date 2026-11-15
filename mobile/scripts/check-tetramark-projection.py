#!/usr/bin/env python3
"""The mark's motion is pure arithmetic, so its bound is checked numerically, not by eye.

Ports the choreography and the projection in mobile/AutoradioLab/mark.contract (qBeta, qRho, qTheta, qScale, the yaw, pitch,
hop and sway of each state, the blend between pole A and pole B, the perspective with its clamped divisor) and asserts, over
the full cycle of every state at its extremes (listening at three levels, error's shake at its peak), that no projected
point of any blade or face leaves LIMIT x the brand box's half size, and that the rest pose is exactly the brand drawing.
The contract text is checked for the constants this port assumes. Run: python3 -I mobile/scripts/check-tetramark-projection.py"""
import math
import runpy
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
CONTRACT = (ROOT / "mobile/AutoradioLab/mark.contract").read_text()
LIMIT = 1.15
EYE = 8

for needle in ("max(8 - rxz(", "0.2 * sin(3.1416 * rise(0.02 * i, 0.34 + 0.02 * i, u))", "0.17 * (1 - ue)", "-26 * sin(3.1416 * rise(0.3, 0.8, u))",
               "r * (0.04 + 0.14 * amp)", "6.2832 * tfa", "0.25 * sin(3.1416 * rise(0.52, 0.8, u))", "1 + (0.05) * amp * r", "1 - 0.3 * sin(3.1416 * rise(0.02 * i, 0.34 + 0.02 * i, u))"):
    if needle not in CONTRACT:
        sys.exit(f"mark.contract no longer carries `{needle}`; update this script and the contract together")

M = runpy.run_path(str(ROOT / "mobile/scripts/gen-tetramark.py"), run_name="gen")["model"]()


def smooth(u): return u * u * (3 - 2 * u)
def rise(a, b, u): return smooth(min(1, max(0, (u - a) / (b - a))))
def pulsei(tt, i): return 0.5 + 0.5 * math.sin(6.2832 * 3.1 * tt + 2.1 * i)


def q(st, i, tt, r, amp, u, ue, dd=1, dr=8):
    pi = math.pi
    if st == "thinking":
        return (rise(0.3 + 0.04 * i, 0.46 + 0.04 * i, u) - rise(0.8 + 0.03 * i, 0.92 + 0.03 * i, u),
                0.2 * math.sin(pi * rise(0.02 * i, 0.34 + 0.02 * i, u)), 6.2832 * rise(0.02 * i, 0.28 + 0.02 * i, u), 1 - 0.3 * math.sin(pi * rise(0.02 * i, 0.34 + 0.02 * i, u)))
    if st == "listening":
        return (r * 0.2 * amp * (0.5 + 0.5 * math.sin(6.2832 * tt / 1.6 + 2.1 * i + 1)),
                r * (0.04 + 0.14 * amp) * (0.6 + 0.4 * math.sin(6.2832 * tt / 1.6 + 2.1 * i)),
                r * 0.1 * amp * math.sin(6.2832 * tt / 1.6 + 2.1 * i), 1 + 0.05 * amp * r)
    if st == "speaking":
        return (r * (0.1 + 0.12 * pulsei(tt, i)), r * 0.03 * pulsei(tt, i), r * 0.08 * math.sin(6.2832 * tt / 2.4 + 2.1 * i), 1)
    if st == "success":
        return (rise(0.05 * i, 0.25 + 0.05 * i, ue) - rise(0.6 + 0.05 * i, 0.9 + 0.05 * i, ue), 0, 0, 1)
    if st == "error":
        return (0, 0.17 * (1 - ue) * math.sin(6.2832 * (3 * ue + i / 3)), 0.25 * (1 - ue) * math.sin(6.2832 * (4 * ue + i / 3)), 1)
    return (dd * 0.14 * (0.5 + 0.5 * math.sin(6.2832 * tt / dr + 2.1 * i)), 0, 0, 1)


def whole(st, tt, amp, u, ue, bmin):
    """yaw, pitch, tx, ty, sc, with the shake and the sway at their extremes."""
    if st == "thinking":
        tfa = rise(0.55, 0.77, u)
        return (6.2832 * tfa, 0.25 * math.sin(math.pi * rise(0.52, 0.8, u)), 10 * math.sin(6.2832 * rise(0.3, 0.8, u)), -26 * math.sin(math.pi * rise(0.3, 0.8, u)), 1)
    if st == "success":
        return (6.2832 * rise(0.3, 0.6, ue), 0.2 * math.sin(math.pi * rise(0.25, 0.65, ue)), 0, -22 * math.sin(math.pi * rise(0, 0.6, ue)), 1 + 0.04 * math.sin(math.pi * ue))
    if st == "listening":
        return (0.3 * amp * math.sin(6.2832 * tt / 2.2) * bmin, 0, 0, 0, 1)
    if st == "speaking":
        return (0.3 * math.sin(6.2832 * tt / 3.2) * bmin, 0, 0, 0, 1)
    if st == "error":
        return (0, 0, 20 * (1 - ue) ** 2, 0, 1)
    return (0, 0, 0, 0, 1)


def pxy(x, y, z, yaw, pitch, sc, tx, ty):
    sa, ca, sb, cb = math.sin(yaw), math.cos(yaw), math.sin(pitch), math.cos(pitch)
    x1, z1 = x * ca + z * sa, z * ca - x * sa
    y2, z2 = y * cb - z1 * sb, y * sb + z1 * cb
    k = EYE / max(EYE - z2, 4)
    return 256 * x1 * k * sc + tx, 256 * y2 * k * sc + ty


def points(m, b, rho, th, s, yaw, pitch, sc, tx, ty):
    out = []
    ct, stt = math.cos(th), math.sin(th)
    for (ax, ay), (bx, by, bz) in zip(m["a"], m["b"]):
        lx, ly = ax + (bx - ax) * b, ay + (by - ay) * b
        out.append(pxy(s * (lx * ct - ly * stt) + rho * m["u"][0], s * (lx * stt + ly * ct) + rho * m["u"][1], s * bz * b, yaw, pitch, sc, tx, ty))
    return out


def main():
    rest = 0.0
    for m in M:
        for (px, py), (ax, ay) in zip(points(m, 0, 0, 0, 1, 0, 0, 1, 0, 0), m["a"]):
            rest = max(rest, abs(px - 256 * ax), abs(py - 256 * ay))
    worst = 0.0
    where = ""
    cases = []
    for n in range(0, 401):
        v = n / 400
        cases += [("thinking", 0, 0, v, 0), ("success", 0, 0, 0, v), ("error", 0, 0, 0, v)]
    for n in range(0, 400):
        tt = n * 0.05
        for amp in (0.0, 0.5, 1.0):
            cases += [("listening", tt, amp, 0, 0)]
        cases += [("speaking", tt, 0, 0, 0), ("idle", tt, 0, 0, 0)]
    for st, tt, amp, u, ue in cases:
        qs = [q(st, i, tt, 1, amp, u, ue) for i in range(3)]
        bmin = min(p[0] for p in qs)
        yaw, pitch, tx, ty, sc = whole(st, tt, amp, u, ue, bmin)
        for m, (b, rho, th, s) in zip(M, qs):
            pts = points(m, b, rho, th, s, yaw, pitch, sc, tx, ty)
            if bmin > 0.2:
                pts += [pxy(x, y, z, yaw, pitch, sc, tx, ty) for x, y, z in m["f"]]
            for px, py in pts:
                r = max(abs(px), abs(py))
                if r > worst:
                    worst, where = r, f"{st} t={tt:.2f} u={u:.3f} ue={ue:.3f}"
    lim = 256 * LIMIT
    print(f"rest pose error {rest:.6f} (must be 0); worst point {worst:.0f} of limit {lim:.0f} at {where}")
    if rest > 1e-9:
        sys.exit("FAIL: the rest pose is not the brand drawing")
    if worst > lim:
        sys.exit("FAIL: a projected point leaves the bound")
    print("ok: every state's cycle stays inside the bound, rest pose exact")


main()
