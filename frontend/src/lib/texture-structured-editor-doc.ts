import {
  escapeHTML,
  renderInlineSourceRef,
  sourceEntityID,
} from './texture-source-renderer';
import {
  citationAnchorsInHTML,
  renderCitationNotesHTML,
  renderPublishedCitationRef,
  type PublishedCitationContext,
} from './texture-published-citations';
import {
  allBlankOperations,
  embedProjectionText,
  renderEmbedBlockHTML,
  serializeEmbedElement,
} from './texture-embeds';

export const TEXTURE_DOC_SCHEMA = 'choir.texture_doc.v1';

const ELEMENT_NODE = 1;
const TEXT_NODE = 3;

type StructuredMark = {
  type: string;
  attrs?: Record<string, unknown>;
};

type StructuredNode = {
  type: string;
  attrs?: Record<string, unknown>;
  content?: StructuredNode[];
  text?: string;
  marks?: StructuredMark[];
};

export type StructuredTextureDoc = {
  schema: string;
  doc: StructuredNode;
};

type SerializeOptions = {
  existingDoc?: StructuredTextureDoc | null;
};

function nodeIDFactory(existingDoc?: StructuredTextureDoc | null) {
  let index = 0;
  const prefix = String(existingDoc?.doc?.attrs?.id || 'editor-doc').replace(/[^A-Za-z0-9_.:-]+/g, '-');
  return (kind: string) => `${prefix}-${kind}-${index++}`;
}

function childNodes(node: any): any[] {
  return Array.from(node?.childNodes || []);
}

function elementTag(node: any): string {
  return String(node?.tagName || '').toLowerCase();
}

function elementText(node: any): string {
  return String(node?.textContent || '').replace(/\u00a0/g, ' ');
}

function getAttr(node: any, name: string): string {
  return String(node?.getAttribute?.(name) || '').trim();
}

function matches(node: any, selector: string): boolean {
  return !!node?.matches?.(selector);
}

function isElement(node: any): boolean {
  return node?.nodeType === ELEMENT_NODE;
}

function textNode(value: string, marks: StructuredMark[] = []): StructuredNode | null {
  if (!value) return null;
  const node: StructuredNode = { type: 'text', text: value };
  if (marks.length > 0) node.marks = marks;
  return node;
}

function withMark(marks: StructuredMark[], type: string): StructuredMark[] {
  if (marks.some((mark) => mark.type === type)) return marks;
  return [...marks, { type }];
}

function serializeInlineNodes(node: any, nextID: (kind: string) => string, marks: StructuredMark[] = []): StructuredNode[] {
  if (!node) return [];
  if (node.nodeType === TEXT_NODE) {
    const item = textNode(elementText(node), marks);
    return item ? [item] : [];
  }
  if (!isElement(node)) return [];
  // An embed's caption is not the owner's text (texture-embeds.ts).
  if (matches(node, '[data-texture-embed]')) return [];
  if (matches(node, '[data-texture-source-ref]')) {
    const sourceEntityID = getAttr(node, 'data-source-entity-id');
    if (!sourceEntityID) return [];
    const displayMode = getAttr(node, 'data-source-display-mode') === 'expanded_ref' ? 'expanded_ref' : 'numbered_ref';
    return [{
      type: 'source_ref',
      attrs: {
        id: getAttr(node, 'data-texture-source-node-id') || nextID('source-ref'),
        source_entity_id: sourceEntityID,
        display_mode: displayMode,
      },
    }];
  }
  if (matches(node, '[data-texture-source-flow]') || matches(node, '[data-texture-source-entity]')) return [];
  const tag = elementTag(node);
  if (tag === 'br') return [{ type: 'hard_break' }];
  let nextMarks = marks;
  if (tag === 'strong' || tag === 'b') nextMarks = withMark(nextMarks, 'strong');
  if (tag === 'em' || tag === 'i') nextMarks = withMark(nextMarks, 'emphasis');
  if (tag === 'code') nextMarks = withMark(nextMarks, 'code');
  return childNodes(node).flatMap((child) => serializeInlineNodes(child, nextID, nextMarks));
}

function paragraphFromInline(inline: StructuredNode[], nextID: (kind: string) => string): StructuredNode {
  return {
    type: 'paragraph',
    attrs: { id: nextID('paragraph') },
    content: inline,
  };
}

