import fs from 'node:fs/promises';
import path from 'node:path';
import { test, expect } from './helpers/fixtures.js';
import { getSession, registerPasskey } from './helpers/auth.js';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'https://draft.choir-ip.com';
const PORTFOLIO_WAVE = process.env.GO_CHOIR_ALT_PORTFOLIO_WAVE
  || (process.env.GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE2 === '1' ? '2' : '1');
const RUN_PORTFOLIO_WAVE = process.env.GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE1 === '1'
  || process.env.GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE2 === '1';
const PACKAGE_WAIT_MS = Number(process.env.GO_CHOIR_ALT_PORTFOLIO_PACKAGE_WAIT_MS
  || process.env.GO_CHOIR_ALT_PORTFOLIO_WAVE1_PACKAGE_WAIT_MS
  || 40 * 60 * 1000);

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(Math.max(PACKAGE_WAIT_MS + 20 * 60 * 1000, 60 * 60 * 1000));
test.skip(!RUN_PORTFOLIO_WAVE, 'set GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE1=1 or GO_CHOIR_RUN_ALT_PORTFOLIO_WAVE2=1 to run deployed portfolio proof');

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

async function synthesizeRunAcceptance(page, lane, marker, wave) {
  const res = await postJSON(page, '/api/run-acceptances/synthesize', {
    target_mission_id: `mission-alternate-computer-ux-experiment-portfolio-v0-wave${wave}-${lane.id}`,
    source_prompt_or_objective: [
      `Wave ${wave} ${lane.name} portfolio lane.`,
      `Marker: ${marker}.`,
      lane.objective,
    ].join(' '),
    trajectory_id: lane.submission_id,
    staging_url: new URL(BASE_URL).origin,
  });
  if (res.ok) {
    return res.json;
  }
  return {
    state: 'blocked',
    acceptance_level: 'docs-level',
    synthesize_status: res.status,
    synthesize_error: res.text,
  };
}

function tryParseJSON(value) {
  if (typeof value !== 'string' || value.trim() === '') {
    return null;
  }
  try {
    return JSON.parse(value);
  } catch (_err) {
    return null;
  }
}

function summarizeTracePayload(payload) {
  const parsedPayload = typeof payload === 'string' ? tryParseJSON(payload) : payload;
  if (!parsedPayload || typeof parsedPayload !== 'object') {
    return { raw_preview: String(payload || '').slice(0, 2000) };
  }

  const summarized = {};
  for (const key of ['tool', 'call_id', 'is_error', 'output_len', 'phase', 'status', 'chained_from']) {
    if (Object.prototype.hasOwnProperty.call(parsedPayload, key)) {
      summarized[key] = parsedPayload[key];
    }
  }
  if (parsedPayload.arguments) {
    summarized.arguments = parsedPayload.arguments;
  }

  if (typeof parsedPayload.output === 'string') {
    summarized.output_preview = parsedPayload.output.slice(0, 4000);
    const output = tryParseJSON(parsedPayload.output);
    if (output && typeof output === 'object') {
      summarized.output_json = {};
      for (const key of [
        'status',
        'state',
        'loop_id',
        'agent_id',
        'profile',
        'completion_blocker',
        'terminal_error',
        'error',
        'worker_update_checkpoint',
        'worker_event_error',
        'worker_channel_message_count',
        'worker_spawned_profiles',
        'worker_child_run_ids',
        'worker_child_statuses',
        'worker_child_status_errors',
        'app_change_packages',
      ]) {
        if (Object.prototype.hasOwnProperty.call(output, key)) {
          summarized.output_json[key] = output[key];
        }
      }
      if (Array.isArray(output.worker_event_summary)) {
        summarized.output_json.worker_event_summary = output.worker_event_summary.slice(0, 40);
      }
      if (Object.prototype.hasOwnProperty.call(output, 'chained_delegation_output')) {
        summarized.output_json.chained_delegation_output = output.chained_delegation_output;
      }
    }
  }

  return summarized;
}

function detailContainsPackageSignal(detail) {
  const haystack = JSON.stringify(detail || {}).toLowerCase();
  return (
    haystack.includes('delegate_worker_vm') ||
    haystack.includes('request_worker_vm') ||
    haystack.includes('publish_app_change_package') ||
    haystack.includes('appchangepackage') ||
    haystack.includes('app_change_package') ||
    haystack.includes('worker_run') ||
    haystack.includes('completion_blocker')
  );
}

function summarizeTraceDetail(detail) {
  return {
    moment: detail?.moment || null,
    references: detail?.references || {},
    artifacts: {
      app_change_package_id: detail?.artifacts?.app_change_package?.package_id || '',
      app_adoption_id: detail?.artifacts?.app_adoption?.adoption_id || '',
      run_memory_entry_id: detail?.artifacts?.run_memory?.entry_id || '',
      continuation_id: detail?.artifacts?.continuation?.continuation_id || '',
    },
    events: Array.isArray(detail?.events) ? detail.events.map((event) => ({
      event_id: event.event_id,
      kind: event.kind,
      run_id: event.run_id,
      agent_id: event.agent_id,
      stream_seq: event.stream_seq,
      payload: summarizeTracePayload(event.payload),
    })) : [],
    messages: Array.isArray(detail?.messages) ? detail.messages.map((message) => ({
      channel_id: message.channel_id,
      seq: message.seq,
      sender_profile: message.sender_profile,
      sender_role: message.sender_role,
      content_preview: String(message.content || '').slice(0, 4000),
    })) : [],
  };
}

