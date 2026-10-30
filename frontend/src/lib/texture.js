/**
 * Texture API client for the go-choir desktop shell.
 *
 * Communicates with the versioned document APIs through the same-origin proxy:
 *   POST   /api/texture/documents                   — create a new document
 *   GET    /api/texture/documents                   — list documents
 *   POST   /api/texture/files/open                  — resolve/create aliased file doc
 *   POST   /api/texture/documents/{id}/manifest     — ensure a filesystem manifestation
 *   GET    /api/texture/documents/{id}              — get a document
 *   POST   /api/texture/documents/{id}/revisions    — create a revision
 *   GET    /api/texture/documents/{id}/revisions    — list revisions
 *   GET    /api/texture/revisions/{id}              — get a revision (snapshot)
 *   GET    /api/texture/documents/{id}/stream       — document-scoped stream
 *   POST   /api/texture/documents/{id}/revise        — request a Texture revision
 *   POST   /api/content/items                      — create owner-scoped content item
 *   POST   /api/platform/texture/publications      — publish selected Texture revision
 *   GET    /api/platform/publications/resolve      — resolve public publication bundle
 *   GET    /api/platform/publications/export       — export canonical publication artifact
 *   POST   /api/platform/publications/{id}/proposals — submit reader derivative proposal
 */

import { fetchWithRenewal } from './auth.js';
import { withDesktopSelector } from './desktop-selector.js';

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

export async function getDocument(docId) {
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}`), {
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

export function lifecycleCurrentDocumentRevisionID(snapshot) {
  return snapshot?.current_document_head?.revision_id
    || snapshot?.document?.current_revision_id
    || snapshot?.head_revision?.revision_id
    || '';
}

// createRevision posts an owner revision. The server decodes the body strictly
// (unknown fields are a 400) and always records the authenticated owner as the
// author, so the body carries no authorship fields
// (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 1).
export async function createRevision(docId, { content, bodyDoc, sourceEntities, citations, metadata, parentRevisionId, allowRebase = false }) {
  const body = { content };
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
    body.parent_revision_id = parentRevisionId || lifecycleCurrentDocumentRevisionID(lifecycle);
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

export async function listRevisions(docId, { limit = 10000 } = {}) {
  const params = new URLSearchParams();
  if (limit) {
    params.set('limit', String(limit));
  }
  const query = params.toString();
  const res = await fetchWithRenewal(texturePath(`/documents/${encodeURIComponent(docId)}/revisions${query ? `?${query}` : ''}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `List revisions failed (${res.status})`);
  }

  return res.json();
}

export async function getRevision(revisionId) {
  const res = await fetchWithRenewal(texturePath(`/revisions/${encodeURIComponent(revisionId)}`), {
    method: 'GET',
  });

  if (!res.ok) {
    await decodeError(res, `Get revision failed (${res.status})`);
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

export async function submitAgentRevision(docId, payload = {}) {
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

export async function cancelAgentRevision(docId, options = {}) {
  const document = await getDocument(docId);
  const trajectoryId = String(document.trajectory_id || '').trim();
  if (!trajectoryId) {
    throw new Error('Document has no durable lifecycle');
  }
  // The summary view is a single-object read; the full lifecycle snapshot
  // scans the whole computer and took 9-18 s on long-lived computers.
  const summaryResponse = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}?view=summary`, { method: 'GET' });
  if (!summaryResponse.ok) {
    await decodeError(summaryResponse, `Lifecycle summary failed (${summaryResponse.status})`);
  }
  const summary = await summaryResponse.json();
  // An unfinished earlier Cancel is durable; retrying it with a key derived
  // from the now-advanced version would conflict, so resend it exactly.
  const pending = summary?.pending_cancellation;
  const expectedVersion = pending ? pending.expected_lifecycle_version : summary?.trajectory?.lifecycle_version;
  const expectedHead = pending ? pending.expected_head_revision_id : summary?.head_revision_id;
  if (!Number.isSafeInteger(expectedVersion) || expectedVersion <= 0 || !expectedHead) {
    throw new Error('Lifecycle snapshot lacks cancellation preconditions');
  }
  const commandId = pending?.idempotency_key || options.commandId || `desktop-cancel:${trajectoryId}:${expectedVersion}:${expectedHead}`;
  const res = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}/cancel`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      idempotency_key: commandId,
      expected_lifecycle_version: expectedVersion,
      expected_head_revision_id: expectedHead,
      reason: pending ? pending.reason : (options.reason || 'owner cancellation'),
    }),
  });
  if (!res.ok) {
    await decodeError(res, `Cancel Texture revision failed (${res.status})`);
  }
  return res.json();
}

export function openDocumentStream(docId, { onEvent, onError } = {}) {
  const source = new EventSource(withDesktopSelector(texturePath(`/documents/${encodeURIComponent(docId)}/stream`)));

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

// publishTextureAsOpinion publishes one revision as a labelled opinion piece,
// listed in the Autopaper Opinion section. It takes the signed-in browser
// session only; the server sets the mode, the access policy and the byline
// (a short handle, never the email), and refuses with a plain message when
// opinion publishing is off, the daily limit is reached, or the Texture was
// already published as a plain link.
export async function publishTextureAsOpinion(docId, { revisionId = '', slug = '' } = {}) {
  const res = await fetchWithRenewal(platformPath('/texture/opinions'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ doc_id: docId, revision_id: revisionId, slug }),
  });

  if (!res.ok) {
    await decodeError(res, `Publish as opinion failed (${res.status})`);
  }

  return res.json();
}

// resolvePublication reads a published link's bundle. view 'nav' asks for
// the route, publication and nav alone: the reader's poll for a newer
// version needs nothing else, and the whole bundle costs about a second to
// build (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md).
export async function resolvePublication(routePath, { view = '' } = {}) {
  const params = new URLSearchParams({ route: routePath });
  if (view === 'nav') params.set('view', 'nav');
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
