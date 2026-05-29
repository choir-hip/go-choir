<!--
  VTextEditor — focused version-native document surface for go-choir.

  The window should feel like the document itself:
    - the text area fills almost the entire window
    - floating controls handle prompt/apply and version navigation
    - prompt/apply creates a user revision, then invokes the vtext appagent
-->
<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount, tick } from 'svelte';
  import { AuthRequiredError, fetchWithRenewal } from './auth.js';
  import {
    cancelAgentRevision,
    createDocument,
    createRevision,
    ensureDocumentManifest,
    getDocument,
    getRevision,
    listDocuments,
    listRevisions,
    openDocumentStream,
    publishVText,
    resolvePublication,
    submitAgentRevision,
    submitPublicationProposal,
  } from './vtext.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import { previewVTextDocument } from './public-preview-data';

  export let currentUser = null;
  export let authenticated = false;
  export let appContext = {};
  export let windowId = '';

  const dispatch = createEventDispatcher();

  let loading = true;
  let submitting = false;
  let agentPending = false;
  let error = '';
  let saveStatus = '';
  let currentDoc = null;
  let currentRevision = null;
  let revisions = [];
  let activeRevisionIndex = -1;
  let editorValue = '';
  let initializedKey = '';
  let latestHeadRevisionId = '';
  let pendingHeadRevisionId = '';
  let newVersionAvailable = false;
  let streamSource = null;
  let streamDocId = '';
  let showRecent = false;
  let recentLoading = false;
  let recentDocuments = [];
  let editorSurface = null;
  let surfaceFocused = false;
  let toolbarHidden = false;
  let lastDocumentScrollTop = 0;
  let toolbarHideSettleUntil = 0;
  let autosaveTimer = null;
  let autosavePromise = null;
  let autosaveInFlight = false;
  let autosaveQueued = false;
  let lastAutosavedContent = '';
  let publishedBundle = null;
  let publishedRoutePath = '';
  let publishedDerivativeActive = false;
  let publishedTransclusions = [];
  let publishedProposal = null;
  let publishedActionPending = false;
  let publishResult = null;
  let cancelPending = false;
  let removeLiveListener = () => {};

  const AUTOSAVE_DELAY_MS = 900;
  const TOOLBAR_HIDE_SCROLL_DELTA = 8;
  const TOOLBAR_HIDE_SCROLL_TOP = 56;
  const TOOLBAR_HIDE_SETTLE_MS = 260;

  function escapeHTML(value) {
    return String(value || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function renderInlineMarkdown(value) {
    let html = escapeHTML(value);
    html = html.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>');
    html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
    html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');
    return html;
  }

  function renderMarkdown(value) {
    const normalized = String(value || '').replace(/\|\s+\|/g, '|\n|');
    const lines = normalized.split(/\r?\n/);
    const blocks = [];
    let paragraph = [];
    let list = [];
    let quote = [];
    let table = [];

    function flushParagraph() {
      if (paragraph.length === 0) return;
      blocks.push(`<p>${renderInlineMarkdown(paragraph.join(' '))}</p>`);
      paragraph = [];
    }

    function flushList() {
      if (list.length === 0) return;
      blocks.push(`<ul>${list.map((item) => `<li>${renderInlineMarkdown(item)}</li>`).join('')}</ul>`);
      list = [];
    }

    function flushQuote() {
      if (quote.length === 0) return;
      blocks.push(`<blockquote>${quote.map((item) => `<p>${renderInlineMarkdown(item)}</p>`).join('')}</blockquote>`);
      quote = [];
    }

    function parseTableRow(line) {
      const trimmed = line.trim();
      if (!trimmed.startsWith('|') || !trimmed.endsWith('|')) return null;
      const cells = trimmed
        .slice(1, -1)
        .split('|')
        .map((cell) => cell.trim());
      return cells.length >= 2 ? cells : null;
    }

    function isTableSeparator(cells) {
      return Array.isArray(cells) && cells.every((cell) => /^:?-{3,}:?$/.test(cell));
    }

    function flushTable() {
      if (table.length === 0) return;
      const parsed = table.map(parseTableRow).filter(Boolean);
      if (parsed.length >= 2 && isTableSeparator(parsed[1])) {
        const headers = parsed[0];
        const rows = parsed.slice(2);
        blocks.push(`<div class="table-scroll"><table><thead><tr>${headers.map((cell) => `<th>${renderInlineMarkdown(cell)}</th>`).join('')}</tr></thead><tbody>${rows.map((row) => `<tr>${row.map((cell) => `<td>${renderInlineMarkdown(cell)}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`);
      } else {
        blocks.push(`<p>${renderInlineMarkdown(table.join(' '))}</p>`);
      }
      table = [];
    }

    for (const rawLine of lines) {
      const line = rawLine.trimEnd();
      const trimmed = line.trim();
      if (!trimmed) {
        flushParagraph();
        flushList();
        flushQuote();
        flushTable();
        continue;
      }

      const heading = trimmed.match(/^(#{1,4})\s+(.+)$/);
      if (heading) {
        flushParagraph();
        flushList();
        flushQuote();
        flushTable();
        const level = heading[1].length;
        blocks.push(`<h${level}>${renderInlineMarkdown(heading[2])}</h${level}>`);
        continue;
      }

      if (parseTableRow(trimmed)) {
        flushParagraph();
        flushList();
        flushQuote();
        table.push(trimmed);
        continue;
      }

      const bullet = trimmed.match(/^[-*]\s+(.+)$/);
      if (bullet) {
        flushParagraph();
        flushQuote();
        flushTable();
        list.push(bullet[1]);
        continue;
      }

      const quoteLine = trimmed.match(/^>\s?(.*)$/);
      if (quoteLine) {
        flushParagraph();
        flushList();
        flushTable();
        quote.push(quoteLine[1]);
        continue;
      }

      flushList();
      flushQuote();
      flushTable();
      paragraph.push(trimmed);
    }

    flushParagraph();
    flushList();
    flushQuote();
    flushTable();
    return blocks.join('\n') || '<p class="empty-doc">Blank document.</p>';
  }

  function serializeInlineMarkdown(node) {
    if (!node) return '';
    if (node.nodeType === Node.TEXT_NODE) {
      return (node.textContent || '').replace(/\u00a0/g, ' ');
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return '';

    const tag = node.tagName.toLowerCase();
    if (tag === 'br') return '\n';
    const childText = Array.from(node.childNodes).map(serializeInlineMarkdown).join('');
    if (!childText) return '';
    if (tag === 'strong' || tag === 'b') return `**${childText}**`;
    if (tag === 'em' || tag === 'i') return `*${childText}*`;
    if (tag === 'code') return `\`${childText}\``;
    if (tag === 'a') {
      const href = node.getAttribute('href') || '';
      return href ? `[${childText}](${href})` : childText;
    }
    return childText;
  }

  function serializeBlockMarkdown(node) {
    if (!node) return '';
    if (node.nodeType === Node.TEXT_NODE) {
      return (node.textContent || '').replace(/\u00a0/g, ' ');
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return '';

    const tag = node.tagName.toLowerCase();
    if (/^h[1-4]$/.test(tag)) {
      return `${'#'.repeat(Number(tag.slice(1)))} ${serializeInlineMarkdown(node).trim()}`;
    }
    if (tag === 'ul') {
      return Array.from(node.children)
        .filter((child) => child.tagName?.toLowerCase() === 'li')
        .map((child) => `- ${serializeInlineMarkdown(child).trim()}`)
        .join('\n');
    }
    if (tag === 'ol') {
      return Array.from(node.children)
        .filter((child) => child.tagName?.toLowerCase() === 'li')
        .map((child, index) => `${index + 1}. ${serializeInlineMarkdown(child).trim()}`)
        .join('\n');
    }
    if (tag === 'blockquote') {
      return Array.from(node.children)
        .map((child) => `> ${serializeInlineMarkdown(child).trim()}`)
        .join('\n');
    }
    if (tag === 'table') {
      const rows = Array.from(node.querySelectorAll('tr')).map((row) =>
        `| ${Array.from(row.children).map((cell) => serializeInlineMarkdown(cell).trim()).join(' | ')} |`
      );
      if (rows.length > 1) {
        const width = Array.from(node.querySelectorAll('tr:first-child > *')).length || 1;
        rows.splice(1, 0, `| ${Array.from({ length: width }).map(() => '---').join(' | ')} |`);
      }
      return rows.join('\n');
    }
    return serializeInlineMarkdown(node).trimEnd();
  }

  function serializeEditorMarkdown(root) {
    if (!root) return '';
    const blocks = Array.from(root.childNodes)
      .map(serializeBlockMarkdown)
      .map((block) => block.trimEnd())
      .filter((block) => block.trim() !== '');
    if (blocks.length > 0) {
      return blocks.join('\n\n');
    }
    return (root.innerText || '').replace(/\u00a0/g, ' ').trimEnd();
  }

  function syncEditorSurface(html, { force = false } = {}) {
    if (!editorSurface || (surfaceFocused && !force)) return;
    if (editorSurface.innerHTML !== html) {
      editorSurface.innerHTML = html;
    }
  }

  function normalizeTitle(ctx) {
    if (ctx?.windowTitle) return ctx.windowTitle;
    if (ctx?.fileName) return ctx.fileName;
    if (ctx?.sourcePath) {
      const bits = ctx.sourcePath.split('/');
      return bits[bits.length - 1] || 'VText';
    }
    return 'VText';
  }

  function publishWindowContext(nextContext = {}, title = '') {
    const merged = {
      ...(appContext || {}),
      ...(nextContext || {}),
    };
    appContext = merged;
    initializedKey = getContextKey(merged);
    dispatch('contextchange', {
      windowId,
      appContext: merged,
      title: title || merged.windowTitle || normalizeTitle(merged),
    });
  }

  function publishCurrentDocumentContext(title = '') {
    if (!currentDoc?.doc_id) return;
    publishWindowContext({
      docId: currentDoc.doc_id,
      windowTitle: title || currentDoc.title || appContext.windowTitle || 'VText',
      createInitialVersion: false,
      initialContent: '',
      seedPrompt: '',
    }, title || currentDoc.title || appContext.windowTitle || 'VText');
  }

  function getAuthorLabel() {
    return currentUser?.email || 'unknown';
  }

  function getContextKey(ctx) {
    const key = {
      allowMultiple: !!ctx?.allowMultiple,
      docId: ctx?.docId || '',
      sourcePath: ctx?.sourcePath || '',
      fileName: ctx?.fileName || '',
      windowTitle: ctx?.windowTitle || '',
      initialContent: ctx?.initialContent || '',
      seedPrompt: ctx?.seedPrompt || '',
      sourceUrl: ctx?.sourceUrl || '',
      sourceContentId: ctx?.sourceContentId || '',
      appHint: ctx?.appHint || '',
      createdFrom: ctx?.createdFrom || '',
      createInitialVersion: !!ctx?.createInitialVersion,
      publishedRoutePath: ctx?.publishedRoutePath || '',
      publishedGuest: !!ctx?.publishedGuest,
      startPublishedDerivative: !!ctx?.startPublishedDerivative,
    };
    return JSON.stringify(key);
  }

  function shouldShowRecentLanding(ctx) {
    return !ctx?.publishedRoutePath &&
      !ctx?.docId &&
      !ctx?.sourcePath &&
      !ctx?.initialContent &&
      !ctx?.seedPrompt &&
      !ctx?.createInitialVersion;
  }

  function formatDocTime(value) {
    if (!value) return 'unknown';
    try {
      return new Date(value).toLocaleString([], {
        month: 'short',
        day: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
      });
    } catch {
      return 'unknown';
    }
  }

  function buildFilePath(sourcePath) {
    if (!sourcePath) return '';
    return '/api/files/' + sourcePath.split('/').map(encodeURIComponent).join('/');
  }

  function isVTextShortcutPath(sourcePath) {
    return typeof sourcePath === 'string' && sourcePath.toLowerCase().endsWith('.vtext');
  }

  function sortRevisionsChronologically(items) {
    const byId = new Map();
    for (const item of items || []) {
      if (item?.revision_id) {
        byId.set(item.revision_id, item);
      }
    }

    const fallbackCompare = (left, right) => {
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

    const ordered = [];
    const visited = new Set();
    function visit(item) {
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

  function buildRevisionMetadata() {
    const metadata = {
      source_path: appContext.sourcePath || '',
      seed_prompt: appContext.seedPrompt || '',
      conductor_loop_id: appContext.conductorLoopId || '',
    };
    if (appContext.sourceUrl) metadata.source_url = appContext.sourceUrl;
    if (appContext.sourceContentId) metadata.source_content_id = appContext.sourceContentId;
    if (appContext.appHint) metadata.app_hint = appContext.appHint;
    if (appContext.createdFrom) metadata.created_from = appContext.createdFrom;
    if (publishedBundle?.publication?.id) {
      metadata.source_publication_id = publishedBundle.publication.id;
      metadata.source_publication_version_id = publishedBundle.version?.id || '';
      metadata.transclusions = publishedTransclusions;
    }
    return metadata;
  }

  function titleForPublishedBundle(bundle = publishedBundle) {
    return bundle?.publication?.title || 'Published VText';
  }

  function publicURLForPublishResult(result = publishResult) {
    const direct = String(result?.public_url || '').trim();
    if (direct) return direct;
    const routePath = String(result?.route_path || '').trim();
    if (!routePath) return '';
    if (/^https?:\/\//.test(routePath)) return routePath;
    if (typeof window === 'undefined' || !window.location) return routePath;
    return `${window.location.origin}${routePath.startsWith('/') ? routePath : `/${routePath}`}`;
  }

  function openPublishedURL(result = publishResult) {
    const publicURL = publicURLForPublishResult(result);
    if (!publicURL || typeof window === 'undefined') return false;
    const opened = window.open(publicURL, '_blank', 'noopener,noreferrer');
    return !!opened;
  }

  function truncateText(value, max = 360) {
    const text = String(value || '').trim();
    if (text.length <= max) return text;
    return `${text.slice(0, max - 1).trimEnd()}…`;
  }

  function shortHash(value) {
    const text = String(value || '');
    if (text.length <= 18) return text;
    return `${text.slice(0, 10)}…${text.slice(-6)}`;
  }

  function buildPublishedTransclusionRef(bundle = publishedBundle) {
    if (!bundle?.publication?.id || !bundle?.version?.id) return null;
    const firstSpan = bundle.retrieval?.spans?.[0] || null;
    const firstBlock = bundle.artifact?.render_model?.[0] || null;
    const selector = firstSpan?.selector || {
      kind: 'document',
      route_path: bundle.route?.path || publishedRoutePath || appContext.publishedRoutePath || '',
    };
    return {
      source_kind: firstSpan?.id ? 'published_vtext_span' : 'publication_version',
      publication_id: bundle.publication.id,
      publication_version_id: bundle.version.id,
      span_id: firstSpan?.id || firstBlock?.span_id || '',
      content_hash: bundle.version?.content_hash || '',
      selector,
      snapshot_text: truncateText(firstSpan?.snippet || firstBlock?.text || bundle.artifact?.content || '', 720),
    };
  }

  function derivativeContentForPublished(bundle = publishedBundle) {
    const title = titleForPublishedBundle(bundle);
    const source = String(bundle?.artifact?.content || '').trim();
    const quoted = (source || 'Blank published VText.')
      .split(/\r?\n/)
      .map((line) => `> ${line}`)
      .join('\n');
    return `# My version of ${title}\n\n${quoted}\n\n## Notes\n\n`;
  }

  function publishedCitationPayload(ref) {
    if (!ref) return [];
    return [{
      kind: 'published_vtext_span',
      title: titleForPublishedBundle(),
      publication_id: ref.publication_id,
      publication_version_id: ref.publication_version_id,
      span_id: ref.span_id,
      content_hash: ref.content_hash,
      selector: ref.selector,
    }];
  }

  function requestPublishedEditAuth() {
    dispatch('authrequired', {
      kind: 'published_vtext_edit',
      routePath: publishedRoutePath || appContext.publishedRoutePath || '',
      title: titleForPublishedBundle(),
    });
  }

  function hasAppAgentRevision() {
    return revisions.some((rev) => rev.author_kind === 'appagent') || currentRevision?.author_kind === 'appagent';
  }

  function synthStatusLabel() {
    return hasAppAgentRevision() ? 'Revising…' : 'Writing first draft…';
  }

  function clearNewVersionIndicator() {
    pendingHeadRevisionId = '';
    newVersionAvailable = false;
  }

  function closeDocumentStream() {
    if (streamSource) {
      streamSource.close();
      streamSource = null;
    }
    streamDocId = '';
  }

  function connectDocumentStream(docId) {
    if (!docId) return;
    if (streamSource && streamDocId === docId) return;
    closeDocumentStream();
    streamDocId = docId;
    streamSource = openDocumentStream(docId, {
      onEvent: (event) => {
        void handleDocumentStreamEvent(event);
      },
      onError: () => {
        // EventSource retries automatically. Each reconnect receives a fresh
        // snapshot from the server, which re-synchronizes the editor.
      },
    });
  }

  async function refreshRevisions(docId, preferredRevisionId = '') {
    const listed = await listRevisions(docId);
    const ordered = sortRevisionsChronologically(listed.revisions || []);
    revisions = ordered;

    if (ordered.length === 0) {
      activeRevisionIndex = -1;
      currentRevision = null;
      return;
    }

    let nextIndex = ordered.length - 1;
    if (preferredRevisionId) {
      const found = ordered.findIndex((rev) => rev.revision_id === preferredRevisionId);
      if (found >= 0) {
        nextIndex = found;
      }
    }

    await loadRevisionAt(nextIndex);
  }

  async function loadRevisionAt(index) {
    if (index < 0 || index >= revisions.length) return;
    const summary = revisions[index];
    const revision = await getRevision(summary.revision_id);
    currentRevision = revision;
    activeRevisionIndex = index;
    editorValue = revision.content || '';
    lastAutosavedContent = editorValue;
    const knownHeadId = latestHeadRevisionId || currentDoc?.current_revision_id || '';
    if (summary.revision_id === knownHeadId) {
      clearNewVersionIndicator();
    }
  }

  async function writeThroughToFile(content) {
    if (!appContext.sourcePath) return;
    if (isVTextShortcutPath(appContext.sourcePath)) return;
    const filePath = buildFilePath(appContext.sourcePath);
    const fileRes = await fetchWithRenewal(filePath, {
      method: 'PUT',
      headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      body: content,
    });
    if (!fileRes.ok) {
      const body = await fileRes.json().catch(() => ({}));
      throw new Error(body.error || `File save failed (${fileRes.status})`);
    }
  }

  async function ensureFileManifest() {
    if (!currentDoc?.doc_id || appContext.sourcePath) return;
    const manifest = await ensureDocumentManifest(currentDoc.doc_id);
    if (!manifest?.source_path) return;
    const bits = manifest.source_path.split('/');
    appContext = {
      ...appContext,
      sourcePath: manifest.source_path,
      fileName: appContext.fileName || bits[bits.length - 1] || '',
    };
    initializedKey = getContextKey(appContext);
    dispatch('contextchange', {
      windowId,
      appContext,
      title: appContext.windowTitle || appContext.fileName || 'VText',
    });
  }

  async function reloadDocument(preferredRevisionId = '') {
    currentDoc = await getDocument(currentDoc.doc_id);
    latestHeadRevisionId = currentDoc.current_revision_id || latestHeadRevisionId;
    await refreshRevisions(currentDoc.doc_id, preferredRevisionId);
  }

  async function ensureCurrentRevisionSaved(statusPrefix = 'Saving user version…') {
    if (!currentDoc) return null;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'save_vtext', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return null;
    }
    if (autosavePromise) {
      saveStatus = 'Finishing draft save...';
      await autosavePromise;
    }
    if (!currentRevision || editorValue !== (currentRevision.content || '')) {
      saveStatus = statusPrefix;
      await writeThroughToFile(editorValue);
      return saveUserVersion();
    }
    return currentRevision;
  }

  async function saveUserVersion() {
    const revision = await createRevision(currentDoc.doc_id, {
      content: editorValue,
      authorKind: 'user',
      authorLabel: getAuthorLabel(),
      metadata: buildRevisionMetadata(),
      parentRevisionId: currentRevision?.revision_id || '',
      allowRebase: true,
    });

    await reloadDocument(revision.revision_id);
    return revision;
  }

  function clearAutosaveTimer() {
    if (!autosaveTimer) return;
    clearTimeout(autosaveTimer);
    autosaveTimer = null;
  }

  function shouldAutosave() {
    if (!authenticated || !currentDoc || loading || submitting || agentPending || isViewingHistorical || isPublishedReadOnly) return false;
    const savedContent = currentRevision?.content || '';
    if (editorValue === savedContent || editorValue === lastAutosavedContent) return false;
    if (!currentRevision && editorValue.trim() === '') return false;
    return true;
  }

  async function recordSavedRevision(revision, contentAtSave) {
    currentDoc = await getDocument(currentDoc.doc_id);
    latestHeadRevisionId = revision.revision_id;

    const nextRevision = {
      ...revision,
      created_at: revision.created_at || new Date().toISOString(),
    };
    const existing = revisions.filter((item) => item.revision_id !== revision.revision_id);
    revisions = sortRevisionsChronologically([...existing, nextRevision]);
    activeRevisionIndex = revisions.findIndex((item) => item.revision_id === revision.revision_id);
    currentRevision = revision;
    lastAutosavedContent = contentAtSave;
    clearNewVersionIndicator();

    if (editorValue === contentAtSave) {
      editorValue = revision.content || '';
      if (revision.content !== contentAtSave) {
        syncEditorSurface(renderMarkdown(editorValue), { force: true });
      }
    }
  }

  async function autosaveUserDraft() {
    autosaveTimer = null;
    if (!shouldAutosave()) return;
    if (autosaveInFlight) {
      autosaveQueued = true;
      return;
    }

    autosaveInFlight = true;
    const contentAtSave = editorValue;
    saveStatus = 'Saving draft...';

    try {
      await writeThroughToFile(contentAtSave);
      const revision = await createRevision(currentDoc.doc_id, {
        content: contentAtSave,
        authorKind: 'user',
        authorLabel: getAuthorLabel(),
        metadata: {
          ...buildRevisionMetadata(),
          autosaved: true,
        },
        parentRevisionId: currentRevision?.revision_id || '',
        allowRebase: true,
      });
      await recordSavedRevision(revision, contentAtSave);
      saveStatus = 'Saved';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Autosave failed';
      saveStatus = 'Autosave failed';
    } finally {
      autosaveInFlight = false;
      if (autosaveQueued) {
        autosaveQueued = false;
        scheduleAutosave();
      }
    }
  }

  function scheduleAutosave() {
    clearAutosaveTimer();
    if (!shouldAutosave()) return;
    saveStatus = 'Unsaved changes';
    autosaveTimer = setTimeout(() => {
      const promise = autosaveUserDraft();
      autosavePromise = promise;
      promise.finally(() => {
        if (autosavePromise === promise) {
          autosavePromise = null;
        }
      });
    }, AUTOSAVE_DELAY_MS);
  }

  async function applyHeadChange(revisionId) {
    if (!currentDoc || !revisionId) return;
    if (currentRevision?.revision_id === revisionId) {
      clearNewVersionIndicator();
      return;
    }

    const shouldAutoAdvance = !isViewingHistorical && !isDirty;
    if (!shouldAutoAdvance) {
      pendingHeadRevisionId = revisionId;
      newVersionAvailable = true;
      saveStatus = 'New version available';
      return;
    }

    const hadAgentVersionBefore = hasAppAgentRevision();
    await reloadDocument(revisionId);
    clearNewVersionIndicator();
    saveStatus = hadAgentVersionBefore ? 'Agent created next version' : 'First draft ready';
    if (appContext.sourcePath) {
      await writeThroughToFile(editorValue);
    }
  }

  async function handleDocumentStreamEvent(event) {
    if (!event || event.doc_id !== currentDoc?.doc_id) return;

    switch (event.kind) {
      case 'snapshot':
        latestHeadRevisionId = event.current_revision_id || latestHeadRevisionId;
        agentPending = !!event.pending;
        if (agentPending) {
          saveStatus = synthStatusLabel();
        }
        if (latestHeadRevisionId && currentRevision?.revision_id !== latestHeadRevisionId) {
          await applyHeadChange(latestHeadRevisionId);
        }
        return;
      case 'synth_started':
        agentPending = true;
        error = '';
        saveStatus = synthStatusLabel();
        return;
      case 'synth_completed':
        agentPending = false;
        return;
      case 'revision_created':
        latestHeadRevisionId = event.current_revision_id || event.revision_id || latestHeadRevisionId;
        return;
      case 'head_changed':
        latestHeadRevisionId = event.current_revision_id || event.revision_id || latestHeadRevisionId;
        agentPending = false;
        await applyHeadChange(latestHeadRevisionId);
        return;
      case 'synth_failed':
        agentPending = false;
        error = event.error || 'Agent revision failed';
        saveStatus = 'Revision failed';
        return;
      default:
        return;
    }
  }

  async function loadContext() {
    loading = true;
    submitting = false;
    agentPending = false;
    error = '';
    saveStatus = '';
    currentDoc = null;
    currentRevision = null;
    revisions = [];
    activeRevisionIndex = -1;
    editorValue = '';
    lastAutosavedContent = '';
    latestHeadRevisionId = '';
    showRecent = false;
    surfaceFocused = false;
    toolbarHidden = false;
    lastDocumentScrollTop = 0;
    toolbarHideSettleUntil = 0;
    clearAutosaveTimer();
    clearNewVersionIndicator();
    closeDocumentStream();

    try {
      publishedBundle = null;
      publishedRoutePath = '';
      publishedDerivativeActive = false;
      publishedTransclusions = [];
      publishedProposal = null;
      publishedActionPending = false;
      publishResult = null;

      if (shouldShowRecentLanding(appContext)) {
        if (!authenticated) {
          loadGuestDocument();
          return;
        }
        showRecent = true;
        saveStatus = 'Recent VTexts';
        await loadRecentDocuments();
        return;
      }

      if (appContext.publishedRoutePath) {
        await loadPublishedContext(appContext.publishedRoutePath);
        return;
      }

      const initialValue = appContext.initialContent ?? appContext.seedPrompt ?? '';

      if (!authenticated) {
        loadGuestDocument(initialValue);
        return;
      }

      if (appContext.docId) {
        currentDoc = await getDocument(appContext.docId);
        latestHeadRevisionId = currentDoc.current_revision_id || '';
        await refreshRevisions(currentDoc.doc_id);
        if (revisions.length === 0) {
          editorValue = initialValue || '';
          saveStatus = initialValue ? 'Loaded document content' : 'Blank document ready';
        } else {
          saveStatus = 'Document loaded';
        }
      } else {
        currentDoc = await createDocument(normalizeTitle(appContext));
        editorValue = initialValue || '';

        if (appContext.createInitialVersion && initialValue) {
          const initialRevision = await createRevision(currentDoc.doc_id, {
            content: initialValue,
            authorKind: 'user',
            authorLabel: getAuthorLabel(),
            metadata: {
              ...buildRevisionMetadata(),
              created_from: appContext.createdFrom || 'conductor',
            },
          });
          await reloadDocument(initialRevision.revision_id);
          saveStatus = 'Created v0';
        } else {
          saveStatus = initialValue ? 'Loaded document content' : 'Blank document ready';
        }
      }

      if (currentDoc?.doc_id) {
        await ensureFileManifest();
        publishCurrentDocumentContext(normalizeTitle(appContext));
        connectDocumentStream(currentDoc.doc_id);
      }
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to initialize VText';
    } finally {
      loading = false;
    }
  }

  function loadGuestDocument(initialValue = '') {
    const content = initialValue || previewVTextDocument.content;
    currentDoc = {
      doc_id: previewVTextDocument.doc_id,
      title: normalizeTitle(appContext) || previewVTextDocument.title,
      current_revision_id: previewVTextDocument.revisions[previewVTextDocument.revisions.length - 1].revision_id,
    };
    revisions = previewVTextDocument.revisions.map((revision, index) => ({
      revision_id: revision.revision_id,
      content: index === previewVTextDocument.revisions.length - 1 ? content : `${content}\n\nPreview ${revision.label}: ${revision.summary}`,
      author_kind: index === 1 ? 'agent' : 'user',
      author_label: index === 1 ? 'Preview agent' : 'Local preview',
      created_at: new Date(Date.now() - (previewVTextDocument.revisions.length - index) * 90_000).toISOString(),
      metadata: { summary: revision.summary, preview: true },
    }));
    activeRevisionIndex = revisions.length - 1;
    currentRevision = revisions[activeRevisionIndex];
    editorValue = currentRevision.content || content;
    latestHeadRevisionId = currentRevision.revision_id;
    lastAutosavedContent = editorValue;
    showRecent = false;
    saveStatus = 'Local preview - sign in to save';
    publishCurrentDocumentContext(currentDoc.title);
    tick().then(() => syncEditorSurface(renderMarkdown(editorValue), { force: true }));
  }

  async function loadPublishedContext(routePath) {
    const bundle = await resolvePublication(routePath);
    publishedBundle = bundle;
    publishedRoutePath = bundle.route?.path || routePath;
    editorValue = bundle.artifact?.content || '';
    lastAutosavedContent = editorValue;
    const ref = buildPublishedTransclusionRef(bundle);
    publishedTransclusions = ref ? [ref] : [];
    saveStatus = currentUser ? 'Published VText loaded' : 'Guest published VText';

    if (appContext.startPublishedDerivative && currentUser) {
      await createPublishedDerivative({ auto: true });
    }
  }

  async function loadRecentDocuments() {
    recentLoading = true;
    error = '';
    try {
      const response = await listDocuments();
      recentDocuments = response.documents || [];
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to load recent VTexts';
    } finally {
      recentLoading = false;
    }
  }

  async function createPublishedDerivative({ auto = false } = {}) {
    if (!publishedBundle) return;
    if (!currentUser) {
      requestPublishedEditAuth();
      return;
    }
    if (publishedDerivativeActive && currentDoc) return;

    publishedActionPending = true;
    error = '';
    saveStatus = auto ? 'Preparing private version...' : 'Creating private version...';
    try {
      const ref = buildPublishedTransclusionRef(publishedBundle);
      publishedTransclusions = ref ? [ref] : [];
      const title = `My version of ${titleForPublishedBundle()}`;
      currentDoc = await createDocument(title);
      editorValue = derivativeContentForPublished(publishedBundle);
      const revision = await createRevision(currentDoc.doc_id, {
        content: editorValue,
        authorKind: 'user',
        authorLabel: getAuthorLabel(),
        citations: publishedCitationPayload(ref),
        metadata: {
          ...buildRevisionMetadata(),
          created_from: 'published_vtext_derivative',
          source_route_path: publishedRoutePath || appContext.publishedRoutePath || '',
        },
      });
      publishedDerivativeActive = true;
      await reloadDocument(revision.revision_id);
      await ensureFileManifest();
      publishCurrentDocumentContext(title);
      connectDocumentStream(currentDoc.doc_id);
      saveStatus = auto ? 'Private version ready' : 'Private version created';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to create private version';
      saveStatus = 'Private version failed';
    } finally {
      publishedActionPending = false;
    }
  }

  async function handleOpenRecent(doc) {
    if (!doc?.doc_id) return;
    publishWindowContext({
      docId: doc.doc_id,
      windowTitle: doc.title || 'VText',
      createInitialVersion: false,
    }, doc.title || 'VText');
    await loadContext();
  }

  async function handleNewDocument() {
    if (!authenticated) {
      loadGuestDocument('');
      saveStatus = 'New local preview - sign in to save';
      return;
    }
    loading = true;
    clearAutosaveTimer();
    showRecent = false;
    surfaceFocused = false;
    toolbarHidden = false;
    lastDocumentScrollTop = 0;
    toolbarHideSettleUntil = 0;
    error = '';
    try {
      currentDoc = await createDocument('Untitled VText');
      latestHeadRevisionId = currentDoc.current_revision_id || '';
      revisions = [];
      activeRevisionIndex = -1;
      currentRevision = null;
      editorValue = '';
      lastAutosavedContent = '';
      saveStatus = 'Blank document ready';
      await ensureFileManifest();
      publishCurrentDocumentContext('Untitled VText');
      connectDocumentStream(currentDoc.doc_id);
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to create VText';
      showRecent = true;
    } finally {
      loading = false;
    }
  }

  async function handlePrompt() {
    if (!currentDoc || loading || submitting || agentPending) return;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'save_vtext', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }

    submitting = true;
    clearAutosaveTimer();
    error = '';
    saveStatus = 'Saving user version…';

    try {
      await ensureCurrentRevisionSaved('Saving user version…');
      saveStatus = 'Submitting revise event…';
      await submitAgentRevision(currentDoc.doc_id, {
        intent: 'revise',
      });
      agentPending = true;
      saveStatus = synthStatusLabel();
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to prompt VText';
      saveStatus = 'Prompt failed';
      agentPending = false;
    } finally {
      submitting = false;
    }
  }

  async function handleCancelRevision() {
    if (!currentDoc || !agentPending || cancelPending) return;
    cancelPending = true;
    error = '';
    saveStatus = 'Cancelling revision…';
    try {
      await cancelAgentRevision(currentDoc.doc_id);
      agentPending = false;
      pendingHeadRevisionId = '';
      saveStatus = 'Revision cancelled. You can revise again from the current version.';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to cancel revision';
      saveStatus = 'Cancel failed';
    } finally {
      cancelPending = false;
    }
  }

  async function handlePublishCurrent() {
    if (!currentDoc || isPublishedMode || loading || submitting || agentPending || publishedActionPending) return;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'publish_vtext', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }
    publishedActionPending = true;
    error = '';
    publishResult = null;
    try {
      const revision = await ensureCurrentRevisionSaved('Saving selected revision...');
      if (!revision?.revision_id) {
        throw new Error('No revision is available to publish');
      }
      saveStatus = `Publishing ${versionLabel}...`;
      publishResult = await publishVText(currentDoc.doc_id, {
        revisionId: revision.revision_id,
      });
      const opened = openPublishedURL(publishResult);
      saveStatus = opened ? `Published ${versionLabel}; opened public link` : `Published ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to publish VText';
      saveStatus = 'Publish failed';
    } finally {
      publishedActionPending = false;
    }
  }

  async function handleCopyPublishedURL() {
    const publicURL = publicURLForPublishResult();
    if (!publicURL) return;
    try {
      await navigator.clipboard.writeText(publicURL);
      saveStatus = 'Public link copied';
    } catch (_err) {
      saveStatus = 'Could not copy public link';
    }
  }

  function handleOpenPublishedURL() {
    if (!openPublishedURL()) {
      saveStatus = 'Could not open public link';
    }
  }

  async function handleCreatePublishedDerivative() {
    await createPublishedDerivative();
  }

  async function handleSubmitProposal() {
    if (!publishedBundle) return;
    if (!currentUser) {
      requestPublishedEditAuth();
      return;
    }
    publishedActionPending = true;
    error = '';
    publishedProposal = null;
    try {
      if (!publishedDerivativeActive || !currentDoc) {
        await createPublishedDerivative({ auto: true });
      }
      if (!currentDoc) {
        throw new Error('No private version is available to propose');
      }
      const revision = await ensureCurrentRevisionSaved('Saving proposal revision...');
      if (!revision?.revision_id) {
        throw new Error('No revision is available to propose');
      }
      saveStatus = 'Submitting proposal...';
      publishedProposal = await submitPublicationProposal(publishedBundle.publication.id, {
        docId: currentDoc.doc_id,
        revisionId: revision.revision_id,
        publicationVersionId: publishedBundle.version?.id || '',
        transclusions: publishedTransclusions,
      });
      saveStatus = 'Proposal recorded for author';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to submit proposal';
      saveStatus = 'Proposal failed';
    } finally {
      publishedActionPending = false;
    }
  }

  async function handlePrevVersion() {
    if (activeRevisionIndex <= 0 || submitting) return;
    error = '';
    saveStatus = '';
    await loadRevisionAt(activeRevisionIndex - 1);
    saveStatus = `Viewing v${activeRevisionIndex}`;
  }

  async function handleNextVersion() {
    if (activeRevisionIndex < 0 || activeRevisionIndex >= revisions.length - 1 || submitting) return;
    error = '';
    saveStatus = '';
    await loadRevisionAt(activeRevisionIndex + 1);
    if (activeRevisionIndex === revisions.length - 1) {
      saveStatus = 'Viewing latest version';
    } else {
      saveStatus = `Viewing v${activeRevisionIndex}`;
    }
  }

  async function handleShowLatestVersion() {
    if (!currentDoc || !pendingHeadRevisionId) return;
    error = '';
    await reloadDocument(pendingHeadRevisionId);
    clearNewVersionIndicator();
    saveStatus = 'Viewing latest version';
  }

  function handleEditorFocus() {
    surfaceFocused = true;
  }

  function handleEditorInput() {
    editorValue = serializeEditorMarkdown(editorSurface);
    scheduleAutosave();
  }

  function handleEditorBlur() {
    surfaceFocused = false;
    syncEditorSurface(renderMarkdown(editorValue));
  }

  function handleDocumentScroll(event) {
    const scrollTop = event.currentTarget.scrollTop || 0;
    const delta = scrollTop - lastDocumentScrollTop;
    const now = Date.now();
    if (scrollTop <= TOOLBAR_HIDE_SCROLL_TOP) {
      toolbarHidden = false;
      toolbarHideSettleUntil = 0;
    } else if (delta > TOOLBAR_HIDE_SCROLL_DELTA) {
      if (!toolbarHidden) {
        toolbarHidden = true;
        toolbarHideSettleUntil = now + TOOLBAR_HIDE_SETTLE_MS;
      }
    } else if (delta < -TOOLBAR_HIDE_SCROLL_DELTA) {
      if (!toolbarHidden || now > toolbarHideSettleUntil) {
        toolbarHidden = false;
        toolbarHideSettleUntil = 0;
      }
    }
    lastDocumentScrollTop = Math.max(0, scrollTop);
  }

  $: contextKey = getContextKey(appContext);
  $: if (contextKey && contextKey !== initializedKey) {
    initializedKey = contextKey;
    loadContext();
  }

  $: isViewingHistorical = revisions.length > 0 && activeRevisionIndex !== revisions.length - 1;
  $: isDirty = !!currentDoc && !isViewingHistorical && editorValue !== (currentRevision?.content || '');
  $: versionLabel = activeRevisionIndex >= 0 ? `v${activeRevisionIndex}` : 'v0';
  $: promptLabel = submitting ? 'Submitting…' : agentPending ? 'Revising…' : 'Revise';
  $: navDisabled = loading || submitting;
  $: isPublishedMode = !!publishedBundle || !!appContext?.publishedRoutePath;
  $: isPublishedReadOnly = isPublishedMode && !publishedDerivativeActive;
  $: isEditorReadOnly = isViewingHistorical || loading || isPublishedReadOnly;
  $: renderedMarkdown = renderMarkdown(editorValue);
  $: syncEditorSurface(renderedMarkdown);

  onMount(() => {
    if (!initializedKey) {
      initializedKey = contextKey;
      loadContext();
    }
    removeLiveListener = addLiveEventListener((message) => {
      const kind = liveEventKind(message);
      if (
        showRecent &&
        (kind === 'vtext.document_revision.created' ||
          kind === 'vtext.agent_revision.completed')
      ) {
        void loadRecentDocuments();
      }
    });
  });

  onDestroy(() => {
    clearAutosaveTimer();
    closeDocumentStream();
    removeLiveListener();
  });
</script>

<div class="vtext-editor" data-vtext-editor data-vtext-doc-id={currentDoc?.doc_id || ''}>
  {#if showRecent}
    <section class="recent-panel" data-vtext-recent>
      <div class="recent-hero">
        <p class="eyebrow">VText</p>
        <h2>Recent living documents</h2>
        <p>Open an existing document, or start a clean one. Prompt-bar requests still create agentic VTexts directly.</p>
      </div>

      <div class="recent-actions">
        <button class="primary-action" data-vtext-new-document on:click={handleNewDocument} disabled={loading || recentLoading}>
          New document
        </button>
      </div>

      <div class="recent-list" data-vtext-recent-list>
        {#if recentLoading}
          <div class="recent-empty">Loading recent VTexts…</div>
        {:else if recentDocuments.length === 0}
          <div class="recent-empty">No VTexts yet.</div>
        {:else}
          {#each recentDocuments as doc (doc.doc_id)}
            <button class="recent-card" data-vtext-recent-document on:click={() => handleOpenRecent(doc)}>
              <span class="recent-title">{doc.title || 'Untitled VText'}</span>
              <span class="recent-meta">
                v{Math.max(0, (doc.revision_count || 1) - 1)}
                {#if doc.last_editor}
                  · {doc.last_editor}
                {/if}
                · {formatDocTime(doc.updated_at || doc.created_at)}
              </span>
            </button>
          {/each}
        {/if}
      </div>
    </section>
  {:else}
    <div class="doc-toolbar" class:toolbar-hidden={toolbarHidden} data-vtext-toolbar>
      <div class="version-controls">
        <span class="nav-version" data-vtext-version>{versionLabel}</span>
        <button
          class="nav-btn"
          data-vtext-prev
          aria-label={activeRevisionIndex > 0 ? `Older version (v${activeRevisionIndex - 1})` : 'At oldest version'}
          title={activeRevisionIndex > 0 ? `Go to v${activeRevisionIndex - 1}` : 'At oldest version'}
          on:click={handlePrevVersion}
          disabled={navDisabled || activeRevisionIndex <= 0}
        >
          &lt;
        </button>
        <button
          class="nav-btn"
          data-vtext-next
          aria-label={activeRevisionIndex >= 0 && activeRevisionIndex < revisions.length - 1 ? `Newer version (v${activeRevisionIndex + 1})` : 'At latest version'}
          title={activeRevisionIndex >= 0 && activeRevisionIndex < revisions.length - 1 ? `Go to v${activeRevisionIndex + 1}` : 'At latest version'}
          on:click={handleNextVersion}
          disabled={navDisabled || activeRevisionIndex < 0 || activeRevisionIndex >= revisions.length - 1}
        >
          &gt;
        </button>
      </div>

      <div class="doc-state" data-vtext-state>
        {#if isPublishedMode && !publishedDerivativeActive}
          {currentUser ? 'Published reader' : 'Guest reader'}
        {:else if isPublishedMode && publishedDerivativeActive}
          Private proposal draft
        {:else if publishResult}
          Published {versionLabel}
        {:else if isViewingHistorical}
          Historical version
        {:else if isDirty}
          Unsaved edit
        {:else if agentPending}
          {synthStatusLabel()}
        {:else}
          Latest
        {/if}
      </div>

      <div class="doc-actions">
        {#if isPublishedMode && !publishedDerivativeActive}
          <button
            class="prompt-btn"
            data-vtext-edit-published
            on:click={handleCreatePublishedDerivative}
            disabled={loading || publishedActionPending}
          >
            {publishedActionPending ? 'Opening…' : currentUser ? 'Edit my version' : 'Edit'}
          </button>
        {:else}
          <button
            class="prompt-btn"
            data-vtext-prompt
            data-vtext-save
            on:click={handlePrompt}
            disabled={loading || submitting || agentPending || isViewingHistorical || publishedActionPending}
          >
            {promptLabel}
          </button>
          {#if agentPending}
            <button
              class="secondary-action danger"
              data-vtext-cancel-revision
              on:click={handleCancelRevision}
              disabled={cancelPending}
            >
              {cancelPending ? 'Cancelling…' : 'Cancel'}
            </button>
          {/if}
          {#if isPublishedMode}
            <button
              class="secondary-action"
              data-vtext-submit-proposal
              on:click={handleSubmitProposal}
              disabled={loading || submitting || agentPending || publishedActionPending || !currentDoc}
            >
              {publishedActionPending ? 'Submitting…' : 'Propose'}
            </button>
          {:else}
            <button
              class="secondary-action"
              data-vtext-publish
              on:click={handlePublishCurrent}
              disabled={loading || submitting || agentPending || isViewingHistorical || publishedActionPending || !currentDoc}
            >
              {publishedActionPending ? 'Publishing…' : `Publish ${versionLabel}`}
            </button>
          {/if}
        {/if}
      </div>
    </div>

    <div class="document-body" data-vtext-document-body>
      {#if publishResult}
        <section
          class="publication-panel publication-result"
          data-vtext-publish-result
          data-publication-id={publishResult.publication_id || ''}
          data-publication-version-id={publishResult.publication_version_id || ''}
          data-public-route={publishResult.route_path || ''}
          data-public-url={publicURLForPublishResult(publishResult)}
        >
          <div class="publication-heading">
            <p class="eyebrow">Published</p>
            <a
              class="public-link"
              data-vtext-public-link
              href={publicURLForPublishResult(publishResult)}
              target="_blank"
              rel="noreferrer"
            >
              {publicURLForPublishResult(publishResult) || 'Public route ready'}
            </a>
          </div>
          <div class="publication-actions">
            <button type="button" class="secondary-action" data-vtext-open-public on:click={handleOpenPublishedURL}>
              Open
            </button>
            <button type="button" class="secondary-action" data-vtext-copy-public on:click={handleCopyPublishedURL}>
              Copy
            </button>
          </div>
          <div class="publication-facts">
            <span>{shortHash(publishResult.content_hash || '')}</span>
            <span>{shortHash(publishResult.publication_version_id || '')}</span>
          </div>
        </section>
      {/if}

      {#if publishedProposal}
        <section
          class="publication-panel publication-result"
          data-vtext-proposal-result
          data-proposal-id={publishedProposal.proposal_id || ''}
          data-proposal-state={publishedProposal.state || ''}
          data-delivery-state={publishedProposal.delivery_state || ''}
        >
          <div class="publication-heading">
            <p class="eyebrow">Proposal</p>
            <h2>{publishedProposal.state || 'recorded'}</h2>
          </div>
          <div class="publication-facts">
            <span>{publishedProposal.delivery_state || 'recorded_for_author'}</span>
            <span>{shortHash(publishedProposal.proposal_revision_hash || '')}</span>
          </div>
        </section>
      {/if}

      <div
        class="rendered-doc editable-doc"
        class:readonly={isEditorReadOnly}
        class:published-readonly={isPublishedReadOnly}
        data-vtext-editor-area
        data-vtext-rendered
        data-vtext-published-reader={publishedBundle ? '' : undefined}
        data-publication-id={publishedBundle?.publication?.id || undefined}
        data-publication-version-id={publishedBundle?.version?.id || undefined}
        data-content-hash={publishedBundle?.version?.content_hash || undefined}
        data-source-revision-hash={publishedBundle?.version?.source_revision_hash || undefined}
        bind:this={editorSurface}
        contenteditable={!isEditorReadOnly}
        role="textbox"
        aria-multiline="true"
        aria-label="VText document"
        spellcheck="true"
        on:focus={handleEditorFocus}
        on:input={handleEditorInput}
        on:blur={handleEditorBlur}
        on:scroll={handleDocumentScroll}
      ></div>
    </div>
  {/if}

  {#if newVersionAvailable}
    <button
      class="update-pill"
      data-vtext-new-version
      on:click={handleShowLatestVersion}
      disabled={loading}
    >
      New version available
    </button>
  {/if}

  {#if error}
    <div class="error-float" role="alert">{error}</div>
  {/if}

  <div class="sr-only" aria-live="polite" data-vtext-save-status>{saveStatus}</div>
  <div class="sr-only" aria-live="polite">{loading ? 'Loading VText…' : ''}</div>
</div>

<style>
  .vtext-editor {
    position: relative;
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
    color: #eef2ff;
    background:
      radial-gradient(circle at top right, rgba(59, 130, 246, 0.08), transparent 30%),
      rgba(9, 10, 16, 0.98);
  }

  .doc-toolbar {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.55rem;
    padding: 0.58rem 0.72rem;
    border-bottom: 1px solid rgba(148, 163, 184, 0.12);
    background: rgba(17, 24, 39, 0.58);
    max-height: 4.2rem;
    overflow: hidden;
    transition:
      opacity 180ms ease,
      transform 180ms ease,
      max-height 180ms ease,
      padding 180ms ease,
      border-color 180ms ease;
    will-change: opacity, transform, max-height;
  }

  .doc-toolbar.toolbar-hidden {
    height: 0;
    max-height: 0;
    min-height: 0;
    padding-top: 0;
    padding-bottom: 0;
    border-bottom-color: transparent;
    opacity: 0;
    pointer-events: none;
    transform: translateY(-100%);
  }

  .doc-toolbar.toolbar-hidden > * {
    visibility: hidden;
  }

  .doc-toolbar.toolbar-hidden:focus-within {
    height: auto;
    max-height: 4.2rem;
    padding-top: 0.58rem;
    padding-bottom: 0.58rem;
    border-bottom-color: rgba(148, 163, 184, 0.12);
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
  }

  .doc-toolbar.toolbar-hidden:focus-within > * {
    visibility: visible;
  }

  .version-controls,
  .doc-actions {
    display: flex;
    align-items: center;
    gap: 0.42rem;
    min-width: 0;
  }

  .doc-actions {
    justify-content: flex-end;
  }

  .doc-state {
    min-width: 0;
    text-align: center;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: rgba(203, 213, 225, 0.72);
    font-size: 0.74rem;
  }

  .document-body {
    position: relative;
    flex: 1 1 auto;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .nav-version {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 2.3rem;
    height: 1.95rem;
    padding: 0 0.6rem;
    border-radius: 999px;
    border: 1px solid rgba(148, 163, 184, 0.16);
    background: rgba(15, 23, 42, 0.72);
    color: #e2e8f0;
    font-size: 0.76rem;
    font-weight: 650;
    backdrop-filter: blur(8px);
  }

  .nav-btn,
  .prompt-btn,
  .secondary-action,
  .update-pill,
  .primary-action {
    border: 1px solid rgba(96, 165, 250, 0.28);
    background: rgba(15, 23, 42, 0.82);
    color: #e0ecff;
    cursor: pointer;
    backdrop-filter: blur(10px);
    transition: transform 120ms ease, background 120ms ease, border-color 120ms ease;
  }

  .nav-btn {
    width: 1.95rem;
    height: 1.95rem;
    border-radius: 999px;
    font-size: 0.92rem;
    font-weight: 700;
  }

  .prompt-btn {
    border-radius: 999px;
    padding: 0.62rem 0.95rem;
    font-size: 0.82rem;
    font-weight: 700;
  }

  .secondary-action {
    border-radius: 999px;
    padding: 0.62rem 0.84rem;
    font-size: 0.78rem;
    font-weight: 720;
    color: #c7d2fe;
  }

  .secondary-action.danger {
    border-color: rgba(248, 113, 113, 0.34);
    color: #fecaca;
  }

  .rendered-doc {
    flex: 1 1 auto;
    min-height: 0;
    height: auto;
    overflow: auto;
    overflow-anchor: none;
    padding: clamp(1.1rem, 2.2vw, 2rem);
    line-height: 1.72;
    color: #f8fafc;
    user-select: text;
  }

  .editable-doc {
    outline: none;
    caret-color: #bfdbfe;
  }

  .editable-doc:empty::before {
    content: "Start typing the document...";
    color: rgba(203, 213, 225, 0.45);
  }

  .editable-doc:focus {
    box-shadow: inset 0 0 0 1px rgba(96, 165, 250, 0.22);
  }

  .editable-doc.readonly {
    color: rgba(226, 232, 240, 0.82);
  }

  .editable-doc.published-readonly {
    cursor: default;
  }

  .rendered-doc :global(h1),
  .rendered-doc :global(h2),
  .rendered-doc :global(h3),
  .rendered-doc :global(h4) {
    margin: 0 0 1rem;
    line-height: 1.18;
    letter-spacing: -0.03em;
  }

  .rendered-doc :global(p),
  .rendered-doc :global(ul) {
    margin: 0 0 1rem;
  }

  .rendered-doc :global(ul) {
    padding-left: 1.25rem;
  }

  .rendered-doc :global(a) {
    color: #93c5fd;
    text-underline-offset: 0.18em;
  }

  .rendered-doc :global(code) {
    border-radius: 0.35rem;
    background: rgba(148, 163, 184, 0.14);
    padding: 0.08rem 0.3rem;
  }

  .rendered-doc :global(blockquote) {
    margin: 0 0 1rem;
    border-left: 3px solid rgba(96, 165, 250, 0.44);
    padding: 0.1rem 0 0.1rem 0.9rem;
    color: rgba(226, 232, 240, 0.86);
    background: rgba(15, 23, 42, 0.34);
  }

  .rendered-doc :global(blockquote p:last-child) {
    margin-bottom: 0;
  }

  .rendered-doc :global(.table-scroll) {
    max-width: 100%;
    margin: 0 0 1.15rem;
    overflow-x: auto;
  }

  .rendered-doc :global(table) {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.95em;
  }

  .rendered-doc :global(th),
  .rendered-doc :global(td) {
    border: 1px solid rgba(148, 163, 184, 0.22);
    padding: 0.48rem 0.58rem;
    text-align: left;
    vertical-align: top;
  }

  .rendered-doc :global(th) {
    background: rgba(59, 130, 246, 0.12);
    color: #dbeafe;
    font-weight: 800;
  }

  .rendered-doc :global(td) {
    background: rgba(15, 23, 42, 0.34);
  }

  .publication-panel {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.7rem;
    align-items: start;
    padding: 0.72rem 0.86rem;
    border-bottom: 1px solid rgba(148, 163, 184, 0.12);
    background: rgba(15, 23, 42, 0.72);
  }

  .publication-heading {
    min-width: 0;
  }

  .publication-heading h2 {
    margin: 0;
    color: #f8fafc;
    font-size: 1rem;
    line-height: 1.24;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .publication-facts {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    justify-content: flex-end;
    color: #9fb1cf;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.68rem;
  }

  .publication-facts span {
    max-width: 14rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .publication-result {
    background: rgba(20, 83, 45, 0.32);
    border-bottom-color: rgba(134, 239, 172, 0.16);
  }

  .public-link {
    display: block;
    color: #f8fafc;
    font-size: clamp(0.95rem, 2vw, 1.25rem);
    font-weight: 800;
    line-height: 1.25;
    overflow-wrap: anywhere;
    text-decoration: none;
  }

  .public-link:hover,
  .public-link:focus-visible {
    color: #bfdbfe;
    text-decoration: underline;
  }

  .publication-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
  }

  .recent-panel {
    flex: 1 1 auto;
    min-height: 0;
    display: grid;
    grid-template-rows: auto auto minmax(0, 1fr);
    gap: 1rem;
    padding: clamp(1rem, 2.5vw, 2rem);
    overflow: auto;
  }

  .recent-hero {
    max-width: 40rem;
  }

  .eyebrow {
    margin: 0 0 0.35rem;
    color: #93c5fd;
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .recent-hero h2 {
    margin: 0 0 0.45rem;
    color: #f8fafc;
    font-size: clamp(1.5rem, 4vw, 2.4rem);
    letter-spacing: -0.05em;
  }

  .recent-hero p {
    margin: 0;
    color: #a8b3c7;
    line-height: 1.5;
  }

  .recent-actions {
    display: flex;
    gap: 0.6rem;
    flex-wrap: wrap;
  }

  .primary-action {
    border-radius: 999px;
    padding: 0.62rem 0.9rem;
    font-weight: 750;
  }

  .primary-action {
    background: rgba(37, 99, 235, 0.74);
    border-color: rgba(147, 197, 253, 0.48);
  }

  .recent-list {
    display: grid;
    align-content: start;
    gap: 0.65rem;
    min-height: 0;
  }

  .recent-card,
  .recent-empty {
    border: 1px solid rgba(148, 163, 184, 0.14);
    border-radius: 16px;
    background: rgba(15, 23, 42, 0.48);
  }

  .recent-card {
    display: grid;
    gap: 0.25rem;
    width: 100%;
    padding: 0.85rem 0.95rem;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .recent-card:hover {
    border-color: rgba(96, 165, 250, 0.36);
    background: rgba(30, 41, 59, 0.66);
  }

  .recent-title {
    color: #f8fafc;
    font-weight: 780;
  }

  .recent-meta,
  .recent-empty {
    color: #94a3b8;
    font-size: 0.8rem;
  }

  .recent-empty {
    padding: 1rem;
  }

  .update-pill {
    position: absolute;
    right: 0.85rem;
    bottom: 4rem;
    z-index: 2;
    border-radius: 999px;
    padding: 0.55rem 0.9rem;
    font-size: 0.76rem;
    font-weight: 700;
    color: #bfdbfe;
  }

  .error-float {
    position: absolute;
    left: 0.85rem;
    bottom: 0.85rem;
    z-index: 2;
    max-width: min(32rem, calc(100% - 8rem));
    border-radius: 12px;
    border: 1px solid rgba(248, 113, 113, 0.32);
    background: rgba(127, 29, 29, 0.82);
    color: #fecaca;
    padding: 0.55rem 0.75rem;
    font-size: 0.75rem;
    line-height: 1.4;
    backdrop-filter: blur(10px);
  }

  .nav-btn:hover:enabled,
  .prompt-btn:hover:enabled,
  .secondary-action:hover:enabled,
  .update-pill:hover:enabled,
  .primary-action:hover:enabled {
    transform: translateY(-1px);
    background: rgba(30, 41, 59, 0.92);
    border-color: rgba(96, 165, 250, 0.42);
  }

  .nav-btn:disabled,
  .prompt-btn:disabled,
  .secondary-action:disabled,
  .update-pill:disabled,
  .primary-action:disabled {
    opacity: 0.46;
    cursor: not-allowed;
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  @media (max-width: 768px) {
    .doc-toolbar {
      grid-template-columns: auto minmax(0, 1fr) auto;
      gap: 0.42rem;
      padding: 0.46rem 0.55rem;
    }

    .version-controls,
    .doc-actions {
      gap: 0.32rem;
    }

    .doc-state {
      text-align: center;
      font-size: 0.68rem;
    }

    .rendered-doc {
      padding: 1rem;
    }

    .nav-version {
      min-width: 2.05rem;
      height: 1.78rem;
      padding: 0 0.48rem;
      font-size: 0.7rem;
    }

    .nav-btn {
      width: 1.78rem;
      height: 1.78rem;
      font-size: 0.82rem;
    }

    .prompt-btn {
      padding: 0.5rem 0.7rem;
      font-size: 0.75rem;
    }

    .secondary-action {
      padding: 0.5rem 0.64rem;
      font-size: 0.72rem;
    }

    .publication-panel {
      grid-template-columns: minmax(0, 1fr);
      gap: 0.5rem;
      padding: 0.62rem 0.7rem;
    }

    .publication-heading h2 {
      font-size: 0.92rem;
    }

    .publication-facts {
      justify-content: flex-start;
    }

    .update-pill {
      right: 0.7rem;
      bottom: 3.75rem;
    }

    .error-float {
      left: 0.7rem;
      bottom: 6.6rem;
      max-width: calc(100% - 1.4rem);
    }
  }
</style>
