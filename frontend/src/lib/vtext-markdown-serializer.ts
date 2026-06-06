function serializeInlineMarkdown(node: Node | null): string {
  if (!node) return '';
  if (node.nodeType === Node.TEXT_NODE) {
    return (node.textContent || '').replace(/\u00a0/g, ' ');
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return '';
  const element = node as Element;
  if (element.matches?.('[data-vtext-source-ref]')) {
    const label = element.getAttribute('data-source-label') || element.querySelector?.('.vtext-source-ref-label')?.textContent || 'source';
    const entityID = element.getAttribute('data-source-entity-id') || '';
    return entityID ? `[${label}](source:${entityID})` : label;
  }
  if (element.closest?.('[data-vtext-source-flow]')) return '';
  if (element.closest?.('[data-vtext-source-entity]')) return '';

  const tag = element.tagName.toLowerCase();
  if (tag === 'br') return '\n';
  const childText = Array.from(element.childNodes).map(serializeInlineMarkdown).join('');
  if (!childText) return '';
  if (tag === 'strong' || tag === 'b') return `**${childText}**`;
  if (tag === 'em' || tag === 'i') return `*${childText}*`;
  if (tag === 'code') return `\`${childText}\``;
  if (tag === 'a') {
    const href = element.getAttribute('href') || '';
    return href ? `[${childText}](${href})` : childText;
  }
  return childText;
}

function isMarkdownTableSeparatorCells(cells: string[] = []): boolean {
  return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(String(cell || '').trim()));
}

function serializeBlockMarkdown(node: Node | null): string {
  if (!node) return '';
  if (node.nodeType === Node.TEXT_NODE) {
    return (node.textContent || '').replace(/\u00a0/g, ' ');
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return '';
  const element = node as Element;
  if (element.matches?.('[data-vtext-source-flow]')) return '';
  if (element.matches?.('[data-vtext-source-entity]')) return '';

  const tag = element.tagName.toLowerCase();
  if (element.matches?.('.table-scroll') && element.querySelector?.('table')) {
    return serializeBlockMarkdown(element.querySelector('table'));
  }
  if (/^h[1-4]$/.test(tag)) {
    return `${'#'.repeat(Number(tag.slice(1)))} ${serializeInlineMarkdown(element).trim()}`;
  }
  if (tag === 'ul') {
    return Array.from(element.children)
      .filter((child) => child.tagName?.toLowerCase() === 'li')
      .map((child) => `- ${serializeInlineMarkdown(child).trim()}`)
      .join('\n');
  }
  if (tag === 'ol') {
    return Array.from(element.children)
      .filter((child) => child.tagName?.toLowerCase() === 'li')
      .map((child, index) => `${index + 1}. ${serializeInlineMarkdown(child).trim()}`)
      .join('\n');
  }
  if (tag === 'blockquote') {
    return Array.from(element.children)
      .map((child) => `> ${serializeInlineMarkdown(child).trim()}`)
      .join('\n');
  }
  if (tag === 'table') {
    const rows = Array.from(element.querySelectorAll('tr')).map((row) => {
      const cells = Array.from(row.children).filter((cell) => {
        const cellTag = cell.tagName?.toLowerCase();
        return cellTag === 'th' || cellTag === 'td';
      });
      return cells.map((cell) => serializeInlineMarkdown(cell).trim().replace(/\|/g, '\\|'));
    }).filter((cells) => cells.length > 0 && !isMarkdownTableSeparatorCells(cells));
    if (rows.length > 1) {
      const width = rows[0].length || 1;
      rows.splice(1, 0, Array.from({ length: width }).map(() => '---'));
    }
    return rows.map((cells) => `| ${cells.join(' | ')} |`).join('\n');
  }
  return serializeInlineMarkdown(element).trimEnd();
}

export function serializeEditorMarkdown(root: HTMLElement | null): string {
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
