import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { EventEmitter } from 'node:events';
import { mkdir, mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { get as httpGet } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { promisify } from 'node:util';

import {
  DefinitionParseError,
  buildDashboardModel,
  createDashboardServer,
  generateDashboard,
  isLoopbackHost,
  isReloadableDashboardScript,
  parseDefinitionSource,
  parseListenAddress,
  parseYamlSubset,
  renderDefinitionSource,
} from './dashboard.mjs';
import { renderDashboard } from './dashboard-view.mjs';
import { collectRepositoryMetadata } from './dashboard-git.mjs';
import { createSessionLog } from './dashboard-session.mjs';

const execFileAsync = promisify(execFile);

const actualMission = new URL(
  '../../../docs/definitions/choir-audited-autoputer-construction-2026-07-15.md',
  import.meta.url,
);

function definition({ title = 'Test mission', status = 'working', next = 'Inspect evidence' } = {}) {
  return `---
title: "${title.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"
definition_version: 2
start:
  captured_at: 2026-07-15T00:00:00Z
finish:
  deliver: "A verifiable outcome"
  acceptance:
    - action: "Inspect the product"
      proves: "The product works"
      evidence_class: product_path
now:
  status: ${status}
  evidence_refs: [receipt-one]
  next_action: "${next.replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"
orchestration:
  decision_gates:
    - id: G1
      adjudication: [accept, repair, reject, escalate]
successor:
  status: unauthorized
---

# Test mission
`;
}

function requestText(url) {
  return new Promise((resolve, reject) => {
    const request = httpGet(url, (response) => {
      response.setEncoding('utf8');
      let body = '';
      response.on('data', (chunk) => {
        body += chunk;
      });
      response.on('end', () => resolve({ status: response.statusCode, headers: response.headers, body }));
    });
    request.on('error', reject);
  });
}

async function eventually(probe, { timeout = 4000, interval = 30 } = {}) {
  const deadline = Date.now() + timeout;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const value = await probe();
      if (value) return value;
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, interval));
  }
  throw lastError ?? new Error('condition was not met before timeout');
}

test('parses the actual Definition v2 mission into the dashboard shape', async () => {
  const source = await readFile(actualMission, 'utf8');
  const parsed = parseDefinitionSource(source);

  assert.equal(parsed.definition_version, 2);
  assert.equal(typeof parsed.title, 'string');
  assert.equal(typeof parsed.start.source.canonical_ref, 'string');
  assert.ok(Array.isArray(parsed.start.observed_artifact));
  assert.ok(parsed.finish.acceptance.length >= 1);
  assert.equal(typeof parsed.finish.acceptance[0].proves, 'string');
  assert.ok(Array.isArray(parsed.orchestration.decision_gates));
  assert.equal(typeof parsed.now.status, 'string');
  assert.ok(Array.isArray(parsed.now.evidence_refs));
  assert.equal(typeof parsed.now.next_action, 'string');
});

test('collects branch, worktree, upstream, dirty-file, and LOC metadata without fetching', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-git-'));
  const originPath = join(directory, 'origin.git');
  const worktreePath = join(directory, 'worktree');
  const linkedPath = join(directory, 'linked');
  const git = (cwd, ...argumentsList) =>
    execFileAsync('git', ['-C', cwd, ...argumentsList], { encoding: 'utf8' });
  t.after(() => rm(directory, { recursive: true, force: true }));

  await git(directory, 'init', '--bare', originPath);
  await git(directory, 'init', '-b', 'main', worktreePath);
  await git(worktreePath, 'config', 'user.name', 'Definition Dashboard Test');
  await git(worktreePath, 'config', 'user.email', 'dashboard@example.test');
  await writeFile(join(worktreePath, 'mission.md'), definition());
  await writeFile(join(worktreePath, 'tracked.txt'), 'alpha\n');
  await git(worktreePath, 'add', 'mission.md', 'tracked.txt');
  await git(worktreePath, 'commit', '-m', 'baseline');
  await git(worktreePath, 'remote', 'add', 'origin', originPath);
  await git(worktreePath, 'push', '-u', 'origin', 'main');
  await writeFile(join(worktreePath, 'ahead.txt'), 'ahead\n');
  await git(worktreePath, 'add', 'ahead.txt');
  await git(worktreePath, 'commit', '-m', 'ahead');

  await writeFile(join(worktreePath, 'tracked.txt'), 'alpha\nbeta\n');
  await writeFile(join(worktreePath, 'new.txt'), 'one\ntwo\n');
  await writeFile(join(worktreePath, 'binary.bin'), Buffer.from([0, 1, 2]));
  const metadata = await collectRepositoryMetadata(join(worktreePath, 'mission.md'));

  assert.equal(metadata.available, true);
  assert.equal(metadata.branch, 'main');
  assert.equal(metadata.detached, false);
  assert.match(metadata.head, /^[a-f0-9]{12}$/);
  assert.equal(metadata.worktreePath, await realpath(worktreePath));
  assert.equal(metadata.worktreeKind, 'primary');
  assert.equal(metadata.upstream, 'origin/main');
  assert.match(metadata.upstreamHead, /^[a-f0-9]{12}$/);
  assert.equal(metadata.ahead, 1);
  assert.equal(metadata.behind, 0);
  assert.equal(metadata.dirtyFiles, 3);
  assert.equal(metadata.changedFiles.length, 3);
  assert.deepEqual(
    metadata.changedFiles.map(({ lastModifiedAt, ...rest }) => rest),
    [
      { path: 'binary.bin', state: 'untracked', code: '??', addedLines: null, deletedLines: null, binary: true },
      { path: 'new.txt', state: 'untracked', code: '??', addedLines: 2, deletedLines: 0, binary: false },
      { path: 'tracked.txt', state: 'unstaged', code: '.M', addedLines: 1, deletedLines: 0, binary: false },
    ],
  );
  for (const file of metadata.changedFiles) {
    assert.match(file.lastModifiedAt, /^\d{4}-\d{2}-\d{2}T/);
  }
  assert.equal(metadata.addedLines, 3);
  assert.equal(metadata.deletedLines, 0);
  assert.equal(metadata.binaryFiles, 1);
  assert.equal(metadata.unreadableFiles, 0);
  assert.match(metadata.fingerprint, /^[a-f0-9]{64}$/);

  await git(worktreePath, 'worktree', 'add', '-b', 'feature-dashboard', linkedPath);
  const linked = await collectRepositoryMetadata(join(linkedPath, 'mission.md'));
  assert.equal(linked.available, true);
  assert.equal(linked.branch, 'feature-dashboard');
  assert.equal(linked.worktreePath, await realpath(linkedPath));
  assert.equal(linked.worktreeKind, 'linked');
  assert.equal(linked.upstream, null);
  assert.equal(linked.ahead, null);
  assert.equal(linked.behind, null);
});

