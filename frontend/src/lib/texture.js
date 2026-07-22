/**
 * Texture API client for the go-choir desktop shell.
 *
 * Communicates with the versioned document APIs through the same-origin proxy:
 *   POST   /api/texture/documents                   — create a new document
 *   GET    /api/texture/documents                   — list documents
 *   POST   /api/texture/files/open                  — resolve/create aliased file doc
 *   POST   /api/texture/documents/{id}/manifest     — ensure a filesystem manifestation
 *   GET    /api/texture/documents/{id}              — get a document
 *   PUT    /api/texture/documents/{id}              — update a document (title)
 *   DELETE /api/texture/documents/{id}              — delete a document
 *   POST   /api/texture/documents/{id}/revisions    — create a revision
 *   GET    /api/texture/documents/{id}/revisions    — list revisions
 *   GET    /api/texture/revisions/{id}              — get a revision (snapshot)
 *   GET    /api/texture/documents/{id}/history      — revision history
 *   GET    /api/texture/diff?from=X&to=Y            — diff two revisions
 *   GET    /api/texture/revisions/{id}/blame        — blame revision
 *   GET    /api/texture/documents/{id}/stream       — document-scoped stream
 *   POST   /api/texture/documents/{id}/revise        — request a Texture revision
 *   POST   /api/content/items                      — create owner-scoped content item
 *   POST   /api/content/import-url                 — import readable URL content
 *   POST   /api/content/import-file                — import an existing user-computer file
 *   POST   /api/platform/texture/publications      — publish selected Texture revision
 *   GET    /api/platform/publications/resolve      — resolve public publication bundle
 *   GET    /api/platform/publications/export       — export canonical publication artifact
 *   GET    /api/platform/retrieval/search          — search public published spans
 *   POST   /api/platform/publications/{id}/proposals — submit reader derivative proposal
 */

import { fetchWithRenewal } from './auth.js';
import { withDesktopSelector } from './desktop-selector.js';

export const WIRE_PLATFORM_READ_OWNER = 'universal-wire-platform';

export function pushTextureReadOwner(ownerId = '') {
  return String(ownerId || '').trim();
}

export function popTextureReadOwner() {
  return '';
}

function withReadOwnerQuery(path, { method = 'GET', readOwner = '' } = {}) {
  const owner = String(readOwner || '').trim();
  if (!owner || (method !== 'GET' && method !== 'HEAD')) {
    return path;
  }
  const join = path.includes('?') ? '&' : '?';
  return `${path}${join}read_owner=${encodeURIComponent(owner)}`;
}

function texturePath(path) {
  return `/api/texture${path}`;
}

function platformPath(path) {
  return `/api/platform${path}`;
}

function contentPath(path) {
  return `/api/content${path}`;
}

async function decodeError(res, fallback) {
  const err = await res.json().catch(() => ({}));
  throw new Error(err.error || fallback);
}

export async function createDocument(title) {
  const res = await fetchWithRenewal(texturePath('/documents'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  });

  if (!res.ok) {
    await decodeError(res, `Create document failed (${res.status})`);
  }

  return res.json();
}

export async function openFileDocument({ sourcePath, title, initialContent }) {
  const res = await fetchWithRenewal(texturePath('/files/open'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      source_path: sourcePath,
      title,
      initial_content: initialContent,
    }),
  });

  if (!res.ok) {
    await decodeError(res, `Open file document failed (${res.status})`);
  }

  return res.json();
}

export async function listDocuments() {
  const res = await fetchWithRenewal(texturePath('/documents'), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `List documents failed (${res.status})`);
  }

  return res.json();
}

export async function getDocument(docId, { readOwner = '' } = {}) {
  const res = await fetchWithRenewal(withReadOwnerQuery(texturePath(`/documents/${encodeURIComponent(docId)}`), { readOwner }), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get document failed (${res.status})`);
  }

  return res.json();
}

export async function ensureDocumentManifest(docId) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/manifest`), {
    method: 'POST',
  });

  if (!res.ok) {
    await decodeError(res, `Ensure document manifest failed (${res.status})`);
  }

  return res.json();
}

export async function updateDocument(docId, title) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}`), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  });

  if (!res.ok) {
    await decodeError(res, `Update document failed (${res.status})`);
  }

  return res.json();
}

export async function deleteDocument(docId) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}`), {
    method: 'DELETE',
  });

  if (!res.ok) {
    await decodeError(res, `Delete document failed (${res.status})`);
  }

  return res.json();
}

