// pickFile: ask the owner for a Files path in the shell modal (FilePicker.svelte
// over lib/modal.ts). Resolves with the path, or null when the owner cancelled
// (Cancel, Esc, backdrop) or no desktop is mounted. Nothing is changed by
// opening or cancelling it.
import FilePicker from './FilePicker.svelte';
import { openModal } from './modal';

export type PickFileOptions = {
  title?: string;
  confirmLabel?: string;
  /** Lower-case extensions that may be chosen; empty means any file. */
  extensions?: string[];
  /** The filter's name for those extensions ("Images"). */
  filterLabel?: string;
  /** Folder names at the Files root that are never listed. */
  hideAtRoot?: string[];
  /** Folder (or the file) to start at, as a Files path. */
  startPath?: string;
};

export async function pickFile(options: PickFileOptions = {}): Promise<string | null> {
  const startPath = String(options.startPath || '').split('/').filter(Boolean);
  // A file path starts in its folder.
  if (startPath.length && /\.[a-z0-9]{1,8}$/i.test(startPath[startPath.length - 1])) startPath.pop();
  const result = await openModal<string>(
    FilePicker as never,
    {
      title: options.title || 'Choose a file',
      confirmLabel: options.confirmLabel || 'Choose',
      extensions: options.extensions || [],
      filterLabel: options.filterLabel || 'Matching files',
      hideAtRoot: options.hideAtRoot || [],
      startPath: startPath.join('/'),
    },
    { label: options.title || 'Choose a file' },
  );
  return typeof result === 'string' && result ? result : null;
}
