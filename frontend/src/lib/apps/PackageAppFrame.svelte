<script lang="ts">
  // Hosts one package's module: a base package shipped beside the shell
  // (forced modularity M4) or an installed release (runtime install, S12;
  // docs/design/s12-runtime-install-build-2026-11-04.md), by one path. The module is
  // fetched by digest and verified (packages.ts) and checked against the shell
  // SDK (requires_sdk, and every SDK name it uses). Then one of two realms:
  //  - shell: the base packages and releases on the owner's own track mount
  //    into a plain element with ctx: { sdk, package, release, windowId,
  //    props, launchContext }; registerCommand and setAppContext are bound to
  //    this package and this window;
  //  - frame (MA6.1): every other release runs in an opaque-origin sandboxed
  //    iframe and reaches the shell only through the bridge, each call checked
  //    against its ceiling (package-sandbox.ts). The computer says which
  //    (`isolation`); anything but "shell" is a frame.
  import { getContext, onDestroy } from 'svelte';
  import * as appSdk from '../app-sdk';
  import { SDK_VERSION, sdkSatisfies } from '../sdk-version';
  import { bindRegisterCommand } from '../services/commands';
  import { fetchVerifiedModule, isBaseRecord, loadPackageModule, PackageLoadError, type InstalledPackageRecord } from './packages';
  import { mountSandboxed, sandboxMissing } from './package-sandbox';
  import { packageRecordFor, registryRevision } from './registry';

  export let windowId = '';
  export let appContext: any = {};
  export let currentUser: any = null;
  export let authenticated = false;
  export let currentTheme: any = null;

  type FrameState = 'loading' | 'mounted' | 'refused' | 'failed' | 'unavailable';

  // AppHost provides the window's launch context; the module's own Svelte
  // reads it through ctx.launchContext.
  const launchContext = getContext(Symbol.for('choir.launch-context'));
  // The SDK reaches a module only as ctx.sdk: there is no global to read (M5).
  const sdk = Object.freeze({ ...appSdk });

  let target: HTMLElement;
  let state: FrameState = 'loading';
  let message = '';
  let handle: { update?: (props: any) => void; destroy?: () => void } | null = null;
  let isolation: 'shell' | 'frame' = 'shell';
  let mountedKey = '';
  let token = 0;

  $: name = String(appContext?.appId || '');
  // The base copy, or the installed release that replaces it (the registry decides).
  $: record = recordFor($registryRevision, name);
  function recordFor(_revision: number, appId: string): InstalledPackageRecord | null {
    return packageRecordFor(appId);
  }
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
    const realm = isolationOf(rec);
    const key = `${rec.install_id}|${rec.module.sha256}|${realm}`;
    if (key === mountedKey && state === 'mounted') return;
    teardown();
    const mine = token;
    mountedKey = key;
    isolation = realm;
    state = 'loading';
    message = '';
    try {
      if (!sdkSatisfies(rec.requires_sdk)) {
        throw new PackageLoadError('refused', `${rec.name} needs SDK ${rec.requires_sdk}; this shell has ${SDK_VERSION}.`);
      }
      if (realm === 'frame') {
        await mountFramed(rec, el, mine);
        return;
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
        sdk: Object.freeze({
          ...sdk,
          registerCommand: bindRegisterCommand(rec.name),
          // A module tells the shell only its own window's context and title.
          setAppContext: (change: any) => appSdk.setAppContext({ ...(change || {}), windowId }),
        }),
        package: rec.name,
        release: rec.release_id,
        windowId,
        props,
        launchContext,
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

  /** Where the record's code runs: the base copy and the owner's own track in the shell, every other release framed. */
  function isolationOf(rec: InstalledPackageRecord): 'shell' | 'frame' {
    return isBaseRecord(rec) || rec.isolation === 'shell' ? 'shell' : 'frame';
  }

  async function mountFramed(rec: InstalledPackageRecord, el: HTMLElement, mine: number) {
    const uses = Array.isArray(rec.uses) ? rec.uses.map(String) : null;
    if (!uses) throw new PackageLoadError('refused', `${rec.name} runs in a sandboxed frame, and this computer did not say which SDK names it uses.`);
    const missing = sandboxMissing(uses);
    if (missing.length) throw new PackageLoadError('refused', `${rec.name} runs in a sandboxed frame, which does not offer: ${missing.join(', ')}.`);
    const code = await fetchVerifiedModule(rec);
    if (mine !== token) return;
    const sandboxed = mountSandboxed(el, {
      pkg: rec.name,
      release: rec.release_id,
      windowId,
      manifest: rec.manifest,
      uses,
      code,
      props,
      launchContext: launchContext as any,
      onState(next, why) {
        if (mine !== token) return;
        state = next;
        message = why || '';
      },
    });
    handle = { update: (p: any) => sandboxed.update(p), destroy: () => sandboxed.destroy() };
  }

  onDestroy(teardown);
</script>

<div class="package-frame" data-package-frame data-package-name={name} data-package-install={record?.install_id || ''} data-package-state={state} data-package-isolation={isolation}>
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

  .package-mount :global(iframe.package-sandbox) {
    flex: 1;
    display: block;
    width: 100%;
    min-height: 0;
    border: 0;
    background: transparent;
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
