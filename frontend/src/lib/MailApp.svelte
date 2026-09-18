<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';

  export let authenticated = false;
  export let appContext = {};
  export let windowId = '';

  const dispatch = createEventDispatcher();

  const folders = [
    { id: 'inbox', label: 'Inbox', icon: 'inbox' },
    { id: 'drafts', label: 'Drafts', icon: 'draft' },
    { id: 'sent', label: 'Sent', icon: 'sent' },
    { id: 'quarantine', label: 'Quarantine', icon: 'shield' },
  ];
  const EMAIL_REQUEST_TIMEOUT_MS = 15000;
  const COMPACT_MAX = 720;
  const MEDIUM_MAX = 1040;

  let aliases = [];
  let activeFolder = normalizeFolder(appContext?.activeFolder) || 'inbox';
  let messages = [];
  let selectedId = appContext?.selectedId || '';
  let detail = null;
  let loading = false;
  let detailLoading = false;
  let nextCursor = '';
  let loadingMore = false;
  let folderTotals: Record<string, number> = {};
  let folderUnread: Record<string, number> = {};
  let folderCounts: Record<string, { total: number; unread: number }> = {};
  let error = '';
  let notice = '';
  let noticeKind = 'info';
  let noticeTimer: ReturnType<typeof setTimeout> | null = null;
  let replyOpen = false;
  let replyBody = '';
  let composeOpen = false;
  let composeTo = '';
  let composeSubject = '';
  let composeBody = '';
  let sending = false;
  let stagedAttachments = [];
  let attachmentInput: HTMLInputElement | null = null;
  let attachmentBusy = false;
  let fromFilesOpen = false;
  let fromFilesPath = [];
  let fromFilesEntries = [];
  let fromFilesLoading = false;
  let fromFilesError = '';
  let filter = '';
  let bodyViewMode = 'html';
  let detailPaneOpen = Boolean(appContext?.detailPaneOpen);
  let openedContextDraftId = '';
  let appStateEmitTimer: ReturnType<typeof setTimeout> | null = null;
  let mounted = false;
  let bootstrappedAuthState: boolean | null = null;
  let mailboxBootstrapStarted = false;
  let aliasLoadGeneration = 0;
  let messageLoadGeneration = 0;
  let detailLoadGeneration = 0;
  let countsLoadGeneration = 0;
  let appWidth = 1280;

  $: layout = appWidth < COMPACT_MAX ? 'compact' : appWidth < MEDIUM_MAX ? 'medium' : 'wide';
  $: compactLayout = layout === 'compact';
  $: selectedMessage = messages.find((message) => message.id === selectedId) || null;
  $: activeAddress = aliases[0]?.address || '';
  $: displayAddress = activeAddress || (authenticated ? 'No address' : 'preview@choir.news');
  $: detailHeaderEntries = headerEntries(detail?.raw_headers);
  $: detailToRecipients = detail?.recipients?.to || [];
  $: detailCcRecipients = detail?.recipients?.cc || [];
  $: detailBccRecipients = detail?.recipients?.bcc || [];
  $: detailToLine = addressListLabel(detailToRecipients) || activeAddress;
  $: composeRecipients = parseAddressList(composeTo);
  $: hasHtmlBody = Boolean(detail?.html_body && detail.html_body.trim());
  $: effectiveBodyMode = hasHtmlBody ? bodyViewMode : 'text';
  $: filteredMessages = filterMessages(messages, filter);
  $: activeFolderMeta = folders.find((folder) => folder.id === activeFolder) || folders[0];
  $: isDraftView = Boolean(detail?.draft);
  $: draftPending = detail?.draft && detail.draft.status !== 'sent';

  $: if (authenticated && appContext?.draftId && openedContextDraftId !== appContext.draftId) {
    void openContextDraft(appContext.draftId);
  }

  function handleVisibilityOrFocus() {
    if (document.visibilityState === 'visible' && authenticated && !loading && !loadingMore) {
      void loadMessages(activeFolder, { persist: false, background: true });
      void loadFolderCounts({ background: true });
    }
  }

  function handleKeydown(event) {
    if (!mounted) return;
    const target = event.target;
    if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT' || target.isContentEditable)) {
      return;
    }
    if (composeOpen) {
      if (event.key === 'Escape') {
        void discardCompose();
        event.preventDefault();
      }
      return;
    }
    if (event.key === 'Escape') {
      if (replyOpen) {
        replyOpen = false;
      } else if (detailPaneOpen && compactLayout) {
        showMessageList();
      }
      return;
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp' && event.key !== 'Enter') return;
    const list = filteredMessages;
    if (!list.length) return;
    event.preventDefault();
    if (event.key === 'Enter') {
      if (selectedId) void loadDetail(selectedId, { openPane: true });
      return;
    }
    const index = list.findIndex((message) => message.id === selectedId);
    const nextIndex = event.key === 'ArrowDown'
      ? Math.min(list.length - 1, index + 1)
      : Math.max(0, index <= 0 ? 0 : index - 1);
    const next = list[nextIndex];
    if (next && next.id !== selectedId) {
      void loadDetail(next.id, { openPane: !compactLayout });
    }
  }

  onMount(() => {
    mounted = true;
    window.addEventListener('focus', handleVisibilityOrFocus);
    window.addEventListener('keydown', handleKeydown);
    document.addEventListener('visibilitychange', handleVisibilityOrFocus);
  });

  onDestroy(() => {
    mounted = false;
    window.removeEventListener('focus', handleVisibilityOrFocus);
    window.removeEventListener('keydown', handleKeydown);
    document.removeEventListener('visibilitychange', handleVisibilityOrFocus);
    invalidateMailRequests();
    if (appStateEmitTimer) clearTimeout(appStateEmitTimer);
    if (noticeTimer) clearTimeout(noticeTimer);
  });

  $: if (mounted && authenticated !== bootstrappedAuthState) {
    bootstrappedAuthState = authenticated;
    mailboxBootstrapStarted = false;
    invalidateMailRequests();
    if (!authenticated) {
      // Defer out of the reactive statement: synchronous invalidations made
      // mid-update do not re-run dependent reactive statements (filteredMessages).
      queueMicrotask(() => {
        if (mounted && !authenticated) loadPreviewMailbox();
      });
    } else {
      void bootstrapMailbox();
    }
  }
  function normalizeFolder(value) {
    const id = String(value || '').trim();
    return folders.some((folder) => folder.id === id) ? id : '';
  }

  function currentMailAppContext() {
    const draftId = activeFolder === 'drafts' && detail?.draft?.id
      ? detail.draft.id
      : '';
    return {
      activeFolder,
      selectedId: selectedId || '',
      detailPaneOpen: Boolean(detailPaneOpen),
      view: composeOpen ? 'compose' : detailPaneOpen ? 'detail' : 'list',
      draftId,
      windowTitle: 'Mail',
    };
  }

  function invalidateMailRequests() {
    aliasLoadGeneration += 1;
    messageLoadGeneration += 1;
    detailLoadGeneration += 1;
    countsLoadGeneration += 1;
    loading = false;
    detailLoading = false;
  }

  function isLatestAliasLoad(requestId) {
    return requestId === aliasLoadGeneration;
  }

  function isLatestMessageLoad(requestId) {
    return requestId === messageLoadGeneration;
  }

  function isLatestDetailLoad(requestId, ownerMessageLoad = 0) {
    return requestId === detailLoadGeneration &&
      (!ownerMessageLoad || ownerMessageLoad === messageLoadGeneration);
  }

  async function bootstrapMailbox() {
    if (mailboxBootstrapStarted) return;
    mailboxBootstrapStarted = true;
    await Promise.all([
      loadAliases(),
      loadMessages(activeFolder, {
        openPane: detailPaneOpen,
        selectedId,
        persist: false,
      }),
    ]);
    void loadFolderCounts({ background: true });
  }

  async function fetchMailWithTimeout(url, options = {}) {
    const controller = new AbortController();
    let timeout: ReturnType<typeof setTimeout> | null = null;
    const timeoutPromise = new Promise((_, reject) => {
      timeout = setTimeout(() => {
        controller.abort();
        reject(new Error('Mail request timed out'));
      }, EMAIL_REQUEST_TIMEOUT_MS);
    });
    try {
      return await Promise.race([
        fetchWithRenewal(url, {
          ...options,
          signal: controller.signal,
        }),
        timeoutPromise,
      ]);
    } catch (err) {
      if (err?.name === 'AbortError') {
        throw new Error('Mail request timed out');
      }
      throw err;
    } finally {
      if (timeout) clearTimeout(timeout);
    }
  }

  function scheduleAppStateEmit() {
    if (!windowId) return;
    if (appStateEmitTimer) clearTimeout(appStateEmitTimer);
    appStateEmitTimer = setTimeout(() => {
      appStateEmitTimer = null;
      emitAppState();
    }, 120);
  }

  function emitAppState() {
    if (!windowId) return;
    if (appStateEmitTimer) {
      clearTimeout(appStateEmitTimer);
      appStateEmitTimer = null;
    }
    dispatch('contextchange', {
      windowId,
      appContext: currentMailAppContext(),
      title: 'Mail',
    });
  }

  function showNotice(text, kind = 'info') {
    notice = text;
    noticeKind = kind;
    if (noticeTimer) clearTimeout(noticeTimer);
    noticeTimer = setTimeout(() => {
      notice = '';
      noticeTimer = null;
    }, 5000);
  }

  async function loadAliases() {
    const requestId = ++aliasLoadGeneration;
    try {
      const res = await fetchMailWithTimeout('/api/email/aliases');
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not load addresses');
      }
      const data = await res.json();
      if (!isLatestAliasLoad(requestId)) return;
      aliases = data.aliases || [];
    } catch (err) {
      if (isLatestAliasLoad(requestId)) handleError(err);
    }
  }

  function loadPreviewMailbox() {
    aliases = [{ address: 'preview@choir.news' }];
    messages = previewMessagesFor(activeFolder);
    folderCounts = previewFolderCounts();
    folderTotals = {};
    folderUnread = {};
    nextCursor = '';
    selectedId = messages[0]?.id || '';
    detail = selectedId ? previewDetailFor(messages[0]) : null;
    detailPaneOpen = false;
  }

  function previewMessagesFor(folder) {
    return (previewMailbox[folder] || []).map((message) => ({ ...message }));
  }

  function previewFolderCounts() {
    const counts: Record<string, { total: number; unread: number }> = {};
    for (const folder of folders) {
      const list = previewMailbox[folder.id] || [];
      counts[folder.id] = {
        total: list.length,
        unread: list.filter((message) => message.direction === 'inbound' && !message.read_at).length,
      };
    }
    return counts;
  }

  function previewDetailFor(message) {
    if (!message) return null;
    if (message.direction === 'draft') {
      return {
        draft: message.draft,
        message,
        text_body: message.draft?.text_body || message.snippet || '',
        html_body: '',
        raw_headers: {},
        recipients: {
          to: (message.draft?.to_addresses || []).map((address) => ({ address })),
          cc: [],
          bcc: [],
        },
        attachments: [],
      };
    }
    return {
      message,
      text_body: message.text_body || `${message.snippet || ''}\n\nThis mailbox is local preview data. Sign in to read real mail, drafts, and history.`,
      html_body: message.html_body || '',
      raw_headers: { 'X-Choir-Preview': 'local' },
      recipients: { to: [{ display: 'You', address: 'preview@choir.news' }], cc: [], bcc: [] },
      attachments: message.attachments || [],
    };
  }

  async function loadFolderCounts(options = {}) {
    if (!authenticated) return;
    const requestId = ++countsLoadGeneration;
    try {
      const [inboxRes, sentRes, quarantineRes, draftsRes] = await Promise.all([
        fetchMailWithTimeout('/api/email/messages?folder=inbox&limit=1'),
        fetchMailWithTimeout('/api/email/messages?folder=sent&limit=1'),
        fetchMailWithTimeout('/api/email/messages?folder=quarantine&limit=1'),
        fetchMailWithTimeout('/api/email/drafts'),
      ]);
      if (requestId !== countsLoadGeneration) return;
      const next = { ...folderCounts };
      for (const [folder, res] of [['inbox', inboxRes], ['sent', sentRes], ['quarantine', quarantineRes]]) {
        if (res.ok) {
          const data = await res.json();
          next[folder] = {
            total: typeof data.total === 'number' ? data.total : (data.messages || []).length,
            unread: typeof data.unread === 'number' ? data.unread : 0,
          };
        }
      }
      if (draftsRes.ok) {
        const data = await draftsRes.json();
        next.drafts = { total: (data.drafts || []).length, unread: 0 };
      }
      folderCounts = next;
    } catch (err) {
      // Count badges are advisory; failures stay silent.
    }
  }

  function recordFolderStats(folder, data) {
    if (typeof data.total === 'number') {
      folderTotals[folder] = data.total;
      folderTotals = folderTotals;
    }
    if (typeof data.unread === 'number') {
      folderUnread[folder] = data.unread;
      folderUnread = folderUnread;
    }
    if (typeof data.total === 'number' || typeof data.unread === 'number') {
      folderCounts = {
        ...folderCounts,
        [folder]: {
          total: typeof data.total === 'number' ? data.total : folderCounts[folder]?.total || 0,
          unread: typeof data.unread === 'number' ? data.unread : folderCounts[folder]?.unread || 0,
        },
      };
    }
  }

  async function loadMessages(folder, options = {}) {
    const requestId = ++messageLoadGeneration;
    const nextFolder = normalizeFolder(folder) || 'inbox';
    if (!authenticated) {
      activeFolder = nextFolder;
      messages = previewMessagesFor(nextFolder);
      selectedId = options.selectedId || messages[0]?.id || '';
      detail = selectedId ? previewDetailFor(messages.find((m) => m.id === selectedId)) : null;
      detailPaneOpen = Boolean(options.openPane);
      composeOpen = false;
      replyOpen = false;
      if (options.persist !== false) scheduleAppStateEmit();
      return;
    }
    detailLoadGeneration += 1;
    detailLoading = false;
    if (!options.background) {
      loading = true;
      error = '';
    }
    activeFolder = nextFolder;
    if (!options.background) {
      detailPaneOpen = Boolean(options.openPane);
      composeOpen = false;
      replyOpen = false;
      filter = '';
    }
    if (options.persist !== false) emitAppState();
    try {
      if (nextFolder === 'drafts') {
        nextCursor = '';
        await loadDrafts(options, requestId);
        return;
      }
      const res = await fetchMailWithTimeout(`/api/email/messages?folder=${encodeURIComponent(nextFolder)}&limit=100`);
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not load mail');
      }
      const data = await res.json();
      if (!isLatestMessageLoad(requestId)) return;
      const incoming = data.messages || [];
      recordFolderStats(nextFolder, data);

      if (options.background && messages.length > 0) {
        const incomingMap = new Map(incoming.map((m) => [m.id, m]));
        const olderMessages = messages.filter((m) => !incomingMap.has(m.id));
        messages = [...incoming, ...olderMessages];
        if (!nextCursor) {
          nextCursor = data.next_cursor || '';
        }
      } else {
        messages = incoming;
        nextCursor = data.next_cursor || '';
      }

      if (options.selectedId && messages.some((message) => message.id === options.selectedId)) {
        selectedId = options.selectedId;
      }
      if (!messages.some((message) => message.id === selectedId)) {
        selectedId = messages[0]?.id || '';
        detail = null;
      }
      if (selectedId && (!options.background || !detail)) {
        await loadDetail(selectedId, {
          openPane: Boolean(options.openPane),
          persist: false,
          ownerMessageLoad: requestId,
          background: options.background,
        });
      }
    } catch (err) {
      if (isLatestMessageLoad(requestId) && !options.background) handleError(err);
    } finally {
      if (isLatestMessageLoad(requestId)) {
        loading = false;
        if (options.persist !== false) scheduleAppStateEmit();
      }
    }
  }

  async function loadMoreMessages() {
    if (!nextCursor || loadingMore || !authenticated || activeFolder === 'drafts') return;
    const currentFolder = activeFolder;
    const cursor = nextCursor;
    const requestId = messageLoadGeneration;
    loadingMore = true;
    try {
      const res = await fetchMailWithTimeout(
        `/api/email/messages?folder=${encodeURIComponent(currentFolder)}&cursor=${encodeURIComponent(cursor)}&limit=100`
      );
      if (!res.ok) return;
      const data = await res.json();
      if (!isLatestMessageLoad(requestId) || activeFolder !== currentFolder) return;
      nextCursor = data.next_cursor || '';
      recordFolderStats(currentFolder, data);
      const incoming = data.messages || [];
      if (incoming.length > 0) {
        const seen = new Set(messages.map((m) => m.id));
        const toAppend = incoming.filter((m) => !seen.has(m.id));
        messages = [...messages, ...toAppend];
      }
    } catch (err) {
      // transient pagination failure
    } finally {
      loadingMore = false;
    }
  }

  function handleListScroll(event) {
    const el = event.currentTarget;
    if (!el || !nextCursor || loadingMore || filter) return;
    if (el.scrollHeight - el.scrollTop - el.clientHeight < 160) {
      void loadMoreMessages();
    }
  }

  async function loadDrafts(options = {}, requestId = messageLoadGeneration) {
    const res = await fetchMailWithTimeout('/api/email/drafts');
    if (!res.ok) {
      if (res.status === 401) throw new AuthRequiredError();
      throw new Error('Could not load drafts');
    }
    const data = await res.json();
    if (!isLatestMessageLoad(requestId)) return;
    const drafts = (data.drafts || []).map(draftListItem);
    messages = drafts;
    folderCounts = {
      ...folderCounts,
      drafts: { total: drafts.length, unread: 0 },
    };
    if (options.selectedId && messages.some((message) => message.id === options.selectedId)) {
      selectedId = options.selectedId;
    }
    if (!messages.some((message) => message.id === selectedId)) {
      selectedId = messages[0]?.id || '';
      detail = null;
    }
    if (selectedId) {
      await loadDetail(selectedId, {
        openPane: Boolean(options.openPane),
        persist: false,
        ownerMessageLoad: requestId,
        background: options.background,
      });
    }
  }

  async function openContextDraft(draftId) {
    const id = String(draftId || '').trim();
    if (!id) return;
    openedContextDraftId = id;
    await loadMessages('drafts', { selectedId: id, openPane: true, persist: false });
    await loadDetail(id, { openPane: true });
  }

  function isMessageUnread(msg) {
    return Boolean(msg && msg.direction === 'inbound' && !msg.read_at);
  }

  async function toggleReadStatus() {
    if (!detail?.message || activeFolder === 'drafts' || !authenticated) return;
    const msg = detail.message;
    const currentlyUnread = !msg.read_at;
    const newReadAt = currentlyUnread ? new Date().toISOString() : '';
    msg.read_at = newReadAt;
    detail = detail;

    const target = messages.find((m) => m.id === msg.id);
    if (target) {
      target.read_at = newReadAt;
      messages = messages;
    }

    adjustUnread(activeFolder, currentlyUnread ? -1 : 1);
    try {
      await fetchMailWithTimeout(`/api/email/messages/${encodeURIComponent(msg.id)}/${currentlyUnread ? 'read' : 'unread'}`, {
        method: 'POST',
      });
    } catch (err) {
      console.warn('Failed to toggle read state:', err);
    }
  }

  function adjustUnread(folder, delta) {
    const next = Math.max(0, (folderUnread[folder] || 0) + delta);
    folderUnread = { ...folderUnread, [folder]: next };
    const counts = folderCounts[folder];
    if (counts) {
      folderCounts = { ...folderCounts, [folder]: { ...counts, unread: Math.max(0, counts.unread + delta) } };
    }
  }

  async function loadDetail(id, options = {}) {
    const requestId = ++detailLoadGeneration;
    const ownerMessageLoad = options.ownerMessageLoad || 0;
    selectedId = id;
    detailLoading = true;
    if (!options.background) {
      replyOpen = false;
      composeOpen = false;
      bodyViewMode = 'html';
    }
    if (options.openPane) {
      detailPaneOpen = true;
    }
    if (!authenticated) {
      const previewMessage = messages.find((m) => m.id === id);
      detail = previewDetailFor(previewMessage);
      if (previewMessage && isMessageUnread(previewMessage)) {
        previewMessage.read_at = new Date().toISOString();
        messages = messages;
        folderCounts = previewFolderCounts();
      }
      detailLoading = false;
      if (options.persist !== false) scheduleAppStateEmit();
      return;
    }
    try {
      if (activeFolder === 'drafts') {
        const res = await fetchMailWithTimeout(`/api/email/drafts/${encodeURIComponent(id)}`);
        if (!res.ok) {
          if (res.status === 401) throw new AuthRequiredError();
          throw new Error('Could not open draft');
        }
        const draft = await res.json();
        if (!isLatestDetailLoad(requestId, ownerMessageLoad)) return;
        detail = draftDetail(draft);
        return;
      }
      const res = await fetchMailWithTimeout(`/api/email/messages/${encodeURIComponent(id)}`);
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not open message');
      }
      const data = await res.json();
      if (!isLatestDetailLoad(requestId, ownerMessageLoad)) return;
      detail = data;

      const msg = data?.message;
      if (msg && msg.direction === 'inbound' && !msg.read_at && activeFolder !== 'drafts') {
        const readAt = new Date().toISOString();
        msg.read_at = readAt;

        const target = messages.find((m) => m.id === id);
        if (target && !target.read_at) {
          target.read_at = readAt;
          messages = messages;
          adjustUnread(activeFolder, -1);
        }

        void fetchMailWithTimeout(`/api/email/messages/${encodeURIComponent(id)}/read`, {
          method: 'POST',
        }).catch((err) => {
          console.warn('Failed to mark message read on server:', err);
        });
      }
    } catch (err) {
      if (isLatestDetailLoad(requestId, ownerMessageLoad)) handleError(err);
    } finally {
      if (isLatestDetailLoad(requestId, ownerMessageLoad)) {
        detailLoading = false;
        if (options.persist !== false) scheduleAppStateEmit();
      }
    }
  }

  function requireAuth(kind) {
    if (authenticated) return false;
    dispatch('authrequired', { kind, appId: 'email', appName: 'Mail' });
    return true;
  }

  async function sendReply() {
    if (requireAuth('email_reply')) return;
    if (!selectedMessage || !replyBody.trim() || !activeAddress) return;
    sending = true;
    notice = '';
    try {
      const res = await fetchMailWithTimeout('/api/email/drafts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_address: activeAddress,
          to_addresses: [selectedMessage.from_address],
          subject: selectedMessage.subject?.startsWith('Re:') ? selectedMessage.subject : `Re: ${selectedMessage.subject || ''}`,
          text_body: replyBody.trim(),
          reply_to_message_id: selectedMessage.id,
        }),
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not create reply draft');
      }
      const draft = await res.json();
      replyBody = '';
      replyOpen = false;
      showNotice('Reply saved to Drafts — review and approve to send');
      await loadMessages('drafts', { selectedId: draft.id, openPane: true, persist: false });
      await loadDetail(draft.id, { openPane: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function sendCompose() {
    if (requireAuth('email_compose')) return;
    if (!composeRecipients.length || !composeBody.trim() || !activeAddress) return;
    sending = true;
    notice = '';
    try {
      const res = await fetchMailWithTimeout('/api/email/drafts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_address: activeAddress,
          to_addresses: composeRecipients,
          subject: composeSubject.trim(),
          text_body: composeBody.trim(),
          attachment_ids: stagedAttachments.map((attachment) => attachment.id),
        }),
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not create draft');
      }
      const draft = await res.json();
      composeTo = '';
      composeSubject = '';
      composeBody = '';
      composeOpen = false;
      stagedAttachments = [];
      showNotice('Saved to Drafts — review and approve to send');
      await loadMessages('drafts', { selectedId: draft.id, openPane: true, persist: false });
      await loadDetail(draft.id, { openPane: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function sendDraft() {
    if (requireAuth('email_send')) return;
    const draftId = detail?.draft?.id;
    if (!draftId) return;
    sending = true;
    notice = '';
    try {
      const res = await fetchMailWithTimeout(`/api/email/drafts/${encodeURIComponent(draftId)}/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          version_hash: detail?.draft?.version_hash || '',
        }),
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not send draft');
      }
      showNotice('Draft sent');
      await loadMessages('sent');
      void loadFolderCounts({ background: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function emailApprovalLink() {
    if (requireAuth('email_send')) return;
    const draftId = detail?.draft?.id;
    if (!draftId) return;
    sending = true;
    notice = '';
    try {
      const res = await fetchMailWithTimeout(`/api/email/drafts/${encodeURIComponent(draftId)}/approval-email`, {
        method: 'POST',
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not send approval email');
      }
      showNotice('Approval link sent to your account email');
      await loadDetail(draftId, { openPane: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function responseError(res, fallback) {
    const text = await res.text().catch(() => '');
    if (!text) return fallback;
    try {
      const data = JSON.parse(text);
      if (typeof data?.error === 'string' && data.error) return data.error;
      if (typeof data?.message === 'string' && data.message) return data.message;
    } catch (_err) {
      // A plain-text API response is already the most useful error.
    }
    return text;
  }

  async function stageAttachment(file) {
    if (!file || requireAuth('email_compose')) return false;
    attachmentBusy = true;
    try {
      const res = await fetchMailWithTimeout('/api/email/attachments', {
        method: 'POST',
        headers: {
          'X-Choir-Filename': file.name,
          'Content-Type': file.type || 'application/octet-stream',
        },
        body: file,
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        showNotice(await responseError(res, 'Could not upload attachment'), 'error');
        return false;
      }
      const attachment = await res.json();
      stagedAttachments = [...stagedAttachments, attachment];
      return true;
    } catch (err) {
      handleError(err);
      return false;
    } finally {
      attachmentBusy = false;
    }
  }

  async function handleClientAttachments(event) {
    const input = event.currentTarget as HTMLInputElement;
    const files = Array.from(input.files || []);
    input.value = '';
    for (const file of files) {
      await stageAttachment(file);
    }
  }

  function filesAPIPath(path = []) {
    return path.length
      ? `/api/files/${path.map(encodeURIComponent).join('/')}`
      : '/api/files';
  }

  async function loadFilesForAttachment(path = []) {
    if (requireAuth('email_compose')) return;
    fromFilesLoading = true;
    fromFilesError = '';
    try {
      const res = await fetchMailWithTimeout(filesAPIPath(path));
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        fromFilesError = await responseError(res, 'Could not load files');
        return;
      }
      const data = await res.json();
      fromFilesPath = path;
      fromFilesEntries = (Array.isArray(data) ? data : []).sort((left, right) => {
        if (left.type === 'directory' && right.type !== 'directory') return -1;
        if (left.type !== 'directory' && right.type === 'directory') return 1;
        return String(left.name || '').localeCompare(String(right.name || ''));
      });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        handleError(err);
      } else {
        fromFilesError = err?.message || 'Could not load files';
      }
    } finally {
      fromFilesLoading = false;
    }
  }

  async function openFilesPicker() {
    fromFilesOpen = true;
    await loadFilesForAttachment(fromFilesPath);
  }

  async function attachFromFiles(entry) {
    const path = [...fromFilesPath, entry.name];
    attachmentBusy = true;
    fromFilesError = '';
    try {
      const res = await fetchMailWithTimeout(filesAPIPath(path));
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        fromFilesError = await responseError(res, 'Could not read file');
        return;
      }
      const bytes = await res.blob();
      const file = new File([bytes], entry.name, {
        type: bytes.type || entry.content_type || 'application/octet-stream',
      });
      attachmentBusy = false;
      if (await stageAttachment(file)) fromFilesOpen = false;
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        handleError(err);
      } else {
        fromFilesError = err?.message || 'Could not read file';
      }
    } finally {
      attachmentBusy = false;
    }
  }

  async function removeStagedAttachment(attachment) {
    attachmentBusy = true;
    try {
      const res = await fetchMailWithTimeout(`/api/email/attachments/${encodeURIComponent(attachment.id)}`, {
        method: 'DELETE',
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        showNotice(await responseError(res, 'Could not remove attachment'), 'error');
        return;
      }
      stagedAttachments = stagedAttachments.filter((item) => item.id !== attachment.id);
    } catch (err) {
      handleError(err);
    } finally {
      attachmentBusy = false;
    }
  }

  async function discardCompose() {
    if (attachmentBusy) return;
    for (const attachment of [...stagedAttachments]) {
      await removeStagedAttachment(attachment);
    }
    if (stagedAttachments.length) return;
    composeOpen = false;
    fromFilesOpen = false;
  }

  async function downloadAttachment(attachment) {
    try {
      const res = await fetchMailWithTimeout(`/api/email/attachments/${encodeURIComponent(attachment.id)}`);
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        showNotice(await responseError(res, 'Could not download attachment'), 'error');
        return;
      }
      const url = URL.createObjectURL(await res.blob());
      const link = document.createElement('a');
      link.href = url;
      link.download = attachment.filename || 'attachment';
      link.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 0);
    } catch (err) {
      handleError(err);
    }
  }

  function canDownloadAttachment() {
    return detail?.message?.direction === 'draft' || detail?.message?.direction === 'outbound';
  }

  function openCompose() {
    if (requireAuth('email_compose')) return;
    composeOpen = true;
    replyOpen = false;
    detailPaneOpen = true;
    notice = '';
    error = '';
    stagedAttachments = [];
    fromFilesOpen = false;
    fromFilesPath = [];
    fromFilesEntries = [];
    scheduleAppStateEmit();
  }

  function showMessageList() {
    detailPaneOpen = false;
    composeOpen = false;
    replyOpen = false;
    notice = '';
    scheduleAppStateEmit();
  }

  function handleError(err) {
    if (err instanceof AuthRequiredError) {
      dispatch('authexpired');
      return;
    }
    error = err?.message || 'Mail action failed';
    notice = '';
  }

  function filterMessages(list, query) {
    const q = String(query || '').trim().toLowerCase();
    if (!q) return list;
    return list.filter((message) => {
      const haystack = [
        message.from_display,
        message.from_address,
        message.subject,
        message.snippet,
        message.direction === 'draft' ? (message.to_addresses || []).join(' ') : '',
      ].join(' ').toLowerCase();
      return haystack.includes(q);
    });
  }

  function trustChip(status) {
    if (status === 'draft') return { label: 'Pending approval', kind: 'accent' };
    if (status === 'quarantined') return { label: 'Quarantined', kind: 'warn' };
    if (status === 'public' || status === 'public-preview') return { label: 'Public inbound', kind: 'neutral' };
    if (status === 'trusted') return null;
    if (status === 'draft-preview') return { label: 'Preview draft', kind: 'accent' };
    return { label: 'Untrusted', kind: 'danger' };
  }

  function headerEntries(rawHeaders) {
    if (!rawHeaders || typeof rawHeaders !== 'object') return [];
    return Object.entries(rawHeaders)
      .filter(([key, value]) => key && value !== null && value !== undefined && String(value).trim())
      .sort(([left], [right]) => left.localeCompare(right));
  }

  function addressLabel(recipient) {
    if (!recipient) return '';
    const address = String(recipient.address || '').trim();
    const display = String(recipient.display || '').trim();
    if (!display) return address;
    if (!address) return display;
    return `${display} <${address}>`;
  }

  function addressListLabel(recipients) {
    if (!Array.isArray(recipients)) return '';
    return recipients.map(addressLabel).filter(Boolean).join(', ');
  }

  function parseAddressList(value) {
    return String(value || '')
      .split(/[,;\n]+/)
      .map((item) => item.trim())
      .filter(Boolean);
  }

  function draftListItem(draft) {
    return {
      id: draft.id,
      direction: 'draft',
      from_address: draft.from_address,
      to_addresses: draft.to_addresses || [],
      subject: draft.subject,
      snippet: draft.text_body,
      trust_status: 'draft',
      created_at: draft.updated_at || draft.created_at,
      sent_at: '',
      received_at: '',
      has_attachments: Boolean(draft.attachments?.length),
      draft,
    };
  }

  function draftDetail(draft) {
    return {
      draft,
      message: draftListItem(draft),
      text_body: draft.text_body,
      html_body: draft.html_body,
      raw_headers: {},
      recipients: {
        to: (draft.to_addresses || []).map((address) => ({ address })),
        cc: (draft.cc_addresses || []).map((address) => ({ address })),
        bcc: (draft.bcc_addresses || []).map((address) => ({ address })),
      },
      attachments: draft.attachments || [],
    };
  }

  function rowTitle(message) {
    if (message.direction === 'draft') {
      const to = (message.to_addresses || [])[0];
      return to ? `To: ${to}` : 'Draft';
    }
    if (message.direction === 'outbound') {
      return message.subject ? `You · ${message.from_address}` : 'You';
    }
    return message.from_display || message.from_address || 'Unknown sender';
  }

  function messageTimestamp(message) {
    return message.received_at || message.sent_at || message.created_at || '';
  }

  function formatListTime(value) {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    const now = new Date();
    const sameDay = date.toDateString() === now.toDateString();
    if (sameDay) {
      return date.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
    }
    const sameYear = date.getFullYear() === now.getFullYear();
    return date.toLocaleDateString([], sameYear
      ? { month: 'short', day: 'numeric' }
      : { month: 'short', day: 'numeric', year: 'numeric' });
  }

  function formatFullTime(value) {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString([], {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
    });
  }

  function senderInitials(message) {
    const source = String(message?.from_display || message?.from_address || '?').trim();
    const cleaned = source.replace(/<.*>/, '').trim();
    const words = cleaned.split(/[\s@._-]+/).filter(Boolean);
    if (!words.length) return '?';
    if (words.length === 1) return words[0].slice(0, 2).toUpperCase();
    return (words[0][0] + words[1][0]).toUpperCase();
  }

  const AVATAR_COLORS = ['--choir-chart-1', '--choir-chart-2', '--choir-chart-3', '--choir-chart-4', '--choir-chart-5', '--choir-accent'];
  function avatarColor(message) {
    const source = String(message?.from_address || message?.from_display || '');
    let hash = 0;
    for (let i = 0; i < source.length; i += 1) {
      hash = (hash * 31 + source.charCodeAt(i)) >>> 0;
    }
    return `var(${AVATAR_COLORS[hash % AVATAR_COLORS.length]})`;
  }

  function folderBadge(folderId, counts, unreadMap) {
    const entry = counts[folderId];
    if (!entry) return '';
    if (folderId === 'inbox') {
      const unread = unreadMap[folderId] ?? entry.unread;
      return unread > 0 ? String(unread) : '';
    }
    if (folderId === 'sent') return '';
    return entry.total > 0 ? String(entry.total) : '';
  }

  function folderCountLine(folder, totals, counts, unreadMap, list) {
    const total = totals[folder] ?? counts[folder]?.total ?? list.length;
    const unread = unreadMap[folder] ?? counts[folder]?.unread ?? 0;
    const noun = folder === 'drafts' ? 'drafts' : 'messages';
    return `${total} ${noun}${unread > 0 ? ` · ${unread} unread` : ''}`;
  }

  function sanitizeEmailHtml(html) {
    if (!html) return '';
    let sanitized = String(html)
      .replace(/<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>/gi, '')
      .replace(/\s+on\w+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
      .replace(/<\/?(form|iframe|object|embed|base)\b[^>]*>/gi, '')
      .replace(/href\s*=\s*["']?\s*javascript:[^"'>]*/gi, 'href="#"');

    sanitized = sanitized.replace(/<a\b([^>]*)>/gi, (match, attrs) => {
      let cleaned = attrs.replace(/\s*target\s*=\s*["'][^"']*["']/gi, '');
      cleaned = cleaned.replace(/\s*rel\s*=\s*["'][^"']*["']/gi, '');
      return `<a ${cleaned} target="_blank" rel="noopener noreferrer">`;
    });
    return sanitized;
  }

  function buildIframeContent(html) {
    const safeBody = sanitizeEmailHtml(html);
    return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src https: data: cid:; style-src 'unsafe-inline'; font-src https: data:; media-src https:; base-uri 'none'; form-action 'none';">
<style>
  html, body {
    margin: 0;
    box-sizing: border-box;
  }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', sans-serif;
    font-size: 15px;
    line-height: 1.65;
    color: #1c1c1e;
    background: #ffffff;
    padding: 24px 28px;
    word-wrap: break-word;
    overflow-wrap: break-word;
    overflow-y: auto;
  }
  img { max-width: 100%; height: auto; }
  table { max-width: 100%; }
  a { color: #2f6fed; }
  pre { overflow-x: auto; white-space: pre-wrap; }
  blockquote {
    border-left: 3px solid #d7d7de;
    margin: 0;
    padding: 6px 16px;
    color: #55555c;
  }
  hr { border: 0; border-top: 1px solid #e4e4e9; }
</style>
</head>
<body>${safeBody}</body>
</html>`;
  }

  function attachmentIconName(attachment) {
    const name = String(attachment?.filename || '').toLowerCase();
    const type = String(attachment?.content_type || '').toLowerCase();
    if (type.startsWith('text/calendar') || name.endsWith('.ics')) return 'calendar';
    if (type.startsWith('image/')) return 'image';
    if (type.startsWith('audio/')) return 'audio';
    if (type.startsWith('video/')) return 'video';
    return 'file';
  }

  function isCalendarAttachment(attachment) {
    const name = String(attachment?.filename || '').toLowerCase();
    const type = String(attachment?.content_type || '').toLowerCase();
    return type.startsWith('text/calendar') || name.endsWith('.ics');
  }

  function handleAddToCalendar(attachment) {
    dispatch('launchapp', {
      appId: 'calendar',
      appName: 'Calendar',
      icon: '📅',
      appContext: {
        windowTitle: 'Calendar',
        icsAttachmentId: attachment.id,
        icsFilename: attachment.filename,
        sourceMessageId: detail?.message?.id || '',
      },
    });
  }

  function formatFileSize(bytes) {
    if (!bytes || bytes <= 0) return '';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  const ICONS = {
    inbox: '<path d="M22 12h-5l-2 3h-6l-2-3H2"/><path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/>',
    draft: '<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/>',
    sent: '<path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/>',
    shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>',
    compose: '<path d="M12 20h9"/><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z"/>',
    refresh: '<path d="M21 12a9 9 0 1 1-2.64-6.36"/><path d="M21 3v6h-6"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/>',
    back: '<path d="m15 18-6-6 6-6"/>',
    reply: '<path d="m9 17-5-5 5-5"/><path d="M4 12h9a7 7 0 0 1 7 7v1"/>',
    clip: '<path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48"/>',
    calendar: '<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>',
    file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/>',
    image: '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="m21 15-5-5L5 21"/>',
    audio: '<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>',
    video: '<path d="m22 8-6 4 6 4V8Z"/><rect x="2" y="6" width="14" height="12" rx="2"/>',
    close: '<path d="M18 6 6 18M6 6l12 12"/>',
    check: '<path d="M20 6 9 17l-5-5"/>',
    warn: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4M12 17h.01"/>',
    envelope: '<rect x="2" y="4" width="20" height="16" rx="2"/><path d="m22 7-10 6L2 7"/>',
    eye: '<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7Z"/><circle cx="12" cy="12" r="3"/>',
    send: '<path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/>',
    info: '<circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>',
    download: '<path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/>',
  };

  function icon(name, size = 16) {
    const path = ICONS[name] || ICONS.envelope;
    return `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${path}</svg>`;
  }

  // ---- Local preview mailbox (signed-out public preview; owned by this component) ----
  const previewNow = Date.now();
  const previewMailbox = {
    inbox: [
      {
        id: 'preview-in-1',
        direction: 'inbound',
        from_address: 'hello@startupbos.org',
        from_display: 'Startup Boston Team',
        subject: "IT'S HERE: Startup Boston Week Starts TODAY",
        snippet: 'Five days of founders, startups, and the whole ecosystem here in Boston. Kickoff at BU GSU…',
        trust_status: 'public',
        read_at: '',
        received_at: new Date(previewNow - 42 * 60000).toISOString(),
        has_attachments: false,
        html_body: '<h2 style="margin:0 0 12px">Startup Boston Week starts today</h2><p>Five days of founders, startups, and the whole ecosystem here in Boston.</p><p>We are kicking off Monday at BU\'s GSU — <a href="https://www.startupbos.org/">see the schedule</a>.</p>',
      },
      {
        id: 'preview-in-2',
        direction: 'inbound',
        from_address: 'events@bostonaiweek.org',
        from_display: 'Boston AI Week',
        subject: 'Rana el Kaliouby and Ed Baker at the Museum of Science, September 30',
        snippet: 'BOSTON AI WEEK 2026 — reserve your seat for the opening keynote…',
        trust_status: 'public',
        read_at: '',
        received_at: new Date(previewNow - 5 * 3600000).toISOString(),
        has_attachments: true,
      },
      {
        id: 'preview-in-3',
        direction: 'inbound',
        from_address: 'team@choir.news',
        from_display: 'Choir',
        subject: 'Your weekly computer digest',
        snippet: '3 missions completed, 1 awaiting review, 12 new sources ingested…',
        trust_status: 'trusted',
        read_at: new Date(previewNow - 26 * 3600000).toISOString(),
        received_at: new Date(previewNow - 26 * 3600000).toISOString(),
        has_attachments: false,
      },
      {
        id: 'preview-in-4',
        direction: 'inbound',
        from_address: 'newsletter@longread.example',
        from_display: 'The Long Read',
        subject: 'What we get wrong about attention',
        snippet: 'Attention is not a resource to be spent but a relationship to be maintained…',
        trust_status: 'public',
        read_at: new Date(previewNow - 3 * 86400000).toISOString(),
        received_at: new Date(previewNow - 3 * 86400000).toISOString(),
        has_attachments: false,
      },
    ],
    drafts: [
      {
        id: 'preview-draft-1',
        direction: 'draft',
        from_address: 'preview@choir.news',
        to_addresses: ['friend@example.com'],
        subject: 'Re: dinner on Thursday',
        snippet: 'Thursday works — let\'s do 7pm at the usual place…',
        trust_status: 'draft',
        created_at: new Date(previewNow - 2 * 3600000).toISOString(),
        has_attachments: false,
        draft: {
          id: 'preview-draft-1',
          status: 'draft_pending_owner_approval',
          from_address: 'preview@choir.news',
          to_addresses: ['friend@example.com'],
          subject: 'Re: dinner on Thursday',
          text_body: 'Thursday works — let\'s do 7pm at the usual place.\n\n— preview',
          updated_at: new Date(previewNow - 2 * 3600000).toISOString(),
        },
      },
    ],
    sent: [
      {
        direction: 'outbound',
        from_address: 'preview@choir.news',
        subject: 'Re: Welcome aboard',
        snippet: 'Thanks — excited to get started. I will send over the paperwork…',
        trust_status: 'trusted',
        sent_at: new Date(previewNow - 86400000).toISOString(),
        created_at: new Date(previewNow - 86400000).toISOString(),
        has_attachments: false,
      },
    ],
    quarantine: [
      {
        id: 'preview-q-1',
        direction: 'inbound',
        from_address: 'noreply@suspicious.example',
        from_display: 'Unknown Sender',
        subject: 'Invoice attached',
        snippet: 'Please find attached invoice INV-2041 due upon receipt…',
        trust_status: 'quarantined',
        read_at: '',
        received_at: new Date(previewNow - 2 * 86400000).toISOString(),
        has_attachments: true,
        attachments: [
          { id: 'preview-att-1', filename: 'INV-2041.pdf', content_type: 'application/pdf', size_bytes: 48211, status: 'quarantined' },
        ],
      },
    ],
  };
</script>

<section
  class="mail-app"
  class:mail-compact={compactLayout}
  class:mail-medium={layout === 'medium'}
  bind:clientWidth={appWidth}
  data-mail-app
>
  <aside class="mail-side" aria-label="Mailboxes">
    <div class="mail-brand">
      <span class="mail-brand-icon">{@html icon('envelope', 18)}</span>
      <div class="mail-brand-text">
        <span class="mail-brand-name">Mail</span>
        <span class="mail-brand-address" title={displayAddress}>{displayAddress}</span>
      </div>
    </div>

    <button type="button" class="mail-compose-btn" on:click={openCompose} data-mail-compose>
      {@html icon('compose', 15)}
      <span>New message</span>
    </button>

    <nav class="mail-folders">
      {#each folders as folder}
        <button
          type="button"
          class="mail-folder"
          class:mail-selected={activeFolder === folder.id}
          on:click={() => loadMessages(folder.id)}
          data-mail-folder={folder.id}
          title={folder.label}
        >
          <span class="mail-folder-icon">{@html icon(folder.icon, 16)}</span>
          <span class="mail-folder-label">{folder.label}</span>
          {#if folderBadge(folder.id, folderCounts, folderUnread)}
            <span class="mail-folder-count">{folderBadge(folder.id, folderCounts, folderUnread)}</span>
          {/if}
        </button>
      {/each}
    </nav>

    {#if !authenticated}
      <div class="mail-preview-note">
        {@html icon('info', 13)}
        <span>Preview data — sign in for your mailbox</span>
      </div>
    {/if}
  </aside>

  <div class="mail-main">
    <div class="mail-listpane" class:mail-covered={compactLayout && detailPaneOpen} data-mail-listpane>
      <header class="mail-listhead">
        {#if compactLayout}
          <div class="mail-chips" role="tablist" aria-label="Mailboxes">
            {#each folders as folder}
              <button
                type="button"
                role="tab"
                class="mail-chip"
                class:mail-selected={activeFolder === folder.id}
                aria-selected={activeFolder === folder.id}
                on:click={() => loadMessages(folder.id)}
                data-mail-folder={folder.id}
              >
                {@html icon(folder.icon, 13)}
                <span>{folder.label}</span>
                {#if folderBadge(folder.id, folderCounts, folderUnread)}
                  <span class="mail-chip-count">{folderBadge(folder.id, folderCounts, folderUnread)}</span>
                {/if}
              </button>
            {/each}
          </div>
        {/if}
        <div class="mail-listhead-row">
          <div class="mail-listhead-title">
            <h2>{activeFolderMeta.label}</h2>
            <span class="mail-listhead-count">{folderCountLine(activeFolder, folderTotals, folderCounts, folderUnread, messages)}</span>
          </div>
          <div class="mail-listhead-actions">
            <label class="mail-filter" title="Filter loaded messages">
              {@html icon('search', 13)}
              <input
                type="search"
                placeholder="Filter"
                bind:value={filter}
                data-mail-filter
                aria-label="Filter loaded messages"
              />
            </label>
            <button
              type="button"
              class="mail-icon-btn"
              title="Refresh"
              aria-label="Refresh"
              on:click={() => { void loadMessages(activeFolder); void loadFolderCounts({ background: true }); }}
              data-mail-refresh
            >{@html icon('refresh', 15)}</button>
            {#if compactLayout}
              <button
                type="button"
                class="mail-icon-btn"
                title="New message"
                aria-label="New message"
                on:click={openCompose}
                data-mail-compose
              >{@html icon('compose', 15)}</button>
            {/if}
          </div>
        </div>
      </header>

      {#if error}
        <div class="mail-error" role="alert">
          <span>{error}</span>
          <button type="button" on:click={() => void loadMessages(activeFolder)}>Retry</button>
        </div>
      {/if}

      {#if loading}
        <div class="mail-rows" aria-busy="true">
          {#each Array(6) as _}
            <div class="mail-skel-row">
              <div class="mail-skel-avatar"></div>
              <div class="mail-skel-lines">
                <div class="mail-skel-line" style="width: 45%"></div>
                <div class="mail-skel-line" style="width: 80%"></div>
                <div class="mail-skel-line mail-skel-faint" style="width: 65%"></div>
              </div>
            </div>
          {/each}
        </div>
      {:else if filteredMessages.length === 0}
        <div class="mail-empty">
          <span class="mail-empty-icon">{@html icon(activeFolderMeta.icon, 28)}</span>
          {#if filter}
            <strong>No matches for “{filter}”</strong>
            <span>Filter only searches messages already loaded.</span>
          {:else}
            <strong>Nothing in {activeFolderMeta.label}</strong>
            <span>{activeFolder === 'drafts' ? 'New messages you write land here for approval.' : 'New mail will appear here.'}</span>
          {/if}
        </div>
      {:else}
        <div class="mail-rows" on:scroll={handleListScroll} role="list" aria-label="{activeFolderMeta.label} messages">
          {#each filteredMessages as message (message.id)}
            {@const chip = trustChip(message.trust_status)}
            <button
              type="button"
              class="mail-row"
              class:mail-selected={message.id === selectedId}
              class:mail-unread={isMessageUnread(message)}
              on:click={() => loadDetail(message.id, { openPane: true })}
              data-mail-row={message.id}
            >
              <span class="mail-avatar" class:mail-avatar-draft={message.direction === 'draft' || message.direction === 'outbound'} style="--mail-avatar-color: {avatarColor(message)}">
                {#if message.direction === 'draft'}
                  {@html icon('draft', 15)}
                {:else if message.direction === 'outbound'}
                  {@html icon('sent', 15)}
                {:else}
                  {senderInitials(message)}
                {/if}
                {#if isMessageUnread(message)}
                  <span class="mail-unread-dot" aria-label="Unread"></span>
                {/if}
              </span>
              <span class="mail-row-main">
                <span class="mail-row-top">
                  <span class="mail-row-sender">{rowTitle(message)}</span>
                  <span class="mail-row-time" title={formatFullTime(messageTimestamp(message))}>{formatListTime(messageTimestamp(message))}</span>
                </span>
                <span class="mail-row-subject">{message.subject || '(no subject)'}</span>
                <span class="mail-row-bottom">
                  <span class="mail-row-snippet">{message.snippet || ''}</span>
                  {#if message.has_attachments}
                    <span class="mail-row-clip" title="Has attachments">{@html icon('clip', 12)}</span>
                  {/if}
                  {#if chip}
                    <span class="mail-chip-tag mail-chip-{chip.kind}">{chip.label}</span>
                  {/if}
                </span>
              </span>
            </button>
          {/each}
          {#if loadingMore}
            <div class="mail-more">Loading older mail…</div>
          {:else if nextCursor && !filter}
            <div class="mail-more">
              <button type="button" class="mail-more-btn" on:click={() => void loadMoreMessages()} data-mail-load-more>
                Load older messages
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <section
      class="mail-reader"
      class:mail-open={!compactLayout || detailPaneOpen}
      data-mail-reader
      aria-label="Message"
    >
      {#if compactLayout}
        <div class="mail-reader-topbar">
          <button type="button" class="mail-back" on:click={showMessageList} data-mail-back>
            {@html icon('back', 15)}
            <span>{activeFolderMeta.label}</span>
          </button>
        </div>
      {/if}

      {#if composeOpen}
        <div class="mail-compose" data-mail-compose-panel>
          <header class="mail-compose-head">
            <div>
              <h2>New message</h2>
              <span class="mail-compose-from">From {displayAddress}</span>
            </div>
            <button type="button" class="mail-icon-btn" title="Discard draft" aria-label="Close composer" disabled={attachmentBusy} on:click={() => void discardCompose()}>
              {@html icon('close', 15)}
            </button>
          </header>
          <div class="mail-compose-fields">
            <label class="mail-field">
              <span>To</span>
              <input bind:value={composeTo} type="text" inputmode="email" placeholder="name@example.com, another@example.com" data-mail-compose-to />
            </label>
            <label class="mail-field">
              <span>Subject</span>
              <input bind:value={composeSubject} type="text" placeholder="Subject" data-mail-compose-subject />
            </label>
            <label class="mail-field mail-field-grow">
              <span>Message</span>
              <textarea bind:value={composeBody} placeholder="Write your message…" data-mail-compose-body></textarea>
            </label>
            <div class="mail-compose-attachments" data-mail-attachment-strip>
              <div class="mail-compose-attachments-head">
                <span>Attachments</span>
                <div class="mail-attachment-actions">
                  <input
                    class="mail-attachment-input"
                    bind:this={attachmentInput}
                    type="file"
                    multiple
                    on:change={handleClientAttachments}
                    data-mail-attachment-input
                  />
                  <button type="button" class="mail-btn-ghost" disabled={attachmentBusy} on:click={() => attachmentInput?.click()} data-mail-attachment-upload>
                    Upload
                  </button>
                  <button type="button" class="mail-btn-ghost" disabled={attachmentBusy} on:click={() => void openFilesPicker()} data-mail-attachment-from-files>
                    From Files
                  </button>
                </div>
              </div>
              {#if stagedAttachments.length}
                <div class="mail-staged-attachments">
                  {#each stagedAttachments as attachment (attachment.id)}
                    <div class="mail-staged-attachment" data-mail-staged-attachment={attachment.id}>
                      <span class="mail-attach-icon">{@html icon(attachmentIconName(attachment), 16)}</span>
                      <span class="mail-staged-attachment-name" title={attachment.filename}>{attachment.filename}</span>
                      <span class="mail-staged-attachment-size">{formatFileSize(attachment.size_bytes)}</span>
                      <button
                        type="button"
                        class="mail-staged-attachment-remove"
                        aria-label="Remove {attachment.filename}"
                        disabled={attachmentBusy}
                        on:click={() => void removeStagedAttachment(attachment)}
                        data-mail-attachment-remove={attachment.id}
                      >{@html icon('close', 13)}</button>
                    </div>
                  {/each}
                </div>
              {:else}
                <span class="mail-compose-note">Files are added to this draft for your approval.</span>
              {/if}
              {#if attachmentBusy}
                <span class="mail-compose-note">Working with attachment…</span>
              {/if}
              {#if fromFilesOpen}
                <div class="mail-files-picker" data-mail-files-picker>
                  <div class="mail-files-picker-head">
                    <strong>Files</strong>
                    <span>/{fromFilesPath.join('/')}</span>
                    <button type="button" class="mail-icon-btn" aria-label="Close file picker" on:click={() => (fromFilesOpen = false)}>
                      {@html icon('close', 14)}
                    </button>
                  </div>
                  {#if fromFilesPath.length}
                    <button type="button" class="mail-btn-ghost" on:click={() => void loadFilesForAttachment(fromFilesPath.slice(0, -1))}>Up</button>
                  {/if}
                  {#if fromFilesError}
                    <div class="mail-error" role="alert">{fromFilesError}</div>
                  {:else if fromFilesLoading}
                    <span class="mail-compose-note">Loading files…</span>
                  {:else if !fromFilesEntries.length}
                    <span class="mail-compose-note">This folder is empty.</span>
                  {:else}
                    <div class="mail-files-picker-list">
                      {#each fromFilesEntries as entry}
                        <button
                          type="button"
                          class="mail-files-picker-entry"
                          disabled={attachmentBusy}
                          on:click={() => entry.type === 'directory'
                            ? void loadFilesForAttachment([...fromFilesPath, entry.name])
                            : void attachFromFiles(entry)}
                          data-mail-file-picker-entry={entry.name}
                        >
                          {@html icon(entry.type === 'directory' ? 'inbox' : attachmentIconName(entry), 15)}
                          <span>{entry.name}</span>
                          {#if entry.type !== 'directory'}<small>{formatFileSize(entry.size)}</small>{/if}
                        </button>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          </div>
          <footer class="mail-compose-foot">
            <span class="mail-compose-note">New mail is saved to Drafts and sends after your approval.</span>
            <button
              type="button"
              class="mail-btn-primary"
              disabled={sending || attachmentBusy || !activeAddress || !composeRecipients.length || !composeBody.trim()}
              on:click={sendCompose}
              data-mail-compose-save
            >
              {sending ? 'Saving…' : 'Save draft'}
            </button>
          </footer>
        </div>
      {:else if detailLoading}
        <div class="mail-reader-loading">
          <div class="mail-skel-line" style="width: 40%"></div>
          <div class="mail-skel-line" style="width: 70%"></div>
          <div class="mail-skel-block"></div>
        </div>
      {:else if !detail?.message}
        <div class="mail-empty mail-empty-reader">
          <span class="mail-empty-icon">{@html icon('envelope', 30)}</span>
          <strong>Select a message to read</strong>
          <span>Nothing is sent without your approval.</span>
        </div>
      {:else}
        {@const msg = detail.message}
        {@const chip = trustChip(msg.trust_status)}
        <header class="mail-reader-head">
          <div class="mail-reader-subject-row">
            <h2 class="mail-reader-subject">{msg.subject || '(no subject)'}</h2>
            {#if chip}
              <span class="mail-chip-tag mail-chip-{chip.kind} mail-chip-lg">{chip.label}</span>
            {/if}
          </div>
          <div class="mail-reader-meta">
            <span class="mail-avatar mail-avatar-lg" class:mail-avatar-draft={msg.direction === 'draft' || msg.direction === 'outbound'} style="--mail-avatar-color: {avatarColor(msg)}">
              {#if msg.direction === 'draft'}
                {@html icon('draft', 17)}
              {:else if msg.direction === 'outbound'}
                {@html icon('sent', 17)}
              {:else}
                {senderInitials(msg)}
              {/if}
            </span>
            <div class="mail-reader-meta-text">
              <div class="mail-reader-fromline">
                <strong>{msg.direction === 'draft' ? 'Draft' : (msg.from_display || msg.from_address)}</strong>
                {#if msg.from_display && msg.direction !== 'draft'}
                  <span class="mail-reader-addr">&lt;{msg.from_address}&gt;</span>
                {/if}
              </div>
              <div class="mail-reader-toline">
                to {detailToLine}
                {#if detailCcRecipients.length}
                  · cc {addressListLabel(detailCcRecipients)}
                {/if}
              </div>
            </div>
            <time class="mail-reader-time" title={formatFullTime(messageTimestamp(msg))}>
              {formatFullTime(messageTimestamp(msg))}
            </time>
          </div>
          <div class="mail-reader-actions">
            {#if isDraftView}
              {#if draftPending}
                <button type="button" class="mail-btn-primary" disabled={sending} on:click={sendDraft} data-mail-send>
                  {@html icon('send', 14)}
                  <span>{sending ? 'Sending…' : 'Approve & send'}</span>
                </button>
                <button type="button" class="mail-btn-ghost" disabled={sending} on:click={emailApprovalLink} data-mail-approval-link>
                  Email approval link
                </button>
              {:else}
                <span class="mail-chip-tag mail-chip-success mail-chip-lg">Sent</span>
              {/if}
            {:else}
              {#if msg.direction === 'inbound'}
                <button type="button" class="mail-btn-primary" on:click={() => (replyOpen = !replyOpen)} data-mail-reply>
                  {@html icon('reply', 14)}
                  <span>{replyOpen ? 'Cancel reply' : 'Reply'}</span>
                </button>
                <button
                  type="button"
                  class="mail-btn-ghost"
                  on:click={toggleReadStatus}
                  title={msg.read_at ? 'Mark as unread' : 'Mark as read'}
                  data-mail-read-toggle
                >
                  {@html icon('eye', 14)}
                  <span>{msg.read_at ? 'Mark unread' : 'Mark read'}</span>
                </button>
              {/if}
              {#if hasHtmlBody}
                <div class="mail-seg" role="group" aria-label="Body format">
                  <button type="button" class:mail-seg-on={effectiveBodyMode === 'html'} on:click={() => (bodyViewMode = 'html')} data-mail-body-html>HTML</button>
                  <button type="button" class:mail-seg-on={effectiveBodyMode === 'text'} on:click={() => (bodyViewMode = 'text')} data-mail-body-text>Text</button>
                </div>
              {/if}
            {/if}
          </div>
        </header>

        {#if isDraftView && draftPending}
          <div class="mail-draft-banner">
            {@html icon('info', 14)}
            <span>Pending your approval — nothing sends until you approve this draft.</span>
          </div>
        {/if}

        <div class="mail-body-scroll">
          {#if effectiveBodyMode === 'html' && hasHtmlBody}
            <div class="mail-body-html">
              <iframe
                class="mail-body-iframe"
                sandbox="allow-popups allow-popups-to-escape-sandbox"
                referrerpolicy="no-referrer"
                title="Email body"
                srcdoc={buildIframeContent(detail.html_body)}
              ></iframe>
            </div>
          {:else}
            <article class="mail-body-text">{detail.text_body || 'No plain text body.'}</article>
          {/if}

          {#if detail.attachments?.length}
            <div class="mail-attachments">
              <h3>Attachments <span class="mail-attach-count">{detail.attachments.length}</span></h3>
              {#each detail.attachments as attachment}
                <div class="mail-attach" data-status={attachment.status}>
                  <span class="mail-attach-icon">{@html icon(attachmentIconName(attachment), 18)}</span>
                  <div class="mail-attach-info">
                    <span class="mail-attach-name">{attachment.filename}</span>
                    <span class="mail-attach-meta">
                      {formatFileSize(attachment.size_bytes)}
                      {#if attachment.status && attachment.status !== 'trusted'}
                        · {attachment.status === 'quarantined' ? 'Quarantined — held back from your computer' : attachment.status}
                      {/if}
                    </span>
                  </div>
                  {#if canDownloadAttachment()}
                    <button type="button" class="mail-attach-action" on:click={() => void downloadAttachment(attachment)} data-mail-attachment-download={attachment.id}>
                      <span>Download</span>
                    </button>
                  {/if}
                  {#if isCalendarAttachment(attachment)}
                    <button type="button" class="mail-attach-action" on:click={() => handleAddToCalendar(attachment)}>
                      {@html icon('calendar', 13)}
                      <span>Add to Calendar</span>
                    </button>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          <details class="mail-headers" data-mail-headers>
            <summary>
              {@html icon('info', 13)}
              <span>Message details</span>
            </summary>
            <dl>
              <div><dt>From</dt><dd>{msg.from_address}</dd></div>
              <div><dt>To</dt><dd>{detailToLine}</dd></div>
              {#if detailCcRecipients.length}
                <div><dt>Cc</dt><dd>{addressListLabel(detailCcRecipients)}</dd></div>
              {/if}
              {#if detailBccRecipients.length}
                <div><dt>Bcc</dt><dd>{addressListLabel(detailBccRecipients)}</dd></div>
              {/if}
              <div><dt>Trust</dt><dd>{chip ? chip.label : 'Trusted sender'}</dd></div>
              {#each detailHeaderEntries as [key, value]}
                <div><dt>{key}</dt><dd>{value}</dd></div>
              {/each}
            </dl>
          </details>
        </div>


        {#if replyOpen && !isDraftView}
          <div class="mail-reply" data-mail-reply-box>
            <textarea bind:value={replyBody} rows="4" placeholder="Reply to {msg.from_display || msg.from_address}…" data-mail-reply-body></textarea>
            <div class="mail-reply-foot">
              <span class="mail-compose-note">Replies are saved to Drafts for approval.</span>
              <button
                type="button"
                class="mail-btn-primary"
                disabled={sending || !activeAddress || !replyBody.trim()}
                on:click={sendReply}
                data-mail-reply-send
              >
                {sending ? 'Saving…' : 'Save reply draft'}
              </button>
            </div>
          </div>
        {/if}
      {/if}
    </section>
  </div>

  {#if notice}
    <div class="mail-notice mail-notice-{noticeKind}" role="status" data-mail-notice>
      {notice}
    </div>
  {/if}
</section>

<style>
  .mail-app {
    height: 100%;
    min-height: 0;
    display: flex;
    background: var(--choir-surface-app);
    color: var(--choir-text-primary);
    overflow: hidden;
    position: relative;
    font-size: 14px;
    line-height: 1.45;
  }

  .mail-app h2,
  .mail-app h3,
  .mail-app p {
    margin: 0;
  }

  .mail-app button {
    font: inherit;
  }

  /* ---------- Side rail ---------- */

  .mail-side {
    flex: none;
    width: 212px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 16px 12px;
    background: var(--choir-surface-pane);
    border-right: 1px solid var(--choir-border);
    min-height: 0;
  }

  .mail-brand {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 2px 6px 14px;
    min-width: 0;
  }

  .mail-brand-icon {
    flex: none;
    display: grid;
    place-items: center;
    width: 32px;
    height: 32px;
    border-radius: 10px;
    background: color-mix(in srgb, var(--choir-accent) 16%, transparent);
    color: var(--choir-accent);
  }

  .mail-brand-text {
    display: grid;
    min-width: 0;
  }

  .mail-brand-name {
    font-size: 15px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .mail-brand-address {
    font-size: 11.5px;
    color: var(--choir-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-compose-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    margin: 0 4px 14px;
    padding: 9px 12px;
    border-radius: var(--choir-radius-control, 12px);
    background: var(--choir-accent) !important;
    color: var(--choir-text-on-accent) !important;
    font-weight: 650;
    font-size: 13px;
    cursor: pointer;
    box-shadow: 0 6px 18px color-mix(in srgb, var(--choir-accent) 24%, transparent) !important;
    transition: filter var(--choir-motion-fast, 120ms ease);
  }

  .mail-compose-btn:hover {
    filter: brightness(1.08);
  }

  .mail-folders {
    display: grid;
    gap: 2px;
  }

  .mail-folder {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border-radius: 10px;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-text-muted) !important;
    font-size: 13px;
    font-weight: 550;
    cursor: pointer;
    text-align: left;
    transition: background var(--choir-motion-fast, 120ms ease), color var(--choir-motion-fast, 120ms ease);
  }

  .mail-folder:hover {
    background: var(--choir-state-hover) !important;
    color: var(--choir-text-primary) !important;
  }

  .mail-folder.mail-selected {
    background: color-mix(in srgb, var(--choir-accent) 14%, transparent) !important;
    color: var(--choir-text-primary) !important;
  }

  .mail-folder.mail-selected .mail-folder-icon {
    color: var(--choir-accent);
  }

  .mail-folder-icon {
    flex: none;
    display: grid;
    place-items: center;
    width: 18px;
    color: inherit;
    opacity: 0.85;
  }

  .mail-folder-label {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-folder-count {
    flex: none;
    min-width: 20px;
    padding: 1px 6px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--choir-accent) 18%, transparent);
    color: var(--choir-accent);
    font-size: 11px;
    font-weight: 700;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }

  .mail-preview-note {
    margin-top: auto;
    display: flex;
    gap: 8px;
    align-items: flex-start;
    padding: 10px;
    border-radius: 10px;
    background: color-mix(in srgb, var(--choir-accent) 8%, transparent);
    color: var(--choir-text-muted);
    font-size: 11.5px;
    line-height: 1.4;
  }

  .mail-preview-note :global(svg) {
    flex: none;
    margin-top: 1px;
    color: var(--choir-accent);
  }

  /* ---------- Main column ---------- */

  .mail-main {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    position: relative;
  }

  .mail-listpane {
    flex: none;
    width: 348px;
    display: flex;
    flex-direction: column;
    min-height: 0;
    border-right: 1px solid var(--choir-border);
    background: var(--choir-surface-app);
  }

  .mail-listhead {
    flex: none;
    border-bottom: 1px solid var(--choir-border);
    padding: 12px 14px 10px;
    display: grid;
    gap: 10px;
  }

  .mail-listhead-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    min-width: 0;
  }

  .mail-listhead-title {
    display: grid;
    min-width: 0;
  }

  .mail-listhead-title h2 {
    font-size: 16px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .mail-listhead-count {
    font-size: 11.5px;
    color: var(--choir-text-muted);
    font-variant-numeric: tabular-nums;
  }

  .mail-listhead-actions {
    display: flex;
    align-items: center;
    gap: 6px;
    flex: none;
  }

  .mail-filter {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 9px;
    height: 30px;
    border-radius: 999px;
    background: var(--choir-surface-control);
    color: var(--choir-text-muted);
    min-width: 0;
  }

  .mail-filter input {
    width: 84px;
    min-width: 0;
    border: 0;
    padding: 0;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-text-primary) !important;
    font-size: 12.5px;
  }

  .mail-filter input::placeholder {
    color: var(--choir-text-subtle);
  }

  .mail-filter input::-webkit-search-cancel-button {
    -webkit-appearance: none;
  }

  .mail-icon-btn {
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border-radius: 9px;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-text-muted) !important;
    cursor: pointer;
    transition: background var(--choir-motion-fast, 120ms ease), color var(--choir-motion-fast, 120ms ease);
  }

  .mail-icon-btn:hover {
    background: var(--choir-state-hover) !important;
    color: var(--choir-text-primary) !important;
  }

  /* ---------- Message rows ---------- */

  .mail-rows {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 4px 6px 10px;
  }

  .mail-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    width: 100%;
    padding: 10px 10px;
    border-radius: 12px;
    background: transparent !important;
    box-shadow: none !important;
    color: inherit !important;
    text-align: left;
    cursor: pointer;
    transition: background var(--choir-motion-fast, 120ms ease);
  }

  .mail-row:hover {
    background: var(--choir-state-hover) !important;
  }

  .mail-row.mail-selected {
    background: color-mix(in srgb, var(--choir-accent) 12%, transparent) !important;
    box-shadow: inset 2px 0 0 var(--choir-accent) !important;
  }

  .mail-avatar {
    position: relative;
    flex: none;
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    border-radius: 50%;
    background:
      linear-gradient(
        135deg,
        color-mix(in srgb, var(--mail-avatar-color, var(--choir-accent)) 34%, transparent),
        color-mix(in srgb, var(--mail-avatar-color, var(--choir-accent)) 16%, transparent)
      ),
      var(--choir-surface-card);
    color: var(--choir-text-primary);
    font-size: 12px;
    font-weight: 700;
    letter-spacing: 0.02em;
  }

  .mail-avatar-draft {
    background: color-mix(in srgb, var(--choir-accent) 14%, transparent);
    color: var(--choir-accent);
  }

  .mail-avatar-lg {
    width: 40px;
    height: 40px;
    font-size: 13px;
  }

  .mail-unread-dot {
    position: absolute;
    top: -1px;
    right: -1px;
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--choir-accent);
    box-shadow: 0 0 0 2px var(--choir-surface-app);
  }

  .mail-row-main {
    flex: 1;
    min-width: 0;
    display: grid;
    gap: 1px;
  }

  .mail-row-top {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
  }

  .mail-row-sender {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    font-weight: 550;
    color: var(--choir-text-muted);
  }

  .mail-unread .mail-row-sender {
    font-weight: 720;
    color: var(--choir-text-primary);
  }

  .mail-row-time {
    flex: none;
    font-size: 11px;
    color: var(--choir-text-subtle);
    font-variant-numeric: tabular-nums;
  }

  .mail-unread .mail-row-time {
    color: var(--choir-accent);
    font-weight: 650;
  }

  .mail-row-subject {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 13px;
    color: var(--choir-text-muted);
  }

  .mail-unread .mail-row-subject {
    color: var(--choir-text-primary);
    font-weight: 600;
  }

  .mail-row-bottom {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  .mail-row-snippet {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
    color: var(--choir-text-subtle);
  }

  .mail-row-clip {
    flex: none;
    display: grid;
    place-items: center;
    color: var(--choir-text-subtle);
  }

  /* ---------- Trust chips ---------- */

  .mail-chip-tag {
    flex: none;
    padding: 1.5px 7px;
    border-radius: 999px;
    font-size: 10.5px;
    font-weight: 700;
    letter-spacing: 0.01em;
    white-space: nowrap;
  }

  .mail-chip-lg {
    padding: 3px 10px;
    font-size: 11.5px;
  }

  .mail-chip-neutral {
    background: color-mix(in srgb, var(--choir-text-muted) 14%, transparent);
    color: var(--choir-text-muted);
  }

  .mail-chip-accent {
    background: color-mix(in srgb, var(--choir-accent) 16%, transparent);
    color: var(--choir-accent);
  }

  .mail-chip-warn {
    background: var(--choir-status-warning-soft);
    color: var(--choir-status-warning);
  }

  .mail-chip-danger {
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  .mail-chip-success {
    background: var(--choir-status-success-soft);
    color: var(--choir-status-success);
  }

  /* ---------- Reader ---------- */

  .mail-reader {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--choir-surface-app);
  }

  .mail-reader-topbar {
    flex: none;
    padding: 10px 12px 0;
  }

  .mail-back {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 7px 12px 7px 8px;
    border-radius: 999px;
    background: var(--choir-surface-control) !important;
    color: var(--choir-text-primary) !important;
    font-size: 12.5px;
    font-weight: 600;
    cursor: pointer;
  }

  .mail-reader-head {
    flex: none;
    padding: 16px 22px 12px;
    border-bottom: 1px solid var(--choir-border);
    display: grid;
    gap: 12px;
  }

  .mail-reader-subject-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    min-width: 0;
  }

  .mail-reader-subject {
    flex: 1;
    min-width: 0;
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
    line-height: 1.3;
    overflow-wrap: anywhere;
  }

  .mail-reader-meta {
    display: flex;
    align-items: center;
    gap: 11px;
    min-width: 0;
  }

  .mail-reader-meta-text {
    flex: 1;
    min-width: 0;
    display: grid;
    gap: 1px;
  }

  .mail-reader-fromline {
    display: flex;
    align-items: baseline;
    gap: 6px;
    min-width: 0;
    font-size: 13.5px;
  }

  .mail-reader-fromline strong {
    font-weight: 650;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-reader-addr {
    color: var(--choir-text-muted);
    font-size: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-reader-toline {
    font-size: 12px;
    color: var(--choir-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-reader-time {
    flex: none;
    font-size: 11.5px;
    color: var(--choir-text-subtle);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }

  .mail-reader-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .mail-btn-primary {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 7px 14px;
    border-radius: var(--choir-radius-control, 12px);
    background: var(--choir-accent) !important;
    color: var(--choir-text-on-accent) !important;
    font-size: 12.5px;
    font-weight: 650;
    cursor: pointer;
    box-shadow: 0 4px 14px color-mix(in srgb, var(--choir-accent) 20%, transparent) !important;
    transition: filter var(--choir-motion-fast, 120ms ease);
  }

  .mail-btn-primary:hover:not(:disabled) {
    filter: brightness(1.08);
  }

  .mail-btn-ghost {
    display: inline-flex;
    align-items: center;
    gap: 7px;
    padding: 7px 12px;
    border-radius: var(--choir-radius-control, 12px);
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-text-muted) !important;
    font-size: 12.5px;
    font-weight: 600;
    cursor: pointer;
    transition: background var(--choir-motion-fast, 120ms ease), color var(--choir-motion-fast, 120ms ease);
  }

  .mail-btn-ghost:hover:not(:disabled) {
    background: var(--choir-state-hover) !important;
    color: var(--choir-text-primary) !important;
  }

  .mail-seg {
    display: inline-flex;
    border-radius: 9px;
    background: var(--choir-surface-control);
    padding: 2px;
    gap: 2px;
  }

  .mail-seg button {
    padding: 4px 11px;
    border-radius: 7px;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-text-muted) !important;
    font-size: 11.5px;
    font-weight: 650;
    cursor: pointer;
  }

  .mail-seg button.mail-seg-on {
    background: var(--choir-surface-app) !important;
    color: var(--choir-text-primary) !important;
    box-shadow: var(--choir-control-shadow) !important;
  }

  .mail-draft-banner {
    flex: none;
    display: flex;
    align-items: center;
    gap: 9px;
    margin: 12px 22px 0;
    padding: 9px 13px;
    border-radius: 10px;
    background: color-mix(in srgb, var(--choir-accent) 10%, transparent);
    color: var(--choir-text-primary);
    font-size: 12.5px;
  }

  .mail-draft-banner :global(svg) {
    flex: none;
    color: var(--choir-accent);
  }

  .mail-body-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 16px 22px 22px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .mail-body-html {
    flex: 1;
    min-height: 320px;
    border-radius: 14px;
    overflow: hidden;
    background: #ffffff;
    box-shadow: var(--choir-shadow-soft);
  }

  .mail-body-iframe {
    display: block;
    width: 100%;
    height: 100%;
    min-height: inherit;
    border: 0;
    background: #ffffff;
  }

  .mail-body-text {
    max-width: 72ch;
    font-size: 14.5px;
    line-height: 1.68;
    white-space: pre-wrap;
    overflow-wrap: break-word;
    color: var(--choir-text-primary);
  }

  /* ---------- Attachments & headers ---------- */

  .mail-attachments {
    display: grid;
    gap: 8px;
  }

  .mail-attachments h3 {
    font-size: 11.5px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--choir-text-muted);
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .mail-attach-count {
    font-variant-numeric: tabular-nums;
  }

  .mail-attach {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 13px;
    border-radius: 12px;
    background: var(--choir-surface-card);
    box-shadow: var(--choir-shadow-soft);
  }

  .mail-attach[data-status='quarantined'] {
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--choir-status-warning) 35%, transparent);
  }

  .mail-attach-icon {
    flex: none;
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    border-radius: 9px;
    background: color-mix(in srgb, var(--choir-accent) 12%, transparent);
    color: var(--choir-accent);
  }

  .mail-attach[data-status='quarantined'] .mail-attach-icon {
    background: var(--choir-status-warning-soft);
    color: var(--choir-status-warning);
  }

  .mail-attach-info {
    flex: 1;
    min-width: 0;
    display: grid;
    gap: 1px;
  }

  .mail-attach-name {
    font-size: 13px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-attach-meta {
    font-size: 11.5px;
    color: var(--choir-text-muted);
  }

  .mail-attach-action {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 11px;
    border-radius: 9px;
    background: color-mix(in srgb, var(--choir-accent) 12%, transparent) !important;
    box-shadow: none !important;
    color: var(--choir-accent) !important;
    font-size: 12px;
    font-weight: 650;
    cursor: pointer;
  }

  .mail-headers {
    border-radius: 12px;
    background: var(--choir-surface-card);
    overflow: hidden;
  }

  .mail-headers summary {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 13px;
    cursor: pointer;
    font-size: 12.5px;
    font-weight: 600;
    color: var(--choir-text-muted);
    list-style: none;
  }

  .mail-headers summary::-webkit-details-marker {
    display: none;
  }

  .mail-headers summary:hover {
    color: var(--choir-text-primary);
  }

  .mail-headers dl {
    display: grid;
    gap: 7px;
    margin: 0;
    padding: 2px 13px 13px;
  }

  .mail-headers dl div {
    display: grid;
    grid-template-columns: minmax(96px, 0.3fr) minmax(0, 1fr);
    gap: 12px;
    font-size: 12px;
  }

  .mail-headers dt {
    color: var(--choir-text-muted);
    overflow-wrap: anywhere;
  }

  .mail-headers dd {
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--choir-text-primary);
    font-family: var(--choir-font-mono, ui-monospace, monospace);
    font-size: 11.5px;
  }

  /* ---------- Reply & compose ---------- */

  .mail-reply {
    flex: none;
    margin: 0 22px 16px;
    padding: 12px;
    border-radius: 14px;
    background: var(--choir-surface-card);
    box-shadow: var(--choir-shadow-soft);
    display: grid;
    gap: 10px;
  }

  .mail-reply textarea {
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    min-height: 96px;
    font-size: 13.5px;
    line-height: 1.55;
  }

  .mail-reply-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    flex-wrap: wrap;
  }

  .mail-compose {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .mail-compose-head {
    flex: none;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 12px;
    padding: 18px 22px 12px;
    border-bottom: 1px solid var(--choir-border);
  }

  .mail-compose-head h2 {
    font-size: 17px;
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .mail-compose-from {
    font-size: 12px;
    color: var(--choir-text-muted);
  }

  .mail-compose-fields {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 16px 22px;
  }

  .mail-field {
    display: grid;
    gap: 6px;
    font-size: 12px;
    font-weight: 650;
    color: var(--choir-text-muted);
  }

  .mail-field input,
  .mail-field textarea {
    font-size: 13.5px;
    font-weight: 500;
    color: var(--choir-text-primary) !important;
    padding: 10px 12px;
    border-radius: 10px;
  }

  .mail-field-grow {
    flex: 1;
    min-height: 0;
  }

  .mail-field-grow textarea {
    flex: 1;
    min-height: 160px;
    resize: none;
    line-height: 1.55;
  }

  .mail-compose-foot {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 12px 22px 16px;
    border-top: 1px solid var(--choir-border);
    flex-wrap: wrap;
  }

  .mail-compose-note {
    font-size: 11.5px;
    color: var(--choir-text-subtle);
  }

  /* ---------- States ---------- */

  .mail-empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    padding: 32px 24px;
    text-align: center;
    color: var(--choir-text-muted);
  }

  .mail-empty strong {
    font-size: 14px;
    font-weight: 650;
    color: var(--choir-text-primary);
  }

  .mail-empty span {
    font-size: 12.5px;
    max-width: 34ch;
  }

  .mail-empty-icon {
    display: grid;
    place-items: center;
    width: 56px;
    height: 56px;
    border-radius: 18px;
    background: color-mix(in srgb, var(--choir-accent) 10%, transparent);
    color: var(--choir-accent);
    margin-bottom: 8px;
  }

  .mail-error {
    flex: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin: 10px 12px 0;
    padding: 9px 12px;
    border-radius: 10px;
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
    font-size: 12.5px;
  }

  .mail-error button {
    flex: none;
    padding: 4px 10px;
    border-radius: 7px;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-status-danger) !important;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    text-decoration: underline;
  }

  .mail-more {
    padding: 10px;
    text-align: center;
    font-size: 12px;
    color: var(--choir-text-muted);
  }

  .mail-more-btn {
    padding: 7px 14px;
    border-radius: 999px;
    background: var(--choir-surface-control) !important;
    color: var(--choir-text-primary) !important;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }

  .mail-skel-row {
    display: flex;
    gap: 10px;
    padding: 12px 10px;
  }

  .mail-skel-avatar {
    flex: none;
    width: 34px;
    height: 34px;
    border-radius: 50%;
    background: var(--choir-surface-card);
  }

  .mail-skel-lines {
    flex: 1;
    display: grid;
    gap: 6px;
    align-content: start;
  }

  .mail-skel-line {
    height: 10px;
    border-radius: 5px;
    background: var(--choir-surface-card);
    animation: mail-shimmer 1.4s ease-in-out infinite;
  }

  .mail-skel-faint {
    opacity: 0.6;
  }

  .mail-skel-block {
    margin: 16px 22px;
    flex: 1;
    border-radius: 14px;
    background: var(--choir-surface-card);
    animation: mail-shimmer 1.4s ease-in-out infinite;
  }

  .mail-reader-loading {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 22px;
  }

  .mail-reader-loading .mail-skel-line {
    margin: 0;
  }

  @keyframes mail-shimmer {
    0%, 100% { opacity: 0.55; }
    50% { opacity: 1; }
  }

  .mail-notice {
    position: absolute;
    left: 50%;
    bottom: 18px;
    transform: translateX(-50%);
    z-index: 30;
    padding: 9px 16px;
    border-radius: 999px;
    background: var(--choir-surface-pane);
    color: var(--choir-text-primary);
    font-size: 12.5px;
    font-weight: 600;
    box-shadow: var(--choir-shadow-floating);
    animation: mail-notice-in 180ms ease;
    max-width: min(90%, 480px);
    text-align: center;
  }

  .mail-notice-error {
    background: var(--choir-status-danger-soft);
    color: var(--choir-status-danger);
  }

  @keyframes mail-notice-in {
    from { opacity: 0; transform: translateX(-50%) translateY(6px); }
    to { opacity: 1; transform: translateX(-50%) translateY(0); }
  }

  /* ---------- Medium layout: icon rail ---------- */

  .mail-medium .mail-side {
    width: 60px;
    padding: 14px 8px;
    align-items: center;
  }

  .mail-medium .mail-brand {
    padding: 0 0 12px;
  }

  .mail-medium .mail-brand-text,
  .mail-medium .mail-folder-label,
  .mail-medium .mail-preview-note span {
    display: none;
  }

  .mail-medium .mail-compose-btn {
    width: 40px;
    height: 40px;
    padding: 0;
    margin: 0 0 14px;
    border-radius: 12px;
  }

  .mail-medium .mail-compose-btn span {
    display: none;
  }

  .mail-medium .mail-folder {
    justify-content: center;
    padding: 9px;
    width: 42px;
    position: relative;
  }

  .mail-medium .mail-folder-count {
    position: absolute;
    top: 2px;
    right: 0;
    min-width: 15px;
    padding: 0 4px;
    font-size: 9.5px;
  }

  .mail-medium .mail-listpane {
    width: 300px;
  }

  /* ---------- Compact layout: single column ---------- */

  .mail-compact {
    flex-direction: column;
  }

  .mail-compact .mail-side {
    display: none;
  }

  .mail-compact .mail-main {
    flex: 1;
  }

  .mail-compact .mail-listpane {
    width: 100%;
    border-right: 0;
  }

  .mail-compact .mail-listpane.mail-covered {
    visibility: hidden;
  }

  .mail-compact .mail-reader {
    position: absolute;
    inset: 0;
    display: none;
    z-index: 10;
  }

  .mail-compact .mail-reader.mail-open {
    display: flex;
    animation: mail-reader-in 200ms cubic-bezier(0.2, 0.8, 0.3, 1);
  }

  @keyframes mail-reader-in {
    from { transform: translateX(4%); opacity: 0.4; }
    to { transform: translateX(0); opacity: 1; }
  }

  .mail-chips {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    padding-bottom: 2px;
    scrollbar-width: none;
  }

  .mail-compact .mail-listhead-title h2 {
    display: none;
  }

  .mail-compact .mail-listhead-count {
    font-size: 11px;
    white-space: nowrap;
  }

  .mail-chips::-webkit-scrollbar {
    display: none;
  }

  .mail-chip {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 6px 11px;
    border-radius: 999px;
    background: var(--choir-surface-control) !important;
    color: var(--choir-text-muted) !important;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
  }

  .mail-chip.mail-selected {
    background: color-mix(in srgb, var(--choir-accent) 18%, transparent) !important;
    color: var(--choir-text-primary) !important;
  }

  .mail-chip-count {
    min-width: 16px;
    padding: 0 5px;
    border-radius: 999px;
    background: color-mix(in srgb, var(--choir-accent) 20%, transparent);
    color: var(--choir-accent);
    font-size: 10.5px;
    font-weight: 700;
    text-align: center;
    font-variant-numeric: tabular-nums;
  }

  .mail-compact .mail-listhead {
    padding: 10px 12px 8px;
    gap: 8px;
  }

  .mail-compact .mail-filter {
    flex: 1;
    max-width: 180px;
  }

  .mail-compact .mail-filter input {
    width: 100%;
  }

  .mail-compact .mail-reader-head {
    padding: 12px 16px 10px;
    gap: 10px;
  }

  .mail-compact .mail-reader-subject {
    font-size: 15.5px;
  }

  .mail-compact .mail-reader-time {
    display: none;
  }

  .mail-compact .mail-body-scroll {
    padding: 12px 14px 18px;
  }

  .mail-compact .mail-body-html {
    min-height: 260px;
  }

  .mail-compact .mail-draft-banner {
    margin: 10px 14px 0;
  }

  .mail-compact .mail-reply {
    margin: 0 14px 12px;
  }

  .mail-compact .mail-compose-head,
  .mail-compact .mail-compose-fields {
    padding-left: 16px;
    padding-right: 16px;
  }

  .mail-compact .mail-compose-foot {
    padding: 10px 16px 14px;
  }

  .mail-compact .mail-headers dl div {
    grid-template-columns: 1fr;
    gap: 2px;
  }

  .mail-compact .mail-attach {
    flex-wrap: wrap;
  }

  .mail-compose-attachments {
    display: grid;
    gap: 9px;
    padding: 12px;
    border: 1px solid var(--choir-border);
    border-radius: 12px;
    background: var(--choir-surface-card);
  }

  .mail-compose-attachments-head,
  .mail-attachment-actions,
  .mail-staged-attachment,
  .mail-files-picker-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .mail-compose-attachments-head {
    justify-content: space-between;
    font-size: 12px;
    font-weight: 650;
    color: var(--choir-text-muted);
  }

  .mail-attachment-input {
    display: none;
  }

  .mail-staged-attachments,
  .mail-files-picker-list {
    display: grid;
    gap: 6px;
  }

  .mail-staged-attachment {
    min-width: 0;
    padding: 7px 8px;
    border-radius: 9px;
    background: var(--choir-surface-control);
  }

  .mail-staged-attachment .mail-attach-icon {
    width: 26px;
    height: 26px;
    border-radius: 7px;
  }

  .mail-staged-attachment-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
    color: var(--choir-text-primary);
  }

  .mail-staged-attachment-size {
    flex: none;
    font-size: 11px;
    color: var(--choir-text-muted);
  }

  .mail-staged-attachment-remove {
    flex: none;
    display: grid;
    place-items: center;
    width: 26px;
    height: 26px;
    padding: 0;
    border-radius: 7px;
    background: transparent !important;
    box-shadow: none !important;
    color: var(--choir-text-muted) !important;
    cursor: pointer;
  }

  .mail-staged-attachment-remove:hover:not(:disabled) {
    background: var(--choir-state-hover) !important;
    color: var(--choir-status-danger) !important;
  }

  .mail-files-picker {
    display: grid;
    gap: 8px;
    padding: 10px;
    border-radius: 10px;
    background: var(--choir-surface-control);
  }

  .mail-files-picker-head strong {
    color: var(--choir-text-primary);
    font-size: 12px;
  }

  .mail-files-picker-head > span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 11px;
    color: var(--choir-text-muted);
  }

  .mail-files-picker-entry {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding: 7px 8px;
    border-radius: 8px;
    background: var(--choir-surface-card) !important;
    color: var(--choir-text-primary) !important;
    box-shadow: none !important;
    cursor: pointer;
    text-align: left;
  }

  .mail-files-picker-entry:hover:not(:disabled) {
    background: var(--choir-state-hover) !important;
  }

  .mail-files-picker-entry > span {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mail-files-picker-entry small {
    flex: none;
    color: var(--choir-text-muted);
  }

  @media (prefers-reduced-motion: reduce) {
    .mail-app *,
    .mail-app *::before,
    .mail-app *::after {
      animation-duration: 0.01ms !important;
      animation-iteration-count: 1 !important;
      transition-duration: 0.01ms !important;
    }
  }
</style>
