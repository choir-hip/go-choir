# S2: a layered release cannot be transported — the signed offer carries payload bytes inline under a 64 MB cap

**Date:** 2026-10-04
**Status:** open. Source-traced; reproduced live on staging while running the
S2 layering acceptance (`scripts/s2_layered_update_probe.mjs`).
**Mutation class of this record:** green (problem documentation only).
**Station receiving it:** S2 layering-runtime-from-release, as the transport
slice of the deployed acceptance.

## Finding

The layering mechanism is fully landed and deployed (manifest base-join,
narchive materializer, GC-rooted private store, mount-ns exec, store-path
entrypoint — `20880d75`/`b68357a1`/`a7052d2a`/`f5460bdc`/`c7bb4a12`/
`1be8bd72`, deployed at `3142979b`). The deployed acceptance now fails one
transport step before apply: **the signed `PlatformUpdateOffer` cannot carry a
real app-layer release**, because the release binary is shipped inline and a
real autoputer is ~146 MB.

## Evidence

- `internal/platform/platform_update.go:71` — the corpusd mint
  `HandlePlatformUpdateOfferMint` decodes the request under
  `http.MaxBytesReader(w, r.Body, 64<<20)`.
- `internal/platform/platform_update.go:42-46` — `platformUpdateOfferMintRequest`
  carries `files[].bytes` (base64) inline.
- `internal/selfdevprotocol/platform_update.go:35-40` — `PlatformUpdateFile`
  has only `path/sha256/mode/bytes`; `Bytes` is base64 inline. There is no
  CAS-ref field.
- `internal/agentcore/platform_update.go:589` — `stagePlatformUpdatePayload`
  decodes `file.Bytes` inline into `incoming/<digest>`.

Live reproduction (`s2_layered_update_probe`, deploy `3142979b`):
`closure.nar` = a `nix-store --export` of the deployed autoputer's store path
= 145,980,976 bytes → ~195 MB base64 → POST /internal/computers/
platform-updates/offer rejected by the 64 MB body cap.

## Why the cap is load-bearing here, not incidental

A layered release is exactly "the base-absent app closure." For Choir, the
app layer's entrypoint is `bin/autoputer` — a real, dynamically-linked
146 MB binary whose deps resolve through the overlay. The probe that
exercises the layering path must ship that binary (a stub would not be a real
autoputer, and a stub that doesn't serve the event tape cannot promote the
route slot). So the acceptance *requires* a ~146 MB payload; inline base64 is
structurally over any sub-~200 MB cap.

## Design space (to be adjudicated)

1. **CAS-ref files in the offer** — add `ref`/`artifact_uri` to
   `PlatformUpdateFile`; corpusd mint persists the blob to CAS and emits
   `artifact+sha256://<digest>`; guest `stagePlatformUpdatePayload` resolves
   the ref through a CAS reader instead of decoding inline bytes. Most
   correct/scalable; new wire field + guest fetch path.
2. **Raise the caps** — mint body cap + whatever caps the vmctl proxy and
   guest apply impose must all clear ~200 MB base64. Cheap; pushes a large
   signed object through three transports.
3. **Chunked payload** — split the nar into <cap chunks, reassemble in
   `incoming/`. No schema change beyond file count; ugly.
4. **Side-channel the nar, sign only the digest** — transport closure.nar
   out-of-band (host→guest disk), offer binds `closure_digest`; apply reads a
   host-staged path. Bypasses the signed-offer invariant for the large blob —
## Decision (2026-10-04) — CAS-ref, adjudicated by convergent panel

The 2026-10-04 convergent panel (`agentic-consensus-20261004-094113`)
recommended **CAS-ref** unanimously among panelists that read the code
(claude, muse-spark, glm53 explicit; no dissenting mechanism). Two findings
scoped the fix:

- The 64 MB cap is in **three** places, not one: caller→mint request, and
  guest apply at `internal/agentcore/platform_update.go:653`. Raising it is
  O(release-size) on every hop and grows per release.
- `eventPayloadReader` cannot be reused as-is — `FetchPayload` is
  JSON+base64, capped at `EventPayloadMaxResponseBytes = 64MB`, and reads the
  `computer-event-payload` namespace, not `platform-update`.
- glm53 named the correct precedent: `HandleProjectionBaseBlob`
  (`internal/platform/file_cas_http.go:252`) already streams a large blob raw
  while the installer verifies the digest after download — the exact trust
  posture needed.

So the fix adds a streaming pair rather than reusing a capped JSON path:

1. `selfdevprotocol.PlatformUpdateFile` gains `ref` — exactly one of
   `bytes`/`ref`; `ref` is digest-locked to
   `artifact+sha256://<SHA256>/sha256/platform-update/<SHA256>`
   (`PlatformUpdateArtifactRef`), so the signed SHA256, not a caller URI,
   names the object. Offer signature still covers `SHA256`.
2. corpusd `PUT /internal/computers/platform-updates/blob/<sha256>` —
   `trustedInternalCaller`, streams body to temp + hashes (TeeReader),
   renames into `sha256/platform-update/<digest>` only on digest match.
3. corpusd `GET` same path — `authorizeFileCAS(event:read)`, streams the blob
   raw (`http.ServeContent`); the guest re-hashes after download, never
   trusting the wire.
4. mint `files[].ref` — validates the blob exists (`refExists` via
   `artifactPath`), emits `Ref` not `Bytes`, skips re-writing the blob.
5. `computerevent.HTTPClient.FetchBlobRaw` — streams the blob over the
   guest's authenticated connection (Bearer capability, `event:read`); guest
   client timeout widened to 10m for large streams.
6. `stagePlatformUpdatePayload` — for `Ref` files, `fetchRefPayload` streams
   into a temp file in `incoming/`, re-hashes (`sha256 == file.SHA256`),
   then atomic rename. Type-asserts `eventPayloadReader` to the blob-fetch
   interface (production `*HTTPClient` implements it).

Mutation class: **red** — platform-update authority path + CAS. Rollback is
git revert; inline `bytes` offers are unchanged and remain the small-payload
path.


## What the fix must do

- The signed offer (or a signed binding to it) must carry everything needed
  to stage `closure.nar` into `incoming/` — whether by bytes or by a
  content-addressed ref the guest can resolve under its trust boundary.
- The digest on the wire must still match the replayed nar (materializer
  already enforces `closure.nar` sha256 == `manifest.closure_digest`).
- No new trust surface: a ref must resolve to the same CAS the mint already
  writes (`sha256/platform-update/<sha>`) under the existing internal-caller
  gate.

## Verification

`scripts/s2_layered_update_probe.mjs` passes: mint accepts the layered
request, the guest stages `closure.nar`, replays it into the private store,
and execs the recorded store-path entrypoint inside the mount-ns overlay; a
base-mismatched layered offer fails closed.

## Deployed defect found during acceptance (2026-10-04, fixed `1518abc1`)

The first live probe mint attempt 400'd: the corpusd mint decodes the request
with `DisallowUnknownFields`, but `platformUpdateOfferMintRequest` had no
`layering_entrypoint` field — so the layered mint was rejected outright. The
probe pushed the returned error object, which surfaced on the guest as the
misleading refusal `"platform update: offer binds a different computer"`
(empty `computer_id` ≠ guest's bound computer). Two defects, one fix:

1. mint request gained `layering_entrypoint` (else the layered mint 400s);
2. `buildPlatformUpdateOffer` now forwards it into `updater.FinalizeManifest`
   (else even a parsing mint could never let the guest exec the overlay
   binary — the entrypoint would silently drop).

Deployed acceptance pending on `1518abc1` going live.
