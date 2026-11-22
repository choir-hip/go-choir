import XCTest
@testable import AutoradioKit

// NATIVE'S PART OF THE ACCOUNT BOUNDARY (docs/design/account-boundary-2026-11-21.md, "Round 12"): the key is Exact's and the verdict is shared code's, so what is tested here is what native still does. Failure list, written first:
//  1. `forgetAll` removes the account folder, the old stores, the app target's files and the shared roots, forgets the mail seal, and answers true only when none is left and the seal's status is clean
//  2. `retireOldKey`: the old Keychain items are read once, the key is revoked, the three items are deleted; a revoke that fails still deletes; nothing there is `none`
//  3. `HeldKeys` is the in-memory key: held and cleared by `hold`; the mail seal makes a key only while one is held; native persists nothing
//  4. the voice carry-over moves the old file only while a key is held, and never deletes an old file when none is held
//  5. no store builds its own account path; no test names a real folder; a new native file may not start calling the server with `ChoirClient` (the residual only shrinks)
final class AccountWipeTests: KeychainFakeCase {
    private let fm = FileManager.default
    private func plant(_ rel: String) throws { let u = AccountWipe.base.appendingPathComponent(rel); try fm.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true); try Data("old account".utf8).write(to: u) }
    private func plantAll() throws {
        try plant("account/takes-copy-own.json"); try plant("account/MikeVoice/speaker.json"); try plant("account/turns/t1/record.json")
        for o in ["takes-own.json", "deck-journal.json", "notebook-cache.md", "turns/t1/record.json", "voice-choice.json", "../MikeVoice/speaker.json"] { try plant(o) }
        AccountWipe.extraPaths = ["books", "content", "console.log", "recent-files.json"].map { AccountWipe.base.appendingPathComponent($0) }
        for e in ["books/b1/text.txt", "content/c.json", "console.log", "recent-files.json"] { try plant(e) }
    }
    private func leftovers() -> [String] {
        (["account", "takes-own.json", "deck-journal.json", "notebook-cache.md", "turns", "voice-choice.json", "../MikeVoice", "books", "content", "console.log", "recent-files.json"]).filter { fm.fileExists(atPath: AccountWipe.base.appendingPathComponent($0).path) }
    }

    func testForgetAllRemovesEveryNativePathAndIsTrueOnlyWhenNoneIsLeft() throws {
        try plantAll(); XCTAssertEqual(leftovers().count, 11)
        XCTAssertTrue(AccountWipe.forgetAll()); XCTAssertEqual(leftovers(), []); XCTAssertTrue(AccountWipe.forgetAll(), "idempotent: nothing to remove is clean")
    }
    func testForgetAllIsFalseWhenAPathStaysOrTheSealSaysNo() throws {
        try plantAll(); let stuck = AccountWipe.root.appendingPathComponent("turns/t1"); try fm.createDirectory(at: stuck, withIntermediateDirectories: true); try Data("x".utf8).write(to: stuck.appendingPathComponent("f"))
        try fm.setAttributes([.posixPermissions: 0o500], ofItemAtPath: stuck.path); defer { try? fm.setAttributes([.posixPermissions: 0o700], ofItemAtPath: stuck.path) }
        XCTAssertFalse(AccountWipe.forgetAll(), "a path that stays is not done")
        try fm.setAttributes([.posixPermissions: 0o700], ofItemAtPath: stuck.path); XCTAssertTrue(AccountWipe.forgetAll())
        for (status, expect) in [(errSecSuccess, true), (errSecItemNotFound, true), (errSecInteractionNotAllowed, false)] as [(OSStatus, Bool)] { AccountWipe.forgetMail = { status }; XCTAssertEqual(AccountWipe.forgetAll(), expect, "status \(status)") }
    }
    /// THE PRODUCTION WIRING, built by the very function the module calls: the shared account roots under Exact's data, cache and temporary folders are removed too.
    func testTheWiringTheModuleUsesRemovesTheSharedRootsToo() throws {
        let app = AccountWipe.base.appendingPathComponent("../exact/app1")
        let data = app.appendingPathComponent("data"), cache = app.appendingPathComponent("cache"), tmp = app.appendingPathComponent("tmp"), support = AccountWipe.base, files = cache.appendingPathComponent("files"), scratch = tmp.appendingPathComponent("takes")
        AccountWipe.configure(support: support, data: data, cache: cache, temporary: tmp, filesCache: files, takesScratch: scratch)
        let planted = [data, cache, tmp].map { $0.appendingPathComponent("accounts/abc") } + [tmp.appendingPathComponent("picked/doc.pdf"), files.appendingPathComponent("h/f"), scratch.appendingPathComponent("rec.m4a"), support.appendingPathComponent("content/c.json")].map { $0.appendingPathComponent("") }
        for u in planted { try fm.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true); try Data("old".utf8).write(to: u.deletingLastPathComponent().appendingPathComponent("z.bin")) }
        XCTAssertTrue(AccountWipe.forgetAll()); XCTAssertFalse(planted.contains { fm.fileExists(atPath: $0.deletingLastPathComponent().path) }, "every production path is gone")
    }

    func testTheOldKeyIsReadOnceRevokedAndTheThreeItemsAreDeleted() throws {
        let k = KeychainKeyStore(); try k.set("old-key-not-real", account: KeyAccount.apiKey); try k.set("old-id", account: KeyAccount.keyID); try k.set("comp", account: KeyAccount.computerID)
        nonisolated(unsafe) var seen: [String] = []; AccountWipe.revoke = { id, key in seen.append("\(id):\(key == "old-key-not-real")"); return true }
        XCTAssertEqual(AccountWipe.retireOldKey(), "revoked"); XCTAssertEqual(seen, ["old-id:true"])
        for a in [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID] { XCTAssertNil(k.get(a)) }
        XCTAssertEqual(AccountWipe.retireOldKey(), "none"); XCTAssertEqual(seen.count, 1, "nothing left: no second revoke")
    }
    func testARevokeThatFailsStillDeletesAndSaysSo() throws {
        let k = KeychainKeyStore(); try k.set("old-key-not-real", account: KeyAccount.apiKey); try k.set("old-id", account: KeyAccount.keyID)
        AccountWipe.revoke = { _, _ in false }; XCTAssertEqual(AccountWipe.retireOldKey(), "not revoked"); XCTAssertNil(k.get(KeyAccount.apiKey)); XCTAssertNil(k.get(KeyAccount.keyID))
        try k.set("old-key-not-real", account: KeyAccount.apiKey); XCTAssertEqual(AccountWipe.retireOldKey(), "not revoked", "a key with no id cannot be revoked, and is still deleted"); XCTAssertNil(k.get(KeyAccount.apiKey))
    }
    func testNothingTheOldKeyCleanerDoesLeavesTheKeyInAnyReturnValue() throws {
        let k = KeychainKeyStore(); try k.set("old-key-not-real", account: KeyAccount.apiKey)
        XCTAssertFalse(AccountWipe.retireOldKey().contains("old-key"))
    }

    func testHeldKeysAreHeldAndClearedAndTheMailSealMakesAKeyOnlyWhileOneIsHeld() throws {
        XCTAssertFalse(HeldKeys.present); XCTAssertNil(MailSeal().seal(Data("m".utf8)), "no key held: the seal makes none")
        HeldKeys.hold(key: "k-not-real", keyId: "id", computerId: "c"); XCTAssertTrue(HeldKeys.present); XCTAssertEqual(HeldKeys.store.get(KeyAccount.computerID), "c")
        XCTAssertNotNil(MailSeal().seal(Data("m".utf8)), "a key is held: the seal is made")
        HeldKeys.hold(key: "", keyId: "", computerId: ""); XCTAssertFalse(HeldKeys.present); XCTAssertNil(HeldKeys.store.get(KeyAccount.apiKey)); XCTAssertNil(HeldKeys.store.get(KeyAccount.keyID))
        XCTAssertNil(KeychainKeyStore().get(KeyAccount.apiKey), "native never persists the key")
    }

    /// The one carry-over (voice): the speaker profile and the voice choice are moved once from the old path into the account folder, only while a key is held; with none held the old file stays in use and is NEVER deleted here.
    func testTheVoiceIsMovedOnceWhileAKeyIsHeldAndNeverDeletedWithoutOne() throws {
        let base = AccountWipe.base
        try fm.createDirectory(at: base.appendingPathComponent("../MikeVoice"), withIntermediateDirectories: true)
        let oldProfile = base.appendingPathComponent("../MikeVoice/speaker.json"), oldChoice = base.appendingPathComponent("voice-choice.json")
        try Data("profile".utf8).write(to: oldProfile); try Data("choice".utf8).write(to: oldChoice)
        let kept = AccountWipe.url("MikeVoice/speaker.json"); XCTAssertEqual(kept.standardizedFileURL.path, oldProfile.standardizedFileURL.path, "no key held: the old file stays in use"); XCTAssertTrue(fm.fileExists(atPath: oldProfile.path))
        HeldKeys.hold(key: "k-not-real", keyId: "", computerId: "")
        let p = AccountWipe.url("MikeVoice/speaker.json"), c = AccountWipe.url("voice-choice.json")
        XCTAssertEqual(try String(contentsOf: p, encoding: .utf8), "profile"); XCTAssertEqual(try String(contentsOf: c, encoding: .utf8), "choice")
        XCTAssertFalse(fm.fileExists(atPath: oldProfile.path)); XCTAssertFalse(fm.fileExists(atPath: oldChoice.path), "moved, not copied")
        try Data("newer".utf8).write(to: oldChoice); _ = AccountWipe.url("voice-choice.json")
        XCTAssertEqual(try String(contentsOf: c, encoding: .utf8), "choice", "a second call never overwrites the file in the folder")
        XCTAssertNil(try? String(contentsOf: AccountWipe.url("takes-copy-own.json"), encoding: .utf8), "nothing else is carried over")
    }

    /// THE SCAN: a native store may not build its own path in the app's folders. Scanned: the Kit, the app target (exact-package/apple, not the gitignored Kit copy) and the native modules. Tokens: every way to name a folder. Exceptions are per LINE, with a reason.
    /// A store that takes a root or dir parameter is checked at its callers: they must pass `AccountWipe.url`.
    func testNoStoreBuildsItsOwnAccountPath() throws {
        let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        let roots = ["AutoradioKit/Sources/AutoradioKit", "exact-package/apple", "AutoradioLab/modules/apple"]
        let tokens = ["applicationSupportDirectory", "cachesDirectory", "documentDirectory", "documentsDirectory", "libraryDirectory", "temporaryDirectory", "NSTemporaryDirectory", "NSHomeDirectory", "containerURL", "URL.applicationSupport"]
        // (file, text the line contains, reason)
        let allowed: [(String, String, String)] = [
            ("AccountWipe.swift", "", "the one function and the wipe itself"),
            ("KnownModels.swift", "", "model downloads: the device's, not an account's"),
            ("Voice/WeSpeakerEmbedder.swift", "applicationSupportDirectory", "the speaker model download: the device's"),
            ("FixtureClip.swift", "temporaryDirectory", "a proof fixture clip: no account content"), ("FixtureTurn.swift", "temporaryDirectory", "a proof fixture turn: no account content"),
            ("AutoradioModule.swift", "temporaryDirectory.appendingPathComponent(\"takes\")", "the take recording scratch in `extraPaths`: removed and verified by the wipe"),
            ("AutoradioModule.swift", "applicationSupportDirectory", "the support root the not-yet-moved stores and `extraPaths` hang off (onboarding, books, content)"),
            ("AutoradioModule.swift", "cachesDirectory", "removal of the old Autoradio/files cache: no write"),
            ("ModelsEntry.swift", "applicationSupportDirectory", "the legacy theme folder, read only: themes are the device's"),
            ("VoiceSetup.swift", "NSHomeDirectory", "free disk space for the device profile"), ("Configurator.swift", "NSHomeDirectory", "free disk space for the device profile"),
        ]
        var bad: [String] = []; var callers: [String] = []
        for r in roots {
            let dir = mobile.appendingPathComponent(r); guard FileManager.default.fileExists(atPath: dir.path) else { XCTFail("missing \(r)"); continue }
            let en = FileManager.default.enumerator(at: dir, includingPropertiesForKeys: nil)
            while let u = en?.nextObject() as? URL {
                guard u.pathExtension == "swift", !u.path.contains("/Kit/"), let text = try? String(contentsOf: u, encoding: .utf8) else { continue }
                let rel = String(u.path.dropFirst(dir.path.count + 1))
                for (n, line) in text.components(separatedBy: "\n").enumerated() {
                    let code = line.components(separatedBy: "//").first ?? line
                    if tokens.contains(where: { code.contains($0) }), !allowed.contains(where: { rel.hasSuffix($0.0) && ($0.1.isEmpty || code.contains($0.1)) }) { bad.append("\(rel):\(n + 1)") }
                    if code.contains("KeptTurnStore(root:") || code.contains("ConfigStore(dir:"), !code.contains("AccountWipe.url") { callers.append(rel + (code.contains("ConfigStore") ? " ConfigStore" : " KeptTurnStore")) }
                }
            }
        }
        XCTAssertEqual(bad, [], "a native store names its account file through AccountWipe.url, or the line is excepted with a reason")
        XCTAssertEqual(callers.filter { !$0.contains("Tests") }, ["LabRuntime.swift ConfigStore"], "KeptTurnStore gets AccountWipe.url; the lab's ConfigStore (LabConfiguration presets, no account content) is the one listed exception")
    }
    final class Box: @unchecked Sendable { private let l = NSLock(); private var v = false; func set() { l.lock(); v = true; l.unlock() }; var value: Bool { l.lock(); defer { l.unlock() }; return v } }
    /// Test isolation: no test names the real Application Support, Caches or UserDefaults itself (the account base under XCTest is a temp folder). The guard test and this scan are the only places the words appear.
    func testNoTestNamesARealFolder() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent(); var bad: [String] = []
        for u in try fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil) where u.pathExtension == "swift" && !["KeychainFakeCase.swift", "AccountWipeTests.swift"].contains(u.lastPathComponent) {
            for (n, line) in (try String(contentsOf: u, encoding: .utf8)).components(separatedBy: "\n").enumerated() where ["applicationSupportDirectory", "cachesDirectory", "UserDefaults.standard", "homeDirectoryForCurrentUser", "NSHomeDirectory"].contains(where: { (line.components(separatedBy: "//").first ?? line).contains($0) }) { bad.append("\(u.lastPathComponent):\(n + 1)") }
        }
        XCTAssertEqual(bad, [])
    }
    /// THE RESIDUAL CAN ONLY SHRINK: the Swift files that still call the server with the key (`ChoirClient`) are pages still in Swift (station 2 moves them to shared code). A new file that uses it fails here; a file that stops using it must leave the list.
    func testOnlyTheListedNativeFilesCallTheServerWithTheKey() throws {
        let kit = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Sources/AutoradioKit")
        let listed: Set<String> = ["AccountWipe.swift", "BasicStages.swift", "Choir/AutoradioDesk.swift", "Choir/ChoirApp.swift", "Choir/ChoirDesk.swift", "Choir/ChoirProgramme.swift", "Choir/ChoirSystem1.swift", "Choir/PodcastClips.swift", "Choir/RadioProgramme.swift", "Choir/StretchCandidates.swift", "Choir/Takes.swift", "Notebook.swift", "TextureEditor.swift", "TextureReaderSource.swift"]
        var using: Set<String> = []; let en = FileManager.default.enumerator(at: kit, includingPropertiesForKeys: nil)
        while let u = en?.nextObject() as? URL { guard u.pathExtension == "swift", u.lastPathComponent != "ChoirClient.swift", let t = try? String(contentsOf: u, encoding: .utf8) else { continue }
            if t.components(separatedBy: "\n").contains(where: { let c = $0.components(separatedBy: "//").first ?? $0; return c.contains("ChoirClient") || c.contains(".client(") }) { using.insert(String(u.path.dropFirst(kit.path.count + 1))) } }
        XCTAssertEqual(using, listed, "a file that newly calls the server with ChoirClient is refused (move the call to shared code); a file that no longer does must be removed from this list")
    }
    /// The Kit answers `choirPlayer` with default values while no module is built; they must carry exactly the fields the interface declares as that name's substitute.
    func testTheNotReadyAnswerCarriesTheDeclaredFields() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("exact-package/exact.json")
        let names = (try JSONSerialization.jsonObject(with: Data(contentsOf: url)) as! [String: Any])["names"] as! [[String: Any]]
        let declared = Set((((names.first { $0["source"] as? String == "choirPlayer" }!["substitute"] as! [String: Any])["default"]) as! [String: Any]).keys)
        XCTAssertEqual(Set((try JSONSerialization.jsonObject(with: JSONEncoder().encode(ChoirPlayerView())) as! [String: Any]).keys), declared)
    }
}
