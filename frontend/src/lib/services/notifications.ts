// Notifications (slice H4; docs/design/modular-architecture-2026-11-04.md
// sections 2 and 4b). A notification is posted through the `notification.post`
// pipeline (payload { notification: { title, body, kind } }, the SDK lock's
// sections), and what comes out is shown by the notifications.host surface
// (the `notifications` package) or, with no healthy surface, the host's
// last-resort list. The host keeps the list and its timers, no UI of its own.
//
// Failure modes (written before the code):
//  N1 a handler throws or overruns its budget: the kernel skips it and the
//     notification goes out as it was (pipeline P2);
//  N2 a handler drops a notification (returns no item for it): nothing shows;
//     a handler that returns a malformed item: that item is dropped, the rest show;
//  N3 the pipeline itself fails (not defined, a cloning error): the
//     notification shows unchanged, so an error the owner must see is never lost;
//  N4 a dismissed notification's timer fires later: it removes nothing else.
import { writable, type Readable } from 'svelte/store';
import { shellKernel } from '../shell-kernel';

export const NOTIFICATION_PIPELINE = 'notification.post';
export type NotificationKind = 'info' | 'error';
export type ShellNotification = { id: number; title: string; body: string; kind: NotificationKind };

shellKernel.pipelines.define(shellKernel.root, NOTIFICATION_PIPELINE, { budgetMs: 50 });

const list = writable<ShellNotification[]>([]);
/** The notifications on screen, oldest first. */
export const notifications: Readable<ShellNotification[]> = { subscribe: list.subscribe };
let counter = 0;

const kindOf = (kind: unknown): NotificationKind => (kind === 'error' ? 'error' : 'info');

function show(title: string, body: string, kind: NotificationKind, durationMs: number): void {
  const id = ++counter;
  list.update((items) => [...items, { id, title, body, kind }]);
  setTimeout(() => dismissNotification(id), durationMs);
}

/** Post a notification: through the pipeline, then onto the screen. */
export async function postNotification(body: string, { title = '', kind = 'info', durationMs }: { title?: string; kind?: string; durationMs?: number } = {}): Promise<void> {
  const k = kindOf(kind);
  const duration = durationMs ?? (k === 'error' ? 9000 : 2400);
  let items: Record<string, unknown>[];
  try {
    const result = await shellKernel.pipelines.run(NOTIFICATION_PIPELINE, [{ notification: { title, body, kind: k } }]);
    for (const s of result.skipped) console.warn(`[notifications] handler ${s.handler} skipped: ${s.reason}`);
    items = result.items;
  } catch (err) {
    console.warn('[notifications] the pipeline failed; posting as is:', err);
    items = [{ notification: { title, body, kind: k } }];
  }
  for (const item of items) {
    const n = item?.notification as { title?: unknown; body?: unknown; kind?: unknown } | undefined;
    if (!n || typeof n.body !== 'string' || !n.body.trim()) continue;
    show(typeof n.title === 'string' ? n.title : '', n.body, kindOf(n.kind), duration);
  }
}

export function dismissNotification(id: number): void {
  list.update((items) => (items.some((n) => n.id === id) ? items.filter((n) => n.id !== id) : items));
}
