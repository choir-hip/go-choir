# Continuous Texture Supervision — Joined Runtime Review

**Date:** 2026-08-08  
**Reviewed runtime candidate:** `363bf39398128fa0e1a85a19ae9a7762f92ba3dc`  
**Base deployed source:** `cdaa787bf2d006a1d4e59c1650a232f2083d8f9d`  
**Mutation class:** red  
**Effects:** OFF pending exact Linux staging acceptance and registry promotion

## Joined result

The local candidate now uses the standard lifecycle object graph, event tape,
conditional transitions, and `choir.lifecycle_command` receipts for continuous
Texture control. It does not add a second queue, callback authority, poller,
public supervision API, generic capsule route, or test-only product route.

Independent lifecycle red-team review returned **ACCEPT** on `3ac0935b` after
repairs for terminal Super evidence, complete ordered delivery, authenticated
memory-derived settlement, same-run append recovery, owner-head rebase, one
public version per turn, cancellation/report races, and unbound assignment fate.
The later runtime candidate preserves those surfaces and adds resident drain and
bounded delayed-report closure.

Independent capsule/security red-team review returned **ACCEPT** on the exact
runtime candidate `363bf393`. It found no remaining critical/high source defect
in assignment cancellation or late-receipt authority. The review confirmed:

- durable cancellation intent wins every report/freeze/candidate race;
- bounded retry retains exact delayed receipts across fate/version movement;
- terminal and unbound capsules require revoke intent, executor cleanup and a
  structured durable acknowledgement;
- restarted Linux executors must kill/delete exact orphan cgroups, detach exact
  overlays, remove state, and independently prove residue absent before receipt;
- receipt-bearing late evidence requires exact stored execution-handle identity;
- durable cancellation drains the intersection of exact trajectory runs and
  process-local resident activations without starting historical actors; and
- only one sole runtime-selected terminal assignment report receives a bounded
  detached closure after provider cancellation. Partial reports, mixed batches,
  capsule effects, injections, and later calls remain cancelled.

## Verification performed

All commands below passed on the joined source (with repeated focused runs during
repair):

```text
go test ./internal/store -count=1
go test ./internal/agentcore -run '^(Test[A-F]|Example[A-F])' -count=1
go test ./internal/agentcore -run '^(Test[G-L]|Example[G-L])' -count=1
go test ./internal/agentcore -run '^(Test[M-R]|Example[M-R])' -count=1
go test ./internal/agentcore -run '^(Test[S-Z]|Example[S-Z])' -count=1
go test ./internal/textureowner ./internal/toolregistry ./internal/types ./cmd/choir -count=1
go test -race ./internal/store ./internal/agentcore ./internal/textureowner <focused joined patterns> -count=1
go test -race ./internal/toolregistry ./internal/agentcore   -run 'Test(RunToolLoopDetachesOnlySoleSelectedTerminalEvidenceAfterProviderCancellation|DetachedAssignmentReportClosureSelectsOnlySoleTerminalResult|LateAssignmentExecutionReceiptsAuthenticateExactDetachedAuthority)$' -count=1
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test ./internal/capsule -c   -o /tmp/choir-capsule-final-linux.test
```

The Linux artifact was an x86-64 statically linked ELF. It was not executable on
the Darwin development host. This is compile evidence, not Linux behavior proof.

The Definition receipt linter passed with zero errors before this receipt was
added. The Definition dashboard remained live at `http://127.0.0.1:8787/`.

## Red-mutation receipt

- **Conjecture delta:** the joined source makes C6/C7 testable through one durable
  feedback substrate; they remain conjectures until deployed product evidence.
- **Protected surfaces:** Texture canonical writes/observation; lifecycle CAS,
  event and receipt projection; actor delivery and cancellation; authenticated
  run memory; persistent Super identity; capsule capability, cgroup, overlay,
  receipt and late-result authority; API/CLI; run acceptance and deployment.
- **Admissible evidence reached:** local focused/full/race, exact independent
  source review, and Linux cross-compile. Deployed Linux behavior is outstanding.
- **Rollback:** revert the unpublished range to
  `cdaa787bf2d006a1d4e59c1650a232f2083d8f9d`; retain additive packets/receipts as
  pending or late; keep all effects OFF.
- **Heresy delta:** `discovered` — cancellation/report, restart residue,
  execution-handle compatibility, resident drain, and cancelled-provider closure
  gaps documented before their repairs. `introduced` — none known after review.
  `repaired` — source-level instances above; no global or deployed claim yet.

## Outstanding acceptance

Publication remains blocked on one immutable pushed SHA, CI, Linux staging build
identity, authenticated repeated-cycle product proof, same-build no-SSH restart,
real capsule/cgroup/overlay cleanup, in-flight cancellation plus actual delayed
result, no-effect comparison, artifact/identity fetch, and registry closure.
