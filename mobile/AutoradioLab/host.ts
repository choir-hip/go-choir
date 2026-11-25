// Is there a native module on this host? Exact's own fact, `native.available` (pinned prelude.js: "whether a module is linked is the device's fact, not the build's"; door.rs: true when the app links a
// module): read in the answer, with no native call and no timer, so nothing here can be let go. "none" is Android and the web (no app module); "native" is the iPhone. The deck's page set, the shared
// reader and the shared notebook use it: they are the Android path and must do no work on iOS. (This replaced a probe that sent the app module a call under three timers: docs/problems/phone-native-round-trips-and-two-key-readers-2026-11-25.md.)
export type HostMode = 'native' | 'none';
let said = '';
export function hostCaps(native: { available?: boolean } | null | undefined): { mode: HostMode } {
  const mode: HostMode = native && native.available === true ? 'native' : 'none';
  if (mode !== said) { said = mode; console.log(`host: ${mode} (native.available=${native ? String(native.available) : 'no handle'})`); }      // on change only: the phone's log says what the first answer saw
  return { mode };
}
