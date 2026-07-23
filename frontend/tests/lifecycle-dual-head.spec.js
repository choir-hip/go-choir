import { expect, test } from '@playwright/test';
import { lifecycleCurrentDocumentRevisionID } from '../src/lib/texture.js';

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
