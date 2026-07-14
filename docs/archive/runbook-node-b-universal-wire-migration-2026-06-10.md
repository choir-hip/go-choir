# Runbook: Node B Imperative Ops — Universal Wire Rename (Workstream c)

Date: 2026-06-10

Status: **executed on staging Node B**; keep as operator intel for future
platform-disk migrations, edition-alias repair, and platform-VM recovery.

Related evidence:
[mission-report-wire-community-news-2026-06-09.md](mission-report-wire-community-news-2026-06-09.md)
(Workstream c completion). Architecture for the next workstream:
[universal-wire-activation-topology-2026-06-10.md](universal-wire-activation-topology-2026-06-10.md).

---

## Scope

These steps are **imperative SSH mutations** on Node B. They are not applied by
`origin/main` deploy alone. Code rename landed in commits `2a8afddf` (product
ids/routes) and `eb9cb93c` (proxy routes `/api/universal-wire/stories` to the
platform computer). Disk, Dolt, ownership, and edition state required manual
ops documented here.

**Do not repeat** the 2026-06-09 platform VText purge (432 docs) unless
deliberately resetting the corpus — that purge is a separate incident documented
in the mission report Slice 0 section.

---

## Access and constants

| Item | Value |
|------|--------|
| SSH | `ssh node-b` (operator alias to `root@147.135.70.196`) |
| Platform owner id | `universal-wire-platform` |
| Platform desktop id | `platform` |
| Platform VM id | `vm-universal-wire-platform` |
| Legacy VM id (remove) | `vm-global-wire-platform` |
| Platform data disk | `/var/lib/go-choir/vm-state/vm-universal-wire-platform/data.img` |
| Ownership registry | `/var/lib/go-choir/vm-state/ownerships.json` |
| Sourcecycled runtime pointer | `/var/lib/go-choir/platform-wire-runtime.env` |
| Dolt workspace (inside disk) | `state.vtext/vtext/` — **not** the empty `state` file at disk root |
| Edition alias | `universal-wire/Wire.vtext` |
| Edition doc id (post-migration) | `5f75737f-c373-4031-81e7-21acd5b661c1` |
| Edition revision id | `84730f60-3731-410f-8d69-658f40bd4eb3` |

**IP warning:** Firecracker guest IPs change on every `go-choir-vmctl` restart.
Always read the live URL from `ownerships.json` or `platform-wire-runtime.env`.
Stale `sandbox_url` with `state=active` causes proxy **502** (ownership says up,
guest is down or on a new IP).

---

## Safety checklist

1. **Stop or avoid concurrent platform Firecracker** before copying `data.img`.
2. **Backup** before mutating `ownerships.json` or `data.img`:
   - `ownerships.json.backup-before-universal-wire-migration-<timestamp>`
   - `data.img.pre-migration-empty` (if replacing empty new disk with legacy)
3. **Unmount** loop mounts when done (`umount`, `losetup -D`). Leftover mounts
   block vmctl from using the disk.
4. **Dolt commit** after in-place SQL so the guest sees consistent state on boot.
5. **Restart vmctl** after disk/ownership edits so Firecracker picks up config.
6. **Do not hand-edit** tracked repo files on Node B as a substitute for deploy.

---

## Operation A — Platform VM disk cutover (`vm-global-wire-platform` → `vm-universal-wire-platform`)

**When:** Deploy introduced `vm-universal-wire-platform` with an empty disk;
legacy corpus lived on `vm-global-wire-platform/data.img`.

**Steps (executed 2026-06-10):**

1. Stop duplicate Firecracker processes for both VM ids (read `firecracker.pid`).
2. Backup empty new disk: `cp -a NEW_DIR/data.img NEW_DIR/data.img.pre-migration-empty`
3. Copy legacy: `cp -a OLD_DIR/data.img NEW_DIR/data.img`
4. Patch `NEW_DIR/fc-config.json`:
   - `vm-global-wire-platform` → `vm-universal-wire-platform`
   - `global-wire-platform` → `universal-wire-platform`
   - path segments `/vm-state/vm-global-wire-platform/` → `/vm-state/vm-universal-wire-platform/`

**Verify:** `ls -lh NEW_DIR/data.img` shows legacy size (~GB), not empty sparse image.

---

## Operation B — Dolt `owner_id` migration (inside `data.img`)

**When:** Code expects owner `universal-wire-platform`; disk rows still say
`global-wire-platform`.

**Mount (read-write):**