test('session log keeps first-seen dirty timestamps and newest-first events', () => {
  const session = createSessionLog({ startedAt: '2026-07-16T00:00:00.000Z' });
  session.record({ kind: 'started', summary: 'Dashboard session started', at: '2026-07-16T00:00:00.000Z' });
  const first = session.observeRepository({
    available: true,
    fingerprint: 'one',
    changedFiles: [
      { path: 'a.txt', state: 'unstaged', lastModifiedAt: '2026-07-16T00:01:00.000Z' },
    ],
  }, '2026-07-16T00:02:00.000Z');
  assert.equal(first.changedFiles[0].firstSeenAt, '2026-07-16T00:02:00.000Z');
  const second = session.observeRepository({
    available: true,
    fingerprint: 'two',
    changedFiles: [
      { path: 'a.txt', state: 'unstaged', lastModifiedAt: '2026-07-16T00:05:00.000Z' },
      { path: 'b.txt', state: 'untracked', lastModifiedAt: '2026-07-16T00:04:00.000Z' },
    ],
  }, '2026-07-16T00:06:00.000Z');
  assert.equal(second.changedFiles.find((file) => file.path === 'a.txt').firstSeenAt, '2026-07-16T00:02:00.000Z');
  assert.equal(second.changedFiles.find((file) => file.path === 'b.txt').firstSeenAt, '2026-07-16T00:06:00.000Z');
  session.record({ kind: 'current', summary: 'Became current · repository', at: '2026-07-16T00:06:01.000Z' });
  const snap = session.snapshot();
  assert.equal(snap.events[0].summary, 'Became current · repository');
  assert.equal(snap.dirtyFiles.length, 2);
});

test('footer session log shows recent events and collapses the rest', () => {
  const events = Array.from({ length: 7 }, (_, index) => ({
    at: `2026-07-16T00:00:0${index}.000Z`,
    kind: 'current',
    summary: `Event ${index}`,
    detail: null,
  })).reverse();
  const html = renderDashboard({
    title: 'Session mission',
    source: { path: 'mission.md', digest: 'abc123' },
    session: {
      startedAt: '2026-07-16T00:00:00.000Z',
      eventCount: 7,
      recentCount: 5,
      events,
      recentEvents: events.slice(0, 5),
      earlierEvents: events.slice(5),
      dirtyFiles: [
        {
          path: 'skills/definition/scripts/dashboard-view.mjs',
          state: 'unstaged',
          firstSeenAt: '2026-07-16T00:00:01.000Z',
          lastModifiedAt: '2026-07-16T00:00:06.000Z',
        },
      ],
    },
    finish: { deliver: 'Outcome' },
    now: { status: 'working', next_action: 'Inspect' },
  });
  assert.match(html, /class="session-log"/);
  assert.match(html, /Session only/);
  assert.match(html, /Event 6/);
  assert.match(html, /2 earlier events · 1 dirty-file timestamps/);
  assert.match(html, /skills\/definition\/scripts\/dashboard-view\.mjs/);
  assert.match(html, /seen /);
  assert.match(html, /mtime /);
  assert.match(html, /<details class="session-more">/);
  assert.doesNotMatch(html, /<details class="session-more" open>/);
});

