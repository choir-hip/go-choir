<!--
  BrowserApp — Web Lens, the launcher.

  Web Lens does not browse. It takes an address and offers two ways to read it:

    - "Open original" opens the page in the device's own browser, as a
      top-level page that cannot reach back into the desktop (noopener) or
      leak its address (noreferrer);
    - "Source Viewer snapshot" opens the Source Viewer on the address, which
      reads it through the guarded import path (/api/content/import-url, the
      same path research uses). It needs a signed-in desktop.

  A source that already carries a reader snapshot (a stored source entity
  whose open surface is web_lens) still shows that snapshot here, and can be
  opened in Texture.

  Nothing here runs a second browser: the backend Web Lens (Obscura, started
  by the guest daemon as root) and the unsandboxed iframe preview are gone
  (docs/problems/obscura-untrusted-javascript-as-guest-root-2026-10-21.md).

  Data attributes for test targeting:
    data-browser-app             — root container (also data-browser-launcher)
    data-browser-url-bar         — address bar area
    data-browser-url-input       — address text input
    data-browser-go-btn          — submit button
    data-browser-error           — message when an address is refused
    data-browser-target          — the checked address, once there is one
    data-browser-open-original   — link to the original page
    data-browser-open-snapshot   — button for the Source Viewer snapshot
    data-browser-reader-markdown — stored reader snapshot, if the source has one
    data-browser-import-texture  — open that snapshot in Texture
-->
<script>
  import { createEventDispatcher } from 'svelte';
  import { AuthRequiredError } from './auth.js';
  import {
    renderInlineMarkdown,
    sourceEntitySnapshotWarnings,
    sourceEntitySnapshotText,
    sourceEntityTitle,
  } from './texture-source-renderer.ts';
  import { webLensAddress } from './source-url';
  import { createContentItem } from './texture.js';

  export let appContext = {};
  export let authenticated = false;
  const dispatch = createEventDispatcher();

  // ---- State ----
  const sourceEntity = appContext?.sourceEntity || null;
  const sourceSnapshot = sourceEntitySnapshotText(sourceEntity);
  const sourceSnapshotWarnings = sourceEntitySnapshotWarnings(sourceEntity);
  const initialTarget = webLensAddress(appContext?.initialUrl || appContext?.sourceUrl || '');
  let urlInput = initialTarget;
  let target = initialTarget;
  let busy = false;
  let error = '';

  $: readableSnapshotHTML = renderSnapshotMarkdown(sourceSnapshot);

  // ---- Actions ----

  function handleGo() {
    const address = webLensAddress(urlInput);
    if (!address) {
      target = '';
      error = 'Enter a web address that starts with http:// or https://, such as example.com.';
      return;
    }
    error = '';
    urlInput = address;
    target = address;
  }

  function handleUrlKeydown(event) {
    if (event.key === 'Enter') {
      handleGo();
    }
  }

  // The Source Viewer reads the address through the guarded import path
  // (ContentViewer posts /api/content/import-url); Web Lens makes no request.
  function openSnapshot() {
    if (!target || !authenticated) return;
    dispatch('launchapp', {
      appId: 'content',
      appName: 'Source',
      appContext: {
        sourceUrl: target,
        windowTitle: target,
        title: target,
        appHint: 'content',
      },
    });
  }

  async function importSnapshotToTexture() {
    if (!sourceSnapshot) return;
    const sourceURL = target || webLensAddress(appContext?.sourceUrl || '') || '';
    const title = sourceEntityTitle(sourceEntity) || 'Web Lens Import';
    busy = true;
    error = '';
    let contentItem = null;
    try {
      contentItem = await createContentItem({
        source_type: 'text',
        media_type: 'text/markdown',
        app_hint: 'content',
        title,
        source_url: sourceURL,
        canonical_url: sourceURL,
        text_content: sourceSnapshot,
        metadata: {
          reader_artifact_kind: 'web_lens_reader_markdown',
          snapshot_mode: 'source_entity',
          warning_count: sourceSnapshotWarnings.length,
          warnings: sourceSnapshotWarnings,
          source_entity_id: sourceEntity?.entity_id || sourceEntity?.source_entity_id || '',
        },
        provenance: {
          created_by: 'web_lens',
          source_url: sourceURL,
          rights_scope: 'public_source',
          publish_source_snapshot: true,
          untrusted_source_text: true,
        },
      });
    } catch (err) {
      if (err instanceof AuthRequiredError) {
        dispatch('authexpired');
        return;
      }
      error = err.message || 'Web Lens snapshot import failed';
      busy = false;
      return;
    }
    busy = false;
    dispatch('opentexture', {
      title,
      initialContent: [
        '# Source reader import',
        '',
        `Source: ${sourceURL || 'unknown'}`,
        `Content item: ${contentItem?.content_id || 'unknown'}`,
        '',
        '## Snapshot',
        '',
        sourceSnapshot,
      ].join('\n'),
      seedPrompt: `Import Web Lens snapshot for ${sourceURL || 'web page'}`,
      sourceUrl: sourceURL,
      sourceContentId: contentItem?.content_id || '',
      appHint: 'content',
      createdFrom: 'web_lens',
      toastMessage: 'Opened Web Lens snapshot in Texture',
    });
  }

  function renderSnapshotMarkdown(value) {
    const raw = String(value || '').trim();
    if (!raw) return '';
    const lines = raw.split(/\r?\n/);
    const blocks = [];
    let paragraph = [];
    const flushParagraph = () => {
      const text = paragraph.join(' ').trim();
      if (text) blocks.push(`<p>${renderInlineMarkdown(text, [], [], { linkMode: 'anchor' })}</p>`);
      paragraph = [];
    };
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed) {
        flushParagraph();
        continue;
      }
      const heading = /^(#{1,4})\s+(.+)$/.exec(trimmed);
      if (heading) {
        flushParagraph();
        const level = Math.min(4, heading[1].length + 1);
        blocks.push(`<h${level}>${renderInlineMarkdown(heading[2], [], [], { linkMode: 'anchor' })}</h${level}>`);
        continue;
      }
      paragraph.push(trimmed);
    }
    flushParagraph();
    return blocks.join('\n');
  }
