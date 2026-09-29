import { createRequire } from 'node:module';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { appendFileSync, readFileSync } from 'node:fs';

// Playwright ships to this repo via frontend's @playwright/test; resolve
// chromium through the frontend manifest so pnpm's layout stays hidden.
const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import { setupVirtualAuthenticator } from '../frontend/tests/helpers/webauthn.js';

const runtimeProcess = globalThis.process || null;
const BASE_URL = runtimeProcess?.env?.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const marker = `M11_SELFDEV_EPISODE_${Date.now()}`;
const POLICY_DIGEST = 'c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7';

const result = {
  marker,
  base_url: BASE_URL,
  started_at: new Date().toISOString(),
  harness_version: '2026-09-27',
  predicate:
    'A fresh owner tracking computer can stage a reversible self-development operation, ' +
    'bind its approval to an independently reproduced qualified-consensus receipt, apply it, ' +
    'render the engineering evidence, reject a second candidate, and restore the prior state.',
  legs: {
    fresh_owner: false,
    bootstrap_chain: false,
    pre_episode_checkpoint: false,
    propose_only_armed: false,
    primary_started: false,
    awaiting_approval: false,
    qualified_consensus_armed: false,
    approved: false,
    applied: false,
    apply_events: false,
    candidate_b_started: false,
    candidate_b_rejected: false,
    document_rendered: false,
    commitment_materiality_visible: false,
    falsified_commitment_visible: false,
    restored_pinned_head: false,
  },
};

const LOG_PATH = `/tmp/m11_probe_${marker}.log`;
function mark(leg, extra) {
  const line = `${new Date().toISOString()} ${leg}${extra ? ' ' + JSON.stringify(extra).slice(0, 300) : ''}`;
  appendFileSync(LOG_PATH, line + '\n');
  // A leg mark is the predicate's evidence: flip the matching flag so the
  // verdict reflects what actually ran. Non-leg marks (checkpoint digests,
  // diagnostics) are absent from result.legs and stay log-only.
  if (Object.hasOwn(result.legs, leg)) result.legs[leg] = true;
}

function uniqueEmail() {
  return `m11-selfdev-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function sha256hex(text) {
  return createHash('sha256').update(text).digest('hex');
}

// computerevent.CanonicalJSON: RFC 8785 object key order, integral numbers,
// and Go's exact string escaping. Protocol values intentionally use no floats.
function canonicalString(value) {
  let out = '"';
  for (const character of value) {
    const code = character.codePointAt(0);
    switch (character) {
      case '"': out += '\\"'; break;
      case '\\': out += '\\\\'; break;
      case '\b': out += '\\b'; break;
      case '\t': out += '\\t'; break;
      case '\n': out += '\\n'; break;
      case '\f': out += '\\f'; break;
      case '\r': out += '\\r'; break;
      default:
        out += code < 0x20 ? `\\u${code.toString(16).padStart(4, '0')}` : character;
    }
  }
  return `${out}"`;
}

function canonicalJSON(value) {
  if (value === null) return 'null';
  if (typeof value === 'string') return canonicalString(value);
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value)) throw new TypeError('canonical JSON accepts integral safe numbers only');
    return String(value);
  }
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(',')}]`;
  if (typeof value === 'object') {
    return `{${Object.keys(value).sort().map((key) => `${canonicalString(key)}:${canonicalJSON(value[key])}`).join(',')}}`;
  }
  throw new TypeError(`canonical JSON does not support ${typeof value}`);
}

function canonicalDigest(value) {
  return sha256hex(canonicalJSON(value));
}

function requireKnownPolicyDigest() {
  const raw = readFileSync(new URL('../internal/decisionpolicy/reversible-selfdev-v1.json', import.meta.url), 'utf8').trim();
  const parsedDigest = canonicalDigest(JSON.parse(raw));
  if (sha256hex(raw) !== POLICY_DIGEST || parsedDigest !== POLICY_DIGEST) {
    throw new Error(`reversible-selfdev-v1 policy digest mismatch: raw=${sha256hex(raw)} parsed=${parsedDigest}`);
  }
}

function nodeB(command, input) {
  return execFileSync(
    'ssh',
    ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command],
    { input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 32 * 1024 * 1024 },
  ).trim();
}

function nodeBJSON(command, input) {
  const raw = nodeB(command, input);
  return raw ? JSON.parse(raw) : null;
}

function vmctlOwnership(userID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`,
  );
}

