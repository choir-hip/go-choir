import Foundation
// NATIVE'S PART OF THE ACCOUNT BOUNDARY (docs/design/account-boundary-2026-11-21.md, "Round 12"). The sign-in key is Exact's (`secret.keep`), and the launch verdict is shared code's (AutoradioLab/account.ts): native holds no verdict and decides nothing.
// What is left, each with the reason only native can meet it: `forgetAll` (DELETION ONLY: these files are the Swift stores' own folders under Application Support, which Exact's `app:/` roots do not reach; shared code decides and writes the mark), `retireOldKey` (the one-shot
// read of the old Keychain item that earlier builds wrote under their own service, which Exact's store cannot see), and `HeldKeys` (below: an in-memory copy of the key for the Swift clients that still call the server).
public enum AccountWipe {
    nonisolated(unsafe) static var forgetMail: () -> OSStatus = { MailSeal().forget() }
    /// `Application Support/Autoradio` (a test gives its own; under XCTest with none given it is a temp folder, never the real one); `extraPaths`: the app target's stores not in the folder (removed and listed too).
    nonisolated(unsafe) public static var baseOverride: URL?; nonisolated(unsafe) public static var extraPaths: [URL] = []
    public static var base: URL {
        if let b = baseOverride { return b }; if NSClassFromString("XCTestCase") != nil { return FileManager.default.temporaryDirectory.appendingPathComponent("autoradio-xctest-base", isDirectory: true) }
        return FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio", isDirectory: true)
    }
    public static var root: URL { base.appendingPathComponent("account", isDirectory: true) }
    /// The ONLY place a store's account file is named (a scan fails a store that builds its own). THE ONE CARRY-OVER, voice: the speaker profile and voice choice are MOVED once from the old path (`Application Support/MikeVoice`) before the store first reads
    /// (an enrolment costs minutes of speaking and is not saved in the cloud); only while a key is held. With none held the old file stays IN USE and is never deleted here (the key arrives from shared code after the store may have asked; `forgetAll` removes it at a sign-out).
    public static func url(_ rel: String) -> URL {
        let u = root.appendingPathComponent(rel), fm = FileManager.default
        try? fm.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let old = ["MikeVoice/speaker.json": "../MikeVoice/speaker.json", "voice-choice.json": "voice-choice.json"][rel].map({ base.appendingPathComponent($0) }), !fm.fileExists(atPath: u.path), fm.fileExists(atPath: old.path) {
            if HeldKeys.present { if (try? fm.moveItem(at: old, to: u)) == nil { return old } } else { return old } }
        return u
    }
    private static var oldPaths: [URL] { ["takes-own.json", "takes-muted.json", "takes-copy-own.json", "takes-copy-muted.json", "notebook-cache.md", "notebook-topics-written", "edition-topics.json", "deck-journal.json", "deck-journal.json.seq", "deck-journal.json.bad", "local-entries.json", "texture-pending.json", "turns", "voice-choice.json", "../MikeVoice", "../../Caches/autoradio-clips"].map { base.appendingPathComponent($0) } }
    /// THE WIRING, the one function the module and a test call: every path removed and listed besides the account folder: the support files, files cache, take scratch, `picked` and the shared account roots from Exact's own context.
    public static func configure(support: URL, data: URL, cache: URL, temporary: URL, filesCache: URL, takesScratch: URL) {
        extraPaths = ["onboarding.json", "voice-setup.json", "recent-files.json", "books", "content", "console.log", "console.log.1"].map { support.appendingPathComponent($0) } + [filesCache, takesScratch, temporary.appendingPathComponent("picked")] + [data, cache, temporary].map { $0.appendingPathComponent("accounts") }
    }
    /// DELETION ONLY: remove the account folder, the old per-account stores and the app target's files, forget the mail seal, then list them. True only when none is left and the seal's status is clean (success or item-not-found).
    public static func forgetAll() -> Bool {
        let fm = FileManager.default, all = [root] + oldPaths + extraPaths
        for p in all { try? fm.removeItem(at: p) }
        let mail = forgetMail()
        return [errSecSuccess, errSecItemNotFound].contains(mail) && !all.contains { fm.fileExists(atPath: $0.path) }
    }
    /// Revokes the old key on the server, bounded (6 s: the phone may be offline); blocks, because it runs once at launch off the main thread.
    nonisolated(unsafe) static var revoke: (String, String) -> Bool = { id, key in
        nonisolated(unsafe) var ok = false; let sem = DispatchSemaphore(value: 0)
        Task.detached {
            let held = ChoirClient(baseURL: ChoirEnvironment.shared.configuredBaseURL, keys: InMemoryKeyStore([KeyAccount.apiKey: key]), session: ChoirEnvironment.shared.session, log: nil, trustedOrigin: URL(string: ChoirEnvironment.shared.configuredBaseURL))
            do { try await held.send("DELETE", "/auth/api-keys/\(PathSegment.encode(id))", route: "/auth/api-keys/{id}", timeout: 6); ok = true } catch ChoirError.unauthorized { ok = true } catch ChoirError.server(let s, _) where s == 404 { ok = true } catch { ok = false }
            sem.signal()
        }
        return sem.wait(timeout: .now() + 8) == .success && ok
    }
    nonisolated(unsafe) static var oldKeys: KeyStore = KeychainKeyStore()
    /// ONE-SHOT, every launch until it finds nothing: the old key (the Keychain items earlier builds wrote under service `news.choir.autoradio.choir`: key, key id, computer id) is read ONCE, revoked on the server, and the three items are deleted; a revoke that fails
    /// still deletes. Answers a state word for the log, never the key: `none`, `revoked`, `not revoked`. DELETE THIS FUNCTION (and `oldKeys`) when no phone that ran a build before the key moved to Exact's store is left: it then always answers `none`.
    @discardableResult public static func retireOldKey() -> String {
        guard let key = oldKeys.get(KeyAccount.apiKey), !key.isEmpty else { for a in [KeyAccount.keyID, KeyAccount.computerID] { oldKeys.delete(a) }; return "none" }
        let revoked = oldKeys.get(KeyAccount.keyID).map { revoke($0, key) } ?? false
        for a in [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID] { oldKeys.delete(a) }
        return revoked ? "revoked" : "not revoked"
    }
}

/// THE KEY IN MEMORY, for the Swift clients that still call the server with it (`ChoirClient`: the Choir page's player and stories, the programme, the desks, takes, the notebook, the Texture editor, podcasts). Shared code hands it (`holdKey`) after the launch opens the account and clears it
/// at a sign-out BEFORE the ending mark is written; native never persists, reads or returns it. REASON: those clients are pages still in Swift (docs/problems/autoradio-app-logic-is-swift-not-exact-2026-11-18.md); moving them to shared code is station 2 of the account boundary. DELETE THIS TYPE with
/// the last of them (a test fails if a new file starts using `ChoirClient`, so this can only shrink).
public enum HeldKeys {
    public static let store = InMemoryKeyStore()
    public static var present: Bool { !(store.get(KeyAccount.apiKey) ?? "").isEmpty }
    public static func hold(key: String, keyId: String, computerId: String) {
        if key.isEmpty { for a in [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID] { store.delete(a) }; return }
        try? store.set(key, account: KeyAccount.apiKey); if !keyId.isEmpty { try? store.set(keyId, account: KeyAccount.keyID) }; if !computerId.isEmpty { try? store.set(computerId, account: KeyAccount.computerID) }
    }
}
