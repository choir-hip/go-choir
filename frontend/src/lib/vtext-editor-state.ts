export function draftStorageKey(ownerID = 'guest', docID = '') {
  if (!docID) return '';
  return `choir:vtext:draft:${ownerID || 'guest'}:${docID}`;
}

export function markdownTableBlockCount(content = '') {
  const lines = String(content || '').split(/\r?\n/);
  let count = 0;
  for (let i = 0; i < lines.length - 1; i += 1) {
    const header = lines[i] || '';
    const separator = lines[i + 1] || '';
    if (!header.includes('|') || !separator.includes('|')) continue;
    if (!/^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(separator)) continue;
    count += 1;
    i += 1;
    while (i + 1 < lines.length && (lines[i + 1] || '').includes('|')) {
      i += 1;
    }
  }
  return count;
}

export function sortRevisionsChronologically(items: any[] = []) {
  const byId = new Map();
  for (const item of items || []) {
    if (item?.revision_id) {
      byId.set(item.revision_id, item);
    }
  }

  const fallbackCompare = (left: any, right: any) => {
    const leftTime = new Date(left?.created_at || 0).getTime();
    const rightTime = new Date(right?.created_at || 0).getTime();
    if (leftTime !== rightTime) return leftTime - rightTime;
    return String(left?.revision_id || '').localeCompare(String(right?.revision_id || ''));
  };

  const childrenByParent = new Map();
  const roots = [];
  for (const item of byId.values()) {
    const parentId = item.parent_revision_id || '';
    if (parentId && byId.has(parentId)) {
      const children = childrenByParent.get(parentId) || [];
      children.push(item);
      childrenByParent.set(parentId, children);
    } else {
      roots.push(item);
    }
  }

  for (const children of childrenByParent.values()) {
    children.sort(fallbackCompare);
  }
  roots.sort(fallbackCompare);

  const ordered: any[] = [];
  const visited = new Set();
  function visit(item: any) {
    if (!item?.revision_id || visited.has(item.revision_id)) return;
    visited.add(item.revision_id);
    ordered.push(item);
    for (const child of childrenByParent.get(item.revision_id) || []) {
      visit(child);
    }
  }

  for (const root of roots) {
    visit(root);
  }

  const leftovers = [...byId.values()]
    .filter((item) => !visited.has(item.revision_id))
    .sort(fallbackCompare);
  for (const item of leftovers) {
    visit(item);
  }

  return ordered;
}

export function revisionVersionNumber(revision: any, fallbackIndex = -1) {
  const value = Number(revision?.version_number);
  if (Number.isFinite(value) && value >= 0) return value;
  return Math.max(0, Number(fallbackIndex) || 0);
}

export function versionLabelForRevision(revision: any, fallbackIndex = -1) {
  return `v${revisionVersionNumber(revision, fallbackIndex)}`;
}

export function documentCurrentVersionNumber(doc: any, revisions: any[] = []) {
  const fromDoc = Number(doc?.current_version_number);
  if (Number.isFinite(fromDoc) && fromDoc >= 0) return fromDoc;
  const maxKnown = revisions.reduce((max, rev, index) => Math.max(max, revisionVersionNumber(rev, index)), -1);
  if (maxKnown >= 0) return maxKnown;
  const revisionCount = Number(doc?.revision_count);
  if (Number.isFinite(revisionCount) && revisionCount > 0) return revisionCount - 1;
  return 0;
}

export function nextVersionNumber(doc: any, revisions: any[] = []) {
  return documentCurrentVersionNumber(doc, revisions) + 1;
}

export function explicitPublishAccessPolicy() {
  return {
    visibility: 'public',
    route: 'public',
  };
}

export function explicitPublishExportPolicy() {
  return {
    copy_allowed: true,
    download_allowed: true,
    formats: ['txt', 'md', 'html', 'docx', 'pdf'],
  };
}

export function publicURLForPublishResult(result: any, origin = '') {
  const direct = String(result?.public_url || '').trim();
  if (direct) return direct;
  const routePath = String(result?.route_path || '').trim();
  if (!routePath) return '';
  if (/^https?:\/\//.test(routePath)) return routePath;
  if (!origin) return routePath;
  return `${origin}${routePath.startsWith('/') ? routePath : `/${routePath}`}`;
}

export function truncateText(value: any, max = 360) {
  const text = String(value || '').trim();
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1).trimEnd()}…`;
}

export function shortHash(value: any) {
  const text = String(value || '');
  if (text.length <= 18) return text;
  return `${text.slice(0, 10)}…${text.slice(-6)}`;
}
