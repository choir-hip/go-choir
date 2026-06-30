# Parallax: M9 — App Change / Promotion Mutation Hardening

**Conjecture (C18):** The `appchange/promotion` system can be hardened to
enforce complete capture, valid rollback refs, verifier evidence, atomic
transaction semantics, freshness checks, and author identity (SubjectID) on
every mutation — without breaking existing promotion, rollback, or roll-forward
flows.

**Class:** red — protected surface (promotion/rollback, Trace/evidence,
candidate computers)
**Worktree:** /Users/wiz/.windsurf/worktrees/go-choir/m9-mutation-hardening
**Branch:** orchestrator/m9-mutation-hardening

## Mission Requirements

Every promotion-family mutation must satisfy six hardening invariants:

1. **Complete capture** — both runtime and UI artifact digests plus the
   package manifest hash must be present. A partial capture is not promotable.
2. **Valid rollback refs** — every promotion must carry a rollback source ref
   to the previous active state. Missing rollback ref = no rollback path =
   unsafe promotion.
3. **Verifier evidence** — every promotion must carry non-empty verifier
   results with at least one passed contract. No evidence = no promotion.
4. **Atomic transaction semantics** — the adoption status update and the
   lineage advancement commit in a single database transaction. Either both
   commit or neither commits — no half-promoted state.
5. **Freshness checks** — the foreground lineage must not have moved since
   verification (CAS guard). Evidence about a stale base authorizes nothing.
6. **Author identity (SubjectID)** — every transaction records who authorized
   it and how they authenticated (SubjectID + SubjectAuthMethod).

## Changes

### 1. Author Identity — Types & Schema

**Files:** `internal/types/app_promotion.go`, `internal/store/store.go`

- Added `SubjectID` and `SubjectAuthMethod` fields to
  `types.AppChangePackageRecord` and `types.AppAdoptionRecord`.
- Added `subject_id` (VARCHAR 255) and `subject_auth_method` (VARCHAR 64)
  columns to `app_change_packages` and `app_adoptions` tables in the schema
  DDL.
- Added `ensureColumn` migrations for all four new columns in
  `store.bootstrap()` so existing databases get the columns on next startup.

### 2. Author Identity — Store SQL

**File:** `internal/store/app_promotion.go`

- Updated `UpsertAppChangePackage` INSERT and ON DUPLICATE KEY UPDATE to
  include `subject_id` and `subject_auth_method`.
- Updated `scanAppChangePackage` and `appChangePackageSelectSQL` to read the
  new columns.
- Updated `UpsertAppAdoption` INSERT and ON DUPLICATE KEY UPDATE to include
  the new columns.
- Updated `scanAppAdoption` and `appAdoptionSelectSQL` to read the new
  columns.

### 3. Atomic Transaction Semantics

**File:** `internal/store/app_promotion.go`

- Added `PromotionTransactionInput` struct carrying the adoption record and
  lineage record that must commit together.
- Added `PromoteAppAdoptionTransaction` method: opens a `BeginTx`, executes
  the adoption upsert and lineage upsert within the transaction, commits on
  success, and defers `Rollback` if not committed. This replaces the previous
  pattern of two independent `UpsertAppAdoption` + `UpsertComputerSourceLineage`
  calls that could leave a half-promoted state on partial failure.

### 4. Runtime Hardening — Promotion Gate

**File:** `internal/runtime/app_promotion.go`

- `PromoteAppAdoption` now accepts a `subjectContext` and enforces all six
  invariants in sequence:
  1. Status must be `owner_approved` (existing gate).
  2. Runtime + UI artifact digests required (existing, now commented as
     "complete capture").
  3. Package manifest hash required (new — binds source deltas, contract,
     and artifact digests).
  4. Rollback source ref required (existing, now commented).
  5. Verifier evidence required (new — `verifyPromotionEvidence` gate).
  6. Freshness CAS (existing, now commented).
  7. SubjectID + SubjectAuthMethod recorded.
  8. Atomic transaction via `PromoteAppAdoptionTransaction` (replaces two
     independent store calls).

- Added `verifyPromotionEvidence` helper: rejects empty/nil/malformed verifier
  results, rejects results with no passed contract, accepts results with at
  least one "passed" status (case-insensitive).

- Added `subjectContext` struct: carries `SubjectID` and `SubjectAuthMethod`
  through the mutation flow.

### 5. Runtime Hardening — Rollback & Roll-Forward

**File:** `internal/runtime/app_promotion.go`

- `RollbackAppAdoption` now accepts `subjectContext`, records SubjectID +
  SubjectAuthMethod, and uses `PromoteAppAdoptionTransaction` for atomic
  lineage restoration + adoption status update.
- `RollForwardAppAdoption` now accepts `subjectContext`, enforces the
  verifier evidence gate (re-promotion requires evidence), records
  SubjectID + SubjectAuthMethod, and uses `PromoteAppAdoptionTransaction`
  for atomic commit.
- Both emit `subject_id` and `subject_auth_method` in the event payload.

### 6. Runtime Hardening — Verify & Approve

**File:** `internal/runtime/app_promotion.go`

- `VerifyAppAdoption`, `StartVerifyAppAdoptionAsync`,
  `startAppAdoptionVerification`, and `ApproveAppAdoption` now accept
  `subjectContext` and record SubjectID + SubjectAuthMethod on the adoption
  record.

### 7. Runtime API — Auth Method Extraction

**File:** `internal/runtime/api.go`

