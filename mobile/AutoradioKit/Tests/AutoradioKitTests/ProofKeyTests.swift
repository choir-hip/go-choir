import XCTest
@testable import AutoradioKit

final class ProofKeyTests: XCTestCase {
    var dir: URL!
    override func setUp() {
        dir = FileManager.default.temporaryDirectory.appendingPathComponent("proofkey-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    }
    override func tearDown() { try? FileManager.default.removeItem(at: dir) }
    func write(_ s: String) { try? Data(s.utf8).write(to: dir.appendingPathComponent(ProofKey.fileName)) }
    var exists: Bool { FileManager.default.fileExists(atPath: dir.appendingPathComponent(ProofKey.fileName).path) }

    func testNoFileDoesNothing() {
        let k = InMemoryKeyStore()
        XCTAssertFalse(ProofKey.importIfPresent(documents: dir, into: k))
        XCTAssertNil(k.get(KeyAccount.apiKey))
    }
    func testKeyIsStoredTrimmedAndFileDeleted() {
        let k = InMemoryKeyStore([KeyAccount.computerID: "old", KeyAccount.keyID: "old-key"])
        write("choir_sk_INVENTED0123456789\n")
        XCTAssertTrue(ProofKey.importIfPresent(documents: dir, into: k))
        XCTAssertEqual(k.get(KeyAccount.apiKey), "choir_sk_INVENTED0123456789")
        XCTAssertNil(k.get(KeyAccount.computerID))
        XCTAssertNil(k.get(KeyAccount.keyID))
        XCTAssertFalse(exists)
    }
    func testEmptyAndWrongFilesAreDeletedAndStoreNothing() {
        for body in ["", "  \n", "not a key at all", "choir_sk_has a space inside"] {
            let k = InMemoryKeyStore()
            write(body)
            XCTAssertFalse(ProofKey.importIfPresent(documents: dir, into: k), body)
            XCTAssertNil(k.get(KeyAccount.apiKey))
            XCTAssertFalse(exists)
        }
    }
    func testRefusedWriteKeepsTheFile() {
        final class Refusing: KeyStore, @unchecked Sendable {
            func get(_ a: String) -> String? { nil }
            func set(_ v: String, account: String) throws { throw LabError.unavailable("no") }
            func delete(_ a: String) {}
        }
        write("choir_sk_INVENTED0123456789")
        XCTAssertFalse(ProofKey.importIfPresent(documents: dir, into: Refusing()))
        XCTAssertTrue(exists)
    }
}
