import { expect, test } from '@playwright/test';

import { closeSources, loadSource } from './helpers/source-modules.js';

let lifecycleCurrentDocumentRevisionID;
test.beforeAll(async () => {
  ({ lifecycleCurrentDocumentRevisionID } = await loadSource('/src/packages/texture/texture.js'));
});
test.afterAll(closeSources);


test('lifecycle document editing prefers the current document head over the terminal artifact head', () => {
  expect(lifecycleCurrentDocumentRevisionID({
    current_document_head: { revision_id: 'revision-unbound-later' },
    document: { current_revision_id: 'revision-document-fallback' },
    head_revision: { revision_id: 'revision-terminal-pinned' },
  })).toBe('revision-unbound-later');

  expect(lifecycleCurrentDocumentRevisionID({
    document: { current_revision_id: 'revision-document-fallback' },
    head_revision: { revision_id: 'revision-terminal-pinned' },
  })).toBe('revision-document-fallback');

  expect(lifecycleCurrentDocumentRevisionID({
    head_revision: { revision_id: 'revision-terminal-pinned' },
  })).toBe('revision-terminal-pinned');
});
