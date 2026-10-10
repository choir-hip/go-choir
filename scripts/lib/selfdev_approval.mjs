// Shared self-development approval helpers for staging harnesses: canonical
// JSON digests matching computerevent.CanonicalJSON, Node B internal reads,
// signed mode arming, and the reversible-selfdev-v1 qualified-consensus
// approval. Extracted from scripts/m11_selfdev_episode_probe.mjs.
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

export const POLICY_DIGEST = 'c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7';

export function sha256hex(text) {
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

export function canonicalJSON(value) {
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

export function canonicalDigest(value) {
  return sha256hex(canonicalJSON(value));
}

export function requireKnownPolicyDigest() {
  const raw = readFileSync(new URL('../../internal/decisionpolicy/reversible-selfdev-v1.json', import.meta.url), 'utf8').trim();
  const parsedDigest = canonicalDigest(JSON.parse(raw));
  if (sha256hex(raw) !== POLICY_DIGEST || parsedDigest !== POLICY_DIGEST) {
    throw new Error(`reversible-selfdev-v1 policy digest mismatch: raw=${sha256hex(raw)} parsed=${parsedDigest}`);
  }
}

export function nodeB(command, input) {
  return execFileSync(
    'ssh',
    ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command],
    { input, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe'], maxBuffer: 32 * 1024 * 1024 },
  ).trim();
}

export function nodeBJSON(command, input) {
  const raw = nodeB(command, input);
  return raw ? JSON.parse(raw) : null;
}

export function corpusdEventHead(computerID, ownerID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" 'http://127.0.0.1:8086/internal/computers/events/head?computer_id=${encodeURIComponent(computerID)}'`,
  );
}

function modeURL(computerID) {
  return `http://127.0.0.1:8086/internal/computers/self-development/mode?computer_id=${encodeURIComponent(computerID)}`;
}

export function getMode(computerID, ownerID) {
  return nodeBJSON(`curl -fsS -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" '${modeURL(computerID)}'`);
}

export function setMode(computerID, ownerID, body) {
  return nodeBJSON(
    `curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" -H "X-Authenticated-User: ${ownerID}" --data-binary @- '${modeURL(computerID)}'`,
    canonicalJSON(body),
  );
}

// Go time.RFC3339Nano strips trailing zeros from the fractional second.
export function goRFC3339Nano(date = new Date()) {
  return date.toISOString().replace(/(\.\d*?)0+Z$/, '$1Z').replace(/\.Z$/, 'Z');
}

export function decisionBindings(operation, head) {
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

// mintQualifiedConsensus builds the reversible-selfdev-v1 bundle the decision
// route reduces. The ballots are harness-minted attestations, not
// independently signed votes (see the probe's notes).
export function mintQualifiedConsensus({ marker, computerID, operation, head }) {
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
  selection.selection_digest = canonicalDigest(selection);
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
      signer_provenance: `harness:${marker}:${seat.seat_id}`,
    };
    ballot.attestation = sha256hex(`choir-ballot-attestation-v1:${canonicalJSON(ballot)}`);
    ballot.ballot_digest = canonicalDigest(ballot);
    return ballot;
  });
  const now = goRFC3339Nano();
  const expires = goRFC3339Nano(new Date(new Date(now).getTime() + 30 * 60_000));
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

// approveOperation arms qualified_consensus for this exact operation and
// posts the owner decision. postJSON(path, body) carries the owner session.
export async function approveOperation({ marker, computerID, ownerID, operation, postJSON, getJSON }) {
  const head = corpusdEventHead(computerID, ownerID);
  const consensus = mintQualifiedConsensus({ marker, computerID, operation, head });
  const mode = getMode(computerID, ownerID);
  const armed = setMode(computerID, ownerID, {
    mode: 'qualified_consensus', expected_generation: mode.generation,
    operation_id: operation.operation_id, ...consensus.bindings,
    policy_digest: POLICY_DIGEST, consensus_receipt_digest: consensus.receipt.receipt_digest,
    expires_at: goRFC3339Nano(new Date(Date.now() + 20 * 60_000)),
    idempotency_key: `${marker}:arm-qualified-consensus:${operation.operation_id}`,
  });
  if (armed?.mode !== 'qualified_consensus' || !armed?.receipt) throw new Error(`qualified_consensus arm refused: ${JSON.stringify(armed)}`);
  const path = `/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(operation.operation_id)}`;
  const decided = await postJSON(`${path}/decision`, {
    decision: 'approve', idempotency_key: `${marker}:approve:${operation.operation_id}`, verifier_ref: operation.verifier_refs[0],
    ...consensus.bindings, mode_receipt: armed.receipt,
    qualified_consensus: { seat_manifest: consensus.manifest, subject: consensus.subject, selection: consensus.selection, ballots: consensus.ballots, now: consensus.now },
  });
  if (decided.status === 200) return decided;
  // The decision can commit before the handler's own transition lands; the
  // durable operation record is the authority.
  const current = await getJSON(path);
  if (['accepted', 'materializing', 'applied'].includes(current?.json?.state)) return { status: 200, json: current.json, recovered: decided };
  throw new Error(`approval refused: ${JSON.stringify(decided.json ?? decided.text)}`);
}
