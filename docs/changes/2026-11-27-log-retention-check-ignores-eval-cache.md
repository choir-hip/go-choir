---
date: 2026-11-27
title: the log-retention check no longer passes or fails by nix's evaluation cache
class: yellow
---
What: `scripts/test-node-b-log-retention.sh` reads the disk sweep's script text with `nix eval --no-eval-cache`, and prints nix's error when the text cannot be read instead of hiding it.

Why: on the Mac the check failed in about three runs of four, on main, with "L1 the disk sweep text could not be read from the evaluated configuration". The hidden error was "experimental Nix feature 'dynamic-derivations' is disabled (building output 'out' of ...-go-choir-nix-retention.drv)": with the flake's evaluation cache, nix tries to realise a Linux store path the Mac cannot build. `scripts/gates` runs this check before every landing, so a landing failed or passed by the state of a cache.

Proof: before, the bare `nix eval --raw` of that attribute failed 3 of 3 and with `--read-only` 2 of 2, while `--no-eval-cache` passed 4 of 4 (run back to back on main at 5b61bface); after, the whole check passed 4 of 4 (`pass=10 fail=0`). Not proved: why the cached path sometimes succeeded; other checks that call `nix eval` with the cache (`check-host-firecracker-pin.sh`, `check-guest-data-fstrim.sh`) were not seen to fail and are unchanged.

Replaced: an evaluation that depended on the cache, with its error sent to /dev/null.

Undo: revert this commit; the check is flaky on the Mac again.
