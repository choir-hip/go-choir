// demo_selfdev_prompt_bar.mjs — the owner-visible self-development demo on a
// test computer (made by scripts/qa_test_computer.mjs).
//
// It types a request into the desktop prompt bar, then watches until the work
// ends, saving evidence as it happens:
//   - a screenshot of the live desktop at each step and every new revision;
//   - every Texture revision as markdown plus a rendered screenshot, so the
//     versions before the final one are kept;
//   - every self-development candidate state, found from the document's
//     engineering assignments (a freeze opens the candidate keyed by them);
//   - with --approve, the owner approval, the applied state, and the desktop
//     after the computer serves the change.
//
//   node scripts/demo_selfdev_prompt_bar.mjs --label demo-computer-2026-10-10 \
//     --name minesweeper --prompt "Make a Minesweeper game" [--approve] [--hours 3]
import { createRequire } from 'node:module';
import { appendFileSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { approveOperation, requireKnownPolicyDigest, sha256hex } from './lib/selfdev_approval.mjs';

const requireFrontend = createRequire(new URL('../frontend/package.json', import.meta.url));
const { chromium } = requireFrontend('@playwright/test');

function arg(name, fallback) {
  const index = process.argv.indexOf(`--${name}`);
  if (index < 0) return fallback;
  const value = process.argv[index + 1];
  return value && !value.startsWith('--') ? value : true;
}

const label = arg('label');
const name = arg('name', 'demo');
const prompt = arg('prompt');
const approve = arg('approve', false) === true;
const hours = Number(arg('hours', 3));
const pollMs = Number(arg('poll-seconds', 20)) * 1000;
if (!label || !prompt) throw new Error('--label and --prompt are required');

const credential = JSON.parse(readFileSync(join(homedir(), '.config', 'choir-qa', 'test-computers', `${label}.json`), 'utf8'));
const storagePath = join(homedir(), '.config', 'choir-qa', 'test-computers', `${label}.storage.json`);
const BASE_URL = credential.base_url;
const computerID = credential.computer_id;
const ownerID = credential.user_id;
const started = new Date();
const marker = `DEMO_${name.toUpperCase()}_${started.getTime()}`;
const outDir = arg('out', join(process.cwd(), 'demo-output', `${name}-${started.toISOString().replace(/[:.]/g, '-')}`));
mkdirSync(join(outDir, 'revisions'), { recursive: true });
mkdirSync(join(outDir, 'screens'), { recursive: true });

const manifest = { name, prompt, label, computer_id: computerID, started_at: started.toISOString(), base_url: BASE_URL, approve, steps: [], revisions: [], operations: {} };
let shot = 0;
function log(step, extra) {
  const line = `${new Date().toISOString()} ${step}${extra ? ' ' + JSON.stringify(extra).slice(0, 400) : ''}`;
  appendFileSync(join(outDir, 'log.txt'), line + '\n');
  console.log(line);
}
function save() {
  writeFileSync(join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2));
}

