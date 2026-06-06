/**
 * VText API client for the go-choir desktop shell.
 *
 * Communicates with the versioned document APIs through the same-origin proxy:
 *   POST   /api/vtext/documents                   — create a new document
 *   GET    /api/vtext/documents                   — list documents
 *   POST   /api/vtext/files/open                  — resolve/create aliased file doc
 *   POST   /api/vtext/documents/{id}/manifest     — ensure a filesystem manifestation
 *   GET    /api/vtext/documents/{id}              — get a document
 *   PUT    /api/vtext/documents/{id}              — update a document (title)
 *   DELETE /api/vtext/documents/{id}              — delete a document
 *   POST   /api/vtext/documents/{id}/revisions    — create a revision
 *   GET    /api/vtext/documents/{id}/revisions    — list revisions
 *   GET    /api/vtext/revisions/{id}              — get a revision (snapshot)
 *   GET    /api/vtext/documents/{id}/history      — revision history
 *   GET    /api/vtext/diff?from=X&to=Y            — diff two revisions
 *   GET    /api/vtext/revisions/{id}/blame        — blame revision
 *   GET    /api/vtext/documents/{id}/stream       — document-scoped stream
 *   POST   /api/vtext/documents/{id}/revise        — request a VText revision
 *   POST   /api/vtext/documents/{id}/source-attachments — attach readable source artifacts
 *   POST   /api/content/items                      — create owner-scoped content item
 *   POST   /api/content/import-url                 — import readable URL content
 *   POST   /api/platform/vtext/publications        — publish selected VText revision
 *   GET    /api/platform/publications/resolve      — resolve public publication bundle
 *   GET    /api/platform/publications/export       — export canonical publication artifact
 *   GET    /api/platform/retrieval/search          — search public published spans
 *   POST   /api/platform/publications/{id}/proposals — submit reader derivative proposal
 */

import { fetchWithRenewal } from './auth.js';
import { withDesktopSelector } from './desktop-selector.js';

function vtextPath(path) {
  return `/api/vtext${path}`;
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
  const res = await fetchWithRenewal(vtextPath('/documents'), {
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
  const res = await fetchWithRenewal(vtextPath('/files/open'), {
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
  const res = await fetchWithRenewal(vtextPath('/documents'), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `List documents failed (${res.status})`);
  }

  return res.json();
}

export async function getDocument(docId) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get document failed (${res.status})`);
  }

  return res.json();
}

export async function ensureDocumentManifest(docId) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/manifest`), {
    method: 'POST',
  });

  if (!res.ok) {
    await decodeError(res, `Ensure document manifest failed (${res.status})`);
  }

  return res.json();
}

export async function updateDocument(docId, title) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}`), {
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
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}`), {
    method: 'DELETE',
  });

  if (!res.ok) {
    await decodeError(res, `Delete document failed (${res.status})`);
  }

  return res.json();
}

export async function createRevision(docId, { content, authorKind, authorLabel, citations, metadata, parentRevisionId, allowRebase = false }) {
  const body = {
    content,
    author_kind: authorKind,
    author_label: authorLabel,
  };
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

  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/revisions`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    await decodeError(res, `Create revision failed (${res.status})`);
  }

  return res.json();
}

export async function listRevisions(docId, { limit = 10000 } = {}) {
  const params = new URLSearchParams();
  if (limit) {
    params.set('limit', String(limit));
  }
  const query = params.toString();
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/revisions${query ? `?${query}` : ''}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `List revisions failed (${res.status})`);
  }

  return res.json();
}

export async function getRevision(revisionId) {
  const res = await fetchWithRenewal(vtextPath(`/revisions/${encodeURIComponent(revisionId)}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get revision failed (${res.status})`);
  }

  return res.json();
}

export async function getHistory(docId) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/history`), {
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
  const res = await fetchWithRenewal(vtextPath(`/diff?${params.toString()}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get diff failed (${res.status})`);
  }

  return res.json();
}

export async function semanticCompareVText(docId, { sourceRevisionId, targetRevisionId = '' } = {}) {
  const params = new URLSearchParams({
    source: sourceRevisionId || '',
    target: targetRevisionId || '',
  });
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/compare?${params.toString()}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Compare VText versions failed (${res.status})`);
  }

  return res.json();
}

export async function previewVTextMerge(docId, payload = {}) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/merge-preview`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Preview VText merge failed (${res.status})`);
  }

  return res.json();
}

export async function acceptVTextMerge(docId, payload = {}) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/accept-merge`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Accept VText merge failed (${res.status})`);
  }

  return res.json();
}

export async function restoreVTextRevision(docId, { revisionId, mode = 'restore_as_latest' } = {}) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/restore`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      revision_id: revisionId,
      mode,
    }),
  });

  if (!res.ok) {
    await decodeError(res, `Restore VText revision failed (${res.status})`);
  }

  return res.json();
}

export async function getVTextDiagnosis(docId, limit = 50, options = {}) {
  const params = new URLSearchParams({ limit: String(limit || 50) });
  if (options.includeContent === false) {
    params.set('include_content', 'false');
  }
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/diagnosis?${params.toString()}`), {
    method: 'GET',
    signal: options.signal,
  });

  if (!res.ok) {
    await decodeError(res, `Get VText diagnosis failed (${res.status})`);
  }

  return res.json();
}

export async function repairVTextSourceGaps(docId, payload = {}) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/source-repairs`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Repair VText sources failed (${res.status})`);
  }

  return res.json();
}

export async function attachVTextSourceArtifacts(docId, payload = {}) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/source-attachments`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Attach VText source artifacts failed (${res.status})`);
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

export async function getBlame(revisionId) {
  const res = await fetchWithRenewal(vtextPath(`/revisions/${encodeURIComponent(revisionId)}/blame`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get blame failed (${res.status})`);
  }

  return res.json();
}

export async function createAgentRevision(docId, payload = {}) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/revise`), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `VText revise failed (${res.status})`);
  }

  return res.json();
}

export async function submitAgentRevision(docId, payload = {}) {
  return createAgentRevision(docId, payload);
}

export async function cancelAgentRevision(docId) {
  const res = await fetchWithRenewal(vtextPath(`/documents/${encodeURIComponent(docId)}/cancel`), {
    method: 'POST',
  });

  if (!res.ok) {
    await decodeError(res, `Cancel VText revision failed (${res.status})`);
  }

  return res.json();
}

export function openDocumentStream(docId, { onEvent, onError } = {}) {
  const source = new EventSource(withDesktopSelector(vtextPath(`/documents/${encodeURIComponent(docId)}/stream`)));

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

export async function publishVText(docId, { revisionId = '', slug = '', accessPolicy = null, exportPolicy = null } = {}) {
  const payload = {
    doc_id: docId,
    revision_id: revisionId,
    slug,
  };
  if (accessPolicy && typeof accessPolicy === 'object') payload.access_policy = accessPolicy;
  if (exportPolicy && typeof exportPolicy === 'object') payload.export_policy = exportPolicy;

  const res = await fetchWithRenewal(platformPath('/vtext/publications'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    await decodeError(res, `Publish VText failed (${res.status})`);
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

export async function searchPublishedVText(query) {
  const params = new URLSearchParams({ q: query || '' });
  const res = await fetch(platformPath(`/retrieval/search?${params.toString()}`), {
    method: 'GET',
    credentials: 'include',
  });

  if (!res.ok) {
    await decodeError(res, `Search published VTexts failed (${res.status})`);
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
