const EMPTY_VALUES = new Set(["", "none", "unknown", "not_applicable", "not applicable", "n/a"]);

const GATE_ADJUDICATIONS = new Set(["accept", "repair", "reject", "escalate"]);

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function humanize(value) {
  return String(value ?? "")
    .replace(/^[A-Z]\d+-/i, "")
    .replaceAll(/[_-]+/g, " ")
    .replaceAll(/([a-z\d])([A-Z])/g, "$1 $2")
    .replace(/^./, (character) => character.toUpperCase());
}

function isRecord(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isMeaningful(value) {
  if (value === null || value === undefined) return false;
  if (Array.isArray(value)) return value.length > 0;
  if (isRecord(value)) return Object.keys(value).length > 0;
  return !EMPTY_VALUES.has(String(value).trim().toLowerCase());
}

function values(value) {
  if (value === null || value === undefined) return [];
  return Array.isArray(value) ? value : [value];
}

function text(value, fallback = "Not recorded") {
  if (!isMeaningful(value)) return fallback;
  if (Array.isArray(value)) return value.map((item) => scalarText(item)).filter(Boolean).join(", ") || fallback;
  if (isRecord(value)) return summarizeRecord(value) || fallback;
  return String(value);
}

function scalarText(value) {
  if (!isMeaningful(value)) return "";
  if (isRecord(value)) return summarizeRecord(value);
  if (Array.isArray(value)) return value.map((item) => scalarText(item)).filter(Boolean).join(", ");
  if (typeof value === "boolean") return value ? "Yes" : "No";
  return String(value);
}

function summarizeRecord(record, limit = 4) {
  const entries = Object.entries(record ?? {}).filter(([, value]) => isMeaningful(value));
  return entries
    .slice(0, limit)
    .map(([key, value]) => `${humanize(key)}: ${scalarText(value)}`)
    .join(" · ") + (entries.length > limit ? ` · ${entries.length - limit} more` : "");
}

function truncate(value, length = 190) {
  const string = text(value, "");
  return string.length > length ? `${string.slice(0, length - 1).trimEnd()}…` : string;
}

function renderList(items, className = "plain-list", empty = "Nothing recorded") {
  const populated = values(items).filter(isMeaningful);
  if (!populated.length) return `<p class="empty">${escapeHTML(empty)}</p>`;
  return `<ul class="${className}">${populated.map((item) => `<li>${escapeHTML(scalarText(item))}</li>`).join("")}</ul>`;
}

function renderDefinitionList(record, preferredKeys) {
  if (!isRecord(record)) return `<p class="body-copy">${escapeHTML(text(record))}</p>`;
  const keys = preferredKeys?.filter((key) => Object.hasOwn(record, key)) ?? Object.keys(record);
  const entries = keys.filter((key) => isMeaningful(record[key]));
  if (!entries.length) return `<p class="empty">Nothing recorded</p>`;
  return `<div class="prose-fields">${entries.map((key) => `<p><strong class="field-label">${escapeHTML(humanize(key))}:</strong> <span>${escapeHTML(scalarText(record[key]))}</span></p>`).join("")}</div>`;
}

function statusLabel(value) {
  const normalized = String(value ?? "").trim().toLowerCase();
  const labels = {
    working: "In progress",
    complete: "Complete",
    checkpoint_incomplete: "Checkpoint — incomplete",
    blocked_incomplete: "Blocked — incomplete",
    superseded: "Superseded",
    reconciled: "Reconciled",
    reconciling: "Reconciling",
  };
  return labels[normalized] ?? humanize(value || "Unknown");
}

function statusTone(value) {
  const normalized = String(value ?? "").trim().toLowerCase();
  if (["complete", "accepted", "accept", "passed", "pass"].includes(normalized)) return "positive";
  if (["blocked_incomplete", "blocked", "reject", "rejected", "failed", "fail"].includes(normalized)) return "critical";
  if (["working", "reconciling", "repair", "escalate", "checkpoint_incomplete"].includes(normalized)) return "attention";
  return "neutral";
}

function gateAdjudication(value) {
  if (typeof value !== "string") return "";
  const normalized = value.trim().toLowerCase();
  return GATE_ADJUDICATIONS.has(normalized) ? normalized : "";
}

function renderRecordCollection(items, preferredKeys, empty = "Nothing recorded") {
  const populated = values(items).filter(isMeaningful);
  if (!populated.length) return `<p class="empty">${escapeHTML(empty)}</p>`;
  return `<div class="detail-records">${populated.map((item) => `<div class="detail-record">${renderDefinitionList(item, preferredKeys)}</div>`).join("")}</div>`;
}

function distinctMeaningful(items) {
  return [...new Set(items.filter(isMeaningful))];
}

function weakMeasures(model, now) {
  return distinctMeaningful([
    ...values(model?.weakMeasures),
    ...values(model?.weak_measures),
    ...values(now?.weak_measures),
  ]);
}

function dissentItems(model, now) {
  return distinctMeaningful([...values(model?.dissent), ...values(now?.dissent)]);
}

function gateReceiptCandidates(model) {
  return [
    ...values(model?.evidence?.receipts),
    ...values(model?.evidence?.obtained),
    ...values(model?.orchestration?.receipts),
    ...values(model?.now?.receipts),
  ].filter(isMeaningful);
}

function inferGateStatus(gate, model) {
  const gateId = gate?.id;
  if (gateId === null || gateId === undefined || gateId === "") {
    return { label: "Not reviewed", tone: "neutral" };
  }

  for (const receipt of gateReceiptCandidates(model)) {
    if (!isRecord(receipt) || receipt.gate_id !== gateId) continue;
    const adjudication = gateAdjudication(receipt.adjudication);
    if (adjudication) return { label: statusLabel(adjudication), tone: statusTone(adjudication) };
  }
  return { label: "Not reviewed", tone: "neutral" };
}

function phaseItems(model) {
  const orchestration = model?.orchestration ?? {};
  const phases = values(orchestration.phase_topology ?? orchestration.phases ?? orchestration.execution ?? model?.execution);
  return phases.filter(isMeaningful);
}

function phaseState(phase, active, index, all) {
  const id = String(phase?.phase ?? phase?.id ?? phase?.stage ?? phase ?? "");
  const activeIndex = all.findIndex((item) => String(item?.phase ?? item?.id ?? item?.stage ?? item) === active);
  if (id === active) return "active";
  if (activeIndex >= 0 && index < activeIndex) return "prior";
  return "upcoming";
}

function proofGroups(model) {
  const evidence = model?.evidence ?? {};
  const required = evidence.required ?? model?.acceptance ?? model?.finish?.acceptance ?? [];
  const obtained = evidence.obtained ?? evidence.available ?? evidence.referenced ?? model?.now?.evidence_refs ?? [];
  const explicitMissing = evidence.missing;
  return {
    required: values(required),
    obtained: values(obtained),
    missing: explicitMissing === undefined ? [] : values(explicitMissing),
    missingKnown: explicitMissing !== undefined,
  };
}

function acceptanceSummary(item) {
  if (!isRecord(item)) return scalarText(item);
  return item.proves ?? item.action ?? summarizeRecord(item);
}

function renderWorktrees(start) {
  const inventory = start?.worktree_inventory ?? {};
  const worktrees = values(start?.worktrees);
  const entries = worktrees.length ? worktrees : (isMeaningful(inventory) ? [inventory] : []);
  if (!entries.length) return `<p class="empty">No worktree inventory recorded.</p>`;
  return `<div class="worktree-list">${entries.map((entry) => {
    const record = isRecord(entry) ? entry : { path: entry };
    const state = record.status ?? inventory.status ?? "unknown";
    return `<section class="worktree"><h3>${escapeHTML(text(record.path, "Repository worktree"))}</h3><p class="line-status"><strong>Status:</strong> ${escapeHTML(statusLabel(state))}</p>${renderDefinitionList(record, ["class", "owner", "touch", "branch", "paths_or_digest", "recovery"])}</section>`;
  }).join("")}</div>`;
}

function renderGates(model) {
  const gates = values(model?.orchestration?.decision_gates ?? model?.orchestration?.gates).filter(isMeaningful);
  if (!gates.length) return `<p class="empty">No decision gates are defined.</p>`;
  return `<ol class="gate-list">${gates.map((gate, index) => {
    const record = isRecord(gate) ? gate : { id: gate };
    const state = inferGateStatus(record, model);
    const gateName = record.id ?? record.name ?? `Gate ${index + 1}`;
    const decision = record.changes_decision ?? record.decision ?? record.before ?? "Review boundary";
    const prerequisite = record.after ? `After ${scalarText(record.after)}` : "Sequence not recorded";
    const obligation = (label, value, fallback = "Not recorded") => `<p><strong class="obligation-label">${label}:</strong> <em>${escapeHTML(text(value, fallback))}</em></p>`;
    return `<li class="gate">
      <article>
        <p class="gate-position">Gate ${index + 1} · ${escapeHTML(prerequisite)}</p>
        <h3>${escapeHTML(humanize(gateName))}</h3>
        <p class="gate-status ${state.tone}"><strong>Status:</strong> ${escapeHTML(state.label)}</p>
        <p class="gate-decision">${escapeHTML(truncate(decision, 220))}</p>
        <div class="gate-obligations">
          ${obligation("Deterministic checks first", record.deterministic_first)}
          ${obligation("Builder must show", record.builder_obligation)}
          ${obligation("Falsifier must challenge", record.falsifier_obligation)}
          ${obligation("Verifier must confirm", record.verifier_obligation)}
          ${obligation("Dissent and blockers", distinctMeaningful([...values(record.dissent), ...values(record.minority_findings), ...values(record.unresolved_blockers), ...values(record.blockers)]))}
          ${obligation("Minority rule", record.minority_rule)}
          ${obligation("Durable receipt", record.durable_evidence_ref)}
        </div>
      </article>
    </li>`;
  }).join("")}</ol>`;
}

function renderPhases(model) {
  const phases = phaseItems(model);
  const active = String(model?.now?.slice ?? "");
  if (!phases.length) return `<p class="empty">No phase topology recorded.</p>`;
  return `<ol class="phase-list">${phases.map((phase, index) => {
    const record = isRecord(phase) ? phase : { phase };
    const id = String(record.phase ?? record.id ?? record.stage ?? phase);
    const state = phaseState(record, active, index, phases);
    const description = record.outcome ?? record.rule ?? record.fan_out ?? record.depends_on;
    return `<li class="phase ${state}"${state === "active" ? ' aria-current="step"' : ""}><div><p class="phase-name">${escapeHTML(humanize(id))}${state === "active" ? ' <strong class="phase-state">Current phase</strong>' : ""}</p>${isMeaningful(description) ? `<p class="phase-description">${escapeHTML(truncate(description, 150))}</p>` : ""}</div></li>`;
  }).join("")}</ol>`;
}

function renderProof(model) {
  const groups = proofGroups(model);
  const required = groups.required.filter(isMeaningful);
  const obtained = groups.obtained.filter(isMeaningful);
  const missing = groups.missing.filter(isMeaningful);
  return `<div class="proof-list">
    <section aria-labelledby="proof-required"><h3 id="proof-required">Required proof <span>${required.length}</span></h3>${renderList(required.map(acceptanceSummary), "plain-list", "No required proof recorded")}</section>
    <section aria-labelledby="proof-obtained"><h3 id="proof-obtained">Obtained references <span>${obtained.length}</span></h3>${renderList(obtained.map(scalarText), "plain-list", "No evidence receipts recorded yet")}</section>
    <section aria-labelledby="proof-missing"><h3 id="proof-missing">Explicitly missing <span>${groups.missingKnown ? missing.length : "Not assessed"}</span></h3>${groups.missingKnown ? renderList(missing.map(scalarText), "plain-list", "Nothing is explicitly marked missing") : '<p class="empty">Required proof is not assumed missing or complete.</p>'}</section>
  </div>`;
}

function sourceMeta(source) {
  if (!isRecord(source)) return "Source provenance unavailable";
  const parts = [source.path, source.digest ? `SHA-256 ${source.digest}` : "", source.generatedAt, source.generatorVersion ? `Generator ${source.generatorVersion}` : ""].filter(isMeaningful);
  return parts.join(" · ") || "Source provenance unavailable";
}
function fileLineDelta(file) {
  if (file?.binary) return `<span class="repo-change-delta"><span class="repo-delta muted">binary</span></span>`;
  if (file?.addedLines === null || file?.deletedLines === null || file?.addedLines === undefined || file?.deletedLines === undefined) {
    return `<span class="repo-change-delta"><span class="repo-delta muted">?</span></span>`;
  }
  const added = Number(file.addedLines);
  const deleted = Number(file.deletedLines);
  if (added === 0 && deleted === 0) {
    return `<span class="repo-change-delta"><span class="repo-delta muted">0</span></span>`;
  }
  return `<span class="repo-change-delta">${added ? `<span class="repo-delta add">+${added}</span>` : ""}${deleted ? `<span class="repo-delta del">−${deleted}</span>` : ""}</span>`;
}


function formatSessionTime(value) {
  if (!isMeaningful(value)) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function renderSessionEvent(event) {
  const record = isRecord(event) ? event : {};
  const detail = isMeaningful(record.detail) ? `<span class="session-detail">${escapeHTML(record.detail)}</span>` : "";
  return `<li><time datetime="${escapeHTML(text(record.at, ""))}">${escapeHTML(formatSessionTime(record.at))}</time><span class="session-summary">${escapeHTML(text(record.summary, "Event"))}</span>${detail}</li>`;
}

function renderSessionLog(session) {
  if (!isRecord(session) || !Array.isArray(session.events) || session.events.length === 0) return "";
  const recent = values(session.recentEvents).filter(isMeaningful);
  const earlier = values(session.earlierEvents).filter(isMeaningful);
  const dirtyFiles = values(session.dirtyFiles).filter((file) => isRecord(file) && isMeaningful(file.path));
  const recentMarkup = recent.length
    ? `<ol class="session-recent">${recent.map(renderSessionEvent).join("")}</ol>`
    : "";
  const earlierMarkup = earlier.length
    ? `<ol class="session-earlier">${earlier.map(renderSessionEvent).join("")}</ol>`
    : "";
  const dirtyMarkup = dirtyFiles.length
    ? `<ul class="session-files">${dirtyFiles.map((file) => {
        const first = formatSessionTime(file.firstSeenAt);
        const last = formatSessionTime(file.lastModifiedAt);
        return `<li><code>${escapeHTML(file.path)}</code><span class="session-file-state">${escapeHTML(humanize(file.state ?? "changed"))}</span><span class="session-file-times">seen ${escapeHTML(first)} · mtime ${escapeHTML(last)}</span></li>`;
      }).join("")}</ul>`
    : `<p class="session-empty">No dirty files in this session.</p>`;
  const moreLabel = earlier.length
    ? `${earlier.length} earlier events · ${dirtyFiles.length} dirty-file timestamps`
    : `${dirtyFiles.length} dirty-file timestamps`;
  return `<section class="session-log" aria-label="Dashboard session log">
    <p class="session-kicker"><strong>Session only.</strong> Ephemeral process log — not mission authority. Started ${escapeHTML(formatSessionTime(session.startedAt))}.</p>
    ${recentMarkup}
    <details class="session-more">
      <summary>${escapeHTML(moreLabel)}</summary>
      ${earlierMarkup}
      <h3 class="session-files-title">Dirty files</h3>
      ${dirtyMarkup}
    </details>
  </section>`;
}

function renderRepositoryMetadata(repository) {
  if (!isRecord(repository) || repository.available !== true) {
    return `<section class="repo-status unavailable" aria-label="Repository status"><p><strong>Git status unavailable</strong><span>${escapeHTML(text(repository?.reason, "Repository metadata was not collected."))}</span></p></section>`;
  }
  const identity = repository.branch
    ? `Branch ${repository.branch}`
    : repository.detached
      ? "Detached HEAD"
      : "Branch unavailable";
  const head = repository.head ? `HEAD ${repository.head}` : "HEAD unavailable";
  const worktree = `${humanize(repository.worktreeKind || "unknown")} worktree · ${text(repository.worktreePath, "Path unavailable")}`;
  const upstream = repository.upstream
    ? `${repository.upstream}${repository.upstreamHead ? ` @ ${repository.upstreamHead}` : ""} · ${repository.ahead ?? "?"} ahead · ${repository.behind ?? "?"} behind`
    : "No configured upstream";
  const totals = repository.addedLines === null || repository.deletedLines === null
    ? `LOC unavailable · ${repository.unreadableFiles ?? "?"} unreadable`
    : `+${repository.addedLines ?? 0} −${repository.deletedLines ?? 0}`;
  const binary = repository.binaryFiles ? ` · ${repository.binaryFiles} binary` : "";
  const changedFiles = values(repository.changedFiles).filter((file) => isRecord(file) && isMeaningful(file.path));
  const fileList = changedFiles.length
    ? `<ul class="repo-change-list">${changedFiles.map((file) => `<li><code>${escapeHTML(file.path)}</code><span class="repo-change-state">${escapeHTML(humanize(file.state ?? "changed"))}</span>${fileLineDelta(file)}</li>`).join("")}</ul>`
    : Number(repository.dirtyFiles ?? 0) > 0
      ? `<p class="repo-change-empty muted">File inventory unavailable.</p>`
      : `<p class="repo-change-empty">Working tree clean.</p>`;
  return `<section class="repo-status" aria-label="Repository status">
    <div class="repo-meta">
      <p><strong>${escapeHTML(identity)}</strong><span>${escapeHTML(head)}</span></p>
      <p><strong>${escapeHTML(worktree)}</strong><span>${escapeHTML(upstream)}</span></p>
    </div>
    <details class="repo-changes" open>
      <summary><span class="repo-summary-line"><strong>${escapeHTML(`${repository.dirtyFiles ?? "?"} uncommitted files`)}</strong><span>${escapeHTML(`${totals}${binary}`)}</span></span></summary>
      ${fileList}
    </details>
  </section>`;
}


export function renderDashboard(model) {
  const safeModel = isRecord(model) ? model : {};
  const finish = safeModel.finish ?? {};
  const now = safeModel.now ?? {};
  const start = safeModel.start ?? {};
  const candidate = now.candidate ?? safeModel.orchestration?.candidate ?? {};
  const decision = now.decision ?? {};
  const successor = safeModel.successor ?? {};
  const activePhase = now.slice ?? "No active phase recorded";
  const currentStatus = now.status ?? "unknown";
  const blocker = now.blocker_or_risk;
  const question = now.question;
  const visibleWeakMeasures = weakMeasures(safeModel, now);
  const visibleDissent = dissentItems(safeModel, now);
  const evidenceReferences = now.evidence_refs ?? safeModel.evidence?.obtained;
  const title = safeModel.title ?? finish.deliver ?? "Definition mission";
  const subtitle = safeModel.subtitle ?? finish.artifact ?? "Owner mission control";
  const eventScript = "const stream=new EventSource('/events');const reload=()=>location.reload();stream.addEventListener('unavailable',reload);stream.addEventListener('reload',reload);";

  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>${escapeHTML(text(title))} — Mission control</title>
  <style>
    :root {
      --paper: #f4f0e8;
      --ink: #25231f;
      --muted: #6e675d;
      --line: #d6cec0;
      --line-strong: #aaa092;
      --accent: #155f59;
      --attention: #8b5a16;
      --critical: #963f35;
      --positive: #356847;
      --space-1: 0.25rem;
      --space-2: 0.5rem;
      --space-3: 0.75rem;
      --space-4: 1rem;
      --space-5: 1.5rem;
      --space-6: 2rem;
      --space-7: 3rem;
      --space-8: 4rem;
      --text-sm: 0.8125rem;
      --text-base: 1rem;
      --text-lead: 1.15rem;
      --text-h3: 1.25rem;
      --text-h2: clamp(1.5rem, 2vw, 1.95rem);
      --text-h1: clamp(1.9rem, 3.5vw, 2.8rem);
      --content: 80rem;
      --rule-emphasis: 0.1875rem;
      --font-body: Charter, "Bitstream Charter", "Sitka Text", Georgia, serif;
      --font-label: Avenir, "Avenir Next", "Trebuchet MS", sans-serif;
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body {
      margin: 0;
      background: var(--paper);
      color: var(--ink);
      font-family: var(--font-body);
      font-size: var(--text-base);
      line-height: 1.55;
    }
    p, h1, h2, h3 { margin-top: 0; overflow-wrap: anywhere; }
    h1, h2, h3 { line-height: 1.15; }
    a { color: var(--accent); }
    .shell { width: min(calc(100% - var(--space-6)), var(--content)); margin-inline: auto; }
    .masthead { padding-block: var(--space-5); border-bottom: 1px solid var(--line-strong); }
    .masthead-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.15fr) minmax(18rem, 0.85fr);
      gap: var(--space-7);
      align-items: end;
    }
    .kicker, .eyebrow {
      margin: 0 0 var(--space-2);
      color: var(--accent);
      font-family: var(--font-label);
      font-size: var(--text-sm);
      font-weight: 700;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }
    h1 {
      max-width: 28ch;
      margin-bottom: var(--space-2);
      font-size: var(--text-h1);
      font-weight: 600;
      letter-spacing: -0.025em;
      text-wrap: balance;
    }
    .subtitle { margin: 0; color: var(--muted); font-size: var(--text-lead); }
    .provenance { margin-bottom: var(--space-3); color: var(--muted); font-family: var(--font-label); font-size: var(--text-sm); overflow-wrap: anywhere; }
    .authority-note { margin: 0; padding-left: var(--space-4); border-left: var(--rule-emphasis) solid var(--accent); color: var(--muted); }
    .authority-note strong { color: var(--ink); }
    .repo-status {
      display: flex;
      flex-direction: column;
      gap: var(--space-3);
      margin-top: var(--space-4);
      padding-top: var(--space-3);
      border-top: 1px solid var(--line);
      font-family: var(--font-label);
      font-size: var(--text-sm);
    }
    .repo-meta {
      display: grid;
      grid-template-columns: minmax(12rem, 0.85fr) minmax(0, 1.15fr);
      gap: var(--space-4) var(--space-6);
    }
    .repo-status p { min-width: 0; margin: 0; }
    .repo-status .repo-meta strong, .repo-status .repo-meta span { display: block; overflow-wrap: anywhere; }
    .repo-status .repo-meta strong { color: var(--ink); }
    .repo-status .repo-meta span { margin-top: var(--space-1); color: var(--muted); }
    .repo-status.unavailable { display: block; }
    .repo-changes { min-width: 0; }
    .repo-changes > summary {
      cursor: pointer;
      color: var(--ink);
    }
    .repo-summary-line {
      display: inline-flex;
      align-items: baseline;
      flex-wrap: wrap;
      gap: 0.15rem 0.75rem;
    }
    .repo-summary-line strong { font-weight: 700; }
    .repo-summary-line span { color: var(--muted); font-variant-numeric: tabular-nums; }
    .repo-change-list {
      display: grid;
      gap: 0.15rem;
      margin: 0.4rem 0 0 1.15rem;
      padding: 0;
      list-style: none;
    }
    .repo-change-list li {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto auto;
      column-gap: 0.9rem;
      align-items: baseline;
      line-height: 1.35;
    }
    .repo-change-list code {
      min-width: 0;
      overflow-wrap: anywhere;
      color: var(--ink);
      font-family: ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace;
      font-size: 0.92em;
    }
    .repo-change-state {
      color: var(--muted);
      white-space: nowrap;
    }
    .repo-change-delta {
      display: inline-flex;
      gap: 0.35rem;
      justify-content: end;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
    }
    .repo-delta.add { color: var(--positive); }
    .repo-delta.del { color: var(--critical); }
    .repo-delta.muted { color: var(--muted); }
    .repo-change-empty { margin: 0.4rem 0 0 1.15rem; color: var(--positive); }
    .repo-change-empty.muted { color: var(--muted); }
    main { padding-block: var(--space-6) var(--space-8); }
    .briefing { padding-bottom: var(--space-6); border-bottom: 1px solid var(--line-strong); }
    .briefing-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.1fr) minmax(20rem, 0.9fr);
      gap: var(--space-7);
      align-items: start;
    }
    .current-line { display: flex; flex-wrap: wrap; gap: var(--space-3) var(--space-5); align-items: baseline; margin-bottom: var(--space-5); }
    .current-line h2 { margin: 0; font-size: var(--text-h2); font-weight: 600; }
    .line-status, .gate-status { margin: 0; color: var(--muted); font-family: var(--font-label); font-size: var(--text-sm); }
    .line-status strong, .gate-status strong { color: var(--ink); }
    .next-action { max-width: 48ch; margin: 0; font-size: var(--text-lead); }
    .next-action strong, .priority-notes strong, .field-label, .obligation-label {
      color: var(--accent);
      font-family: var(--font-label);
      font-size: var(--text-sm);
      font-weight: 700;
      letter-spacing: 0.035em;
      text-transform: uppercase;
    }
    .next-action strong { display: block; margin-bottom: var(--space-2); }
    .finish-brief h2 { margin-bottom: var(--space-3); font-size: var(--text-h2); font-weight: 600; }
    .finish-outcome h3 { margin-bottom: var(--space-2); font-size: var(--text-h3); font-weight: 600; }
    .artifact { margin-bottom: var(--space-4); color: var(--muted); font-size: var(--text-lead); font-style: italic; }
    .priority-notes { display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4); margin-top: var(--space-5); }
    .priority-notes p { margin: 0; padding-top: var(--space-3); border-top: 1px solid var(--line); }
    .priority-notes strong { display: block; margin-bottom: var(--space-1); }
    .finish-details {
      display: grid;
      grid-template-columns: 1.2fr 0.8fr 1fr;
      gap: var(--space-6);
      margin-top: var(--space-6);
      padding-top: var(--space-5);
      border-top: 1px solid var(--line);
    }
    .section { margin-top: var(--space-6); padding-top: var(--space-6); border-top: 1px solid var(--line); }
    .section-heading { margin-bottom: var(--space-5); }
    .section-heading h2 { margin-bottom: var(--space-2); font-size: var(--text-h2); font-weight: 600; letter-spacing: -0.015em; }
    .section-heading p { max-width: 72ch; margin-bottom: 0; color: var(--muted); }
    .subsection { min-width: 0; }
    .subsection h3, .worktree h3, .proof-list h3 { margin-bottom: var(--space-2); font-size: var(--text-h3); font-weight: 600; }
    .plain-list { margin: 0; padding-left: var(--space-5); }
    .plain-list li { overflow-wrap: anywhere; }
    .plain-list li + li { margin-top: var(--space-2); }
    .phase-list {
      display: grid;
      grid-auto-flow: column;
      grid-auto-columns: minmax(0, 1fr);
      margin: 0;
      padding: 0;
      list-style-position: inside;
    }
    .phase { min-width: 0; padding: var(--space-3) var(--space-4); border-left: 1px solid var(--line); }
    .phase:first-child { border-left: 0; }
    .phase::marker { color: var(--muted); font-family: var(--font-label); font-weight: 700; }
    .phase.active { border-top: var(--rule-emphasis) solid var(--accent); }
    .phase-name { margin-bottom: var(--space-1); font-weight: 600; }
    .phase-state { display: block; margin-top: var(--space-1); color: var(--accent); font-family: var(--font-label); font-size: var(--text-sm); font-weight: 700; }
    .phase-description { margin: 0; color: var(--muted); font-size: var(--text-sm); }
    .gate-list {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0 var(--space-7);
      margin: 0;
      padding: 0;
      list-style: none;
    }
    .gate { min-width: 0; padding-block: var(--space-5); border-top: 1px solid var(--line-strong); }
    .gate:nth-child(-n + 2) { padding-top: 0; border-top: 0; }
    .gate h3 { margin-bottom: var(--space-2); font-size: var(--text-h3); font-weight: 600; }
    .gate-position { margin-bottom: var(--space-2); color: var(--muted); font-family: var(--font-label); font-size: var(--text-sm); }
    .gate-status { margin-bottom: var(--space-3); }
    .gate-status.positive strong { color: var(--positive); }
    .gate-status.attention strong { color: var(--attention); }
    .gate-status.critical strong { color: var(--critical); }
    .gate-decision { margin-bottom: var(--space-4); }
    .gate-obligations { margin-top: var(--space-4); }
    .gate-obligations p { margin: 0; padding-block: var(--space-2); border-top: 1px solid var(--line); }
    .gate-obligations em { color: var(--muted); }
    .proof-list {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: var(--space-6);
    }
    .proof-list section { min-width: 0; padding-top: var(--space-3); border-top: var(--rule-emphasis) solid var(--line-strong); }
    .proof-list h3 span { display: block; margin-top: var(--space-1); color: var(--accent); font-family: var(--font-label); font-size: var(--text-sm); }
    .start-grid, .start-support-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--space-7);
    }
    .start-support-grid { margin-top: var(--space-6); }
    .prose-fields { margin-top: var(--space-3); }
    .prose-fields p { margin: 0; padding-block: var(--space-2); border-top: 1px solid var(--line); }
    .prose-fields span { color: var(--muted); }
    .worktree-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-6); margin-top: var(--space-6); }
    .worktree { min-width: 0; padding-top: var(--space-4); border-top: 1px solid var(--line-strong); }
    .detail-record + .detail-record { margin-top: var(--space-5); }
    .secondary-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 var(--space-7); }
    .secondary-grid .section { min-width: 0; }
    .steering-heading { color: var(--attention); }
    .empty { color: var(--muted); font-style: italic; }
    .body-copy { overflow-wrap: anywhere; }
    footer { padding-block: var(--space-5); border-top: 1px solid var(--line); color: var(--muted); font-size: var(--text-sm); }
    footer strong { color: var(--ink); }
    .session-log { margin-top: var(--space-4); padding-top: var(--space-3); border-top: 1px solid var(--line); font-family: var(--font-label); }
    .session-kicker { margin: 0 0 var(--space-3); }
    .session-recent, .session-earlier, .session-files { margin: 0; padding: 0; list-style: none; }
    .session-recent { display: grid; gap: 0.2rem; }
    .session-recent li, .session-earlier li, .session-files li {
      display: grid;
      grid-template-columns: 5.5rem minmax(0, 1fr) auto;
      gap: 0.55rem 0.85rem;
      align-items: baseline;
      line-height: 1.35;
    }
    .session-recent time, .session-earlier time { color: var(--muted); font-variant-numeric: tabular-nums; }
    .session-summary { color: var(--ink); }
    .session-detail { color: var(--muted); white-space: nowrap; }
    .session-more { margin-top: var(--space-3); }
    .session-more > summary { cursor: pointer; color: var(--ink); }
    .session-earlier { margin-top: var(--space-3); display: grid; gap: 0.2rem; }
    .session-files-title { margin: var(--space-4) 0 var(--space-2); font-size: var(--text-sm); font-weight: 700; letter-spacing: 0.04em; text-transform: uppercase; color: var(--accent); }
    .session-files { display: grid; gap: 0.2rem; }
    .session-files code { min-width: 0; overflow-wrap: anywhere; color: var(--ink); font-family: ui-monospace, "SFMono-Regular", Menlo, Consolas, monospace; font-size: 0.92em; }
    .session-file-state { color: var(--muted); white-space: nowrap; }
    .session-file-times { color: var(--muted); font-variant-numeric: tabular-nums; white-space: nowrap; }
    .session-empty { margin: var(--space-2) 0 0; color: var(--muted); font-style: italic; }
    @media (max-width: 64rem) {
      .masthead-grid, .briefing-grid { grid-template-columns: 1fr; gap: var(--space-5); }
      .repo-meta { grid-template-columns: 1fr; }
      .finish-details { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .phase-list { grid-auto-flow: row; grid-template-columns: repeat(3, minmax(0, 1fr)); }
      .phase { border-top: 1px solid var(--line); }
    }
    @media (max-width: 48rem) {
      .shell { width: min(calc(100% - var(--space-5)), var(--content)); }
      .masthead, main { padding-top: var(--space-5); }
      .section { margin-top: var(--space-5); padding-top: var(--space-5); }
      .finish-details, .gate-list, .proof-list, .start-grid, .start-support-grid, .worktree-list, .secondary-grid { grid-template-columns: 1fr; }
      .repo-meta { grid-template-columns: 1fr; }
      .session-recent li, .session-earlier li, .session-files li { grid-template-columns: 4.75rem minmax(0, 1fr); }
      .session-detail, .session-file-times { grid-column: 2; }
      .gate:nth-child(2) { padding-top: var(--space-5); border-top: 1px solid var(--line-strong); }
      .phase-list { grid-template-columns: 1fr; }
      .phase { padding: var(--space-3) 0; border-left: 0; }
      .phase.active { padding-left: var(--space-3); border-left: var(--rule-emphasis) solid var(--accent); border-top: 1px solid var(--line); }
    }
    @media (max-width: 30rem) {
      .shell { width: calc(100% - var(--space-4)); }
      .priority-notes { grid-template-columns: 1fr; }
      h1 { font-size: var(--text-h1); }
    }
    @media (prefers-reduced-motion: reduce) {
      html { scroll-behavior: auto; }
    }
    @media print {
      :root { --paper: #faf9f6; --ink: #171614; }
      body { font-size: 9pt; }
      .shell { width: 100%; }
      .masthead, main, footer { padding-block: var(--space-4); }
      .section { margin-top: var(--space-5); padding-top: var(--space-5); }
      .gate, .worktree, .proof-list section, .subsection { break-inside: avoid; }
      a { color: inherit; text-decoration: none; }
    }
  </style>
</head>
<body>
  <header class="masthead">
    <div class="shell">
      <div class="masthead-grid">
      <div>
        <p class="kicker">Live Definition projection</p>
        <h1>${escapeHTML(text(title))}</h1>
        <p class="subtitle">${escapeHTML(text(subtitle))}</p>
      </div>
      <div>
        <p class="provenance">${escapeHTML(sourceMeta(safeModel.source))}</p>
        <p class="authority-note"><strong>Read-only owner view.</strong> This current projection is non-authoritative. It does not approve a gate, authorize work, or prove completion; the source Definition remains the sole authority.</p>
      </div>
      </div>
      ${renderRepositoryMetadata(safeModel.repository)}
    </div>
  </header>

  <main class="shell">
    <section class="briefing" aria-labelledby="finish-title">
      <div class="briefing-grid">
        <div class="finish-block">
          <section class="finish-brief" aria-labelledby="finish-title">
            <p class="eyebrow">Intended finish</p>
            <h2 id="finish-title">Outcome and immediate constraints</h2>
            <div class="finish-outcome">
              <h3>${escapeHTML(text(finish.deliver, title))}</h3>
              <p class="artifact">${escapeHTML(text(finish.artifact, "No finish artifact recorded."))}</p>
            </div>
          </section>
          <div class="finish-details">
            <section class="subsection" aria-labelledby="acceptance-title">
              <h3 id="acceptance-title">Acceptance</h3>
              ${renderList(values(finish.acceptance ?? safeModel.acceptance).map(acceptanceSummary), "plain-list", "No acceptance actions recorded")}
            </section>
            <section class="subsection" aria-labelledby="rollback-title">
              <h3 id="rollback-title">Rollback</h3>
              <p>${escapeHTML(text(finish.rollback, "No rollback recorded."))}</p>
            </section>
            <section class="subsection" aria-labelledby="not-done-title">
              <h3 id="not-done-title">Not done when</h3>
              ${renderList(finish.not_done_when, "plain-list", "No non-completion conditions recorded")}
            </section>
          </div>
        </div>
        <div class="current">
          <p class="eyebrow">Current phase</p>
          <div class="current-line">
            <h2 id="current-title">${escapeHTML(humanize(activePhase))}</h2>
            <p class="line-status"><strong>Status:</strong> ${escapeHTML(statusLabel(currentStatus))}</p>
          </div>
          <p class="next-action"><strong>Next action</strong>${escapeHTML(text(now.next_action, "No next action is recorded. Do not infer one from this view."))}</p>
        </div>
      </div>
      <div class="priority-notes">
        <p><strong>Blocker or risk</strong>${escapeHTML(text(blocker, "No blocker or risk is explicitly recorded."))}</p>
        <p><strong>Open question</strong>${escapeHTML(text(question, "No execution-changing question is recorded."))}</p>
      </div>
    </section>

    <section class="section" aria-labelledby="phases-title">
      <div class="section-heading">
        <h2 id="phases-title">Mission phase path</h2>
        <p>The ordered path provides orientation only. A phase position is not evidence that earlier work passed review.</p>
      </div>
      ${renderPhases(safeModel)}
    </section>

    <section class="section" aria-labelledby="gates-title">
      <div class="section-heading">
        <h2 id="gates-title">Decision gates</h2>
        <p>Every gate begins as <strong>Not reviewed</strong>. Only a receipt with the exact gate ID and an allowed adjudication can change its state.</p>
      </div>
      ${renderGates(safeModel)}
    </section>

    <section class="section" aria-labelledby="proof-title">
      <div class="section-heading">
        <h2 id="proof-title">Proof readiness</h2>
        <p>References show what has been collected; they do not, by themselves, prove acceptance or completion.</p>
      </div>
      ${renderProof(safeModel)}
    </section>

    <section class="section" aria-labelledby="start-title">
      <div class="section-heading">
        <h2 id="start-title">Protected starting state</h2>
        <p>This is the observed baseline. Dirty work and recovery surfaces remain protected unless the Definition explicitly authorizes otherwise.</p>
      </div>
      <div class="start-grid">
        <section class="subsection" aria-labelledby="source-title">
          <h3 id="source-title">Source at capture</h3>
          ${renderDefinitionList(start.source, ["canonical_ref", "origin_ref", "relation", "deploy_identity"])}
        </section>
        <section class="subsection" aria-labelledby="preservation-title">
          <h3 id="preservation-title">Preservation rule</h3>
          <p>${escapeHTML(text(start?.worktree_inventory?.preservation_rule, "Preserve unclassified work; do not infer mutation authority from this view."))}</p>
        </section>
      </div>
      ${renderWorktrees(start)}
      <div class="start-support-grid">
        ${isMeaningful(start.observed_artifact) ? `<section class="subsection" aria-labelledby="baseline-title"><h3 id="baseline-title">Observed baseline</h3>${renderList(values(start.observed_artifact).map((item) => isRecord(item) ? `${text(item.claim)} — ${text(item.evidence_ref, "Evidence not recorded")}` : scalarText(item)), "plain-list")}</section>` : ""}
        ${isMeaningful(start.unknowns) ? `<section class="subsection" aria-labelledby="unknowns-title"><h3 id="unknowns-title">Starting unknowns</h3>${renderList(start.unknowns, "plain-list")}</section>` : ""}
      </div>
    </section>

    <div class="secondary-grid">
      ${isMeaningful(now.reconciliation) ? `<section class="section" aria-labelledby="reconciliation-title"><div class="section-heading"><h2 id="reconciliation-title">Reconciliation</h2></div>${renderDefinitionList(now.reconciliation, ["observed_at", "source_ref", "deploy_identity", "authority_identities", "policy_resolution_ref", "worktree_inventory_ref", "status"])}</section>` : ""}
      ${isMeaningful(decision) ? `<section class="section" aria-labelledby="decision-title"><div class="section-heading"><h2 id="decision-title">Accepted decision</h2></div>${renderDefinitionList(decision, ["selected", "kind", "status", "source", "evidence_ref", "owner_ratification_ref", "recorded_at", "consequence"])}</section>` : ""}
      ${isMeaningful(candidate) ? `<section class="section" aria-labelledby="candidate-title"><div class="section-heading"><h2 id="candidate-title">Candidate</h2></div>${renderDefinitionList(candidate, ["id", "state", "ref", "owner", "base", "digest", "scope"])}</section>` : ""}
      ${isMeaningful(evidenceReferences) ? `<section class="section" aria-labelledby="evidence-title"><div class="section-heading"><h2 id="evidence-title">Evidence references</h2></div>${renderList(evidenceReferences, "plain-list")}</section>` : ""}
      ${visibleDissent.length ? `<section class="section" aria-labelledby="dissent-title"><div class="section-heading"><h2 id="dissent-title">Dissent and blockers</h2></div>${renderRecordCollection(visibleDissent, ["summary", "status", "source", "finding", "findings", "minority_findings", "unresolved_blockers", "blockers", "evidence_refs", "consequence"])}</section>` : ""}
      ${visibleWeakMeasures.length ? `<section class="section" aria-labelledby="steering-title"><div class="section-heading"><h2 id="steering-title" class="steering-heading">Steering only — not proof</h2><p>These measures may guide the next inspection. They cannot establish acceptance or completion.</p></div>${renderRecordCollection(visibleWeakMeasures, ["name", "kind", "baseline", "desired", "decision_use", "cannot_prove"])}</section>` : ""}
      ${isMeaningful(successor) ? `<section class="section" aria-labelledby="successor-title"><div class="section-heading"><h2 id="successor-title">Successor boundary</h2></div>${renderDefinitionList(successor, ["status", "candidate_goal", "prerequisite_receipts", "note"])}</section>` : ""}
    </div>
  </main>

  <footer><div class="shell"><p><strong>Projection only.</strong> Dashboard health means the renderer is current; it is not an acceptance receipt, gate decision, mutation authority, or completion signal.</p>${renderSessionLog(safeModel.session)}</div></footer>
  <script>${eventScript}</script>
</body>
</html>`;
}
