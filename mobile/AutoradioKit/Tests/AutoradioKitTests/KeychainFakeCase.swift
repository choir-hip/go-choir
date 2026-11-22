import XCTest
import Security
@testable import AutoradioKit

/// NO test reads, writes or deletes a real Keychain item, and NO test touches the real Application Support folder. The Keychain is reached only through `SecDoor` (its real* functions answer "not available" under XCTest);
/// the account base resolves to a temp folder under XCTest even with no override. A test class that can reach a store, the wipe or the Keychain derives from this class, which gives it an in-memory Keychain, its own temp
/// base, no extra paths and the seams set to inert values, and puts everything back at tearDown. `unreadable` makes every Keychain read answer "interaction not allowed" (a locked phone).
final class FakeKeychain: @unchecked Sendable {
    private let l = NSLock(); private var items: [String: Data] = [:]; var unreadable = false
    private func key(_ q: CFDictionary) -> String { let d = q as! [String: Any]; return "\(d[kSecAttrService as String] as? String ?? "")/\(d[kSecAttrAccount as String] as? String ?? "")" }
    func copy(_ q: CFDictionary, _ out: UnsafeMutablePointer<CFTypeRef?>?) -> OSStatus {
        l.lock(); defer { l.unlock() }; if unreadable { return errSecInteractionNotAllowed }
        guard let v = items[key(q)] else { return errSecItemNotFound }
        if (q as! [String: Any])[kSecReturnData as String] as? Bool == true { out?.pointee = v as NSData }
        return errSecSuccess
    }
    func add(_ q: CFDictionary, _ out: UnsafeMutablePointer<CFTypeRef?>?) -> OSStatus {
        l.lock(); defer { l.unlock() }; let k = key(q); if items[k] != nil { return errSecDuplicateItem }
        items[k] = (q as! [String: Any])[kSecValueData as String] as? Data ?? Data(); return errSecSuccess
    }
    func delete(_ q: CFDictionary) -> OSStatus { l.lock(); defer { l.unlock() }; return items.removeValue(forKey: key(q)) == nil ? errSecItemNotFound : errSecSuccess }
}

@MainActor class KeychainFakeCase: XCTestCase {
    var keychain = FakeKeychain(); var base: URL!
    override func setUp() {
        super.setUp(); keychain = FakeKeychain(); let k = keychain
        SecDoor.copy = { k.copy($0, $1) }; SecDoor.add = { k.add($0, $1) }; SecDoor.delete = { k.delete($0) }
        base = FileManager.default.temporaryDirectory.appendingPathComponent("autoradio-test-\(UUID().uuidString)", isDirectory: true)
        AccountWipe.baseOverride = base.appendingPathComponent("Autoradio"); AccountWipe.extraPaths = []
        AccountWipe.revoke = { _, _ in true }; AccountWipe.forgetMail = { MailSeal().forget() }; AccountWipe.oldKeys = KeychainKeyStore(); HeldKeys.hold(key: "", keyId: "", computerId: "")
    }
    override func tearDown() {
        SecDoor.copy = SecDoor.realCopy; SecDoor.add = SecDoor.realAdd; SecDoor.delete = SecDoor.realDelete
        AccountWipe.baseOverride = nil; AccountWipe.extraPaths = []; HeldKeys.hold(key: "", keyId: "", computerId: ""); try? FileManager.default.removeItem(at: base)
        super.tearDown()
    }
}

/// The guards, independent of test order: the door's real functions never reach the Keychain under XCTest, and the account base is never the real Application Support.
final class IsolationGuardTests: XCTestCase {
    func testTheKeychainDoorAnswersNotAvailable() {
        let q = [:] as CFDictionary
        XCTAssertEqual(SecDoor.realCopy(q, nil), errSecNotAvailable); XCTAssertEqual(SecDoor.realAdd(q, nil), errSecNotAvailable); XCTAssertEqual(SecDoor.realDelete(q), errSecNotAvailable)
    }
    func testTheAccountBaseUnderXCTestIsNeverTheRealOne() {
        let saved = AccountWipe.baseOverride; AccountWipe.baseOverride = nil; defer { AccountWipe.baseOverride = saved }
        let real = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].path
        XCTAssertFalse(AccountWipe.base.path.hasPrefix(real), "\(AccountWipe.base.path)"); XCTAssertTrue(AccountWipe.base.path.contains("autoradio-xctest-base"))
    }
}
