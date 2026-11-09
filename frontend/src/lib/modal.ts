// The shell modal (owner, 2026-11-09: "I guess we don't have a good flow for
// file selection with a files modal... should think about designing modals").
// One modal at a time, above every window, on a backdrop. ModalHost.svelte (the
// desktop mounts it once) draws it and owns focus: it moves focus in, keeps Tab
// inside, closes on Esc and on a backdrop click, and returns focus to whatever
// opened it. A modal is a Svelte component that receives its props and a
// `close(result)` function; openModal resolves with the result, or with
// `undefined` when the modal was cancelled.
//
// Failure modes:
//  M1 a second modal opens while one is up: the first is cancelled (resolves
//     undefined) and the second replaces it; never two at once;
//  M2 no host is mounted (logged-out boot, a unit): openModal resolves
//     undefined at once and shows nothing;
//  M3 Esc or a backdrop click: cancel, `undefined`;
//  M4 the opener left the page while the modal was up: focus goes to the body,
//     never throws.
import type { ComponentType } from 'svelte';
import { get, writable } from 'svelte/store';

export type ModalRequest = {
  seq: number;
  component: ComponentType;
  props: Record<string, unknown>;
  /** Accessible name of the dialog. */
  label: string;
  /** Whatever had focus when the modal opened. */
  opener: HTMLElement | null;
  resolve: (result: unknown) => void;
};

export const modalState = writable<ModalRequest | null>(null);
let hostMounted = 0;
let seq = 0;

/** ModalHost calls this on mount; the disposer on destroy. */
export function registerModalHost(): () => void {
  hostMounted += 1;
  return () => {
    hostMounted = Math.max(0, hostMounted - 1);
    if (hostMounted === 0) closeModal(undefined);
  };
}

export function openModal<T = unknown>(
  component: ComponentType,
  props: Record<string, unknown> = {},
  options: { label?: string } = {},
): Promise<T | undefined> {
  if (!hostMounted || typeof document === 'undefined') return Promise.resolve(undefined);
  closeModal(undefined); // M1
  const opener = document.activeElement instanceof HTMLElement && document.activeElement !== document.body ? document.activeElement : null;
  return new Promise<T | undefined>((resolve) => {
    modalState.set({
      seq: ++seq,
      component,
      props,
      label: options.label || 'Dialog',
      opener,
      resolve: resolve as (result: unknown) => void,
    });
  });
}

/** Close the open modal with a result (`undefined` cancels). */
export function closeModal(result?: unknown): void {
  const current = get(modalState);
  if (!current) return;
  modalState.set(null);
  current.resolve(result);
}

export function isModalOpen(): boolean {
  return Boolean(get(modalState));
}