function escapeHTML(text) {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
function inline(text) {
  return escapeHTML(text).replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>').replace(/\*([^*]+)\*/g, '<em>$1</em>');
}
// A small markdown renderer: headings, bullet and numbered lists, fenced
// code, paragraphs. Enough to read a Texture revision in a screenshot.
function renderMarkdown(markdown) {
  const out = [];
  let list = null;
  let fence = null;
  const closeList = () => { if (list) { out.push(`</${list}>`); list = null; } };
  for (const line of (markdown || '').split('\n')) {
    if (line.startsWith('```')) {
      if (fence) { out.push(`<pre><code>${escapeHTML(fence.join('\n'))}</code></pre>`); fence = null; } else { closeList(); fence = []; }
      continue;
    }
    if (fence) { fence.push(line); continue; }
    const heading = line.match(/^(#{1,6})\s+(.*)$/);
    const bullet = line.match(/^\s*[-*]\s+(.*)$/);
    const numbered = line.match(/^\s*\d+[.)]\s+(.*)$/);
    if (heading) { closeList(); out.push(`<h${heading[1].length}>${inline(heading[2])}</h${heading[1].length}>`); }
    else if (bullet || numbered) {
      const kind = bullet ? 'ul' : 'ol';
      if (list !== kind) { closeList(); out.push(`<${kind}>`); list = kind; }
      out.push(`<li>${inline((bullet || numbered)[1])}</li>`);
    } else if (!line.trim()) closeList();
    else { closeList(); out.push(`<p>${inline(line)}</p>`); }
  }
  closeList();
  if (fence) out.push(`<pre><code>${escapeHTML(fence.join('\n'))}</code></pre>`);
  return out.join('\n');
}

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ storageState: storagePath, viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
const renderPage = await context.newPage();

async function api(method, path, body) {
  const attempt = () => page.evaluate(async ({ m, p, b }) => {
    const response = await fetch(p, { method: m, credentials: 'same-origin', headers: b ? { 'content-type': 'application/json' } : {}, body: b ? JSON.stringify(b) : undefined });
    const text = await response.text();
    try { return { status: response.status, json: JSON.parse(text) }; } catch { return { status: response.status, text }; }
  }, { m: method, p: path, b: body });
  const first = await attempt();
  if (first.status !== 401) return first;
  await page.evaluate(() => fetch('/auth/session', { credentials: 'same-origin' }));
  return attempt();
}
const getJSON = (path) => api('GET', path);
const postJSON = (path, body) => api('POST', path, body);

async function screenshot(step, extra) {
  shot += 1;
  const file = `screens/${String(shot).padStart(3, '0')}-${step}.png`;
  await page.screenshot({ path: join(outDir, file), fullPage: false }).catch((error) => log('screenshot_failed', { step, error: String(error) }));
  manifest.steps.push({ at: new Date().toISOString(), step, screenshot: file, ...(extra || {}) });
  save();
  return file;
}

async function saveRevision(docID, title, revision, index) {
  const full = await getJSON(`/api/texture/revisions/${encodeURIComponent(revision.revision_id)}`);
  const body = full.json ?? revision;
  const content = body.content ?? revision.content ?? '';
  const base = `revisions/${String(index).padStart(2, '0')}-${(body.author_kind || revision.author_kind || 'unknown')}-${revision.revision_id.slice(0, 8)}`;
  writeFileSync(join(outDir, `${base}.md`), content);
  writeFileSync(join(outDir, `${base}.json`), JSON.stringify(body, null, 2));
  const html = `<!doctype html><html><head><meta charset="utf-8"><style>
    body{font:16px/1.55 -apple-system,system-ui,sans-serif;max-width:860px;margin:32px auto;padding:0 24px;color:#1d1d1f;background:#fff}
    header{border-bottom:1px solid #ddd;margin-bottom:16px;padding-bottom:8px;color:#555;font-size:13px}
    h1,h2,h3{line-height:1.25} pre{background:#f5f5f7;padding:12px;overflow:auto;font-size:13px} code{background:#f5f5f7;padding:1px 4px}
    </style></head><body><header>Texture revision ${index} · ${escapeHTML(title || docID)} · ${escapeHTML(body.author_kind || '')} · ${escapeHTML(body.created_at || '')}</header>${renderMarkdown(content)}</body></html>`;
  await renderPage.setContent(html);
  await renderPage.screenshot({ path: join(outDir, `${base}.png`), fullPage: true });
  manifest.revisions.push({ doc_id: docID, index, revision_id: revision.revision_id, author_kind: body.author_kind, created_at: body.created_at, markdown: `${base}.md`, screenshot: `${base}.png` });
  save();
}

function candidateOperationID(assignmentID) {
  return 'selfdev-' + sha256hex(`${computerID}\u0000${assignmentID}`).slice(0, 32);
}

try {
  requireKnownPolicyDigest();
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  await page.waitForSelector('[data-prompt-input]', { timeout: 300_000 });
  await screenshot('desktop-before');
  const knownDocs = new Set(((await getJSON('/api/texture/documents')).json?.documents || []).map((doc) => doc.doc_id));

  await page.click('[data-prompt-input]');
  await page.fill('[data-prompt-input]', prompt);
  await screenshot('prompt-typed');
  await page.keyboard.press('Enter');
  log('prompt_submitted', { prompt });
  await page.waitForTimeout(8_000);
  await screenshot('prompt-submitted');

  const seenRevisions = new Map();
  const operationStates = {};
  const deadline = Date.now() + hours * 3600_000;
  let approved = false;
  let finished = null;
  while (Date.now() < deadline && !finished) {
    const docs = ((await getJSON('/api/texture/documents')).json?.documents || []).filter((doc) => !knownDocs.has(doc.doc_id));
    for (const doc of docs) {
      const revisions = ((await getJSON(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/revisions`)).json?.revisions || [])
        .slice().sort((a, b) => String(a.created_at).localeCompare(String(b.created_at)));
      const seen = seenRevisions.get(doc.doc_id) || new Set();
      let fresh = false;
      for (const revision of revisions) {
        if (seen.has(revision.revision_id)) continue;
        seen.add(revision.revision_id);
        fresh = true;
        await saveRevision(doc.doc_id, doc.title, revision, seen.size);
        log('revision', { doc: doc.doc_id, index: seen.size, author: revision.author_kind });
      }
      seenRevisions.set(doc.doc_id, seen);
      if (fresh) await screenshot(`desktop-after-revision-${doc.doc_id.slice(0, 8)}-${seen.size}`);

      const diagnosis = await getJSON(`/api/texture/documents/${encodeURIComponent(doc.doc_id)}/diagnosis?include_content=false`);
      const assignments = [...new Set((JSON.stringify(diagnosis.json || {}).match(/assignment-[0-9a-f-]{36}/g) || []))];
      for (const assignmentID of assignments) {
        const operationID = candidateOperationID(assignmentID);
        const operation = await getJSON(`/api/computers/${encodeURIComponent(computerID)}/self-development/operations/${encodeURIComponent(operationID)}`);
        if (operation.status !== 200 || !operation.json?.state) continue;
        const state = operation.json.state;
        if (operationStates[operationID] !== state) {
          operationStates[operationID] = state;
          manifest.operations[operationID] = { assignment_id: assignmentID, doc_id: doc.doc_id, state, history: [...(manifest.operations[operationID]?.history || []), { at: new Date().toISOString(), state }] };
          writeFileSync(join(outDir, `operation-${operationID}.json`), JSON.stringify(operation.json, null, 2));
          log('operation', { operationID, state });
          await screenshot(`operation-${state}`);
        }
        if (state === 'awaiting_approval' && approve && !approved) {
          approved = true;
          const decision = await approveOperation({ marker, computerID, ownerID, operation: operation.json, postJSON, getJSON });
          manifest.approval = { at: new Date().toISOString(), status: decision.status };
          log('approved', { operationID });
          await screenshot('approved');
        }
        if (state === 'applied') {
          await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
          await page.waitForSelector('[data-prompt-input]', { timeout: 300_000 }).catch(() => {});
          await page.waitForTimeout(5_000);
          await screenshot('desktop-after-apply');
          finished = { state, operationID };
        }
        if (['failed', 'rejected', 'cancelled', 'rolled_back'].includes(state)) finished = { state, operationID };
        if (state === 'awaiting_approval' && !approve) finished = { state, operationID };
      }
    }
    if (!finished) await page.waitForTimeout(pollMs);
  }
  manifest.finished = finished || { state: 'timeout' };
  manifest.ended_at = new Date().toISOString();
  await screenshot('desktop-final');
  save();
  log('done', manifest.finished);
} finally {
  save();
  await browser.close();
}
