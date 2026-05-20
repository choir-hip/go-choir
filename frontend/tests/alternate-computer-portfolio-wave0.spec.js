import fs from 'node:fs/promises';
import path from 'node:path';
import { test, expect } from './helpers/fixtures.js';
import { getSession, registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'https://draft.choir-ip.com';
const RUN_WAVE0 = process.env.GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE0 === '1';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(1_500_000);
test.skip(!RUN_WAVE0, 'set GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE0=1 to run deployed portfolio Wave 0 proof');

function uniqueEmail() {
  return `alt-portfolio-wave0-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function safeID(value) {
  return String(value).replace(/[^a-zA-Z0-9_.-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 96);
}

function markerPatch(marker) {
  const file = `docs/portfolio-wave0-substrate-marker-${safeID(marker)}.md`;
  const body = [
    `# Portfolio Wave 0 Substrate Marker ${marker}`,
    '',
    'This file is a tiny AppChangePackage payload used to prove recipient',
    'adoption/rebuild/promote/rollback evidence through product APIs.',
    '',
  ].join('\n');
  const lines = body.split('\n');
  return {
    file,
    patch: [
      `diff --git a/${file} b/${file}`,
      'new file mode 100644',
      'index 0000000..1111111',
      '--- /dev/null',
      `+++ b/${file}`,
      `@@ -0,0 +1,${lines.length} @@`,
      ...lines.map((line) => `+${line}`),
      '',
    ].join('\n'),
  };
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 180_000,
  });
}

async function fetchJSON(page, requestPath) {
  return page.evaluate(async (pathToFetch) => {
    async function read(res) {
      const text = await res.text();
      let json = null;
      try {
        json = text ? JSON.parse(text) : null;
      } catch (_err) {
        json = null;
      }
      return { ok: res.ok, status: res.status, text, json };
    }

    let res = await fetch(pathToFetch, { credentials: 'include' });
    if (res.status === 401) {
      const session = await fetch('/auth/session', { credentials: 'include' })
        .then((sessionRes) => sessionRes.json().catch(() => null))
        .catch(() => null);
      if (session?.authenticated) {
        res = await fetch(pathToFetch, { credentials: 'include' });
      }
    }
    return read(res);
  }, requestPath);
}

async function postJSON(page, requestPath, payload) {
  return page.evaluate(async ({ pathToFetch, body }) => {
    async function read(res) {
      const text = await res.text();
      let json = null;
      try {
        json = text ? JSON.parse(text) : null;
      } catch (_err) {
        json = null;
      }
      return { ok: res.ok, status: res.status, text, json };
    }

    const options = {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(body),
    };
    let res = await fetch(pathToFetch, options);
    if (res.status === 401) {
      const session = await fetch('/auth/session', { credentials: 'include' })
        .then((sessionRes) => sessionRes.json().catch(() => null))
        .catch(() => null);
      if (session?.authenticated) {
        res = await fetch(pathToFetch, options);
      }
    }
    return read(res);
  }, { pathToFetch: requestPath, body: payload });
}

async function rawFetchJSON(page, requestPath) {
  return page.evaluate(async (pathToFetch) => {
    const res = await fetch(pathToFetch, { credentials: 'include' });
    const text = await res.text();
    let json = null;
    try {
      json = text ? JSON.parse(text) : null;
    } catch (_err) {
      json = null;
    }
    return { ok: res.ok, status: res.status, text, json };
  }, requestPath);
}

async function requirePostJSON(page, requestPath, payload) {
  const res = await postJSON(page, requestPath, payload);
  if (!res.ok) {
    throw new Error(`${requestPath} failed: ${res.status} ${res.text}`);
  }
  return res.json;
}

async function requireFetchJSON(page, requestPath) {
  const res = await fetchJSON(page, requestPath);
  if (!res.ok) {
    throw new Error(`${requestPath} failed: ${res.status} ${res.text}`);
  }
  return res.json;
}

async function writeEvidence(testInfo, name, data) {
  const configured = process.env.GO_CHOIR_ALT_PORTFOLIO_EVIDENCE_DIR;
  const dir = configured || testInfo.outputPath('evidence');
  await fs.mkdir(dir, { recursive: true });
  const file = path.join(dir, name);
  await fs.writeFile(file, JSON.stringify(data, null, 2));
  await testInfo.attach(name, { path: file, contentType: 'application/json' });
  return file;
}

async function createVTextReport(page, content) {
  const doc = await requirePostJSON(page, '/api/vtext/documents', {
    title: 'Alternate Computer Portfolio Wave 0 Evidence',
  });
  const revision = await requirePostJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    content,
    metadata: { mission_id: 'mission-alternate-computer-ux-experiment-portfolio-v0', lane: 'wave0' },
  });
  return { doc, revision };
}

