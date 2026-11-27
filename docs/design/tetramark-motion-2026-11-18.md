# The tetramark in motion

Standing: undecided | kind: wanted | settled-by: nothing found on main; line 3 says built on lane tetramark-motion, not seen on a device; origin/lane/tetramark-motion (a445ce8cf) is not an ancestor of origin/main

Status: built on lane `tetramark-motion` (2026-11-18), not yet seen on a device. Owner, 2026-11-17: "we can play with giving
motion to the tetra mark. We started on this a few days ago and never got anywhere ... we can rotate it clockwise or
counterclockwise, but also give it a 3D sense so it can be rotating like a tetrahedron as well. And it can also stretch and
shrink, and the three segments can be moving independently from one another."

## The earlier attempt (2026-11-10 and 11)

Commits `88f1d57ad`, `68dfc163b`, `5ef5ab782`, `ecf50f627` gave `mobile/AutoradioLab/mark.contract` a pure-Contract mark: the
three brand blades (`frontend/src/lib/tetramark.ts`), each with a blend from the flat drawing to a real tetrahedron (beta), an
outward offset, a roll and a scale, and states ("idle", "listening", "thinking", "speaking", "error") as closed forms of the
seconds in the state. A Python port (`check-tetramark-projection.py`) checked the points stayed inside the box.

Why it went nowhere (read from the code, not from a trace of the phone): the states were wired to the bar's lifecycle
(`bi.state`), and the per-frame task ran only while the bar was not idle, so the resting bar mark never moved and the owner's
first impression was a still logo. Every state's pose began at the rest pose at `ee = 0`, so an interrupted motion snapped to
rest and started again (a jump). The whole-mark turn applied only when all three blades sat at the solid, so the 3D reading was
rare. The model lived inside one 190-line contract, with no Swift specification and no test of the maths; the only check was a
Python port of the choreography. Roll in either direction, uniform stretch and the owner's "independent segments" were not
separate controls, and there was nothing to play with: the motion lab showed three still poles.

## The model (`AutoradioKit/Sources/AutoradioKit/TetraMarkMotion.swift`)

Pure arithmetic, no UI, no clock (every function takes a time). The Swift file is the specification and is tested; the contract
`mobile/AutoradioLab/tetramark.contract` mirrors it number for number (a test compares their data blocks).

A **pose** is 24 numbers. Whole mark: `roll` (in-plane, radians, positive is clockwise), `tumble` about an axis given by
`azimuth` and `elevation`, `beta` (0 the flat brand drawing, 1 the real tetrahedron, a point-for-point blend), `scale`, `alpha`,
a nudge `dx dy`. Per blade (three): `phase` (that blade alone rolls about the centre), `spread` (out from the centre; negative
moves in), `length` (stretch along the blade's own tangent), `scale`, `beta`.

**Projection** is real: each blade point is its flat (x, y, 0) place blended to its tetrahedron (x, y, z) place, stretched,
rolled and spread by the blade's numbers, turned by `Rz(roll) * Rodrigues(axis, tumble)`, and perspective-projected (eye 8
radii back, divisor clamped at 4). The blades are the brand's own points, so every stroke keeps its freehand curve. A tumble
past a few degrees pulls the blades to the solid, so a half turn shows the hidden vertex come round, foreshortened; a whole turn
lands on the flat drawing again. When all three blades are at the solid the four faces are drawn behind as a faint solid. Blades
are drawn back to front by the depth of their centres. The rest pose projects to exactly today's drawing.

**Interpolation.** A named motion is a closed form of the seconds since it began. `MarkAnimator.begin` captures the pose on
screen (angles wrapped to the nearest turn: the same picture, so a spinning mark settles without unwinding) and blends from it
to the new target with a smoothstep (an overshoot ease for the tray). Loops are periodic and start and end at rest. An
interruption at any instant therefore never jumps.

**Reduce Motion.** The pose is a still picture per motion; a change cross-fades alpha over 300 ms. After the fade the mark needs
no frames.

**Frames.** `MarkAnimator.frames(at:)` says none, slow (idle breath, five a second) or full. The root's frame task
(`members-mark.contract`, `mkHot` and `mkIdle`) runs only then. A backgrounded app has no display link, and the mark is a closed
form of the clock, so nothing is lost on return. Nothing here touches the audio path; `level` is an input (the mic level for
listening, the gallery's level for playing) and the motions do not yet follow the voice.

## The motions and where each is used

| Motion | What it does | Used |
| --- | --- | --- |
| idle | very slow breath (6.5 s: scale 2.5%, a little solid) | the bar mark at rest |
| pressed | shrink 16%, release with a small overshoot | the bar mark, on a press (`toggleLauncher`) |
| trayOpen / trayClose | a third of a turn clockwise with a spring, back the other way | the bar mark while the launcher is open, and for 700 ms after it closes |
| working | the three blades chase each other round: each blade's phase, spread, scale and length cycle a third apart, the whole turns slowly | the bar mark while a turn or fetch is in flight (bar states `holding`, `heard`); the app's activity indicator |
| playing | a gentle pulse and small tumble; `level` raises it | the bar mark while the radio plays |
| listening | the blades spread and tumble with the voice; `level` is the mic | the bar mark while hearing speech (`listening`, `talking`) |
| tumbling | full 3D rotation, 4 s a turn | the gallery only |
| settle | return to rest | the gallery; any other motion ends by blending to rest |

The old `Mark` (`mark.contract`) is untouched and still draws the onboarding, wait and motion pages: another lane
(`page-motion`) owns those pages.

## The gallery

Settings, Developer details, "Mark gallery" (page `lab:mark`, beside the Motion gallery; no setting gates anything). The mark at
240 pt and at 36 and 64 pt, a button per motion, an input level, and 15 sliders (roll, tumble, tumble axis, solid, scale, and
per blade phase, spread and length). A slider drag holds the pose by hand from whatever was on screen; any motion button
returns to a motion without a jump.

Tour: `mobile/fixtures/tours/tetramark-motion.steps` (needs `agent ios --touch platform` for the drags).

## Regenerating

`python3 -I mobile/scripts/gen-tetramark.py` rewrites the data block of `mark.contract` and `tetramark.contract` and
`TetraMarkGeometry.generated.swift` from `frontend/src/lib/tetramark.ts` (and the tetrahedron it derives); `--check` fails on
drift. The generated Swift is checked in because the package builds without Python.

## Exact: what draws it, and what it could not do

Drawn as an `svg` of `polygon`s (Core Animation shape layers) whose `points` are recomputed per frame in Contract `fn`s from the
clock `mkT`. Exact has no `preserve-3d`, no `matrix` or `skew` on boxes, and a 3D box rotation flattens its children, so a
solid cannot be a nest of rotated boxes; SVG `d` morphs animate only in `transition`, not keyframes; SVG filters need the Metal
toolchain this Mac lacks. So the 3D is the projection maths in Contract, and the cost is a per-frame polygon update (one mark;
the baker's 65,536 list-step cap forced the pose blend to evaluate the target once, not once per number).

## Not yet seen

On a device: that it looks right and runs at frame rate at 36 pt; that the sliders drag; that the press, tray and working
motions read as the owner wants; Reduce Motion; battery of the five-a-second idle breath. Tuning the amplitudes needs the
owner's eye.