export async function createRevision(docId, { content, bodyDoc, sourceEntities, authorKind, authorLabel, citations, metadata, parentRevisionId, allowRebase = false }) {
  const body = {
    content,
    author_kind: authorKind,
    author_label: authorLabel,
  };
  if (bodyDoc !== undefined) {
    body.body_doc = bodyDoc;
  }
  if (sourceEntities !== undefined) {
    body.source_entities = sourceEntities;
  }
  if (citations !== undefined) {
    body.citations = citations;
  }
  if (metadata !== undefined) {
    body.metadata = metadata;
  }
  if (parentRevisionId) {
    body.parent_revision_id = parentRevisionId;
  }
  if (allowRebase) {
    body.allow_rebase = true;
  }
  const document = await getDocument(docId);
  if (document?.trajectory_id) {
    const lifecycleRes = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(document.trajectory_id)}`, {
      method: 'GET',
    });
    if (!lifecycleRes.ok) {
      await decodeError(lifecycleRes, `Load lifecycle snapshot failed (${lifecycleRes.status})`);
    }
    const lifecycle = await lifecycleRes.json();
    body.idempotency_key = crypto.randomUUID();
    body.expected_lifecycle_version = lifecycle?.trajectory?.lifecycle_version;
    body.parent_revision_id = parentRevisionId || lifecycle?.head_revision?.revision_id;
  }


  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/revisions`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    await decodeError(res, `Create revision failed (${res.status})`);
  }

  return res.json();
}

export async function listRevisions(docId, { limit = 10000, readOwner = '' } = {}) {
  const params = new URLSearchParams();
  if (limit) {
    params.set('limit', String(limit));
  }
  const query = params.toString();
  const res = await fetchWithRenewal(withReadOwnerQuery(texturePath(`/documents/${encodeURIComponent(docId)}/revisions${query ? `?${query}` : ''}`), { readOwner }), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `List revisions failed (${res.status})`);
  }

  return res.json();
}

export async function getRevision(revisionId, { readOwner = '' } = {}) {
  const res = await fetchWithRenewal(withReadOwnerQuery(texturePath(`/revisions/${encodeURIComponent(revisionId)}`), { readOwner }), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get revision failed (${res.status})`);
  }

  return res.json();
}

export async function getHistory(docId, { readOwner = '' } = {}) {
  const res = await fetchWithRenewal(withReadOwnerQuery(texturePath(`/documents/${encodeURIComponent(docId)}/history`), { readOwner }), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get history failed (${res.status})`);
  }

  return res.json();
}

export async function getDiff(fromRevisionId, toRevisionId) {
  const params = new URLSearchParams({
    from: fromRevisionId,
    to: toRevisionId,
  });
  const res = await fetchWithRenewal(texturePath(`/diff?${params.toString()}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get diff failed (${res.status})`);
  }

  return res.json();
}

export async function semanticCompareTexture(docId, { sourceRevisionId, targetRevisionId = '', readOwner = '' } = {}) {
  const params = new URLSearchParams({
    source: sourceRevisionId || '',
    target: targetRevisionId || '',
  });
  const res = await fetchWithRenewal(withReadOwnerQuery(texturePath(`/documents/${encodeURIComponent(docId)}/compare?${params.toString()}`), { readOwner }), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Compare Texture versions failed (${res.status})`);
  }

  return res.json();
}

export async function previewTextureMerge(docId, payload = {}) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/merge-preview`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Preview Texture merge failed (${res.status})`);
  }

  return res.json();
}

export async function acceptTextureMerge(docId, payload = {}) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/accept-merge`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Accept Texture merge failed (${res.status})`);
  }

  return res.json();
}

export async function restoreTextureRevision(docId, { revisionId, mode = 'restore_as_latest' } = {}) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/restore`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      revision_id: revisionId,
      mode,
    }),
  });

  if (!res.ok) {
    await decodeError(res, `Restore Texture revision failed (${res.status})`);
  }

  return res.json();
}

export async function getTextureDiagnosis(docId, limit = 50, options = {}) {
  const params = new URLSearchParams({ limit: String(limit || 50) });
  if (options.includeContent === false) {
    params.set('include_content', 'false');
  }
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/diagnosis?${params.toString()}`), {
    method: 'GET',
    signal: options.signal,
  });

  if (!res.ok) {
    await decodeError(res, `Get Texture diagnosis failed (${res.status})`);
  }

  return res.json();
}

export async function createContentItem(payload = {}) {
  const res = await fetchWithRenewal(contentPath('/items'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Create content item failed (${res.status})`);
  }

  return res.json();
}

export async function importContentURL(url, query = '') {
  const res = await fetchWithRenewal(contentPath('/import-url'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url, query }),
  });

  if (!res.ok) {
    await decodeError(res, `Import source URL failed (${res.status})`);
  }

  return res.json();
}

