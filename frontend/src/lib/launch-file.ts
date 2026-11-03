// Launch args: the file a window was opened on (Open, Open with, Upload and
// open). AppHost provides the window's appContext to its app through Svelte
// context; an app (a package included, through app-sdk.ts) reads it with
// getLaunchFile() during component setup, or subscribes to launchFileStore()
// when the shell may hand an open window a new file.
import { getContext, setContext } from 'svelte';
import { derived, get, type Readable } from 'svelte/store';
import { apiFileURL } from './media-utils.js';

export type LaunchFile = {
  /** Path under the owner's files root, e.g. "Decks/Q3 review.pptx"; '' for a URL source. */
  path: string;
  name: string;
  mediaType: string;
  /** Same-origin URL that serves the file inline (the session cookie authorizes it). */
  url: string;
  /** A remote source the window was opened on instead of a stored file. */
  sourceUrl: string;
  /** 'choice' when the user picked this app under Open with; 'default' otherwise. */
  openedWith: 'default' | 'choice';
};

const LAUNCH_CONTEXT_KEY = Symbol.for('choir.launch-context');

export function launchFileFromAppContext(appContext: any): LaunchFile | null {
  const ctx = appContext || {};
  const path = String(ctx.filePath || ctx.sourcePath || '').replace(/^\/+/, '');
  const sourceUrl = String(ctx.sourceUrl || '');
  if (!path && !sourceUrl) return null;
  const name = String(ctx.fileName || (path ? path.split('/').pop() : '') || ctx.windowTitle || '');
  return {
    path,
    name,
    mediaType: String(ctx.mediaType || ''),
    url: path ? apiFileURL(path, true) : sourceUrl,
    sourceUrl: path ? '' : sourceUrl,
    openedWith: ctx.openedWith === 'choice' ? 'choice' : 'default',
  };
}

/** AppHost calls this once per window with a store of the window's appContext. */
export function provideLaunchContext(store: Readable<any>) {
  setContext(LAUNCH_CONTEXT_KEY, store);
}

function contextStore(): Readable<any> | null {
  try {
    return (getContext(LAUNCH_CONTEXT_KEY) as Readable<any>) || null;
  } catch (_err) {
    // Called outside component setup: there is no window to read.
    return null;
  }
}

/**
 * The file this window was launched on, or null. Pass the app's appContext
 * prop, or call with no argument during component setup.
 */
export function getLaunchFile(appContext?: any): LaunchFile | null {
  if (appContext !== undefined) return launchFileFromAppContext(appContext);
  const store = contextStore();
  return store ? launchFileFromAppContext(get(store)) : null;
}

/** A store of the launch file that follows the window (call during setup). */
export function launchFileStore(): Readable<LaunchFile | null> {
  const store = contextStore();
  if (!store) return { subscribe: (run) => { run(null); return () => {}; } };
  return derived(store, (ctx) => launchFileFromAppContext(ctx));
}
