// bun test preload: write build-info.gen.ts (scripts/gen-build-info.sh) before any test imports app.ts.
import { fileURLToPath } from 'node:url';
Bun.spawnSync(['sh', fileURLToPath(new URL('../scripts/gen-build-info.sh', import.meta.url))]);
