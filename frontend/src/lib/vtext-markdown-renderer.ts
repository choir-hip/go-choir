import { escapeHTML, renderInlineMarkdown } from './vtext-source-renderer';

type RenderMarkdownOptions = {
  emptyHTML?: string;
  headingLevelOffset?: number;
  wrapTables?: boolean;
  relatedVTexts?: any[];
};

function splitTableCells(value: string): string[] {
  const cells: string[] = [];
  let cell = '';
  let escaped = false;
  for (const char of value) {
    if (escaped) {
      cell += char === '|' ? '|' : `\\${char}`;
      escaped = false;
      continue;
    }
    if (char === '\\') {
      escaped = true;
      continue;
    }
    if (char === '|') {
      cells.push(cell);
      cell = '';
      continue;
    }
    cell += char;
  }
  cells.push(cell);
  return cells;
}

function parseTableRow(line: string): string[] | null {
  const trimmed = line.trim();
  if (!trimmed.startsWith('|')) return null;
  const body = trimmed.endsWith('|') ? trimmed.slice(1, -1) : trimmed.slice(1);
  const cells = splitTableCells(body).map((cell) => cell.trim());
  return cells.length >= 2 ? cells : null;
}

function isTableSeparator(cells: string[] | null): boolean {
  return Array.isArray(cells) && cells.every((cell) => /^:?-{3,}:?$/.test(cell));
}

function headingLevel(markers: string, offset: number): number {
  return Math.min(6, Math.max(1, markers.length + offset));
}

export function renderMarkdownBlocks(value: unknown, sourceEntities: any[] = [], options: RenderMarkdownOptions = {}): string {
  const normalized = String(value || '').replace(/\|\s+\|/g, '|\n|');
  const lines = normalized.split(/\r?\n/);
  const blocks: string[] = [];
  let paragraph: string[] = [];
  let unordered: string[] = [];
  let ordered: string[] = [];
  let quote: string[] = [];
  let table: string[] = [];
  let code: string[] = [];
  let inCode = false;
  const headingOffset = Number.isFinite(options.headingLevelOffset) ? Number(options.headingLevelOffset) : 0;
  const wrapTables = options.wrapTables !== false;

  const inline = (text: string) => renderInlineMarkdown(text, sourceEntities, options.relatedVTexts || []);

  function flushParagraph() {
    if (paragraph.length === 0) return;
    blocks.push(`<p>${inline(paragraph.join(' '))}</p>`);
    paragraph = [];
  }

  function flushLists() {
    if (unordered.length > 0) {
      blocks.push(`<ul>${unordered.map((item) => `<li>${inline(item)}</li>`).join('')}</ul>`);
      unordered = [];
    }
    if (ordered.length > 0) {
      blocks.push(`<ol>${ordered.map((item) => `<li>${inline(item)}</li>`).join('')}</ol>`);
      ordered = [];
    }
  }

  function flushQuote() {
    if (quote.length === 0) return;
    blocks.push(`<blockquote>${quote.map((item) => `<p>${inline(item)}</p>`).join('')}</blockquote>`);
    quote = [];
  }

  function flushTable() {
    if (table.length === 0) return;
    const parsed = table.map(parseTableRow).filter(Boolean) as string[][];
    if (parsed.length >= 2 && isTableSeparator(parsed[1])) {
      const headers = parsed[0];
      const rows = parsed.slice(2);
      const html = `<table><thead><tr>${headers.map((cell) => `<th>${inline(cell)}</th>`).join('')}</tr></thead><tbody>${rows.map((row) => `<tr>${row.map((cell) => `<td>${inline(cell)}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
      blocks.push(wrapTables ? `<div class="table-scroll">${html}</div>` : html);
    } else {
      blocks.push(`<p>${inline(table.join(' '))}</p>`);
    }
    table = [];
  }

  function flushCode() {
    if (code.length === 0) return;
    blocks.push(`<pre><code>${escapeHTML(code.join('\n'))}</code></pre>`);
    code = [];
  }

  function flushAll() {
    flushCode();
    flushParagraph();
    flushLists();
    flushQuote();
    flushTable();
  }

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();
    if (trimmed.startsWith('```')) {
      if (inCode) {
        inCode = false;
        flushCode();
      } else {
        flushAll();
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      code.push(rawLine);
      continue;
    }
    if (!trimmed) {
      flushAll();
      continue;
    }

    const heading = trimmed.match(/^(#{1,4})\s+(.+)$/);
    if (heading) {
      flushAll();
      const level = headingLevel(heading[1], headingOffset);
      blocks.push(`<h${level}>${inline(heading[2])}</h${level}>`);
      continue;
    }

    if (parseTableRow(trimmed)) {
      flushParagraph();
      flushLists();
      flushQuote();
      table.push(trimmed);
      continue;
    }

    const bullet = trimmed.match(/^[-*]\s+(.+)$/);
    if (bullet) {
      flushParagraph();
      flushQuote();
      flushTable();
      ordered = [];
      unordered.push(bullet[1]);
      continue;
    }

    const numbered = trimmed.match(/^\d+\.\s+(.+)$/);
    if (numbered) {
      flushParagraph();
      flushQuote();
      flushTable();
      unordered = [];
      ordered.push(numbered[1]);
      continue;
    }

    const quoteLine = trimmed.match(/^>\s?(.*)$/);
    if (quoteLine) {
      flushParagraph();
      flushLists();
      flushTable();
      quote.push(quoteLine[1]);
      continue;
    }

    flushLists();
    flushQuote();
    flushTable();
    paragraph.push(trimmed);
  }

  flushAll();
  return blocks.join('\n') || options.emptyHTML || '';
}
