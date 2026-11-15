# Replay-completeness refuses on d@e for lack of scratch space

Recorded 2026-11-14T21:00Z. Not repaired.

## Problem

`GET /api/computers/<d@e>/self-development/replay-completeness` answers 500:
"replay scratch insufficient: probe needs about 2.07 GB plus a 3.35 GB reserve
on /mnt/persistent/replay-probe; 975 MB free". The check that says a computer
can be rebuilt from its tape cannot run on the proof computer.

## Evidence

One call during the stored-result step 2 drill on d@e, 2026-11-14 about
20:45Z, build fdbe766d6. Nothing was freed and nothing else was tried.

## Belief

d@e's persistent volume is nearly full. The likely holders are the things
part 3 is about (updater releases, run memory, the store itself); prune at
boot, landing tonight, may free enough. Not measured: what holds the space,
and whether the paper would refuse the same way (its probe would need far
more than 2 GB).

## Remaining error

- No reading of d@e's volume by holder.
- Step 5 of the stored-result proof is open until this runs.
- A replay check that needs free space on the volume it is checking fails
  exactly when the volume is in trouble; whether it should run elsewhere is a
  design question.