```bash
MNT=/tmp/uw-mnt
mkdir -p "$MNT"
LOOP=$(losetup -f --show /var/lib/go-choir/vm-state/vm-universal-wire-platform/data.img)
mount "$LOOP" "$MNT"
cd "$MNT/state.vtext/vtext"   # critical path — not $MNT/state
```

**Bulk update** (pattern used on staging):

```bash
TABLES=$(dolt sql -r csv -q \
  "SELECT DISTINCT table_name FROM information_schema.columns
   WHERE table_schema='vtext' AND column_name='owner_id'
   AND table_name NOT LIKE 'global_wire_%'" | tail -n +2)
for t in $TABLES; do
  n=$(dolt sql -r csv -q "SELECT COUNT(*) FROM $t WHERE owner_id='global-wire-platform'" | tail -1)
  if [ "$n" != "0" ] && [ -n "$n" ]; then
    dolt sql -q "UPDATE $t SET owner_id='universal-wire-platform' WHERE owner_id='global-wire-platform'"
  fi
done
```

**Alias path renames** (if old edition alias rows exist):

```bash
dolt sql -q "UPDATE vtext_document_aliases SET source_path='universal-wire/Wire.vtext'
  WHERE owner_id='universal-wire-platform' AND source_path='global-wire/Wire.vtext'"
dolt sql -q "UPDATE vtext_document_aliases SET source_path=REPLACE(source_path,'global-wire/','universal-wire/')
  WHERE owner_id='universal-wire-platform' AND source_path LIKE 'global-wire/%'"
```

**On-disk style file** (if present):

```bash
cd "$MNT/files"
[ -f global-wire-article-vtext.vtext ] && mv global-wire-article-vtext.vtext universal-wire-article-vtext.vtext
```

**Commit:**

```bash
cd "$MNT/state.vtext/vtext"
dolt sql -q "CALL DOLT_COMMIT('-Am', 'staging: migrate global-wire-platform owner to universal-wire-platform')"
```

**Staging evidence:** 14 tables, ~56k rows updated; Dolt commit
`atid6768gaf8s4fek42gp8k9shij9u5s`.

**Unmount:**

```bash
umount "$MNT" && losetup -d "$LOOP"
```

---

## Operation C — Create edition alias `universal-wire/Wire.vtext`

**When:** Article VTexts exist but no edition alias — `/api/universal-wire/stories`
returns honest-empty `source=universal-wire-no-edition` despite articles on disk.

**Idempotent check:**

```bash
dolt sql -r csv -q "SELECT doc_id FROM vtext_document_aliases
  WHERE owner_id='universal-wire-platform' AND source_path='universal-wire/Wire.vtext'"
```

**Create** (staging pattern — transclude up to 12 article VTexts with
`article_version:true` and `source:edit_vtext` in revision metadata):

- Insert `vtext_documents` + `vtext_revisions` for edition doc/rev
- Edition body: `# Wire` + markdown transclusion links `vtext:<doc_id>` per article
- Insert `vtext_document_aliases` row for `universal-wire/Wire.vtext`

**Staging evidence:**

| Field | Value |
|-------|--------|
| doc_id | `5f75737f-c373-4031-81e7-21acd5b661c1` |
| revision_id | `84730f60-3731-410f-8d69-658f40bd4eb3` |
| transcluded articles | 12 |
| Dolt commit | `pr0pf4grqgiboibc8i68qkft95pvtp7j` |

**Verify inside guest** (after vmctl restart):

```bash
BASE=$(grep SOURCE_SERVICE_RUNTIME_BASE_URL /var/lib/go-choir/platform-wire-runtime.env | cut -d= -f2)
curl -fsS "$BASE/health"
# Authenticated product path (preferred):
# GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 npm run e2e -- tests/universal-wire-staging-acceptance.spec.js
```

---

## Operation D — Drop legacy `global_wire_*` Dolt tables

**When:** Workstream (b) deleted Go store/schema for StoryGraph; tables may
remain on old platform disks.

```bash
cd "$MNT/state.vtext/vtext"   # mounted rw
mapfile -t GW_TABLES < <(dolt sql -r csv -q "SHOW TABLES LIKE 'global_wire%'" | tail -n +2)
for t in "${GW_TABLES[@]}"; do
  dolt sql -q "DROP TABLE IF EXISTS \`$t\`"
done
dolt sql -q "CALL DOLT_COMMIT('-Am', 'staging: drop legacy global_wire tables')"
```

**Staging evidence:** 28 tables dropped (same commit family as Operation C).

**Verify:**

```bash
dolt sql -r csv -q "SHOW TABLES LIKE 'global_wire%'" | tail -n +2 | wc -l   # expect 0
```

