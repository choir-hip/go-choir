import { fetchWithRenewal } from './auth.js';

const DURABLE_WORK_SCHEMA = 'choir.durable_work.v1';
const SUPERVISION_SCHEMA = 'choir.supervision_transaction.v1';

function requireLifecycleSchema(value, label) {
  if (value?.schema !== DURABLE_WORK_SCHEMA && value?.schema !== SUPERVISION_SCHEMA) {
    throw new Error(`${label} returned unsupported schema`);
  }
  return value;
}

const SUPERVISION_ENTRY_PREVIEW_LIMIT = 2;
const SUPERVISION_EVENT_LIMIT = 12;
const SUPERVISION_PROVENANCE_LIMIT = 16;

function asObject(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return value;
}

function parseEntryBody(value) {
  if (typeof value === 'string') {
    try {
      return asObject(JSON.parse(value));
    } catch (_error) {
      return {};
    }
  }
  return asObject(value);
}

function textValue(value) {
  return typeof value === 'string' ? value.trim() : '';
}

function numericValue(value) {
  return Number.isSafeInteger(value) && value >= 0 ? value : 0;
}

function optionalNumericValue(value) {
  return Number.isSafeInteger(value) && value >= 0 ? value : null;
}

function humanize(value) {
  const text = textValue(value).replaceAll('_', ' ');
  return text ? `${text.charAt(0).toUpperCase()}${text.slice(1)}` : '';
}

function compareSupervisionEntries(left, right) {
  const leftKey = `${left.kind}\u0000${left.id}`;
  const rightKey = `${right.kind}\u0000${right.id}`;
  if (leftKey === rightKey) return 0;
  return leftKey < rightKey ? -1 : 1;
}

function entryID(entry, body) {
  const direct = textValue(entry?.id);
  if (direct) return direct;
  const idKeys = [
    'assignment_id', 'attempt_id', 'result_id', 'update_id', 'disposition_id',
    'finding_id', 'dissent_id', 'decision_id', 'reconciliation_id',
    'rebase_obligation_id', 'proposal_id', 'belief_id', 'intent_revision_id',
  ];
  for (const key of idKeys) {
    const id = textValue(body[key]);
    if (id) return id;
  }
  return '';
}

function supervisionEntrySummary(kind, body, id) {
  switch (kind) {
    case 'intent_revised':
      return textValue(body.intent) || 'Intent text unavailable';
    case 'assignment_opened': {
      const role = humanize(body.assigned_role) || 'Worker';
      const actor = textValue(body.assigned_actor_id);
      return actor ? `${role} · ${actor}` : role;
    }
    case 'attempt_started': {
      const ordinal = Number.isSafeInteger(body.ordinal) ? `Attempt ${body.ordinal}` : 'Attempt';
      const assignment = textValue(body.assignment_id);
      return assignment ? `${ordinal} · ${assignment}` : ordinal;
    }
    case 'attempt_result': {
      const outcome = humanize(body.outcome) || 'Result returned';
      const attempt = textValue(body.attempt_id);
      const late = body.delivered_after_cancellation === true ? ' · late delivery' : '';
      return `${outcome}${attempt ? ` · ${attempt}` : ''}${late}`;
    }
    case 'update_recorded':
      return textValue(body.summary) || (textValue(body.work_item_id) ? `Update · ${body.work_item_id}` : 'Update recorded');
    case 'super_belief_recorded':
      return `Super belief${textValue(body.supersedes_belief_id) ? ` · supersedes ${body.supersedes_belief_id}` : ''}`;
    case 'super_finding_recorded': {
      const severity = humanize(body.severity);
      const invariant = textValue(body.invariant);
      return [severity, invariant].filter(Boolean).join(' · ') || 'Open finding';
    }
    case 'compensation_obligation':
      return `Compensation required · ${id}`;
    case 'dissent_recorded': {
      const subject = asObject(body.subject);
      return subject.kind && subject.id ? `${humanize(subject.kind)} · ${subject.id}` : 'Dissent retained';
    }
    case 'super_decision_proposed':
      return body.reserved_authority === 'owner'
        ? 'Owner decision required'
        : (textValue(body.selected_option_id) ? `Selected option · ${body.selected_option_id}` : 'Decision proposed');
    case 'owner_decision_recorded':
      return textValue(body.proposal_id) ? `Owner decision · ${body.proposal_id}` : 'Owner decision recorded';
    case 'rebase_obligation': {
      const target = asObject(body.affected_target);
      return target.kind && target.id ? `${humanize(target.kind)} ${target.id} requires rebase` : `Rebase required · ${id}`;
    }
    case 'disposition_recorded': {
      const target = asObject(body.target);
      const targetLabel = target.kind && target.id ? `${humanize(target.kind)} ${target.id}` : 'Target';
      return `${targetLabel} · ${humanize(body.value) || 'disposition recorded'}`;
    }
    case 'super_reconciliation_recorded':
      return 'Super reconciliation recorded';
    case 'settlement_proposed':
      return `Settlement proposed${textValue(body.proposal_id) ? ` · ${body.proposal_id}` : ''}`;
    default:
      return humanize(kind) || id || 'Supervision record';
  }
}