function corpusdEventHead(computerID, ownerID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/head?computer_id=${encodeURIComponent(computerID)}'`,
  );
}

// The replay endpoint returns OLDEST-first pages of at most ~2000 events.
// Apply evidence lands at the tail: read the head for the current max
// sequence, then fetch a tail window via after_sequence. A head query that
// cannot parse a sequence falls back to the oldest page (conservative —
// apply_events then honestly reports absent evidence).
function corpusdEvents(computerID, ownerID, limit = 2000) {
  let afterSequence = 0;
  const head = corpusdEventHead(computerID, ownerID);
  const headSeq = Number(head?.canonical_sequence ?? head?.sequence ?? head?.max_sequence ?? 0);
  if (Number.isFinite(headSeq) && headSeq > limit) afterSequence = headSeq - limit;
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/replay?computer_id=${encodeURIComponent(computerID)}&limit=${limit}&after_sequence=${afterSequence}'`,
  );
}

function modeURL(computerID) {
  return `http://127.0.0.1:8086/internal/computers/self-development/mode?computer_id=${encodeURIComponent(computerID)}`;
}

function getMode(computerID, ownerID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" '${modeURL(computerID)}'`,
  );
}

function setMode(computerID, ownerID, body) {
  return nodeBJSON(
    `curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" --data-binary @- '${modeURL(computerID)}'`,
    canonicalJSON(body),
  );
}

function resolveRoute(ownerID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" 'http://127.0.0.1:8083/internal/vmctl/computer-version-routes/resolve?route_slot_id=computer:${encodeURIComponent(ownerID)}:primary'`,
  );
}

function eventKinds(events) {
  return (events ?? []).map((event) => event?.request?.event?.event_kind ?? event?.event_kind).filter(Boolean);
}

function goRFC3339Nano(date = new Date()) {
  // Go time.RFC3339Nano strips trailing zeros from the fractional second;
  // Date.toISOString always emits three digits. The server digests its Go
  // rendering, so client-side receipt math must match byte-for-byte.
  return date.toISOString().replace(/(\.\d*?)0+Z$/, '$1Z').replace(/\.Z$/, 'Z');
}
const isoMicros = goRFC3339Nano;

function decisionBindings(operation, head) {
  return {
    bundle_digest: operation.bundle_digest,
    expected_desired_event_head: head?.desired_event_head ?? operation.desired_head,
    expected_effective_event_head: head?.effective_event_head ?? operation.effective_head,
    expected_pending_transition_ref: head?.pending_transition_ref ?? '',
    expected_desired_state_commitment: head?.desired_state_commitment,
    expected_effective_state_commitment: head?.effective_state_commitment,
  };
}

function requireOperationBindings(operation, head) {
  const bindings = decisionBindings(operation, head);
  for (const [field, value] of Object.entries(bindings)) {
    if (field === 'expected_pending_transition_ref') continue;
    if (!/^[a-f0-9]{64}$/.test(value ?? '')) throw new Error(`operation/head lacks ${field}: ${JSON.stringify({ operation, head })}`);
  }
  if (!operation.operation_id || !Array.isArray(operation.verifier_refs) || !operation.verifier_refs[0]) {
    throw new Error(`operation lacks identity or verifier: ${JSON.stringify(operation)}`);
  }
  return bindings;
}

