<script lang="ts">
  import { createEventDispatcher, onMount } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';
  import { demoEmailMessages } from './demo-fixtures';

  export let authenticated = false;
  export let appContext = {};

  const dispatch = createEventDispatcher();

  const folders = [
    { id: 'inbox', label: 'Inbox' },
    { id: 'drafts', label: 'Drafts' },
    { id: 'sent', label: 'Sent' },
    { id: 'quarantine', label: 'Quarantine' },
  ];

  let aliases = [];
  let activeFolder = 'inbox';
  let messages = [];
  let selectedId = '';
  let detail = null;
  let loading = false;
  let detailLoading = false;
  let error = '';
  let actionStatus = '';
  let replyOpen = false;
  let replyBody = '';
  let composeOpen = false;
  let composeTo = '';
  let composeSubject = '';
  let composeBody = '';
  let sending = false;
  let loadedOnce = false;
  let detailPaneOpen = false;
  let openedContextDraftId = '';

  $: selectedMessage = messages.find((message) => message.id === selectedId) || null;
  $: activeAddress = aliases[0]?.address || '';
  $: displayAddress = activeAddress || 'No address';
  $: detailHeaderEntries = headerEntries(detail?.raw_headers);
  $: detailToRecipients = detail?.recipients?.to || [];
  $: detailCcRecipients = detail?.recipients?.cc || [];
  $: detailBccRecipients = detail?.recipients?.bcc || [];
  $: detailToLine = addressListLabel(detailToRecipients) || activeAddress;
  $: composeRecipients = parseAddressList(composeTo);

  $: if (authenticated && appContext?.draftId && openedContextDraftId !== appContext.draftId) {
    void openContextDraft(appContext.draftId);
  }

  onMount(() => {
    if (!authenticated) {
      loadPreviewMailbox();
    } else {
      void loadAliases();
      void loadMessages(activeFolder);
    }
  });

  $: if (authenticated && !loadedOnce && !loading) {
    void loadAliases();
    void loadMessages(activeFolder);
  }

  async function loadAliases() {
    try {
      const res = await fetchWithRenewal('/api/email/aliases');
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not load addresses');
      }
      const data = await res.json();
      aliases = data.aliases || [];
    } catch (err) {
      handleError(err);
    }
  }

  function loadPreviewMailbox() {
    aliases = [{ address: 'preview@choir-ip.com' }];
    activeFolder = 'inbox';
    messages = demoEmailMessages;
    selectedId = messages[0]?.id || '';
    detail = selectedId ? {
      ...messages[0],
      text_body: `${messages[0].snippet}\n\nThis is a frontend-only mailbox preview. Real drafts, send, and mailbox data require sign-in.`,
      recipients: { to: [{ display: 'Owner', address: 'preview@choir-ip.com' }], cc: [], bcc: [] },
      raw_headers: { 'X-Choir-Preview': 'fixture' },
    } : null;
    loadedOnce = true;
  }

  async function loadMessages(folder) {
    if (!authenticated) {
      loadPreviewMailbox();
      activeFolder = folder;
      return;
    }
    loading = true;
    error = '';
    activeFolder = folder;
    detailPaneOpen = false;
    try {
      if (folder === 'drafts') {
        await loadDrafts();
        return;
      }
      const res = await fetchWithRenewal(`/api/email/messages?folder=${encodeURIComponent(folder)}`);
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not load mail');
      }
      const data = await res.json();
      loadedOnce = true;
      messages = data.messages || [];
      if (!messages.some((message) => message.id === selectedId)) {
        selectedId = messages[0]?.id || '';
        detail = null;
      }
      if (selectedId) {
        await loadDetail(selectedId, { openPane: false });
      }
    } catch (err) {
      handleError(err);
    } finally {
      loading = false;
    }
  }

  async function loadDrafts() {
    const res = await fetchWithRenewal('/api/email/drafts');
    if (!res.ok) {
      if (res.status === 401) throw new AuthRequiredError();
      throw new Error('Could not load drafts');
    }
    const data = await res.json();
    loadedOnce = true;
    messages = (data.drafts || []).map(draftListItem);
    if (!messages.some((message) => message.id === selectedId)) {
      selectedId = messages[0]?.id || '';
      detail = null;
    }
    if (selectedId) {
      await loadDetail(selectedId, { openPane: false });
    }
  }

  async function openContextDraft(draftId) {
    const id = String(draftId || '').trim();
    if (!id) return;
    openedContextDraftId = id;
    await loadMessages('drafts');
    await loadDetail(id, { openPane: true });
  }

  async function loadDetail(id, options = {}) {
    selectedId = id;
    detailLoading = true;
    actionStatus = '';
    replyOpen = false;
    composeOpen = false;
    if (options.openPane) {
      detailPaneOpen = true;
    }
    try {
      if (activeFolder === 'drafts') {
        const res = await fetchWithRenewal(`/api/email/drafts/${encodeURIComponent(id)}`);
        if (!res.ok) {
          if (res.status === 401) throw new AuthRequiredError();
          throw new Error('Could not open draft');
        }
        const draft = await res.json();
        detail = draftDetail(draft);
        return;
      }
      const res = await fetchWithRenewal(`/api/email/messages/${encodeURIComponent(id)}`);
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not open message');
      }
      detail = await res.json();
    } catch (err) {
      handleError(err);
    } finally {
      detailLoading = false;
    }
  }

  async function sendReply() {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'email_reply', appId: 'email', appName: 'Email' });
      return;
    }
    if (!selectedMessage || !replyBody.trim() || !activeAddress) return;
    sending = true;
    actionStatus = '';
    try {
      const res = await fetchWithRenewal('/api/email/drafts', {
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
      actionStatus = 'Reply draft ready for review';
      activeFolder = 'drafts';
      selectedId = draft.id;
      await loadMessages('drafts');
      await loadDetail(draft.id, { openPane: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function sendCompose() {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'email_compose', appId: 'email', appName: 'Email' });
      return;
    }
    if (!composeRecipients.length || !composeBody.trim() || !activeAddress) return;
    sending = true;
    actionStatus = '';
    try {
      const res = await fetchWithRenewal('/api/email/drafts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_address: activeAddress,
          to_addresses: composeRecipients,
          subject: composeSubject.trim(),
          text_body: composeBody.trim(),
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
      actionStatus = 'Draft ready for review';
      activeFolder = 'drafts';
      selectedId = draft.id;
      await loadMessages('drafts');
      await loadDetail(draft.id, { openPane: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function sendDraft() {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'email_send', appId: 'email', appName: 'Email' });
      return;
    }
    const draftId = detail?.draft?.id;
    if (!draftId) return;
    sending = true;
    actionStatus = '';
    try {
      const res = await fetchWithRenewal(`/api/email/drafts/${encodeURIComponent(draftId)}/send`, {
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
      actionStatus = 'Draft sent';
      await loadMessages('sent');
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function emailApprovalLink() {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'email_send', appId: 'email', appName: 'Email' });
      return;
    }
    const draftId = detail?.draft?.id;
    if (!draftId) return;
    sending = true;
    actionStatus = '';
    try {
      const res = await fetchWithRenewal(`/api/email/drafts/${encodeURIComponent(draftId)}/approval-email`, {
        method: 'POST',
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not send approval email');
      }
      actionStatus = 'Approval email sent to your account email';
      await loadDetail(draftId, { openPane: true });
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  function openCompose() {
    if (!authenticated) {
      dispatch('authrequired', { kind: 'email_compose', appId: 'email', appName: 'Email' });
      return;
    }
    composeOpen = true;
    replyOpen = false;
    detailPaneOpen = true;
    actionStatus = '';
    error = '';
  }

  function showMessageList() {
    detailPaneOpen = false;
    composeOpen = false;
    replyOpen = false;
    actionStatus = '';
  }

  function handleError(err) {
    if (err instanceof AuthRequiredError) {
      dispatch('authexpired');
      return;
    }
    error = err?.message || 'Mail action failed';
    actionStatus = '';
  }

  function trustLabel(status) {
    if (status === 'draft') return 'Pending approval';
    if (status === 'trusted') return 'Trusted sender';
    if (status === 'quarantined') return 'Attachment quarantined';
    if (status === 'public') return 'Public inbound';
    return 'Untrusted source';
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
      subject: draft.subject,
      snippet: draft.text_body,
      trust_status: 'draft',
      created_at: draft.updated_at || draft.created_at,
      sent_at: '',
      received_at: '',
      has_attachments: false,
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
      attachments: [],
    };
  }

  function formatTime(value) {
    if (!value) return '';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return date.toLocaleString([], { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' });
  }
</script>

<section class="email-app" data-email-app>
  <aside class="mail-rail">
    <div class="mail-title">
      <div>
        <h1>Email</h1>
        <p>{displayAddress}</p>
      </div>
      <span class="address-dot" aria-hidden="true"></span>
    </div>

    <nav class="folder-list" aria-label="Mailboxes">
      {#each folders as folder}
        <button
          type="button"
          class:active={activeFolder === folder.id}
          on:click={() => loadMessages(folder.id)}
          data-email-folder={folder.id}
        >
          <span>{folder.label}</span>
        </button>
      {/each}
    </nav>
  </aside>

  <div class="mobile-mailbar">
    <div>
      <h1>Email</h1>
      <p>{displayAddress}</p>
    </div>
    <label>
      <span>Mailbox</span>
      <select bind:value={activeFolder} on:change={(event) => loadMessages(event.currentTarget.value)}>
        {#each folders as folder}
          <option value={folder.id}>{folder.label}</option>
        {/each}
      </select>
    </label>
  </div>

  <main class="message-list" class:mobile-hidden={detailPaneOpen} data-email-message-list>
    <header class="list-header">
      <div>
        <h2>{folders.find((folder) => folder.id === activeFolder)?.label || 'Inbox'}</h2>
        <p>{messages.length} {activeFolder === 'drafts' ? 'drafts' : 'messages'}</p>
      </div>
      <div class="list-actions">
        <button type="button" on:click={openCompose} data-email-compose>Compose</button>
        <button type="button" class="icon-button" title="Refresh" on:click={() => loadMessages(activeFolder)}>↻</button>
      </div>
    </header>

    {#if error}
      <div class="mail-error" role="status">{error}</div>
    {/if}

    {#if loading}
      <div class="empty-state">Loading mail...</div>
    {:else if messages.length === 0}
      <div class="empty-state">No messages</div>
    {:else}
      <div class="rows">
        {#each messages as message}
          <button
            type="button"
            class="message-row"
            class:selected={message.id === selectedId}
            on:click={() => loadDetail(message.id, { openPane: true })}
            data-email-message-id={message.id}
          >
            <span class="sender">{message.from_display || message.from_address}</span>
            <span class="time">{formatTime(message.received_at || message.sent_at || message.created_at)}</span>
            <span class="subject">{message.subject || '(no subject)'}</span>
            <span class="snippet">{message.snippet}</span>
            <span class="trust">{trustLabel(message.trust_status)}</span>
            {#if message.has_attachments}
              <span class="attachment-indicator" title="Has attachments" aria-label="Has attachments">📎</span>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  </main>

  <section class="message-detail" class:mobile-open={detailPaneOpen} data-email-message-detail>
    <button type="button" class="mobile-back" on:click={showMessageList}>Back</button>

    {#if composeOpen}
      <div class="compose-box" data-email-compose-panel>
        <header class="compose-header">
          <div>
            <h2>New draft</h2>
            <p>From {displayAddress}</p>
          </div>
          <button type="button" class="icon-button" title="Close" on:click={() => (composeOpen = false)}>×</button>
        </header>
        <label>
          <span>To</span>
          <input bind:value={composeTo} type="text" autocomplete="email" data-email-compose-to />
        </label>
        <label>
          <span>Subject</span>
          <input bind:value={composeSubject} type="text" data-email-compose-subject />
        </label>
        <label>
          <span>Message</span>
          <textarea bind:value={composeBody} rows="9" data-email-compose-body></textarea>
        </label>
        <div class="compose-actions">
          <button type="button" disabled={sending || !activeAddress || !composeRecipients.length || !composeBody.trim()} on:click={sendCompose}>Create draft</button>
        </div>
      </div>
    {:else if detailLoading}
      <div class="empty-state">Opening message...</div>
    {:else if !detail?.message}
      <div class="empty-state">Select a message</div>
    {:else}
      <header class="detail-header">
        <div>
          <h2>{detail.message.subject || '(no subject)'}</h2>
          <p>{detail.message.from_address} → {detailToLine}</p>
        </div>
        <span class="detail-trust">{trustLabel(detail.message.trust_status)}</span>
      </header>

      <div class="metadata">
        <span>{detail.draft ? 'Updated' : 'Received'}</span>
        <strong>{formatTime(detail.message.received_at || detail.message.created_at)}</strong>
      </div>

      <article class="body-text">{detail.text_body || 'No plain text body.'}</article>

      <details class="message-details" data-email-headers>
        <summary>Details</summary>
        <dl>
          <div>
            <dt>From</dt>
            <dd>{detail.message.from_address}</dd>
          </div>
          <div>
            <dt>To</dt>
            <dd>{detailToLine}</dd>
          </div>
          {#if detailCcRecipients.length}
            <div>
              <dt>Cc</dt>
              <dd>{addressListLabel(detailCcRecipients)}</dd>
            </div>
          {/if}
          {#if detailBccRecipients.length}
            <div>
              <dt>Bcc</dt>
              <dd>{addressListLabel(detailBccRecipients)}</dd>
            </div>
          {/if}
          <div>
            <dt>Trust</dt>
            <dd>{trustLabel(detail.message.trust_status)}</dd>
          </div>
          {#each detailHeaderEntries as [key, value]}
            <div>
              <dt>{key}</dt>
              <dd>{value}</dd>
            </div>
          {/each}
        </dl>
      </details>

      {#if detail.attachments?.length}
        <div class="attachments">
          <h3>Attachments</h3>
          {#each detail.attachments as attachment}
            <div class="attachment">
              <span>{attachment.filename}</span>
              <strong>{attachment.status}</strong>
            </div>
          {/each}
        </div>
      {/if}

      <div class="actions">
        {#if detail.draft}
          <button type="button" disabled={sending || detail.draft.status === 'sent'} on:click={emailApprovalLink}>Email approval link</button>
          <button type="button" disabled={sending || detail.draft.status === 'sent'} on:click={sendDraft}>Send approved draft</button>
        {:else}
          <button type="button" on:click={() => (replyOpen = !replyOpen)}>Reply</button>
        {/if}
      </div>

      {#if replyOpen}
        <div class="reply-box">
          <label>
            <span>From {displayAddress}</span>
            <textarea bind:value={replyBody} rows="5" placeholder="Write a reply..."></textarea>
          </label>
          <button type="button" disabled={sending || !activeAddress || !replyBody.trim()} on:click={sendReply}>Create draft</button>
        </div>
      {/if}

    {/if}
    {#if actionStatus}
      <p class="action-status">{actionStatus}</p>
    {/if}
  </section>
</section>

<style>
  .email-app {
    height: 100%;
    display: grid;
    grid-template-columns: 220px minmax(260px, 0.95fr) minmax(320px, 1.25fr);
    background: #07101d;
    color: #edf4ff;
    overflow: hidden;
  }

  .mobile-mailbar,
  .mobile-back {
    display: none;
  }

  .mail-rail,
  .message-list,
  .message-detail {
    min-width: 0;
    min-height: 0;
    border-right: 1px solid rgba(118, 151, 194, 0.18);
  }

  .mail-rail {
    padding: 18px;
    background: #081321;
  }

  .mail-title {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: flex-start;
    margin-bottom: 24px;
  }

  h1,
  h2,
  h3,
  p {
    margin: 0;
  }

  h1 {
    font-size: 20px;
  }

  .mail-title p,
  .list-header p,
  .detail-header p,
  .metadata,
  .snippet {
    color: #9ba9bf;
  }

  .address-dot {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: #20d47c;
    margin-top: 9px;
  }

  .folder-list {
    display: grid;
    gap: 8px;
  }

  button {
    font: inherit;
  }

  .folder-list button,
  .actions button,
  .reply-box button,
  .compose-actions button,
  .list-actions button,
  .icon-button {
    border: 1px solid rgba(121, 147, 194, 0.24);
    background: rgba(21, 35, 58, 0.72);
    color: #edf4ff;
    border-radius: 8px;
    cursor: pointer;
  }

  .folder-list button {
    text-align: left;
    padding: 12px 14px;
  }

  .folder-list button.active,
  .message-row.selected {
    border-color: #3c78ff;
    background: rgba(40, 76, 150, 0.42);
  }

  .message-list {
    display: flex;
    flex-direction: column;
    background: #07101d;
  }

  .list-header,
  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 14px;
    padding: 18px 20px;
    border-bottom: 1px solid rgba(118, 151, 194, 0.18);
  }

  .list-actions {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .list-header h2,
  .detail-header h2 {
    font-size: 18px;
  }

  .detail-header > div {
    min-width: 0;
  }

  .detail-header h2,
  .detail-header p {
    overflow-wrap: anywhere;
  }

  .icon-button {
    width: 36px;
    height: 36px;
  }

  .rows {
    overflow: auto;
    padding: 10px;
  }

  .message-row {
    width: 100%;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto auto;
    grid-template-areas:
      "sender sender time"
      "subject subject subject"
      "snippet snippet snippet"
      "trust trust attachment";
    gap: 4px 10px;
    text-align: left;
    padding: 13px 12px;
    color: inherit;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
  }

  .sender {
    grid-area: sender;
    font-weight: 700;
  }

  .time {
    grid-area: time;
    color: #9ba9bf;
    font-size: 12px;
  }

  .subject {
    grid-area: subject;
  }

  .snippet {
    grid-area: snippet;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .trust {
    grid-area: trust;
    color: #86b4ff;
    font-size: 12px;
  }

  .attachment-indicator {
    grid-area: attachment;
    color: #c7d4e8;
    font-size: 14px;
    line-height: 1;
    justify-self: end;
  }

  .message-detail {
    display: flex;
    flex-direction: column;
    background: #08111d;
    overflow: auto;
  }

  .detail-trust {
    flex: none;
    border: 1px solid rgba(76, 124, 255, 0.5);
    color: #bdd0ff;
    border-radius: 999px;
    padding: 5px 10px;
    font-size: 12px;
  }

  .metadata,
  .body-text,
  .message-details,
  .attachments,
  .actions,
  .reply-box,
  .compose-box,
  .action-status,
  .mail-error,
  .empty-state {
    margin: 18px 20px 0;
  }

  .metadata {
    display: flex;
    gap: 12px;
  }

  .body-text {
    white-space: pre-wrap;
    line-height: 1.55;
  }

  .message-details {
    border: 1px solid rgba(118, 151, 194, 0.18);
    border-radius: 8px;
    color: #cbd8ea;
  }

  .message-details summary {
    cursor: pointer;
    padding: 10px 12px;
    color: #edf4ff;
  }

  .message-details dl {
    display: grid;
    gap: 8px;
    margin: 0;
    padding: 0 12px 12px;
  }

  .message-details dl div {
    display: grid;
    grid-template-columns: minmax(88px, 0.32fr) minmax(0, 1fr);
    gap: 12px;
  }

  .message-details dt {
    color: #91a4bf;
    overflow-wrap: anywhere;
  }

  .message-details dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .attachments {
    display: grid;
    gap: 8px;
  }

  .attachments h3 {
    font-size: 14px;
    color: #c7d4e8;
  }

  .attachment {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    padding: 10px 12px;
    border: 1px solid rgba(118, 151, 194, 0.18);
    border-radius: 8px;
  }

  .attachment strong {
    color: #ffb15c;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  .actions button,
  .reply-box button,
  .compose-actions button {
    padding: 9px 13px;
  }

  .reply-box {
    display: grid;
    gap: 10px;
  }

  .compose-box {
    display: grid;
    gap: 12px;
  }

  .compose-header {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: flex-start;
  }

  .compose-header h2 {
    font-size: 18px;
  }

  .compose-header p {
    color: #9ba9bf;
  }

  .reply-box label,
  .compose-box label {
    display: grid;
    gap: 8px;
    color: #9ba9bf;
  }

  input,
  select,
  textarea {
    border: 1px solid rgba(121, 147, 194, 0.3);
    background: rgba(5, 11, 20, 0.8);
    color: #edf4ff;
    border-radius: 8px;
    padding: 10px;
    font: inherit;
  }

  textarea {
    resize: vertical;
    min-height: 110px;
  }

  .compose-actions {
    display: flex;
    justify-content: flex-end;
  }

  button:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .mail-error {
    color: #ff8c8c;
  }

  .empty-state {
    color: #9ba9bf;
  }

  @media (max-width: 760px) {
    .email-app {
      grid-template-columns: minmax(0, 1fr);
      grid-template-rows: auto minmax(0, 1fr);
      height: 100%;
      min-height: 0;
    }

    .mail-rail {
      display: none;
    }

    .mobile-mailbar {
      display: flex;
      justify-content: space-between;
      align-items: flex-end;
      gap: 12px;
      padding: 14px 16px 12px;
      border-bottom: 1px solid rgba(118, 151, 194, 0.18);
      background: #081321;
      min-width: 0;
    }

    .mobile-mailbar h1 {
      font-size: 20px;
      line-height: 1.15;
    }

    .mobile-mailbar p,
    .mobile-mailbar span {
      color: #9ba9bf;
      font-size: 12px;
    }

    .mobile-mailbar label {
      display: grid;
      gap: 5px;
      min-width: 132px;
    }

    .mobile-mailbar select {
      min-height: 36px;
      padding: 7px 32px 7px 10px;
    }

    .message-list {
      border-right: 0;
      min-height: 0;
      overflow: hidden;
    }

    .message-list.mobile-hidden {
      display: none;
    }

    .list-header,
    .detail-header {
      padding: 14px 16px;
    }

    .list-header h2,
    .detail-header h2 {
      font-size: 17px;
    }

    .list-header {
      align-items: center;
    }

    .list-actions button:not(.icon-button) {
      min-height: 36px;
      padding: 7px 11px;
    }

    .rows {
      padding: 8px;
    }

    .message-row {
      grid-template-columns: minmax(0, 1fr) auto;
      grid-template-areas:
        "sender time"
        "subject subject"
        "snippet snippet"
        "trust attachment";
      padding: 12px 10px;
    }

    .sender,
    .subject,
    .snippet {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .message-detail {
      display: none;
      border-top: 1px solid rgba(118, 151, 194, 0.18);
      border-right: 0;
      min-height: 0;
    }

    .message-detail.mobile-open {
      display: flex;
    }

    .mobile-back {
      display: inline-flex;
      align-items: center;
      align-self: flex-start;
      margin: 12px 16px 0;
      min-height: 36px;
      padding: 7px 11px;
      border: 1px solid rgba(121, 147, 194, 0.24);
      background: rgba(21, 35, 58, 0.72);
      color: #edf4ff;
      border-radius: 8px;
      cursor: pointer;
    }

    .detail-header {
      flex-direction: column;
      align-items: stretch;
      gap: 10px;
    }

    .detail-header p,
    .body-text,
    .message-details dd,
    .attachment span {
      overflow-wrap: anywhere;
    }

    .detail-trust {
      align-self: flex-start;
      border-radius: 8px;
    }

    .metadata,
    .body-text,
    .message-details,
    .attachments,
    .actions,
    .reply-box,
    .compose-box,
    .action-status,
    .mail-error,
    .empty-state {
      margin-left: 16px;
      margin-right: 16px;
    }

    .message-details dl div {
      grid-template-columns: 1fr;
      gap: 3px;
    }

    .attachment {
      align-items: flex-start;
      flex-direction: column;
    }

    .actions button,
    .reply-box button,
    .compose-actions button {
      min-height: 40px;
      padding: 9px 12px;
    }

    .reply-box textarea,
    .compose-box textarea {
      min-height: 150px;
    }
  }
</style>