test('dashboard model exposes only canonical weak-signal measures as steering', () => {
  const model = buildDashboardModel({
    definition_version: 2,
    finish: { deliver: 'Verified outcome' },
    start: {},
    now: {},
    measures: [
      { name: 'Canonical agreement', kind: 'weak_signal', cannot_prove: 'Acceptance' },
      { name: 'Internal latency', kind: 'telemetry', baseline: '12ms' },
    ],
    weak_measures: [{ name: 'Legacy steering signal', kind: 'weak_signal' }],
  });

  assert.deepEqual(model.weakMeasures.map((measure) => measure.name), [
    'Canonical agreement',
    'Legacy steering signal',
  ]);
  const html = renderDashboard(model);
  assert.match(html, /Canonical agreement/);
  assert.match(html, /Legacy steering signal/);
  assert.doesNotMatch(html, /Internal latency/);
});

test('parses supported scalars and comments without guessing', () => {
  const parsed = parseYamlSubset(`name: "quoted # value" # discarded comment
single: 'owner''s value'
plain: refs/heads/main@abc#digest
saying: say "hello"
flags: [true, false, null, 12, -2.5, "x,y", [one, two]]
empty:
`);
  assert.deepEqual(parsed, {
    name: 'quoted # value',
    single: "owner's value",
    plain: 'refs/heads/main@abc#digest',
    saying: 'say "hello"',
    flags: [true, false, null, 12, -2.5, 'x,y', ['one', 'two']],
    empty: null,
  });
});

test('refuses malformed or unsupported YAML constructs', () => {
  const invalid = [
    ['tabs', 'root:\n\tchild: value'],
    ['duplicate keys', 'root: one\nroot: two'],
    ['nested duplicate keys', 'root:\n  key: one\n  key: two'],
    ['block literal', 'root: |\n  content'],
    ['block fold', 'root: >-\n  content'],
    ['tag', 'root: !unsafe value'],
    ['anchor', 'root: &copy value'],
    ['alias', 'root: *copy'],
    ['odd indentation', 'root:\n child: value'],
    ['indentation jump', 'root:\n    child: value'],
    ['mixed collection', 'root:\n  key: value\n  - item'],
    ['scalar with nested block', 'root: value\n  child: value'],
    ['inline map', 'root: { key: value }'],
    ['unterminated array', 'root: [one, two'],
    ['trailing array comma', 'root: [one,]'],
    ['ambiguous leading zero', 'root: 007'],
    ['unsafe prototype key', '__proto__: value'],
  ];

  for (const [name, yaml] of invalid) {
    assert.throws(() => parseYamlSubset(yaml), DefinitionParseError, name);
  }
});

test('requires complete Definition v2 front matter', () => {
  assert.throws(() => parseDefinitionSource('definition_version: 2'), /front matter/);
  assert.throws(
    () => parseDefinitionSource('---\ndefinition_version: 1\nstart:\n  captured_at: now\nfinish:\n  deliver: result\nnow:\n  status: working\n---\n'),
    /definition_version must be 2/,
  );
  assert.throws(
    () => parseDefinitionSource('---\ndefinition_version: 2\nfinish:\n  deliver: x\nnow:\n  status: working\n---\n'),
    /start must be a map/,
  );
});

test('accepts only localhost and literal loopback addresses', () => {
  for (const host of ['localhost', 'LOCALHOST.', '127.0.0.1', '127.255.1.9', '::1', '0:0:0:0:0:0:0:1']) {
    assert.equal(isLoopbackHost(host), true, host);
  }
  for (const host of ['0.0.0.0', '192.168.1.2', '::', 'example.test', 'localhost.example.test']) {
    assert.equal(isLoopbackHost(host), false, host);
  }
  assert.deepEqual(parseListenAddress('127.0.0.1:8787'), { host: '127.0.0.1', port: 8787 });
  assert.deepEqual(parseListenAddress('[::1]:0'), { host: '::1', port: 0 });
  assert.throws(() => parseListenAddress('0.0.0.0:8787'), /restricted/);
  assert.throws(() => parseListenAddress('example.test:8787'), /restricted/);
  assert.throws(() => parseListenAddress('::1:8787'), /bracket IPv6/);
});

