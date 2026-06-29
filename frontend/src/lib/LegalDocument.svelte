<script lang="ts">
  import { onMount } from 'svelte';

  export let kind: 'privacy' | 'terms' = 'privacy';

  type MarkdownBlock = {
    type: 'h1' | 'h2' | 'h3' | 'p' | 'li';
    text: string;
  };

  let status: 'loading' | 'ready' | 'error' = 'loading';
  let blocks: MarkdownBlock[] = [];

  $: documentTitle = kind === 'privacy' ? 'Privacy Policy' : 'Terms of Service';
  $: assetPath = kind === 'privacy' ? '/legal/privacy-policy.md' : '/legal/terms-of-service.md';

  function stripMarkdown(value: string) {
    return value.replace(/\*\*/g, '').replace(/\[(.*?)\]\((.*?)\)/g, '$1');
  }

  function parseMarkdown(markdown: string): MarkdownBlock[] {
    const parsed: MarkdownBlock[] = [];
    let paragraph: string[] = [];

    function flushParagraph() {
      if (paragraph.length === 0) return;
      parsed.push({ type: 'p', text: stripMarkdown(paragraph.join(' ')) });
      paragraph = [];
    }

    for (const rawLine of markdown.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line) {
        flushParagraph();
        continue;
      }
      if (line.startsWith('**')) {
        flushParagraph();
        parsed.push({ type: 'p', text: stripMarkdown(line) });
        continue;
      }
      if (line.startsWith('# ')) {
        flushParagraph();
        parsed.push({ type: 'h1', text: stripMarkdown(line.slice(2).trim()) });
        continue;
      }
      if (line.startsWith('## ')) {
        flushParagraph();
        parsed.push({ type: 'h2', text: stripMarkdown(line.slice(3).trim()) });
        continue;
      }
      if (line.startsWith('### ')) {
        flushParagraph();
        parsed.push({ type: 'h3', text: stripMarkdown(line.slice(4).trim()) });
        continue;
      }
      if (line.startsWith('- ')) {
        flushParagraph();
        parsed.push({ type: 'li', text: stripMarkdown(line.slice(2).trim()) });
        continue;
      }
      const lastBlock = parsed[parsed.length - 1];
      if (lastBlock?.type === 'li' && paragraph.length === 0) {
        lastBlock.text = `${lastBlock.text} ${stripMarkdown(line)}`;
        continue;
      }
      paragraph.push(line);
    }
    flushParagraph();
    return parsed;
  }

  onMount(async () => {
    status = 'loading';
    try {
      const res = await fetch(assetPath, { cache: 'no-store' });
      if (!res.ok) throw new Error(`legal document returned ${res.status}`);
      blocks = parseMarkdown(await res.text());
      status = 'ready';
    } catch (_err) {
      status = 'error';
    }
  });
</script>

<main class="legal-reader" data-legal-reader data-legal-kind={kind}>
  <header>
    <a class="reader-brand" href="/">Choir</a>
    <nav aria-label="Legal documents">
      <a href="/privacy" aria-current={kind === 'privacy' ? 'page' : undefined}>Privacy</a>
      <a href="/terms" aria-current={kind === 'terms' ? 'page' : undefined}>Terms</a>
    </nav>
  </header>

  <article class="legal-panel">
    {#if status === 'loading'}
      <p data-legal-status>Loading {documentTitle}...</p>
    {:else if status === 'error'}
      <h1>{documentTitle}</h1>
      <p data-legal-error>Could not load this document. Contact privacy@choir.news or legal@choir.news.</p>
    {:else}
      {#each blocks as block}
        {#if block.type === 'h1'}
          <h1>{block.text}</h1>
        {:else if block.type === 'h2'}
          <h2>{block.text}</h2>
        {:else if block.type === 'h3'}
          <h3>{block.text}</h3>
        {:else if block.type === 'li'}
          <p class="legal-list-item">{block.text}</p>
        {:else}
          <p>{block.text}</p>
        {/if}
      {/each}
    {/if}
  </article>
</main>

<style>
  .legal-reader {
    width: 100%;
    min-height: 100%;
    overflow: auto;
    background: var(--choir-bg);
    color: var(--choir-fg);
  }

  .legal-reader header {
    position: sticky;
    top: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem clamp(1rem, 4vw, 2.5rem);
    border-bottom: 1px solid var(--choir-border);
    background: color-mix(in srgb, var(--choir-bg) 92%, transparent);
    backdrop-filter: blur(12px);
  }

  .reader-brand,
  .legal-reader nav a {
    color: var(--choir-fg);
    font-size: 0.85rem;
    font-weight: 780;
    text-decoration: none;
  }

  .reader-brand {
    text-transform: uppercase;
    overflow-wrap: anywhere;
  }

  .legal-reader nav {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }

  .legal-reader nav a {
    min-height: 2rem;
    display: inline-flex;
    align-items: center;
    padding: 0.35rem 0.65rem;
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-control);
    color: var(--choir-text-primary);
    font-size: 0.78rem;
  }

  .legal-reader nav a:hover,
  .legal-reader nav a:focus-visible,
  .legal-reader nav a[aria-current='page'] {
    border-color: var(--choir-border-strong);
    background: var(--choir-state-hover);
    outline: none;
  }

  .legal-panel {
    width: min(920px, calc(100% - 2rem));
    margin: 1rem auto 2rem;
    padding: clamp(1rem, 3vw, 2rem);
    border: 1px solid var(--choir-border);
    border-radius: 8px;
    background: var(--choir-surface-pane);
    box-shadow: var(--choir-window-shadow);
  }

  .legal-panel h1,
  .legal-panel h2,
  .legal-panel h3 {
    color: var(--choir-text-primary);
    overflow-wrap: anywhere;
  }

  .legal-panel h1 {
    margin: 0 0 0.8rem;
    font-size: clamp(1.65rem, 3vw, 2.6rem);
    line-height: 1.08;
  }

  .legal-panel h2 {
    margin: 1.45rem 0 0.55rem;
    font-size: clamp(1.2rem, 2vw, 1.55rem);
    line-height: 1.2;
  }

  .legal-panel h3 {
    margin: 1rem 0 0.45rem;
    font-size: 1rem;
    line-height: 1.25;
  }

  .legal-panel p {
    margin: 0.58rem 0;
    color: var(--choir-text-primary);
    font-size: 1rem;
    line-height: 1.58;
    overflow-wrap: anywhere;
  }

  .legal-list-item {
    padding-left: 1rem;
    text-indent: -0.8rem;
  }

  .legal-list-item::before {
    content: "- ";
    color: var(--choir-text-muted);
  }

  @media (max-width: 720px) {
    .legal-reader header {
      align-items: flex-start;
      flex-direction: column;
    }
  }
</style>