- Added `authenticatedAuthMethod(r *http.Request) string`: extracts the auth
  method from the `X-Authenticated-Auth-Method` header. Returns `"cookie"` as
  a conservative default when the header is absent (legacy proxy paths and
  test harnesses that only set `X-Authenticated-User`).

**File:** `internal/runtime/api_app_promotion.go`

- All mutation handlers (create adoption, publish package, verify, approve,
  promote, rollback, roll-forward) now construct a `subjectContext` with
  `SubjectID: ownerID` and `SubjectAuthMethod: authenticatedAuthMethod(r)`
  and pass it to the runtime methods.

### 8. Proxy — Auth Method Propagation

**File:** `internal/proxy/handlers.go`

- Added `X-Authenticated-Auth-Method` to `clientIdentityHeaders` (the
  allowlist for headers forwarded to sandbox/runtime).
- The proxy auth middleware now reads `X-Proxy-Trusted-Auth-Method` (set by
  `setTrustedAuthHeaders` from `AuthResult.AuthMethod`) and forwards it as
  `X-Authenticated-Auth-Method` to the runtime.
- `setTrustedAuthHeaders` now sets `X-Proxy-Trusted-Auth-Method` from
  `authResult.AuthMethod`.
- WebSocket handlers (`HandleWS`, `HandleSuperConsoleWS`) now forward the
  auth method header to the sandbox.

**File:** `internal/proxy/app_change_packages.go`

- `HandleAppChangePackageReviewEvidence` now forwards the auth method header
  to platformd.

### 9. Tests

**File:** `internal/runtime/app_promotion_hardening_test.go`

Focused tests for the hardening invariants (no comprehensive build tag
required):

- `TestVerifyPromotionEvidenceRejectsEmptyResults` — empty `[]` rejected.
- `TestVerifyPromotionEvidenceRejectsNilResults` — nil and malformed JSON
  rejected.
- `TestVerifyPromotionEvidenceRejectsAllFailed` — all-failed results
  rejected.
- `TestVerifyPromotionEvidenceAcceptsPassedResults` — all-passed accepted.
- `TestVerifyPromotionEvidenceAcceptsMixedResults` — mixed with at least one
  pass accepted.
- `TestPromoteFreshnessCASAlreadyFresh` — fresh lineage passes CAS.
- `TestPromoteFreshnessCASRejectsStaleLineage` — moved lineage rejected with
  "re-verify" directive.
- `TestRollbackSourceRefFromProfile` — valid, empty, nil, malformed profile
  extraction.
- `TestSubjectContextDefaultsToOwnerID` — empty SubjectID falls back to
  ownerID.
- `TestAuthenticatedAuthMethodDefaultsToCookie` — absent header defaults to
  "cookie".
- `TestAuthenticatedAuthMethodPropagatesAPIKey` — "api_key" header
  propagated correctly.

## Verification

```
cd /Users/wiz/.windsurf/worktrees/go-choir/m9-mutation-hardening
nix develop -c go build ./...                                    # PASS
nix develop -c go test -race ./internal/runtime/...              # PASS
nix develop -c go test ./internal/store/... -count=1             # PASS
nix develop -c go test ./internal/proxy/... -count=1             # PASS
```

**Pre-existing failures (not introduced by M9):** The comprehensive build-tag
tests in `internal/runtime/` have pre-existing compile errors in
`prompts_test.go` and `texture_test.go` from M8 dead-code deletion (removed
`provideriface`, `AuthorKind`, `AuthorLabel` fields). These exist on the clean
branch and are unrelated to the promotion system.

## Heresy Delta

- **Discovered:** The promotion flow previously used two independent store
  calls (`UpsertAppAdoption` + `UpsertComputerSourceLineage`) with no
  transaction boundary — a failure between them could leave a half-promoted
  state (adoption marked adopted but lineage pointing at the old ref).
- **Discovered:** The promotion gate did not check verifier evidence at
  promotion time — it relied solely on the verified→approved transition.
  Evidence could be accidentally cleared between approval and promotion.
- **Discovered:** No author identity was recorded on promotion transactions.
  The transaction log could not answer "who authorized this, and how?"
- **Discovered:** The package manifest hash was not checked at promotion —
  a package with source deltas but no manifest hash could be promoted.
- **Introduced:** None — all changes are additive gates and transaction
  wrapping. Existing passing flows continue to pass.
- **Repaired:** All four discovered heresies are repaired by this mission.

## Rollback Path

The changes are backward-compatible:
- New DB columns default to empty strings (existing rows are unaffected).
- New gates only reject records that were already invalid (missing digests,
  missing rollback ref, missing evidence).
- The `subjectContext` defaults SubjectID to ownerID and AuthMethod to
  "cookie" when not explicitly provided.
- If the atomic transaction introduces a regression, reverting to the
  two-call pattern is a single-method replacement in
  `PromoteAppAdoption`, `RollbackAppAdoption`, and `RollForwardAppAdoption`.

## Residual Risks

- The `PromoteAppAdoptionTransaction` method duplicates the SQL from
  `UpsertAppAdoption` and `UpsertComputerSourceLineage` inline (to execute
  within the transaction). If the upsert SQL drifts, the transaction path
  could diverge. A future refactor should extract the SQL into shared
  helpers that accept a `sql.Writer` interface.
- The auth method default of "cookie" is conservative but could mislabel
  API-key-authenticated requests if the proxy is not updated. This is
  acceptable: the proxy is updated in this same mission.
- Staging acceptance (deployed proof on choir.news) is not yet run — this
  mission is local-proof-only at this stage.