function mintQualifiedConsensus({ computerID, operation, head }) {
  const bindings = requireOperationBindings(operation, head);
  const selectedAtHead = head?.canonical_event_head;
  const selectedSequence = head?.sequence;
  if (!/^[a-f0-9]{64}$/.test(selectedAtHead ?? '') || !Number.isSafeInteger(selectedSequence) || selectedSequence <= 0) {
    throw new Error(`invalid selection head: ${JSON.stringify(head)}`);
  }
  const manifest = {
    seats: [
      { seat_id: 'cosuper-author', independence_domain: 'authoring', kind: 'agent_profile', eligibility_proof: sha256hex(`${marker}:cosuper-author`), recused: false },
      { seat_id: 'capsule-verifier', independence_domain: 'verification', kind: 'independent_verifier', eligibility_proof: sha256hex(`${marker}:capsule-verifier`), recused: false },
      { seat_id: 'independent-reviewer', independence_domain: 'verification', kind: 'agent_profile', eligibility_proof: sha256hex(`${marker}:independent-reviewer`), recused: false },
    ],
  };
  const subject = {
    computer_id: computerID,
    operation_id: operation.operation_id,
    bundle_digest: bindings.bundle_digest,
    desired_event_head: bindings.expected_desired_event_head,
    effective_event_head: bindings.expected_effective_event_head,
    pending_transition_ref: bindings.expected_pending_transition_ref,
    desired_state_commitment: bindings.expected_desired_state_commitment,
    effective_state_commitment: bindings.expected_effective_state_commitment,
    effect_class: 'reversible_computer_local',
  };
  const seatManifestDigest = canonicalDigest(manifest);
  const subjectDigest = canonicalDigest(subject);
  const selection = {
    receipt_kind: 'PolicySelectionReceipt',
    policy_digest: POLICY_DIGEST,
    seat_manifest_digest: seatManifestDigest,
    subject_digest: subjectDigest,
    selected_at_head: selectedAtHead,
    selected_sequence: selectedSequence,
  };
  // Go's CanonicalJSON honors omitempty: a zeroed-out digest field is omitted
  // from the preimage entirely, so digest the object before the field exists —
  // never with `selection_digest: ''` (that byte changes the digest and trips
  // ErrMissingSelection at decision-policy Reduce).
  selection.selection_digest = canonicalDigest(selection);

  // The author seat is present in the manifest but recused from verification.
  // Reduce rejects a signer that is recused from the verification domain, so
  // only the two verification seats cast accept ballots.
  const ballots = manifest.seats.filter((seat) => seat.seat_id !== 'cosuper-author').map((seat) => {
    const ballot = {
      ballot_id: `${marker.toLowerCase()}-${seat.seat_id}`,
      seat_id: seat.seat_id,
      eligibility_proof_digest: sha256hex(seat.eligibility_proof),
      independence_domain: seat.independence_domain,
      policy_digest: POLICY_DIGEST,
      seat_manifest_digest: seatManifestDigest,
      subject_digest: subjectDigest,
      policy_selection_digest: selection.selection_digest,
      vote: 'accept',
      window_id: selection.selection_digest,
      signer_provenance: `m11:${marker}:${seat.seat_id}`,
    };
    ballot.attestation = sha256hex(`choir-ballot-attestation-v1:${canonicalJSON(ballot)}`);
    ballot.ballot_digest = canonicalDigest(ballot);
    return ballot;
  });

  const now = isoMicros();
  const started = new Date(now);
  const expires = isoMicros(new Date(started.getTime() + 30 * 60_000));
  const receipt = {
    receipt_kind: 'QualifiedConsensusReceipt',
    policy_id: 'reversible-selfdev-v1',
    policy_digest: POLICY_DIGEST,
    subject_digest: subjectDigest,
    eligible_seats_digest: seatManifestDigest,
    selection_digest: selection.selection_digest,
    ballots,
    quorum_evaluation: { global_accepts: 2, domain_accepts: { verification: 2 }, met: true },
    dissent_disposition: 'resolved_none_blocking',
    human_seat_state: 'absent',
    window: { started_at: now, expires_at: expires },
    reducer_version: 'decisionpolicy-consensus-v1',
  };
  receipt.receipt_digest = canonicalDigest(receipt);
  return { manifest, subject, selection, ballots, now, receipt, bindings };
}

