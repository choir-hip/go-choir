# Density measurements for FP0 (RES-19, RES-07)

FP0 runs these on Node B (or from a machine with `ssh node-b`). Everything is
read-only except `density_bench.py --create-vms`. No script prints a secret;
the internal token header is read on the host and sent only to localhost.
Python 3 standard library only.

| script | does | creates VMs |
|---|---|---|
| `host_snapshot.py` | host meminfo (page cache, slab), PSI, KSM, per-VM RSS (vmctl list plus Firecracker anon/file split), optional host `fincore` | no |
| `guest_memory.py` | guest `procmem` (Cached, AnonPages, Slab, per-process RSS) through the host-sourced route | no |
| `density_bench.py` | benchmark step 1, the idle ladder | only with `--create-vms` |
| `accept_five.py` | D7 acceptance: five test computers started in a row with no hand stop, admission decisions, make-room victims and snapshot bytes, MemAvailable series, then each victim woken through the product API | only with `--run` |
| `test_guard.py` | canned-JSON test of the guard and safety stops | no |
| `test_accept_five.py` | canned-JSON test of `accept_five.py` (guard, stops, decisions, wake rules, plan, summary) | no |

## Steps (about 40 minutes with the 3 and 10 minute marks)

1. Baseline, before any P0 resource change (RES-19 says run before and after):
   `python3 scripts/density/host_snapshot.py --fincore --out base-host.json`
   Read `summary`: `mem_available_gib`, `page_cache_gib`, `slab_gib`,
   `ksm_pages_sharing` (the report measured 0, finding 5), and `vms` sorted by RSS.
2. Guest split on named computers (RES-07):
   `python3 scripts/density/guest_memory.py --vm <vm_id> --vm <vm_id> --fincore --out base-guest.json`
   Use the vm ids from `summary.vms`. Reading the paper or owner computer is allowed
   (a read); the benchmark below is not.
3. Idle ladder, only in a quiet window (no deploy, no owner activity):
   `python3 scripts/density/density_bench.py --out receipts/` (dry run: preflight only)
   `python3 scripts/density/density_bench.py --create-vms --ladder 1,2,4,8 --marks 3,10 --out receipts/`
   Add `--marks 3,10,30` and a longer ladder for the full report step. It mints
   test computers with `scripts/qa_test_computer.mjs`, stops them all at the end, and
   writes `receipts/idle-ladder-<run>.json`.
4. Put the receipts and a summary under `docs/evidence/` with the commit, host and
   tier (report §4 step 5), and replace the estimates in `vm-density-limits` §3.

## D7 acceptance run (`accept_five.py`)

Plan and preflight only (changes nothing; reads the host over ssh):
`python3 scripts/density/accept_five.py --out receipts/`

The run, from a checkout that has `frontend/node_modules` (the minting script
needs it; pass `--repo-root` for another checkout), in a quiet window:
`python3 scripts/density/accept_five.py --run --n 5 --idle-s 60 --out receipts/`

It mints `d7-accept-<run>-<i>` computers one at a time and never stops
anything; admission (D1) must hibernate idle test computers to make room. Per
start it records `admission.recent` decisions newer than the run start, the
hibernated VM ids with warmness class, MemAvailable before and after, the wall
time to active, and the snapshot bytes of each victim (vmctl's hibernate
timeline plus `du` on that VM's own `vm-state/<vm_id>/snapshot`, never the
artifacts root). After the idle pause it wakes each make-room victim this run
minted, through `choir computer start` with that computer's own key (read from
its 0600 credential file, passed in the CLI environment, never printed or
written to the receipt; build with `go build -o /tmp/choir ./cmd/choir`, done
automatically), and records the wall time and the boot timeline (`resume` is
a snapshot restore, `cold` and `fallback_cold` are not wakes). Output:
`receipts/accept-five-<run>.json` and `.summary.txt`: none refused, the
number of make-room hibernations, every wake under 1 s (expected to fail
until D2 lands; the times are reported either way) and the MemAvailable
series. The minted computers are left to idle hibernation; their ids are in
the receipt.

Stops (the only ones): memory PSI some avg10 above 5, a protected or non-test
computer that was active leaving active, a minted computer or make-room
victim that is not safe test class. There is no MemAvailable floor stop, since
the floor is vmctl's and admission is what is under test. The preflight blocks
the run when health shows admission disabled or `make_room` not `active`
(`--allow-weak-preflight` overrides, and the result is then vacuous); it warns
when the host looks able to hold all five. Protected: the paper (by class),
`computer-ad1b46d4c20129e84b9e239cf4720fb9`, `computer-03335285269bdba4f94377e56879f9e6`
and any `--protect-id`. Offline check: `python3 -I scripts/density/test_accept_five.py`;
`--canned FILE` (`{"collect": ..., "health": ...}`) prints the plan and preflight
without a host.

## What the guard does

`--create-vms` touches only computers vmctl lists with `warmness_class: test`
(an allowlist). The paper, the owner computer and every paid or platform
computer have another class and are refused; `computer-ad1b46d4c20129e84b9e239cf4720fb9`
and any `--protect-id` are refused even if misclassified; held computers are refused.
A minted computer that does not list as `test` ends the run. Stops (PSI some avg10
> 1.0, MemAvailable under floor + 2 GiB, a non-test active computer leaving
`active`, host CPU over 80% for 60 s) are polled every 30 s and before every mint.
`--floor-gib` defaults to the report's 4.67 on Node B; confirm it against the
current vmctl memory floor before the run.

## Named gaps

- **No guest `fincore` route (RES-07).** The only host-to-guest memory route is
  `GET :8085/internal/debug/pprof/procmem` (`internal/server/debug_procmem.go`);
  it reports total `Cached` and per-process `RssFile`, not residency per file tree.
  Nix-store pages versus Dolt pages is therefore not read directly. `--fincore` gives
  the host's residency of the shared store image and each `data.img` to set beside the
  guest's `Cached`; that is correlation, not attribution. Closing the gap is an
  orange guest-image change (a procmem field running mincore over the store mount
  and the Dolt directory); it is not in this lane. Decision for FP0/owner: ship that
  field, or accept the correlational answer.
- **Benchmark steps 2 to 4** (hibernate and wake waves, busy ladder, deploy storm)
  are not scripted: they need a fixed workload and an announced window.
- **LR source sweep and oversubscription probe** (`choir-ledger-lr-resource` acceptance)
  need autopaper sources and the paper; the idle ladder is the closest piece here.
- `host_snapshot.py` slab detail (`/proc/slabinfo`) needs root and is not read;
  `Slab` and `SReclaimable` come from meminfo.
