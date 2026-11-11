import { escapeHTML, renderInlineMarkdown } from './texture-source-renderer';

type RenderMarkdownOptions = {
  emptyHTML?: string;
  headingLevelOffset?: number;
  wrapTables?: boolean;
  relatedTextures?: any[];
  linkMode?: 'anchor' | 'text';
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
  // List lines in reading order, each with its nesting depth (1 + indent / 2, as the native reader reads them) and kind.
  let listItems: { depth: number; ordered: boolean; start: number; text: string }[] = [];
  let quote: string[] = [];
  let table: string[] = [];
  let code: string[] = [];
  let inCode = false;
  const headingOffset = Number.isFinite(options.headingLevelOffset) ? Number(options.headingLevelOffset) : 0;
  const wrapTables = options.wrapTables !== false;

  const inline = (text: string) => renderInlineMarkdown(text, sourceEntities, options.relatedTextures || [], { linkMode: options.linkMode });

  function flushParagraph() {
    if (paragraph.length === 0) return;
    blocks.push(`<p>${inline(paragraph.join(' '))}</p>`);
    paragraph = [];
  }

  // A deeper line opens a list inside the open item; a shallower one closes lists; a change of kind at one depth starts a new list.
  function flushLists() {
    if (listItems.length === 0) return;
    let html = '';
    const open: { depth: number; ordered: boolean }[] = [];
    const tag = (ordered: boolean) => (ordered ? 'ol' : 'ul');
    const close = () => {
      const top = open.pop()!;
      html += `</li></${tag(top.ordered)}>`;
    };
    for (const item of listItems) {
      while (open.length > 0 && open[open.length - 1].depth > item.depth) close();
      const top = open[open.length - 1];
      if (top && top.depth === item.depth && top.ordered === item.ordered) {
        html += `</li><li>${inline(item.text)}`;
        continue;
      }
      if (top && top.depth === item.depth) close(); // the same depth, the other kind: a new list
      open.push({ depth: item.depth, ordered: item.ordered });
      // a list opening at 3 counts from 3, as the native reader does
      const startAttr = item.ordered && item.start > 1 ? ` start="${item.start}"` : '';
      html += `<${tag(item.ordered)}${startAttr}><li>${inline(item.text)}`;
    }
    while (open.length > 0) close();
    blocks.push(html);
    listItems = [];
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

    // A thematic break: three or more of -, * or _ alone on a line (the native reader's rule).
    if (/^([-*_])(\s*\1){2,}$/.test(trimmed)) {
      flushAll();
      blocks.push('<hr>');
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

    const indent = line.match(/^[ \t]*/)![0].replace(/\t/g, '    ').length;
    const depth = 1 + Math.floor(indent / 2);
    const bullet = trimmed.match(/^[-*+]\s+(.+)$/);
    const numbered = bullet ? null : trimmed.match(/^(\d+)\.\s+(.+)$/);
    if (bullet || numbered) {
      flushParagraph();
      flushQuote();
      flushTable();
      listItems.push({ depth, ordered: !bullet, start: numbered ? Number(numbered[1]) : 1, text: bullet ? bullet[1] : numbered![2] });
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