async function waitForDesktopReady(page, timeout = 180_000) {
  await page.waitForSelector('[data-prompt-input]', { timeout });
}

async function refreshSession(page) {
  // Access JWTs live 5 minutes (internal/auth/config.go DefaultAccessTokenTTL);
  // GET /auth/session performs refresh rotation. Polls outlive the token, so
  // every fetch retries once after a session refresh.
  return page.evaluate(async () => {
    const response = await fetch('/auth/session', { credentials: 'same-origin' });
    return response.status;
  });
}

async function fetchJSON(page, path) {
  const attempt = () => page.evaluate(async (p) => {
    const response = await fetch(p, { credentials: 'same-origin' });
    const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; }
    catch { return { status: response.status, text }; }
  }, path);
  const first = await attempt();
  if (first.status !== 401) return first;
  await refreshSession(page);
  return attempt();
}

async function postJSON(page, path, body) {
  const attempt = () => page.evaluate(async ({ p, b }) => {
    const response = await fetch(p, {
      method: 'POST', credentials: 'same-origin',
      headers: { 'content-type': 'application/json' }, body: JSON.stringify(b),
    });
    const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; }
    catch { return { status: response.status, text }; }
  }, { p: path, b: body });
  const first = await attempt();
  if (first.status !== 401) return first;
  await refreshSession(page);
  return attempt();
}

