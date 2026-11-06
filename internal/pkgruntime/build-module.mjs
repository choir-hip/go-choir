// Runtime install (S12): build one first-party package (ABI
// choir.app-frontend/svelte-inline/v1) into one self-contained ES module of
// ABI choir.app-frontend/module/v1 (docs/design/s12-runtime-install-build-2026-11-04.md).
//
// Usage: node build-module.mjs <frontend root> <package name> <out file> <requires_sdk>
//
// The Go side (internal/pkgruntime) lays out <frontend root> as
//   package.json                 the computer's own locked dependency manifest
//   node_modules -> its locked dependencies (read only)
//   src/packages/<name>/...      the verified release tree, nothing else
//   src/lib/app-sdk.js           the SDK shim written below, never the shell's
// and runs this script with no network. The module:
//   - bundles Svelte and the package's declared dependencies;
//   - reaches the shell SDK only through the shim, whose named imports read
//     ctx.sdk (bound at mount) at call time;
//   - exports abi, requires_sdk, sdk_imports (the SDK names it uses, so the
//     shell refuses a module whose names it lacks) and mount(target, ctx).
// The package boundary plugin is the build's own copy of
// frontend/scripts/package-boundary-plugin.mjs (a Go test keeps them equal).
import { writeFileSync, mkdirSync, rmSync, readFileSync, realpathSync, readdirSync, statSync } from 'node:fs';
import path from 'node:path';
import { build } from 'vite';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import packageBoundary from './package-boundary-plugin.mjs';

const [frontendRoot, name, outFile, requiresSdk] = process.argv.slice(2);
if (!frontendRoot || !/^[a-z][a-z0-9-]{1,39}$/.test(name || '') || !outFile || !requiresSdk) {
  console.error('usage: build-module.mjs <frontend root> <package name> <out file> <requires_sdk>');
  process.exit(2);
}
// Vite reports real paths (macOS /var is /private/var), so compare against the real root.
const root = realpathSync(path.resolve(frontendRoot));
const pkgDir = path.join(root, 'src/packages', name);
const manifest = JSON.parse(readFileSync(path.join(pkgDir, 'choir-package.json'), 'utf8'));
const entry = path.join(pkgDir, manifest.app.entry);
const sdkFile = path.join(root, 'src/lib/app-sdk.js');

mkdirSync(path.dirname(sdkFile), { recursive: true });
writeFileSync(sdkFile, `// The module/v1 SDK shim: every named import a package makes from the SDK
// resolves to a property of __sdk (Rollup synthetic named exports), read at
// call time from the SDK the shell bound at mount.
import { getContext } from 'svelte';
import { derived, get } from 'svelte/store';
const KEY = Symbol.for('choir.launch-context');
let bound = null;
export function bindSdk(sdk) { if (sdk) bound = sdk; }
// ctx.sdk is the only way in: the shell sets no SDK global (forced modularity M5).
const base = () => bound || {};
const launchStore = () => { try { return getContext(KEY) || null; } catch { return null; } };
// A module's settings are its own: the loader binds the package name, so no
// argument a module passes can name another package's setting.
const PACKAGE = ${JSON.stringify(name)};
const local = {
  getSetting(key) { return base().getSetting(key, PACKAGE); },
  setSetting(key, value) { return base().setSetting(key, value, PACKAGE); },
  onSetting(key, fn) { return base().onSetting(key, fn, PACKAGE); },
  getLaunchFile(appContext) {
    const b = base();
    if (appContext !== undefined) return b.launchFileFromAppContext(appContext);
    const s = launchStore();
    return s ? b.launchFileFromAppContext(get(s)) : null;
  },
  launchFileStore() {
    const s = launchStore();
    const b = base();
    if (!s) return { subscribe: (run) => { run(null); return () => {}; } };
    return derived(s, (v) => b.launchFileFromAppContext(v));
  },
};
export const __sdk = new Proxy({}, {
  get(_t, key) { return Object.prototype.hasOwnProperty.call(local, key) ? local[key] : base()[key]; },
  has(_t, key) { return key in local || key in base(); },
});
`);

const ENTRY_ID = '\0choir-module-entry';

const sdkImports = new Set();

