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
    acceptVTextMerge,
    attachVTextSourceArtifacts,
    cancelAgentRevision,
    createContentItem,
    createDocument,
    createRevision,
    ensureDocumentManifest,
    exportPublication,
    getDocument,
    getRevision,
    getVTextDiagnosis,
    listDocuments,
    listRevisions,
    openDocumentStream,
    previewVTextMerge,
    publishVText,
    repairVTextSourceGaps,
    importContentURL,
    resolvePublication,
    restoreVTextRevision,
    semanticCompareVText,
    submitAgentRevision,
    submitPublicationProposal,
  } from './vtext.js';
  import { addLiveEventListener, liveEventKind } from './live-events.js';
  import { previewVTextDocument } from './public-preview-data';
  import { buildSourceReviewPayload } from './vtext-source-review.js';
  import VTextSourcePanel from './VTextSourcePanel.svelte';
  import { sourceEntityLaunchPayload } from './vtext-source-launcher';
  import {
    mediaRefToSourceEntity,
    publicationBundleSourceEntities as publicationBundleSourceEntitiesFromRenderer,
    sourceEntityID,
    selectorTextQuote,
    sourceEntityTargetURL,
    sourceEntityTitle,
  } from './vtext-source-renderer';
  import { renderMarkdownBlocks } from './vtext-markdown-renderer';
  import { clearSourceJournalFlows, mountSourceJournalFlow } from './vtext-source-flow';
  import './vtext-source-flow.css';

  export let currentUser = null;
  export let authenticated = false;
  export let appContext = {};
  export let windowId = '';

  const dispatch = createEventDispatcher();

  let loading = true;
  let submitting = false;
  let agentPending = false;
  let agentRunId = '';
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
  let compareResult = null;
  let compareError = '';
  let comparePending = false;
  let mergePending = false;
  let restorePending = false;
  let mergePreview = null;
  let selectedMergeSuggestionIds = [];
  let sourcePanelOpen = false;
  let sourceDiagnosis = null;
  let sourceDiagnosisPending = false;
  let sourceDiagnosisAbortController = null;
  let sourceDiagnosisAbortReason = '';
  let sourceRepairPending = false;
  let sourceRepairPayload = '';
  let sourceRepairError = '';
  let sourceReviewMarker = '';
  let sourceReviewTitle = '';
  let sourceReviewURL = '';
  let sourceReviewExcerpt = '';
  let sourceReviewStatus = '';
  let selectedSourceEntityID = '';
  let sourceArtifactTitle = '';
  let sourceArtifactURL = '';
  let sourceArtifactText = '';
  let sourceArtifactPending = false;
  let sourceArtifactStatus = '';
  let sourceArtifactError = '';
  let sourceOpenPointerHandledAt = 0;
  let sourceOpenPointerHandledEntityID = '';
  let removeLiveListener = () => {};

  const AUTOSAVE_DELAY_MS = 900;
  const TOOLBAR_HIDE_SCROLL_DELTA = 8;
  const TOOLBAR_HIDE_SCROLL_TOP = 56;
  const TOOLBAR_HIDE_SETTLE_MS = 260;
  const SOURCE_FLOW_MIN_WIDTH = 620;
  const SOURCE_FLOW_GAP = 24;
  const SOURCE_FLOW_LINE_HEIGHT = 29;
  const SOURCE_DIAGNOSIS_TIMEOUT_MS = 12000;

  function revisionMediaSourceRefs(revision = currentRevision) {
    const refs = revision?.metadata?.media_source_refs;
    return Array.isArray(refs) ? refs : [];
  }

  function revisionSourceEntities(revision = currentRevision, bundle = publishedBundle) {
    const publishedEntities = publicationBundleSourceEntities(bundle);
    if (publishedEntities.length > 0) return publishedEntities;
    const entities = revision?.metadata?.source_entities;
    if (Array.isArray(entities) && entities.length > 0) return entities;
    return revisionMediaSourceRefs(revision).map(mediaRefToSourceEntity).filter(Boolean);
  }

  function revisionSourceGaps(revision = currentRevision) {
    const gaps = revision?.metadata?.source_gaps;
    return Array.isArray(gaps) ? gaps : [];
  }

  function unresolvedCitationMarkers(content = editorValue) {
    const sourceLinked = new Set();
    for (const match of String(content || '').matchAll(/\[([^\]]+)\]\(source:[^)]+\)/g)) {
      sourceLinked.add(`[${match[1]}]`);
    }
    const markers = new Set();
    for (const match of String(content || '').matchAll(/\[(\d+)\](?!\()/g)) {
      const marker = `[${match[1]}]`;
      if (!sourceLinked.has(marker)) markers.add(marker);
    }
    return [...markers];
  }

  function sourceRepairCandidates(content = editorValue, gaps = revisionSourceGaps()) {
    const fromGaps = gaps
      .map((gap) => String(gap?.marker || '').trim())
      .filter(Boolean);
    return [...new Set([...fromGaps, ...unresolvedCitationMarkers(content)])];
  }

  function sourceDiagnosisSummary(diagnosis = sourceDiagnosis) {
    if (!diagnosis) return null;
    const revisions = Array.isArray(diagnosis.revisions) ? diagnosis.revisions : [];
    const runs = Array.isArray(diagnosis.runs) ? diagnosis.runs : [];
    const latest = revisions[0] || null;
    return {
      revisionCount: revisions.length,
      runCount: runs.length,
      latestRevisionId: latest?.revision_id || '',
      latestVersion: latest ? versionLabelForRevision(latest, revisions.length - 1) : '',
      latestAuthor: latest ? `${latest.author_kind || ''}:${latest.author_label || ''}` : '',
      errorCount: Array.isArray(diagnosis.error_matches) ? diagnosis.error_matches.length : 0,
    };
  }

  function numberMetadataValue(metadata, key) {
    const value = metadata?.[key];
    if (typeof value === 'number' && Number.isFinite(value)) return value;
    if (typeof value === 'string' && value.trim() !== '') {
      const parsed = Number(value);
      if (Number.isFinite(parsed)) return parsed;
    }
    return null;
  }

  function stringMetadataValue(metadata, key) {
    return String(metadata?.[key] || '').trim();
  }

  function revisionEditEvidence(revision) {
    const metadata = revision?.metadata || {};
    const contextMode = stringMetadataValue(metadata, 'vtext_context_mode');
    const operation = stringMetadataValue(metadata, 'vtext_edit_operation');
    const promptChars = numberMetadataValue(metadata, 'vtext_run_prompt_chars');
    const editCount = numberMetadataValue(metadata, 'vtext_edit_count');
    const deltaChars = numberMetadataValue(metadata, 'vtext_edit_delta_chars');
    const latencyMs = numberMetadataValue(metadata, 'vtext_run_latency_ms');
    if (!contextMode && !operation && promptChars === null && editCount === null && deltaChars === null && latencyMs === null) {
      return null;
    }
    return {
      revisionID: revision?.revision_id || '',
      version: typeof revision?.version_number === 'number' ? `v${revision.version_number}` : '',
      author: [revision?.author_kind, revision?.author_label].filter(Boolean).join(':'),
      contextMode,
      operation,
      promptChars,
      editCount,
      deltaChars,
      latencyMs,
    };
  }

  function sourceEditEvidence(current = currentRevision, diagnosis = sourceDiagnosis) {
    const revisions = Array.isArray(diagnosis?.revisions) ? diagnosis.revisions : [];
    const candidates = [current, ...revisions].filter(Boolean);
    const seen = new Set();
    for (const revision of candidates) {
      const revisionID = revision?.revision_id || '';
      if (revisionID && seen.has(revisionID)) continue;
      if (revisionID) seen.add(revisionID);
      const evidence = revisionEditEvidence(revision);
      if (evidence) return evidence;
    }
    return null;
  }

  function defaultSourceRepairPayload() {
    const candidates = sourceRepairCandidates();
    const existing = revisionSourceEntities();
    return {
      base_revision_id: currentRevision?.revision_id || '',
      source_entities: [],
      citation_resolutions: candidates.map((marker, index) => ({
        marker,
        entity_id: sourceEntityID(existing[index]) || '',
      })),
    };
  }

  function ensureSourceRepairPayload() {
    if (sourceRepairPayload.trim()) return;
    sourceRepairPayload = JSON.stringify(defaultSourceRepairPayload(), null, 2);
  }

  function prepareSourceReviewForm(marker = sourceRepairCandidates()[0] || sourceReviewMarker) {
    sourceReviewMarker = marker || '';
    sourceReviewTitle = '';
    sourceReviewURL = '';
    sourceReviewExcerpt = '';
    sourceReviewStatus = '';
    sourceRepairError = '';
  }

  function ensureSourceReviewSelection(candidates = sourceRepairCandidates()) {
    if (sourceReviewMarker && candidates.includes(sourceReviewMarker)) return;
    sourceReviewMarker = candidates[0] || '';
  }

  function sourceReviewPayload() {
    return buildSourceReviewPayload({
      marker: sourceReviewMarker,
      title: sourceReviewTitle,
      excerpt: sourceReviewExcerpt,
      url: sourceReviewURL,
      revisionID: currentRevision?.revision_id || '',
    });
  }

  function selectedSourceEntity() {
    return sourceEntities.find((entity) => sourceEntityID(entity) === selectedSourceEntityID) || sourceEntities[0] || null;
  }

  function prepareSourceArtifactForm(entity = selectedSourceEntity()) {
    if (!entity) return;
    selectedSourceEntityID = sourceEntityID(entity);
    sourceArtifactTitle = sourceEntityTitle(entity);
    sourceArtifactURL = sourceEntityTargetURL(entity);
    sourceArtifactText = '';
    sourceArtifactStatus = '';
    sourceArtifactError = '';
  }

  function ensureSourceArtifactSelection() {
    if (selectedSourceEntityID && sourceEntities.some((entity) => sourceEntityID(entity) === selectedSourceEntityID)) return;
    if (sourceEntities.length > 0) {
      prepareSourceArtifactForm(sourceEntities[0]);
    }
  }

  function contentItemPayloadForSource(entity, text) {
    const title = sourceArtifactTitle.trim() || sourceEntityTitle(entity);
    const sourceURL = sourceArtifactURL.trim() || sourceEntityTargetURL(entity);
    return {
      source_type: 'text',
      media_type: 'text/markdown',
      app_hint: 'content',
      title,
      source_url: sourceURL,
      canonical_url: sourceURL,
      text_content: text,
      metadata: {
        source_entity_id: sourceEntityID(entity),
        created_from: 'vtext_source_artifact_ui',
      },
      provenance: {
        rights_scope: 'public_source',
        publish_source_snapshot: true,
        untrusted_source_text: true,
      },
    };
  }

  function publicationBundleSourceEntities(bundle = publishedBundle) {
    return publicationBundleSourceEntitiesFromRenderer(bundle, publishedRoutePath, appContext);
  }

  function renderDocumentHTML(value = editorValue) {
    const entities = revisionSourceEntities();
    return renderMarkdownBlocks(value, entities, { emptyHTML: '<p class="empty-doc">Blank document.</p>' });
  }

  function serializeInlineMarkdown(node) {
    if (!node) return '';
    if (node.nodeType === Node.TEXT_NODE) {
      return (node.textContent || '').replace(/\u00a0/g, ' ');
    }
    if (node.nodeType !== Node.ELEMENT_NODE) return '';
    if (node.matches?.('[data-vtext-source-ref]')) {
      const label = node.getAttribute('data-source-label') || node.querySelector?.('.vtext-source-ref-label')?.textContent || 'source';
      const entityID = node.getAttribute('data-source-entity-id') || '';
      return entityID ? `[${label}](source:${entityID})` : label;
    }
    if (node.closest?.('[data-vtext-source-flow]')) return '';
    if (node.closest?.('[data-vtext-source-entity]')) return '';

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
    if (node.matches?.('[data-vtext-source-flow]')) return '';
    if (node.matches?.('[data-vtext-source-entity]')) return '';

    const tag = node.tagName.toLowerCase();
    if (node.matches?.('.table-scroll') && node.querySelector?.('table')) {
      return serializeBlockMarkdown(node.querySelector('table'));
    }
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
      const rows = Array.from(node.querySelectorAll('tr')).map((row) => {
        const cells = Array.from(row.children).filter((cell) => {
          const cellTag = cell.tagName?.toLowerCase();
          return cellTag === 'th' || cellTag === 'td';
        });
        return `| ${cells.map((cell) => serializeInlineMarkdown(cell).trim().replace(/\|/g, '\\|')).join(' | ')} |`;
      });
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

  function draftStorageKey(docId = currentDoc?.doc_id) {
    if (!docId) return '';
    const owner = currentUser?.id || currentUser?.email || 'guest';
    return `choir:vtext:draft:${owner}:${docId}`;
  }

  function persistLocalDraft(content, parentRevisionId = currentRevision?.revision_id || '') {
    const key = draftStorageKey();
    if (!key || typeof localStorage === 'undefined') return;
    try {
      localStorage.setItem(key, JSON.stringify({
        doc_id: currentDoc?.doc_id || '',
        parent_revision_id: parentRevisionId,
        content,
        updated_at: new Date().toISOString(),
      }));
    } catch (_err) {
      // Browser storage is a best-effort autosave cache; canonical revisions
      // are still created only through the explicit save/revise action.
    }
  }

  function clearLocalDraft(docId = currentDoc?.doc_id) {
    const key = draftStorageKey(docId);
    if (!key || typeof localStorage === 'undefined') return;
    try {
      localStorage.removeItem(key);
    } catch (_err) {
      // Ignore storage cleanup failures; the next identical draft is harmless.
    }
  }

  function loadLocalDraft(docId = currentDoc?.doc_id) {
    const key = draftStorageKey(docId);
    if (!key || typeof localStorage === 'undefined') return null;
    try {
      const raw = localStorage.getItem(key);
      return raw ? JSON.parse(raw) : null;
    } catch (_err) {
      return null;
    }
  }

  function restoreLocalDraftIfNewer() {
    const draft = loadLocalDraft();
    if (!draft || typeof draft.content !== 'string') return false;
    const savedContent = currentRevision?.content || '';
    if (draft.content === savedContent) {
      clearLocalDraft();
      return false;
    }
    editorValue = draft.content;
    lastAutosavedContent = draft.content;
    saveStatus = 'Autosaved draft restored';
    tick().then(() => syncEditorSurface(renderDocumentHTML(editorValue), { force: true }));
    return true;
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

  function revisionVersionNumber(revision, fallbackIndex = -1) {
    const value = Number(revision?.version_number);
    if (Number.isFinite(value) && value >= 0) return value;
    return Math.max(0, Number(fallbackIndex) || 0);
  }

  function versionLabelForRevision(revision, fallbackIndex = -1) {
    return `v${revisionVersionNumber(revision, fallbackIndex)}`;
  }

  function documentCurrentVersionNumber(doc = currentDoc) {
    const fromDoc = Number(doc?.current_version_number);
    if (Number.isFinite(fromDoc) && fromDoc >= 0) return fromDoc;
    const maxKnown = revisions.reduce((max, rev, index) => Math.max(max, revisionVersionNumber(rev, index)), -1);
    if (maxKnown >= 0) return maxKnown;
    const revisionCount = Number(doc?.revision_count);
    if (Number.isFinite(revisionCount) && revisionCount > 0) return revisionCount - 1;
    return 0;
  }

  function nextVersionNumber() {
    return documentCurrentVersionNumber() + 1;
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
    const nextURL = new URL(publicURL, window.location.href);
    window.history.pushState({ choirPublicRoute: nextURL.pathname }, '', nextURL);
    return true;
  }

  async function copyPublicURL(publicURL) {
    if (!publicURL) return false;
    if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) return false;
    try {
      await navigator.clipboard.writeText(publicURL);
      return true;
    } catch (_err) {
      return false;
    }
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

  function resetCompareMergeState({ keepEditor = true } = {}) {
    compareResult = null;
    compareError = '';
    mergePreview = null;
    selectedMergeSuggestionIds = [];
    if (!keepEditor && currentRevision) {
      editorValue = currentRevision.content || '';
      lastAutosavedContent = editorValue;
      tick().then(() => syncEditorSurface(renderDocumentHTML(editorValue), { force: true }));
    }
  }

  function targetRevisionForCompare() {
    if (!currentDoc?.current_revision_id) return null;
    const target = revisions.find((rev) => rev.revision_id === currentDoc.current_revision_id) || revisions[revisions.length - 1];
    return target || null;
  }

  function compareTargetVersionLabel() {
    const target = targetRevisionForCompare();
    if (!target) return 'latest';
    const index = revisions.findIndex((rev) => rev.revision_id === target.revision_id);
    return index >= 0 ? versionLabelForRevision(target, index) : 'latest';
  }

  function suggestionSelected(id) {
    return selectedMergeSuggestionIds.includes(id);
  }

  function toggleMergeSuggestion(id) {
    if (!id) return;
    if (suggestionSelected(id)) {
      selectedMergeSuggestionIds = selectedMergeSuggestionIds.filter((item) => item !== id);
    } else {
      selectedMergeSuggestionIds = [...selectedMergeSuggestionIds, id];
    }
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

  function applyDocumentWorkState(doc) {
    agentPending = !!doc?.agent_revision_pending;
    agentRunId = doc?.agent_revision_run_id || '';
    if (agentPending) {
      saveStatus = synthStatusLabel();
    }
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
    resetCompareMergeState();
    sourceRepairError = '';
    sourceRepairPayload = '';
    sourceReviewMarker = '';
    sourceReviewTitle = '';
    sourceReviewURL = '';
    sourceReviewExcerpt = '';
    sourceReviewStatus = '';
    sourceArtifactError = '';
    sourceArtifactStatus = '';
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
    if (sourcePanelOpen) {
      ensureSourceRepairPayload();
      ensureSourceArtifactSelection();
    }
  }

  async function ensureFileManifest() {
    if (!currentDoc?.doc_id || isVTextShortcutPath(appContext.sourcePath)) return;
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
    applyDocumentWorkState(currentDoc);
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

    clearLocalDraft();
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
      persistLocalDraft(contentAtSave, currentRevision?.revision_id || '');
      lastAutosavedContent = contentAtSave;
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
  }

  async function handleDocumentStreamEvent(event) {
    if (!event || event.doc_id !== currentDoc?.doc_id) return;

    switch (event.kind) {
      case 'snapshot':
        latestHeadRevisionId = event.current_revision_id || latestHeadRevisionId;
        agentPending = !!event.pending;
        agentRunId = event.loop_id || '';
        if (agentPending) {
          saveStatus = synthStatusLabel();
        }
        if (latestHeadRevisionId && currentRevision?.revision_id !== latestHeadRevisionId) {
          await applyHeadChange(latestHeadRevisionId);
        }
        return;
      case 'synth_started':
        agentPending = true;
        agentRunId = event.loop_id || agentRunId;
        error = '';
        saveStatus = synthStatusLabel();
        return;
      case 'synth_completed':
        agentPending = false;
        agentRunId = '';
        return;
      case 'revision_created':
        latestHeadRevisionId = event.current_revision_id || event.revision_id || latestHeadRevisionId;
        return;
      case 'head_changed':
        latestHeadRevisionId = event.current_revision_id || event.revision_id || latestHeadRevisionId;
        agentPending = false;
        agentRunId = '';
        await applyHeadChange(latestHeadRevisionId);
        return;
      case 'synth_failed':
        agentPending = false;
        agentRunId = '';
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
    agentRunId = '';
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
    sourcePanelOpen = false;
    sourceDiagnosis = null;
    sourceDiagnosisPending = false;
    cancelSourceDiagnosis();
    sourceRepairPending = false;
    sourceRepairPayload = '';
    sourceRepairError = '';
    sourceReviewMarker = '';
    sourceReviewTitle = '';
    sourceReviewURL = '';
    sourceReviewExcerpt = '';
    sourceReviewStatus = '';
    selectedSourceEntityID = '';
    sourceArtifactTitle = '';
    sourceArtifactURL = '';
    sourceArtifactText = '';
    sourceArtifactPending = false;
    sourceArtifactStatus = '';
    sourceArtifactError = '';
    resetCompareMergeState();
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
        applyDocumentWorkState(currentDoc);
        if (revisions.length === 0) {
          editorValue = initialValue || '';
          if (!agentPending) {
            saveStatus = initialValue ? 'Loaded document content' : 'Blank document ready';
          }
        } else if (!agentPending) {
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
        if (!agentPending && !isPublishedReadOnly) {
          restoreLocalDraftIfNewer();
        }
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
    tick().then(() => syncEditorSurface(renderDocumentHTML(editorValue), { force: true }));
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
      const response = await submitAgentRevision(currentDoc.doc_id, {
        intent: 'revise',
      });
      agentPending = true;
      agentRunId = response?.run_id || agentRunId;
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
      agentRunId = '';
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
      const copied = await copyPublicURL(publicURLForPublishResult(publishResult));
      const opened = openPublishedURL(publishResult);
      if (opened) {
        saveStatus = copied ? `Published ${versionLabel}; opened public link and copied URL` : `Published ${versionLabel}; opened public link`;
      } else {
        saveStatus = copied ? `Published ${versionLabel}; link copied` : `Published ${versionLabel}; copy link below`;
      }
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

  async function handleCompareToDraft() {
    if (!currentDoc || !currentRevision || loading || comparePending || submitting || agentPending) return;
    const target = targetRevisionForCompare();
    if (!target?.revision_id || target.revision_id === currentRevision.revision_id) {
      saveStatus = 'Choose a historical version to compare';
      return;
    }
    comparePending = true;
    error = '';
    compareError = '';
    publishResult = null;
    mergePreview = null;
    try {
      compareResult = await semanticCompareVText(currentDoc.doc_id, {
        sourceRevisionId: currentRevision.revision_id,
        targetRevisionId: target.revision_id,
      });
      selectedMergeSuggestionIds = (compareResult.suggestions || []).slice(0, 3).map((suggestion) => suggestion.id);
      saveStatus = `Comparing ${versionLabel} to ${compareTargetVersionLabel()}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      compareError = err.message || 'Failed to compare versions';
      saveStatus = 'Compare failed';
    } finally {
      comparePending = false;
    }
  }

  async function handlePreviewMerge() {
    if (!currentDoc || !currentRevision || !compareResult || mergePending) return;
    const target = targetRevisionForCompare();
    if (!target?.revision_id) return;
    mergePending = true;
    error = '';
    try {
      mergePreview = await previewVTextMerge(currentDoc.doc_id, {
        source_revision_id: compareResult.source_revision_id || currentRevision.revision_id,
        target_revision_id: target.revision_id,
        suggestion_ids: selectedMergeSuggestionIds,
        source_version_label: versionLabel,
        target_version_label: compareTargetVersionLabel(),
      });
      editorValue = mergePreview.content || editorValue;
      lastAutosavedContent = editorValue;
      await tick();
      syncEditorSurface(renderDocumentHTML(editorValue), { force: true });
      saveStatus = `${nextVersionLabel} preview`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to preview merge';
      saveStatus = 'Merge preview failed';
    } finally {
      mergePending = false;
    }
  }

  async function handleAcceptMerge() {
    if (!currentDoc || !mergePreview || mergePending) return;
    mergePending = true;
    error = '';
    try {
      const revision = await acceptVTextMerge(currentDoc.doc_id, {
        preview_id: mergePreview.preview_id,
        content: editorValue,
        source_revision_id: mergePreview.source_revision_id,
        target_revision_id: mergePreview.target_revision_id,
        suggestion_ids: (mergePreview.suggestions || []).map((suggestion) => suggestion.id),
        metadata: {
          draft_line: mergePreview.draft_line || { id: 'primary', name: 'Primary draft' },
          merge_provenance: mergePreview.provenance || {},
        },
      });
      resetCompareMergeState();
      await reloadDocument(revision.revision_id);
      saveStatus = `Accepted ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to accept merge';
      saveStatus = 'Accept failed';
    } finally {
      mergePending = false;
    }
  }

  async function handleRestoreHistoricalRevision() {
    if (!currentDoc || !currentRevision || !isViewingHistorical || restorePending) return;
    restorePending = true;
    error = '';
    saveStatus = `Restoring ${versionLabel}...`;
    try {
      const revision = await restoreVTextRevision(currentDoc.doc_id, {
        revisionId: currentRevision.revision_id,
        mode: 'restore_as_latest',
      });
      resetCompareMergeState();
      await reloadDocument(revision.revision_id);
      saveStatus = `Restored ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Failed to restore version';
      saveStatus = 'Restore failed';
    } finally {
      restorePending = false;
    }
  }

  async function handleOpenSourcePanel() {
    sourcePanelOpen = !sourcePanelOpen;
    sourceRepairError = '';
    if (!sourcePanelOpen) {
      cancelSourceDiagnosis();
      return;
    }
    if (sourcePanelOpen) {
      ensureSourceRepairPayload();
      ensureSourceReviewSelection();
    }
  }

  function cancelSourceDiagnosis(reason = 'cancelled') {
    sourceDiagnosisAbortReason = reason;
    if (sourceDiagnosisAbortController) {
      sourceDiagnosisAbortController.abort();
      sourceDiagnosisAbortController = null;
    }
  }

  function handleSourceDiagnosisButton() {
    if (sourceDiagnosisPending) {
      cancelSourceDiagnosis('cancelled');
      return;
    }
    void handleLoadSourceDiagnosis();
  }

  async function handleLoadSourceDiagnosis() {
    if (!currentDoc?.doc_id) return;
    if (sourceDiagnosisPending) {
      cancelSourceDiagnosis('cancelled');
      return;
    }
    if (!authenticated) {
      dispatch('authrequired', { kind: 'vtext_diagnosis', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }
    const controller = new AbortController();
    let timeout = null;
    sourceDiagnosisAbortController = controller;
    sourceDiagnosisAbortReason = '';
    sourceDiagnosisPending = true;
    sourceRepairError = '';
    try {
      timeout = window.setTimeout(() => {
        if (sourceDiagnosisAbortController === controller) {
          sourceDiagnosisAbortReason = 'timeout';
          controller.abort();
        }
      }, SOURCE_DIAGNOSIS_TIMEOUT_MS);
      sourceDiagnosis = await getVTextDiagnosis(currentDoc.doc_id, 80, { signal: controller.signal });
      saveStatus = 'Source diagnosis loaded';
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      if (err?.name === 'AbortError') {
        if (sourceDiagnosisAbortReason === 'timeout') {
          sourceRepairError = 'Source diagnosis timed out; source review remains available';
          saveStatus = 'Source diagnosis timed out';
        } else {
          sourceRepairError = '';
          saveStatus = 'Source diagnosis cancelled';
        }
        return;
      }
      sourceRepairError = err.message || 'Could not load source diagnosis';
      saveStatus = 'Source diagnosis failed';
    } finally {
      if (timeout) window.clearTimeout(timeout);
      if (sourceDiagnosisAbortController === controller) {
        sourceDiagnosisAbortController = null;
      }
      sourceDiagnosisAbortReason = '';
      sourceDiagnosisPending = false;
    }
  }

  async function handleApplySourceReview() {
    if (!currentDoc?.doc_id || !currentRevision?.revision_id || sourceRepairPending) return;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'vtext_source_repair', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }
    ensureSourceReviewSelection();
    const marker = String(sourceReviewMarker || '').trim();
    const title = String(sourceReviewTitle || '').trim();
    const excerpt = String(sourceReviewExcerpt || '').trim();
    if (!marker) {
      sourceRepairError = 'Choose a citation marker to repair';
      return;
    }
    if (!title) {
      sourceRepairError = 'Source title is required';
      return;
    }
    if (!excerpt) {
      sourceRepairError = 'Confirming source excerpt is required';
      return;
    }
    sourceRepairPending = true;
    sourceRepairError = '';
    sourceReviewStatus = 'Applying source review...';
    saveStatus = 'Repairing sources...';
    try {
      const payload = {
        ...sourceReviewPayload(),
        base_revision_id: currentRevision.revision_id,
        author_label: getAuthorLabel(),
      };
      sourceReviewStatus = 'Sending source review...';
      const revision = await repairVTextSourceGaps(currentDoc.doc_id, {
        ...payload,
      });
      sourceDiagnosis = null;
      sourceRepairPayload = '';
      sourceReviewStatus = `Applied source review for ${marker}`;
      sourceReviewTitle = '';
      sourceReviewURL = '';
      sourceReviewExcerpt = '';
      saveStatus = 'Loading repaired source revision...';
      await reloadDocument(revision.revision_id);
      ensureSourceRepairPayload();
      ensureSourceReviewSelection();
      saveStatus = `Repaired sources in ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      sourceRepairError = err.message || 'Source review failed';
      sourceReviewStatus = '';
      saveStatus = 'Source repair failed';
    } finally {
      sourceRepairPending = false;
    }
  }

  async function handleApplySourceRepair() {
    if (!currentDoc?.doc_id || !currentRevision?.revision_id || sourceRepairPending) return;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'vtext_source_repair', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }
    let payload;
    try {
      payload = JSON.parse(sourceRepairPayload || '{}');
    } catch (_err) {
      sourceRepairError = 'Repair payload must be valid JSON';
      return;
    }
    if (!Array.isArray(payload?.citation_resolutions) || payload.citation_resolutions.length === 0) {
      sourceRepairError = 'Repair payload needs citation_resolutions';
      return;
    }
    sourceRepairPending = true;
    sourceRepairError = '';
    saveStatus = 'Repairing sources...';
    try {
      const revision = await repairVTextSourceGaps(currentDoc.doc_id, {
        ...payload,
        base_revision_id: payload.base_revision_id || currentRevision.revision_id,
        author_label: getAuthorLabel(),
      });
      sourceDiagnosis = null;
      sourceRepairPayload = '';
      await reloadDocument(revision.revision_id);
      ensureSourceRepairPayload();
      saveStatus = `Repaired sources in ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      sourceRepairError = err.message || 'Source repair failed';
      saveStatus = 'Source repair failed';
    } finally {
      sourceRepairPending = false;
    }
  }

  async function attachContentItemToSelectedSource(item, entity = selectedSourceEntity()) {
    if (!currentDoc?.doc_id || !currentRevision?.revision_id || !sourceEntityID(entity) || !item?.content_id) {
      throw new Error('Choose a source and readable content item first');
    }
    const revision = await attachVTextSourceArtifacts(currentDoc.doc_id, {
      base_revision_id: currentRevision.revision_id,
      author_label: getAuthorLabel(),
      attachments: [{
        entity_id: sourceEntityID(entity),
        content_id: item.content_id,
        text_quote: selectorTextQuote(entity),
      }],
    });
    sourceDiagnosis = null;
    sourceRepairPayload = '';
    await reloadDocument(revision.revision_id);
    ensureSourceRepairPayload();
    return revision;
  }

  async function handleImportAndAttachSourceArtifact() {
    const entity = selectedSourceEntity();
    if (!currentDoc?.doc_id || !currentRevision?.revision_id || !entity || sourceArtifactPending) return;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'vtext_source_artifact', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }
    const sourceURL = sourceArtifactURL.trim() || sourceEntityTargetURL(entity);
    if (!sourceURL) {
      sourceArtifactError = 'Source URL is required for URL import';
      return;
    }
    sourceArtifactPending = true;
    sourceArtifactError = '';
    sourceArtifactStatus = 'Importing source URL...';
    saveStatus = 'Importing source artifact...';
    try {
      const item = await importContentURL(sourceURL, sourceArtifactTitle.trim() || sourceEntityTitle(entity));
      sourceArtifactStatus = 'Attaching imported source...';
      await attachContentItemToSelectedSource(item, entity);
      sourceArtifactStatus = `Attached imported source to ${sourceEntityTitle(entity)}`;
      saveStatus = `Attached source artifact in ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      sourceArtifactError = err.message || 'Source import failed';
      sourceArtifactStatus = '';
      saveStatus = 'Source attachment failed';
    } finally {
      sourceArtifactPending = false;
    }
  }

  async function handleCreateAndAttachSourceArtifact() {
    const entity = selectedSourceEntity();
    if (!currentDoc?.doc_id || !currentRevision?.revision_id || !entity || sourceArtifactPending) return;
    if (!authenticated) {
      dispatch('authrequired', { kind: 'vtext_source_artifact', appId: 'vtext', appName: 'VText', title: currentDoc.title });
      return;
    }
    const text = sourceArtifactText.trim();
    if (!text) {
      sourceArtifactError = 'Readable source text is required';
      return;
    }
    sourceArtifactPending = true;
    sourceArtifactError = '';
    sourceArtifactStatus = 'Creating source artifact...';
    saveStatus = 'Creating source artifact...';
    try {
      const item = await createContentItem(contentItemPayloadForSource(entity, text));
      sourceArtifactStatus = 'Attaching source artifact...';
      await attachContentItemToSelectedSource(item, entity);
      sourceArtifactText = '';
      sourceArtifactStatus = `Attached source artifact to ${sourceEntityTitle(entity)}`;
      saveStatus = `Attached source artifact in ${versionLabel}`;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      sourceArtifactError = err.message || 'Source attachment failed';
      sourceArtifactStatus = '';
      saveStatus = 'Source attachment failed';
    } finally {
      sourceArtifactPending = false;
    }
  }

  async function handleDiscardMerge() {
    const targetId = mergePreview?.target_revision_id || currentDoc?.current_revision_id || currentRevision?.revision_id || '';
    resetCompareMergeState();
    if (targetId) {
      const targetIndex = revisions.findIndex((rev) => rev.revision_id === targetId);
      if (targetIndex >= 0) {
        await loadRevisionAt(targetIndex);
      } else if (currentDoc) {
        await reloadDocument(targetId);
      }
    }
    saveStatus = 'Merge preview discarded';
  }

  async function handleCopyPublishedURL() {
    const publicURL = publicURLForPublishResult();
    if (!publicURL) return;
    saveStatus = await copyPublicURL(publicURL) ? 'Public link copied' : 'Could not copy public link';
  }

  function currentPublicationRoute() {
    return publishResult?.route_path || publishedBundle?.route?.path || publishedRoutePath || appContext?.publishedRoutePath || '';
  }

  async function handleCopyPublishedText() {
    const route = currentPublicationRoute();
    if (!route) return;
    try {
      const exported = await exportPublication(route, 'txt');
      await navigator.clipboard.writeText(exported.content || '');
      saveStatus = 'Published text copied';
    } catch (err) {
      saveStatus = err.message || 'Could not copy published text';
    }
  }

  async function handleDownloadPublished(format = 'md') {
    const route = currentPublicationRoute();
    if (!route) return;
    try {
      const exported = await exportPublication(route, format);
      const body = exported.content_base64 ? base64ToUint8Array(exported.content_base64) : (exported.content || '');
      const blob = new Blob([body], { type: exported.media_type || 'text/plain;charset=utf-8' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = exported.filename || `published-vtext.${format}`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      saveStatus = `Downloaded ${exported.format || format}`;
    } catch (err) {
      saveStatus = err.message || 'Download failed';
    }
  }

  function base64ToUint8Array(value) {
    const binary = atob(value || '');
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
    return bytes;
  }

  function handleOpenPublishedURL() {
    if (!openPublishedURL()) {
      saveStatus = 'Could not open public link';
      return;
    }
    saveStatus = 'Public link shown in address bar';
  }

  function handlePublishedLinkClick(event) {
    event.preventDefault();
    handleOpenPublishedURL();
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
    saveStatus = `Viewing ${versionLabel}`;
  }

  async function handleNextVersion() {
    if (activeRevisionIndex < 0 || activeRevisionIndex >= revisions.length - 1 || submitting) return;
    error = '';
    saveStatus = '';
    await loadRevisionAt(activeRevisionIndex + 1);
    if (activeRevisionIndex === revisions.length - 1) {
      saveStatus = 'Viewing latest version';
    } else {
      saveStatus = `Viewing ${versionLabel}`;
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
    syncEditorSurface(renderDocumentHTML(editorValue));
  }

  function handleSourceEntityOpen(entity) {
    const payload = sourceEntityLaunchPayload(entity);
    if (payload) dispatch('launchapp', payload);
  }

  function handleSourceOpenButton(button) {
    const entityID = button?.getAttribute?.('data-source-entity-id') || '';
    const entity = revisionSourceEntities().find((item) => sourceEntityID(item) === entityID);
    handleSourceEntityOpen(entity);
    return entityID;
  }

  function handleEditorClick(event) {
    const collapse = event.target?.closest?.('[data-vtext-source-flow-collapse]');
    if (collapse) {
      event.preventDefault();
      event.stopPropagation();
      editorSurface?.querySelectorAll?.('[data-vtext-source-ref][data-expanded="true"]').forEach((node) => {
        node.setAttribute('data-expanded', 'false');
      });
      clearSourceJournalFlows(editorSurface);
      return;
    }
    const button = event.target?.closest?.('[data-vtext-open-source]');
    if (button) {
      event.preventDefault();
      event.stopPropagation();
      const entityID = button.getAttribute('data-source-entity-id') || '';
      if (entityID && entityID === sourceOpenPointerHandledEntityID && Date.now() - sourceOpenPointerHandledAt < 800) {
        return;
      }
      handleSourceOpenButton(button);
      return;
    }
    const sourceRef = event.target?.closest?.('[data-vtext-source-ref]');
    if (!sourceRef) return;
    event.preventDefault();
  }

  function handleEditorKeydown(event) {
    const sourceRef = event.target?.closest?.('[data-vtext-source-ref]');
    if (sourceRef && (event.key === 'Enter' || event.key === ' ')) {
      event.preventDefault();
      event.stopPropagation();
      toggleInlineSourceRef(sourceRef);
      return;
    }
    const button = event.target?.closest?.('[data-vtext-open-source]');
    if (!button || (event.key !== 'Enter' && event.key !== ' ')) return;
    event.preventDefault();
    event.stopPropagation();
    handleSourceOpenButton(button);
  }

  function refreshSourceJournalFlow() {
    const expanded = editorSurface?.querySelector?.('[data-vtext-source-ref][data-expanded="true"]');
    if (!expanded) return;
    clearSourceJournalFlows(editorSurface);
    requestAnimationFrame(() => mountSourceJournalFlow(expanded, {
      minWidth: SOURCE_FLOW_MIN_WIDTH,
      gap: SOURCE_FLOW_GAP,
      lineHeight: SOURCE_FLOW_LINE_HEIGHT,
    }));
  }

  function renderedSourceRefForEntity(entityID) {
    if (!entityID || !editorSurface) return null;
    return Array.from(editorSurface.querySelectorAll?.('[data-vtext-source-ref]') || [])
      .find((node) => !node.closest?.('[data-vtext-source-flow]') && node.getAttribute?.('data-source-entity-id') === entityID) || null;
  }

  function expandSourceRefAsJournalFlow(sourceRef) {
    if (!sourceRef) return;
    sourceRef.setAttribute('data-expanded', 'true');
    requestAnimationFrame(() => mountSourceJournalFlow(sourceRef, {
      minWidth: SOURCE_FLOW_MIN_WIDTH,
      gap: SOURCE_FLOW_GAP,
      lineHeight: SOURCE_FLOW_LINE_HEIGHT,
    }));
  }

  function toggleInlineSourceRef(sourceRef) {
    if (!sourceRef) return;
    const flow = sourceRef.closest?.('[data-vtext-source-flow]');
    if (flow) {
      const entityID = sourceRef.getAttribute('data-source-entity-id') || '';
      const ownerID = flow.getAttribute('data-source-flow-owner-id') || '';
      clearSourceJournalFlows(editorSurface);
      editorSurface?.querySelectorAll?.('[data-vtext-source-ref][data-expanded="true"]').forEach((node) => {
        node.setAttribute('data-expanded', 'false');
      });
      if (!entityID || entityID === ownerID) return;
      expandSourceRefAsJournalFlow(renderedSourceRefForEntity(entityID));
      return;
    }
    const expanded = sourceRef.getAttribute('data-expanded') === 'true';
    clearSourceJournalFlows(editorSurface);
    editorSurface?.querySelectorAll?.('[data-vtext-source-ref][data-expanded="true"]').forEach((node) => {
      if (node !== sourceRef) node.setAttribute('data-expanded', 'false');
    });
    sourceRef.setAttribute('data-expanded', expanded ? 'false' : 'true');
    if (!expanded) {
      expandSourceRefAsJournalFlow(sourceRef);
    }
  }

  function handleEditorPointerDown(event) {
    const collapse = event.target?.closest?.('[data-vtext-source-flow-collapse]');
    if (collapse) {
      event.preventDefault();
      editorSurface?.querySelectorAll?.('[data-vtext-source-ref][data-expanded="true"]').forEach((node) => {
        node.setAttribute('data-expanded', 'false');
      });
      clearSourceJournalFlows(editorSurface);
      return;
    }
    const sourceOpen = event.target?.closest?.('[data-vtext-open-source]');
    if (sourceOpen) {
      event.preventDefault();
      event.stopPropagation();
      sourceOpenPointerHandledEntityID = handleSourceOpenButton(sourceOpen);
      sourceOpenPointerHandledAt = Date.now();
      return;
    }
    const sourceRef = event.target?.closest?.('[data-vtext-source-ref]');
    if (!sourceRef) return;
    event.preventDefault();
    toggleInlineSourceRef(sourceRef);
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
  $: versionLabel = currentRevision ? versionLabelForRevision(currentRevision, activeRevisionIndex) : `v${documentCurrentVersionNumber()}`;
  $: nextVersionLabel = `v${nextVersionNumber()}`;
  $: promptLabel = submitting ? 'Submitting…' : agentPending ? 'Revising…' : 'Revise';
  $: navDisabled = loading || submitting;
  $: isPublishedMode = !!publishedBundle || !!appContext?.publishedRoutePath;
  $: isPublishedReadOnly = isPublishedMode && !publishedDerivativeActive;
  $: isEditorReadOnly = !!mergePreview || isViewingHistorical || loading || isPublishedReadOnly;
  $: sourceGaps = revisionSourceGaps(currentRevision);
  $: sourceEntities = revisionSourceEntities(currentRevision, publishedBundle);
  $: sourceCandidates = sourceRepairCandidates(editorValue, sourceGaps);
  $: if (sourcePanelOpen) ensureSourceReviewSelection(sourceCandidates);
  $: sourceSummary = sourceDiagnosisSummary(sourceDiagnosis);
  $: editEvidence = sourceEditEvidence(currentRevision, sourceDiagnosis);
  $: if (sourcePanelOpen) ensureSourceArtifactSelection();
  $: showSourceRepairDiagnostics = !!(
    appContext?.showSourceRepairDiagnostics
    || appContext?.debugSourceRepair
  );
  $: renderedMarkdown = renderDocumentHTML(editorValue);
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
    window.addEventListener('resize', refreshSourceJournalFlow);
  });

  onDestroy(() => {
    clearAutosaveTimer();
    cancelSourceDiagnosis();
    closeDocumentStream();
    removeLiveListener();
    window.removeEventListener('resize', refreshSourceJournalFlow);
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
                v{documentCurrentVersionNumber(doc)}
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
          aria-label={activeRevisionIndex > 0 ? `Older version (${versionLabelForRevision(revisions[activeRevisionIndex - 1], activeRevisionIndex - 1)})` : 'At oldest version'}
          title={activeRevisionIndex > 0 ? `Go to ${versionLabelForRevision(revisions[activeRevisionIndex - 1], activeRevisionIndex - 1)}` : 'At oldest version'}
          on:click={handlePrevVersion}
          disabled={navDisabled || activeRevisionIndex <= 0}
        >
          &lt;
        </button>
        <button
          class="nav-btn"
          data-vtext-next
          aria-label={activeRevisionIndex >= 0 && activeRevisionIndex < revisions.length - 1 ? `Newer version (${versionLabelForRevision(revisions[activeRevisionIndex + 1], activeRevisionIndex + 1)})` : 'At latest version'}
          title={activeRevisionIndex >= 0 && activeRevisionIndex < revisions.length - 1 ? `Go to ${versionLabelForRevision(revisions[activeRevisionIndex + 1], activeRevisionIndex + 1)}` : 'At latest version'}
          on:click={handleNextVersion}
          disabled={navDisabled || activeRevisionIndex < 0 || activeRevisionIndex >= revisions.length - 1}
        >
          &gt;
        </button>
        <span class="draft-line" data-vtext-draft-line>Primary draft</span>
      </div>

      <div class="doc-state" data-vtext-state>
        {#if mergePreview}
          {nextVersionLabel} preview
        {:else if compareResult}
          Comparing to {compareTargetVersionLabel()}
        {:else if isPublishedMode && !publishedDerivativeActive}
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
            class="secondary-action"
            data-vtext-copy-full-text
            on:click={handleCopyPublishedText}
            disabled={loading || publishedActionPending}
          >
            Copy text
          </button>
          <details class="download-menu" data-vtext-download-menu>
            <summary>Download</summary>
            <button type="button" data-vtext-download-md on:click={() => handleDownloadPublished('md')} disabled={loading || publishedActionPending}>Markdown</button>
            <button type="button" data-vtext-download-txt on:click={() => handleDownloadPublished('txt')} disabled={loading || publishedActionPending}>Text</button>
            <button type="button" data-vtext-download-html on:click={() => handleDownloadPublished('html')} disabled={loading || publishedActionPending}>HTML</button>
            <button type="button" data-vtext-download-docx on:click={() => handleDownloadPublished('docx')} disabled={loading || publishedActionPending}>DOCX</button>
            <button type="button" data-vtext-download-pdf on:click={() => handleDownloadPublished('pdf')} disabled={loading || publishedActionPending}>PDF</button>
          </details>
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
            {#if mergePreview}
              <button
                class="prompt-btn"
                data-vtext-accept-merge
                on:click={handleAcceptMerge}
                disabled={mergePending || publishedActionPending || !currentDoc}
              >
                {mergePending ? 'Accepting…' : 'Accept'}
              </button>
              <button
                class="secondary-action danger"
                data-vtext-discard-merge
                on:click={handleDiscardMerge}
                disabled={mergePending || publishedActionPending}
              >
                Discard
              </button>
            {:else}
              <button
                class="secondary-action"
                data-vtext-compare
                on:click={handleCompareToDraft}
                disabled={loading || submitting || agentPending || comparePending || activeRevisionIndex < 0 || activeRevisionIndex >= revisions.length - 1}
              >
                {comparePending ? 'Comparing…' : 'Compare'}
              </button>
              <button
                class="secondary-action"
                data-vtext-source-panel
                on:click={handleOpenSourcePanel}
                disabled={loading || submitting || agentPending || !currentDoc}
              >
                Sources{sourceCandidates.length ? ` ${sourceCandidates.length}` : ''}
              </button>
              {#if isViewingHistorical}
                <button
                  class="secondary-action"
                  data-vtext-restore-version
                  on:click={handleRestoreHistoricalRevision}
                  disabled={loading || submitting || agentPending || restorePending || !currentRevision}
                >
                  {restorePending ? 'Restoring…' : 'Restore'}
                </button>
              {/if}
              {#if compareResult}
                <button
                  class="secondary-action"
                  data-vtext-merge-preview
                  on:click={handlePreviewMerge}
                  disabled={mergePending || selectedMergeSuggestionIds.length === 0}
                >
                  {mergePending ? 'Merging…' : 'Merge into draft'}
                </button>
              {/if}
            {/if}
            <button
              class="secondary-action"
              data-vtext-publish
              on:click={handlePublishCurrent}
              disabled={loading || submitting || agentPending || !!mergePreview || publishedActionPending || !currentDoc}
            >
              {publishedActionPending ? 'Publishing…' : `Publish ${versionLabel}`}
            </button>
          {/if}
        {/if}
      </div>
    </div>

    {#if agentPending}
      <div
        class="work-banner"
        data-vtext-working
        data-vtext-agent-run-id={agentRunId || undefined}
        role="status"
        aria-live="polite"
      >
        <span class="work-pulse" aria-hidden="true"></span>
        <span class="work-copy">{synthStatusLabel()}</span>
        {#if agentRunId}
          <span class="work-run">{shortHash(agentRunId)}</span>
        {/if}
      </div>
    {/if}

    <div class="document-body" data-vtext-document-body>
      {#if sourcePanelOpen}
        <VTextSourcePanel
          {currentDoc}
          {currentRevision}
          {isPublishedReadOnly}
          {sourceCandidates}
          {sourceEntities}
          {sourceSummary}
          {editEvidence}
          {sourceDiagnosisPending}
          {sourceRepairPending}
          {sourceRepairError}
          bind:sourceReviewMarker
          bind:sourceReviewTitle
          bind:sourceReviewURL
          bind:sourceReviewExcerpt
          {sourceReviewStatus}
          bind:selectedSourceEntityID
          bind:sourceArtifactTitle
          bind:sourceArtifactURL
          bind:sourceArtifactText
          {sourceArtifactPending}
          {sourceArtifactStatus}
          {sourceArtifactError}
          bind:sourceRepairPayload
          showDiagnosticRepair={showSourceRepairDiagnostics}
          on:diagnosis={handleSourceDiagnosisButton}
          on:source-entity-open={(event) => handleSourceEntityOpen(event.detail.entity)}
          on:source-review-marker={(event) => prepareSourceReviewForm(event.detail.marker)}
          on:apply-source-review={handleApplySourceReview}
          on:source-artifact-target={(event) => prepareSourceArtifactForm(event.detail.entity)}
          on:import-source-artifact={handleImportAndAttachSourceArtifact}
          on:attach-source-artifact={handleCreateAndAttachSourceArtifact}
          on:apply-source-repair={handleApplySourceRepair}
        />
      {/if}

      {#if compareResult || mergePreview || comparePending || mergePending || compareError}
        <section class="compare-panel" class:compare-panel-error={compareError && !comparePending && !mergePending} data-vtext-compare-panel>
          <div class="compare-heading">
            <div>
              <p class="eyebrow">{compareError ? 'Compare failed' : mergePreview ? 'Merge preview' : `What changed since ${versionLabel}`}</p>
              <h3>
                {#if mergePending}
                  Model merge in progress
                {:else if comparePending}
                  Model compare in progress
                {:else if compareError}
                  Could not compare {versionLabel} to {compareTargetVersionLabel()}
                {:else}
                  {mergePreview ? `Merged into ${nextVersionLabel}` : `Compare ${versionLabel} → ${compareTargetVersionLabel()}`}
                {/if}
              </h3>
            </div>
            {#if mergePreview}
              <span class="compare-chip">from {versionLabel}</span>
            {/if}
          </div>
          {#if comparePending || mergePending}
            <div class="compare-working" role="status" aria-live="polite">
              <span class="work-pulse" aria-hidden="true"></span>
              <span>{mergePending ? 'Building a reviewable merge preview with the configured VText model.' : 'Comparing versions with the configured VText model.'}</span>
            </div>
          {/if}
          {#if compareError && !comparePending && !mergePending}
            <div class="compare-error" role="alert" data-vtext-compare-error>
              <span>{compareError}</span>
              <button type="button" class="secondary-action" on:click={handleCompareToDraft}>
                Retry compare
              </button>
            </div>
          {/if}
          {#if compareResult?.summary?.length}
            <div class="compare-summary">
              {#each compareResult.summary as finding}
                <span>{finding}</span>
              {/each}
            </div>
          {/if}
          {#if compareResult?.suggestions?.length && !mergePreview}
            <div class="merge-suggestions">
              {#each compareResult.suggestions as suggestion}
                <label class="merge-suggestion" data-vtext-merge-suggestion>
                  <input
                    type="checkbox"
                    checked={suggestionSelected(suggestion.id)}
                    on:change={() => toggleMergeSuggestion(suggestion.id)}
                  />
                  <span>
                    <strong>{suggestion.label}</strong>
                    <small>{suggestion.status} · {suggestion.description}</small>
                  </span>
                </label>
              {/each}
            </div>
          {:else if mergePreview?.suggestions?.length}
            <div class="provenance-strip">
              {#each mergePreview.suggestions as suggestion}
                <span>{suggestion.label}</span>
              {/each}
            </div>
          {/if}
        </section>
      {/if}

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
              on:click={handlePublishedLinkClick}
            >
              {publicURLForPublishResult(publishResult) || 'Public route ready'}
            </a>
          </div>
          <div class="publication-actions">
            <button type="button" class="primary-action" data-vtext-copy-public on:click={handleCopyPublishedURL}>
              Copy link
            </button>
            <button type="button" class="secondary-action" data-vtext-open-public on:click={handleOpenPublishedURL}>
              Open link
            </button>
            <button type="button" class="secondary-action" data-vtext-copy-full-text on:click={handleCopyPublishedText}>
              Copy text
            </button>
            <details class="download-menu" data-vtext-download-menu>
              <summary>Download</summary>
              <button type="button" data-vtext-download-md on:click={() => handleDownloadPublished('md')}>Markdown</button>
              <button type="button" data-vtext-download-txt on:click={() => handleDownloadPublished('txt')}>Text</button>
              <button type="button" data-vtext-download-html on:click={() => handleDownloadPublished('html')}>HTML</button>
              <button type="button" data-vtext-download-docx on:click={() => handleDownloadPublished('docx')}>DOCX</button>
              <button type="button" data-vtext-download-pdf on:click={() => handleDownloadPublished('pdf')}>PDF</button>
            </details>
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
        tabindex="0"
        role="textbox"
        aria-multiline="true"
        aria-label="VText document"
        spellcheck="true"
        on:focus={handleEditorFocus}
        on:pointerdown={handleEditorPointerDown}
        on:click={handleEditorClick}
        on:keydown={handleEditorKeydown}
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
    color: var(--choir-text-accent);
    background:
      radial-gradient(circle at top right, var(--choir-state-hover), transparent 30%),
      var(--choir-state-selected);
  }

  .doc-toolbar {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.55rem;
    padding: 0.58rem 0.72rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
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
    border-bottom-color: var(--choir-border-strong);
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
    color: var(--choir-text-accent);
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

  .compare-panel {
    flex: 0 0 auto;
    display: grid;
    gap: 0.75rem;
    padding: 0.8rem 0.95rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-surface-raised);
    color: var(--choir-text-primary);
  }

  .compare-panel-error {
    border-bottom-color: var(--choir-status-danger);
  }

  .compare-heading {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.8rem;
    min-width: 0;
  }

  .compare-heading h3 {
    margin: 0.12rem 0 0;
    color: var(--choir-text-primary);
    font-size: 0.92rem;
    line-height: 1.2;
  }

  .compare-chip {
    flex: 0 0 auto;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    padding: 0.34rem 0.55rem;
    color: var(--choir-text-accent);
    font-size: 0.68rem;
    font-weight: 720;
  }

  .compare-working {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
    color: var(--choir-text-secondary);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .compare-error {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.65rem;
    color: var(--choir-text-primary);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .compare-error span {
    flex: 1 1 18rem;
    min-width: 0;
  }

  .compare-summary,
  .provenance-strip {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.42rem 0.8rem;
    color: var(--choir-text-secondary);
    font-size: 0.72rem;
    line-height: 1.35;
  }

  .merge-suggestions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.48rem;
  }

  .merge-suggestion {
    display: flex;
    align-items: flex-start;
    gap: 0.48rem;
    min-width: 0;
    padding: 0.56rem 0.62rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.5rem;
    background: var(--choir-state-selected);
    color: var(--choir-text-primary);
    cursor: pointer;
  }

  .merge-suggestion input {
    flex: 0 0 auto;
    margin-top: 0.1rem;
  }

  .merge-suggestion span {
    min-width: 0;
    display: grid;
    gap: 0.18rem;
  }

  .merge-suggestion strong {
    font-size: 0.76rem;
    line-height: 1.2;
  }

  .merge-suggestion small {
    color: var(--choir-text-secondary);
    font-size: 0.66rem;
    line-height: 1.25;
  }

  .work-banner {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: 0.55rem;
    min-height: 2.5rem;
    padding: 0.52rem 0.78rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    font-size: 0.78rem;
    font-weight: 760;
  }

  .work-pulse {
    width: 0.72rem;
    height: 0.72rem;
    flex: 0 0 auto;
    border-radius: 999px;
    background: var(--choir-state-selected);
    box-shadow: 0 0 0 0 var(--choir-state-active-glow);
    animation: work-pulse 1.1s ease-out infinite;
  }

  .work-copy {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .work-run {
    margin-left: auto;
    max-width: 8rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--choir-text-accent);
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.68rem;
    font-weight: 700;
  }

  @keyframes work-pulse {
    0% {
      box-shadow: 0 0 0 0 var(--choir-state-active-glow);
    }
    100% {
      box-shadow: 0 0 0 0.55rem var(--choir-state-active-glow);
    }
  }

  .nav-version {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 2.3rem;
    height: 1.95rem;
    padding: 0 0.6rem;
    border-radius: 999px;
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
    font-size: 0.76rem;
    font-weight: 650;
    backdrop-filter: blur(8px);
  }

  .draft-line {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    max-width: 9rem;
    height: 1.95rem;
    padding: 0 0.72rem;
    border-radius: 999px;
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-surface-raised);
    color: var(--choir-text-primary);
    font-size: 0.74rem;
    font-weight: 680;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .nav-btn,
  .prompt-btn,
  .secondary-action,
  .download-menu > summary,
  .download-menu button,
  .update-pill,
  .primary-action {
    border: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
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
    color: var(--choir-text-accent);
  }

  .secondary-action.danger {
    border-color: var(--choir-status-danger);
    color: var(--choir-status-danger);
  }

  .download-menu {
    position: relative;
    display: inline-flex;
    align-items: center;
  }

  .download-menu > summary {
    list-style: none;
    border-radius: 999px;
    padding: 0.62rem 0.84rem;
    font-size: 0.78rem;
    font-weight: 720;
    color: var(--choir-text-accent);
  }

  .download-menu > summary::-webkit-details-marker {
    display: none;
  }

  .download-menu[open] > summary {
    background: var(--choir-accent-soft);
    border-color: var(--choir-accent);
  }

  .download-menu[open] {
    z-index: 4;
  }

  .download-menu[open]::after {
    content: '';
    position: fixed;
    inset: 0;
    z-index: -1;
  }

  .download-menu button {
    display: block;
    width: 100%;
    border-radius: 0;
    border-width: 0;
    border-bottom: 1px solid var(--choir-border);
    background: transparent;
    color: var(--choir-text-primary);
    padding: 0.58rem 0.7rem;
    text-align: left;
    font-size: 0.76rem;
    font-weight: 680;
  }

  .download-menu button:last-child {
    border-bottom: 0;
  }

  .download-menu[open] button {
    min-width: 8rem;
  }

  .download-menu[open] > button,
  .download-menu[open] > :global(button) {
    display: block;
  }

  .download-menu[open] {
    flex-direction: column;
    align-items: stretch;
    border: 1px solid var(--choir-border-strong);
    border-radius: 0.65rem;
    background: var(--choir-surface-elevated);
    box-shadow: var(--choir-shadow-lg);
    overflow: hidden;
  }

  .rendered-doc {
    flex: 1 1 auto;
    min-height: 0;
    height: auto;
    overflow: auto;
    overflow-anchor: none;
    padding: clamp(1.1rem, 2.2vw, 2rem);
    line-height: 1.72;
    color: var(--choir-text-accent);
    user-select: text;
  }

  .editable-doc {
    outline: none;
    caret-color: var(--choir-text-accent);
  }

  .editable-doc:empty::before {
    content: "Start typing the document...";
    color: var(--choir-text-accent);
  }

  .editable-doc:focus {
    box-shadow: inset 0 0 0 1px var(--choir-state-active-glow);
  }

  .editable-doc.readonly {
    color: var(--choir-text-accent);
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
    color: var(--choir-text-accent);
    text-underline-offset: 0.18em;
  }

  .rendered-doc :global(.vtext-source-ref) {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 1.1rem;
    min-height: 1.1rem;
    margin: 0 0.04rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 50%;
    padding: 0;
    color: var(--choir-text-accent);
    background: var(--choir-state-selected);
    font-size: 0.62em;
    font-weight: 820;
    line-height: 1;
    vertical-align: super;
    cursor: pointer;
  }

  .rendered-doc :global(.vtext-source-ref[data-expanded="true"]) {
    display: inline-grid;
    grid-template-columns: auto minmax(12rem, 1fr);
    align-items: start;
    min-width: min(24rem, 100%);
    max-width: min(30rem, 100%);
    margin: 0.28rem 0.08rem;
    border-radius: 8px;
    padding: 0.18rem 0.22rem;
    font-size: 0.88rem;
    line-height: 1.35;
    vertical-align: baseline;
  }

  .rendered-doc :global(.vtext-source-ref:focus-visible) {
    outline: 2px solid var(--choir-state-active-glow);
    outline-offset: 2px;
  }

  .rendered-doc :global(.vtext-source-ref--missing) {
    border-color: var(--choir-status-danger);
  }

  .rendered-doc :global(.vtext-source-ref-popover) {
    position: absolute;
    z-index: 30;
    left: 0;
    top: calc(100% + 0.35rem);
    display: none;
    width: min(22rem, calc(100vw - 3rem));
    border: 1px solid var(--choir-border-strong);
    border-radius: 8px;
    padding: 0.58rem 0.64rem;
    color: var(--choir-text-accent);
    background: var(--choir-surface-pane);
    box-shadow: 0 18px 42px color-mix(in srgb, var(--choir-shadow-color) 28%, transparent);
    font-size: 0.82rem;
    font-weight: 620;
    line-height: 1.35;
    text-transform: none;
  }

  .rendered-doc :global(.vtext-source-ref[data-expanded="true"] .vtext-source-ref-popover) {
    position: static;
    z-index: auto;
    display: grid;
    width: auto;
    margin-left: 0.42rem;
    border-color: color-mix(in srgb, var(--choir-border-strong) 72%, transparent);
    background: color-mix(in srgb, var(--choir-surface-pane) 88%, transparent);
    box-shadow: none;
  }

  .rendered-doc :global(.vtext-source-ref-popover strong),
  .rendered-doc :global(.vtext-source-ref-popover span) {
    display: block;
  }

  .rendered-doc :global(.vtext-source-ref:not([data-expanded="true"]):hover .vtext-source-ref-popover),
  .rendered-doc :global(.vtext-source-ref:not([data-expanded="true"]):focus .vtext-source-ref-popover) {
    display: block;
  }

  .rendered-doc :global(code) {
    border-radius: 0.35rem;
    background: var(--choir-state-hover);
    padding: 0.08rem 0.3rem;
  }

  .rendered-doc :global(blockquote) {
    margin: 0 0 1rem;
    border-left: 3px solid var(--choir-border-strong);
    padding: 0.1rem 0 0.1rem 0.9rem;
    color: var(--choir-text-accent);
    background: var(--choir-state-selected);
  }

  .rendered-doc :global(blockquote p:last-child) {
    margin-bottom: 0;
  }

  .rendered-doc :global(.vtext-transclusion-body) {
    display: grid;
    gap: 0.52rem;
  }

  .rendered-doc :global(.vtext-transclusion-quote) {
    margin: 0;
    border-left: 3px solid var(--choir-border-strong);
    padding: 0.42rem 0.58rem;
    background: var(--choir-state-hover);
  }

  .rendered-doc :global(.vtext-source-video),
  .rendered-doc :global(.vtext-source-image) {
    display: block;
    min-height: 8rem;
    background: var(--choir-state-selected);
  }

  .rendered-doc :global(.vtext-source-video--inline),
  .rendered-doc :global(.vtext-source-image--inline) {
    margin: 0.35rem 0;
  }

  .rendered-doc :global(.vtext-source-video iframe) {
    display: block;
    width: 100%;
    aspect-ratio: 16 / 9;
    border: 0;
  }

  .rendered-doc :global(.vtext-source-image img) {
    display: block;
    width: 100%;
    height: 100%;
    max-height: 16rem;
    object-fit: contain;
  }

  .rendered-doc :global(.vtext-source-facts) {
    display: flex;
    flex-wrap: wrap;
    gap: 0.34rem;
    margin-top: 0.54rem;
  }

  .rendered-doc :global(.vtext-source-facts span) {
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    padding: 0.14rem 0.42rem;
    color: var(--choir-text-accent);
    background: var(--choir-state-selected);
    font-size: 0.72rem;
  }

  .rendered-doc :global(.vtext-source-open) {
    margin-top: 0.62rem;
    border: 1px solid var(--choir-border-strong);
    border-radius: 999px;
    padding: 0.34rem 0.62rem;
    color: var(--choir-text-accent);
    background: var(--choir-state-hover);
    font: inherit;
    font-size: 0.76rem;
    font-weight: 760;
    cursor: pointer;
  }

  .rendered-doc :global(.vtext-source-open:hover) {
    background: var(--choir-state-selected);
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
    border: 1px solid var(--choir-border-strong);
    padding: 0.48rem 0.58rem;
    text-align: left;
    vertical-align: top;
  }

  .rendered-doc :global(th) {
    background: var(--choir-state-hover);
    color: var(--choir-text-accent);
    font-weight: 800;
  }

  .rendered-doc :global(td) {
    background: var(--choir-state-selected);
  }

  .publication-panel {
    flex: 0 0 auto;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 0.7rem;
    align-items: start;
    padding: 0.72rem 0.86rem;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .publication-heading {
    min-width: 0;
  }

  .publication-heading h2 {
    margin: 0;
    color: var(--choir-text-accent);
    font-size: 1rem;
    line-height: 1.24;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .publication-result {
    background: var(--choir-status-success-soft);
    border-bottom-color: var(--choir-status-success);
  }

  .public-link {
    display: inline-block;
    max-width: min(34rem, 100%);
    color: var(--choir-text-accent);
    font-size: 0.84rem;
    font-weight: 720;
    line-height: 1.35;
    overflow: hidden;
    overflow-wrap: anywhere;
    text-overflow: ellipsis;
    text-decoration: none;
  }

  .public-link:hover,
  .public-link:focus-visible {
    color: var(--choir-text-accent);
    text-decoration: underline;
  }

  .publication-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    align-items: center;
  }

  .publication-actions .primary-action {
    background: var(--choir-accent);
    border-color: var(--choir-accent);
    color: var(--choir-text-on-accent);
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
    color: var(--choir-text-accent);
    font-size: 0.72rem;
    font-weight: 800;
    letter-spacing: 0.16em;
    text-transform: uppercase;
  }

  .recent-hero h2 {
    margin: 0 0 0.45rem;
    color: var(--choir-text-accent);
    font-size: clamp(1.5rem, 4vw, 2.4rem);
    letter-spacing: -0.05em;
  }

  .recent-hero p {
    margin: 0;
    color: var(--choir-text-accent);
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
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
  }

  .recent-list {
    display: grid;
    align-content: start;
    gap: 0.65rem;
    min-height: 0;
  }

  .recent-card,
  .recent-empty {
    border: 1px solid var(--choir-border-strong);
    border-radius: 16px;
    background: var(--choir-state-selected);
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
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .recent-title {
    color: var(--choir-text-accent);
    font-weight: 780;
  }

  .recent-meta,
  .recent-empty {
    color: var(--choir-text-accent);
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
    color: var(--choir-text-accent);
  }

  .error-float {
    position: absolute;
    left: 0.85rem;
    bottom: 0.85rem;
    z-index: 2;
    max-width: min(32rem, calc(100% - 8rem));
    border-radius: 12px;
    border: 1px solid var(--choir-status-danger);
    background: var(--choir-status-danger);
    color: var(--choir-text-on-accent);
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
    background: var(--choir-state-selected);
    border-color: var(--choir-border-strong);
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

    .draft-line {
      max-width: 6.8rem;
      height: 1.78rem;
      padding: 0 0.52rem;
      font-size: 0.68rem;
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

    .compare-panel {
      padding: 0.68rem 0.72rem;
      gap: 0.6rem;
    }

    .compare-summary,
    .provenance-strip,
    .merge-suggestions {
      grid-template-columns: minmax(0, 1fr);
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
