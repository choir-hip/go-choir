// `bun test` runs shared code without Exact's runtime, so the packages Exact supplies are stubbed here by name: the real `@exact/passkeys` is an interface (an exact.json) the bake
// links to the platform's passkey screen, which no test can open. A test that needs a passkey result passes its own through `CeremonyEnv.passkeys` (passkey-ceremony.ts).
import { plugin } from 'bun';

plugin({
  name: 'exact-package-stubs',
  setup(build) {
    build.module('@exact/passkeys', () => ({
      exports: {
        credentials: {
          create: async () => { throw Object.assign(new Error('no passkey screen under bun test'), { name: 'NotSupportedError' }); },
          get: async () => { throw Object.assign(new Error('no passkey screen under bun test'), { name: 'NotSupportedError' }); },
        },
      },
      loader: 'object',
    }));
  },
});
