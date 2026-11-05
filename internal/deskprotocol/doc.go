// Package deskprotocol is the locked, versioned protocol between Choir's
// desks (docs/design/app-agent-desks-2026-11-04.md section 5, slice D0).
//
// The protocol is everything one desk can say to another or read from the
// shared ledger: the staged intents, the commitment records they become, the
// work item, trajectory and engineering assignment lifecycles, the choir
// verbs, the transition table and the broker wire. It records no allowlist of
// who may address whom (the registry and the resolver own that, owner ruling
// R5). It is the Go-side
// counterpart of the front end's SDK surface
// (frontend/scripts/sdk-surface.mjs): a surface extracted from source, a lock
// per version, and a checker that classifies a change as none, patch, minor
// or major and refuses a change with too small a bump.
//
// Owner, 2026-11-04: a locked, versioned protocol between desks is enforced
// regardless of the rest of the architecture.
//
// # Failure modes, written before the code
//
// Each is answered by a named mechanism or test.
//
//	F1  A broken extractor yields an empty or partial surface that matches an
//	    empty lock. Answer: floors on names, verbs and desks, and a root that
//	    does not resolve is an error, never a skip (extract tests).
//	F2  A lock is edited in place, deleted or renamed. Answer: each lock
//	    carries a SHA-256 of its own body, and --base REF refuses any lock
//	    that existed at the merge base and now differs or is gone.
//	F3  A pull request weakens the checker and passes under the weakened
//	    rules. Answer: CI also runs the base's checker over the head's locks
//	    (--locks-only --base), and locks are immutable, so a loosened
//	    extractor cannot rewrite the history it is judged against.
//	F4  The extraction depends on the machine: GOOS, absolute paths, map
//	    order. Answer: a fixed linux/amd64 build context, no paths in the
//	    output, canonical sorted JSON, and a test that two extractions are
//	    byte-equal.
//	F5  The surface changes and Version does not move. Answer: the surface of
//	    Version must equal lock/<Version>.json exactly; a difference is
//	    reported with its classification and the bump it needs.
//	F6  Version moves by less than the change needs. Answer: the step check
//	    between every adjacent pair of locks, with the same rank order as the
//	    SDK checker.
//	F7  Version moves and no lock exists, or a lock's name and content
//	    disagree. Answer: both are failures.
//	F8  A new value in a closed enum is read as harmless. Answer: closed
//	    enums (the default) classify a new value as major; only an enum
//	    declared open in roots.go classifies it as minor.
//	F9  A required field added to something a desk emits reaches an older
//	    desk that does not send it. Answer: a non-optional field added to an
//	    emitted or both-way struct is major.
//	F10 The lifecycle rules drift from the table, because the store keeps
//	    inline checks. Answer: the store reads the table for the checks it
//	    holds, and a test compares each against a frozen copy of the old
//	    check over every state pair.
//	F11 Reading the table changes behavior: an unknown state, an extra
//	    transition. Answer: the same oracle test, plus the table is the
//	    only source of "terminal".
//	F12 Fixtures leak production data. Answer: every fixture is built by a
//	    test constructor; a test scans the corpus for email addresses and
//	    long opaque tokens.
//	F13 Golden decoding is lenient and hides a removed or renamed field.
//	    Answer: fixtures decode with unknown fields disallowed and must
//	    re-encode to the same JSON.
//	F14 The CI job is skipped silently. Answer: check requires it whenever
//	    the Go lane is selected, and the workflow contract test pins the
//	    wiring.
//	F15 The base ref is unavailable (shallow clone). Answer: fetch-depth 0,
//	    and a git failure is a failure of the check, not a pass.
//	F16 A verb exists only as a closure in ChoirExports, so no signature is
//	    locked. Answer: such verbs are recorded as closures; the verb set
//	    and each desk's verbs are still locked.
//	F17 The desk table or transition table is read at run time from the
//	    checker's own build (the base's), not the head's tree. Answer: the
//	    base checker never extracts; it checks the lock files only.
package deskprotocol
