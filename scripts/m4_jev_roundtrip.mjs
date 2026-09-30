#!/usr/bin/env node
// Run this inside an already-authorized staging VM, never from the browser or
// workstation. It exercises the VM's RUNTIME_GATEWAY_TOKEN against the pinned
// /provider/v1/judgments route and prints a diagnostic JSON result.
//
// Normal VM A/B isolation sequence:
//   1. Run in VM A: node scripts/m4_jev_roundtrip.mjs
//   2. Run in VM B: node scripts/m4_jev_roundtrip.mjs
//   3. Run in VM A with JEV_CROSS_COMPUTER_TOKEN=<VM B token>; expect 403.
//   4. Temporarily set GATEWAY_RATE_LIMIT_MAX_REQUESTS low, run VM A with
//      JEV_CALLS=<limit+1> JEV_EXPECT_RATE_LIMIT=1, then run VM B normally;
//      VM B's 200 proves it owns an independent judgment bucket.
// The gateway writes the authoritative request/distribution receipt; this
// probe only records transport status and the returned distribution for the
// deploy acceptance transcript.

const gatewayURL = (process.env.RUNTIME_GATEWAY_URL || '').replace(/\/$/, '');
const token = process.env.RUNTIME_GATEWAY_TOKEN || '';
const crossToken = process.env.JEV_CROSS_COMPUTER_TOKEN || '';
const calls = Number(process.env.JEV_CALLS || '1');
const expectRateLimit = process.env.JEV_EXPECT_RATE_LIMIT === '1';

if (!gatewayURL || !token) {
  throw new Error('RUNTIME_GATEWAY_URL and RUNTIME_GATEWAY_TOKEN are required; run inside an authorized VM');
}
if (!Number.isInteger(calls) || calls < 1) {
  throw new Error('JEV_CALLS must be a positive integer');
}

const payload = {
  state: {
    hypothesis: 'The gateway can transport one typed Jev choice decision.',
    evidence: ['M4 deployed transport acceptance probe'],
    outcome: 'transport acceptance pending',
  },
  questions: {
    transport_ready: {
      type: 'choice',
      instructions: 'Choose whether the transport condition is satisfied from the supplied state.',
      criteria: {
        yes: 'The gateway route returned a pinned-model decision distribution.',
        no: 'The gateway route did not return a pinned-model decision distribution.',
      },
      choices: ['yes', 'no'],
    },
  },
};

async function request(bearer) {
  const response = await fetch(`${gatewayURL}/provider/v1/judgments`, {
    method: 'POST',
    headers: bearer ? { Authorization: `Bearer ${bearer}`, 'Content-Type': 'application/json' } : { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  const text = await response.text();
  let body;
  try {
    body = JSON.parse(text);
  } catch {
    body = { raw: text };
  }
  return { status: response.status, body };
}

const result = {
  probe: 'm4_jev_roundtrip',
  at: new Date().toISOString(),
  gateway: gatewayURL,
  missing_bearer: (await request('')).status,
  invalid_bearer: (await request('not-a-valid-vm-bearer')).status,
  valid_calls: [],
};

for (let i = 0; i < calls; i += 1) result.valid_calls.push(await request(token));
if (crossToken) result.cross_computer_bearer = await request(crossToken);

const validStatuses = result.valid_calls.map(({ status }) => status);
const lastStatus = validStatuses.at(-1);
const refusalOK = result.missing_bearer === 401 && result.invalid_bearer === 401;
const validOK = expectRateLimit ? lastStatus === 429 : validStatuses.every((status) => status === 200);
const crossOK = !crossToken || result.cross_computer_bearer.status === 403;
result.ok = refusalOK && validOK && crossOK;

console.log(JSON.stringify(result, null, 2));
if (!result.ok) process.exitCode = 1;
