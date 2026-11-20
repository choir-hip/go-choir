// The app's native module. It exists so that the app target compiles Swift of its own: Exact links
// `modules/apple/*.xcframework` (static third-party libraries) into this module, and the model stages
// (Models_*.swift, copied in from AutoradioKit/Sources/AutoradioModels by scripts/sync-exact-package.sh) import them.
// The kit (package module ExactPkg_autoradio) finds AutoradioModelsRegistrar by name; see ExternalModels.swift there.
import Foundation
import ExactPkg_autoradio
import Security

// The one native call the signed-in seam (api.ts) makes, with the reason only native code can meet: it reads the Keychain item today's
// sign-in wrote (service news.choir.autoradio.choir), in place. Exact's own secret store keeps items under this app's id, so shared code
// cannot see that item; the owner's phone stays signed in because nothing is moved. Read only: `{op: "get", account}` for the three
// accounts CredentialReader allows, answering `{found, value}`. It logs nothing. It goes when sign-in itself is moved and the key is
// kept by Exact's store.
//
// The mail cipher (Choir/MailSeal.swift): `mailSeal`, `mailOpen`, `mailForget`, a platform cipher shared code has none of; the key never leaves the Keychain.
//
// The edition picks' calls (`editionLegacy`, `editionLegacyDrop`, `editionMirror`, `editionHeard`, `editionSaid`): the old Swift file (Application Support, which shared code cannot see), the voice's in-memory copy of the picks, and the words the voice heard at the setup screen (docs/reference/AUTORADIO-EDITION-PICKS.md).
//
// The second native call, `{op: "signOut"}` (later only), answers `{done: Bool}` once the native half of sign-out has finished. Sign-out is led
// by shared code (AutoradioLab/account.ts); the reason it is native is that the key is a Keychain item the Swift sign-in wrote and the Swift stores
// (AccountWipe: the voice's and the pages not yet moved) are wiped by Swift. It goes with them when sign-in itself moves.
// The sign-out reconcile (AutoradioLab/account.ts) needs the stored key as THREE answers, which the reader's `nil` cannot give: `keystate` -> present (the item is
// there), absent (the Keychain answered item-not-found) or unreadable (locked before the first unlock, or any other status). Read only: no data is returned, nothing
// is written, moved or re-keyed. `wipe` is the native half of an ACCOUNT CHANGE: the old account's Swift stores and mail key go, the stored key stays.
//
// The themes' one-time carry-over (AutoradioLab/themes.ts `migrate`): `{op: "themeLegacy"}` READS what the Swift app kept for the themes: the choice (UserDefaults) and the user's theme files
// (Application Support/Autoradio/themes). The reason it is native: those are places shared code cannot see (Exact's `app:/data` is another folder). It only reads and removes nothing, so a reverted
// build still finds its old copy; a later change removes the old copy when phones have moved, and this op goes with it. At most 64 files of at most 64 KiB are sent; the rest are named in `skipped`
// (shared code shows each as a problem under its own name). Nothing else about the themes is native.
final class ModelsModule: ExactModule {
    static var legacyThemeDir: URL { FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio/themes") }
    static func legacyThemes() -> [String: Any] {
        var files: [String: String] = [:], skipped: [String] = []
        for n in ((try? FileManager.default.contentsOfDirectory(atPath: legacyThemeDir.path)) ?? []).sorted() where n.hasSuffix(".json") {
            if files.count < 64, let d = try? Data(contentsOf: legacyThemeDir.appendingPathComponent(n)), d.count <= 65536, let t = String(data: d, encoding: .utf8) { files[n] = t } else { skipped.append(n) }
        }
        let theme = UserDefaults.standard.string(forKey: "AutoradioThemeID") ?? "", mode = UserDefaults.standard.string(forKey: "AutoradioThemeMode") ?? ""
        return ["found": !theme.isEmpty || !mode.isEmpty || !files.isEmpty || !skipped.isEmpty, "theme": theme, "mode": mode, "files": files, "skipped": skipped]
    }
    static func keyState() -> String {
        let q: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: "news.choir.autoradio.choir", kSecAttrAccount as String: KeyAccount.apiKey, kSecMatchLimit as String: kSecMatchLimitOne]
        switch SecItemCopyMatching(q as CFDictionary, nil) { case errSecSuccess: return "present"; case errSecItemNotFound: return "absent"; default: return "unreadable" }
    }
    override func call(_ request: [String: Any]) -> [String: Any] {
        #if targetEnvironment(simulator)
        // The real-data proof only (compiled out of device builds): the `proof-key` file is imported into the Keychain by the package module's init, which can run AFTER the first
        // shared-code read on a fresh install (the first real drive after an install showed Settings, Files and Mail signed out for the whole drive, the answer being kept). Importing
        // here, before any read of the key, removes that race; the file is deleted by the import, so every later call finds none.
        if let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first { ProofKey.importIfPresent(documents: docs, into: KeychainKeyStore()) }
        #endif
        if (request["op"] as? String) == "keystate" { return ["state": Self.keyState()] }
        if (request["op"] as? String) == "themeLegacy" { return Self.legacyThemes() }
        return MailSeal().reply(request) ?? CredentialReader().reply(request)
    }
    override func later(_ request: [String: Any], reply: ExactReply) {
        let op = request["op"] as? String
        if op == "signOut" || op == "wipe" {
            Task { @MainActor in
                guard let m = AutoradioModule.current else { reply.send(["done": false]); return }
                await m.signOutForShared(keepKey: op == "wipe")
                MailSeal().forget()      // the mail copy's key goes with the account, whatever shared code does next
                // Truthful: the Keychain delete ignores its status (ChoirClient.swift), so read the item again. Still there: not done. The Account screen's sign-out then wipes
                // nothing shared (the key remains: the marker goes); at a launch (account.ts reconcile) the shared stores of the account being left are wiped regardless of this answer and the marker stays. Idempotent: with the key already gone it only runs the Swift wipe again.
                reply.send(["done": op == "wipe" || !CredentialReader().keyRemains()])
            }
            return
        }
        // The edition picks' native calls (EditionSetup.swift `EditionNative`), each with the reason only native code can meet: the old file in Application Support, the voice's copy of the picks, the words the voice heard.
        if let op = op, op.hasPrefix("edition") { reply.send(EditionNative.op(request)); return }
        reply.send(call(request))
    }
}
let exactModule: ExactModule.Type = ModelsModule.self
