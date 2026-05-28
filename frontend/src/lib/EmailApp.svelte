<script>
  import { createEventDispatcher, onMount } from 'svelte';
  import { fetchWithRenewal, AuthRequiredError } from './auth.js';

  export let authenticated = false;

  const dispatch = createEventDispatcher();

  const folders = [
    { id: 'inbox', label: 'Inbox' },
    { id: 'sent', label: 'Sent' },
    { id: 'quarantine', label: 'Quarantine' },
  ];

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

  $: selectedMessage = messages.find((message) => message.id === selectedId) || null;
  $: activeAddress = '000@choir.news';
  $: detailHeaderEntries = headerEntries(detail?.raw_headers);
  $: detailToRecipients = detail?.recipients?.to || [];
  $: detailCcRecipients = detail?.recipients?.cc || [];
  $: detailBccRecipients = detail?.recipients?.bcc || [];
  $: detailToLine = addressListLabel(detailToRecipients) || activeAddress;
  $: composeRecipients = parseAddressList(composeTo);

  onMount(() => {
    if (authenticated) {
      void loadMessages(activeFolder);
    }
  });

  $: if (authenticated && !loadedOnce && !loading) {
    void loadMessages(activeFolder);
  }

  async function loadMessages(folder) {
    loading = true;
    error = '';
    activeFolder = folder;
    try {
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
        await loadDetail(selectedId);
      }
    } catch (err) {
      handleError(err);
    } finally {
      loading = false;
    }
  }

  async function loadDetail(id) {
    selectedId = id;
    detailLoading = true;
    actionStatus = '';
    replyOpen = false;
    composeOpen = false;
    try {
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

  async function markRead() {
    if (!selectedId) return;
    try {
      const res = await fetchWithRenewal(`/api/email/messages/${encodeURIComponent(selectedId)}/read`, {
        method: 'POST',
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not mark read');
      }
      await loadMessages(activeFolder);
    } catch (err) {
      handleError(err);
    }
  }

  async function respondWithChoir() {
    if (!selectedId) return;
    actionStatus = 'Creating Choir response handoff...';
    try {
      const res = await fetchWithRenewal(`/api/email/messages/${encodeURIComponent(selectedId)}/send-to-choir`, {
        method: 'POST',
      });
      if (!res.ok) {
        if (res.status === 401) throw new AuthRequiredError();
        throw new Error('Could not send to Choir');
      }
      const data = await res.json();
      if (data.ingress_event_recorded === false) {
        actionStatus = data.submission_id
          ? `Choir response handoff: ${data.submission_id} (receipt pending)`
          : 'Choir response handoff created (receipt pending)';
      } else {
        actionStatus = data.submission_id ? `Choir response handoff: ${data.submission_id}` : 'Choir response handoff created';
      }
    } catch (err) {
      handleError(err);
    }
  }

  async function sendReply() {
    if (!selectedMessage || !replyBody.trim()) return;
    sending = true;
    actionStatus = '';
    try {
      const res = await fetchWithRenewal('/api/email/send', {
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
        throw new Error('Could not send reply');
      }
      replyBody = '';
      replyOpen = false;
      actionStatus = 'Reply sent';
      await loadMessages(activeFolder);
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  async function sendCompose() {
    if (!composeRecipients.length || !composeBody.trim()) return;
    sending = true;
    actionStatus = '';
    try {
      const res = await fetchWithRenewal('/api/email/send', {
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
        throw new Error('Could not send message');
      }
      composeTo = '';
      composeSubject = '';
      composeBody = '';
      composeOpen = false;
      actionStatus = 'Message sent';
      await loadMessages('sent');
    } catch (err) {
      handleError(err);
    } finally {
      sending = false;
    }
  }

  function openCompose() {
    composeOpen = true;
    replyOpen = false;
    actionStatus = '';
    error = '';
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
        <p>{activeAddress}</p>
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

  <main class="message-list" data-email-message-list>
    <header class="list-header">
      <div>
        <h2>{folders.find((folder) => folder.id === activeFolder)?.label || 'Inbox'}</h2>
        <p>{messages.length} messages</p>
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
            on:click={() => loadDetail(message.id)}
            data-email-message-id={message.id}
          >
            <span class="unread-dot" class:read={message.read_at}></span>
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

  <section class="message-detail" data-email-message-detail>
    {#if composeOpen}
      <div class="compose-box" data-email-compose-panel>
        <header class="compose-header">
          <div>
            <h2>New message</h2>
            <p>From {activeAddress}</p>
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
          <button type="button" disabled={sending || !composeRecipients.length || !composeBody.trim()} on:click={sendCompose}>Send</button>
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
        <span>Received</span>
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
        <button type="button" on:click={() => (replyOpen = !replyOpen)}>Reply</button>
        <button type="button" on:click={respondWithChoir}>Respond with Choir</button>
        <button type="button" on:click={markRead}>Mark read</button>
      </div>

      {#if replyOpen}
        <div class="reply-box">
          <label>
            <span>From {activeAddress}</span>
            <textarea bind:value={replyBody} rows="5" placeholder="Write a reply..."></textarea>
          </label>
          <button type="button" disabled={sending || !replyBody.trim()} on:click={sendReply}>Send</button>
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
    grid-template-columns: 12px minmax(0, 1fr) auto auto;
    grid-template-areas:
      "dot sender sender time"
      "dot subject subject subject"
      "dot snippet snippet snippet"
      "dot trust trust attachment";
    gap: 4px 10px;
    text-align: left;
    padding: 13px 12px;
    color: inherit;
    border: 1px solid transparent;
    border-radius: 8px;
    background: transparent;
    cursor: pointer;
  }

  .unread-dot {
    grid-area: dot;
    width: 8px;
    height: 8px;
    align-self: center;
    border-radius: 50%;
    background: #4088ff;
  }

  .unread-dot.read {
    background: transparent;
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
      grid-template-columns: 1fr;
    }

    .mail-rail {
      display: none;
    }

    .message-list {
      min-height: 48%;
    }

    .message-detail {
      border-top: 1px solid rgba(118, 151, 194, 0.18);
    }
  }
</style>
