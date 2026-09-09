import { expect, test } from '@playwright/test';
import {
  assetGraphFromHTML,
  shouldReplaceDocument,
} from '../src/lib/computer-surface-handoff.js';

test('assetGraphFromHTML fingerprints hashed /assets links', () => {
  const host = `<html><script type="module" crossorigin src="/assets/index-CXDFEMio.js"></script>
<link rel="stylesheet" crossorigin href="/assets/index-BOKUt_rj.css"></html>`;
  const guest = `<html><script type="module" crossorigin src="/assets/index-Bi7uNT3r.js"></script>
<link rel="stylesheet" crossorigin href="/assets/index-BOKUt_rj.css"></html>`;
  expect(assetGraphFromHTML(host)).toContain('/assets/index-CXDFEMio.js');
  expect(assetGraphFromHTML(guest)).toContain('/assets/index-Bi7uNT3r.js');
  expect(assetGraphFromHTML(host)).not.toBe(assetGraphFromHTML(guest));
});

test('shouldReplaceDocument hands off once from host shell to computer surface', () => {
  const host = '/assets/index-CXDFEMio.js';
  const guest = '/assets/index-Bi7uNT3r.js';
  expect(shouldReplaceDocument(host, guest, '')).toBe(true);
  expect(shouldReplaceDocument(guest, guest, guest)).toBe(false);
  expect(shouldReplaceDocument(host, guest, guest)).toBe(false);
  expect(shouldReplaceDocument(host, '', '')).toBe(false);
});
