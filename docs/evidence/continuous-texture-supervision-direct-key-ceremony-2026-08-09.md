# Continuous Texture Supervision — Direct narrow-key ceremony

**Date:** 2026-08-09
**Status:** DEPLOYMENT GATE SATISFIED AT `fbc7ff5a`; AUTHORIZED ONLY WITH EXACT-OWNER PHYSICAL PRESENCE; NOT EXECUTED
**Mutation class on execution:** RED — authenticated API-key registry
**Exact payload file:** `continuous-texture-supervision-direct-key-ceremony-2026-08-09.payload.js`
**Exact file bytes SHA-256 (no final newline):** `a66562ec9964ca8d0e8a6932427f97a1a115c49fc3e59751654e12c8e36017b8`
**Once-ever label:** `cts-7ba05599-direct-narrow-8b7873810a8e`

## Problem and authority

The deployment gate is satisfied: repair
`fbc7ff5a048ed58d0f6dd02ae8462ae211eca328` passed CI/deploy 31326948312,
public proxy health reports that exact deployed commit, and selected-service
activation receipts exact-join it. The frozen payload is now authorized for one
execution only when ordinary headed persistent Chrome and native owner presence
make canonical `/auth/session` return exact retained owner
`c72404bb-3c43-4a53-8671-b5cbc48b24a7`. It remains NOT EXECUTED. Do not run it
against any other host identity, owner, origin, profile, or headless browser.
The historical once-ever `cts-7ba05599-…` label is part of the frozen accepted
bytes and must not be renamed.

The ordinary Settings API-key form cannot express the acceptance authority:
`frontend/src/lib/SettingsApp.svelte` sends only `label` and `scopes`, omits
`computer_id` and `expires_at`, and does not offer the lifecycle or
self-development scopes. The first purported-owner handoff therefore produced a
broad, unbound administrator key; subsequent source/live reconciliation showed
that target-bound registry rows were caller-supplied metadata rather than exact
owner proof, while the accidental bearer's public compute status selected a
conflicting primary epoch. The admissible result is owner ambiguity, not a
categorical different-owner claim.
Its reviewed attenuation failed locally and retired
both accidental authorities with DELETE-204/post-401 evidence.

This direct same-origin ceremony is authorized only in ordinary headed Chrome at
canonical `https://choir.news/`. Its initial canonical `GET /auth/session` may
rotate refresh/access cookies and is explicitly authorized normal RED session
renewal. It must prove session owner
`c72404bb-3c43-4a53-8671-b5cbc48b24a7` before the sole POST; binds stable computer
`computer-42850e9734d9442386c5dd8bf3afbf19`, not realization VM `vm-bb…`; uses
exactly the eight accepted scopes; expires after 105 minutes; rejects any prior
registry row or browser marker for the once-ever label; and never prints the
secret. It offers an owner-clicked 120-second copy/cancel overlay.

Any ambiguous creation, response/metadata failure, cancellation, timeout, or
clipboard failure attempts every nonce-matching DELETE, requires zero live
nonce rows, and—when the secret is known—requires cookie-free bearer HTTP 401.
If cleanup cannot be proved, the only result is a sanitized high-severity STOP;
never retry the nonce or proceed. Effects remain OFF.

Independent reviewer `broad-key-attenuation-review` returned `ACCEPT`: exact
owner, once-ever nonce, single POST, exception-safe cleanup, registry readback,
known-secret post-401, exact stable binding/scope/expiry, bounded owner click, and
no secret logging are sound. After copy, the agent must atomically persist mode
`0600`, clear the clipboard, run only the authorized read-only target probes, and
self-revoke/post-401 on any ingestion or target mismatch.

## Exact headed-browser payload

The adjacent `.payload.js` file is the canonical executable byte sequence and
the hash above covers its complete raw bytes, with no final newline. The fenced
mirror below excludes the Markdown separator newline before the closing fence.
Read the payload before execution. Run it once in DevTools Console on canonical
Choir only. Do not run it in OMP/headless Chrome, another origin, or another
profile. Verify the canonical raw file without rewriting it:

```bash
python3 -c 'import hashlib,pathlib; p=pathlib.Path("docs/evidence/continuous-texture-supervision-direct-key-ceremony-2026-08-09.payload.js"); assert hashlib.sha256(p.read_bytes()).hexdigest()=="a66562ec9964ca8d0e8a6932427f97a1a115c49fc3e59751654e12c8e36017b8"'
node --check docs/evidence/continuous-texture-supervision-direct-key-ceremony-2026-08-09.payload.js
```

