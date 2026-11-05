// Open with, as a contribution on file targets (slice S4). A file row in an app
// passes `target: { kind: 'file', name, data: { openWith } }` to its
// contextMenu; the shell adds "Open with" after the item with id "open",
// listing every app that opens the file. The app no longer builds the submenu
// by hand. `data.openWith(opener)` is the app's own callback (it knows the
// file's folder), so it never crosses the contextMenu.build pipeline.
import { shellKernel } from '../shell-kernel';
import { openersForFile, type FileOpener } from '../open-with';
import { registerMenuContributor } from './context-menu';

registerMenuContributor(shellKernel.root, {
  id: 'core.open-with',
  kinds: ['file'],
  contribute(target) {
    const openWith = target.data?.openWith as ((opener: FileOpener) => void) | undefined;
    if (typeof openWith !== 'function' || !target.name) return null;
    const openers = openersForFile(target.name);
    if (!openers.length) return null;
    return {
      after: 'open',
      entries: [{
        id: 'open-with',
        label: 'Open with',
        items: openers.map((opener) => ({
          id: `open-with:${opener.appId}`,
          label: opener.isDefault ? `${opener.name} (default)` : opener.name,
          icon: opener.icon,
          onSelect: () => openWith(opener),
        })),
      }],
    };
  },
});
