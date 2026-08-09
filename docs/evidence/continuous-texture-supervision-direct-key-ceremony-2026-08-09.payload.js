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