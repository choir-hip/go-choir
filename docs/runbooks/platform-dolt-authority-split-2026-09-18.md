# Runbook: platform-dolt authority split (Store A / Store B) — 2026-09-18

Fenced cutover that moves the corpus (Autopaper object graph) tables out of the
`platform` Dolt repo into a dedicated `corpus` repo served by
`go-choir-corpus-dolt` (port 13307). Design:
`docs/archive/platform-dolt-storage-normalization-2026-09-18.md` (Move 2).

**Historical procedure (the split is done).** `sourcecycled` was deleted 2026-10-13 (Cut 2 unit 2b) and "world-wire" in older text means Autopaper; Store B is the corpus Dolt. The unit names below are those of the 2026-09-18 cutover.

**Tools retired 2026-10-12, see git history:** `scripts/dolt-dump-split`, `scripts/og-body-backfill`, `scripts/og-delta-sync`.

**Class:** red (canonical platform store topology change).
**Rollback:** the pre-cutover dump is the rollback artifact; the env flip is
a file delete + restart. Never dual-write.

## Preconditions

- `go-choir-corpus-dolt.service` deployed and `active` (nix unit on 13307).
- `corpusd` and `sourcecycled` running the build that accepts
  `CORPUSD_CORPUS_DOLT_DSN` / `SOURCECYCLED_DOLT_DSN` and tolerates the
  absent `corpus-dsn.env` (they share the Store A pool until the flip).
- `scripts/dolt-dump-split/main.go` copied to Node B (`/root/dolt-dump-split.go`); stdlib-only, runs as `go run /root/dolt-dump-split.go` with no module.
- Low-traffic window: world-wire + event APIs go dark for the dump/import
  duration (~1–1.5 h at current sizes).

## Steps

```bash
# 0. Pre-check: og_* must hold zero computer-scoped rows. Abort if nonzero —
#    a row-level split is out of scope for this runbook.
mysql -h 127.0.0.1 -P 13306 -u root platform \
  -e "SELECT COUNT(*) FROM og_objects WHERE computer_id != '';
      SELECT COUNT(*) FROM og_edges WHERE computer_id != '';"
# expect: 0, 0

# 1. Quiesce writers (fence). Order matters: API services first, then the
#    ingestion daemon, so no in-flight write lands mid-dump.
systemctl stop go-choir-sourcecycled go-choir-corpusd
# vmctl stays up: it writes only Store A tables (computer_*, route ledger).

# 2. Dump the whole platform repo (works against the live sql-server repo;
#    writers are already fenced so the dump is the cutover snapshot).
cd /var/lib/go-choir/platform-dolt/platform
HOME=/var/lib/go-choir/platform-dolt dolt dump -fn /root/platform-dump-$(date +%Y%m%d).sql
# ~20G, tens of minutes. KEEP THIS FILE — it is the rollback artifact.

# 3. Split the dump by authority.
cd /root && go run /root/dolt-dump-split.go store-a.sql store-b.sql \
  < platform-dump-$(date +%Y%m%d).sql
# The splitter aborts on any table in neither set — do not bypass that.

# 4. Import Store B into the corpus repo (offline import into the repo dir;
#    stop corpus-dolt first so dolt sql can open the repo).
systemctl stop go-choir-corpus-dolt
cd /var/lib/go-choir/corpus-dolt/corpus
HOME=/var/lib/go-choir/corpus-dolt dolt sql < /root/store-b.sql
# Seal the import as the corpus repo's base commit so the debounced
# committer diffs against a clean head.
HOME=/var/lib/go-choir/corpus-dolt dolt sql \
  -q "CALL DOLT_COMMIT('-Am','authority split: corpus store import')"
systemctl start go-choir-corpus-dolt

# 5. Verify Store B before the flip.
mysql -h 127.0.0.1 -P 13307 -u root corpus \
  -e "SELECT object_kind, COUNT(*) FROM og_objects GROUP BY object_kind
      ORDER BY 2 DESC LIMIT 5; SELECT COUNT(*) FROM items;
      SELECT COUNT(*) FROM og_edges;"
# counts must match the pre-dump live values.

# 6. Flip the DSNs (host-local env file; no redeploy). Since 2026-10-22 the
#    file lives in /var/lib/go-choir/unit-env, which every unit hides as a
#    directory (docs/problems/host-containment-gaps-after-fp3-cut-2-2026-10-22.md);
#    /var/lib/go-choir/corpus-dsn.env is only a link to it.
cat > /var/lib/go-choir/unit-env/corpus-dsn.env <<'EOF'
CORPUSD_CORPUS_DOLT_DSN=root@tcp(127.0.0.1:13307)/corpus?parseTime=true&multiStatements=true&clientFoundRows=true
SOURCECYCLED_DOLT_DSN=root@tcp(127.0.0.1:13307)/corpus?parseTime=true&multiStatements=true&clientFoundRows=true
EOF
systemctl start go-choir-corpusd go-choir-sourcecycled

# 7. Verify two sql-server processes and correct routing.
systemctl is-active go-choir-platform-dolt go-choir-corpus-dolt \
  go-choir-corpusd go-choir-sourcecycled
# event queries hit Store A only:
journalctl -u go-choir-corpusd --since "-5 min" | grep -i "corpus\|13307"
curl -s https://choir.news/health
# world-wire read path serves from Store B (publication/og_objects queries).

# 8. Drop the B tables from Store A ONLY after a clean verification window
#    (recommend ≥24h). Until then the platform repo still carries them —
#    they are dead weight but harmless, and they are the fast rollback.
```

## Rollback

```bash
# Fast path (B tables still present in Store A). Remove the file in
# unit-env: `rm /var/lib/go-choir/corpus-dsn.env` removes only the link and
# leaves corpusd on Store B.
rm /var/lib/go-choir/unit-env/corpus-dsn.env
systemctl restart go-choir-corpusd go-choir-sourcecycled
systemctl stop go-choir-corpus-dolt   # optional

# Full path (B tables already dropped from Store A):
# re-import /root/store-b.sql into the platform repo, then the fast path.
```

## Post-cutover cleanup (separate change, after verification window)

```bash
# Drop Store B tables from the platform repo, then GC to reclaim chunks.
mysql -h 127.0.0.1 -P 13306 -u root platform -e "
  SET FOREIGN_KEY_CHECKS=0;
  DROP TABLE artifact_blobs, artifact_manifests, citation_edges,
    consent_records, cycle_events, cycles, fetches, ingestion_events,
    issues, items, og_edges, og_objects, platform_subjects,
    platform_texture_documents, platform_texture_revisions,
    platform_vtext_documents, platform_vtext_revisions, processor_requests,
    proposal_delivery_records, provenance_activities, provenance_agents,
    provenance_edges, provenance_entities, public_routes,
    publication_policies, publication_proposals, publication_source_entities,
    publication_transclusions, publication_version_proposals,
    publication_versions, publications, reconciler_requests,
    retrieval_manifests, retrieval_sources, retrieval_spans, review_records,
    rollback_refs, sources, verifier_attestations;"
# Then the squash runbook (docs/evidence/platform-dolt-oldgen-218g-…):
# offline dolt gc --full on the platform repo.
```