test('HTTP failures stay generic and do not disclose source paths', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-private-'));
  const sourcePath = join(directory, 'private-mission-name.md');
  const dashboard = await createDashboardServer({ sourcePath, host: '127.0.0.1', port: 0 });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  const response = await requestText(`${dashboard.url}/`);
  assert.equal(response.status, 503);
  assert.doesNotMatch(response.body, new RegExp(directory.replaceAll(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  assert.doesNotMatch(response.body, /private-mission-name/);
  assert.match(response.body, /Definition source could not be read/);
});

test('live server fails closed, never serves stale HTML, and recovers from source repair', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'Initial current title' }));
  const dashboard = await createDashboardServer({
    sourcePath,
    host: '127.0.0.1',
    port: 0,
    watch: true,
  });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  const initial = await requestText(`${dashboard.url}/`);
  assert.equal(initial.status, 200);
  assert.match(initial.body, /Initial current title/);
  assert.match(initial.body, /class="session-log"/);
  assert.match(initial.body, /Became current · initial render/);
  assert.equal(dashboard.getState().model.session.events[0].kind, 'current');
  const healthy = await requestText(`${dashboard.url}/healthz`);
  assert.equal(healthy.status, 200);
  assert.match(healthy.body, /dashboard current/);
  assert.match(healthy.body, /non-authoritative/);
  assert.match(healthy.body, /not evidence of mission completion/);

  await writeFile(sourcePath, '---\ndefinition_version: 2\nfinish: [broken\n---\n');
  assert.equal(await dashboard.refresh(), false);
  const unavailable = await eventually(async () => {
    const response = await requestText(`${dashboard.url}/`);
    return response.status === 503 ? response : null;
  });
  assert.doesNotMatch(unavailable.body, /Initial current title/);
  assert.match(unavailable.body, /Dashboard unavailable/);
  assert.match(unavailable.body, /new EventSource\('\/events'\)/);
  assert.match(unavailable.body, /addEventListener\('reload'/);
  assert.doesNotMatch(unavailable.body, /addEventListener\('unavailable'/);
  const unhealthy = await requestText(`${dashboard.url}/healthz`);
  assert.equal(unhealthy.status, 503);
  assert.match(unhealthy.body, /dashboard not current/);
  assert.match(unhealthy.body, /not evidence of mission completion/);

  await writeFile(sourcePath, definition({ title: 'Recovered title', next: 'Proceed safely' }));
  await dashboard.refresh();
  const recovered = await eventually(async () => {
    const response = await requestText(`${dashboard.url}/`);
    return response.status === 200 && response.body.includes('Recovered title') ? response : null;
  });
  assert.doesNotMatch(recovered.body, /Initial current title/);
  assert.equal(dashboard.getState().available, true);
});

test('live dashboard refreshes when repository metadata changes without a Definition edit', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-repository-refresh-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'Repository refresh mission' }));
  let repository = {
    available: true,
    fingerprint: 'repository-main',
    branch: 'main',
    head: '111111111111',
    worktreePath: directory,
    worktreeKind: 'primary',
    upstream: 'origin/main',
    upstreamHead: '111111111111',
    ahead: 0,
    behind: 0,
    dirtyFiles: 0,
    addedLines: 0,
    deletedLines: 0,
    binaryFiles: 0,
    unreadableFiles: 0,
  };
  const dashboard = await createDashboardServer({
    sourcePath,
    host: '127.0.0.1',
    port: 0,
    watch: true,
    repositoryPollInterval: 20,
    repositoryMetadataLoader: async () => repository,
  });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  const initial = await eventually(async () => {
    const response = await requestText(`${dashboard.url}/`);
    return response.status === 200 && response.body.includes('Branch main') ? response : null;
  });
  repository = {
    ...repository,
    fingerprint: 'repository-feature',
    branch: 'feature/dashboard',
    head: '222222222222',
    ahead: 2,
    dirtyFiles: 1,
    addedLines: 7,
  };

  const refreshed = await eventually(async () => {
    const response = await requestText(`${dashboard.url}/`);
    return response.status === 200 && response.body.includes('Branch feature/dashboard')
      ? response
      : null;
  });
  assert.match(refreshed.body, /2 ahead/);
  assert.match(refreshed.body, /1 uncommitted files/);
  assert.match(refreshed.body, /\+7 −0/);
});

test('live dashboard hot-reloads renderer scripts without process restart', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-script-reload-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'Before script reload' }));

  const scriptWatcher = new EventEmitter();
  scriptWatcher.close = () => {};
  let loadCount = 0;

  const dashboard = await createDashboardServer({
    sourcePath,
    host: '127.0.0.1',
    port: 0,
    watch: true,
    reloadScripts: true,
    repositoryPollInterval: 0,
    watcherFactory: () => {
      const watcher = new EventEmitter();
      watcher.close = () => {};
      return watcher;
    },
    scriptWatcherFactory: (_directory, listener) => {
      scriptWatcher.on('change', listener);
      return scriptWatcher;
    },
    scriptReloadDebounceMs: 20,
    loadScripts: async () => {
      loadCount += 1;
      return {
        renderer: (model) => `<!doctype html><title>reloaded-${loadCount}-${model.title}</title>`,
        repositoryMetadataLoader: async () => ({
          available: false,
          reason: 'unused',
          fingerprint: `script-reload-${loadCount}`,
        }),
      };
    },
  });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  assert.deepEqual(isReloadableDashboardScript('dashboard-view.mjs'), {
    kind: 'module',
    name: 'dashboard-view.mjs',
  });
  assert.equal(isReloadableDashboardScript('README.md').kind, 'ignored');

  const initial = await requestText(`${dashboard.url}/`);
  assert.equal(initial.status, 200);
  assert.match(initial.body, /Before script reload/);
  assert.doesNotMatch(initial.body, /reloaded-/);

  scriptWatcher.emit('change', 'change', 'dashboard-view.mjs');
  const reloaded = await eventually(async () => {
    const response = await requestText(`${dashboard.url}/`);
    return response.status === 200 && response.body.includes('reloaded-1-Before script reload')
      ? response
      : null;
  });
  assert.match(reloaded.body, /reloaded-1-Before script reload/);
  assert.equal(loadCount, 1);
});

