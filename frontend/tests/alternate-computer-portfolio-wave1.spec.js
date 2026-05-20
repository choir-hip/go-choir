import fs from 'node:fs/promises';
import path from 'node:path';
import { test, expect } from './helpers/fixtures.js';
import { getSession, registerPasskey } from './helpers/auth.js';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'https://draft.choir-ip.com';
const RUN_WAVE1 = process.env.GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE1 === '1';
const PACKAGE_WAIT_MS = Number(process.env.GO_CHOIR_ALT_PORTFOLIO_WAVE1_PACKAGE_WAIT_MS || 40 * 60 * 1000);

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(Math.max(PACKAGE_WAIT_MS + 20 * 60 * 1000, 60 * 60 * 1000));
test.skip(!RUN_WAVE1, 'set GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE1=1 to run deployed portfolio Wave 1 proof');

function uniqueEmail(prefix) {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function safeID(value) {
  return String(value).replace(/[^a-zA-Z0-9_.-]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 96);
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

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 180_000,
  });
}

async function newRegisteredDesktop(browser, emailPrefix) {
  const context = await browser.newContext();
  const page = await context.newPage();
  const { client, authenticatorId } = await setupVirtualAuthenticator(page);
  const email = uniqueEmail(emailPrefix);
  await registerAndLoadDesktop(page, email);
  const session = await getSession(page, BASE_URL);
  if (!session.authenticated) {
    throw new Error(`failed to authenticate ${email}`);
  }
  return {
    context,
    page,
    email,
    session,
    close: async () => {
      await removeVirtualAuthenticator(client, authenticatorId).catch(() => {});
      await context.close();
    },
  };
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

async function createVTextReport(page, title, lane, content) {
  const doc = await requirePostJSON(page, '/api/vtext/documents', { title });
  const revision = await requirePostJSON(page, `/api/vtext/documents/${encodeURIComponent(doc.doc_id)}/revisions`, {
    content,
    metadata: { mission_id: 'mission-alternate-computer-ux-experiment-portfolio-v0', lane },
  });
  return { doc, revision };
}

async function submitPromptBar(page, text) {
  const res = await postJSON(page, '/api/prompt-bar', { text });
  if (!res.ok) {
    throw new Error(`/api/prompt-bar failed: ${res.status} ${res.text}`);
  }
  return res.json;
}

async function promptStatus(page, submissionId) {
  const res = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
  return res.ok ? res.json : { state: 'unknown', error: res.text, status: res.status };
}

async function traceSnapshot(page, trajectoryId) {
  const res = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
  return res.ok ? res.json : { error: res.text, status: res.status };
}

function packageMatchesLane(pkg, lane) {
  const haystack = JSON.stringify(pkg || {}).toLowerCase();
  return haystack.includes(lane.marker.toLowerCase()) || haystack.includes(lane.appID.toLowerCase());
}

async function listLanePackages(page, lanes) {
  const listed = await requireFetchJSON(page, '/api/app-change-packages?limit=100');
  const packages = Array.isArray(listed.packages) ? listed.packages : [];
  const byLane = {};
  for (const lane of lanes) {
    byLane[lane.id] = packages.filter((pkg) => packageMatchesLane(pkg, lane));
  }
  return { packages, byLane };
}

async function waitForLanePackages(page, lanes, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let latest = null;
  while (Date.now() < deadline) {
    latest = await listLanePackages(page, lanes);
    if (lanes.every((lane) => latest.byLane[lane.id]?.length > 0)) {
      return latest;
    }
    await page.waitForTimeout(10_000);
  }
  return latest || await listLanePackages(page, lanes);
}

async function adoptPackageIntoRecipient(page, lane, pkg, marker) {
  const targetComputerID = `owner-review-${lane.id}-${safeID(marker)}`;
  const targetCandidateID = `candidate-owner-review-${lane.id}-${safeID(marker)}`;
  const adoptionID = `adoption-owner-review-${lane.id}-${safeID(marker)}`;
  const traceID = pkg.trace_id || lane.submission_id || `traj-owner-review-${lane.id}-${safeID(marker)}`;

  const detail = await fetchJSON(page, `/api/app-change-packages/${encodeURIComponent(pkg.package_id)}`);
  if (!detail.ok) {
    return {
      status: 'blocked_incomplete',
      blocker: `recipient could not inspect package ${pkg.package_id}: ${detail.status} ${detail.text}`,
    };
  }

  const lineageStart = await requireFetchJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/source-lineage`);
  const adoption = await requirePostJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/adoptions`, {
    adoption_id: adoptionID,
    package_id: pkg.package_id,
    target_candidate_id: targetCandidateID,
    candidate_source_ref: `refs/computers/${targetComputerID}/candidates/${targetCandidateID}`,
    foreground_tail_merge_result: 'wave1-no-conflict',
    merge_strategy: 'rebase',
    trace_id: traceID,
  });

  const verify = await postJSON(page, `/api/adoptions/${encodeURIComponent(adoptionID)}/verify`, {
    target_active_source_ref_at_cutover: `${lineageStart.active_source_ref}-foreground-tail-${safeID(marker)}`,
    foreground_tail_merge_result: 'wave1-no-conflict',
    merge_strategy: 'rebase',
    merge_conflicts: [],
  });

  if (!verify.ok) {
    const finalAdoption = await fetchJSON(page, `/api/adoptions/${encodeURIComponent(adoptionID)}`);
    return {
      status: 'checkpoint_package',
      package: detail.json,
      adoption_initial: adoption,
      verify_status: verify.status,
      blocker: `recipient verify failed: ${verify.status} ${verify.text}`,
      final_adoption: finalAdoption.json,
      rollback_refs: finalAdoption.json?.rollback_profile_json || finalAdoption.json?.rollback_profile || null,
    };
  }

  const promoted = await postJSON(page, `/api/adoptions/${encodeURIComponent(adoptionID)}/promote`, {});
  const lineageEnd = await fetchJSON(page, `/api/computers/${encodeURIComponent(targetComputerID)}/source-lineage`);
  return {
    status: promoted.ok ? 'owner_pullable_experiment' : 'checkpoint_package',
    package: detail.json,
    adoption_initial: adoption,
    verified: verify.json,
    promoted: promoted.json,
    promote_status: promoted.status,
    promote_error: promoted.ok ? '' : promoted.text,
    target_lineage_start: lineageStart,
    target_lineage_end: lineageEnd.json,
    rollback_refs: promoted.json?.rollback_profile_json || promoted.json?.rollback_profile || null,
  };
}

