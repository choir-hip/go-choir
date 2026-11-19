import Foundation

// The shared code's one way to the stored sign-in (AutoradioLab/api.ts, `authedJSON`). Native, because only native code can read the
// Keychain item today's sign-in wrote, in place: Exact's own secret store keeps items under another service, so it cannot see it.
// Read only, three accounts only, through the very store that wrote them (KeychainKeyStore), so what one writes the other reads.
// Nothing here logs, caches or returns anything but the stored text. Writing stays with ChoirSignIn until sign-in itself moves.

public struct CredentialReader: Sendable {
    /// The accounts the shared code may read: the bearer key (a secret), the key's id and the listener's computer id.
    public static let accounts: Set<String> = [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID]
    private let store: KeyStore
    public init(store: KeyStore = KeychainKeyStore()) { self.store = store }
    /// The stored text, or nil when there is none or the name is not one of the three.
    public func read(_ account: String) -> String? {
        guard Self.accounts.contains(account), let v = store.get(account), !v.isEmpty else { return nil }
        return v
    }
    /// The reply the native module gives the shared code: `{found, value}`.
    public func reply(_ request: [String: Any]) -> [String: Any] {
        guard request["op"] as? String == "get", let v = read(request["account"] as? String ?? "") else { return ["found": false] }
        return ["found": true, "value": v]
    }
}
