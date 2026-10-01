# The first boot timeline, and what it found

*Written October 1, 2026. This letter covers the S0a slice of station S0 — the reality and boot-timeline instrumentation for the supervised app-development metamission — from mission open this morning through the landed receipts this evening.*

Here is the short version: the boot timeline instrument is deployed, it survived its own first test on staging, and it already produced the mission's first honest numbers — the fresh computer boots in under nine seconds, the owner computer takes eleven minutes when the tape is long, and the stalls are not where the guesswork put them.

## Where the work stands

Station S0 exists because every later station in this metamission — the security floor, the layering system, fast resume, forks and fleets — depends on knowing what a boot actually does. Before today, a boot was a black box: the host knew when a Firecracker process spawned and when the guest answered /health, and nothing in between. The S0a slice adds an observer that records a per-boot timeline, and it does it on the same observer the S3 resume work will reuse, so the marks are comparable across boot kinds.

## What became true

The instrument is two halves. Inside the guest, autoputer stamps phase marks — process entry, config loaded, runtime store open, credential exchange done, replay begin, listen ready, reconstruct done, vocabulary fenced, replay done, lifecycle reconciled, runtime started — and serves a merged receipt at a new internal endpoint. On the host, vmctl stamps the complementary marks — boot begin, port allocated, epoch claimed, data image ready, credential disk ready, Firecracker spawned, first HTTP response, first healthy — and persists the merged record to the VM's state dir so it survives restarts. There is also a bounded TCP-dial endpoint inside the guest that is only there to answer reachability questions the probe needs, not for any production use.

The instrument's first run caught a bug in itself. The owner computer's first post-instrument boot recorded the guest receipt as an EOF error, and a live probe of the endpoint confirmed it: the guest handler was panicking on a slice underflow in the systemd-analyze output parser, and on the value the parser emitted at that — it was reading the next labeled field's duration, so kernel_s was the initrd's number. A fix commit landed, a second deploy ran, and the receipt went from the EOF to a full merged timeline on the very next boot.

## The receipts

Fresh computer, cold boot: 8.9 seconds to healthy. Replay moves zero rows because there is no tape yet. This is the baseline the fresh-computer story is judged against.

Owner computer, refresh boot after the deploy: 9.9 seconds. The tape was already committed at the prior boot, so replay moved zero rows again — a real measurement, not a missing one, and it sets the catch-up lower bound.

Owner computer, a second refresh with real tape history: 662.7 seconds to healthy. The guest receipt shows the tape head at 240,542 events — that is how far behind the tape ran, not how many rows this boot applied (the boundary panel caught the distinction: the head position is what the snapshot reports, and the applied count only exists on a newer instrument). The health gate kept ticking a progress counter the whole time — over a hundred and seventy thousand beats — while the post-replay vocabulary migration scanned the applied tape. So the honest reading is: some seconds of tape reconstruction, then roughly eleven minutes inside the vocabulary scan, and the health gate only opens when the scan finishes. The clean per-phase split lands with the next instrument revision — the receipt that names it does not exist yet, and this report should not claim it does. That scan is a substrate problem, not a symptom: it is the dominant cost on an owner-sized tape, and it is what S2 and S3 will hit every time a computer with history reboots.

Three other findings belong to other stations. The tap interface is fully open: one guest can dial another guest's listener, the host's vmctl port, and the public internet — and because the instrument's internal endpoints are gated only by a forgeable header, that openness makes the new timeline and dial surfaces reachable from any guest, which S1 will have to close. The gateway token is on the guest kernel command line, readable by anything inside the guest, and it sits in the host's Firecracker config file. And the deploy classifier skips the active-VM refresh when only the autoputer internals change, so a deploy can go green while computers still run the old binary — that is exactly what happened with the fix deploy, and a manual refresh had to be issued. All of it is written up as problem docs now, for S1 and the deploy path respectively.

## What I would do next

The S0b slice is the live work: run the disposable-computer probe suite — capsule health map, the M9a bundle lifecycle, a Go effect, an absent runtime dependency, and a snapshot/resume — on a fresh computer, and do it without touching the owner computer again until the memory-capacity question is answered. The epoch-995 boot's eleven-minute window was survivable because the refresh curl's three hundred second timeout was the only thing that broke. A deploy that actually refreshes the owner computer on this tape will exceed that timeout. That ceiling is now on the record.

The one thing worth remembering: the instrument did what it was built to do. It measured itself, caught itself being wrong, and then measured the thing it was pointed at. The numbers it returned are the numbers S1 and S3 get to build on.

## Names and receipts

- Goal file: `docs/definitions/choir-appdev-s0-reality-boot-timeline-2026-10-01.md`
- Metamission spine: `docs/definitions/choir-supervised-app-development-metamission-2026-10-01.md`
- Receipts under `docs/evidence/`: `s0a-boot-timeline-fresh-2026-10-01.json`, `s0a-boot-timeline-owner-sized-2026-10-01.json`, `s0a-boot-timeline-owner-sized-post-refresh-2026-10-01.json`, `s0a-guest-layout-2026-10-01.json`, `s0a-runtime-closure-2026-10-01.json`, `s0a-tap-reachability-2026-10-01.json`, `s0a-gateway-token-visibility-2026-10-01.json`, `s0-probe-index-2026-10-01.json`
- Problem docs: `docs/problems/s0-gateway-token-on-kernel-cmdline-2026-10-01.md`, `docs/problems/s0-tap-egress-unfiltered-2026-10-01.md`, `docs/problems/s0-deploy-refresh-skips-autoputer-internals-2026-10-01.md`
- Instrument commits: 90af3b2f, 2ee67928, 1beed1a9, d44a719e
- Probe harness: `frontend/tests/s0a-reality-probe.mjs`
- Deploy runs: 36918174321, 36920971343, 36924036358
- Deployed commit at probe time: 1beed1a9 (guest autoputer), ff513419 (docs)