</script>

<div class="browser-app" data-browser-app data-browser-launcher>
  <!-- Address bar -->
  <div class="url-bar" data-browser-url-bar>
    <input
      type="text"
      class="url-input"
      data-browser-url-input
      bind:value={urlInput}
      on:keydown={handleUrlKeydown}
      placeholder="Enter a web address..."
      aria-label="Web address"
      autocapitalize="off"
      autocomplete="off"
      spellcheck="false"
    />
    <button
      class="go-btn"
      data-browser-go-btn
      on:click={handleGo}
      title="Check the address"
      aria-label="Check the web address"
    >
      Go
    </button>
  </div>

  {#if error}
    <div class="error-message" data-browser-error role="alert">
      <span class="error-icon">⚠️</span>
      <span class="error-text">{error}</span>
      <button
        class="error-dismiss"
        on:click={() => { error = ''; }}
        title="Dismiss"
        aria-label="Dismiss error"
      >
        ✕
      </button>
    </div>
  {/if}

  {#if target}
    <div class="launch-panel" data-browser-target={target}>
      <p class="launch-address">{target}</p>
      <div class="launch-actions">
        <a
          class="launch-link"
          data-browser-open-original
          href={target}
          target="_blank"
          rel="noopener noreferrer"
        >
          Open original
        </a>
        {#if authenticated}
          <button
            class="launch-btn"
            data-browser-open-snapshot
            on:click={openSnapshot}
          >
            Source Viewer snapshot
          </button>
        {/if}
      </div>
      <p class="launch-note">
        The original opens in your own browser. A snapshot is read by your
        computer through Choir's guarded import and opens in the Source Viewer.
      </p>
    </div>
  {/if}

  {#if sourceSnapshot}
    <div class="backend-snapshot" data-browser-source-snapshot>
      <div class="backend-main">
        <div class="snapshot-actions">
          <span>Source reader snapshot</span>
          {#if authenticated}
            <button
              class="import-btn"
              data-browser-import-texture
              on:click={importSnapshotToTexture}
              disabled={busy}
            >
              Open in Texture
            </button>
          {/if}
        </div>
        {#if sourceSnapshotWarnings.length}
          <div class="snapshot-warnings" data-browser-snapshot-warnings>
            {#each sourceSnapshotWarnings as warning}
              <span>{warning}</span>
            {/each}
          </div>
        {/if}
        <article class="snapshot-reader" data-browser-reader-markdown>
          {@html readableSnapshotHTML}
        </article>
      </div>
    </div>
  {:else if !target}
    <div class="empty-state">
      <span class="empty-icon">🌐</span>
      <span>Enter a web address to open it or read a snapshot</span>
    </div>
  {/if}
</div>

<style>
  .browser-app {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
  }

  /* ---- Address bar ---- */
  .url-bar {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 8px;
    background: var(--choir-state-selected);
    border-bottom: 1px solid var(--choir-border-strong);
    flex-shrink: 0;
  }

  .url-input {
    flex: 1;
    padding: 6px 10px;
    background: var(--choir-state-selected);
    border: 1px solid var(--choir-border);
    border-radius: 4px;
    color: var(--choir-text-primary);
    font-size: 0.85rem;
    min-width: 0;
  }

  .url-input:focus {
    outline: none;
    border-color: var(--choir-border-strong);
  }

  .go-btn {
    padding: 6px 14px;
    background: var(--choir-state-hover);
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.8rem;
    white-space: nowrap;
    transition: background 0.15s;
    flex-shrink: 0;
  }

  .go-btn:hover {
    background: var(--choir-state-selected);
  }

  /* ---- Error message ---- */
  .error-message {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: var(--choir-status-danger-soft);
    border-bottom: 1px solid var(--choir-status-danger);
    color: var(--choir-status-danger);
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .error-icon {
    font-size: 1rem;
    flex-shrink: 0;
  }

  .error-text {
    flex: 1;
    min-width: 0;
  }

  .error-dismiss {
    width: 24px;
    height: 24px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--choir-status-danger);
    cursor: pointer;
    font-size: 0.8rem;
    flex-shrink: 0;
  }

  .error-dismiss:hover {
    background: var(--choir-status-danger-soft);
  }

  /* ---- Launch panel ---- */
  .launch-panel {
    display: grid;
    gap: 10px;
    flex-shrink: 0;
    padding: 14px 12px;
    border-bottom: 1px solid var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .launch-address {
    margin: 0;
    overflow-wrap: anywhere;
    color: var(--choir-text-primary);
    font-size: 0.9rem;
    font-weight: 600;
  }

  .launch-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .launch-link,
  .launch-btn {
    display: inline-flex;
    align-items: center;
    min-height: 32px;
    padding: 6px 14px;
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-state-hover);
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.8rem;
    text-decoration: none;
  }

  .launch-link:hover,
  .launch-btn:hover {
    background: var(--choir-state-selected);
  }

  .launch-note {
    margin: 0;
    color: var(--choir-text-subtle);
    font-size: 0.74rem;
    line-height: 1.4;
  }

  /* ---- Stored reader snapshot ---- */
  .backend-snapshot {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 18px;
    background: var(--choir-state-selected);
    color: var(--choir-text-accent);
  }

  .backend-main {
    min-width: 0;
  }

  .snapshot-actions {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: 10px;
    color: var(--choir-text-accent);
    font-size: 0.78rem;
    font-weight: 700;
  }

  .snapshot-warnings {
    display: grid;
    gap: 4px;
    margin: 0 0 10px;
    border: 1px solid var(--choir-status-warning);
    border-radius: 4px;
    padding: 8px;
    background: color-mix(in srgb, var(--choir-status-warning) 12%, transparent);
    color: var(--choir-status-warning);
    font-size: 0.74rem;
    line-height: 1.35;
  }

  .import-btn {
    border: 1px solid var(--choir-border-strong);
    border-radius: 4px;
    background: var(--choir-surface-document);
    color: var(--choir-text-accent);
    cursor: pointer;
    font-size: 0.74rem;
    padding: 5px 9px;
  }

  .import-btn:hover:not(:disabled) {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-selected);
  }

  .import-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .snapshot-reader {
    max-width: 70ch;
    color: var(--choir-text-primary);
    font: 0.95rem/1.62 ui-serif, Georgia, Cambria, "Times New Roman", Times, serif;
  }

  .snapshot-reader :global(p) {
    margin: 0 0 0.95rem;
  }

  .snapshot-reader :global(h2),
  .snapshot-reader :global(h3),
  .snapshot-reader :global(h4),
  .snapshot-reader :global(h5) {
    margin: 1.15rem 0 0.55rem;
    color: var(--choir-text-accent);
    font-family: inherit;
    line-height: 1.2;
  }

  .snapshot-reader :global(a) {
    color: var(--choir-text-accent);
    text-decoration-thickness: 1px;
    text-underline-offset: 0.16em;
  }

  /* ---- Empty state ---- */
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    flex: 1;
    color: var(--choir-text-subtle);
    font-size: 0.9rem;
    text-align: center;
    padding: 0 16px;
  }

  .empty-icon {
    font-size: 3rem;
    opacity: 0.4;
  }

  /* ---- Mobile responsive ---- */
  @media (max-width: 768px) {
    .url-bar {
      padding: 6px 6px;
      gap: 3px;
    }

    .url-input {
      font-size: 16px; /* Prevent iOS zoom */
      padding: 8px 8px;
    }

    .go-btn {
      padding: 8px 10px;
      min-height: 36px;
    }

    .launch-link,
    .launch-btn {
      min-height: 40px;
    }
  }
</style>
