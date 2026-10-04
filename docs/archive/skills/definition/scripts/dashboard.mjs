#!/usr/bin/env node

import { createHash, randomBytes } from 'node:crypto';
import { watch as watchFileSystem } from 'node:fs';
import {
  mkdir,
  readFile,
  rename,
  stat,
  unlink,
  writeFile,
} from 'node:fs/promises';
import { createServer } from 'node:http';
import { isIP } from 'node:net';
import { basename, dirname, join, relative, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { TextDecoder } from 'node:util';

import { renderDashboard } from './dashboard-view.mjs';
import { collectRepositoryMetadata } from './dashboard-git.mjs';
import { createSessionLog } from './dashboard-session.mjs';

const SCRIPT_DIRECTORY = dirname(fileURLToPath(import.meta.url));
const RELOADABLE_SCRIPT_NAMES = new Set(['dashboard-view.mjs', 'dashboard-git.mjs']);
const GENERATOR_SCRIPT_NAME = 'dashboard.mjs';

export function isReloadableDashboardScript(filename) {
  if (filename === null || filename === undefined) return { kind: 'unknown' };
  const name = basename(String(filename));
  if (name === GENERATOR_SCRIPT_NAME) return { kind: 'generator', name };
  if (RELOADABLE_SCRIPT_NAMES.has(name)) return { kind: 'module', name };
  return { kind: 'ignored', name };
}

export async function loadDashboardScriptModules(scriptDirectory = SCRIPT_DIRECTORY) {
  const stamp = Date.now();
  const viewUrl = `${pathToFileURL(join(scriptDirectory, 'dashboard-view.mjs')).href}?t=${stamp}`;
  const gitUrl = `${pathToFileURL(join(scriptDirectory, 'dashboard-git.mjs')).href}?t=${stamp}`;
  const [view, git] = await Promise.all([import(viewUrl), import(gitUrl)]);
  if (typeof view.renderDashboard !== 'function') {
    throw new TypeError('dashboard-view.mjs must export renderDashboard');
  }
  if (typeof git.collectRepositoryMetadata !== 'function') {
    throw new TypeError('dashboard-git.mjs must export collectRepositoryMetadata');
  }
  return {
    renderer: view.renderDashboard,
    repositoryMetadataLoader: git.collectRepositoryMetadata,
  };
}

export const GENERATOR_VERSION = 'definition-dashboard-js/v1';
export const MAX_SOURCE_BYTES = 4 * 1024 * 1024;
export const MAX_RENDER_BYTES = 8 * 1024 * 1024;

const FORBIDDEN_KEYS = new Set(['__proto__', 'constructor', 'prototype']);
const HEALTH_CURRENT =
  'ok: dashboard current; non-authoritative projection; not completion and not evidence of mission completion\n';
const HEALTH_UNAVAILABLE =
  'unavailable: dashboard not current; non-authoritative projection; not completion and not evidence of mission completion\n';

export class DefinitionParseError extends Error {
  constructor(message, line = undefined) {
    super(line === undefined ? message : `YAML line ${line}: ${message}`);
    this.name = 'DefinitionParseError';
    this.line = line;
  }
}

export function escapeHtml(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function quoteStarts(text, index) {
  let previous = index - 1;
  while (previous >= 0 && text[previous] === ' ') previous -= 1;
  return previous < 0 || ':,[\-'.includes(text[previous]);
}

function stripYamlComment(text, lineNumber) {
  let quote = null;
  let escaped = false;

  for (let index = 0; index < text.length; index += 1) {
    const character = text[index];
    if (quote === '"') {
      if (escaped) {
        escaped = false;
      } else if (character === '\\') {
        escaped = true;
      } else if (character === '"') {
        quote = null;
      }
      continue;
    }
    if (quote === "'") {
      if (character === "'" && text[index + 1] === "'") {
        index += 1;
      } else if (character === "'") {
        quote = null;
      }
      continue;
    }
    if ((character === '"' || character === "'") && quoteStarts(text, index)) {
      quote = character;
      continue;
    }
    if (character === '#' && (index === 0 || /\s/.test(text[index - 1]))) {
      return text.slice(0, index).trimEnd();
    }
  }

  if (quote !== null || escaped) {
    throw new DefinitionParseError('unterminated quoted scalar', lineNumber);
  }
  return text.trimEnd();
}

function findMappingColon(text) {
  let quote = null;
  let escaped = false;
  let bracketDepth = 0;

  for (let index = 0; index < text.length; index += 1) {
    const character = text[index];
    if (quote === '"') {
      if (escaped) escaped = false;
      else if (character === '\\') escaped = true;
      else if (character === '"') quote = null;
      continue;
    }
    if (quote === "'") {
      if (character === "'" && text[index + 1] === "'") index += 1;
      else if (character === "'") quote = null;
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }
    if (character === '[') {
      bracketDepth += 1;
      continue;
    }
    if (character === ']') {
      bracketDepth -= 1;
      continue;
    }
    if (
      character === ':' &&
      bracketDepth === 0 &&
      (index + 1 === text.length || /\s/.test(text[index + 1]))
    ) {
      return index;
    }
  }
  return -1;
}

function rejectUnsupportedScalarSyntax(text, lineNumber) {
  if (/^[|>][+\-]?(?:\d+)?(?:\s|$)/.test(text)) {
    throw new DefinitionParseError('block scalars are not supported', lineNumber);
  }
  if (/^(?:!|&|\*)/.test(text)) {
    throw new DefinitionParseError('YAML tags, anchors, and aliases are not supported', lineNumber);
  }
  if (/^[%@`]/.test(text)) {
    throw new DefinitionParseError('reserved YAML indicators are not supported', lineNumber);
  }
  if (text.startsWith('{') || text.endsWith('}')) {
    throw new DefinitionParseError('inline maps are not supported', lineNumber);
  }
}

function parseSingleQuoted(text, lineNumber) {
  if (text.length < 2 || !text.endsWith("'")) {
    throw new DefinitionParseError('unterminated single-quoted scalar', lineNumber);
  }
  let result = '';
  for (let index = 1; index < text.length - 1; index += 1) {
    if (text[index] === "'") {
      if (text[index + 1] !== "'" || index + 1 >= text.length - 1) {
        throw new DefinitionParseError('unexpected content after quoted scalar', lineNumber);
      }
      result += "'";
      index += 1;
    } else {
      result += text[index];
    }
  }
  return result;
}

function parseDoubleQuoted(text, lineNumber) {
  try {
    const value = JSON.parse(text);
    if (typeof value !== 'string') throw new Error('not a string');
    return value;
  } catch {
    throw new DefinitionParseError(
      'invalid double-quoted scalar (use JSON-compatible escapes)',
      lineNumber,
    );
  }
}

function splitInlineArray(text, lineNumber) {
  const body = text.slice(1, -1);
  if (body.trim() === '') return [];

  const parts = [];
  let quote = null;
  let escaped = false;
  let depth = 0;
  let start = 0;

  for (let index = 0; index < body.length; index += 1) {
    const character = body[index];
    if (quote === '"') {
      if (escaped) escaped = false;
      else if (character === '\\') escaped = true;
      else if (character === '"') quote = null;
      continue;
    }
    if (quote === "'") {
      if (character === "'" && body[index + 1] === "'") index += 1;
      else if (character === "'") quote = null;
      continue;
    }
    if (character === '"' || character === "'") {
      quote = character;
      continue;
    }
    if (character === '[') depth += 1;
    else if (character === ']') {
      depth -= 1;
      if (depth < 0) throw new DefinitionParseError('malformed inline array', lineNumber);
    } else if (character === ',' && depth === 0) {
      const part = body.slice(start, index).trim();
      if (part === '') throw new DefinitionParseError('empty inline array item', lineNumber);
      parts.push(part);
      start = index + 1;
    }
  }

  if (quote !== null || escaped || depth !== 0) {
    throw new DefinitionParseError('malformed inline array', lineNumber);
  }
  const last = body.slice(start).trim();
  if (last === '') throw new DefinitionParseError('trailing comma in inline array', lineNumber);
  parts.push(last);
  return parts;
}

function parseScalar(text, lineNumber) {
  const scalar = text.trim();
  if (scalar === '') return null;
  rejectUnsupportedScalarSyntax(scalar, lineNumber);

  if (scalar.startsWith('[')) {
    if (!scalar.endsWith(']')) {
      throw new DefinitionParseError('unterminated inline array', lineNumber);
    }
    return splitInlineArray(scalar, lineNumber).map((item) => parseScalar(item, lineNumber));
  }
  if (scalar.startsWith('"')) return parseDoubleQuoted(scalar, lineNumber);
  if (scalar.startsWith("'")) return parseSingleQuoted(scalar, lineNumber);
  if (/^[?:](?:\s|$)/.test(scalar) || /^-(?:\s|$)/.test(scalar)) {
    throw new DefinitionParseError('ambiguous plain scalar', lineNumber);
  }
  if (/^(?:true|false)$/i.test(scalar)) return scalar.toLowerCase() === 'true';
  if (/^(?:null|~)$/i.test(scalar)) return null;
  if (/^[-+]?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][-+]?\d+)?$/.test(scalar)) {
    const number = Number(scalar);
    if (!Number.isFinite(number)) {
      throw new DefinitionParseError('number is outside the supported range', lineNumber);
    }
    return number;
  }
  if (/^[-+]?0\d/.test(scalar)) {
    throw new DefinitionParseError('leading-zero numbers are ambiguous', lineNumber);
  }
  if (/[\[\]{}]/.test(scalar)) {
    throw new DefinitionParseError('flow syntax is only supported for inline arrays', lineNumber);
  }
  if (/:(?:\s|$)/.test(scalar)) {
    throw new DefinitionParseError('a colon followed by whitespace is ambiguous in a plain scalar', lineNumber);
  }
  return scalar;
}

function parseMapKey(text, lineNumber) {
  const keyText = text.trim();
  if (keyText === '') throw new DefinitionParseError('empty map key', lineNumber);
  let key;
  if (keyText.startsWith('"')) key = parseDoubleQuoted(keyText, lineNumber);
  else if (keyText.startsWith("'")) key = parseSingleQuoted(keyText, lineNumber);
  else {
    if (!/^[A-Za-z0-9_.-]+$/.test(keyText)) {
      throw new DefinitionParseError('map keys must be simple or quoted strings', lineNumber);
    }
    key = keyText;
  }
  if (FORBIDDEN_KEYS.has(key)) {
    throw new DefinitionParseError(`unsafe map key ${JSON.stringify(key)}`, lineNumber);
  }
  return key;
}

function setUnique(target, key, value, lineNumber) {
  if (Object.hasOwn(target, key)) {
    throw new DefinitionParseError(`duplicate map key ${JSON.stringify(key)}`, lineNumber);
  }
  target[key] = value;
}

function isSequenceLine(text) {
  return text === '-' || text.startsWith('- ');
}

export function parseYamlSubset(yaml) {
  if (typeof yaml !== 'string') throw new TypeError('YAML source must be a string');
  const physicalLines = yaml.replaceAll('\r\n', '\n').split('\n');
  const lines = [];

  for (let index = 0; index < physicalLines.length; index += 1) {
    const raw = physicalLines[index];
    const lineNumber = index + 1;
    if (raw.includes('\r')) {
      throw new DefinitionParseError('bare carriage returns are not supported', lineNumber);
    }
    if (raw.includes('\t')) {
      throw new DefinitionParseError('tabs are not allowed', lineNumber);
    }
    const withoutComment = stripYamlComment(raw, lineNumber);
    if (withoutComment.trim() === '') continue;
    const indent = withoutComment.length - withoutComment.trimStart().length;
    if (indent % 2 !== 0) {
      throw new DefinitionParseError('indentation must use two-space levels', lineNumber);
    }
    lines.push({ indent, text: withoutComment.trimStart(), lineNumber });
  }

  if (lines.length === 0) return {};
  if (lines[0].indent !== 0) {
    throw new DefinitionParseError('the document root must not be indented', lines[0].lineNumber);
  }

  function parseMap(index, indent) {
    const result = {};
    let cursor = index;
    while (cursor < lines.length && lines[cursor].indent === indent) {
      const line = lines[cursor];
      if (isSequenceLine(line.text)) {
        throw new DefinitionParseError('cannot mix map and sequence entries at one level', line.lineNumber);
      }
      const colon = findMappingColon(line.text);
      if (colon < 0) throw new DefinitionParseError('expected a map entry', line.lineNumber);
      const key = parseMapKey(line.text.slice(0, colon), line.lineNumber);
      const valueText = line.text.slice(colon + 1).trim();
      cursor += 1;

      let value;
      if (valueText !== '') {
        value = parseScalar(valueText, line.lineNumber);
        if (cursor < lines.length && lines[cursor].indent > indent) {
          throw new DefinitionParseError(
            'a scalar map value cannot also contain an indented block',
            lines[cursor].lineNumber,
          );
        }
      } else if (cursor < lines.length && lines[cursor].indent > indent) {
        if (lines[cursor].indent !== indent + 2) {
          throw new DefinitionParseError('malformed indentation', lines[cursor].lineNumber);
        }
        [value, cursor] = parseBlock(cursor, indent + 2);
      } else {
        value = null;
      }
      setUnique(result, key, value, line.lineNumber);
    }
    return [result, cursor];
  }

  function parseSequence(index, indent) {
    const result = [];
    let cursor = index;
    while (cursor < lines.length && lines[cursor].indent === indent) {
      const line = lines[cursor];
      if (!isSequenceLine(line.text)) {
        throw new DefinitionParseError('cannot mix sequence and map entries at one level', line.lineNumber);
      }
      const itemText = line.text === '-' ? '' : line.text.slice(2).trim();
      cursor += 1;

      if (itemText === '') {
        if (cursor < lines.length && lines[cursor].indent > indent) {
          if (lines[cursor].indent !== indent + 2) {
            throw new DefinitionParseError('malformed indentation', lines[cursor].lineNumber);
          }
          let value;
          [value, cursor] = parseBlock(cursor, indent + 2);
          result.push(value);
        } else {
          result.push(null);
        }
        continue;
      }

      const colon = findMappingColon(itemText);
      if (colon < 0) {
        result.push(parseScalar(itemText, line.lineNumber));
        if (cursor < lines.length && lines[cursor].indent > indent) {
          throw new DefinitionParseError(
            'a scalar sequence item cannot contain an indented block',
            lines[cursor].lineNumber,
          );
        }
        continue;
      }

      const item = {};
      const firstKey = parseMapKey(itemText.slice(0, colon), line.lineNumber);
      const firstValueText = itemText.slice(colon + 1).trim();
      let firstValue;
      if (firstValueText !== '') {
        firstValue = parseScalar(firstValueText, line.lineNumber);
      } else if (cursor < lines.length && lines[cursor].indent >= indent + 4) {
        if (lines[cursor].indent !== indent + 4) {
          throw new DefinitionParseError('malformed indentation', lines[cursor].lineNumber);
        }
        [firstValue, cursor] = parseBlock(cursor, indent + 4);
      } else {
        firstValue = null;
      }
      setUnique(item, firstKey, firstValue, line.lineNumber);

      if (cursor < lines.length && lines[cursor].indent > indent) {
        if (lines[cursor].indent !== indent + 2) {
          throw new DefinitionParseError('malformed sequence map indentation', lines[cursor].lineNumber);
        }
        let continuation;
        [continuation, cursor] = parseMap(cursor, indent + 2);
        for (const [key, value] of Object.entries(continuation)) {
          setUnique(item, key, value, lines[cursor - 1]?.lineNumber ?? line.lineNumber);
        }
      }
      result.push(item);
    }
    return [result, cursor];
  }

  function parseBlock(index, indent) {
    const line = lines[index];
    if (!line || line.indent !== indent) {
      throw new DefinitionParseError('malformed indentation', line?.lineNumber);
    }
    return isSequenceLine(line.text) ? parseSequence(index, indent) : parseMap(index, indent);
  }

  const [document, cursor] = parseBlock(0, 0);
  if (cursor !== lines.length) {
    throw new DefinitionParseError('malformed or inconsistent indentation', lines[cursor].lineNumber);
  }
  if (Array.isArray(document)) {
    throw new DefinitionParseError('the Definition root must be a map', lines[0].lineNumber);
  }
  return document;
}

export function extractFrontMatter(source) {
  if (typeof source !== 'string') throw new TypeError('Definition source must be a string');
  const normalized = source.startsWith('\uFEFF') ? source.slice(1) : source;
  const lines = normalized.replaceAll('\r\n', '\n').split('\n');
  if (lines[0] !== '---') {
    throw new DefinitionParseError('Definition source must begin with YAML front matter');
  }
  const closing = lines.indexOf('---', 1);
  if (closing < 0) {
    throw new DefinitionParseError('Definition YAML front matter is not closed');
  }
  return lines.slice(1, closing).join('\n');
}

export function parseDefinitionSource(source) {
  const parsed = parseYamlSubset(extractFrontMatter(source));
  if (parsed.definition_version !== 2) {
    throw new DefinitionParseError('definition_version must be 2');
  }
  if (!parsed.finish || Array.isArray(parsed.finish) || typeof parsed.finish !== 'object') {
    throw new DefinitionParseError('finish must be a map');
  }
  if (!parsed.start || Array.isArray(parsed.start) || typeof parsed.start !== 'object') {
    throw new DefinitionParseError('start must be a map');
  }
  if (!parsed.now || Array.isArray(parsed.now) || typeof parsed.now !== 'object') {
    throw new DefinitionParseError('now must be a map');
  }
  return parsed;
}

export const parseDefinition = parseDefinitionSource;

function safeDisplayPath(sourcePath) {
  const absolute = resolve(sourcePath);
  const fromWorkingDirectory = relative(process.cwd(), absolute);
  if (
    fromWorkingDirectory !== '' &&
    fromWorkingDirectory !== '..' &&
    !fromWorkingDirectory.startsWith(`..${process.platform === 'win32' ? '\\' : '/'}`) &&
    !resolve(fromWorkingDirectory).startsWith('..')
  ) {
    return fromWorkingDirectory;
  }
  return basename(absolute);
}

function mapOrEmpty(value) {
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}
function normalizeRepositoryMetadata(value) {
  const metadata = mapOrEmpty(value);
  if (typeof metadata.fingerprint === 'string' && metadata.fingerprint !== '') return metadata;
  return {
    ...metadata,
    fingerprint: createHash('sha256').update(JSON.stringify(metadata)).digest('hex'),
  };
}


export function buildDashboardModel(
  parsed,
  {
    sourcePath = 'definition.md',
    digest,
    generatedAt = new Date().toISOString(),
    repositoryMetadata = { available: false, reason: 'Repository metadata was not collected.' },
    session = null,
  } = {},
) {
  const finish = mapOrEmpty(parsed.finish);
  const now = mapOrEmpty(parsed.now);
  const measures = Array.isArray(parsed.measures) ? parsed.measures : [];
  const version = parsed.definition_version;
  const title =
    typeof parsed.title === 'string'
      ? parsed.title
      : typeof finish.deliver === 'string'
        ? finish.deliver
        : 'Definition Dashboard';
  return {
    title,
    subtitle: `Live, non-authoritative projection · Definition v${version}`,
    finish,
    start: mapOrEmpty(parsed.start),
    now,
    repository: normalizeRepositoryMetadata(repositoryMetadata),
    session: session && typeof session === 'object' ? session : null,
    weakMeasures: [
      ...measures.filter((measure) => mapOrEmpty(measure).kind === 'weak_signal'),
      ...(Array.isArray(parsed.weak_measures) ? parsed.weak_measures : []),
      ...(Array.isArray(now.weak_measures) ? now.weak_measures : []),
    ],
    dissent: [
      ...(Array.isArray(parsed.dissent) ? parsed.dissent : []),
      ...(Array.isArray(now.dissent) ? now.dissent : []),
    ],
    orchestration: mapOrEmpty(parsed.orchestration),
    acceptance: Array.isArray(finish.acceptance) ? finish.acceptance : [],
    evidence: {
      referenced: Array.isArray(now.evidence_refs) ? now.evidence_refs : [],
      receipts: Array.isArray(parsed.receipts) ? parsed.receipts : [],
    },
    successor: mapOrEmpty(parsed.successor),
    source: {
      path: safeDisplayPath(sourcePath),
      digest: digest ?? '',
      generatedAt,
      generatorVersion: GENERATOR_VERSION,
    },
  };
}

async function readBoundedSource(sourcePath) {
  const sourceStats = await stat(sourcePath);
  if (!sourceStats.isFile()) throw new Error('source is not a regular file');
  if (sourceStats.size > MAX_SOURCE_BYTES) throw new Error('source exceeds size limit');
  const buffer = await readFile(sourcePath);
  if (buffer.byteLength > MAX_SOURCE_BYTES) throw new Error('source exceeds size limit');
  return buffer;
}

export async function renderDefinitionSource(
  source,
  {
    sourcePath = 'definition.md',
    generatedAt = new Date().toISOString(),
    renderer = renderDashboard,
    repositoryMetadata = { available: false, reason: 'Repository metadata was not collected.' },
    session = null,
  } = {},
) {
  const buffer = Buffer.isBuffer(source) ? source : Buffer.from(source, 'utf8');
  if (buffer.byteLength > MAX_SOURCE_BYTES) throw new Error('source exceeds size limit');
  const digest = createHash('sha256').update(buffer).digest('hex');
  let decoded;
  try {
    decoded = new TextDecoder('utf-8', { fatal: true }).decode(buffer);
  } catch {
    throw new DefinitionParseError('Definition source must be valid UTF-8');
  }
  const parsed = parseDefinitionSource(decoded);
  const model = buildDashboardModel(parsed, { sourcePath, digest, generatedAt, repositoryMetadata, session });
  const html = await renderer(model);
  if (typeof html !== 'string') throw new TypeError('renderDashboard must return an HTML string');
  if (Buffer.byteLength(html, 'utf8') > MAX_RENDER_BYTES) {
    throw new Error('rendered dashboard exceeds size limit');
  }
  return { html, model, parsed, digest };
}

export async function generateDashboard(
  sourcePath,
  {
    generatedAt = new Date().toISOString(),
    renderer = renderDashboard,
    repositoryMetadataLoader = collectRepositoryMetadata,
    sessionLog = null,
  } = {},
) {
  const [source, loadedRepositoryMetadata] = await Promise.all([
    readBoundedSource(sourcePath),
    repositoryMetadataLoader(sourcePath).catch(() => ({
      available: false,
      reason: 'Repository metadata could not be read.',
    })),
  ]);
  let repositoryMetadata = normalizeRepositoryMetadata(loadedRepositoryMetadata);
  if (sessionLog && typeof sessionLog.observeRepository === 'function') {
    const fingerprint = repositoryMetadata.fingerprint;
    repositoryMetadata = {
      ...normalizeRepositoryMetadata(sessionLog.observeRepository(repositoryMetadata, generatedAt)),
      fingerprint,
    };
  }
  const session = sessionLog && typeof sessionLog.snapshot === 'function' ? sessionLog.snapshot() : null;
  return renderDefinitionSource(source, {
    sourcePath,
    generatedAt,
    renderer,
    repositoryMetadata,
    session,
  });
}

export function isLoopbackHost(host) {
  if (typeof host !== 'string' || host === '') return false;
  const normalized = host.toLowerCase().replace(/\.$/, '');
  if (normalized === 'localhost') return true;
  if (isIP(normalized) === 4) {
    const first = Number(normalized.split('.')[0]);
    return first === 127;
  }
  if (isIP(normalized) === 6) {
    if (normalized === '::1' || normalized === '0:0:0:0:0:0:0:1') return true;
    const mapped = normalized.match(/^(?:::ffff:|0:0:0:0:0:ffff:)(\d+\.\d+\.\d+\.\d+)$/);
    return mapped ? isLoopbackHost(mapped[1]) : false;
  }
  return false;
}

export function parseListenAddress(address) {
  if (typeof address !== 'string' || address.trim() !== address || address === '') {
    throw new Error('listen address must be HOST:PORT');
  }
  let host;
  let portText;
  if (address.startsWith('[')) {
    const closing = address.indexOf(']');
    if (closing < 0 || address[closing + 1] !== ':') {
      throw new Error('IPv6 listen addresses must use [HOST]:PORT');
    }
    host = address.slice(1, closing);
    portText = address.slice(closing + 2);
  } else {
    const colon = address.lastIndexOf(':');
    if (colon <= 0 || address.indexOf(':') !== colon) {
      throw new Error('listen address must be HOST:PORT (bracket IPv6 hosts)');
    }
    host = address.slice(0, colon);
    portText = address.slice(colon + 1);
  }
  if (!isLoopbackHost(host)) {
    throw new Error('dashboard serving is restricted to localhost or a loopback IP');
  }
  if (!/^\d+$/.test(portText)) throw new Error('listen port must be an integer');
  const port = Number(portText);
  if (!Number.isSafeInteger(port) || port < 0 || port > 65535) {
    throw new Error('listen port must be between 0 and 65535');
  }
  return { host, port };
}

async function writeSnapshotAtomically(outputPath, html) {
  const destination = resolve(outputPath);
  await mkdir(dirname(destination), { recursive: true });
  const temporary = `${destination}.${process.pid}.${randomBytes(6).toString('hex')}.tmp`;
  try {
    await writeFile(temporary, html, { encoding: 'utf8', flag: 'wx', mode: 0o600 });
    await rename(temporary, destination);
  } catch (error) {
    await unlink(temporary).catch(() => {});
    throw error;
  }
}

function securityHeaders(contentType) {
  return {
    'Cache-Control': 'no-store',
    'Content-Type': contentType,
    'Content-Security-Policy':
      "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; font-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
    'Referrer-Policy': 'no-referrer',
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
  };
}

function publicFailure(kind, error) {
  if (kind === 'parse' && error instanceof DefinitionParseError) {
    return error.message.slice(0, 500);
  }
  if (kind === 'render') return 'Dashboard rendering failed.';
  if (kind === 'output') return 'Dashboard snapshot could not be written.';
  return 'Definition source could not be read.';
}

function unavailableDocument(message) {
  const eventScript =
    "const stream=new EventSource('/events');stream.addEventListener('reload',()=>location.reload());";
  return `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Dashboard unavailable</title></head><body><main><h1>Dashboard unavailable</h1><p>${escapeHtml(message)}</p><p>This projection is non-authoritative and is not evidence of mission completion.</p></main><script>${eventScript}</script></body></html>`;
}

export async function createDashboardServer({
  sourcePath,
  host = '127.0.0.1',
  port = 0,
  watch = false,
  outputPath,
  renderer = renderDashboard,
  watcherFactory = watchFileSystem,
  repositoryMetadataLoader = collectRepositoryMetadata,
  repositoryPollInterval = 1000,
  reloadScripts = false,
  scriptDirectory = SCRIPT_DIRECTORY,
  scriptWatcherFactory = watchFileSystem,
  loadScripts = loadDashboardScriptModules,
  scriptReloadDebounceMs = 120,
  onGeneratorScriptChange = null,
} = {}) {
  if (!sourcePath) throw new Error('sourcePath is required');
  if (!isLoopbackHost(host)) {
    throw new Error('dashboard serving is restricted to localhost or a loopback IP');
  }
  if (!Number.isInteger(port) || port < 0 || port > 65535) {
    throw new Error('listen port must be between 0 and 65535');
  }
  if (!Number.isInteger(repositoryPollInterval) || repositoryPollInterval < 0) {
    throw new Error('repository poll interval must be a non-negative integer');
  }
  if (!Number.isInteger(scriptReloadDebounceMs) || scriptReloadDebounceMs < 0) {
    throw new Error('script reload debounce must be a non-negative integer');
  }

  const clients = new Set();
  const state = { current: null, error: 'Dashboard has not been generated.' };
  const runtime = {
    renderer,
    repositoryMetadataLoader,
  };
  const sessionLog = createSessionLog();
  sessionLog.record({ kind: 'started', summary: 'Dashboard session started' });
  let watcher = null;
  let scriptWatcher = null;
  let closed = false;
  let repositoryPoller = null;
  let repositoryPollRunning = false;
  let watchFailed = false;
  let scriptReloadTimer = null;
  let scriptReloadChain = Promise.resolve();
  let refreshChain = Promise.resolve(false);
  let sourceRevision = 0;
  let pendingRefreshCause = 'start';

  function sendEvent(response, event, data) {
    if (response.destroyed || response.writableEnded) return false;
    const dataLines =
      data === undefined ? '' : `${String(data).split(/\r?\n/u).map((line) => `data: ${line}`).join('\n')}\n`;
    const message = `event: ${event}\n${dataLines}\n`;
    try {
      response.write(message);
      return true;
    } catch {
      response.destroy();
      return false;
    }
  }

  function broadcast(event, data) {
    for (const response of clients) {
      if (!sendEvent(response, event, data)) clients.delete(response);
    }
  }

  function invalidate(message) {
    const wasCurrent = state.current !== null;
    state.current = null;
    state.error = message;
    if (wasCurrent) broadcast('unavailable');
  }

  async function loadCurrent(revision) {
    if (closed || revision !== sourceRevision) return false;
    const cause = pendingRefreshCause;
    let generated;
    try {
      generated = await generateDashboard(sourcePath, {
        renderer: (...args) => runtime.renderer(...args),
        repositoryMetadataLoader: (...args) => runtime.repositoryMetadataLoader(...args),
        sessionLog,
      });
    } catch (error) {
      if (revision === sourceRevision) {
        invalidate(publicFailure(error instanceof DefinitionParseError ? 'parse' : 'read', error));
        sessionLog.record({
          kind: 'failed',
          summary: cause === 'start' ? 'Initial render failed' : `Refresh failed · ${cause}`,
        });
      }
      return false;
    }
    if (closed || revision !== sourceRevision) return false;

    const summary =
      cause === 'start'
        ? 'Became current · initial render'
        : `Became current · ${cause}`;
    sessionLog.record({
      kind: 'current',
      summary,
      detail: generated.digest ? `SHA-256 ${generated.digest.slice(0, 12)}` : null,
    });
    const session = sessionLog.snapshot();
    const model = { ...generated.model, session };
    let html;
    try {
      html = await runtime.renderer(model);
      if (typeof html !== 'string') throw new TypeError('renderDashboard must return an HTML string');
      if (Buffer.byteLength(html, 'utf8') > MAX_RENDER_BYTES) {
        throw new Error('rendered dashboard exceeds size limit');
      }
    } catch (error) {
      if (revision === sourceRevision) {
        invalidate(publicFailure('read', error instanceof Error ? error : new Error('render failed')));
        sessionLog.record({ kind: 'failed', summary: `Render failed · ${cause}` });
      }
      return false;
    }
    if (closed || revision !== sourceRevision) return false;

    if (outputPath) {
      try {
        await writeSnapshotAtomically(outputPath, html);
      } catch (error) {
        if (revision === sourceRevision) {
          invalidate(publicFailure('output', error));
          sessionLog.record({ kind: 'failed', summary: `Snapshot write failed · ${cause}` });
        }
        return false;
      }
    }
    if (closed || revision !== sourceRevision) return false;

    const repositoryChanged =
      state.current?.model?.repository?.fingerprint !== model.repository?.fingerprint;
    const shouldReload =
      state.current === null ||
      state.current.digest !== generated.digest ||
      repositoryChanged ||
      state.current?.model?.session?.eventCount !== session.eventCount;
    state.current = { html, digest: generated.digest, model };
    state.error = null;
    if (shouldReload) broadcast('reload', generated.digest);
    return true;
  }

  function refresh(cause = 'refresh') {
    if (closed || watchFailed) return Promise.resolve(false);
    pendingRefreshCause = typeof cause === 'string' && cause !== '' ? cause : 'refresh';
    const revision = ++sourceRevision;
    invalidate('Dashboard is regenerating from the Definition source and repository state.');
    const next = refreshChain.then(() => loadCurrent(revision), () => loadCurrent(revision));
    refreshChain = next;
    return next;
  }

  await refresh('start');

  const server = createServer((request, response) => {
    let pathname;
    try {
      pathname = new URL(request.url ?? '/', 'http://localhost').pathname;
    } catch {
      response.writeHead(400, securityHeaders('text/plain; charset=utf-8'));
      response.end('bad request\n');
      return;
    }
    if (request.method !== 'GET' && request.method !== 'HEAD') {
      response.writeHead(405, { ...securityHeaders('text/plain; charset=utf-8'), Allow: 'GET, HEAD' });
      response.end('method not allowed\n');
      return;
    }
    if (pathname === '/healthz') {
      const healthy = state.current !== null;
      response.writeHead(healthy ? 200 : 503, securityHeaders('text/plain; charset=utf-8'));
      response.end(request.method === 'HEAD' ? undefined : healthy ? HEALTH_CURRENT : HEALTH_UNAVAILABLE);
      return;
    }
    if (pathname === '/events') {
      if (request.method === 'HEAD') {
        response.writeHead(200, securityHeaders('text/event-stream; charset=utf-8'));
        response.end();
        return;
      }
      if (clients.size >= 64) {
        response.writeHead(503, securityHeaders('text/plain; charset=utf-8'));
        response.end('too many event subscribers\n');
        return;
      }
      response.writeHead(200, {
        ...securityHeaders('text/event-stream; charset=utf-8'),
        Connection: 'keep-alive',
      });
      response.write('retry: 1000\n\n');
      clients.add(response);
      const removeClient = () => clients.delete(response);
      request.on('close', removeClient);
      response.on('error', removeClient);
      if (state.current === null && !sendEvent(response, 'unavailable')) removeClient();
      return;
    }
    if (pathname === '/') {
      if (state.current === null) {
        response.writeHead(503, securityHeaders('text/html; charset=utf-8'));
        response.end(request.method === 'HEAD' ? undefined : unavailableDocument(state.error));
      } else {
        response.writeHead(200, securityHeaders('text/html; charset=utf-8'));
        response.end(request.method === 'HEAD' ? undefined : state.current.html);
      }
      return;
    }
    response.writeHead(404, securityHeaders('text/plain; charset=utf-8'));
    response.end(request.method === 'HEAD' ? undefined : 'not found\n');
  });

  await new Promise((fulfill, reject) => {
    const onError = (error) => {
      server.off('listening', onListening);
      reject(error);
    };
    const onListening = () => {
      server.off('error', onError);
      fulfill();
    };
    server.once('error', onError);
    server.once('listening', onListening);
    server.listen(port, host);
  });

  if (watch) {
    try {
      const sourceAbsolute = resolve(sourcePath);
      watcher = watcherFactory(dirname(sourceAbsolute), (eventType, filename) => {
        if (filename === null || basename(String(filename)) === basename(sourceAbsolute)) {
          void refresh('definition');
        }
      });
      watcher.on('error', () => {
        watchFailed = true;
        sourceRevision += 1;
        try {
          watcher?.close();
        } catch {}
        try {
          scriptWatcher?.close();
        } catch {}
        clearTimeout(scriptReloadTimer);
        invalidate('Definition source watch failed.');
      });

      if (reloadScripts) {
        const queueScriptReload = (kind) => {
          if (closed || watchFailed) return;
          clearTimeout(scriptReloadTimer);
          scriptReloadTimer = setTimeout(() => {
            scriptReloadChain = scriptReloadChain
              .catch(() => {})
              .then(async () => {
                if (closed || watchFailed) return;
                if (kind === 'generator') {
                  if (typeof onGeneratorScriptChange === 'function') {
                    await onGeneratorScriptChange();
                  } else {
                    process.stderr.write(
                      'dashboard: dashboard.mjs changed; restart the process to load server changes\n',
                    );
                  }
                  return;
                }
                const loaded = await loadScripts(scriptDirectory);
                if (closed || watchFailed) return;
                runtime.renderer = loaded.renderer;
                runtime.repositoryMetadataLoader = loaded.repositoryMetadataLoader;
                process.stdout.write('dashboard: reloaded renderer scripts\n');
                sessionLog.record({ kind: 'scripts', summary: 'Reloaded renderer scripts' });
                await refresh('scripts');
              })
              .catch((error) => {
                if (closed || watchFailed) return;
                invalidate(
                  publicFailure(
                    'read',
                    error instanceof Error ? error : new Error('Dashboard scripts could not be reloaded.'),
                  ),
                );
              });
          }, scriptReloadDebounceMs);
          scriptReloadTimer.unref?.();
        };

        scriptWatcher = scriptWatcherFactory(scriptDirectory, (eventType, filename) => {
          const classified = isReloadableDashboardScript(filename);
          if (classified.kind === 'module' || classified.kind === 'generator') {
            queueScriptReload(classified.kind);
          }
        });
        scriptWatcher.on('error', () => {
          watchFailed = true;
          sourceRevision += 1;
          try {
            watcher?.close();
          } catch {}
          try {
            scriptWatcher?.close();
          } catch {}
          clearTimeout(scriptReloadTimer);
          invalidate('Dashboard script watch failed.');
        });
      }

      await refresh('watch');
      if (repositoryPollInterval > 0) {
        repositoryPoller = setInterval(async () => {
          if (closed || watchFailed || repositoryPollRunning || state.current === null) return;
          repositoryPollRunning = true;
          try {
            const latest = normalizeRepositoryMetadata(await runtime.repositoryMetadataLoader(sourcePath).catch(() => ({
              available: false,
              reason: 'Repository metadata could not be read.',
            })));
            const currentFingerprint = state.current?.model?.repository?.fingerprint;
            if (latest.fingerprint !== currentFingerprint) void refresh('repository');
          } finally {
            repositoryPollRunning = false;
          }
        }, repositoryPollInterval);
        repositoryPoller.unref?.();
      }
    } catch (error) {
      try {
        watcher?.close();
      } catch {}
      try {
        scriptWatcher?.close();
      } catch {}
      clearTimeout(scriptReloadTimer);
      watcher = null;
      scriptWatcher = null;
      if (server.listening) {
        try {
          await new Promise((fulfill, reject) => {
            server.close((closeError) => (closeError ? reject(closeError) : fulfill()));
          });
        } catch {}
      }
      throw error;
    }
  }

  const listeningAddress = server.address();
  const actualPort = typeof listeningAddress === 'object' && listeningAddress ? listeningAddress.port : port;
  const urlHost = host.includes(':') ? `[${host}]` : host;

  return {
    server,
    host,
    port: actualPort,
    url: `http://${urlHost}:${actualPort}`,
    refresh,
    getState() {
      return state.current
        ? { available: true, digest: state.current.digest, model: state.current.model }
        : { available: false, error: state.error };
    },
    async close() {
      if (closed) return;
      clearInterval(repositoryPoller);
      clearTimeout(scriptReloadTimer);
      closed = true;
      watcher?.close();
      scriptWatcher?.close();
      for (const response of clients) response.end();
      clients.clear();
      await refreshChain.catch(() => {});
      await scriptReloadChain.catch(() => {});
      if (!server.listening) return;
      await new Promise((fulfill, reject) => {
        server.close((error) => (error ? reject(error) : fulfill()));
      });
    },
  };
}

function usage() {
  return `Usage: node skills/definition/scripts/dashboard.mjs <definition.md> [--serve HOST:PORT] [--watch] [--output PATH]\n\nThe Markdown/YAML Definition remains authoritative. --watch also hot-reloads dashboard-view.mjs and dashboard-git.mjs. --output writes only an explicitly requested snapshot.\n`;
}

export function parseCliArguments(argumentsList) {
  let sourcePath;
  let serve;
  let watch = false;
  let outputPath;

  for (let index = 0; index < argumentsList.length; index += 1) {
    const argument = argumentsList[index];
    if (argument === '--help' || argument === '-h') return { help: true };
    if (argument === '--watch') {
      if (watch) throw new Error('--watch may be specified only once');
      watch = true;
      continue;
    }
    if (argument === '--serve' || argument === '--output') {
      const value = argumentsList[index + 1];
      if (!value || value.startsWith('--')) throw new Error(`${argument} requires a value`);
      index += 1;
      if (argument === '--serve') {
        if (serve !== undefined) throw new Error('--serve may be specified only once');
        serve = value;
      } else {
        if (outputPath !== undefined) throw new Error('--output may be specified only once');
        outputPath = value;
      }
      continue;
    }
    if (argument.startsWith('--serve=')) {
      if (serve !== undefined) throw new Error('--serve may be specified only once');
      serve = argument.slice('--serve='.length);
      continue;
    }
    if (argument.startsWith('--output=')) {
      if (outputPath !== undefined) throw new Error('--output may be specified only once');
      outputPath = argument.slice('--output='.length);
      continue;
    }
    if (argument.startsWith('-')) throw new Error(`unknown option: ${argument}`);
    if (sourcePath !== undefined) throw new Error('exactly one Definition source is required');
    sourcePath = argument;
  }

  if (!sourcePath) throw new Error('a Definition source is required');
  if (watch && serve === undefined) throw new Error('--watch requires --serve');
  return { help: false, sourcePath, serve, watch, outputPath };
}

export async function runCli(argumentsList = process.argv.slice(2)) {
  const options = parseCliArguments(argumentsList);
  if (options.help) {
    process.stdout.write(usage());
    return null;
  }

  if (options.serve !== undefined) {
    const listen = parseListenAddress(options.serve);
    const dashboard = await createDashboardServer({
      sourcePath: options.sourcePath,
      host: listen.host,
      port: listen.port,
      watch: options.watch,
      outputPath: options.outputPath,
      reloadScripts: options.watch,
    });
    process.stdout.write(
      `Definition dashboard: ${dashboard.url}/\nAuthority: ${safeDisplayPath(options.sourcePath)} (dashboard is non-authoritative and not completion evidence)\n`,
    );
    return dashboard;
  }

  const generated = await generateDashboard(options.sourcePath);
  if (options.outputPath) await writeSnapshotAtomically(options.outputPath, generated.html);
  else process.stdout.write(generated.html);
  return generated;
}

const invokedPath = process.argv[1] ? resolve(process.argv[1]) : '';
if (invokedPath === fileURLToPath(import.meta.url)) {
  runCli().catch((error) => {
    const message = error instanceof DefinitionParseError ? error.message : String(error?.message ?? error);
    process.stderr.write(`dashboard: ${message}\n`);
    process.exitCode = 1;
  });
}
