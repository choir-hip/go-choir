# Texture documents stuck in "Revising…": 68 runs re-passivated on every boot

Date: 2026-10-09. Owner report (2026-10-08): Texture is slow to load
recent documents; once a document loads, the revision chevrons are slow; a
document can load into "Revising…"; in that state Cancel is not instant.
Operational invariant O1 (station SL: every durable obligation has one
driver and a recorded terminal fate). Mutation class of a fix: red
(Texture lifecycle, run state).

## Evidence (owner computer `computer-03335285…`, read-only, 2026-10-09 ~02:50Z)

Timed from Node B against the owner guest, the way the proxy calls it.
Response bodies were discarded; only status, size, IDs and counts were
kept.

- **Reads are fast when the host is quiet:**
  - `GET /api/texture/documents` (50 docs): 1.20 s cold, 0.15 s warm;
  - one revision (each chevron click): 15–25 ms;
  - revision list: 24 ms;
  - trajectory snapshot: 0.37 s.

  The reported slowness is load-dependent; see "Load" below.
- **"Revising…" on load.** The newest document `3b12d89a…` reports
  `agent_revision_pending: true`, but the list reports 0 pending documents.
  Its trajectory `4f0311fd…` has `activation.state = pending` for run
  `36f5e4e9…` (lifecycle version 10, last updated 2026-10-08 20:02:28,
  when that run authored a Texture turn).
- **The run cycles and never terminates.**
  - The owner console logs `passivated run 36f5e4e9… (was pending) after
    restart` on 7 boots: 23:08, 00:10, 00:36, 01:02, 01:33, 02:18, 02:34.
  - Every boot passivates the same 68 runs (`boot passivation owner-scoped
    candidates=68`; 65 on 2026-10-08).
  - A boot soon after another finds 0 candidates (00:10:46, 01:03:07), so
    passivation does commit. Something later returns the runs to
    `pending`.
  - The dispatcher logs `deferred … texture:… deferrals=35 cause=actor:
    defer unprocessed occurrence: actorruntime: pending Texture occurrence
    produced no exact run` throughout.
- **Passivation does not touch the Texture lifecycle.**
  `passivateInterruptedActivations` (`internal/agentcore/runtime.go:2336`)
  sets the run to `passivated` and marks the agent mutation stale. It
  appends no lifecycle event, so the document's activation stays `pending`
  and the editor shows "Revising…" (`TextureEditor.svelte:518,542`).
- **Cancel is three sequential round trips** (`frontend/src/lib/texture.js:429`):
  get document, trajectory snapshot (0.37 s quiet), then the cancel POST.
  So Cancel can never feel instant. On a pending activation with no live
  run, it may wait on the same stuck machinery (not yet measured).

## Load (hypothesis, to measure)

The quiet-host reads above are fast; the owner saw slowness yesterday.

- Node B at 02:46: one VM (`candidate-fleet-e15cb89f…`, the owner) holds
  14.7 GB RSS of 31 GB; swap 12 MB free; load average 11 at 02:42.
- The owner guest store is 11 GiB live, and `dolt gc` is skipped every
  boot ("exceeds safe bounded-guest GC size (5 GiB)").
- Every boot also spends ~13 s passivating the 68 zombie runs and ~9 s
  reconciling terminal outcomes.

Slowness under load is a hypothesis until it is measured during an
active run on a large-history computer.

## Same session: signup computer failed on a credential timeout

QA account `qa-owner-20261009@example.com` (persistent, virtual passkey):
at 02:42:32 its first boot failed. vmctl logged `computer credential
request failed … /internal/computers/credentials/issue: context deadline
exceeded` → `realization credential unavailable` → state `failed`. A
retry 4 minutes later booted in 17 s. A brand-new user saw a failed
computer during host load.

## Next

1. Find what returns passivated runs to `pending` (trace one run across
   boots: run row updates, dispatcher deferrals, rewarm).
2. Every passivated or abandoned Texture activation needs a terminal
   lifecycle event, so the document leaves "Revising…".
3. Make Cancel one request, and make it work when no live run exists.
4. Measure Texture reads under load on a large-history computer.
5. Credential issuance under load: a refusal or retry, never a `failed`
   computer.