```javascript
void (async () => {
  "use strict";
  const label = "cts-7ba05599-direct-narrow-8b7873810a8e";
  const ownerID = "c72404bb-3c43-4a53-8671-b5cbc48b24a7";
  const computerID = "computer-42850e9734d9442386c5dd8bf3afbf19";
  const scopes = ["computer:lifecycle","computer:self_development:read","acceptance:read","read:runtime","write:runtime","read:texture","write:texture","read:base"];
  const started = Date.now();
  const expiresAt = new Date(started + 105 * 60 * 1000).toISOString();
  const marker = `choir-key-attempt:${label}`;
  let secret = null;
  let panel = null;
  const req = (path, init = {}) => fetch(path, { credentials: "include", cache: "no-store", ...init });
  const list = async () => {
    const r = await req("/auth/api-keys");
    if (!r.ok) throw new Error(`key-list-${r.status}`);
    const d = await r.json();
    const rows = Array.isArray(d) ? d : d?.keys;
    if (!Array.isArray(rows)) throw new Error("key-list-shape");
    return rows;
  };
  const matches = rows => rows.filter(k => k && k.label === label);
  const cleanup = async () => {
    const failures = [];
    let before = [];
    try { before = matches(await list()); } catch (_) { failures.push("initial-list"); }
    for (const k of before.filter(k => !k.revoked_at && typeof k.id === "string")) {
      try {
        const r = await req(`/auth/api-keys/${encodeURIComponent(k.id)}`, { method: "DELETE" });
        if (!r.ok && r.status !== 404) failures.push("delete");
      } catch (_) { failures.push("delete-network"); }
    }
    try {
      const live = matches(await list()).filter(k => !k.revoked_at);
      if (live.length !== 0) failures.push("live-row-remains");
    } catch (_) { failures.push("final-list"); }
    if (typeof secret === "string") {
      try {
        const r = await fetch("/auth/api-keys", { credentials: "omit", cache: "no-store", headers: { Authorization: `Bearer ${secret}` } });
        if (r.status !== 401) failures.push("bearer-not-401");
      } catch (_) { failures.push("bearer-proof-network"); }
    }
    if (panel) panel.remove();
    panel = null;
    secret = null;
    if (failures.length) throw new Error("HIGH-SEVERITY STOP: cleanup could not be proved; do not retry; report this message");
  };
  const stopAfterCleanup = async message => {
    await cleanup();
    throw new Error(message);
  };

  const sr = await req("/auth/session");
  let sj = {};
  try { sj = await sr.json(); } catch (_) {}
  if (sr.status !== 200 || sj?.authenticated !== true || sj?.user?.id !== ownerID) {
    throw new Error("STOP: exact retained owner is not authenticated; no key was created");
  }
  if (matches(await list()).length !== 0) throw new Error("STOP: once-ever nonce already exists; no POST performed");
  if (localStorage.getItem(marker)) throw new Error("STOP: once-ever browser marker already exists; no POST performed");
  localStorage.setItem(marker, new Date(started).toISOString());

  let response;
  try {
    response = await req("/auth/api-keys", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ label, scopes, computer_id: computerID, expires_at: expiresAt })
    });
  } catch (_) {
    await stopAfterCleanup("STOP: ambiguous create was reconciled; do not retry this nonce");
  }

  if (response.status !== 201) {
    await stopAfterCleanup(`STOP: create returned ${response.status}; nonce reconciled; do not retry`);
  }

  try {
    const data = await response.json();
    secret = typeof data?.secret === "string" ? data.secret : null;
    if (data && typeof data === "object") delete data.secret;
    const rows = matches(await list());
    const row = rows.length === 1 ? rows[0] : null;
    const rowScopes = Array.isArray(row?.scopes) ? row.scopes : null;
    const expiry = Date.parse(row?.expires_at);
    const exact = !!row &&
      typeof row.id === "string" && row.id === data?.id &&
      row.label === label && row.computer_id === computerID &&
      rowScopes !== null && rowScopes.length === scopes.length && scopes.every(s => rowScopes.includes(s)) &&
      Number.isFinite(expiry) && expiry > started && expiry <= started + 2 * 60 * 60 * 1000 &&
      typeof secret === "string" && secret.length >= 32 && [...secret].every(c => c.charCodeAt(0) >= 33 && c.charCodeAt(0) <= 126);
    if (!exact) throw new Error("metadata-mismatch");

    panel = document.createElement("div");
    Object.assign(panel.style, { position:"fixed", inset:"20px 20px auto auto", zIndex:"2147483647", padding:"16px", background:"#111", color:"white", border:"2px solid #7dd3fc", borderRadius:"10px", font:"14px system-ui" });
    const text = document.createElement("div");
    text.textContent = "Exact retained-owner narrow key verified. Copy it now (expires in 105 minutes).";
    const copy = document.createElement("button"); copy.textContent = "Copy exact key"; copy.style.margin = "12px 8px 0 0";
    const cancel = document.createElement("button"); cancel.textContent = "Cancel and revoke";
    panel.append(text, copy, cancel); document.body.appendChild(panel);
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("copy-timeout")), 120000);
      copy.onclick = () => navigator.clipboard.writeText(secret).then(() => { clearTimeout(timer); resolve(); }, e => { clearTimeout(timer); reject(e); });
      cancel.onclick = () => { clearTimeout(timer); reject(new Error("owner-cancelled")); };
    });
    panel.remove(); panel = null; secret = null;
    console.log("Exact narrow key copied; reply copied in the agent conversation.", { id: row.id, label: row.label, computer_id: row.computer_id, scopes: rowScopes, expires_at: row.expires_at });
  } catch (_) {
    await stopAfterCleanup("STOP: validation/copy failed; child revoked with zero-live/post-401 proof; do not retry this nonce");
  }
})()
```