function normalizeSupervisionEntry(value, fallbackKind = '') {
  if (typeof value === 'string') {
    const id = value.trim();
    return id ? { id, kind: fallbackKind, status: '', summary: id, body: {}, refs: [] } : null;
  }
  const entry = asObject(value);
  const body = parseEntryBody(entry.body);
  const kind = textValue(entry.kind) || fallbackKind;
  const id = entryID(entry, body);
  const status = textValue(entry.status) || textValue(body.status) || textValue(body.state) || textValue(body.value);
  return {
    id,
    kind,
    status,
    statusLabel: humanize(status),
    summary: supervisionEntrySummary(kind, body, id),
    body,
    refs: supervisionEntryRefs(entry, body),
  };
}

function normalizeSupervisionEntries(value, fallbackKind = '') {
  if (!Array.isArray(value)) return [];
  return value
    .map((entry) => normalizeSupervisionEntry(entry, fallbackKind))
    .filter(Boolean)
    .sort(compareSupervisionEntries);
}

function supervisionEntryRefs(entry, body) {
  const refs = [];
  const seen = new Set();
  const add = (label, value) => {
    const ref = textValue(value);
    if (!ref || seen.has(ref)) return;
    seen.add(ref);
    refs.push({ label: humanize(label), value: ref });
  };
  for (const ref of Array.isArray(entry.artifact_refs) ? entry.artifact_refs : []) add('artifact ref', ref);
  for (const ref of Array.isArray(entry.evidence_refs) ? entry.evidence_refs : []) add('evidence ref', ref);
  const visit = (value, key = '') => {
    if (Array.isArray(value)) {
      if (/(?:_refs|_digests)$/.test(key)) {
        for (const item of value) add(key, item);
      } else {
        for (const item of value) visit(item, key);
      }
      return;
    }
    if (value && typeof value === 'object') {
      for (const [childKey, childValue] of Object.entries(value)) visit(childValue, childKey);
      return;
    }
    if (/(?:_ref|_digest|canonical_event_head)$/.test(key)) add(key, value);
  };
  visit(body);
  return refs;
}

function normalizedSingleEntry(value, fallbackKind, fallbackID = '') {
  if (!value) return null;
  if (typeof value === 'string' && fallbackKind === 'intent_revised') {
    return normalizeSupervisionEntry({
      id: fallbackID,
      kind: fallbackKind,
      body: { intent_revision_id: fallbackID, intent: value },
    }, fallbackKind);
  }
  return normalizeSupervisionEntry(value, fallbackKind);
}

function previewGroup(key, label, entries, count = entries.length) {
  const resolvedCount = Math.max(numericValue(count), entries.length);
  return {
    key,
    label,
    count: resolvedCount,
    entries,
    preview: entries.slice(0, SUPERVISION_ENTRY_PREVIEW_LIMIT),
    hiddenCount: Math.max(0, entries.length - SUPERVISION_ENTRY_PREVIEW_LIMIT),
  };
}

