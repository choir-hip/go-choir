import {
  layoutNextLineRange,
  materializeLineRange,
  prepareWithSegments,
  type LayoutCursor,
} from '@chenglou/pretext';

export type SourceJournalFlowLine = {
  text: string;
  width: number;
  x: number;
  y: number;
};

export type SourceJournalFlowLayout = {
  lines: SourceJournalFlowLine[];
  height: number;
  noteWidth: number;
  noteHeight: number;
  usedNarrowLines: number;
};

export type SourceJournalFlowOptions = {
  text: string;
  containerWidth: number;
  noteWidth: number;
  noteHeight: number;
  gap: number;
  lineHeight: number;
  font: string;
};

export type MountSourceJournalFlowOptions = {
  minWidth: number;
  gap: number;
  lineHeight: number;
};

const MIN_LINE_WIDTH = 180;

export function layoutSourceJournalFlow(options: SourceJournalFlowOptions): SourceJournalFlowLayout {
  const text = String(options.text || '').replace(/\s+/g, ' ').trim();
  const containerWidth = Math.max(0, Math.floor(options.containerWidth || 0));
  const noteWidth = Math.max(0, Math.min(Math.floor(options.noteWidth || 0), containerWidth));
  const noteHeight = Math.max(0, Math.ceil(options.noteHeight || 0));
  const gap = Math.max(0, Math.floor(options.gap || 0));
  const lineHeight = Math.max(1, Math.ceil(options.lineHeight || 1));

  if (!text || containerWidth < MIN_LINE_WIDTH || !options.font) {
    return { lines: [], height: 0, noteWidth, noteHeight, usedNarrowLines: 0 };
  }

  const narrowWidth = containerWidth - noteWidth - gap;
  const canRouteBesideNote = noteWidth > 0 && noteHeight > 0 && narrowWidth >= MIN_LINE_WIDTH;
  const prepared = prepareWithSegments(text, options.font);
  const lines: SourceJournalFlowLine[] = [];
  let cursor: LayoutCursor = { segmentIndex: 0, graphemeIndex: 0 };
  let y = 0;
  let usedNarrowLines = 0;

  while (true) {
    const besideNote = canRouteBesideNote && y < noteHeight;
    const maxWidth = besideNote ? narrowWidth : containerWidth;
    const range = layoutNextLineRange(prepared, cursor, maxWidth);
    if (range === null) break;
    const line = materializeLineRange(prepared, range);
    lines.push({
      text: line.text,
      width: line.width,
      x: 0,
      y,
    });
    if (besideNote) usedNarrowLines += 1;
    cursor = range.end;
    y += lineHeight;
  }

  return {
    lines,
    height: Math.max(y, noteHeight),
    noteWidth,
    noteHeight,
    usedNarrowLines,
  };
}

function sourceFlowFont(paragraph: Element): string {
  if (typeof getComputedStyle !== 'function') return '16px serif';
  const style = getComputedStyle(paragraph);
  if (style.font) return style.font;
  return `${style.fontStyle || 'normal'} ${style.fontWeight || '400'} ${style.fontSize || '16px'} ${style.fontFamily || 'serif'}`;
}

function sourceFlowText(node: Node | null, activeSourceRef: Element): string {
  if (!node) return '';
  if (node === activeSourceRef) return '';
  if (node.nodeType === Node.TEXT_NODE) return node.textContent || '';
  if (node.nodeType !== Node.ELEMENT_NODE) return '';
  const element = node as Element;
  if (element.matches?.('[data-vtext-source-ref]')) {
    const label = element.getAttribute('data-source-label') || element.querySelector?.('.vtext-source-ref-label')?.textContent || '';
    return label ? ` ${label} ` : ' ';
  }
  return Array.from(node.childNodes).map((child) => sourceFlowText(child, activeSourceRef)).join('');
}

export function clearSourceJournalFlows(root?: ParentNode | null): void {
  root?.querySelectorAll?.('[data-vtext-source-flow]').forEach((node) => node.remove());
  root?.querySelectorAll?.('[data-vtext-source-flow-hidden]').forEach((node) => {
    node.removeAttribute('data-vtext-source-flow-hidden');
  });
  root?.querySelectorAll?.('[data-source-flow-mounted]').forEach((node) => {
    node.removeAttribute('data-source-flow-mounted');
  });
}

export function mountSourceJournalFlow(sourceRef: Element | null, options: MountSourceJournalFlowOptions): boolean {
  const paragraph = sourceRef?.closest?.('p');
  const popover = sourceRef?.querySelector?.('[data-vtext-source-ref-popover]');
  if (!sourceRef || !paragraph || !popover || sourceRef.querySelector?.('iframe, img')) return false;

  const containerWidth = Math.floor(paragraph.clientWidth || paragraph.getBoundingClientRect?.().width || 0);
  if (containerWidth < options.minWidth) return false;

  const text = sourceFlowText(paragraph, sourceRef).replace(/\s+/g, ' ').trim();
  if (!text) return false;

  const noteWidth = Math.min(380, Math.max(300, Math.floor(containerWidth * 0.42)));
  const note = document.createElement('span');
  note.setAttribute('data-vtext-source-flow', '');
  note.setAttribute('data-vtext-source-flow-note', '');
  note.className = 'vtext-source-journal-note';
  note.setAttribute('role', 'note');
  note.style.setProperty('--vtext-source-flow-note-width', `${noteWidth}px`);
  note.innerHTML = popover.innerHTML;

  const close = document.createElement('button');
  close.type = 'button';
  close.className = 'vtext-source-flow-close';
  close.setAttribute('data-vtext-source-flow-collapse', '');
  close.setAttribute('aria-label', 'Collapse source');
  close.textContent = 'Close';
  note.append(close);
  sourceRef.insertAdjacentElement('afterend', note);

  const measuredHeight = Math.ceil(note.getBoundingClientRect().height || 0);
  const layout = layoutSourceJournalFlow({
    text,
    containerWidth,
    noteWidth,
    noteHeight: measuredHeight,
    gap: options.gap,
    lineHeight: options.lineHeight,
    font: sourceFlowFont(paragraph),
  });

  if (layout.lines.length === 0 || layout.usedNarrowLines === 0) {
    note.remove();
    return false;
  }

  sourceRef.setAttribute('data-source-flow-mounted', 'true');
  return true;
}
