// `bun test` runs shared code without Exact's runtime, so the packages Exact supplies are stubbed here by name: the real `@exact/passkeys` is an interface (an exact.json) the bake
// links to the platform's passkey screen, which no test can open. A test that needs a passkey result passes its own through `CeremonyEnv.passkeys` (passkey-ceremony.ts).
// `@choir/autoradio` is the generated package binding: its exact.json declares names, and Exact's bake writes the functions. Tests install one package hook; without it every name refuses.
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
    build.module('@choir/autoradio', () => ({
      exports: {
        deckState: (options: Record<string, unknown> = {}) => packageCall('deckState', options),
        deckPlay: (options: Record<string, unknown>) => packageCall('deckPlay', options),
        deckEnqueue: (options: Record<string, unknown>) => packageCall('deckEnqueue', options),
        deckAct: (options: Record<string, unknown>) => packageCall('deckAct', options),
        deckAck: (options: Record<string, unknown>) => packageCall('deckAck', options),
        deckWipe: () => packageCall('deckWipe', {}),
      },
      loader: 'object',
    }));
  },
});

type DeckPackageHook = (name: string, args: Record<string, unknown>) => Promise<unknown>;
async function packageCall(name: string, args: Record<string, unknown>): Promise<unknown> {
  const hook = (globalThis as typeof globalThis & { __autoradioDeckPackage?: DeckPackageHook }).__autoradioDeckPackage;
  if (hook) return hook(name, args);
  // Tests written against the old app-module bus adopt a `native` whose `later` answers the
  // `deck` op: the package names fall back to it, so those tests keep testing the same port.
  const { adoptedNative } = await import('./api.ts');            // after the plugin is registered, never before
  const native = adoptedNative();
  if (native?.later) {
    // The app-module bus flattened an item into the op's args; the package nests it under `item`.
    const item = (args.item ?? {}) as Record<string, unknown>;
    const flat = name === 'deckPlay' ? { ...item } : name === 'deckEnqueue' ? { ...item, at: args.at } : args;
    return native.later({ op: 'deck', name, args: flat });
  }
  throw Object.assign(new Error(`no ${name} package under bun test`), { name: 'NotSupportedError' });
}
