<!--
  ShellSlot: the host side of one shell slot (shell-slots.ts). It mounts the
  slot's active surface by hand, so a surface that throws while mounting is
  set aside and the next provider, or the last resort, takes its place (S3);
  the default slot content is the host's last-resort surface, shown only when
  no provider is healthy once registration has settled (S5).

  Props: `name` (the slot name), `props` (what the host gives the surface),
  `on` (the slot's events, name to handler), `enabled` (false unmounts the
  surface and shows nothing: a closed launcher or overview).
-->
<script lang="ts">
  import { onDestroy } from 'svelte';
  import { activeSurfaces, reportSurfaceFailure, surfacesSettled, type SurfaceSlot } from './shell-slots';
  import { packageIdentityContext } from './package-identity';

  export let name: SurfaceSlot;
  export let props: Record<string, unknown> = {};
  export let on: Record<string, (event: CustomEvent) => void> = {};
  export let enabled = true;

  let target: HTMLElement | null = null;
  let instance: any = null;
  let mountedId = '';
  let mountedComponent: unknown = null;

  $: surface = enabled ? $activeSurfaces[name] : undefined;
  $: if (target) mount(surface);
  $: if (instance) instance.$set(props);

  function unmount() {
    const old = instance;
    instance = null;
    mountedId = '';
    mountedComponent = null;
    try { old?.$destroy(); } catch (err) { console.warn(`[shell] the ${name} surface failed to unmount:`, err); }
  }

  function mount(next: typeof surface) {
    if (next && instance && next.id === mountedId && next.component === mountedComponent) return;
    unmount();
    if (!next || !target) return;
    try {
      const created = new (next.component as any)({ target, props, context: packageIdentityContext(next.id) });
      for (const [type, handler] of Object.entries(on)) created.$on(type, handler);
      instance = created;
      mountedId = next.id;
      mountedComponent = next.component;
    } catch (err) {
      target.replaceChildren();
      reportSurfaceFailure(name, next.id, err instanceof Error ? err.message : String(err));
    }
  }

  onDestroy(unmount);
</script>

<div class="shell-slot" data-shell-slot={name} data-shell-slot-provider={mountedId || undefined} bind:this={target}></div>
{#if enabled && $surfacesSettled && !surface}
  <slot />
{/if}

<style>
  .shell-slot {
    display: contents;
  }
</style>
