import {
  layoutNextLineRange,
  materializeLineRange,
  prepareWithSegments,
  type LayoutCursor,
} from '@chenglou/pretext';
import {
  layoutNextRichInlineLineRange,
  materializeRichInlineLineRange,
  prepareRichInline,
  type RichInlineCursor,
} from '@chenglou/pretext/rich-inline';

export type SourceJournalFlowFragment = {
  text: string;
  itemIndex: number;
  gapBefore: number;
  occupiedWidth: number;
  sourceRefHTML?: string;
};

export type SourceJournalFlowLine = {
  text: string;
  width: number;
  x: number;
  y: number;
  fragments?: SourceJournalFlowFragment[];
};

export type SourceJournalFlowLayout = {
  lines: SourceJournalFlowLine[];
  height: number;
  noteWidth: number;
  noteHeight: number;
  usedNarrowLines: number;
};

export type SourceJournalFlowMode = 'side-note' | 'stacked';

export type SourceJournalFlowOptions = {
  text: string;
  containerWidth: number;
  noteWidth: number;
  noteHeight: number;
  gap: number;
  lineHeight: number;
  font: string;
};

export type SourceJournalFlowBlock = {
  element?: Element;
  text: string;
  items?: SourceJournalFlowItem[];
};

export type SourceJournalFlowItem = {
  text: string;
  font?: string;
  break?: 'normal' | 'never';
  extraWidth?: number;
  sourceRefHTML?: string;
};

export type SourceJournalFlowBlocksOptions = Omit<SourceJournalFlowOptions, 'text'> & {
  blocks: SourceJournalFlowBlock[];
  paragraphGap: number;
};

export type MountSourceJournalFlowOptions = {
  minWidth: number;
  gap: number;
  lineHeight: number;
};

const MIN_LINE_WIDTH = 180;
const MAX_FLOW_BLOCKS = 6;
const SOURCE_FLOW_BLOCK_SELECTOR = 'p';

function normalizeFlowText(value: unknown): string {
  return String(value || '').replace(/\s+/g, ' ').trim();
}

export function layoutSourceJournalFlowBlocks(options: SourceJournalFlowBlocksOptions): SourceJournalFlowLayout {
  const containerWidth = Math.max(0, Math.floor(options.containerWidth || 0));
  const noteWidth = Math.max(0, Math.min(Math.floor(options.noteWidth || 0), containerWidth));
  const noteHeight = Math.max(0, Math.ceil(options.noteHeight || 0));
  const gap = Math.max(0, Math.floor(options.gap || 0));
  const lineHeight = Math.max(1, Math.ceil(options.lineHeight || 1));
  const paragraphGap = Math.max(0, Math.floor(options.paragraphGap || 0));

  if (containerWidth < MIN_LINE_WIDTH || !options.font) {
    return { lines: [], height: 0, noteWidth, noteHeight, usedNarrowLines: 0 };
  }

  const narrowWidth = containerWidth - noteWidth - gap;
  const canRouteBesideNote = noteWidth > 0 && noteHeight > 0 && narrowWidth >= MIN_LINE_WIDTH;
  const lines: SourceJournalFlowLine[] = [];
  let y = 0;
  let usedNarrowLines = 0;

  for (const block of options.blocks || []) {
    const richItems = Array.isArray(block?.items)
      ? block.items
        .map((item) => ({
          ...item,
          text: String(item?.text || ''),
          font: item?.font || options.font,
        }))
        .filter((item) => item.text)
      : [];
    const text = normalizeFlowText(block?.text);
    if (richItems.length === 0 && !text) continue;
    const blockStartLineCount = lines.length;

    if (richItems.length > 0) {
      const prepared = prepareRichInline(richItems.map((item) => ({
        text: item.text,
        font: item.font || options.font,
        break: item.break,
        extraWidth: item.extraWidth,
      })));
      let cursor: RichInlineCursor | undefined;
      while (true) {
        const besideNote = canRouteBesideNote && y < noteHeight;
        const maxWidth = besideNote ? narrowWidth : containerWidth;
        const range = layoutNextRichInlineLineRange(prepared, maxWidth, cursor);
        if (range === null) break;
        const line = materializeRichInlineLineRange(prepared, range);
        lines.push({
          text: line.fragments.map((fragment) => fragment.text).join(''),
          width: line.width,
          x: 0,
          y,
          fragments: line.fragments.map((fragment) => ({
            text: fragment.text,
            itemIndex: fragment.itemIndex,
            gapBefore: fragment.gapBefore,
            occupiedWidth: fragment.occupiedWidth,
            sourceRefHTML: richItems[fragment.itemIndex]?.sourceRefHTML,
          })),
        });
        if (besideNote) usedNarrowLines += 1;
        cursor = line.end;
        y += lineHeight;
      }
    } else {
      const prepared = prepareWithSegments(text, options.font);
      let cursor: LayoutCursor = { segmentIndex: 0, graphemeIndex: 0 };
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
    }

    if (lines.length > blockStartLineCount) {
      y += paragraphGap;
    }
  }

  if (lines.length > 0) y = Math.max(0, y - paragraphGap);

  return {
    lines,
    height: Math.max(y, noteHeight),
    noteWidth,
    noteHeight,
    usedNarrowLines,
  };
}

