<script lang="ts">
  // Hosts one app package's frontend module (S7-0a). The module is loaded by
  // verified digest (packages.ts) and mounted into a plain element; whatever
  // it does wrong stays inside this window.
  import { onDestroy } from 'svelte';
  import {
    installedPackages,
    loadPackageModule,
    packageContext,
    packageNameFromAppId,
    packagesLoaded,
    PackageLoadError,
    type PackageRecord,
  } from './packages';

  export let windowId = '';
  export let appContext: any = {};
  // AppHost passes these to every app; a package receives only its ctx.
  export const currentUser: any = null;
  export const authenticated = false;
  export const currentTheme: any = null;

  type FrameState = 'loading' | 'mounted' | 'refused' | 'failed' | 'unavailable';

  let target: HTMLElement;
  let state: FrameState = 'loading';
  let message = '';
  let handle: { destroy?: () => void } | null = null;
  let mountedKey = '';
  let token = 0;

  $: name = packageNameFromAppId(appContext?.appId || '');
  $: record = name ? $installedPackages.find((p) => p.name === name) || null : null;
  $: void sync(record, $packagesLoaded, target);

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

  async function sync(rec: PackageRecord | null, loaded: boolean, el: HTMLElement) {
    if (!el) return;
    if (!rec) {
      teardown();
      state = loaded ? 'unavailable' : 'loading';
      message = loaded ? `Package ${name || ''} is not installed` : '';
      return;
    }
    if (rec.status !== 'enabled') {
      teardown();
      state = 'unavailable';
      message = `Package ${rec.name} is not enabled`;
      return;
    }
    const key = `${rec.build_digest}|${rec.frontend.sha256}`;
    if (key === mountedKey && state === 'mounted') return;
    teardown();
    const mine = token;
    mountedKey = key;
    state = 'loading';
    message = '';
    try {
      const mod = await loadPackageModule(rec);
      if (mine !== token) return;
      if (typeof mod.mount !== 'function') {
        throw new PackageLoadError('failed', `package ${rec.name} does not export mount(target, ctx)`);
      }
      const result = mod.mount(el, packageContext(rec, windowId));
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

<div class="package-frame" data-package-frame data-package-name={name || ''} data-package-state={state}>
  <div class="package-mount" bind:this={target} hidden={state !== 'mounted'}></div>
  {#if state === 'loading'}
    <div class="package-state" role="status" aria-live="polite">Opening package…</div>
  {:else if state === 'unavailable'}
    <div class="package-state" role="status">{message}</div>
  {:else if state === 'refused' || state === 'failed'}
    <div class="package-state package-error" role="alert">
      <p>{state === 'refused' ? 'Package refused' : 'Package failed'}</p>
      <small>{message}</small>
    </div>
  {/if}
</div>

<style>
  .package-frame {
    display: flex;
    flex-direction: column;
    min-height: 100%;
  }

  .package-mount {
    flex: 1;
    min-height: 0;
  }

  .package-state {
    display: grid;
    gap: 0.3rem;
    place-content: center;
    min-height: 100%;
    flex: 1;
    color: var(--choir-muted);
    text-align: center;
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