test('queued refreshes invalidate immediately and only publish the latest revision', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-revision-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'Initial revision' }));
  const dashboard = await createDashboardServer({ sourcePath, host: '127.0.0.1', port: 0 });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  await writeFile(sourcePath, definition({ title: 'Latest revision' }));
  const supersededRefresh = dashboard.refresh();
  const latestRefresh = dashboard.refresh();
  assert.equal(dashboard.getState().available, false);
  assert.equal(await supersededRefresh, false);
  assert.equal(await latestRefresh, true);

  const current = await requestText(`${dashboard.url}/`);
  assert.equal(current.status, 200);
  assert.match(current.body, /Latest revision/);
  assert.doesNotMatch(current.body, /Initial revision/);
});

test('watcher failure permanently invalidates freshness until server restart', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-watch-failure-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'Current before watcher failure' }));
  const watcher = new EventEmitter();
  watcher.closed = false;
  watcher.close = () => {
    watcher.closed = true;
  };
  const dashboard = await createDashboardServer({
    sourcePath,
    host: '127.0.0.1',
    port: 0,
    watch: true,
    watcherFactory: (_directory, listener) => {
      watcher.on('change', listener);
      return watcher;
    },
  });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  await writeFile(sourcePath, definition({ title: 'Must not publish after watcher failure' }));
  const queuedRefresh = dashboard.refresh();
  watcher.emit('error', new Error('watch failed'));
  assert.equal(watcher.closed, true);
  assert.equal(dashboard.getState().available, false);
  assert.equal(await queuedRefresh, false);
  assert.equal(await dashboard.refresh(), false);
  assert.equal((await requestText(`${dashboard.url}/healthz`)).status, 503);
  assert.equal((await requestText(`${dashboard.url}/`)).status, 503);

  let eventRequest;
  const unavailableEvent = new Promise((resolve, reject) => {
    eventRequest = httpGet(`${dashboard.url}/events`, (response) => {
      let body = '';
      response.setEncoding('utf8');
      response.on('data', (chunk) => {
        body += chunk;
        if (body.includes('event: unavailable\n\n')) resolve();
      });
      response.on('error', reject);
    });
    eventRequest.on('error', reject);
  });
  t.after(() => eventRequest?.destroy());
  await unavailableEvent;
});

test('SSE emits unavailable on failure and reload with the repaired digest', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-sse-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'First generation' }));
  const dashboard = await createDashboardServer({ sourcePath, host: '127.0.0.1', port: 0 });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  let eventBody = '';
  let eventRequest;
  const eventStarted = new Promise((resolve, reject) => {
    eventRequest = httpGet(`${dashboard.url}/events`, (response) => {
      assert.equal(response.statusCode, 200);
      response.setEncoding('utf8');
      response.on('data', (chunk) => {
        eventBody += chunk;
        if (eventBody.includes('retry: 1000')) resolve();
      });
      response.on('error', reject);
    });
    eventRequest.on('error', reject);
  });
  t.after(() => eventRequest?.destroy());
  await eventStarted;

  await writeFile(sourcePath, '---\ndefinition_version: 2\nfinish: |\n  bad\n---\n');
  assert.equal(await dashboard.refresh(), false);
  await eventually(() => eventBody.includes('event: unavailable\n\n'));
  assert.doesNotMatch(eventBody, /event: reload/);
  assert.equal((await requestText(`${dashboard.url}/healthz`)).status, 503);

  const unavailable = await requestText(`${dashboard.url}/`);
  assert.equal(unavailable.status, 503);
  assert.match(unavailable.body, /new EventSource\('\/events'\)/);
  assert.match(unavailable.body, /addEventListener\('reload'/);

  await writeFile(sourcePath, definition({ title: 'Second generation' }));
  assert.equal(await dashboard.refresh(), true);
  const reloadEvent = `event: reload\ndata: ${dashboard.getState().digest}\n\n`;
  await eventually(() => eventBody.includes(reloadEvent));
  assert.ok(eventBody.indexOf(reloadEvent) > eventBody.indexOf('event: unavailable\n\n'));
});

