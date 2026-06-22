# Staging Live Texture Debugging Runbook

This runbook is for read-only diagnosis of a live staging Texture when an
operator has an account email, a visible document title, or a screenshot.

Use this only for debugging. Do not mutate live user state from these commands.

## 1. Resolve Account To Owner ID

On Node B, the auth store maps email addresses to owner ids:

```sh
scp -q node-b:/var/lib/go-choir/auth/auth.db /tmp/choir-auth-node-b.db
python3 - <<'PY'
import sqlite3
email = "owner@example.com"
conn = sqlite3.connect("/tmp/choir-auth-node-b.db")
row = conn.execute("select id, email from users where email = ?", (email,)).fetchone()
print(row)
PY
```

## 2. Resolve Owner To Active Computer

Copy the VM ownership index and locate the owner/desktop entry:

```sh
scp -q node-b:/var/lib/go-choir/vm-state/ownerships.json /tmp/choir-ownerships-node-b.json
python3 - <<'PY'
import json
owner_id = "OWNER_ID"
desktop_id = "primary"
data = json.load(open("/tmp/choir-ownerships-node-b.json"))
for item in data.get("ownerships", []):
    if item.get("user_id") == owner_id and item.get("desktop_id") == desktop_id:
        print(item)
PY
```

The important fields are `vm_id`, `sandbox_url`, `state`, `last_active_at`, and
`epoch`.

If needed, confirm the VM boot config:

```sh
ssh node-b 'cat /var/lib/go-choir/vm-state/VM_ID/fc-config.json'
```

## 3. Confirm Sandbox Build Identity

Use the sandbox URL from the ownership record:

```sh
ssh node-b 'curl -fsS http://SANDBOX_HOST:SANDBOX_PORT/health'
```

Record `build.commit`, `deployed_commit`, `deployed_at`, and `sandbox_id`.

## 4. Read Texture Documents

The sandbox API trusts headers injected by the proxy. For direct Node B
debugging, set the same trusted owner headers manually:

```sh
ssh node-b 'curl -fsS \
  -H "X-Authenticated-User: OWNER_ID" \
  -H "X-Authenticated-Email: owner@example.com" \
  http://SANDBOX_HOST:SANDBOX_PORT/api/texture/documents'
```

Find the document by title, then read revisions:

```sh
ssh node-b 'curl -fsS \
  -H "X-Authenticated-User: OWNER_ID" \
  -H "X-Authenticated-Email: owner@example.com" \
  "http://SANDBOX_HOST:SANDBOX_PORT/api/texture/documents/DOC_ID/revisions?limit=20"'
```

For source/citation bugs, inspect each revision for:

- `version_number`
- `author_kind`
- `content`
- `body_doc`
- `source_entities`
- `metadata.worker_updates_consumed`
- visible `source_ref` / `source_embed` nodes in `body_doc`

## 5. Read Texture Diagnosis Bundle

The diagnosis endpoint includes document state, revision structures, runs,
events, channel messages, and evidence:

```sh
ssh node-b 'curl -fsS \
  -H "X-Authenticated-User: OWNER_ID" \
  -H "X-Authenticated-Email: owner@example.com" \
  "http://SANDBOX_HOST:SANDBOX_PORT/api/texture/documents/DOC_ID/diagnosis?limit=20"'
```

For researcher-to-Texture source propagation, check:

- channel `messages[].content` for `Refs:` and `Evidence:` sections
- `runs[].metadata.worker_update_ids`
- `revisions[].metadata.worker_updates_consumed`
- `revisions[].source_entities`
- `revision_structures[].source_marker_count`
- evidence records with `metadata.content_id`

Native Texture citations require typed source substrate. Useful refs include:

- `source_service_item:<item_id>`
- `content_item:<content_id>` or `content_id:<content_id>`
- `evidence_id:<evidence_id>` where the evidence record has a usable
  `source_uri` or `metadata.content_id`
- execution evidence refs such as command outputs, test runs, diffs, packages,
  screenshots, videos, and benchmark artifacts once their target kinds are
  represented in the source contract

Raw URLs and prose mentions are diagnostics only. They should not become native
Texture sources by scraping.

`update_coagent` is not just a status channel. It is the typed handoff envelope
from researcher, super, vsuper, co-super, processor, reconciler, and future
actors into a Texture-owned artifact. For any claim the canonical Texture may
need to cite later, the update should include a typed source handle in
`evidence_ids`, `refs`, `artifacts`, `tests`, or the structured `evidence`
array. Examples:

- researcher: `source_service_item:<id>`, `content_item:<id>`, or
  `evidence_id:<id>`
- super/vsuper/co-super: command output artifact, diff hunk, patch/package id,
  test run, screenshot/video proof, benchmark log
- processor/reconciler: source item ids, story graph ids, publication refs, or
  corpus evidence records

Plain findings are useful supervision narrative, but they are not native
Texture sources until paired with typed evidence substrate.

## 6. Common Interpretation

If revisions have `body_doc` but empty `source_entities`, the frontend cannot
render native citation points. Look upstream at researcher `update_coagent`
payloads and runtime fallback output.

If researcher messages contain facts but no typed refs or evidence ids, the
handoff is a source-substrate problem, not a renderer problem.

If `source_entities` exist but no `source_ref` / `source_embed` nodes appear in
`body_doc`, the Texture write path did not insert native source nodes.

If source nodes exist in `body_doc` but the UI has no citation bubbles, inspect
the frontend structured renderer.
