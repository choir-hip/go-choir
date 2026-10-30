# Repair Verification: Outage Chain Closed (2026-09-03)

- Date: 2026-09-03
- Mutation class: green (verification record)
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Staging: proxy + guest on `286e3141b2c9770a4e93267bd21514cbef1e10e5`
  (replay tolerance + sleep parity + merge takeover + 4 CPU/8 GiB shape)

## Proof Per Repair

1. **Replay tolerance** (`a4fae242`): live tape head advanced 138611 →
   **138679** (67 events, incl. finalized 138612). Boot replays pass the
   poison op; zero `projection mismatch` lines since. Status: repaired.
2. **Sleep generator** (`286e3141`): code + `TestSleepAfterTurnTapeRefused`
   shipped; both live callers disregard the error. Live proof (a refused
   sleep on a revision-carrying row) has not recurred — no new prepared
   poison rows; head advances monotonically. Status: generator disarmed,
   live behavior pending next occurrence. Root fix tracked in
   `effects-red-sleep-generator-unfinalizable-2026-09-03.md`.
3. **Stale hold** (`689ae8a9`): clean unfenced boot after two unholds — boot
   log has NO `maintenance hold active` line; passivation + rewarm phases ran
   (2 candidates passivated incl. `aa4fc186`, the poison run). Status:
   repaired.
4. **Store/OOM** (offline GC + sizing): drive 9.8G → 1.7G (default `dolt gc`,
   heads/rows/6k commits verified before boot); guest servable in seconds,
   runs-list in ~5s (was 20s+ timeouts), zero OOM kills across three boots.
   Status: repaired; hygiene follow-ups open (generator receipt items 2-6).

## Live State at Close

- No spurious Super rows (latest Super `6e4f885b` completed 16:00Z,
  pre-outage). Owner Texture actors resumed (`b351c120`, `fff3a881` running —
  owner activity, not mission).
- Fence `99949fe2` untouched. Effects remain OFF. No mission state mutated by
  the repair path (all diagnosis read-only except: stopped boots, one offline
  GC with backup (removed after verification), hold/unhold cycles (cleared)).
- Flake note: `TestCancelRunTrajectoryDrainsMoreThanOneActivePage` failed 2x
  on `286e3141` CI (Dolt scan deadline, known signature) then passed on 3rd
  run with no code change; also fails locally only under extreme machine load
  (loadavg 173 on 8 cores). Not a regression.

## Residuals (consensus-adjudicated, filed)

- Sleep generator root fix (dry-run before CAS, sleep parity full path,
  commit discipline, AS OF weaning): `effects-red-sleep-generator-unfinalizable-2026-09-03.md`.
- GC-that-fires + bloat gauge + scan budgets + hold derivation: same receipt.
- Node B scrubbed (copies, scripts, mounts, loops removed). VM shape 4/8192
  (epoch 871+) is incident override, not fleet default.
