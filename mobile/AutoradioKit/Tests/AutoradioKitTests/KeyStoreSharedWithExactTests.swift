import XCTest
@testable import AutoradioKit

/// The sign-in key is written by Exact's secret store (shared code, AutoradioLab/app-signin.ts) as generic passwords with service = the bundle id and account = the secret's name; the voice
/// code reads and deletes the very items here. Made-up values and a made-up service, so no real item is touched.
final class KeyStoreSharedWithExactTests: XCTestCase {
    private let service = "news.choir.autoradio.test-\(UUID().uuidString.prefix(8))"

    func testTheAccountNamesAreTheSecretNamesSharedCodeGrants() {
        XCTAssertEqual(KeyAccount.apiKey, "choir.key"); XCTAssertEqual(KeyAccount.keyID, "choir.key-id"); XCTAssertEqual(KeyAccount.computerID, "choir.computer-id")
    }

    func testTheDefaultServiceIsTheBundleIdThatExactKeepsItsSecretsUnder() {
        XCTAssertEqual(KeychainKeyStore.defaultService, Bundle.main.bundleIdentifier ?? "unbundled-host")
        XCTAssertEqual(KeychainKeyStore().service, KeychainKeyStore.defaultService)
    }

    func testAnItemOneStoreObjectWroteIsReadByAnotherAndDeletedByIt() throws {
        let writer = KeychainKeyStore(service: service)
        defer { for a in [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID] { writer.delete(a) } }
        try writer.set("test-key-0001-not-real", account: KeyAccount.apiKey)
        let reader = KeychainKeyStore(service: service)      // a second object: nothing but the Keychain connects them (as Exact's store and this one)
        XCTAssertEqual(reader.get(KeyAccount.apiKey), "test-key-0001-not-real")
        reader.delete(KeyAccount.apiKey)
        XCTAssertNil(writer.get(KeyAccount.apiKey))
    }

    // The old Swift sign-in's items: revoked first, deleted once the server has answered, nothing carried over.
    func testALegacyKeyIsRevokedThenDeletedOnce() async {
        let store = InMemoryKeyStore(["api-key": "test-legacy-not-real", "key-id": "ak_old", "computer-id": "c1", "choir.key": "test-new-not-real"])
        var revoked: [(String, String)] = []
        var itemsHeldDuringRevoke = false
        let found = await LegacyKey.retire(store: store) { id, key in
            itemsHeldDuringRevoke = store.get("api-key") != nil
            revoked.append((id, key)); return true
        }
        XCTAssertTrue(found); XCTAssertTrue(itemsHeldDuringRevoke, "the key is still on the phone while it is revoked")
        XCTAssertEqual(revoked.count, 1); XCTAssertEqual(revoked.first?.0, "ak_old"); XCTAssertEqual(revoked.first?.1, "test-legacy-not-real")
        for a in LegacyKey.accounts { XCTAssertNil(store.get(a)) }
        XCTAssertEqual(store.get("choir.key"), "test-new-not-real", "the new sign-in is not touched")
        let again = await LegacyKey.retire(store: store) { _, _ in XCTFail("nothing is left to revoke"); return true }
        XCTAssertFalse(again)
    }
    func testAFailedRevokeKeepsTheItemsForTheNextLaunch() async {
        let store = InMemoryKeyStore(["api-key": "test-legacy-not-real", "key-id": "ak_old", "computer-id": "c1"])
        var tries = 0
        _ = await LegacyKey.retire(store: store) { _, _ in tries += 1; return false }
        XCTAssertEqual(tries, 1); XCTAssertEqual(store.get("api-key"), "test-legacy-not-real"); XCTAssertEqual(store.get("key-id"), "ak_old")
        let found = await LegacyKey.retire(store: store) { _, _ in tries += 1; return true }
        XCTAssertTrue(found); XCTAssertEqual(tries, 2)
        for a in LegacyKey.accounts { XCTAssertNil(store.get(a)) }
    }
    func testNothingLegacyIsANoOpAndAKeyWithoutAnIdCannotBeRevokedSoItsPlaintextGoes() async {
        let empty = InMemoryKeyStore()
        let none = await LegacyKey.retire(store: empty) { _, _ in XCTFail("no key"); return true }
        XCTAssertFalse(none)
        let partial = InMemoryKeyStore(["api-key": "test-legacy-not-real"])
        var asked: [String] = []
        let found = await LegacyKey.retire(store: partial) { id, _ in asked.append(id); return true }   // the caller is told there is no id, so it can say so
        XCTAssertTrue(found); XCTAssertNil(partial.get("api-key")); XCTAssertEqual(asked, [""])
    }
    func testTheLegacyServiceIsNeverTheCurrentOne() {
        XCTAssertNotEqual(LegacyKey.service, KeychainKeyStore.defaultService)
    }
}
