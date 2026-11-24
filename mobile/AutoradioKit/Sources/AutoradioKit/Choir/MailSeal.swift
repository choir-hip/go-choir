import Foundation
import CryptoKit
import Security

// A platform cipher; shared code has none. The phone's copy of the listener's mail (AutoradioLab/mail-store.ts) is sealed here: CryptoKit
// AES-256-GCM, the key made and kept natively in the Keychain, this device only, readable after the first unlock, and NEVER returned to shared
// code. Exact's `crypto.subtle` has digests and ECDSA and refuses AES (Exact reference.md, data-module table), so shared code cannot do this;
// hand-made cipher code does not ship. The key is its own Keychain item (service news.choir.autoradio.mailseal), apart from the sign-in item,
// which this never reads, writes or moves. Android owes the same pair in Kotlin (Keystore AES-GCM).
// Ops (app module `later`/`call`): mailSeal {b64, aad} -> {ok, b64}; mailOpen {b64, aad} -> {ok, b64} | {ok: false, why: "bad" | "cannot"}; mailForget; mailCan.
// `aad` (kind and id) is authenticated with the data. "bad": the tag failed, the file may go. "cannot": no key could be read now, the file is KEPT.
// A key is made only while the sign-in key is stored (a late seal after sign-out makes none). Nothing here logs.

public struct MailSeal: Sendable {
    public static let service = "news.choir.autoradio.mailseal"
    private let service: String
    private let signedIn: @Sendable () -> Bool
    public init(service: String = MailSeal.service, signedIn: @escaping @Sendable () -> Bool = { KeychainKeyStore.keyRemains() }) { self.service = service; self.signedIn = signedIn }

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
            let r = openResult(data, aad: aad)
            if case .plain(let d) = r { return ["ok": true, "b64": d.base64EncodedString()] }
            return ["ok": false, "why": r == .bad ? "bad" : "cannot"]
        case "mailForget": forget(); return ["ok": true]
        case "mailCan": return ["ok": true]
        default: return nil
        }
    }
}