export async function importContentFile(filePath) {
  const res = await fetchWithRenewal(contentPath('/import-file'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ file_path: filePath }),
  });

  if (!res.ok) {
    await decodeError(res, `Import source file failed (${res.status})`);
  }

  return res.json();
}

export async function getBlame(revisionId) {
  const res = await fetchWithRenewal(texturePath(`/revisions/${encodeURIComponent(revisionId)}/blame`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get blame failed (${res.status})`);
  }

  return res.json();
}

export async function createAgentRevision(docId, payload = {}) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/revise`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Texture revise failed (${res.status})`);
  }

  return res.json();
}

export async function submitAgentRevision(docId, payload = {}) {
  return createAgentRevision(docId, payload);
}

export async function cancelAgentRevision(docId, options = {}) {
  const document = await getDocument(docId);
  const trajectoryId = String(document.trajectory_id || '').trim();
  if (!trajectoryId) {
    throw new Error('Document has no durable lifecycle');
  }
  const snapshotResponse = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}`, { method: 'GET' });
  if (!snapshotResponse.ok) {
    await decodeError(snapshotResponse, `Lifecycle snapshot failed (${snapshotResponse.status})`);
  }
  const snapshot = await snapshotResponse.json();
  const expectedVersion = snapshot?.trajectory?.lifecycle_version;
  const expectedHead = snapshot?.head_revision?.revision_id;
  if (!Number.isSafeInteger(expectedVersion) || expectedVersion <= 0 || !expectedHead) {
    throw new Error('Lifecycle snapshot lacks cancellation preconditions');
  }
  const commandId = options.commandId || `desktop-cancel:${trajectoryId}:${expectedVersion}:${expectedHead}`;
  const res = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      idempotency_key: commandId,
      expected_lifecycle_version: expectedVersion,
      expected_head_revision_id: expectedHead,
      reason: options.reason || 'owner cancellation',
    }),
  });
  if (!res.ok) {
    await decodeError(res, `Cancel Texture revision failed (${res.status})`);
  }
  return res.json();
}

export function openDocumentStream(docId, { onEvent, onError, readOwner = '' } = {}) {
  const source = new EventSource(withDesktopSelector(withReadOwnerQuery(texturePath(`/documents/${encodeURIComponent(docId)}/stream`), { readOwner })));

  source.onmessage = (event) => {
    if (!onEvent) return;
    try {
      onEvent(JSON.parse(event.data));
    } catch (err) {
      if (onError) onError(err);
    }
  };
  source.onerror = (err) => {
    if (onError) onError(err);
  };

  return source;
}

export async function publishTexture(docId, { revisionId = '', slug = '', accessPolicy = null, exportPolicy = null } = {}) {
  const payload = {
    doc_id: docId,
    revision_id: revisionId,
    slug,
  };
  if (accessPolicy && typeof accessPolicy === 'object') payload.access_policy = accessPolicy;
  if (exportPolicy && typeof exportPolicy === 'object') payload.export_policy = exportPolicy;

  const res = await fetchWithRenewal(platformPath('/texture/publications'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Publish Texture failed (${res.status})`);
  }

  return res.json();
}

export async function resolvePublication(routePath) {
  const params = new URLSearchParams({ route: routePath });
  const res = await fetch(platformPath(`/publications/resolve?${params.toString()}`), {
    method: 'GET',
    credentials: 'include',
  });

  if (!res.ok) {
    await decodeError(res, `Resolve publication failed (${res.status})`);
  }

  return res.json();
}

export async function exportPublication(routePath, format = 'txt') {
  const params = new URLSearchParams({ route: routePath, format });
  const res = await fetch(platformPath(`/publications/export?${params.toString()}`), {
    method: 'GET',
    credentials: 'include',
  });

  if (!res.ok) {
    await decodeError(res, `Export publication failed (${res.status})`);
  }

  return res.json();
}

export async function searchPublishedTexture(query) {
  const params = new URLSearchParams({ q: query || '' });
  const res = await fetch(platformPath(`/retrieval/search?${params.toString()}`), {
    method: 'GET',
    credentials: 'include',
  });

  if (!res.ok) {
    await decodeError(res, `Search published Textures failed (${res.status})`);
  }

  return res.json();
}

export async function submitPublicationProposal(publicationId, { docId, revisionId = '', publicationVersionId = '', transclusions = [] } = {}) {
  const res = await fetchWithRenewal(platformPath(`/publications/${encodeURIComponent(publicationId)}/proposals`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      doc_id: docId,
      revision_id: revisionId,
      publication_version_id: publicationVersionId,
      transclusions,
    }),
  });

  if (!res.ok) {
    await decodeError(res, `Submit proposal failed (${res.status})`);
  }

  return res.json();
}