function serializeBlockNode(node: any, nextID: (kind: string) => string, existingDoc?: StructuredTextureDoc | null): StructuredNode | null {
  if (!node) return null;
  if (node.nodeType === TEXT_NODE) return paragraphFromInline(serializeInlineNodes(node, nextID), nextID);
  if (!isElement(node)) return null;
  // An embed is kept by its id from the loaded revision, never built from
  // the DOM (station TE).
  if (matches(node, '[data-texture-embed]')) return serializeEmbedElement(node, existingDoc) as StructuredNode | null;
  if (matches(node, '[data-texture-source-flow]') || matches(node, '[data-texture-source-entity]')) return null;
  // Citation notes are rendered chrome under a paragraph, never typed text:
  // a note must never serialize back into the document (texture-citation-marker).
  if (matches(node, '[data-texture-source-note]') || matches(node, '[data-texture-citation-notes]')) return null;
  const tag = elementTag(node);
  if (/^h[1-6]$/.test(tag)) {
    return {
      type: 'heading',
      attrs: { id: nextID('heading'), level: Number(tag.slice(1)) },
      content: serializeInlineNodes(node, nextID),
    };
  }
  if (tag === 'ul' || tag === 'ol') {
    const items = Array.from(node?.children || [])
      .filter((child: any) => elementTag(child) === 'li')
      .map((child: any) => ({
        type: 'list_item',
        attrs: { id: nextID('list-item') },
        content: [paragraphFromInline(serializeInlineNodes(child, nextID), nextID)],
      }));
    return {
      type: tag === 'ul' ? 'bullet_list' : 'ordered_list',
      attrs: tag === 'ul' ? { id: nextID('list') } : { id: nextID('list'), start: 1 },
      content: items,
    };
  }
  if (tag === 'blockquote') {
    const content = childNodes(node)
      .map((child) => serializeBlockNode(child, nextID, existingDoc))
      .filter((child) => child && child.type !== 'embed') as StructuredNode[];
    return {
      type: 'blockquote',
      attrs: { id: nextID('blockquote') },
      content: content.length > 0 ? content : [paragraphFromInline(serializeInlineNodes(node, nextID), nextID)],
    };
  }
  if (tag === 'pre') {
    return {
      type: 'code_block',
      attrs: { id: nextID('code-block') },
      content: [{ type: 'text', text: elementText(node) }],
    };
  }
  if (tag === 'hr') {
    return { type: 'horizontal_rule', attrs: { id: nextID('hr') } };
  }
  return paragraphFromInline(serializeInlineNodes(node, nextID), nextID);
}

export function serializeEditorStructuredDoc(root: any, options: SerializeOptions = {}): StructuredTextureDoc {
  const nextID = nodeIDFactory(options.existingDoc);
  const embedIDs = new Set<string>();
  const content = (childNodes(root)
    .map((node) => serializeBlockNode(node, nextID, options.existingDoc))
    .filter(Boolean) as StructuredNode[])
    // A pasted copy of an embed is not a second piece of evidence.
    .filter((node) => {
      if (node.type !== 'embed') return true;
      const id = String(node.attrs?.id || '');
      if (embedIDs.has(id)) return false;
      embedIDs.add(id);
      return true;
    });
  return {
    schema: TEXTURE_DOC_SCHEMA,
    doc: {
      type: 'doc',
      attrs: { id: String(options.existingDoc?.doc?.attrs?.id || nextID('doc')) },
      content: content.length > 0 ? content : [paragraphFromInline([], nextID)],
    },
  };
}

function sourceEntityLabel(entity: any, fallback = 'source'): string {
  return String(entity?.display?.label || entity?.display?.title || entity?.label || entity?.source_entity_id || fallback || 'source').trim();
}

type RenderContext = {
  sourceEntities: any[];
  citations: PublishedCitationContext | null;
  allBlank?: Set<string>;
};

// Paired ** in a text node that carries no marks: bold written before Write
// parsed inline emphasis (d6f82142), which a later Write keeps verbatim when
// it copies the block's frame text back unchanged. The flanking rule is
// Write's: the opener is not preceded by a letter or digit and is followed
// by a non-space; the closer is preceded by a non-space and not followed by
// a letter or digit.
const LEGACY_STRONG_RE = /(^|[^\p{L}\p{N}*])\*\*(?=[^\s*])([^*]*?[^\s*])\*\*(?![\p{L}\p{N}*])/gu;

