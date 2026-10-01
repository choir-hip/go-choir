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