async function traceDiagnostics(page, trajectoryId) {
  const snapshot = await traceSnapshot(page, trajectoryId);
  if (snapshot.error || snapshot.status) {
    return { snapshot };
  }
  const moments = Array.isArray(snapshot.moments) ? snapshot.moments : [];
  const relevant = moments.filter((moment) => (
    detailContainsPackageSignal(moment) ||
    ['tool.invoked', 'tool.result', 'channel.message', 'run.completed', 'run.failed'].includes(moment.kind)
  ));

  const detailResults = [];
  for (const moment of relevant.slice(-40)) {
    const res = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}/moments/${encodeURIComponent(moment.moment_id)}`);
    if (res.ok) {
      const detail = summarizeTraceDetail(res.json);
      if (detailContainsPackageSignal(detail)) {
        detailResults.push(detail);
      }
    } else {
      detailResults.push({
        moment,
        detail_error: `${res.status} ${res.text}`,
      });
    }
  }

  return {
    trajectory: snapshot.trajectory || null,
    mobile_summary: snapshot.mobile_summary || null,
    agent_count: Array.isArray(snapshot.agents) ? snapshot.agents.length : 0,
    moment_count: moments.length,
    relevant_moment_count: relevant.length,
    details: detailResults,
  };
}

function findDelegateBlocker(traceDiag) {
  const details = Array.isArray(traceDiag?.details) ? traceDiag.details : [];
  const delegateResults = [];
  for (const detail of details) {
    for (const event of detail.events || []) {
      const payload = event.payload || {};
      if (payload.tool !== 'delegate_worker_vm' && payload.output_json?.chained_delegation_output == null) {
        continue;
      }
      const output = payload.output_json?.chained_delegation_output || payload.output_json || {};
      delegateResults.push(output);
    }
  }
  const blocked = delegateResults.find((output) => output.completion_blocker || output.terminal_error || output.status === 'worker_run_incomplete');
  if (blocked) {
    return [
      `delegate_worker_vm returned ${blocked.status || 'unknown status'}`,
      blocked.completion_blocker ? `completion_blocker=${blocked.completion_blocker}` : '',
      blocked.terminal_error ? `terminal_error=${blocked.terminal_error}` : '',
    ].filter(Boolean).join('; ');
  }
  const last = delegateResults[delegateResults.length - 1];
  if (last) {
    return `delegate_worker_vm returned ${last.status || 'unknown status'} with ${Array.isArray(last.app_change_packages) ? last.app_change_packages.length : 0} AppChangePackages`;
  }
  return '';
}

function packageMatchesLane(pkg, lane) {
  const haystack = JSON.stringify(pkg || {}).toLowerCase();
  return [lane.marker, lane.appID, lane.name]
    .filter(Boolean)
    .some((needle) => haystack.includes(String(needle).toLowerCase()));
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

  const pulled = await postJSON(page, '/api/app-change-packages/pull', {
    package_id: pkg.package_id,
    source_owner_id: pkg.owner_id || '',
  });
  if (!pulled.ok) {
    return {
      status: 'blocked_incomplete',
      blocker: `recipient could not pull package ${pkg.package_id}: ${pulled.status} ${pulled.text}`,
    };
  }

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
    'If an implementation child publishes an AppChangePackage, the vsuper parent must not publish a second package; return the child package id and treat duplicate package publication as a blocker.',
    'Use publish_app_change_package, not export_patchset or /api/promotions.',
    `Set app_id to ${lane.appID}, visibility unlisted, and include marker ${marker} in the package summary, provenance, or source delta.`,
    'Do not push to GitHub, do not mutate the active computer directly, and do not claim platform promotion.',
    'Return package id, trace id, verification evidence, rollback refs, residual risks, and a promotion recommendation.',
    lane.objective,
  ].join(' ');
}

function portfolioLanes(wave, marker) {
  if (String(wave) === '2') {
    return [
      {
        id: 'liquid',
        name: 'Choir Liquid Material Engine',
        appID: `portfolio-liquid-material-${safeID(marker)}`,
        objective: 'Experiment goal: build a custom WebGL-first shell material prototype for Choir with one renderer context, owned synthetic material fields, registered shell surfaces, DOM controls/text above the material, reduced transparency/reduced motion fallback, and product-safe benchmarks for WebGL context count, frame-time/resource cost, and heavy-window behavior. Do not capture private app DOM into GPU textures and do not use liquid-dom as mobile Safari proof.',
      },
      {
        id: 'python',
        name: 'Python code mode A/B',
        appID: `portfolio-python-code-mode-${safeID(marker)}`,
        objective: 'Experiment goal: create a candidate super/vsuper/co-super profile family that replaces bash with a minimal Python execution primitive for the candidate profile family, then benchmark against the existing bash family on token use, tool-loop iterations, wall-clock time, tool execution time, traceability, changed files, and foreground-mutation safety. Python must replace bash in this candidate family rather than being added beside bash.',
      },
    ];
  }

  return [
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
}

test(`Wave ${PORTFOLIO_WAVE} launches portfolio lanes and records package/adoption evidence`, async ({ browser }, testInfo) => {
  const wave = safeID(PORTFOLIO_WAVE);
  const marker = `alt-portfolio-wave${wave}-${Date.now()}`;
  const lanes = portfolioLanes(wave, marker);

  const source = await newRegisteredDesktop(browser, `alt-portfolio-wave${wave}-source`);
  const recipient = await newRegisteredDesktop(browser, `alt-portfolio-wave${wave}-recipient`);
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
      const diagnostics = await traceDiagnostics(source.page, lane.submission_id);
      const matches = packagesResult.byLane[lane.id] || [];
      const pkg = matches[0] || null;
      const runAcceptance = await synthesizeRunAcceptance(source.page, lane, marker, wave);
      if (pkg) {
        const adoption = await adoptPackageIntoRecipient(recipient.page, lane, pkg, marker);
        laneReports[lane.id] = {
          status: adoption.status,
          lane,
          prompt,
          trace_mobile_summary: trace.mobile_summary || null,
          package_candidates: matches.map((candidate) => ({
            package_id: candidate.package_id,
            owner_id: candidate.owner_id,
            app_id: candidate.app_id,
            status: candidate.status,
            visibility: candidate.visibility,
            trace_id: candidate.trace_id,
            package_manifest_sha256: candidate.package_manifest_sha256,
          })),
          owner_recipient_adoption: adoption,
          run_acceptance: runAcceptance,
          trace_diagnostics: diagnostics,
          recommendation: adoption.status === 'owner_pullable_experiment'
            ? 'iterate: package crossed into an owner-review recipient computer with build/adoption evidence'
            : 'checkpoint: inspect verifier blocker before promotion',
        };
      } else {
        const delegateBlocker = findDelegateBlocker(diagnostics);
        laneReports[lane.id] = {
          status: 'blocked_incomplete',
          lane,
          prompt,
          trace_mobile_summary: trace.mobile_summary || null,
          trace_diagnostics: diagnostics,
          run_acceptance: runAcceptance,
          package_candidates: [],
          blocker: delegateBlocker || `No matching AppChangePackage for ${lane.appID} after ${PACKAGE_WAIT_MS}ms.`,
          recommendation: 'root-cause super/vsuper worker package publication before retrying this lane',
        };
      }
    }

    const report = {
      status: Object.values(laneReports).every((lane) => lane.status === 'owner_pullable_experiment')
        ? `checkpoint_wave${wave}_owner_pullable`
        : 'checkpoint_incomplete',
      marker,
      wave,
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
        lane_or_substrate: `Wave ${wave}`,
        observed_situation: 'Two experiment objectives were submitted through the product prompt-bar from one source computer and monitored for AppChangePackage publication.',
        mission_gradient_pressure: 'Push toward concurrent Choir-in-Choir package evidence without claiming success from prompt submission alone.',
        decision_taken: 'Treat each lane as owner_pullable only after a second account can inspect and adopt the package; otherwise record checkpoint/blocker evidence.',
        evidence_produced: 'Prompt submissions, Trace snapshots, package list, optional recipient adoption/verify/promote records, VText report.',
        cost_risk: 'Concurrent prompt submissions may serialize behind persistent super or reveal worker/vsuper publication gaps.',
        learning: 'Concurrency is evidence-bearing only if packages stay attributable to their lane markers.',
        possible_future_skill_simplification: 'Keep package identity and recipient adoption as the lane terminal proof, not direct loginability.',
      }],
    };

    const vtext = await createVTextReport(source.page, `Alternate Computer Portfolio Wave ${wave} Evidence`, `wave${wave}`, [
      `# Alternate Computer Portfolio Wave ${wave} Evidence`,
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
          `Run acceptance: ${laneReport.run_acceptance?.acceptance_id || 'none'} ${laneReport.run_acceptance?.acceptance_level || ''} ${laneReport.run_acceptance?.state || ''}`.trim(),
          `Recommendation: ${laneReport.recommendation}`,
          laneReport.blocker ? `Blocker: ${laneReport.blocker}` : '',
        ].filter(Boolean);
      }),
    ].join('\n'));
    report.vtext = vtext;

    await writeEvidence(testInfo, `alternate-portfolio-wave${wave}-evidence.json`, report);
    const screenshot = testInfo.outputPath(`alternate-portfolio-wave${wave}-source-desktop.png`);
    await source.page.screenshot({ path: screenshot, fullPage: false });
    await testInfo.attach(`alternate-portfolio-wave${wave}-source-desktop`, { path: screenshot, contentType: 'image/png' });

    expect(forbiddenRequests).toHaveLength(0);
    expect(Object.values(laneReports).some((lane) => lane.status !== 'blocked_incomplete')).toBe(true);
  } finally {
    await source.close();
    await recipient.close();
  }
});
