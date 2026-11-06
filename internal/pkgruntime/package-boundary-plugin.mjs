// Vite plugin: the package boundary, enforced on the resolved module graph
// (docs/design/app-packages-replan-2026-10-24.md §3.3).
//
// A module under src/packages/<name>/ may reach only: its own package, the
// shell's src/lib/app-sdk.ts, the svelte runtime, and the dependencies that
// frontend/package.json declares. The check runs where Vite resolves modules,
// so it covers static, dynamic, glob, `?raw`/`?url`, absolute and bare-path
// imports without parsing import syntax. Two things Vite does not resolve
// through plugins are covered by token rules on the compiled module instead:
// CSS (@import and url() read their files directly) and runtime reach
// (eval, Function, workers, import.meta other than env/hot, non-literal import()).
//
// This is an architectural boundary, not a security boundary: package code
// runs same-origin and can call any /api route the user can. Authority lives
// in the backend.
//
// What a static lint cannot see, said plainly: anything reached by a value
// rather than a name. A reference to the window obtained through an element
// (el.ownerDocument.defaultView is refused by name, but event.view,
// getRootNode(), a prototype walk or a callback that hands it over are not),
// observers and requestAnimationFrame loops, handler properties (el.onclick =),
// `(0, window.addEventListener)(...)`, member-callee timers with string bodies,
// member-object chains (document.body.addEventListener, document.write,
// document.head.appendChild(document.createElement('script'))), and any
// capability a dependency package exposes. The checks here make the
// honest reach visible (named imports, named globals) and refuse the common
// ways around that; they cannot make hostile code safe. The enforcement
// boundary for third-party code is the sandboxed iframe of MA6.1
// (src/lib/apps/package-sandbox.ts: a release on another owner's track runs
// in an opaque-origin frame); this gate holds the code that runs in the
// shell's realm (first-party and the owner's own track) to the contract.
import { readFileSync, realpathSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// The CSS functions that fetch (css-fetch-functions.json, beside this file; the
// theme gate in src/lib/theme.ts reads the same list).
const CSS_FETCH_FUNCTIONS = JSON.parse(readFileSync(path.join(path.dirname(fileURLToPath(import.meta.url)), 'css-fetch-functions.json'), 'utf8')).functions;
const FETCH_FN = new RegExp(`(^|[^a-z0-9_-])(?:${CSS_FETCH_FUNCTIONS.filter((f) => f !== 'url').map((f) => f.replace(/[-]/g, '\\-')).join('|')})\\s*\\(`);

export const SCRIPT_EXT = /\.(svelte|js|mjs|cjs|ts|mts|cts|tsx|jsx)$/i;
const CSS_EXT = /\.(css|scss|sass|less|styl|stylus|pcss|postcss)$/i;
const BANNED_IDENTIFIERS = new Set(['eval', 'Function', 'Worker', 'SharedWorker', 'importScripts']);
const ALLOWED_META = new Set(['env', 'hot']);
const SDK_GLOBAL = '__choirAppSdk';

const splitQuery = (id) => {
  const i = id.search(/[?#]/);
  return i < 0 ? [id, ''] : [id.slice(0, i), id.slice(i)];
};

// Raw registrations a package does not make itself (SDK onWindowEvent,
// onDocumentEvent and repeatEvery return disposers the shell can see).
const RAW_TARGETS = new Set(['window', 'document', 'globalThis', 'self', 'top', 'parent', 'opener']);
const memberName = (node) => (node.computed ? (node.property.type === 'Literal' ? String(node.property.value) : '') : node.property.name);
function rawRegistration(callee) {
  if (callee.type === 'Identifier') return callee.name === 'setInterval' ? 'setInterval' : '';
  if (callee.type !== 'MemberExpression' || callee.object.type !== 'Identifier' || !RAW_TARGETS.has(callee.object.name)) return '';
  const name = memberName(callee);
  if (name === 'addEventListener') return `${callee.object.name}.addEventListener`;
  if (name === 'setInterval') return `${callee.object.name}.setInterval`;
  return '';
}

export function cssProblem(source) {
  const text = source
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\\([0-9a-fA-F]{1,6})\s?/g, (_, hex) => {
      const cp = parseInt(hex, 16);
      return cp > 0 && cp <= 0x10ffff ? String.fromCodePoint(cp) : '';
    })
    .replace(/\\(.)/g, '$1')
    .toLowerCase();
  if (text.includes('@import')) return '@import is not allowed in package CSS';
  if (FETCH_FN.test(text)) return `${CSS_FETCH_FUNCTIONS.filter((f) => f !== 'url').join('(), ')}() are not allowed in package CSS`;
  for (const m of text.matchAll(/url\s*\(\s*(['"]?)\s*([^)'"]*)/g)) {
    const target = m[2].trim();
    if (!(target.startsWith('data:') || target.startsWith('#'))) {
      return 'url() in package CSS may only point at data: or #fragment (import assets from script instead)';
    }
  }
  return '';
}

function walk(node, parent, visit) {
  if (!node || typeof node !== 'object') return;
  if (Array.isArray(node)) {
    for (const child of node) walk(child, parent, visit);
    return;
  }
  if (typeof node.type !== 'string') return;
  visit(node, parent);
  for (const key of Object.keys(node)) {
    if (key === 'type' || key === 'start' || key === 'end' || key === 'loc') continue;
    const value = node[key];
    if (value && typeof value === 'object') walk(value, node, visit);
  }
}

// ---- the SDK reach and the global object ----

const SDK_SPECIFIER = /(^|\/)app-sdk(\.[a-z]+)?$/;
// Roots of the global object: a bare reference is an alias the lint cannot follow.
const GLOBAL_NAMES = new Set(['window', 'globalThis', 'self', 'top', 'parent', 'frames', 'opener', 'document']);
// Properties that hand back the window (or another one) from something else.
const WINDOW_LINKS = new Set(['window', 'globalThis', 'self', 'top', 'parent', 'frames', 'opener']);
const WINDOW_FROM_ANYTHING = new Set(['defaultView', 'contentWindow']);
// A literal key may not name the window again (window['self']['eval'] would drop the chain) or an escape hatch.
const BANNED_KEYS = new Set([...BANNED_IDENTIFIERS, 'constructor', '__proto__', ...GLOBAL_NAMES, ...WINDOW_LINKS, ...WINDOW_FROM_ANYTHING]);
const sdkSource = (node) => {
  if (!node) return false;
  if (node.type === 'Literal' && typeof node.value === 'string') return SDK_SPECIFIER.test(node.value);
  if (node.type === 'TemplateLiteral' && node.expressions.length === 0) return SDK_SPECIFIER.test(node.quasis[0].value.cooked ?? '');
  return false;
};
const isGlobalChain = (node, locals) => {
  if (node.type === 'Identifier') return GLOBAL_NAMES.has(node.name) && !locals.has(node.name);
  if (node.type !== 'MemberExpression' || node.computed || node.property.type !== 'Identifier') return false;
  if (WINDOW_FROM_ANYTHING.has(node.property.name)) return true;
  return (WINDOW_LINKS.has(node.property.name) || node.property.name === 'document') && isGlobalChain(node.object, locals);
};
// The names a module declares at module scope (a module-level `parent` is the module's own, not the window).
// A root name declared in any nested scope (parameter, catch binding, inner let) is refused instead of
// shadowing: a package has no reason to name one, and a scope-blind exemption disabled the gate file-wide.
function moduleLevelNames(ast) {
  const names = new Set();
  const pattern = (p) => {
    if (!p) return;
    if (p.type === 'Identifier') names.add(p.name);
    else if (p.type === 'ObjectPattern') p.properties.forEach((q) => pattern(q.type === 'RestElement' ? q.argument : q.value));
    else if (p.type === 'ArrayPattern') p.elements.forEach(pattern);
    else if (p.type === 'AssignmentPattern') pattern(p.left);
    else if (p.type === 'RestElement') pattern(p.argument);
  };
  for (const raw of ast.body || []) {
    const n = (raw.type === 'ExportNamedDeclaration' || raw.type === 'ExportDefaultDeclaration') && raw.declaration ? raw.declaration : raw;
    if (n.type === 'VariableDeclaration') n.declarations.forEach((d) => pattern(d.id));
    else if ((n.type === 'FunctionDeclaration' || n.type === 'ClassDeclaration') && n.id) pattern(n.id);
    else if (n.type === 'ImportDeclaration') n.specifiers.forEach((sp) => pattern(sp.local));
  }
  return names;
}
function nestedRootDeclarations(ast, moduleLevel) {
  const found = new Set();
  const pattern = (p) => {
    if (!p) return;
    if (p.type === 'Identifier') { if (GLOBAL_NAMES.has(p.name)) found.add(p.name); }
    else if (p.type === 'ObjectPattern') p.properties.forEach((q) => pattern(q.type === 'RestElement' ? q.argument : q.value));
    else if (p.type === 'ArrayPattern') p.elements.forEach(pattern);
    else if (p.type === 'AssignmentPattern') pattern(p.left);
    else if (p.type === 'RestElement') pattern(p.argument);
  };
  const topNodes = new Set();
  for (const raw of ast.body || []) {
    const n = (raw.type === 'ExportNamedDeclaration' || raw.type === 'ExportDefaultDeclaration') && raw.declaration ? raw.declaration : raw;
    topNodes.add(n);
    if (n.type === 'VariableDeclaration') n.declarations.forEach((d) => topNodes.add(d));
  }
  walk(ast, null, (n) => {
    if (n.type === 'VariableDeclarator') { if (!topNodes.has(n)) pattern(n.id); }
    else if (n.type === 'FunctionDeclaration' || n.type === 'FunctionExpression' || n.type === 'ArrowFunctionExpression') {
      if (n.id && !topNodes.has(n)) pattern(n.id);
      n.params.forEach(pattern);
    } else if (n.type === 'ClassDeclaration' && n.id && !topNodes.has(n)) pattern(n.id);
    else if (n.type === 'CatchClause') pattern(n.param);
  });
  return found;
}
// An Identifier that only names something (a property, a parameter, a declaration), not a read of the binding.
function namesOnly(node, parent) {
  if (!parent) return false;
  switch (parent.type) {
    case 'MemberExpression': return parent.property === node && !parent.computed;
    case 'Property': return parent.key === node && !parent.computed && !parent.shorthand;
    case 'MethodDefinition': case 'PropertyDefinition': return parent.key === node && !parent.computed;
    case 'VariableDeclarator': return parent.id === node;
    case 'FunctionDeclaration': case 'FunctionExpression': case 'ArrowFunctionExpression': return parent.params.includes(node) || parent.id === node;
    case 'ImportSpecifier': case 'ImportDefaultSpecifier': case 'ImportNamespaceSpecifier': case 'ExportSpecifier': return true;
    case 'LabeledStatement': case 'BreakStatement': case 'ContinueStatement': return true;
    default: return false;
  }
}
// A key the gate can read: a plain string literal that is not an escape hatch. A concatenation, template or variable is refused (constant folding would only prove the ones that are harmless).
function plainKey(node) {
  if (node.type === 'Literal' && (typeof node.value === 'string' || typeof node.value === 'number')) {
    const key = String(node.value);
    return !BANNED_KEYS.has(key) && !key.includes(SDK_GLOBAL);
  }
  return false;
}

/**
 * What one parsed package module reaches (U1; review r1 B3, N6). The same
 * analysis runs in the in-tree gate (package-uses.mjs, check-packages) and in
 * the installed build (this plugin's transform), so both see the same names:
 *  names    the SDK names the module imports or re-exports, one by one;
 *  problems everything that would hide a reach: a namespace or default import,
 *           `export *`, a dynamic import() of the SDK, computed access to the
 *           global object (split-string keys), and a bare `window`,
 *           `globalThis` or `self` (an alias, or what <svelte:window> compiles to).
 */
export function analyzeModule(ast, { checkMeta = true } = {}) {
  const names = new Set();
  const problems = [];
  const oneByOne = 'must import SDK names one by one (no namespace, default, export * or import() of the SDK)';
  for (const node of ast.body || []) {
    if (node.type === 'ImportDeclaration' && sdkSource(node.source)) {
      for (const spec of node.specifiers) {
        if (spec.type === 'ImportSpecifier') names.add(spec.imported.name ?? spec.imported.value);
        else problems.push(`a package ${oneByOne}`);
      }
    } else if (node.type === 'ExportNamedDeclaration' && node.source && sdkSource(node.source)) {
      for (const spec of node.specifiers) {
        if (spec.type === 'ExportSpecifier') names.add(spec.local.name ?? spec.local.value);
        else problems.push(`a package ${oneByOne}`);
      }
    } else if (node.type === 'ExportAllDeclaration' && sdkSource(node.source)) {
      problems.push(`a package ${oneByOne}`);
    }
  }
  const locals = moduleLevelNames(ast);
  for (const name of nestedRootDeclarations(ast, locals)) problems.push(`"${name}" is a name of the global object and may not be declared in a nested scope of a package (a parameter or inner variable would hide it from this check)`);
  walk(ast, null, (node, parent) => {
    if (node.type === 'ImportExpression' && sdkSource(node.source)) problems.push(`a package ${oneByOne}`);
    else if (node.type === 'MemberExpression' && node.computed && isGlobalChain(node.object, locals) && !plainKey(node.property)) {
      problems.push('computed access to the global object is not allowed in a package: name the property with a plain string literal');
    } else if (node.type === 'MemberExpression' && !node.computed && node.property.type === 'Identifier'
      && (WINDOW_LINKS.has(node.property.name) || WINDOW_FROM_ANYTHING.has(node.property.name))
      && isGlobalChain(node, locals) && !(parent?.type === 'MemberExpression' && parent.object === node)
      && !(parent?.type === 'UnaryExpression' && parent.operator === 'typeof')
      && !(parent?.type === 'BinaryExpression' && ['===', '!==', '==', '!='].includes(parent.operator))) {
      problems.push(`${node.property.name} hands back the window: use it directly (${node.property.name}.<name>) or compare it (window.top !== window); do not store or pass it`);
    } else if (node.type === 'Identifier' && GLOBAL_NAMES.has(node.name) && !locals.has(node.name) && parent) {
      const memberObject = parent.type === 'MemberExpression' && parent.object === node;
      const typeofOperand = parent.type === 'UnaryExpression' && parent.operator === 'typeof';
      const compared = parent.type === 'BinaryExpression' && ['===', '!==', '==', '!='].includes(parent.operator);
      if (!memberObject && !typeofOperand && !compared && !namesOnly(node, parent)) {
        problems.push(`bare "${node.name}" is not allowed in a package (an alias or <svelte:window> hides the listeners the shell must see): use ${node.name}.<name>, or onWindowEvent from the SDK`);
      }
    }
  });
  problems.push(...runtimeProblems(ast, { checkMeta }));
  return { names, problems: [...new Set(problems)] };
}

function runtimeProblems(ast, { checkMeta }) {
  const problems = [];
  walk(ast, null, (node, parent) => {
    if (node.type === 'Identifier' && BANNED_IDENTIFIERS.has(node.name)) {
      problems.push(`"${node.name}" is not allowed in a package`);
    } else if (
      (node.type === 'Identifier' && node.name === SDK_GLOBAL)
      || (node.type === 'Literal' && typeof node.value === 'string' && node.value.includes(SDK_GLOBAL))
      || (node.type === 'TemplateElement' && String(node.value?.cooked ?? node.value?.raw ?? '').includes(SDK_GLOBAL))
    ) {
      // The module/v1 shim's fallback global holds the whole SDK; a package
      // that read it would use names its sdk_imports never lists, so the
      // shell's name check could not see them. Import SDK names one by one.
      problems.push(`${SDK_GLOBAL} is the shell's, not a package's: import SDK names from app-sdk one by one`);
    } else if (node.type === 'MetaProperty' && checkMeta) {
      const prop = parent?.type === 'MemberExpression' && parent.object === node
        ? (parent.computed ? (parent.property.type === 'Literal' ? String(parent.property.value) : '') : parent.property.name)
        : '';
      if (!ALLOWED_META.has(prop)) problems.push('import.meta is only allowed as import.meta.env or import.meta.hot in a package');
    } else if (node.type === 'ImportExpression' && node.source?.type !== 'Literal') {
      problems.push('import() must take a string literal in a package');
    } else if (node.type === 'CallExpression' && rawRegistration(node.callee)) {
      const raw = rawRegistration(node.callee);
      problems.push(`${raw} is not allowed in a package: use ${raw === 'document.addEventListener' ? 'onDocumentEvent' : raw.endsWith('addEventListener') ? 'onWindowEvent' : 'repeatEvery'} from the SDK`);
    } else if (
      node.type === 'CallExpression' && node.callee.type === 'Identifier'
      && (node.callee.name === 'setTimeout' || node.callee.name === 'setInterval')
      && node.arguments[0]?.type === 'Literal' && typeof node.arguments[0].value === 'string'
    ) {
      problems.push(`${node.callee.name} with a string argument is not allowed in a package`);
    }
  });
  return [...new Set(problems)];
}

export default function packageBoundary({ root }) {
  // Vite resolves to real paths, so compare against the real root (macOS /var -> /private/var).
  const frontendRoot = realpathSync(path.resolve(root));
  const packagesDir = path.join(frontendRoot, 'src/packages') + path.sep;
  const sdkBase = path.join(frontendRoot, 'src/lib/app-sdk');
  let declared = new Set();
  try {
    declared = new Set(Object.keys(JSON.parse(readFileSync(path.join(frontendRoot, 'package.json'), 'utf8')).dependencies || {}));
  } catch {
    // no package.json: only svelte is allowed
  }
  // svelte-hmr is injected into every component by the svelte plugin in dev.
  const allowedModules = new Set(['svelte', 'svelte-hmr', ...declared]);
  let checkMeta = true;

  const packageOf = (id) => {
    const [file] = splitQuery(id);
    if (!path.isAbsolute(file) || !file.startsWith(packagesDir)) return '';
    return file.slice(packagesDir.length).split(path.sep)[0];
  };
  const isCss = (id) => {
    const [file, query] = splitQuery(id);
    return CSS_EXT.test(file) || /[?&]type=style\b/.test(query);
  };

  const allowed = (resolvedId, pkg, source) => {
    const [file] = splitQuery(resolvedId);
    if (file.startsWith('\0')) return true; // Vite and Rollup virtual helpers
    if (file.startsWith(packagesDir + pkg + path.sep)) return true;
    if (file.replace(/\.(ts|js|mjs)$/, '') === sdkBase) return true;
    if (/[\\/]node_modules[\\/]\.vite[\\/]deps[\\/]/.test(file)) {
      // The dev optimizer cache names files by package, not path: judge the specifier instead.
      const bare = /^(?:@[a-z0-9._-]+\/)?[a-z0-9._-]+(?:\/|$)/i.test(source) && !source.split('/').includes('..');
      const name = source.startsWith('@') ? source.split('/').slice(0, 2).join('/') : source.split('/')[0];
      return bare && allowedModules.has(name);
    }
    // The package a file belongs to is the one after its LAST node_modules:
    // pnpm keeps node_modules/.pnpm/<name>@<version>/node_modules/<name>/...,
    // where the first segment after node_modules is .pnpm, not the package.
    const segments = [...file.matchAll(/[\\/]node_modules[\\/]((?:@[^\\/]+[\\/])?[^\\/]+)[\\/]/g)];
    const last = segments[segments.length - 1];
    if (last) return allowedModules.has(last[1].replace(/\\/g, '/'));
    return false;
  };

  return [
    {
      name: 'choir-package-boundary:resolve',
      enforce: 'pre',
      async resolveId(source, importer, options) {
        if (!importer) return null;
        const pkg = packageOf(importer);
        if (!pkg) return null;
        // A URL (data:, http:, blob:, any scheme) is never a package's
        // import: Vite does not resolve a data: URL and Rollup would leave it
        // external, so its bytes would run unchecked. (An absolute path is
        // judged below, after it resolves; Svelte's own CSS imports use one.)
        if (/^[a-z][a-z0-9+.-]*:/i.test(source) || source.startsWith('//')) {
          this.error(`package "${pkg}" may not import "${source.slice(0, 80)}"; a package reaches only its own files, src/lib/app-sdk.ts, svelte and declared dependencies`);
        }
        const resolved = await this.resolve(source, importer, { ...options, skipSelf: true });
        if (!resolved || resolved.external) {
          this.error(`package "${pkg}" may not import "${source.slice(0, 80)}" (it does not resolve to a file); a package reaches only its own files, src/lib/app-sdk.ts, svelte and declared dependencies`);
        }
        if (!allowed(resolved.id, pkg, source)) {
          const shown = path.isAbsolute(resolved.id) ? path.relative(frontendRoot, splitQuery(resolved.id)[0]) : resolved.id;
          this.error(`package "${pkg}" may not import "${source}" (resolves to ${shown}); a package reaches only its own files, src/lib/app-sdk.ts, svelte and declared dependencies`);
        }
        return resolved;
      },
      transform(code, id) {
        if (!packageOf(id) || !isCss(id)) return null;
        const problem = cssProblem(code);
        if (problem) this.error(`package "${packageOf(id)}": ${problem} (${path.relative(frontendRoot, splitQuery(id)[0])})`);
        return null;
      },
    },
    {
      name: 'choir-package-boundary:runtime',
      enforce: 'post',
      configResolved(config) {
        // The dev server's svelte-hmr wrapper reads import.meta in every component;
        // that is tooling, not package code. The build is the gate.
        checkMeta = config.command === 'build';
      },
      transform(code, id) {
        const pkg = packageOf(id);
        // Data files (json, images, ?raw text) are not code; script files that fail to parse are refused.
        if (!pkg || isCss(id) || id.startsWith('\0') || !SCRIPT_EXT.test(splitQuery(id)[0])) return null;
        let ast;
        try {
          ast = this.parse(code);
        } catch (e) {
          this.error(`package "${pkg}": cannot parse ${path.relative(frontendRoot, splitQuery(id)[0])} to check it: ${e.message}`);
        }
        const { problems } = analyzeModule(ast, { checkMeta });
        if (problems.length) this.error(`package "${pkg}" ${path.relative(frontendRoot, splitQuery(id)[0])}: ${problems.join('; ')}`);
        return null;
      },
    },
  ];
}
