#!/bin/sh
':' //; if [ -n "$BUN" ]; then exec "$BUN" "$0" "$@"; elif [ -x "$HOME/.bun-1.4.2/bin/bun" ]; then exec "$HOME/.bun-1.4.2/bin/bun" "$0" "$@"; else exec bun "$0" "$@"; fi
// AutoradioLab's commands: `bun exact.mjs --help`. They live in the exact checkout pinned by
// mobile/scripts/setup-exact.sh (default ../.exact-src), in its scripts/app-cli.mjs.
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';

const EXACT2 = resolve(import.meta.dir, process.env.EXACT2 ?? "../.exact-src");
const cli = resolve(EXACT2, 'scripts/app-cli.mjs');
if (!existsSync(cli)) { console.error(`no exact at ${EXACT2}: run mobile/scripts/setup-exact.sh, or set EXACT2`); process.exit(1); }
const { ensureBun } = await import(resolve(EXACT2, 'scripts/bun.mjs'));
ensureBun("1.4.2");
const { run } = await import(cli);
process.exit(await run({ dir: import.meta.dir, name: "autoradiolab", exact2: EXACT2, args: process.argv.slice(2) }));
