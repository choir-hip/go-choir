// Which native ops each host's module answers, as data (docs/evidence/android-ops-and-controls-2026-11-28.md). A control whose op the host's module does not answer is not offered (spine.contract
// `opAnswered`). Exact has no way to tell a contract that a host left a package op unanswered (`NotSupportedError` becomes the declared default with no mark), and a native report would be a new native name
// (scripts/autoradio-native-ceiling.txt only falls), so the fact is written here and kept equal to the module by tests (android-controls.test.ts).
//
// The platform is `exactPage().platform` (contract-for-agents.md "exactPage().platform"): `ios`, `macos`, `android`, `web` and others on native hosts; `web` for a browser, even on an Android phone; "" before the first answer.
// A platform with no row here answers everything: that is the iPhone (every interface name has a Swift handler), the Mac, the web host (where these controls were offered before this table and still are) and any platform
// that does not exist yet ("visionos" answers all, as today).
// The platform not yet known ("") is its own case, and the safe one: it answers NOTHING native, so no guarded control is drawn and a typed question is kept in the field until the host says what it is. The one accepted
// difference on iPhone and web: the guarded controls appear with the host's first answer, not the first frame (contract-for-agents.md:6545: `exactPage().platform` "identifies the host from the first frame").

/** The names Kotlin's `AutoradioModule.later` answers: exactly its `when` cases (the deck's five sends are cases of that `when`, handled by DeckAndroid). Add a name here in the commit that adds its case. */
export const ANDROID_ANSWERS: readonly string[] = [
  'readerOpenExternal',
  'nativeModelFacts', 'nativeModelEnsure', 'nativeModelRemove', 'nativeModelProof',
  'deckState', 'deckPlay', 'deckEnqueue', 'deckAct', 'deckAck', 'deckWipe',
];

const TABLE: Record<string, readonly string[]> = { android: ANDROID_ANSWERS };

/** `known: false` while the platform is not known (nothing is answered); `all: true` for a host that answers every op; otherwise the list of the ones it answers. */
export function nativeOps(platform: string): { known: boolean; all: boolean; ops: string[] } {
  if (platform === '') return { known: false, all: false, ops: [] };
  const row = TABLE[platform];
  return row ? { known: true, all: false, ops: [...row] } : { known: true, all: true, ops: [] };
}
