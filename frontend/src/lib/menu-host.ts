// The context-menu renderers (slice H2; services/context-menu.ts holds the
// slot). The host's last-resort list menu is the slot's base; the in-tree
// renderer packages (menu-default) are ordinary providers that rank over it.
// The desktop imports this module, so the renderers are in place before the
// first menu opens. Kept apart from context-menu.ts, which every package
// reaches through the SDK: this module imports the surface packages, and they
// import the SDK.
//
// Each renderer component is mounted once on first use, on document.body, and
// gets the open menu as its `request` prop while it is the one that rendered
// it. contextMenuState stays the one open menu, so closeContextMenu and
// isContextMenuOpen do not depend on the renderer.
import type { ComponentType } from 'svelte';
import LastResortMenu from './LastResortMenu.svelte';
import { contextMenuState } from './context-menu-state';
import { packageIdentityContext } from './package-identity';
import type { Scope } from './kernel';
import { addSurfaceRegistrar, inTreeSurfaces, recoveryBoot, surfaceScope, type InTreeSurface } from './shell-slots';
import { sdkSatisfies } from './sdk-version';
import { BASE_RENDERER_ID, RENDERER_SLOT, installBaseRenderer, registerContextMenuRenderer, type RenderRequest } from './services/context-menu';

type Mounted = { $set(props: Record<string, unknown>): void; $destroy(): void };
const mounted = new Map<string, Mounted>();
let renderedBy = '';

function mountRenderer(id: string, component: ComponentType, context?: Map<unknown, unknown>): Mounted | null {
  const existing = mounted.get(id);
  if (existing) return existing;
  if (typeof document === 'undefined') return null;
  const instance = new (component as any)({ target: document.body, props: { request: null }, ...(context ? { context } : {}) }) as Mounted;
  mounted.set(id, instance);
  return instance;
}

function unmountRenderer(id: string) {
  const instance = mounted.get(id);
  mounted.delete(id);
  try { instance?.$destroy(); } catch (err) { console.warn(`[context-menu] renderer ${id} failed to unmount:`, err); }
}

contextMenuState.subscribe((value) => {
  for (const [id, instance] of mounted) instance.$set({ request: value && id === renderedBy ? value : null });
});

function show(id: string, request: RenderRequest) {
  if (renderedBy && renderedBy !== id) mounted.get(renderedBy)?.$set({ request: null });
  renderedBy = id;
  contextMenuState.set(request);
}

// The last-resort menu is the base provider of the contextMenu.renderer slot
// (services/context-menu.ts): every renderer package ranks over it, and it
// answers when the active one fails or none is left.
installBaseRenderer((request) => {
  mountRenderer(BASE_RENDERER_ID, LastResortMenu as unknown as ComponentType);
  show(BASE_RENDERER_ID, request);
});

// The in-tree renderer packages (menu-default): ordinary providers of the slot.
function registerRendererSurface(s: InTreeSurface, scope: Scope): void {
  if (s.slot !== RENDERER_SLOT || !s.component) return;
  if (!sdkSatisfies(s.requires)) {
    console.warn(`[context-menu] ${s.name}: needs SDK ${s.requires}; its renderer is skipped`);
    return;
  }
  const { name, component } = s;
  registerContextMenuRenderer(scope, {
    id: name,
    tier: 'core',
    priority: s.priority,
    // An installed renderer is not the base image's default: the menu offers "Use default menu" while it renders.
    isDefault: !s.installed,
    impl: {
      render(request) {
        if (!mountRenderer(name, component as ComponentType, packageIdentityContext(name))) return false;
        show(name, request);
        return true;
      },
    },
  });
  scope.effect(() => {
    if (renderedBy === name) contextMenuState.set(null);
    unmountRenderer(name);
  });
}

// The recovery boot (?recovery=core) registers none: the last-resort menu answers (shell-slots S6).
if (!recoveryBoot) for (const s of inTreeSurfaces()) registerRendererSurface(s, surfaceScope(s.name));
// The recovery surface's "use default" registers it again (shell-slots restoreSurfacePackage).
addSurfaceRegistrar(registerRendererSurface);
