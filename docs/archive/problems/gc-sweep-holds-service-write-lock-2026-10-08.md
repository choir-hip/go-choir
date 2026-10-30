# Artifact GC sweep holds the platform service write lock — fresh boots fail

Date: 2026-10-08. Mutation class: red (credential issuance / VM realization).
Introduced by `0f7c58ba` (recovery policy implementation). Discovered on
staging during the admission-refusal acceptance run.

## Evidence

- Disposable registration at 22:36:55Z (user `76bc9e2d…`,
  `computer-61a31a3d…`) went `failed` immediately.
- vmctl, 22:37:00Z: `computer credential request failed for
  vm-46385bd7…-epoch-12932: Post
  "http://127.0.0.1:8086/internal/computers/credentials/issue": context
  deadline exceeded` → `Firecracker boot failed …: realization credential
  unavailable`.
- corpusd, 22:39:56Z: `artifact gc: mode=dry-run deleted=10000 …` — an hourly
  sweep was in progress across the failed request.

## Cause

`0f7c58ba` added `s.writeMu.Lock()` for the full duration of
`Service.RunArtifactGC` (`internal/platform/artifact_gc.go`) to serialize
explicit projection-base pins against live-set capture and deletion.
`writeMu` is the service-wide write mutex: credential envelope issuance
(`credential_envelope.go`), lifecycle control, watermark POST and other
writers take it. A sweep — minutes long even in dry-run mode — therefore
blocks every credential issuance, so any VM boot or resume during a sweep
times out. The pre-landing review and the frozen-candidate panel did not
flag the lock scope; no test runs GC concurrently with credential issuance.

## Impact

Any fresh realization (registration, cold start, resume needing a new
realization credential) during an hourly sweep window fails. Observed: one
failed disposable boot in the last 6h. The owner computer is exposed
whenever it wakes inside a sweep.

## Fix shape

Scope the serialization to what it protects: a dedicated GC/pin mutex shared
only by `RunArtifactGC` and the projection-base pin handler. Pins are rare
operator/restore actions; blocking them during a sweep is acceptable.
Credential issuance and every other writer stop contending with GC.
Regression: credential issuance must complete while a GC sweep holds its
lock.
