import Foundation
import CryptoKit
import Security

// A platform cipher; shared code has none. The phone's copy of the listener's mail (AutoradioLab/mail-store.ts) is sealed here: CryptoKit
// AES-256-GCM, the key made and kept natively in the Keychain, this device only, readable after the first unlock, and NEVER returned to shared
// code. Exact's `crypto.subtle` has digests and ECDSA and refuses AES (Exact reference.md, data-module table), so shared code cannot do this;
// hand-made cipher code does not ship. The key is its own Keychain item (service news.choir.autoradio.mailseal), apart from the sign-in item,
// which this never reads, writes or moves. Android owes the same pair in Kotlin (Keystore AES-GCM).
// Ops (the app module's `later`/`call`): mailSeal {b64, aad} -> {ok, b64}; mailOpen {b64, aad} -> {ok, b64} or {ok: false, why: "bad" | "cannot"};
// mailForget -> {ok}; mailCan -> {ok} (this platform has the pair). `aad` is the record's identity (kind and id), authenticated with the data: a file
// swapped to another message, folder or account fails to open. `why` "bad": the data is not ours or was changed (the tag failed): it may be deleted.
// "cannot": no key could be read now (the Keychain is locked before the first unlock, or the key is gone): the file is KEPT. Nothing here logs.
// A key is made only while a sign-in key is stored: after a sign-out has deleted it, a late seal cannot make a new key for a copy nobody will wipe.

public struct MailSeal: Sendable {
    public static let service = "news.choir.autoradio.mailseal"
    private let service: String
    private let signedIn: @Sendable () -> Bool
    public init(service: String = MailSeal.service, signedIn: @escaping @Sendable () -> Bool = { CredentialReader().keyRemains() }) { self.service = service; self.signedIn = signedIn }

    private func query() -> [String: Any] { [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: "key"] }

    private func stored() -> SymmetricKey? {
        var q = query(); q[kSecReturnData as String] = true; q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: AnyObject?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let d = out as? Data, d.count == 32 else { return nil }
        return SymmetricKey(data: d)
    }

    /// The key, made on first use; nil when the Keychain refuses (then nothing is sealed and shared code writes no file).
    private func key(create: Bool) -> SymmetricKey? {
        if let k = stored() { return k }
        guard create, signedIn() else { return nil }
        let k = SymmetricKey(size: .bits256)
        var add = query(); add[kSecValueData as String] = k.withUnsafeBytes { Data($0) }
        #if os(iOS)
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        #endif
        let status = SecItemAdd(add as CFDictionary, nil)
        if status == errSecSuccess { return k }
        // Two first uses at once: the other one's key won. Take it (seal and open must use the one key), never fail the write for it.
        return status == errSecDuplicateItem ? stored() : nil
    }

    public func seal(_ plain: Data, aad: Data = Data()) -> Data? {
        guard let k = key(create: true), let box = try? AES.GCM.seal(plain, using: k, authenticating: aad) else { return nil }
        return box.combined
    }
    public enum Opened: Equatable { case plain(Data), bad, cannot }
    public func openResult(_ sealed: Data, aad: Data = Data()) -> Opened {
        guard let k = key(create: false) else { return .cannot }
        guard let box = try? AES.GCM.SealedBox(combined: sealed), let plain = try? AES.GCM.open(box, using: k, authenticating: aad) else { return .bad }
        return .plain(plain)
    }
    public func open(_ sealed: Data, aad: Data = Data()) -> Data? { if case .plain(let d) = openResult(sealed, aad: aad) { return d }; return nil }
    public func forget() { SecItemDelete(query() as CFDictionary) }

    /// The reply for the native module; nil when the request is not one of ours.
    public func reply(_ request: [String: Any]) -> [String: Any]? {
        let aad = Data((request["aad"] as? String ?? "").utf8)
        switch request["op"] as? String {
        case "mailSeal":
            guard let text = request["b64"] as? String, let data = Data(base64Encoded: text), let out = seal(data, aad: aad) else { return ["ok": false] }
            return ["ok": true, "b64": out.base64EncodedString()]
        case "mailOpen":
            guard let text = request["b64"] as? String, let data = Data(base64Encoded: text) else { return ["ok": false, "why": "bad"] }
            switch openResult(data, aad: aad) {
            case .plain(let d): return ["ok": true, "b64": d.base64EncodedString()]
            case .bad: return ["ok": false, "why": "bad"]
            case .cannot: return ["ok": false, "why": "cannot"]
            }
        case "mailForget": forget(); return ["ok": true]
        case "mailCan": return ["ok": true]
        default: return nil
        }
    }
}
