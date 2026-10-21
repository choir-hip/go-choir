# Launch first pass, ledger

Append-only. One terse entry per orchestrator pass.

- 2026-10-21 17:50 UTC. c893a956a pushed (capture labels by key). Metamission drafted (f399a2e8c). Five lanes started from f399a2e8c.
- 18:05. Owner: no iPhone, no voice, the security and density checklist only, self-development items included. FP5 superseded, its worker moved to FP6 (network). FP4 rescoped to ARCH-10, ARCH-04, ARCH-03. FP7 (gateway) started as the sixth worker.
- 18:00. c893a956a deployed (CI 39645339172); the paper runs it; 27 polls ok; engine 70% busy, capture-label scan gone.
- 18:10. Owner: the Node B delegate is for engineering-desk work only; node-a hosts the owner's other project. Recorded (1aa24b84c).
- 18:20. Review round 1: sol send_back, Argon accept with edits, Devin failed. Edits adopted; worker rules tightened; build limiter sets the Go limits; registry repaired, live doc check passes.
- 18:45. Review round 2 on blob 6225eee4: Argon accept; sol send_back with two exact repairs (landing paragraph order, finish settlement), applied verbatim. FP3 group 1 returned frozen; rebased onto cf9751f36 as 809937d1e and dabc5d196; vmctl tests pass; the paper is critical_protected, so the rescue covers it; its candidate panel runs.
- 18:45. B1 timing of c893a956a: CI 15 s after push, test critical path 11 min, deploy done 14 min, paper serving 16 min, proof about 10 more: one deploy slot about every 25 minutes.
- 18:50. SEC-14 first half, 6bfe06fcf (red): every CI ssh to Node B checks pinned host keys strictly. CI 39655640676 success; its receipt read over strict checking returned the deployed base c893a956a. Prebuild and deploy were skipped (workflow-only change), so their pinned setup is proven by the next runtime deploy, FP3 group 1. Still open: CI deploys as root.
- 18:50. RES-06 reading, read-only on Node B: the paper's Firecracker holds 7,970 MiB of host RSS while its guest uses about 2.8 GB with about 5.5 GB available; the other three VMs hold 3,933, 3,165 and 1,468 MiB; the host has 9.3 of 31.1 GiB available. Guest page cache is not returned. RES-07 (what fills that cache) needs a guest-side read FP0 does not have without shell access.
- 18:55. FP3 group 1 returned with sol's four fixes (e02d578dc). Argon's review of the first candidate then finished, send back, with two findings sol did not raise: the rescue can go through RecoverVM with the old boot config and no gateway token, and ResumeVMForDesktop takes no booting claim, so a concurrent warm sweep can double-spawn or kill a booting guest. Sent to FP3 with the CI verification loop and the failed-state gap. Group 2 rulings given; group 2 waits for group 1.
