# Every landing restarts the owner's computer, and the phone calls it "Offline" (2026-11-25)

## Problem

The owner, 2026-11-25 01:07Z, on the latest phone build: the radio said
"Offline: the desk cannot write ahead" and "The desk isn't answering right
now", and Up next was filled from the phone's own stories.

## Evidence

- Caddy on Node B: the phone's `POST /api/autoradio/programme` at 01:05:20Z
  answered 503 in 86 ms. The next ask, at 01:10:40Z, answered 200 (51 s), and
  two more at 01:11:29Z and 01:14:18Z answered 200.
- At 01:04:36Z the deploy of `32820afa4` (lane `purge-unproved`) killed the
  owner's computer (`vm-1669020…`, `cause=refresh`) and it was back at
  01:05:06Z; the proxy refused wakes while it was "booting" until 01:05:20Z
  ("a computer in state \"booting\" is not woken").
- That release changed only a host operator command (`cmd/purge-unproved`,
  `internal/unprovedpurge`) and `flake.nix`. The deploy's impact step marked
  each Go file "conservative Go impact: host + canonical guest image", so every
  computer was refreshed (CI run 45521556092, "Detect deployed artifact changes").
- The owner's computer was refreshed by a deploy 19 times between
  2026-11-24T01:00Z and 2026-11-25T01:10Z (`journalctl`, `destructive kill
  vm=vm-1669020… cause=refresh`).
- The phone's writer backs off after a failed ask (10 s, doubling to 5 min,
  `mobile/AutoradioLab/radio-writer.ts:133`), and the status line says
  "Offline" for any failed ask (`radio-model.ts:272`), so a one-minute restart
  reads as the phone being offline for up to several minutes.

## Belief

Two faults. (1) The deploy decides whether computers must restart from file
paths, and any Go change outside a short list counts as a guest change, so a
host-only release restarts every computer. The question it should ask is
whether the guest image or the autoputer runtime package that would boot is
different from the one running. (2) The phone names a restarting computer as
"Offline" and waits out its backoff instead of asking again when the computer
is back.

## Remaining error

- Whether the autoputer runtime package built from `32820afa4` was
  byte-identical to the one running (not built or compared).
- How many of the 19 refreshes were for releases with no guest change.
- Whether the 503 carried a reason the phone could tell apart from a desk that
  is down (the phone reads it as kind `other`).