export function layoutSourceJournalFlow(options: SourceJournalFlowOptions): SourceJournalFlowLayout {
  return layoutSourceJournalFlowBlocks({
    ...options,
    blocks: [{ text: options.text }],
    paragraphGap: 0,
  });
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
  if (element.matches?.('[data-texture-source-ref]')) {
    const label = element.querySelector?.('.texture-source-ref-label')?.textContent || element.getAttribute('data-source-label') || '';
    return label ? ` ${label} ` : ' ';
  }
  return Array.from(node.childNodes).map((child) => sourceFlowText(child, activeSourceRef)).join('');
}

function sourceRefFlowItem(element: Element): SourceJournalFlowItem {
  const label = element.querySelector?.('.texture-source-ref-label')?.textContent
    || element.getAttribute('data-source-label')
    || 'source';
  const width = Math.ceil(element.getBoundingClientRect?.().width || 0);
  return {
    text: label,
    font: sourceFlowFont(element),
    break: 'never',
    extraWidth: Math.max(0, width - label.length * 4),
    sourceRefHTML: element.outerHTML,
  };
}

function sourceFlowItems(node: Node | null, activeSourceRef: Element, font: string): SourceJournalFlowItem[] {
  if (!node) return [];
  if (node === activeSourceRef) return [];
  if (node.nodeType === Node.TEXT_NODE) {
    return [{ text: node.textContent || '', font }];
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return [];
  const element = node as Element;
  if (element.matches?.('[data-texture-source-ref]')) {
    return [sourceRefFlowItem(element)];
  }
  return Array.from(node.childNodes).flatMap((child) => sourceFlowItems(child, activeSourceRef, font));
}

function sourceRefEntityID(sourceRef: Element): string {
  return sourceRef.getAttribute('data-source-entity-id') || '';
}

function sourceRefTitle(sourceRef: Element, popover: Element): string {
  return normalizeFlowText(popover.querySelector('strong')?.textContent)
    || sourceRef.getAttribute('data-source-label')
    || 'Source';
}

function sourceRefExcerpt(popover: Element): string {
  return normalizeFlowText(popover.querySelector('[data-texture-transclusion-body] .texture-transclusion-quote')?.textContent);
}

function isSourceFlowBlock(element: Element | null): boolean {
  if (!element?.matches?.(SOURCE_FLOW_BLOCK_SELECTOR)) return false;
  if (element.closest?.('[data-texture-source-flow]')) return false;
  if (element.querySelector?.('table, iframe, img, pre, code, ul, ol, blockquote')) return false;
  return !!normalizeFlowText(element.textContent);
}

function buildSourceJournalNoteContent(note: HTMLElement, sourceRef: Element, popover: Element, close: HTMLButtonElement): void {
  const title = document.createElement('cite');
  title.className = 'texture-source-journal-cite';
  title.setAttribute('data-texture-source-flow-note-title', '');
  title.textContent = sourceRefTitle(sourceRef, popover);
  note.append(title);

  const excerpt = sourceRefExcerpt(popover);
  if (excerpt) {
    const body = document.createElement('p');
    body.classList.add('texture-source-journal-body');
    body.setAttribute('data-texture-source-flow-note-body', '');
    body.textContent = excerpt;
    note.append(body);
  }

  const actions = document.createElement('div');
  actions.className = 'texture-source-journal-actions';
  actions.setAttribute('data-texture-source-flow-note-actions', '');
  const entityID = sourceRefEntityID(sourceRef);
  if (entityID) {
    const open = document.createElement('button');
    open.type = 'button';
    open.className = 'texture-source-open';
    open.setAttribute('data-texture-open-source', '');
    open.setAttribute('data-source-entity-id', entityID);
    open.textContent = 'Open source';
    actions.append(open);
  }
  actions.append(close);
  note.append(actions);
}

function collectSourceFlowBlocks(paragraph: Element, sourceRef: Element, layoutOptions: SourceJournalFlowBlocksOptions): {
  blocks: SourceJournalFlowBlock[];
  layout: SourceJournalFlowLayout;
} {
  const blocks: SourceJournalFlowBlock[] = [];
  let cursor: Element | null = paragraph;
  let layout: SourceJournalFlowLayout = { lines: [], height: 0, noteWidth: layoutOptions.noteWidth, noteHeight: layoutOptions.noteHeight, usedNarrowLines: 0 };

  for (let index = 0; cursor && index < MAX_FLOW_BLOCKS; index += 1) {
    if (!isSourceFlowBlock(cursor)) break;
    blocks.push({
      element: cursor,
      text: sourceFlowText(cursor, sourceRef),
      items: sourceFlowItems(cursor, sourceRef, layoutOptions.font),
    });
    layout = layoutSourceJournalFlowBlocks({ ...layoutOptions, blocks });
    if (layout.lines.length > 0 && layout.height >= layoutOptions.noteHeight + layoutOptions.lineHeight) break;
    cursor = cursor.nextElementSibling;
  }

  return { blocks, layout };
}

export function clearSourceJournalFlows(root?: ParentNode | null): void {
  root?.querySelectorAll?.('[data-texture-source-flow]').forEach((node) => node.remove());
  root?.querySelectorAll?.('[data-texture-source-flow-hidden]').forEach((node) => {
    node.removeAttribute('data-texture-source-flow-hidden');
  });
  root?.querySelectorAll?.('[data-source-flow-mounted]').forEach((node) => {
    node.removeAttribute('data-source-flow-mounted');
  });
}

export function mountSourceJournalFlow(sourceRef: Element | null, options: MountSourceJournalFlowOptions): boolean {
  const paragraph = sourceRef?.closest?.('p');
  const popover = sourceRef?.querySelector?.('[data-texture-source-ref-popover]');
  if (!sourceRef || !paragraph || !popover || sourceRef.querySelector?.('iframe, img')) return false;

  const containerWidth = Math.floor(paragraph.clientWidth || paragraph.getBoundingClientRect?.().width || 0);
  if (containerWidth < MIN_LINE_WIDTH) return false;

  const text = sourceFlowText(paragraph, sourceRef).replace(/\s+/g, ' ').trim();
  if (!text) return false;

  const minNoteWidth = Math.min(260, containerWidth);
  const noteWidth = Math.min(340, Math.max(minNoteWidth, Math.floor(containerWidth * 0.34)));
  const paragraphGap = Math.max(8, Math.round(options.lineHeight * 0.55));
  const flow = document.createElement('div');
  flow.setAttribute('data-texture-source-flow', '');
  flow.setAttribute('data-texture-source-flow-region', '');
  flow.setAttribute('data-source-flow-owner-id', sourceRef.getAttribute('data-source-entity-id') || '');
  flow.className = 'texture-source-journal-flow';
  flow.setAttribute('contenteditable', 'false');
  flow.style.setProperty('--texture-source-flow-line-height', `${options.lineHeight}px`);
  flow.style.setProperty('--texture-source-flow-note-width', `${noteWidth}px`);
  flow.style.setProperty('--texture-source-flow-gap', `${options.gap}px`);

  const note = document.createElement('aside');
  note.setAttribute('data-texture-source-flow-note', '');
  note.className = 'texture-source-journal-note';
  note.setAttribute('role', 'note');

  const close = document.createElement('button');
  close.type = 'button';
  close.className = 'texture-source-flow-close';
  close.setAttribute('data-texture-source-flow-collapse', '');
  close.setAttribute('aria-label', 'Collapse source');
  close.textContent = 'Close';
  buildSourceJournalNoteContent(note, sourceRef, popover, close);
  flow.append(note);
  paragraph.insertAdjacentElement('beforebegin', flow);

  const measuredHeight = Math.ceil(note.getBoundingClientRect().height || 0);
  const sideLayoutOptions = {
    blocks: [],
    containerWidth,
    noteWidth,
    noteHeight: containerWidth >= options.minWidth ? measuredHeight : 0,
    gap: options.gap,
    lineHeight: options.lineHeight,
    paragraphGap,
    font: sourceFlowFont(paragraph),
  };
  let mode: SourceJournalFlowMode = 'side-note';
  let { blocks, layout } = collectSourceFlowBlocks(paragraph, sourceRef, sideLayoutOptions);

  if (layout.lines.length > 0 && layout.usedNarrowLines === 0) {
    mode = 'stacked';
    ({ blocks, layout } = collectSourceFlowBlocks(paragraph, sourceRef, {
      ...sideLayoutOptions,
      noteHeight: 0,
    }));
  }

  if (layout.lines.length === 0) {
    flow.remove();
    return false;
  }
  flow.setAttribute('data-texture-source-flow-mode', mode);
  flow.setAttribute('data-texture-source-flow-lines', String(layout.lines.length));
  flow.setAttribute('data-texture-source-flow-routed-lines', String(layout.usedNarrowLines));

  const lineLayer = document.createElement('div');
  lineLayer.className = 'texture-source-journal-lines';
  for (const line of layout.lines) {
    const lineNode = document.createElement('span');
    lineNode.className = 'texture-source-journal-line';
    if (mode === 'side-note' && line.y < layout.noteHeight) {
      lineNode.setAttribute('data-texture-source-flow-line-beside-note', '');
    }
    lineNode.setAttribute('data-texture-source-flow-line-width', String(Math.ceil(line.width)));
    lineNode.style.left = `${line.x}px`;
    lineNode.style.top = `${line.y}px`;
    lineNode.style.width = `${Math.ceil(line.width + 2)}px`;
    if (line.fragments?.length) {
      for (const fragment of line.fragments) {
        const fragmentNode = document.createElement('span');
        fragmentNode.className = fragment.sourceRefHTML ? 'texture-source-journal-fragment texture-source-journal-fragment--source' : 'texture-source-journal-fragment';
        if (fragment.gapBefore > 0) {
          fragmentNode.style.marginLeft = `${fragment.gapBefore}px`;
        }
        if (fragment.sourceRefHTML) {
          const template = document.createElement('template');
          template.innerHTML = fragment.sourceRefHTML;
          const clone = template.content.firstElementChild;
          if (clone) fragmentNode.append(clone);
          else fragmentNode.textContent = fragment.text;
        } else {
          fragmentNode.textContent = fragment.text;
        }
        lineNode.append(fragmentNode);
      }
    } else {
      lineNode.textContent = line.text;
    }
    lineLayer.append(lineNode);
  }
  if (mode === 'stacked') {
    flow.append(lineLayer, note);
    flow.style.setProperty('--texture-source-flow-stacked-note-top', `${Math.ceil(layout.height + paragraphGap)}px`);
    flow.style.height = `${Math.ceil(layout.height + paragraphGap + measuredHeight)}px`;
  } else {
    flow.append(lineLayer);
    flow.style.height = `${Math.ceil(layout.height)}px`;
  }
  blocks.forEach((block) => block.element?.setAttribute('data-texture-source-flow-hidden', ''));
  sourceRef.setAttribute('data-source-flow-mounted', 'true');
  return true;
}
