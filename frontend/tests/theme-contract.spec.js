import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';

const DESIGN_COLOR_LITERAL =
  /#[0-9A-Fa-f]{3,4}(?![0-9A-Za-z_-])|#[0-9A-Fa-f]{6}(?:[0-9A-Fa-f]{2})?(?![0-9A-Za-z_-])|rgba?\([^)]*\)|hsla?\([^)]*\)/g;

const SOURCE_EXTENSIONS = new Set(['.css', '.svelte', '.js', '.ts']);
const THEME_AUTHORITY = path.join('src', 'lib', 'theme.ts');

function walk(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const entryPath = path.join(dir, entry.name);
    return entry.isDirectory() ? walk(entryPath) : [entryPath];
  });
}

function lineNumber(text, index) {
  return text.slice(0, index).split('\n').length;
}

test('theme authority is the only frontend source path with design color literals', () => {
  const frontendRoot = process.cwd();
  const sourceRoot = path.join(frontendRoot, 'src');
  const violations = [];

  for (const filePath of walk(sourceRoot)) {
    const relativePath = path.relative(frontendRoot, filePath);
    if (relativePath === THEME_AUTHORITY) continue;
    if (!SOURCE_EXTENSIONS.has(path.extname(filePath))) continue;

    const text = fs.readFileSync(filePath, 'utf8');
    for (const match of text.matchAll(DESIGN_COLOR_LITERAL)) {
      violations.push(`${relativePath}:${lineNumber(text, match.index)} ${match[0]}`);
    }
  }

  expect(violations, violations.slice(0, 80).join('\n')).toEqual([]);
});
