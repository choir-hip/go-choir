import Foundation

// The shared code's one way to the stored sign-in (AutoradioLab/api.ts, `authedJSON`). Native, because only native code can read the
// Keychain item today's sign-in wrote, in place: Exact's own secret store keeps items under another service, so it cannot see it.
// Read only, three accounts only, through the very store that wrote them (KeychainKeyStore), so what one writes the other reads.
// Nothing here logs, caches or returns anything but the stored text. Writing stays with ChoirSignIn until sign-in itself moves.

public struct CredentialReader: Sendable {
    /// The accounts the shared code may read: the bearer key (a secret), the key's id and the listener's computer id.
    public static let accounts: Set<String> = [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID]
    private let store: KeyStore
    private let demo: @Sendable () -> Bool
    // The default is today's store with its own default service: the one the sign-in has always written under. Never move or rename that item.
    // `demo` answers whether demo mode is on: then the real Keychain is never read for shared code (the demo listener has no real key to send),
    // so shared code cannot make a real request in demo mode (review 2026-11-19, finding 1).
    public init(store: KeyStore = KeychainKeyStore(), demo: @escaping @Sendable () -> Bool = { Demo.isOn }) { self.store = store; self.demo = demo }
    /// The stored text, or nil when there is none, the name is not one of the three, or demo mode is on.
    public func read(_ account: String) -> String? {
        guard !demo(), Self.accounts.contains(account), let v = store.get(account), !v.isEmpty else { return nil }
        return v
    }
    /// After a sign-out: is the bearer key still stored? (Demo mode has no real key to remain.) The native sign-out answers `done: false` when it is.
    public func keyRemains() -> Bool { !demo() && !(store.get(KeyAccount.apiKey) ?? "").isEmpty }
    /// The reply the native module gives the shared code. `get` answers `{found, value}` for one account; `session` answers the key and the
    /// computer id in ONE read, `{found, value, computerId}`, so a sign-out between two reads cannot pair an old key with a new computer.
    public func reply(_ request: [String: Any]) -> [String: Any] {
        switch request["op"] as? String {
        case "get":
            guard let v = read(request["account"] as? String ?? "") else { return ["found": false] }
            return ["found": true, "value": v]
        case "session":
            guard let k = read(KeyAccount.apiKey) else { return ["found": false] }
            return ["found": true, "value": k, "computerId": read(KeyAccount.computerID) ?? ""]
        default:
            return ["found": false]
        }
    }
}