---

## Operation E — Prune stale ownership + refresh runtime env

**When:** `ownerships.json` still lists `vm-global-wire-platform` or stale
`sandbox_url` after restart.

**Backup and prune** (requires `jq` on host):

```bash
cp /var/lib/go-choir/vm-state/ownerships.json \
  /var/lib/go-choir/vm-state/ownerships.json.backup-before-universal-wire-migration-$(date -u +%Y%m%dT%H%M%SZ)
jq 'if type=="object" and has("ownerships") then
  .ownerships |= map(select(.vm_id != "vm-global-wire-platform" or .desktop_id != "platform"))
else map(select(.vm_id != "vm-global-wire-platform" or .desktop_id != "platform")) end' \
  /var/lib/go-choir/vm-state/ownerships.json > /tmp/ownerships.json.new
mv /tmp/ownerships.json.new /var/lib/go-choir/vm-state/ownerships.json
```

**Restart and repoint sourcecycled:**

```bash
systemctl restart go-choir-vmctl
sleep 20
# vmctl rewrites platform-wire-runtime.env on warm boot — prefer that file over manual IP
cat /var/lib/go-choir/platform-wire-runtime.env
systemctl try-restart go-choir-sourcecycled
```

**Verify platform ownership** (example shape after successful migration):

```json
{
  "vm_id": "vm-universal-wire-platform",
  "user_id": "universal-wire-platform",
  "desktop_id": "platform",
  "warmness_class": "public_platform",
  "sandbox_url": "http://10.203.102.2:8085",
  "state": "active"
}
```

---

## Operation F — Code deploy: proxy platform routing (not SSH)

**Problem:** Authenticated `/api/universal-wire/stories` resolved to the
caller's personal computer → empty stories despite platform edition data.

**Fix:** commit `eb9cb93c` — `protectedAPIResolveTarget()` routes
`/api/universal-wire/stories` to `universal-wire-platform` / `platform` via
vmctl lookup (not user `desktop_id=primary`).

**Verify:**

```bash
curl -fsS https://choir.news/health | jq '.build.deployed_commit'
# expect eb9cb93cee6e92c525d464598c09f9dc65b78f21 or later
```

---

## Operation G — Recovery: platform VM down / stale IP / proxy 502

**Symptoms:**

- `curl $sandbox_url/health` times out
- No Firecracker process for `vm-universal-wire-platform`
- `choir.news` lifecycle shows `http_502` on universal-wire stories
- `ownerships.json` shows `state=active` but guest IP is wrong

**Recovery:**

```bash
systemctl restart go-choir-vmctl
sleep 30
ps aux | rg vm-universal-wire-platform
jq -r '.ownerships[] | select(.vm_id=="vm-universal-wire-platform") | .sandbox_url,.state' \
  /var/lib/go-choir/vm-state/ownerships.json
BASE=$(grep SOURCE_SERVICE_RUNTIME_BASE_URL /var/lib/go-choir/platform-wire-runtime.env | cut -d= -f2)
curl -m15 -fsS "$BASE/health"
systemctl try-restart go-choir-sourcecycled
```

**Staging evidence (2026-06-10):** restart moved guest from stale `10.203.99.2`
to `10.203.102.2`; health `ready`, deploy commit `eb9cb93c`; Playwright
acceptance **1 passed**.

---

## Acceptance proof (product path)

```bash
cd frontend
node scripts/setup-auth-state.mjs --baseUrl https://choir.news
GO_CHOIR_RUN_UNIVERSAL_WIRE_STAGING=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news \
  npm run e2e -- tests/universal-wire-staging-acceptance.spec.js
```

**Expected API shape** (authenticated):

| Field | Expected |
|-------|----------|
| `source` | `universal-wire-edition-vtext` |
| `edition.source_path` | `universal-wire/Wire.vtext` |
| `edition.doc_id` | `5f75737f-c373-4031-81e7-21acd5b661c1` |
| `stories.length` | > 0 when edition transcludes article VTexts |

---

## What deploy *does* handle (do not redo manually)

- NixOS service units, `nix/node-b.nix` user-agent strings, vmctl warm loop for
  `WarmnessClassPublicPlatform`
- Proxy code path after `eb9cb93c`
- `EnsureUniversalWirePlatformComputer` naming in vmctl (post `2a8afddf`)

## What still requires code (Workstream d — not Node B SSH)

Per-cycle reconciler on ingestion handoff, processor→researcher delegation,
reconciler `spawn_agent`→vtext, debounced publish-triggered reconciler, and
staging ingestion-chain proof — see activation topology doc.
