<script lang="ts">
  // Hosts one installed package's module (runtime install, S12;
  // docs/design/s12-runtime-install-build-2026-11-04.md). The module is
  // fetched by digest and verified (packages.ts), checked against the shell
  // SDK (requires_sdk, and every SDK name it imports), and mounted into a
  // plain element with ctx: { sdk, package, release, windowId, props,
  // launchContext, dispatch }. It runs in the shell's origin, which is why only
  // the owner's own releases install before the sandboxed frame (S13).
  import { createEventDispatcher, getContext, onDestroy } from 'svelte';
  import * as appSdk from '../app-sdk';
  import { SDK_VERSION, sdkSatisfies } from '../sdk-version';
  import { bindRegisterCommand } from '../services/commands';
  import { installedPackages, loadPackageModule, PackageLoadError, type InstalledPackageRecord } from './packages';

  export let windowId = '';
  export let appContext: any = {};
  export let currentUser: any = null;
  export let authenticated = false;
  export let currentTheme: any = null;

  type FrameState = 'loading' | 'mounted' | 'refused' | 'failed' | 'unavailable';

  const dispatch = createEventDispatcher();
  // AppHost provides the window's launch context; the module's own Svelte
  // reads it through ctx.launchContext.
  const launchContext = getContext(Symbol.for('choir.launch-context'));
  const sdk = Object.freeze({ ...appSdk });
  (globalThis as any).__choirAppSdk = sdk;

  let target: HTMLElement;
  let state: FrameState = 'loading';
  let message = '';
  let handle: { update?: (props: any) => void; destroy?: () => void } | null = null;
  let mountedKey = '';
  let token = 0;

  $: name = String(appContext?.appId || '');
  $: record = $installedPackages.find((p) => p.name === name) || null;
  $: props = { windowId, appContext, currentUser, authenticated, currentTheme };
  $: void sync(record, target);
  $: if (handle?.update && state === 'mounted') {
    try {
      handle.update(props);
    } catch {
      // A package that throws on update keeps its last state.
    }
  }

  function teardown() {
    token += 1;
    const current = handle;
    handle = null;
    mountedKey = '';
    try {
      current?.destroy?.();
    } catch {
      // A package that throws on destroy cannot keep its window alive.
    }
    if (target) target.replaceChildren();
  }

  async function sync(rec: InstalledPackageRecord | null, el: HTMLElement) {
    if (!el) return;
    if (!rec) {
      teardown();
      state = 'unavailable';
      message = `${name || 'This package'} is not installed on this computer.`;
      return;
    }
    const key = `${rec.install_id}|${rec.module.sha256}`;
    if (key === mountedKey && state === 'mounted') return;
    teardown();
    const mine = token;
    mountedKey = key;
    state = 'loading';
    message = '';
    try {
      if (!sdkSatisfies(rec.requires_sdk)) {
        throw new PackageLoadError('refused', `${rec.name} needs SDK ${rec.requires_sdk}; this shell has ${SDK_VERSION}.`);
      }
      const mod = await loadPackageModule(rec);
      if (mine !== token) return;
      if (typeof mod.mount !== 'function' || mod.abi !== rec.module.abi) {
        throw new PackageLoadError('refused', `${rec.name} is not a module/v1 module (no mount, or another ABI).`);
      }
      const missing = (Array.isArray(mod.sdk_imports) ? mod.sdk_imports : []).filter((n) => !(n in sdk));
      if (missing.length) {
        throw new PackageLoadError('refused', `${rec.name} uses SDK names this shell lacks: ${missing.join(', ')}.`);
      }
      // registerCommand is bound to this package: its commands are owned by rec.name whatever id they ask for (B5).
      const result = mod.mount(el, {
        sdk: Object.freeze({ ...sdk, registerCommand: bindRegisterCommand(rec.name) }),
        package: rec.name,
        release: rec.release_id,
        windowId,
        props,
        launchContext,
        dispatch: (type: string, detail: any) => dispatch(type, detail),
      });
      if (mine !== token) {
        result?.destroy?.();
        return;
      }
      handle = result || null;
      state = 'mounted';
    } catch (err) {
      if (mine !== token) return;
      target?.replaceChildren();
      state = err instanceof PackageLoadError ? err.state : 'failed';
      message = err instanceof Error ? err.message : String(err);
    }
  }

  onDestroy(teardown);
</script>

<div class="package-frame" data-package-frame data-package-name={name} data-package-state={state}>
  <div class="package-mount" bind:this={target} hidden={state !== 'mounted'}></div>
  {#if state === 'loading'}
    <div class="package-state" role="status" aria-live="polite">Opening {record?.manifest?.display?.title || name}…</div>
  {:else if state === 'unavailable'}
    <div class="package-state" role="status">{message}</div>
  {:else if state === 'refused' || state === 'failed'}
    <div class="package-state package-error" role="alert">
      <p>{state === 'refused' ? 'This app was not opened' : 'This app failed to open'}</p>
      <small>{message}</small>
    </div>
  {/if}
</div>

<style>
  .package-frame {
    display: flex;
    flex-direction: column;
    min-height: 100%;
    height: 100%;
  }

  .package-mount {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }

  .package-mount[hidden] {
    display: none;
  }

  .package-state {
    display: grid;
    gap: 0.3rem;
    place-content: center;
    min-height: 100%;
    flex: 1;
    color: var(--choir-muted);
    text-align: center;
    padding: 1rem;
  }

  .package-error p {
    margin: 0;
    color: var(--choir-fg);
    font-weight: 760;
  }

  .package-error small {
    color: var(--choir-danger);
    overflow-wrap: anywhere;
  }
</style>
