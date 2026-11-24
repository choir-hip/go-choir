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
        XCTAssertEqual(KeychainKeyStore.defaultService, Bundle.main.bundleIdentifier ?? "news.choir.autoradio.choir")
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
}