test('source read failure is replayed to an event subscriber that connects after invalidation', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-read-failure-'));
  const sourcePath = join(directory, 'mission.md');
  await writeFile(sourcePath, definition({ title: 'Current before removal' }));
  const dashboard = await createDashboardServer({ sourcePath, host: '127.0.0.1', port: 0 });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  await rm(sourcePath);
  assert.equal(await dashboard.refresh(), false);
  assert.equal(dashboard.getState().available, false);

  let eventBody = '';
  let eventStatus;
  let eventRequest;
  const unavailableReceived = new Promise((resolve, reject) => {
    eventRequest = httpGet(`${dashboard.url}/events`, (response) => {
      eventStatus = response.statusCode;
      response.setEncoding('utf8');
      response.on('data', (chunk) => {
        eventBody += chunk;
        if (eventBody.includes('event: unavailable\n\n')) resolve();
      });
      response.on('error', reject);
    });
    eventRequest.on('error', reject);
  });
  t.after(() => eventRequest?.destroy());
  await Promise.race([
    unavailableReceived,
    new Promise((_, reject) =>
      setTimeout(() => reject(new Error('unavailable event timeout')), 2000),
    ),
  ]);

  assert.equal(eventStatus, 200);
  assert.match(eventBody, /retry: 1000\n\n[\s\S]*event: unavailable\n\n/);
  assert.doesNotMatch(eventBody, /event: reload/);

  await writeFile(sourcePath, definition({ title: 'Recovered after read failure' }));
  assert.equal(await dashboard.refresh(), true);
  const reloadEvent = `event: reload\ndata: ${dashboard.getState().digest}\n\n`;
  await eventually(() => eventBody.includes(reloadEvent));
  assert.ok(eventBody.indexOf(reloadEvent) > eventBody.indexOf('event: unavailable\n\n'));
});

test('explicit output write failure invalidates current state and emits unavailable', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-output-failure-'));
  const sourcePath = join(directory, 'mission.md');
  const outputPath = join(directory, 'snapshot.html');
  await writeFile(sourcePath, definition({ title: 'Current before output failure' }));
  const dashboard = await createDashboardServer({
    sourcePath,
    outputPath,
    host: '127.0.0.1',
    port: 0,
  });
  t.after(async () => {
    await dashboard.close();
    await rm(directory, { recursive: true, force: true });
  });

  let eventBody = '';
  let eventRequest;
  const eventStarted = new Promise((resolve, reject) => {
    eventRequest = httpGet(`${dashboard.url}/events`, (response) => {
      response.setEncoding('utf8');
      response.on('data', (chunk) => {
        eventBody += chunk;
        if (eventBody.includes('retry: 1000')) resolve();
      });
      response.on('error', reject);
    });
    eventRequest.on('error', reject);
  });
  t.after(() => eventRequest?.destroy());
  await eventStarted;

  await rm(outputPath);
  await mkdir(outputPath);
  await writeFile(sourcePath, definition({ title: 'Must not become current' }));
  assert.equal(await dashboard.refresh(), false);
  await eventually(() => eventBody.includes('event: unavailable\n\n'));

  assert.deepEqual(dashboard.getState(), {
    available: false,
    error: 'Dashboard snapshot could not be written.',
  });
  const unavailable = await requestText(`${dashboard.url}/`);
  assert.equal(unavailable.status, 503);
  assert.doesNotMatch(unavailable.body, /Current before output failure|Must not become current/);
});

