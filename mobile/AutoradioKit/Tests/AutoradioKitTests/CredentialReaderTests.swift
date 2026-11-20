import XCTest
@testable import AutoradioKit

/// The signed-in seam reads the item today's sign-in wrote, where it is (never moved, re-keyed or renamed). Made-up values; a made-up
/// Keychain service stands in for the real one, so the owner's item is never touched.
final class CredentialReaderTests: XCTestCase {
    private let service = "news.choir.autoradio.choir.test-\(UUID().uuidString.prefix(8))"

    func testTheServiceTodaysSignInWritesUnderIsTheOneTheSeamReads() {
        XCTAssertEqual(KeychainKeyStore().service, "news.choir.autoradio.choir")
        XCTAssertEqual(KeyAccount.apiKey, "api-key"); XCTAssertEqual(KeyAccount.keyID, "key-id"); XCTAssertEqual(KeyAccount.computerID, "computer-id")
    }

    func testAnItemWrittenByTodaysSignInIsReadByTheSeam() throws {
        let written = KeychainKeyStore(service: service)
        defer { for a in [KeyAccount.apiKey, KeyAccount.keyID, KeyAccount.computerID, "other"] { written.delete(a) } }
        try written.set("test-key-0001-not-real", account: KeyAccount.apiKey)
        try written.set("key-id-1", account: KeyAccount.keyID)
        try written.set("comp-1", account: KeyAccount.computerID)
        try written.set("not for the seam", account: "other")
        let r = CredentialReader(store: KeychainKeyStore(service: service))   // a second store object: nothing but the Keychain connects them
        XCTAssertEqual(r.read(KeyAccount.apiKey), "test-key-0001-not-real")
        XCTAssertEqual(r.read(KeyAccount.keyID), "key-id-1")
        XCTAssertEqual(r.read(KeyAccount.computerID), "comp-1")
        XCTAssertNil(r.read("other"), "a name outside the three is never answered")
        XCTAssertEqual(r.reply(["op": "get", "account": "api-key"]) as NSDictionary, ["found": true, "value": "test-key-0001-not-real"] as NSDictionary)
        XCTAssertEqual(r.reply(["op": "get", "account": "other"]) as NSDictionary, ["found": false] as NSDictionary)
        XCTAssertEqual(r.reply(["op": "set", "account": "api-key"]) as NSDictionary, ["found": false] as NSDictionary, "the module only reads")
        // The seam wrote nothing: the item is as sign-in left it.
        XCTAssertEqual(written.get(KeyAccount.apiKey), "test-key-0001-not-real")
    }

    func testSignedOutIsNotFound() {
        let r = CredentialReader(store: InMemoryKeyStore())
        XCTAssertNil(r.read(KeyAccount.apiKey))
        XCTAssertEqual(r.reply(["op": "get", "account": "api-key"]) as NSDictionary, ["found": false] as NSDictionary)
    }

    func testTheSeamSeesTheSignOutThatSignInDoes() throws {
        let keys = InMemoryKeyStore([KeyAccount.apiKey: "test-key-0001-not-real", KeyAccount.keyID: "k"])
        let r = CredentialReader(store: keys)
        XCTAssertNotNil(r.read(KeyAccount.apiKey))
        keys.delete(KeyAccount.apiKey); keys.delete(KeyAccount.keyID); keys.delete(KeyAccount.computerID)
        XCTAssertNil(r.read(KeyAccount.apiKey), "after sign-out nothing is read")
    }

    func testSessionAnswersKeyAndComputerInOneRead() {
        let keys = InMemoryKeyStore([KeyAccount.apiKey: "test-key-0001-not-real", KeyAccount.computerID: "comp-1"])
        let r = CredentialReader(store: keys)
        XCTAssertEqual(r.reply(["op": "session"]) as NSDictionary, ["found": true, "value": "test-key-0001-not-real", "computerId": "comp-1"] as NSDictionary)
        XCTAssertEqual(CredentialReader(store: InMemoryKeyStore()).reply(["op": "session"]) as NSDictionary, ["found": false] as NSDictionary)
        XCTAssertEqual(r.reply(["op": "nope"]) as NSDictionary, ["found": false] as NSDictionary)
    }

    func testKeyRemainsIsTrueWhileTheItemIsStillThere() {
        let keys = InMemoryKeyStore([KeyAccount.apiKey: "test-key-0001-not-real"])
        let r = CredentialReader(store: keys)
        XCTAssertTrue(r.keyRemains(), "a delete that did nothing leaves the key: sign-out must say not done")
        keys.delete(KeyAccount.apiKey)
        XCTAssertFalse(r.keyRemains())
    }
}