test('Wave 0 proves current package/adoption substrate or records precise blocker', async ({ page, authenticator }, testInfo) => {
  expect(authenticator.authenticatorId).toBeTruthy();
  const email = uniqueEmail();
  const marker = `alt-portfolio-wave0-${Date.now()}`;
  const traceID = `traj-${marker}`;
  const sourceComputerID = `source-computer-${marker}`;
  const sourceCandidateID = `candidate-source-${marker}`;
  const targetComputerID = `target-computer-${marker}`;
  const targetCandidateID = `candidate-target-${marker}`;
  const packageID = `package-${marker}`;
  const adoptionID = `adoption-${marker}`;
  const patch = markerPatch(marker);

  const forbiddenRequests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (
      url.pathname.startsWith('/internal/') ||
      url.pathname.startsWith('/api/agent/') ||
      url.pathname.startsWith('/api/prompts') ||
      url.pathname.startsWith('/api/test/') ||
      url.pathname === '/api/events'
    ) {
      forbiddenRequests.push(`${request.method()} ${url.pathname}`);
    }
  });

  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  expect(session.authenticated).toBe(true);
  expect(session.user?.email).toBe(email);

  const legacyPromotions = await rawFetchJSON(page, '/api/promotions');
  expect(legacyPromotions.status).toBe(404);

  const health = await requireFetchJSON(page, '/health');
  const sourceLineage = await requireFetchJSON(page, `/api/computers/${encodeURIComponent(sourceComputerID)}/source-lineage`);
  const targetLineageStart = await requireFetchJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/source-lineage`);

  const pkg = await requirePostJSON(page, '/api/app-change-packages', {
    package_id: packageID,
    app_id: 'alternate-portfolio-wave0',
    visibility: 'unlisted',
    source_computer_id: sourceComputerID,
    source_candidate_id: sourceCandidateID,
    candidate_source_ref: `refs/computers/${sourceComputerID}/candidates/${sourceCandidateID}`,
    source_ledger_repo: 'https://github.com/yusefmosiah/choir-source-ledger.git',
    source_ledger_base_ref: 'origin/main',
    source_ledger_candidate_ref: `refs/computers/${sourceComputerID}/candidates/${sourceCandidateID}`,
    source_ledger_commit_sha: health.build?.deployed_commit || health.build?.commit || 'unknown',
    runtime_source_delta: patch.patch,
    ui_source_delta: '',
    app_protocol_contract: 'recipient_build_required: Wave 0 marker package must rebuild runtime and UI in the recipient computer before adoption.',
    verifier_contracts: [
      { contract_id: 'actual-recipient-runtime-ui-build', required: true },
      { contract_id: 'no-cross-computer-binary-copying', required: true },
      { contract_id: 'rollback-available', required: true },
    ],
    provenance_refs: [
      { kind: 'mission', ref: 'docs/mission-alternate-computer-ux-experiment-portfolio-v0.md' },
      { kind: 'marker_file', ref: patch.file },
    ],
    trace_id: traceID,
  });

  const adoption = await requirePostJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/adoptions`, {
    adoption_id: adoptionID,
    package_id: pkg.package_id,
    target_candidate_id: targetCandidateID,
    candidate_source_ref: `refs/computers/${targetComputerID}/candidates/${targetCandidateID}`,
    foreground_tail_merge_result: 'wave0-no-conflict',
    merge_strategy: 'rebase',
    trace_id: traceID,
  });
  expect(adoption.status).toBe('candidate_applied');

  const verify = await postJSON(page, `/api/adoptions/${encodeURIComponent(adoptionID)}/verify`, {
    target_active_source_ref_at_cutover: `${targetLineageStart.active_source_ref}-foreground-tail-${marker}`,
    foreground_tail_merge_result: 'wave0-no-conflict',
    merge_strategy: 'rebase',
    merge_conflicts: [],
  });

  let verified = null;
  let promoted = null;
  let acceptance = null;
  let finalAdoption = null;
  let targetLineageEnd = null;
  let wave0Status = 'blocked_incomplete';
  let preciseBlocker = null;

  if (verify.ok) {
    verified = verify.json;
    expect(verified.status).toBe('verified');
    expect(verified.runtime_artifact_digest || '').toBeTruthy();
    expect(verified.ui_artifact_digest || '').toBeTruthy();
    expect(verified.runtime_artifact_digest).not.toBe(pkg.source_runtime_artifact_digest);
    expect(verified.ui_artifact_digest).not.toBe(pkg.source_ui_artifact_digest);

    promoted = await requirePostJSON(page, `/api/adoptions/${encodeURIComponent(adoptionID)}/promote`, {});
    expect(promoted.status).toBe('adopted');
    targetLineageEnd = await requireFetchJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/source-lineage`);
    expect(targetLineageEnd.last_adoption_id).toBe(adoptionID);

    const acceptanceRes = await postJSON(page, '/api/run-acceptances/synthesize', {
      target_mission_id: 'mission-alternate-computer-ux-experiment-portfolio-v0-wave0',
      source_prompt_or_objective: 'Wave 0: prove AppChangePackage adoption recipient build substrate for the alternate-computer experiment portfolio.',
      trajectory_id: traceID,
      staging_url: new URL(BASE_URL).origin,
    });
    if (acceptanceRes.ok) {
      acceptance = acceptanceRes.json;
      expect(['promotion-level', 'continuation-level']).toContain(acceptance.acceptance_level);
      if (acceptance.state !== 'accepted') {
        const blockedInvariants = (acceptance.invariant_checks || [])
          .filter((check) => check.state === 'blocked')
          .map((check) => check.name)
          .join(', ');
        preciseBlocker = `run acceptance synthesized ${acceptance.acceptance_level}/${acceptance.state}; blocked invariants: ${blockedInvariants || 'unknown'}`;
      }
    } else {
      preciseBlocker = `run acceptance synthesis failed: ${acceptanceRes.status} ${acceptanceRes.text}`;
    }
    wave0Status = 'checkpoint_package';
    finalAdoption = promoted;
  } else {
    finalAdoption = await requireFetchJSON(page, `/api/adoptions/${encodeURIComponent(adoptionID)}`);
    preciseBlocker = `verify failed: ${verify.status} ${verify.text}`;
    expect(finalAdoption.status).toBe('blocked');
    expect(finalAdoption.error || finalAdoption.verifier_results_json || '').toBeTruthy();
  }

  const trace = await requireFetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(traceID)}`);
  const report = {
    status: wave0Status,
    base_url: BASE_URL,
    account_email: email,
    account_user_id: session.user?.id,
    manual_owner_loginability: 'blocked: Playwright-created passkey credentials live in the virtual authenticator and are not transferable to the owner without an enrollment/review path.',
    marker,
    deployed_commit: health.build?.deployed_commit || health.build?.commit || '',
    source_lineage: sourceLineage,
    target_lineage_start: targetLineageStart,
    target_lineage_end: targetLineageEnd,
    package: pkg,
    adoption_initial: adoption,
    verify_status: verify.status,
    verified,
    promoted,
    final_adoption: finalAdoption,
    acceptance,
    precise_blocker: preciseBlocker,
    trace_mobile_summary: trace.mobile_summary,
    forbidden_browser_requests: forbiddenRequests,
    learning_log: [{
      timestamp: new Date().toISOString(),
      lane_or_substrate: 'Wave 0',
      observed_situation: 'Product APIs can create package/adoption evidence from a real authenticated account; manual owner loginability is not automatic for virtual passkey accounts.',
      mission_gradient_pressure: 'Continue into package/adoption proof without pretending Playwright-created accounts are owner-loginable tomorrow.',
      decision_taken: 'Treat package/adoption proof as checkpoint_package evidence unless a real owner review path is available.',
      evidence_produced: 'Authenticated package/adoption/verify/promote product API responses, Trace snapshot, optional run acceptance.',
      cost_risk: 'Full recipient build can be slow; account credentials are not reusable by the owner.',
      learning: 'Evidence gates prevented fake loginable_experiment success.',
      possible_future_skill_simplification: 'Separate loginability proof from package/adoption proof in MissionGradient portfolio missions.',
    }],
  };

  const vtext = await createVTextReport(page, [
    '# Alternate Computer Portfolio Wave 0 Evidence',
    '',
    `Status: ${wave0Status}`,
    `Account: ${email}`,
    `Package: ${pkg.package_id}`,
    `Adoption: ${adoptionID}`,
    `Trace: ${traceID}`,
    `Acceptance: ${acceptance?.acceptance_id || 'not synthesized'}`,
    `Deployed commit: ${report.deployed_commit}`,
    '',
    'Manual owner loginability:',
    report.manual_owner_loginability,
    '',
    'Precise blocker:',
    preciseBlocker || 'none',
    '',
    'Rollback refs:',
    JSON.stringify(finalAdoption?.rollback_profile_json || finalAdoption?.rollback_profile || {}, null, 2),
  ].join('\n'));
  report.vtext = vtext;

  await writeEvidence(testInfo, 'alternate-portfolio-wave0-evidence.json', report);
  const screenshot = testInfo.outputPath('alternate-portfolio-wave0-desktop.png');
  await page.screenshot({ path: screenshot, fullPage: false });
  await testInfo.attach('alternate-portfolio-wave0-desktop', { path: screenshot, contentType: 'image/png' });

  expect(forbiddenRequests).toHaveLength(0);
});