test('render interface escapes untrusted Definition content', async () => {
  const marker = '<script id="owned">globalThis.definitionOwned=true</script>';
  const source = definition({ title: marker, next: '<img src=x onerror="globalThis.owned=true">' });
  const rendered = await renderDefinitionSource(source, { sourcePath: '/private/user/mission.md' });

  assert.doesNotMatch(rendered.html, /<script id="owned">/);
  assert.doesNotMatch(rendered.html, /<img src=x onerror=/);
  assert.match(rendered.html, /&lt;script id=(?:&quot;|&#34;)owned(?:&quot;|&#34;)&gt;/);
  assert.equal(rendered.model.source.path, 'mission.md');
  assert.match(rendered.model.source.digest, /^[a-f0-9]{64}$/);
  assert.match(rendered.html, /non-authoritative/i);
});

test('gate narrative requires an exact receipt gate_id and allowed adjudication', () => {
  const gates = [
    { id: 'G1-declaration', status: 'accept', adjudication: 'accept' },
    { id: 'G2-alias' },
    { id: 'G3-prose' },
    { id: 'G4-wrong-id' },
    { id: 'G5-accepted' },
    { id: 'G6-exact' },
  ];
  const html = renderDashboard({
    orchestration: { decision_gates: gates },
    evidence: {
      receipts: [
        { id: 'G2-alias', outcome: 'accept' },
        'G3-prose adjudication: accept',
        { gate_id: 'G4-other', adjudication: 'accept' },
        { gate_id: 'G5-accepted', adjudication: 'accepted' },
        { gate_id: 'G6-exact', adjudication: ' Accept ' },
      ],
    },
  });

  assert.equal((html.match(/class="gate-status neutral"><strong>Status:<\/strong> Not reviewed/g) ?? []).length, 5);
  assert.equal((html.match(/class="gate-status positive"><strong>Status:<\/strong> Accept/g) ?? []).length, 1);
  assert.doesNotMatch(html, /class="gate-status [^"]+"><strong>Status:<\/strong> Accepted/);
  for (const label of [
    'Deterministic checks first',
    'Builder must show',
    'Falsifier must challenge',
    'Verifier must confirm',
    'Dissent and blockers',
    'Minority rule',
    'Durable receipt',
  ]) {
    assert.match(html, new RegExp(`<strong class="obligation-label">${label}:</strong> <em>`));
  }
  assert.equal((html.match(/class="gate-obligations"/g) ?? []).length, gates.length);
  assert.match(html, /Deterministic checks first:<\/strong> <em>Not recorded<\/em>/);
  assert.doesNotMatch(html, /Review contract|<details|<summary/);
});

test('owner prose exposes reconciliation, review findings, weak measures, and dissent', () => {
  const html = renderDashboard({
    weakMeasures: [{
      name: 'Panel agreement',
      kind: 'weak_signal',
      baseline: 'Two reviewers agree',
      desired: 'Seek a falsifier',
      decision_use: 'Choose the next inspection',
      cannot_prove: '<accepted artifact>',
      private_dump: 'must stay hidden',
    }],
    measures: [{ name: 'Internal latency', kind: 'telemetry', baseline: '12ms' }],
    dissent: { summary: 'Model-level dissent', unresolved_blockers: ['Owner receipt missing'] },
    now: {
      reconciliation: {
        observed_at: '2026-07-15T22:36:30Z',
        source_ref: 'main@abc123',
        deploy_identity: 'staging@def456',
        status: 'reconciled',
      },
      weak_measures: [{
        name: 'Wrapper count',
        kind: 'weak_signal',
        baseline: 'Four wrappers',
        cannot_prove: 'Behavior preservation',
      }],
      dissent: [{ minority_findings: ['A reproducible minority finding'], evidence_refs: ['review://one'] }],
    },
    orchestration: {
      decision_gates: [{
        id: 'G1-review',
        dissent: 'Verifier disputes provenance',
        minority_findings: ['Route identity is stale'],
        unresolved_blockers: ['Deployment receipt absent'],
        blockers: ['Owner adjudication absent'],
      }],
    },
  });

  assert.match(html, /<h2 id="reconciliation-title">Reconciliation<\/h2>/);
  assert.match(html, /main@abc123/);
  assert.match(html, /staging@def456/);
  assert.match(html, /<h2 id="steering-title" class="steering-heading">Steering only — not proof<\/h2>/);
  assert.match(html, /Panel agreement/);
  assert.match(html, /Wrapper count/);
  assert.match(html, /&lt;accepted artifact&gt;/);
  assert.doesNotMatch(html, /Internal latency|must stay hidden/);
  assert.match(html, /<h2 id="dissent-title">Dissent and blockers<\/h2>/);
  assert.match(html, /Model-level dissent/);
  assert.match(html, /A reproducible minority finding/);
  assert.match(html, /Verifier disputes provenance/);
  assert.match(html, /Route identity is stale/);
  assert.match(html, /Deployment receipt absent/);
  assert.match(html, /Owner adjudication absent/);
  assert.doesNotMatch(html, /<details|<summary/);
});

test('dashboard prose follows owner priority order with only the repository inventory collapsible', () => {
  const html = renderDashboard({
    title: 'Mission brief',
    subtitle: 'Owner-readable projection',
    source: { path: 'mission.md', digest: 'abc123' },
    repository: {
      available: true,
      branch: 'feature/dashboard',
      detached: false,
      head: 'def456def456',
      worktreePath: '/workspace/go-choir-feature',
      worktreeKind: 'linked',
      upstream: 'origin/feature/dashboard',
      upstreamHead: 'abc999abc999',
      ahead: 2,
      behind: 1,
      dirtyFiles: 3,
      addedLines: 10,
      deletedLines: 4,
      binaryFiles: 1,
      unreadableFiles: 0,
      changedFiles: [
        { path: 'docs/mission.md', state: 'staged', code: 'M.', addedLines: 4, deletedLines: 1, binary: false },
        { path: 'skills/dashboard.mjs', state: 'unstaged', code: '.M', addedLines: 6, deletedLines: 3, binary: false },
        { path: 'skills/dashboard-git.mjs', state: 'untracked', code: '??', addedLines: 12, deletedLines: 0, binary: false },
      ],
    },
    finish: {
      deliver: 'Finished outcome',
      artifact: 'Durable artifact',
      acceptance: ['Acceptance action'],
      rollback: 'Rollback action',
      not_done_when: ['Non-completion condition'],
    },
    now: {
      slice: 'build',
      status: 'working',
      next_action: 'Inspect the candidate',
      blocker_or_risk: 'A recorded blocker',
      question: 'A recorded question',
      reconciliation: { status: 'reconciled' },
      decision: { selected: 'Candidate A' },
      candidate: { id: 'candidate-a' },
      evidence_refs: ['receipt://one'],
    },
    orchestration: {
      phase_topology: [{ phase: 'build' }, { phase: 'verify' }],
      decision_gates: [{ id: 'G1-review' }],
    },
    dissent: [{ summary: 'A minority concern' }],
    weak_measures: [{ name: 'Reviewer agreement', cannot_prove: 'Acceptance' }],
    successor: { status: 'unauthorized' },
  });

  const orderedText = [
    'Mission brief',
    'SHA-256 abc123',
    'Read-only owner view.',
    'Branch feature/dashboard',
    'HEAD def456def456',
    'Linked worktree',
    'origin/feature/dashboard @ abc999abc999 · 2 ahead · 1 behind',
    '3 uncommitted files',
    '+10 −4 · 1 binary',
    'docs/mission.md',
    'Staged',
    'repo-delta add">+4',
    'repo-delta del">−1',
    'skills/dashboard.mjs',
    'Unstaged',
    'repo-delta add">+6',
    'repo-delta del">−3',
    'skills/dashboard-git.mjs',
    'Untracked',
    'repo-delta add">+12',
    'Outcome and immediate constraints',
    'Acceptance action',
    'Rollback action',
    'Non-completion condition',
    'Current phase',
    'Next action',
    'Blocker or risk',
    'Open question',
    'Mission phase path',
    'Decision gates',
    'Proof readiness',
    'Protected starting state',
    '<h2 id="reconciliation-title">Reconciliation</h2>',
    '<h2 id="decision-title">Accepted decision</h2>',
    '<h2 id="candidate-title">Candidate</h2>',
    '<h2 id="evidence-title">Evidence references</h2>',
    '<h2 id="dissent-title">Dissent and blockers</h2>',
    'id="steering-title" class="steering-heading">Steering only — not proof',
    '<h2 id="successor-title">Successor boundary</h2>',
  ];
  let previous = -1;
  for (const label of orderedText) {
    const position = html.indexOf(label);
    assert.ok(position > previous, `${label} must follow the preceding briefing section`);
    previous = position;
  }
  assert.equal((html.match(/<details class="repo-changes" open>/g) ?? []).length, 1);
  assert.equal((html.match(/<summary>/g) ?? []).length, 1);
  assert.match(html, /class="repo-meta"/);
  assert.match(html, /class="repo-summary-line"/);
  assert.doesNotMatch(html, /display:\s*contents/);
  assert.doesNotMatch(html, /\.repo-change-list li\s*\{[^}]*border/u);
  assert.match(html, /class="[^"]*\bmasthead-grid\b[^"]*"/);
  assert.match(html, /class="repo-status"/);
  assert.match(html, /class="briefing-grid"/);
  assert.match(html, /class="finish-details"/);
  assert.match(html, /class="phase-list"/);
  assert.match(html, /class="gate-list"/);
  assert.match(html, /class="proof-list"/);
  assert.match(html, /class="secondary-grid"/);
  assert.match(html, /\.gate-list\s*\{[\s\S]*?grid-template-columns:\s*repeat\(2,/);
  assert.match(html, /\.proof-list\s*\{[\s\S]*?grid-template-columns:\s*repeat\(3,/);
  assert.match(html, /@media \(max-width:[^)]+\)[\s\S]*?\.finish-details,[\s\S]*?grid-template-columns:\s*1fr/);
  assert.match(html, /@media \(max-width:[^)]+\)[\s\S]*?\.shell\s*\{[^}]*width:\s*calc\(100%/);
  assert.match(html, /<div class="prose-fields"><p><strong class="field-label">Status:<\/strong> <span>reconciled<\/span><\/p><\/div>/);
  assert.match(html, /<ul class="plain-list"><li>receipt:\/\/one<\/li><\/ul>/);
  assert.doesNotMatch(html, /<dl\b|class="(?:card|pill|badge)\b/);
});

test('explicit snapshot generation reflects source changes and has no implicit output', async (t) => {
  const directory = await mkdtemp(join(tmpdir(), 'definition-dashboard-generate-'));
  const sourcePath = join(directory, 'mission.md');
  t.after(() => rm(directory, { recursive: true, force: true }));

  await writeFile(sourcePath, definition({ title: 'Generation one' }));
  const first = await generateDashboard(sourcePath, { generatedAt: '2026-07-15T00:00:00.000Z' });
  await writeFile(sourcePath, definition({ title: 'Generation two' }));
  const second = await generateDashboard(sourcePath, { generatedAt: '2026-07-15T00:00:01.000Z' });

  assert.notEqual(first.digest, second.digest);
  assert.match(first.html, /Generation one/);
  assert.match(second.html, /Generation two/);
  assert.doesNotMatch(second.html, /Generation one/);
});