// projectSupervisionPanel turns the canonical supervision snapshot into a
// bounded, presentation-only read model. It intentionally does not accept the
// durable-work schema as supervision data; legacy durable snapshots continue
// through their existing editor path unchanged.
export function projectSupervisionPanel(snapshot) {
  if (snapshot?.schema !== SUPERVISION_SCHEMA) return null;

  const control = asObject(snapshot.control);
  const intentRevisionID = textValue(snapshot.intent_revision_id || snapshot.current_intent_revision_id);
  const intent = normalizedSingleEntry(
    control.intent || snapshot.intent || snapshot.current_intent,
    'intent_revised',
    intentRevisionID,
  );
  const latestDelta = normalizedSingleEntry(control.latest_delta || snapshot.latest_delta, 'intent_revised');
  const belief = normalizedSingleEntry(control.belief || snapshot.belief, 'super_belief_recorded');
  const assignments = normalizeSupervisionEntries(snapshot.assignments || control.obligations, 'assignment_opened');
  const attempts = normalizeSupervisionEntries(snapshot.attempts, 'attempt_started');
  const results = normalizeSupervisionEntries(snapshot.results, 'attempt_result');
  const updates = normalizeSupervisionEntries(snapshot.updates, 'update_recorded');
  const dispositions = normalizeSupervisionEntries(snapshot.dispositions, 'disposition_recorded');
  const findings = normalizeSupervisionEntries(snapshot.findings, 'super_finding_recorded');
  const reconciliations = normalizeSupervisionEntries(snapshot.reconciliations, 'super_reconciliation_recorded');
  const blockers = normalizeSupervisionEntries(control.blockers, 'super_finding_recorded');
  const dissent = normalizeSupervisionEntries(control.dissent, 'dissent_recorded');
  const decisions = normalizeSupervisionEntries(control.decisions, 'super_decision_proposed');
  const messages = normalizeSupervisionEntries(control.messages, 'actor_message_recorded');
  const rebase = normalizeSupervisionEntries(control.rebase, 'rebase_obligation');
  const settlement = normalizedSingleEntry(control.settlement || snapshot.settlement, 'settlement_proposed');
  const explicitAttention = normalizeSupervisionEntries(control.owner_attention || snapshot.owner_attention, 'owner_attention');
  const ownerDecisions = decisions.filter((entry) => entry.body.reserved_authority === 'owner');
  const ownerMessages = messages.filter((entry) => entry.body.to_role === 'owner');
  const ownerAttention = [...new Map(
    [...explicitAttention, ...ownerDecisions, ...ownerMessages].map((entry) => [`${entry.kind}\u0000${entry.id}`, entry]),
  ).values()].sort(compareSupervisionEntries);
  const attentionCount = Math.max(numericValue(control.attention_count), ownerAttention.length);
  const overflowCount = numericValue(control.overflow_count);

  const allActivity = [
    ...assignments, ...attempts, ...results, ...updates, ...dispositions,
    ...findings, ...reconciliations, ...messages, ...dissent, ...decisions, ...rebase,
    ...(settlement ? [settlement] : []),
  ];
  const activity = allActivity.slice(0, SUPERVISION_EVENT_LIMIT);
  const allProvenance = [];
  const seenProvenance = new Set();
  for (const entry of [intent, latestDelta, belief, ...allActivity].filter(Boolean)) {
    for (const ref of entry.refs) {
      if (seenProvenance.has(ref.value)) continue;
      seenProvenance.add(ref.value);
      allProvenance.push({ ...ref, entryID: entry.id, entryKind: entry.kind });
    }
  }

  const settled = snapshot.settled === true || snapshot.settlement_state === 'settled';
  const archived = snapshot.archived === true || snapshot.archive_state === 'archived';
  const state = archived ? 'archived' : settled ? 'settled' : settlement ? 'settlement proposed' : 'active';
  return {
    trajectoryID: textValue(snapshot.trajectory_id || snapshot.trajectory?.trajectory_id),
    intentRevisionID: intentRevisionID || intent?.id || '',
    artifactHeadRevisionID: textValue(
      snapshot.artifact_head_revision_id ||
      snapshot.artifact_head?.revision_id ||
      snapshot.head_revision?.revision_id,
    ),
    canonicalEventHead: textValue(snapshot.canonical_event_head || snapshot.canonical_head || snapshot.event_head),
    lifecycleVersion: optionalNumericValue(snapshot.lifecycle_version ?? snapshot.version),
    snapshotCursor: optionalNumericValue(snapshot.snapshot_cursor),
    state,
    stateLabel: humanize(state),
    settled,
    archived,
    settlementProposalID: textValue(snapshot.settlement_proposal_id || settlement?.id),
    intent,
    latestDelta,
    belief,
    overflowCount,
    attentionCount,
    controlGroups: [
      previewGroup('obligations', 'Obligations', assignments),
      previewGroup('blockers', 'Blockers', blockers),
      previewGroup('dissent', 'Dissent', dissent),
      previewGroup('owner-attention', 'Owner attention', ownerAttention, attentionCount),
    ],
    workGroups: [
      previewGroup('assignments', 'Assignments', assignments),
      previewGroup('attempts', 'Attempts', attempts),
      previewGroup('results', 'Results', results),
      previewGroup('updates', 'Updates', updates),
    ],
    rebase,
    settlement,
    activity,
    hiddenActivityCount: Math.max(0, allActivity.length - activity.length),
    provenance: allProvenance.slice(0, SUPERVISION_PROVENANCE_LIMIT),
    hiddenProvenanceCount: Math.max(0, allProvenance.length - SUPERVISION_PROVENANCE_LIMIT),
  };
}



