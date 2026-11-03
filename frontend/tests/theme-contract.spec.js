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

function violationsIn(root) {
  const frontendRoot = process.cwd();
  const violations = [];
  for (const filePath of walk(root)) {
    const relativePath = path.relative(frontendRoot, filePath);
    if (relativePath === THEME_AUTHORITY) continue;
    if (!SOURCE_EXTENSIONS.has(path.extname(filePath))) continue;

    const text = fs.readFileSync(filePath, 'utf8');
    for (const match of text.matchAll(DESIGN_COLOR_LITERAL)) {
      violations.push(`${relativePath}:${lineNumber(text, match.index)} ${match[0]}`);
    }
  }
  return violations;
}

test('theme authority is the only frontend source path with design color literals', () => {
  const violations = violationsIn(path.join(process.cwd(), 'src'));
  expect(violations, violations.slice(0, 80).join('\n')).toEqual([]);
});

// One test per first-party package, so a package round runs only its own check:
//   playwright test tests/theme-contract.spec.js --grep "package files-next has"
// (a package added by the round has its directory by the time the spec is
// collected). The whole-source test above stays the complete gate.
const PACKAGES_DIR = path.join(process.cwd(), 'src', 'packages');
const packageNames = fs.existsSync(PACKAGES_DIR)
  ? fs.readdirSync(PACKAGES_DIR, { withFileTypes: true }).filter((e) => e.isDirectory()).map((e) => e.name).sort()
  : [];
for (const name of packageNames) {
  test(`theme authority: package ${name} has no design color literals`, () => {
    const violations = violationsIn(path.join(PACKAGES_DIR, name));
    expect(violations, violations.slice(0, 80).join('\n')).toEqual([]);
  });
}
