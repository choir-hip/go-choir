import { escapeHTML, renderInlineSourceRef, sourceEntityID } from './texture-source-renderer';

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
  if (matches(node, '[data-texture-source-ref]')) {
    const sourceEntityID = getAttr(node, 'data-source-entity-id');
    if (!sourceEntityID) return [];
    return [{
      type: 'source_ref',
      attrs: {
        id: getAttr(node, 'data-texture-source-node-id') || nextID('source-ref'),
        source_entity_id: sourceEntityID,
        display_mode: 'numbered_ref',
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

function serializeBlockNode(node: any, nextID: (kind: string) => string): StructuredNode | null {
  if (!node) return null;
  if (node.nodeType === TEXT_NODE) return paragraphFromInline(serializeInlineNodes(node, nextID), nextID);
  if (!isElement(node)) return null;
  if (matches(node, '[data-texture-source-flow]') || matches(node, '[data-texture-source-entity]')) return null;
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
      .map((child) => serializeBlockNode(child, nextID))
      .filter(Boolean) as StructuredNode[];
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
  const content = childNodes(root)
    .map((node) => serializeBlockNode(node, nextID))
    .filter(Boolean) as StructuredNode[];
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

function renderInlineNode(node: StructuredNode, sourceEntities: any[] = []): string {
  if (node.type === 'text') {
    let html = escapeHTML(node.text || '');
    for (const mark of node.marks || []) {
      if (mark.type === 'strong') html = `<strong>${html}</strong>`;
      if (mark.type === 'emphasis') html = `<em>${html}</em>`;
      if (mark.type === 'code') html = `<code>${html}</code>`;
    }
    return html;
  }
  if (node.type === 'hard_break') return '<br>';
  if (node.type === 'source_ref') {
    const entityID = String(node.attrs?.source_entity_id || '').trim();
    const entity = sourceEntities.find((item) => sourceEntityID(item) === entityID);
    const html = renderInlineSourceRef(sourceEntityLabel(entity, entityID || 'source'), entityID, sourceEntities);
    const nodeID = String(node.attrs?.id || '').trim();
    return nodeID ? html.replace(' data-texture-source-ref', ` data-texture-source-ref data-texture-source-node-id="${escapeHTML(nodeID)}"`) : html;
  }
  return '';
}

function renderInlineContent(nodes: StructuredNode[] = [], sourceEntities: any[] = []): string {
  return nodes.map((node) => renderInlineNode(node, sourceEntities)).join('');
}

function renderBlockNode(node: StructuredNode, sourceEntities: any[] = []): string {
  switch (node.type) {
    case 'paragraph':
      return `<p>${renderInlineContent(node.content || [], sourceEntities)}</p>`;
    case 'heading': {
      const level = Math.min(6, Math.max(1, Number(node.attrs?.level) || 1));
      return `<h${level}>${renderInlineContent(node.content || [], sourceEntities)}</h${level}>`;
    }
    case 'bullet_list':
      return `<ul>${(node.content || []).map((item) => renderBlockNode(item, sourceEntities)).join('')}</ul>`;
    case 'ordered_list':
      return `<ol>${(node.content || []).map((item) => renderBlockNode(item, sourceEntities)).join('')}</ol>`;
    case 'list_item':
      return `<li>${(node.content || []).map((item) => item.type === 'paragraph' ? renderInlineContent(item.content || [], sourceEntities) : renderBlockNode(item, sourceEntities)).join('')}</li>`;
    case 'blockquote':
      return `<blockquote>${(node.content || []).map((item) => renderBlockNode(item, sourceEntities)).join('')}</blockquote>`;
    case 'code_block':
      return `<pre><code>${escapeHTML((node.content || []).map((item) => item.text || '').join('\n'))}</code></pre>`;
    case 'horizontal_rule':
      return '<hr>';
    case 'source_embed':
      return `<div data-texture-source-embed data-source-entity-id="${escapeHTML(String(node.attrs?.source_entity_id || ''))}"></div>`;
    default:
      return '';
  }
}

export function renderStructuredTextureDocHTML(bodyDoc: any, sourceEntities: any[] = []): string {
  if (!bodyDoc || bodyDoc.schema !== TEXTURE_DOC_SCHEMA || bodyDoc.doc?.type !== 'doc') return '';
  const blocks = Array.isArray(bodyDoc.doc?.content) ? bodyDoc.doc.content : [];
  return blocks.map((node: StructuredNode) => renderBlockNode(node, sourceEntities)).join('\n');
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
    case 'source_embed': {
      const entityID = String(node.attrs?.source_entity_id || '').trim();
      if (!entityID) return '';
      if (!sourceNumbers.has(entityID)) sourceNumbers.set(entityID, sourceNumbers.size + 1);
      return `[source ${sourceNumbers.get(entityID)}]`;
    }
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
  if (node.type === 'source_ref' || node.type === 'source_embed') {
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