function choirModule() {
  return {
    name: 'choir-module',
    enforce: 'pre',
    resolveId(id) {
      if (id === ENTRY_ID) return ENTRY_ID;
      return null;
    },
    load(id) {
      if (id !== ENTRY_ID) return null;
      return `import App from ${JSON.stringify(entry)};
import { bindSdk } from ${JSON.stringify(sdkFile)};
export const abi = 'choir.app-frontend/module/v1';
export const requires_sdk = ${JSON.stringify(requiresSdk)};
export const sdk_imports = "__CHOIR_SDK_IMPORTS__";
const STYLE = "__CHOIR_CSS__";
export function mount(target, ctx) {
  bindSdk(ctx && ctx.sdk);
  let style = null;
  if (STYLE) {
    style = document.createElement('style');
    style.setAttribute('data-choir-package-style', (ctx && ctx.package) || '');
    style.textContent = STYLE;
    document.head.appendChild(style);
  }
  const context = new Map([[Symbol.for('choir.launch-context'), ctx && ctx.launchContext]]);
  const app = new App({ target, props: (ctx && ctx.props) || {}, context });
  return {
    update(props) { app.$set(props); },
    destroy() { app.$destroy(); if (style) style.remove(); },
  };
}
`;
    },
    transform(code, id) {
      if (id === sdkFile) return { code, syntheticNamedExports: '__sdk' };
      return null;
    },
    async moduleParsed(info) {
      if (!info.ast || info.id === sdkFile) return;
      for (const node of info.ast.body) {
        if (node.type === 'ExportAllDeclaration' && node.source && typeof node.source.value === 'string') {
          const target = await this.resolve(node.source.value, info.id);
          if (target && target.id === sdkFile) this.error(`package "${name}" must import SDK names one by one (no export * from the SDK)`);
          continue;
        }
        if (node.type !== 'ImportDeclaration' && node.type !== 'ExportNamedDeclaration') continue;
        if (!node.source || typeof node.source.value !== 'string') continue;
        const resolved = await this.resolve(node.source.value, info.id);
        if (!resolved || resolved.id !== sdkFile) continue;
        for (const spec of node.specifiers || []) {
          const imported = spec.imported || spec.local;
          if (spec.type === 'ImportNamespaceSpecifier' || spec.type === 'ImportDefaultSpecifier') {
            this.error(`package "${name}" must import SDK names one by one (no default or namespace import)`);
          }
          if (imported && imported.name && imported.name !== 'bindSdk') sdkImports.add(imported.name);
        }
      }
    },
    generateBundle(_options, bundle) {
      for (const [file, item] of Object.entries(bundle)) {
        if (item.type === 'chunk' && file !== 'module.js') this.error(`a module/v1 build is one chunk, got ${file}`);
        // Nothing outside the verified bytes: a module that imports anything
        // (an external, a data: URL) would run code the digest does not cover.
        if (item.type === 'chunk' && (item.imports.length || item.dynamicImports.length)) {
          this.error(`a module/v1 build imports nothing, got ${[...item.imports, ...item.dynamicImports].join(', ')}`);
        }
      }
    },
  };
}

const outDir = path.join(path.dirname(path.resolve(outFile)), '.choir-module-out');
rmSync(outDir, { recursive: true, force: true });
await build({
  root,
  configFile: false,
  envFile: false,
  publicDir: false,
  logLevel: 'warn',
  clearScreen: false,
  mode: 'production',
  plugins: [
    choirModule(),
    packageBoundary({ root }),
    svelte({ configFile: false, preprocess: vitePreprocess() }),
  ],
  define: {
    __CHOIR_BUILD_VERSION__: JSON.stringify('package'),
    __CHOIR_BUILD_COMMIT__: JSON.stringify('package'),
    __CHOIR_BUILD_TIME__: JSON.stringify('package'),
  },
  build: {
    outDir,
    emptyOutDir: true,
    write: true,
    minify: true,
    target: 'es2020',
    sourcemap: false,
    copyPublicDir: false,
    modulePreload: false,
    cssCodeSplit: false,
    assetsInlineLimit: () => true,
    reportCompressedSize: false,
    rollupOptions: {
      input: ENTRY_ID,
      preserveEntrySignatures: 'strict',
      output: { format: 'es', entryFileNames: 'module.js', inlineDynamicImports: true },
    },
  },
});
// The CSS (Svelte's and the package's own files) goes into the module, which
// adds it on mount and removes it on destroy; nothing else may be emitted.
let css = '';
let code = '';
for (const file of readdirSync(outDir, { recursive: true }).map(String).sort()) {
  const full = path.join(outDir, file);
  if (statSync(full).isDirectory()) continue;
  if (file === 'module.js') code = readFileSync(full, 'utf8');
  else if (file.endsWith('.css')) css += readFileSync(full, 'utf8');
  else throw new Error(`a module/v1 build carries no separate asset (${file})`);
}
for (const placeholder of ['__CHOIR_SDK_IMPORTS__', '__CHOIR_CSS__']) {
  if (code.split(`"${placeholder}"`).length !== 2) throw new Error(`the module holds its ${placeholder} placeholder other than once`);
}
code = code
  .replace('"__CHOIR_SDK_IMPORTS__"', () => JSON.stringify([...sdkImports].sort()))
  .replace('"__CHOIR_CSS__"', () => JSON.stringify(css));
writeFileSync(path.resolve(outFile), code);
rmSync(outDir, { recursive: true, force: true });