export function renderLegacyStrong(html: string): string {
  return html.replace(LEGACY_STRONG_RE, '$1<strong>$2</strong>');
}

function renderInlineNode(node: StructuredNode, rc: RenderContext): string {
  if (node.type === 'text') {
    let html = escapeHTML(node.text || '');
    if (!node.marks?.length) html = renderLegacyStrong(html);
    for (const mark of node.marks || []) {
      if (mark.type === 'strong') html = `<strong>${html}</strong>`;
      if (mark.type === 'emphasis') html = `<em>${html}</em>`;
      if (mark.type === 'code') html = `<code>${html}</code>`;
    }
    return html;
  }
  if (node.type === 'hard_break') return '<br>';
  if (node.type === 'source_ref') {
    const nodeID = String(node.attrs?.id || '').trim();
    // A published citation renders from its anchor: numbered source, quote
    // in its pinned block, live check badge.
    if (rc.citations && nodeID) {
      const cited = renderPublishedCitationRef(rc.citations, nodeID);
      if (cited !== null) return cited;
    }
    const sourceEntities = rc.sourceEntities;
    const entityID = String(node.attrs?.source_entity_id || '').trim();
    const entity = sourceEntities.find((item) => sourceEntityID(item) === entityID);
    const displayMode = String(node.attrs?.display_mode || 'numbered_ref').trim();
    const html = renderInlineSourceRef(sourceEntityLabel(entity, entityID || 'source'), entityID, sourceEntities, displayMode, { quote: sourceRefQuote(node) });
    return nodeID ? html.replace(' data-texture-source-ref', ` data-texture-source-ref data-texture-source-node-id="${escapeHTML(nodeID)}"`) : html;
  }
  return '';
}

function renderInlineContent(nodes: StructuredNode[] = [], rc: RenderContext): string {
  return nodes.map((node) => renderInlineNode(node, rc)).join('');
}

/** The words a Write citation quotes (its ref attr), or ''. */
export function sourceRefQuote(node: StructuredNode): string {
  let ref: any = node.attrs?.ref;
  if (typeof ref === 'string') {
    try {
      ref = JSON.parse(ref);
    } catch {
      ref = null;
    }
  }
  return String(ref?.quote || '').trim();
}

// A paragraph carries its citation notes in the flow under it: each marker
// toggles its own note and nothing overlays the text (owner direction,
// 2026-10-27). The notes are rendered from the same inline HTML the markers
// came from, so they name exactly the citations the paragraph makes and are
// resolved from the same published citation context.
function renderParagraph(node: StructuredNode, rc: RenderContext): string {
  const inlineHTML = renderInlineContent(node.content || [], rc);
  const notes = renderCitationNotesHTML(rc.citations, citationAnchorsInHTML(inlineHTML));
  return `<p>${inlineHTML}</p>${notes}`;
}

function renderBlockNode(node: StructuredNode, rc: RenderContext): string {
  switch (node.type) {
    case 'paragraph':
      return renderParagraph(node, rc);
    case 'heading': {
      const level = Math.min(6, Math.max(1, Number(node.attrs?.level) || 1));
      return `<h${level}>${renderInlineContent(node.content || [], rc)}</h${level}>`;
    }
    case 'bullet_list':
      return `<ul>${(node.content || []).map((item) => renderBlockNode(item, rc)).join('')}</ul>`;
    case 'ordered_list':
      return `<ol>${(node.content || []).map((item) => renderBlockNode(item, rc)).join('')}</ol>`;
    case 'list_item':
      return `<li>${(node.content || []).map((item) => item.type === 'paragraph' ? renderInlineContent(item.content || [], rc) : renderBlockNode(item, rc)).join('')}</li>`;
    case 'blockquote':
      return `<blockquote>${(node.content || []).map((item) => renderBlockNode(item, rc)).join('')}</blockquote>`;
    case 'code_block':
      return `<pre><code>${escapeHTML((node.content || []).map((item) => item.text || '').join('\n'))}</code></pre>`;
    case 'horizontal_rule':
      return '<hr>';
    case 'embed':
      return renderEmbedBlockHTML(node, { allBlank: rc.allBlank });
    default:
      return '';
  }
}

