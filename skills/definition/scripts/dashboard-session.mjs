import { createHash } from 'node:crypto';

export const MAX_SESSION_EVENTS = 100;
export const SESSION_RECENT_COUNT = 5;

function asIso(value = new Date()) {
  if (typeof value === 'string') return value;
  return new Date(value).toISOString();
}

export function createSessionLog({
  startedAt = new Date().toISOString(),
  maxEvents = MAX_SESSION_EVENTS,
  recentCount = SESSION_RECENT_COUNT,
} = {}) {
  const events = [];
  const dirtyFirstSeen = new Map();
  let dirtyFiles = [];

  function record(event) {
    if (!event || typeof event.kind !== 'string' || typeof event.summary !== 'string') {
      throw new TypeError('session events require kind and summary strings');
    }
    const entry = {
      at: asIso(event.at),
      kind: event.kind,
      summary: event.summary,
      detail: typeof event.detail === 'string' && event.detail !== '' ? event.detail : null,
    };
    events.unshift(entry);
    if (events.length > maxEvents) events.length = maxEvents;
    return entry;
  }

  function observeRepository(repository, observedAt = new Date().toISOString()) {
    const at = asIso(observedAt);
    const current = new Set();
    const nextFiles = [];
    for (const file of Array.isArray(repository?.changedFiles) ? repository.changedFiles : []) {
      if (!file || typeof file.path !== 'string' || file.path === '') continue;
      current.add(file.path);
      if (!dirtyFirstSeen.has(file.path)) dirtyFirstSeen.set(file.path, at);
      nextFiles.push({
        ...file,
        firstSeenAt: dirtyFirstSeen.get(file.path),
        lastModifiedAt: file.lastModifiedAt ?? null,
      });
    }
    for (const path of [...dirtyFirstSeen.keys()]) {
      if (!current.has(path)) dirtyFirstSeen.delete(path);
    }
    nextFiles.sort((left, right) => {
      const leftStamp = left.lastModifiedAt || left.firstSeenAt || '';
      const rightStamp = right.lastModifiedAt || right.firstSeenAt || '';
      return rightStamp.localeCompare(leftStamp) || left.path.localeCompare(right.path);
    });
    dirtyFiles = nextFiles;
    if (!repository || typeof repository !== 'object') return repository;
    return {
      ...repository,
      changedFiles: nextFiles,
    };
  }

  function snapshot() {
    return {
      startedAt: asIso(startedAt),
      eventCount: events.length,
      recentCount,
      events: events.map((event) => ({ ...event })),
      recentEvents: events.slice(0, recentCount).map((event) => ({ ...event })),
      earlierEvents: events.slice(recentCount).map((event) => ({ ...event })),
      dirtyFiles: dirtyFiles.map((file) => ({
        path: file.path,
        state: file.state ?? null,
        firstSeenAt: file.firstSeenAt ?? null,
        lastModifiedAt: file.lastModifiedAt ?? null,
      })),
    };
  }

  return {
    record,
    observeRepository,
    snapshot,
  };
}

export function sessionFingerprint(snapshot) {
  return createHash('sha256').update(JSON.stringify(snapshot ?? null)).digest('hex');
}
