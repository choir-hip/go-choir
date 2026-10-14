# S0 ledger — reality + boot timeline

Append-only move log for `choir-appdev-s0-reality-boot-timeline-2026-10-01.md`.
Terse entries; consult only when auditing or `now` loses a thread.

## 2026-10-01 — session open (this worktree, devin)

- Reconciled: worktree clean on main@5e789bef; staging deployed_commit=a3cfaa00
  matches station start receipt; node-b reachable; all services active;
  corpus-dolt under transient cgroup cap (MemoryHigh=18GiB/MemoryMax=20GiB,
  OOMKills=0, no unit drop-in — durability is the capacity mission's, noted).
- Standing questions run: S0a is the spine-designated live slice; settlement
  authority is owner ratification 2026-10-01 (metamission spine, 5/5+7/7
  panels); single-state authority = the goal files; read path is vmctl
  internal API + guest tap URL; restart behavior = persisted
  boot-timeline.json per VM.
- Problem documented before any code (source-observed):
  docs/problems/s0-gateway-token-on-kernel-cmdline-2026-10-01.md —
  choir.gateway_token on kernel cmdline, world-readable inside guest; also in
  host fc-config.json + ps. S1 owns the fix.
- Scout subagent failed (upstream model quota) — mapped boot path inline
  instead: bootVM → waitForGuestReady (250ms /health poll, 503 replay-aware)
  → guest units (extract-cmdline → signers → updater → autoputer) → Run()
  phases (store open, credential exchange, replay, vocab fence, rt.Start).
- Implemented the S0a observer (commit 90af3b2f):
  - guest: bootMark() phase marks across Run()/runReplayPhase (monotonic
    since-boot via /proc/uptime calibration); GET /internal/boot/timeline
    serves marks + systemd-analyze + per-unit monotonic timestamps +
    journal boot events + mounts/store layout + build/image identity +
    missing_observers; GET /internal/diag/tcp-dial?addr=ip:port is the
    bounded guest-side reachability oracle.
  - host: BootTimeline per bootVM/ResumeVM — marks from boot_begin through
    first_healthy + guest receipt fetch; persisted to
    vm-state/<vmid>/boot-timeline.json; served via
    GET /internal/vmctl/boot-timeline?computer_id=...
  - server: SetOnListen hook marks http_listen_ready.
  - vmctl adapter: BootKind (cold|resume|recover|refresh) plumbed;
    optional-interface BootTimeline reader.
- Verified locally: go build ./..., go vet, package tests
  (vmmanager, server, vmctl, autoputer) all green.


- Pushed main@90af3b2f. CI run 36918174321 in progress. Deploy will run the
  canonical guest boot refresh → serially reboots active computers; the
  owner-computer refresh boot writes the owner-sized timeline receipt.
- Probe harness written: frontend/tests/s0a-reality-probe.mjs (deploy-exempt
  path, .mjs non-spec). Captures all six named S0a receipts into
  docs/evidence/s0*-2026-10-01.json + s0-probe-index.
- Pending: CI green → deploy → verify deployed_commit=90af3b2f → run probe →
  write evidence → commit → boundary panel + reporter + transition receipt.
- Deploy of 90af3b2f succeeded; owner computer refreshed (epoch 991,
  boot_kind=cold because RefreshVM didn't stamp yet). Receipt written but
  guest_receipt=null with guest_fetch_error="EOF". Live guest probe hit the
  same EOF → traced to a panic in parseSystemdAnalyzeTime:
  `raw[idx-200:]` underflows when "reached after" sits early in the output;
  also the durations were read off the wrong side of their labels.
- The instrument caught its own defect before station evidence depended on
  it: the observer recorded the panic as an honest fetch error rather than
  fabricating a receipt. That is the observer behaving correctly.
- Fix commit 2ee67928: exact line-bounds parse (verified against real
  systemd-analyze shapes incl. byte-0 case), guest-receipt fetch now
  retries 6×800ms (post-healthy EOF window is real), RefreshVM stamps
  boot_kind=refresh. Pushed; CI 36920971343; second deploy will write the
  first complete merged receipt (epoch 992 refresh boot, owner-sized).

- Boundary panel (convergent, partial: claude+gemini38+glm53 completed,
  quota-capped rest): verdict S1-sufficient / S3-insufficient. Unanimous
  real findings: (a) replay.sequence is head position not rows applied;
  (b) `replayed := Sequence>0` in run.go defeats the vocab_migrate fast
  path — any boot with ≥1 applied row triggers a full-tape rescan, which
  is the 662.7s epoch-995 cost; (c) reconstruct_done/vocab_fenced were
  never captured in a receipt (added post-evidence); (d) internal
  endpoints header-gated + open tap → SSRF/read surface for S1; (e)
  fetch happens at health-flip → receipts can miss runtime_started.
- New problem docs: s0-vocab-rescan-fires-on-any-replay (the substrate
  defect), s0-internal-surface-forgeable-caller (S1 amplifier).
- Fix commit 21bbabff: applied_rows counter on ReplaySnapshot (reset per
  Reconstruct, incremented per applied row), replayed = AppliedRows>0,
  replay snapshot in guest receipt carries applied_rows; fetch polls
  ~15s for runtime_started before finishing the receipt.
- Report letter corrected: the 9s/11min split is labeled inference until
  a receipt carries the marks.
- Pending: deploy 21bbabff → manual owner refresh → epoch-996 receipt
  with reconstruct_done + vocab_fenced + applied_rows + runtime_started —
  the first complete owner-sized attribution.

- OUTAGE (introduced by 21bbabff, repaired 7e412ca7): ~19:03-19:19 + a
  recurrence under the restarted vmctl, no account could boot a computer.
  fetchGuestBootTimeline's tail-out called t.mark() while holding t.mu →
  self-deadlock on a non-reentrant mutex; the call ran under m.mu, so one
  deadlocked owner fetch serialized every boot and every m.mu reader
  (resolve, boot-timeline, health loop). SIGQUIT dump of pid 434015 proves
  it: goroutine 351 [sync.Mutex.Lock 18min] fetch<-bootVM<-Refresh; ~15
  parked on m.mu.RLock. vmctl restarted via SIGQUIT; same deadlock recurred
  on the re-boot because runtime_started lands at ~407s on owner-sized
  tapes, well past the ~30s fetch window. Problem doc:
  docs/problems/s0-fetch-guest-timeline-deadlock-2026-10-01.md.
  Fix 7e412ca7: tail marks outside t.mu; fetch+finish+persist moved off
  m.mu in bootVM and ResumeVM; MarshalJSON under t.mu via pointer alias.
  Owner guest verified healthy during the outage on its real tap
  (10.200.149.2): vocab_fenced 10263ms, applied_rows=0 — the replayed-predicate
  fix works; the wedge was purely host-side.
- Divergent panel (in flight): hunting sibling lock/timeout defects in
  vmmanager+vmctl before S1 inherits the structure.

- RESOLVED (deploy fd8b2973, forced workflow_dispatch run 36945324052):
  fixed binary live; vmctl pid 681489; owner VM reattached active epoch
  1000; /boot-timeline instant 200. Post-fix probe (82s, zero errors):
  fresh registration booted computer-75ed0602 in 10.2s; owner re-refresh
  epoch 1001 healthy at 26.6s (vocab_fenced 26.2s, applied_rows=0) —
  25x faster than pre-fix. Fetch completed inside its window on both.
  Full receipt set re-captured into docs/evidence + probe index committed
  (4708a034). Deploy-cancel footgun hit 3x tonight (concurrent pushes kill
  in-flight deploys) — recorded as blocker in the station now card.
- S0a boundary receipt recorded in the station file; S0b (disposable-
  computer probes) is the live slice. The divergent panel's lock-substrate
  findings feed S0b/S1 scoping.
