# Deferred Reliability Migrations

**Date:** 2026-05-14
**Status:** deferred; do not execute without an interactive mission window

This note captures detail-heavy cleanup work that is not conceptually hard, but
has very little tolerance for mechanical mistakes.

## Sandbox To Computer Code Cutover

The product noun is persistent user **computer**. The code still has a large
`sandbox` implementation surface. A full hard rename is possible, but should be
treated as a platform behavior-changing reliability mission, not a casual
cleanup.

Current observed surface after PR #4 merged:

- about 1,595 tracked non-doc `sandbox` hits across 86 files;
- Go packages and entrypoints: `cmd/sandbox`, `internal/sandbox`;
- public/internal JSON fields: `sandbox_id`, `sandbox_url`;
- environment variables: `SANDBOX_*`, `PROXY_SANDBOX_URL`,
  `VMCTL_SANDBOX_URL_BASE`, `GATEWAY_SANDBOX_TOKEN_TTL`;
- Nix package, guest VM, and systemd service names;
- gateway identity and rate-limit vocabulary;
- vmctl ownership/routing vocabulary;
- frontend bootstrap and Settings labels;
- persisted SQLite columns such as `agents.sandbox_id` and `runs.sandbox_id`.

The rename should be scripted and atomic, with an explicit exceptions manifest.
Blind replacement is unsafe because some occurrences are not product ontology:
HTML iframe `sandbox` attributes, Nix sandbox terminology, and possibly legacy
compatibility fields that need dual-read/dual-write behavior during cutover.

Required reliability shape:

```text
inventory -> exceptions manifest -> git mv paths -> case-aware rewrite
-> schema/API compatibility decision -> migrations/backfills
-> gofmt/build/tests -> staging deploy -> deployed product-path proof
```

Do not do this as manual spot edits. Use a generated replacement script and
review the full diff by category before committing.

## SQLite To Dolt Cleanup

The accepted direction is Dolt as canonical product state, with SQLite retained
for narrow hot runtime, cache, auth/session, compatibility, or transitional
roles.

The `sandbox` to `computer` cutover should be grouped with SQLite/Dolt cleanup
where the names intersect persisted runtime state. In particular, avoid
renaming SQLite columns in isolation unless the receiving Dolt schema or
compatibility boundary is clear.

Good bundled targets:

- rename runtime identity fields only where they represent the product computer;
- preserve or migrate compatibility for existing staging records;
- move new durable personal-promotion facts into Dolt-shaped records instead of
  deepening SQLite as product truth;
- leave low-level process/service names alone only when they are genuinely
  implementation names.

This mission is best saved for an interactive window where a human can inspect
the replacement manifest, migration policy, and staging evidence before the
cutover lands.
