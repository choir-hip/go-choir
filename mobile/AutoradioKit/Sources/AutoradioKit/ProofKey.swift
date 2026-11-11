import Foundation

// Simulator-only sign-in for the real-data proof (`mobile/scripts/proof-real.sh`). The real sign-in is a browser device flow a script
// cannot approve, so the proof copies a key file named `proof-key` into the app's Documents directory (path from
// `simctl get_app_container <udid> <bundle> data`); at launch the app reads it into the key store and deletes it.
// The host calls this only under `#if targetEnvironment(simulator)`, so it is compiled out of a device build's call path.
//
// WAYS THIS CAN FAIL (written before the code):
//  1. No file: nothing happens (the normal launch).
//  2. An empty or whitespace-only file: deleted, nothing stored.
//  3. A file that is not a `choir_sk_` key (a wrong file copied by mistake): deleted, nothing stored, so a stray file never becomes a key.
//  4. The key store refuses the write: the file is kept (so a retry can work) and the answer is false.
//  5. A new key replaces an old one: the stored computer id and key id belong to the old key and are cleared, so discovery runs again.
//  6. The key must never appear in a message, a log line or a return value: the function returns only a Bool.

public enum ProofKey {
    public static let fileName = "proof-key"

    @discardableResult
    public static func importIfPresent(documents: URL, into keys: KeyStore) -> Bool {
        let file = documents.appendingPathComponent(fileName)
        guard let data = try? Data(contentsOf: file) else { return false }
        let key = (String(data: data, encoding: .utf8) ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        guard key.hasPrefix("choir_sk_"), key.count > 12, !key.contains(where: { $0.isWhitespace }) else {
            try? FileManager.default.removeItem(at: file)
            return false
        }
        do { try keys.set(key, account: KeyAccount.apiKey) } catch { return false }
        keys.delete(KeyAccount.computerID)
        keys.delete(KeyAccount.keyID)
        try? FileManager.default.removeItem(at: file)
        return true
    }
}