export function renderStructuredTextureDocHTML(
  bodyDoc: any,
  sourceEntities: any[] = [],
  options: { citations?: PublishedCitationContext | null } = {},
): string {
  if (!bodyDoc || bodyDoc.schema !== TEXTURE_DOC_SCHEMA || bodyDoc.doc?.type !== 'doc') return '';
  const blocks = Array.isArray(bodyDoc.doc?.content) ? bodyDoc.doc.content : [];
  const rc: RenderContext = { sourceEntities, citations: options.citations || null, allBlank: allBlankOperations(blocks) };
  return blocks.map((node: StructuredNode) => renderBlockNode(node, rc)).join('\n');
}

function projectInlineNode(node: StructuredNode, sourceNumbers: Map<string, number>): string {
  if (node.type === 'text') return node.text || '';
  if (node.type === 'hard_break') return '\n';
  if (node.type === 'source_ref') {
    const entityID = String(node.attrs?.source_entity_id || '').trim();
    if (!entityID) return '';
    if (!sourceNumbers.has(entityID)) sourceNumbers.set(entityID, sourceNumbers.size + 1);
    return `[${sourceNumbers.get(entityID)}]`;
  }
  return '';
}

function projectBlockNode(node: StructuredNode, sourceNumbers: Map<string, number>): string {
  switch (node.type) {
    case 'paragraph':
      return (node.content || []).map((child) => projectInlineNode(child, sourceNumbers)).join('');
    case 'heading':
      return `${'#'.repeat(Math.min(6, Math.max(1, Number(node.attrs?.level) || 1)))} ${(node.content || []).map((child) => projectInlineNode(child, sourceNumbers)).join('')}`;
    case 'bullet_list':
      return (node.content || []).map((item) => `- ${projectBlockNode(item, sourceNumbers)}`).join('\n');
    case 'ordered_list':
      return (node.content || []).map((item, index) => `${index + 1}. ${projectBlockNode(item, sourceNumbers)}`).join('\n');
    case 'list_item':
      return (node.content || []).map((child) => projectBlockNode(child, sourceNumbers)).join('\n');
    case 'blockquote':
      return (node.content || []).map((child) => projectBlockNode(child, sourceNumbers).split('\n').map((line) => `> ${line}`).join('\n')).join('\n\n');
    case 'code_block':
      return `\`\`\`\n${(node.content || []).map((child) => child.text || '').join('\n')}\n\`\`\``;
    case 'horizontal_rule':
      return '---';
    case 'embed':
      return embedProjectionText(node);
    default:
      return '';
  }
}

export function projectStructuredTextureDocText(bodyDoc: any): string {
  if (!bodyDoc || bodyDoc.schema !== TEXTURE_DOC_SCHEMA || bodyDoc.doc?.type !== 'doc') return '';
  const sourceNumbers = new Map<string, number>();
  const blocks = Array.isArray(bodyDoc.doc?.content) ? bodyDoc.doc.content : [];
  return blocks.map((node: StructuredNode) => projectBlockNode(node, sourceNumbers)).join('\n\n').trimEnd();
}

function collectSourceEntityIDsFromNode(node: StructuredNode, ids: Set<string>) {
  if (node.type === 'source_ref') {
    const entityID = String(node.attrs?.source_entity_id || '').trim();
    if (entityID) ids.add(entityID);
  }
  for (const child of node.content || []) collectSourceEntityIDsFromNode(child, ids);
}

export function structuredDocSourceEntityIDs(bodyDoc: any): string[] {
  const ids = new Set<string>();
  if (!bodyDoc || bodyDoc.schema !== TEXTURE_DOC_SCHEMA || bodyDoc.doc?.type !== 'doc') return [];
  for (const node of bodyDoc.doc?.content || []) collectSourceEntityIDsFromNode(node, ids);
  return [...ids];
}

export function sourceEntitiesForStructuredDoc(bodyDoc: any, sourceEntities: any[] = []): any[] {
  const ids = new Set(structuredDocSourceEntityIDs(bodyDoc));
  if (ids.size === 0) return [];
  return sourceEntities.filter((entity) => ids.has(sourceEntityID(entity)));
}