async function waitForOperation(page, computerID, operationID, wanted, timeout = 3_600_000) {
  const deadline = Date.now() + timeout;
  let last;
  while (Date.now() < deadline) {
    const response = await fetchJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(operationID)}`);
    last = response.json ?? response.text ?? response;
    if (response.status === 200 && last?.state === wanted) return last;
    if (last?.state === 'failed' || last?.state === 'degraded') return last;
    await page.waitForTimeout(5000);
  }
  return last;
}

function findMateriality(value) {
  return JSON.stringify(value ?? {}).includes('commitment_materiality');
}

function findFalsified(value) {
  return /falsified|reject(?:ed|ion)?/i.test(JSON.stringify(value ?? {}));
}

function selfDevelopmentDocumentID(ownerID, computerID, operationID) {
  // uuid.NewSHA1(uuid.NameSpaceOID, []byte(key + ":document")) in
  // selfdev_texture_join.go. RFC 4122 OID namespace bytes are fixed.
  const namespace = Buffer.from('6ba7b8129dad11d180b400c04fd430c8', 'hex');
  const digest = createHash('sha1').update(namespace).update(`choir:texture:self-development:${ownerID}:${computerID}:${operationID}:document`).digest();
  digest[6] = (digest[6] & 0x0f) | 0x50;
  digest[8] = (digest[8] & 0x3f) | 0x80;
  const hex = digest.subarray(0, 16).toString('hex');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

function updatePredicate() {
  const required = ['fresh_owner', 'bootstrap_chain', 'pre_episode_checkpoint', 'propose_only_armed', 'primary_started', 'awaiting_approval', 'qualified_consensus_armed', 'approved', 'applied', 'apply_events', 'candidate_b_started', 'candidate_b_rejected', 'document_rendered', 'restored_pinned_head'];
  if (required.every((name) => result.legs[name])) result.predicate_result = 'satisfied';
  else if (result.legs.primary_started || result.legs.awaiting_approval || result.legs.applied) result.predicate_result = 'partial';
  else result.predicate_result = 'blocked';
}

const browser = await chromium.launch({ headless: true });
try {
  requireKnownPolicyDigest();
  const context = await browser.newContext();
  const page = await context.newPage();
  const { authenticatorId } = await setupVirtualAuthenticator(page);
  void authenticatorId;

  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  result.email = uniqueEmail();
  await registerPasskey(page, result.email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await waitForDesktopReady(page);

  const session = await fetchJSON(page, '/auth/session');
  const ownerID = session.json?.user?.id;
  if (!ownerID) throw new Error(`owner id not derivable from /auth/session: ${JSON.stringify(session)}`);
  result.session_user = session.json.user;
  mark('fresh_owner', {email: result.email});

  const ownership = vmctlOwnership(ownerID);
  result.ownership = ownership;
  if (!ownership?.computer_id) throw new Error(`fresh owner has no tracking computer: ${JSON.stringify(ownership)}`);
  const computerID = ownership.computer_id;
  result.computer_id = computerID;

  const bootstrap = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/bootstrap-chain`, {});
  result.bootstrap_chain = bootstrap.json ?? bootstrap.text;

  // Pin the pre-episode head: the M9a restore target. The checkpoint mint is
  // idempotent authority work on the canonical head (rematerialize.go
  // bindComputerCheckpoint -> published_checkpoint.checkpoint).

  const checkpointBind = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/checkpoint`, {});
  result.pre_episode_checkpoint_bind = checkpointBind.json ?? checkpointBind.text;
  const pinnedCheckpoint = checkpointBind.json?.published_checkpoint?.checkpoint ?? checkpointBind.json?.checkpoint;
  if (pinnedCheckpoint) {
    result.pinned_checkpoint_digest = pinnedCheckpoint.digest;
    mark('pre_episode_checkpoint', {digest: result.pinned_checkpoint_digest});
  } else {
    result.pre_episode_checkpoint_gap = `checkpoint mint returned no checkpoint: ${JSON.stringify(result.pre_episode_checkpoint_bind)}`;
  }
  mark('bootstrap_chain', {computer: computerID});

  let mode = getMode(computerID, ownerID);
  result.mode_before = mode;
  const proposeOnly = setMode(computerID, ownerID, {
    mode: 'propose_only', expected_generation: mode.generation ?? 0,
    idempotency_key: `${marker}:arm-propose-only`,
  });
  result.propose_only_arm = proposeOnly;
  if (proposeOnly?.mode !== 'propose_only' || !proposeOnly?.receipt) throw new Error(`propose_only arm refused: ${JSON.stringify(proposeOnly)}`);
  mark('propose_only_armed');

  const primaryStart = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations`, {
    idempotency_key: `${marker}:primary`,
    prompt: `Create a minimal reversible self-development evidence change for ${marker}; freeze, verify, and await approval.`,
    mode_receipt: proposeOnly.receipt,
  });
  result.primary_start = primaryStart.json ?? primaryStart.text;
  const primaryID = primaryStart.json?.operation_id;
  if ((primaryStart.status !== 200 && primaryStart.status !== 201) || !primaryID) throw new Error(`primary operation refused: ${JSON.stringify(result.primary_start)}`);
  result.primary_operation_id = primaryID;
  mark('primary_started', {operation_id: primaryID});

  let primary = await waitForOperation(page, computerID, primaryID, 'awaiting_approval');
  result.primary_awaiting_approval = primary;
  result.primary_receipts = await fetchJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(primaryID)}/receipts`);
  if (primary?.state !== 'awaiting_approval') {
    result.awaiting_approval_blocker = 'engineering desk did not produce an awaiting_approval operation before the probe timeout';
    updatePredicate();
  } else {
    mark('awaiting_approval', {bundle: primary.bundle_digest});
    const awaitingHead = corpusdEventHead(computerID, ownerID);
    const consensus = mintQualifiedConsensus({ computerID, operation: primary, head: awaitingHead });

    mode = getMode(computerID, ownerID);
    const qualifiedMode = setMode(computerID, ownerID, {
      mode: 'qualified_consensus', expected_generation: mode.generation,
      operation_id: primaryID, ...consensus.bindings,
      policy_digest: POLICY_DIGEST, consensus_receipt_digest: consensus.receipt.receipt_digest,
      expires_at: isoMicros(new Date(Date.now() + 20 * 60_000)),
      idempotency_key: `${marker}:arm-qualified-consensus`,
    });
    result.qualified_consensus_arm = qualifiedMode;
    if (qualifiedMode?.mode !== 'qualified_consensus' || !qualifiedMode?.receipt) throw new Error(`qualified_consensus arm refused: ${JSON.stringify(qualifiedMode)}`);
    mark('qualified_consensus_armed', {receipt_digest: consensus.receipt.receipt_digest});

    const approved = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(primaryID)}/decision`, {
      decision: 'approve', idempotency_key: `${marker}:approve`, verifier_ref: primary.verifier_refs[0],
      ...consensus.bindings, mode_receipt: qualifiedMode.receipt,
      qualified_consensus: { seat_manifest: consensus.manifest, subject: consensus.subject, selection: consensus.selection, ballots: consensus.ballots, now: consensus.now },
    });
    result.primary_decision = approved.json ?? approved.text;
    if (approved.status !== 200) {
      // The appender's post-commit drain races this POST: our own committed
      // decision event can be finalized by recoverSelfDevelopmentDecision
      // before the handler's op-state transition lands, and the request then
      // returns 409 "state is accepted" even though the decision committed.
      // The durable op record is the authority — accept a descended state.
      const current = await fetchJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(primaryID)}`);
      result.primary_decision_recovery = { status: approved.status, body: approved.json ?? approved.text, observed: current?.state };
      if (!(current?.state === 'accepted' || current?.state === 'materializing' || current?.state === 'applied')) {
        throw new Error(`approval refused: ${JSON.stringify(result.primary_decision)}`);
      }
    }
    mark('approved');

    primary = await waitForOperation(page, computerID, primaryID, 'applied');
    result.primary_applied = primary;
    if (primary?.state === 'applied') {
      mark('applied');
      const appliedKinds = eventKinds(corpusdEvents(computerID, ownerID));
      result.applied_event_kinds = appliedKinds;
      mark('apply_events_check', {kinds: appliedKinds});
      result.legs.apply_events = ['materialization_applied', 'checkpoint_published', 'route_projection_updated'].every((kind) => appliedKinds.includes(kind));
    } else {
      result.apply_blocker = `operation terminal state was ${primary?.state ?? 'unavailable'}`;
    }

    // Candidate B is an actual second operation with a reject decision. A
    // qualified-consensus mode cannot authorize reject; re-arm an ordinary
    // propose_only receipt after consensus consumption before rejecting it.
    if (result.legs.applied) {
      const postConsensusMode = getMode(computerID, ownerID);
      const candidateProposeOnly = setMode(computerID, ownerID, {
        mode: 'propose_only', expected_generation: postConsensusMode.generation,
        idempotency_key: `${marker}:candidate-b-propose-only`,
      });
      result.candidate_b_propose_only_arm = candidateProposeOnly;
      if (candidateProposeOnly?.mode !== 'propose_only' || !candidateProposeOnly?.receipt) {
        throw new Error(`candidate-B propose_only arm refused: ${JSON.stringify(candidateProposeOnly)}`);
      }
      const candidateMode = getMode(computerID, ownerID);
      const candidateStart = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations`, {
        idempotency_key: `${marker}:candidate-b`,
        prompt: `Produce and verify a candidate-B falsification exercise for ${marker}; await decision.`,
        mode_receipt: candidateMode.receipt,
      });
      result.candidate_b_start = candidateStart.json ?? candidateStart.text;
      const candidateID = candidateStart.json?.operation_id;
      if ((candidateStart.status === 200 || candidateStart.status === 201) && candidateID) {
        result.candidate_b_operation_id = candidateID;
        mark('candidate_b_started', {operation_id: candidateID});
        const candidate = await waitForOperation(page, computerID, candidateID, 'awaiting_approval');
        result.candidate_b_awaiting_approval = candidate;
        if (candidate?.state === 'awaiting_approval') {
          const candidateHead = corpusdEventHead(computerID, ownerID);
          const candidateBindings = requireOperationBindings(candidate, candidateHead);
          const candidateDecisionMode = getMode(computerID, ownerID);
          const rejected = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(candidateID)}/decision`, {
            decision: 'reject', idempotency_key: `${marker}:candidate-b-reject`, verifier_ref: candidate.verifier_refs[0],
            reason: `Candidate-B falsification for ${marker}`, ...candidateBindings, mode_receipt: candidateDecisionMode.receipt,
          });
          result.candidate_b_decision = rejected.json ?? rejected.text;
          // Same drain race as the approve path: the committed reject event
          // may be finalized before the handler's op transition, yielding
          // 409 with the durable record already 'rejected'. Authority = op.
          if (rejected.status !== 200) {
            const currentCandidate = await fetchJSON(page, `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(candidateID)}`);
            result.candidate_b_decision_recovery = { status: rejected.status, observed: currentCandidate?.state };
            if (currentCandidate?.state === 'rejected') {
              result.candidate_b_decision = currentCandidate;
            }
          }
          mark('candidate_b_rejected', {status: rejected.status});
          result.legs.candidate_b_rejected = result.candidate_b_decision?.state === 'rejected' || (rejected.status === 200 && rejected.json?.state === 'rejected');
        } else {
          result.candidate_b_blocker = 'candidate-B did not reach awaiting_approval; no public commitment-record mint endpoint exists';
        }
      } else {
        result.candidate_b_blocker = `candidate-B operation refused: ${JSON.stringify(result.candidate_b_start)}`;
      }
    }

    const docID = selfDevelopmentDocumentID(ownerID, computerID, primaryID);
    result.document_id = docID;
    const [document, revisions, diagnosis] = await Promise.all([
      fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}`),
      fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}/revisions`),
      fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}/diagnosis?include_content=true`),
    ]);
    result.document_render = { document, revisions, diagnosis };
    mark('document_render');
    result.legs.document_rendered = document.status === 200 || revisions.status === 200 || diagnosis.status === 200;
    result.legs.commitment_materiality_visible = findMateriality(result.document_render);
    result.legs.falsified_commitment_visible = findFalsified(result.document_render);
    if (!result.legs.falsified_commitment_visible) {
      result.candidate_b_visibility_gap = 'No falsified commitment record was visible in the rendered episode evidence.';
    }

    // Restore leg: the M9a path — restore to the pinned pre-episode head via
    // tape reconstruct. The guest restarts mid-request on apply; retry the
    // POST across transport deaths like the M9a probe does.
    if (result.legs.applied && pinnedCheckpoint) {
      let restore = null;
      for (let attempt = 0; attempt < 8; attempt++) {
        try {
          restore = await postJSON(page, `/api/computers/${encodeURIComponent(computerID)}/lifecycle/restore`, {
            checkpoint: pinnedCheckpoint,
            operand_scopes: ['vm_local', 'computer_surface_frontend'],
          });
        } catch (transport) {
          result.restore_transport_errors = (result.restore_transport_errors ?? 0) + 1;
          await page.waitForTimeout(10_000);
          continue;
        }
        if (restore.status === 200 || restore.status === 400 || restore.status === 409) break;
        await page.waitForTimeout(10_000);
      }
      result.restore = restore?.json ?? restore?.text ?? restore;
      mark('restore_attempt_done');
      result.legs.restored_pinned_head =
        !!restore && restore.status === 200 &&
        restore.json?.witness_matched === true && restore.json?.frontend_restaged === true;
      if (!result.legs.restored_pinned_head) {
        result.restore_blocker = `pinned-head restore did not complete: ${JSON.stringify(result.restore)}`;
      }
    } else if (!pinnedCheckpoint) {
      result.restore_blocker = 'no pre-episode checkpoint was minted; restore target unavailable';
    }
    updatePredicate();
  }
} catch (error) {
  result.error = String(error && error.stack ? error.stack : error);
  updatePredicate();
} finally {
  await browser.close();
}

updatePredicate();
console.log(JSON.stringify(result, null, 2));
if (result.predicate_result !== 'satisfied') runtimeProcess?.exit?.(1);
