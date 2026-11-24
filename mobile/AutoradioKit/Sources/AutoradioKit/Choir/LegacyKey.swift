import Foundation

// The key the old Swift sign-in kept. Until 2026-11-23 the app wrote its bearer key to a Keychain item under service `news.choir.autoradio.choir`,
// accounts `api-key`, `key-id` and `computer-id`. Sign-in is now shared code (Exact's secret store, service = the bundle id, accounts `choir.key` and so on),
// so those old items are never read again. Nothing is carried over (owner, 2026-11-20: the app is unlaunched, "I don't need to save anything"), but a full-access key
// must not outlive the sign-in it belonged to: once, at the first launch that finds the items, the key is revoked on the server (best effort) and the items are deleted.
// With nothing found (every later launch, every new install) this reads three items and does nothing.
public enum LegacyKey {
    public static let service = "news.choir.autoradio.choir"
    public static let accounts = ["api-key", "key-id", "computer-id"]

    /// Revoke the legacy key if there is one, then delete all three legacy items. The items go first (a revoke that hangs or fails must not keep a key on the phone);
    /// the revoke gets the key and its id from what was read. Returns true when anything was found.
    @discardableResult
    public static func retire(store: KeyStore, revoke: (_ keyID: String, _ key: String) async -> Void) async -> Bool {
        let key = store.get("api-key"), id = store.get("key-id")
        let found = accounts.contains { store.get($0) != nil }
        guard found else { return false }
        for a in accounts { store.delete(a) }
        if let key, !key.isEmpty, let id, !id.isEmpty { await revoke(id, key) }
        return true
    }
}
