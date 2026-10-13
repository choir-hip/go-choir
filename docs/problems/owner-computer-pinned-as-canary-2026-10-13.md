# The owner's computer is pinned as a canary and takes no deploy

Found 2026-10-13 about 14:55 UTC, after the owner asked whether their
account was being updated (it shows the deleted Universal Wire app and no
Autopaper content). Read-only checks on Node B:

- vmctl list: computer-03335285… is active on VM
  candidate-fleet-e15cb89f25d963c220319b7b (epoch 1189), warmness
  premium_always_on, divergence_status canary, construction_version
  code:sha256:499bee7b….
- Its guest /health reports build 86770ae7, built 2026-10-10 16:16 UTC.
  The current guest image is 761d94c5.
- vmctl's journal shows every deploy today (09:38, 10:30, 10:47, 11:24,
  12:12, 13:39, 14:31, 14:51) re-attaching the same VM process; none
  updated it.

Cause: a self-development candidate fleet realized the owner's computer
as a constructed computer version on 2026-10-10. vmctl classifies a
constructed version with no explicit status as "canary"
(internal/vmctl/handlers.go:910-915), and a canary is "pinned ... never
touched by automatic refresh" (internal/vmctl/ownership.go:143). So the
owner's computer has taken none of C0, C1 or C2. The metamission records
"the owner's computer takes every cut (accepted risk, §13.1)"; that has
not been true since 2026-10-10.

Open decision (the owner's computer, a red surface): move it back to
tracking the platform (vmctl's apply-platform-follow route, or a planned
restart onto the current image), which closes or resumes its open work
under the restart rule; and decide whether a constructed version should
be pinned by default at all.
