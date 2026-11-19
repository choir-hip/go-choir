import Foundation
import CryptoKit
import Security

// A platform cipher; shared code has none. The phone's copy of the listener's mail (AutoradioLab/mail-store.ts) is sealed here: CryptoKit
// AES-256-GCM, the key made and kept natively in the Keychain, this device only, readable after the first unlock, and NEVER returned to shared
// code. Exact's `crypto.subtle` has digests and ECDSA and refuses AES (Exact reference.md, data-module table), so shared code cannot do this;
// hand-made cipher code does not ship. The key is its own Keychain item (service news.choir.autoradio.mailseal), apart from the sign-in item,
// which this never reads, writes or moves. Android owes the same pair in Kotlin (Keystore AES-GCM).
// Ops (the app module's `later`/`call`): mailSeal {b64} -> {ok, b64}; mailOpen {b64} -> {ok, b64}; mailForget -> {ok}. Nothing here logs.

public struct MailSeal: Sendable {
    public static let service = "news.choir.autoradio.mailseal"
    private let service: String
    public init(service: String = MailSeal.service) { self.service = service }

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
        guard create else { return nil }
        let k = SymmetricKey(size: .bits256)
        var add = query(); add[kSecValueData as String] = k.withUnsafeBytes { Data($0) }
        #if os(iOS)
        add[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        #endif
        return SecItemAdd(add as CFDictionary, nil) == errSecSuccess ? k : nil
    }

    public func seal(_ plain: Data) -> Data? {
        guard let k = key(create: true), let box = try? AES.GCM.seal(plain, using: k) else { return nil }
        return box.combined
    }
    public func open(_ sealed: Data) -> Data? {
        guard let k = key(create: false), let box = try? AES.GCM.SealedBox(combined: sealed) else { return nil }
        return try? AES.GCM.open(box, using: k)
    }
    public func forget() { SecItemDelete(query() as CFDictionary) }

    /// The reply for the native module; nil when the request is not one of ours.
    public func reply(_ request: [String: Any]) -> [String: Any]? {
        switch request["op"] as? String {
        case "mailSeal", "mailOpen":
            guard let text = request["b64"] as? String, let data = Data(base64Encoded: text) else { return ["ok": false] }
            let out = (request["op"] as? String) == "mailSeal" ? seal(data) : open(data)
            guard let out else { return ["ok": false] }
            return ["ok": true, "b64": out.base64EncodedString()]
        case "mailForget": forget(); return ["ok": true]
        default: return nil
        }
    }
}