function lanePrompt(lane, marker) {
  return [
    `Mission marker ${marker}, lane ${lane.id}: ${lane.name}.`,
    'Run this as Choir-in-Choir candidate computer work, not a platform-default merge.',
    'Foreground super must request a worker VM and delegate a vsuper candidate-world run.',
    'VSuper should coordinate at most two co-super children: one implementation worker and one verifier.',
    'The implementation should make the smallest real source change that expresses this experiment in Choir, commit it in the candidate checkout, and publish exactly one AppChangePackage.',
    'Use publish_app_change_package, not export_patchset or /api/promotions.',
    `Set app_id to ${lane.appID}, visibility unlisted, and include marker ${marker} in the package summary, provenance, or source delta.`,
    'Do not push to GitHub, do not mutate the active computer directly, and do not claim platform promotion.',
    'Return package id, trace id, verification evidence, rollback refs, residual risks, and a promotion recommendation.',
    lane.objective,
  ].join(' ');
}

test('Wave 1 launches Chiron and animation lanes and records package/adoption evidence', async ({ browser }, testInfo) => {
  const marker = `alt-portfolio-wave1-${Date.now()}`;
  const lanes = [
    {
      id: 'chiron',
      name: 'Chiron Shelf observability',
      appID: `portfolio-chiron-shelf-${safeID(marker)}`,
      objective: 'Experiment goal: add a product-event-backed Chiron stream above the Shelf that displays live prompt/tool/run status text without blocking the Desk button, app buttons, or prompt input; hide or quiet it while the prompt input is focused; do not use fake random ticker text.',
    },
    {
      id: 'animation',
      name: 'Process/window/agent animation language',
      appID: `portfolio-animation-language-${safeID(marker)}`,
      objective: 'Experiment goal: add a tasteful state-motion vocabulary for boot/wake, app launch, window raise/minimize/restore, live sync, and candidate/worker status; include reduced-motion handling and do not add decorative shimmer detached from real state.',
    },
  ];

  const source = await newRegisteredDesktop(browser, 'alt-portfolio-wave1-source');
  const recipient = await newRegisteredDesktop(browser, 'alt-portfolio-wave1-recipient');
  const forbiddenRequests = [];
  for (const page of [source.page, recipient.page]) {
    page.on('request', (request) => {
      const url = new URL(request.url());
      if (
        url.pathname.startsWith('/internal/') ||
        url.pathname.startsWith('/api/agent/') ||
        url.pathname.startsWith('/api/prompts') ||
        url.pathname.startsWith('/api/test/') ||
        url.pathname === '/api/events' ||
        url.pathname.startsWith('/api/promotions')
      ) {
        forbiddenRequests.push(`${request.method()} ${url.pathname}`);
      }
    });
  }

  try {
    const health = await requireFetchJSON(source.page, '/health');
    const submissions = {};
    await Promise.all(lanes.map(async (lane) => {
      const submitted = await submitPromptBar(source.page, lanePrompt(lane, marker));
      lane.submission_id = submitted.submission_id;
      submissions[lane.id] = submitted;
    }));

    const packagesResult = await waitForLanePackages(source.page, lanes, PACKAGE_WAIT_MS);
    const laneReports = {};
    for (const lane of lanes) {
      const prompt = await promptStatus(source.page, lane.submission_id);
      const trace = await traceSnapshot(source.page, lane.submission_id);
      const matches = packagesResult.byLane[lane.id] || [];
      const pkg = matches[0] || null;
      if (pkg) {
        const adoption = await adoptPackageIntoRecipient(recipient.page, lane, pkg, marker);
        laneReports[lane.id] = {
          status: adoption.status,
          lane,
          prompt,
          trace_mobile_summary: trace.mobile_summary || null,
          package_candidates: matches.map((candidate) => ({
            package_id: candidate.package_id,
            app_id: candidate.app_id,
            status: candidate.status,
            visibility: candidate.visibility,
            trace_id: candidate.trace_id,
            package_manifest_sha256: candidate.package_manifest_sha256,
          })),
          owner_recipient_adoption: adoption,
          recommendation: adoption.status === 'owner_pullable_experiment'
            ? 'iterate: package crossed into an owner-review recipient computer with build/adoption evidence'
            : 'checkpoint: inspect verifier blocker before promotion',
        };
      } else {
        laneReports[lane.id] = {
          status: 'blocked_incomplete',
          lane,
          prompt,
          trace_mobile_summary: trace.mobile_summary || null,
          package_candidates: [],
          blocker: `No matching AppChangePackage for ${lane.appID} after ${PACKAGE_WAIT_MS}ms.`,
          recommendation: 'root-cause super/vsuper worker package publication before retrying this lane',
        };
      }
    }

    const report = {
      status: Object.values(laneReports).every((lane) => lane.status === 'owner_pullable_experiment')
        ? 'checkpoint_wave1_owner_pullable'
        : 'checkpoint_incomplete',
      marker,
      base_url: BASE_URL,
      deployed_commit: health.build?.deployed_commit || health.build?.commit || '',
      source_account_email: source.email,
      source_account_user_id: source.session.user?.id,
      recipient_account_email: recipient.email,
      recipient_account_user_id: recipient.session.user?.id,
      submissions,
      lane_reports: laneReports,
      forbidden_browser_requests: forbiddenRequests,
      learning_log: [{
        timestamp: new Date().toISOString(),
        lane_or_substrate: 'Wave 1',
        observed_situation: 'Two experiment objectives were submitted through the product prompt-bar from one source computer and monitored for AppChangePackage publication.',
        mission_gradient_pressure: 'Push toward concurrent Choir-in-Choir package evidence without claiming success from prompt submission alone.',
        decision_taken: 'Treat each lane as owner_pullable only after a second account can inspect and adopt the package; otherwise record checkpoint/blocker evidence.',
        evidence_produced: 'Prompt submissions, Trace snapshots, package list, optional recipient adoption/verify/promote records, VText report.',
        cost_risk: 'Concurrent prompt submissions may serialize behind persistent super or reveal worker/vsuper publication gaps.',
        learning: 'Concurrency is evidence-bearing only if packages stay attributable to their lane markers.',
        possible_future_skill_simplification: 'Keep package identity and recipient adoption as the lane terminal proof, not direct loginability.',
      }],
    };

    const vtext = await createVTextReport(source.page, 'Alternate Computer Portfolio Wave 1 Evidence', 'wave1', [
      '# Alternate Computer Portfolio Wave 1 Evidence',
      '',
      `Status: ${report.status}`,
      `Marker: ${marker}`,
      `Deployed commit: ${report.deployed_commit}`,
      `Source account: ${source.email}`,
      `Recipient account: ${recipient.email}`,
      '',
      'Lane summaries:',
      ...lanes.flatMap((lane) => {
        const laneReport = laneReports[lane.id];
        return [
          '',
          `## ${lane.name}`,
          `Status: ${laneReport.status}`,
          `Prompt submission: ${lane.submission_id}`,
          `Packages: ${(laneReport.package_candidates || []).map((pkg) => pkg.package_id).join(', ') || 'none'}`,
          `Recommendation: ${laneReport.recommendation}`,
          laneReport.blocker ? `Blocker: ${laneReport.blocker}` : '',
        ].filter(Boolean);
      }),
    ].join('\n'));
    report.vtext = vtext;

    await writeEvidence(testInfo, 'alternate-portfolio-wave1-evidence.json', report);
    const screenshot = testInfo.outputPath('alternate-portfolio-wave1-source-desktop.png');
    await source.page.screenshot({ path: screenshot, fullPage: false });
    await testInfo.attach('alternate-portfolio-wave1-source-desktop', { path: screenshot, contentType: 'image/png' });

    expect(forbiddenRequests).toHaveLength(0);
    expect(Object.values(laneReports).some((lane) => lane.status !== 'blocked_incomplete')).toBe(true);
  } finally {
    await source.close();
    await recipient.close();
  }
});