export async function getLifecycleEvents(trajectoryId, options = {}) {
  if (!trajectoryId) throw new Error('Lifecycle trajectory ID is required')
  const after = Number.isSafeInteger(options.after) && options.after >= 0 ? options.after : 0
  const limit = Number.isSafeInteger(options.limit) && options.limit > 0 ? options.limit : 100
  const response = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}/events?after=${after}&limit=${limit}`)
  if (!response.ok) {
    const error = await response.json().catch(() => ({}))
    throw new Error(error.reason || error.error || `Lifecycle events failed (${response.status})`)
  }
  return requireLifecycleSchema(await response.json(), 'Lifecycle events');
}





export async function getLifecycleSnapshot(trajectoryId) {
  if (!trajectoryId) {
    throw new Error('Trajectory ID is required');
  }
  const response = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}`, { method: 'GET' });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.reason || error.error || `Lifecycle snapshot failed (${response.status})`);
  }
  return requireLifecycleSchema(await response.json(), 'Lifecycle snapshot');
}

// observeLifecycle opens the event stream before fetching the snapshot, then
// discards buffered events covered by the snapshot cursor and delivers the
// remainder in reducer order. Overflow and expired cursors force replay.
export async function observeLifecycle(trajectoryId, handlers = {}) {
  if (!trajectoryId) throw new Error('Trajectory ID is required');
  const buffer = [];
  let snapshotReady = false;
  let cursor = 0;
  let closed = false;
  const stream = new EventSource(`/api/trajectories/${encodeURIComponent(trajectoryId)}/stream?after=0`, { withCredentials: true });
  const opened = new Promise((resolve, reject) => {
    stream.onopen = resolve;
    stream.onerror = () => {
      if (!snapshotReady) reject(new Error('Lifecycle stream failed before snapshot'));
      else handlers.onError?.(new Error('Lifecycle stream disconnected'));
    };
  });
  stream.addEventListener('lifecycle', (message) => {
    try {
      const event = JSON.parse(message.data);
      requireLifecycleSchema(event, 'Lifecycle stream event');
      if (!snapshotReady) {
        buffer.push(event);
        if (buffer.length > 1000) {
          stream.close();
          closed = true;
          handlers.onReplayRequired?.({ reason: 'buffer_overflow' });
        }
        return;
      }
      if (event.reducer_seq > cursor) {
        cursor = event.reducer_seq;
        handlers.onEvent?.(event);
      }
    } catch (error) {
      handlers.onError?.(error);
    }
  });
  stream.addEventListener('replay_required', (message) => {
    stream.close();
    closed = true;
    handlers.onReplayRequired?.(JSON.parse(message.data));
  });
  try {
    await opened;
    const snapshot = await getLifecycleSnapshot(trajectoryId);
    cursor = snapshot.snapshot_cursor || 0;
    snapshotReady = true;
    handlers.onSnapshot?.(snapshot);
    buffer.sort((left, right) => left.reducer_seq - right.reducer_seq);
    for (const event of buffer) {
      if (event.reducer_seq > cursor) {
        cursor = event.reducer_seq;
        handlers.onEvent?.(event);
      }
    }
  } catch (error) {
    stream.close();
    closed = true;
    throw error;
  }
  return () => {
    if (!closed) stream.close();
  };
}
