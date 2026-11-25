import XCTest
@testable import AutoradioKit

/// The app's `choirAuth` source answers `ChoirAuthState` as JSON (AutoradioModule.swift) and the interface (exact-package/exact.json) says what shape Exact checks it against: an answer must have
/// exactly its declared shape. The two drifted when the device-code sign-in left (the Swift type lost `userCode` and `verificationURL`, the interface kept them) and the console repeated
/// "field `userCode` is missing" every few seconds. The interface is data (JSON), read here as data.
final class AuthShapeTests: XCTestCase {
    func testTheAnswerHasExactlyTheDeclaredFields() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("exact-package/exact.json")
        let iface = try JSONSerialization.jsonObject(with: Data(contentsOf: url)) as! [String: Any]
        let declared = Set(((iface["shapes"] as! [String: Any])["ChoirAuth"] as! [String: Any]).keys)
        let answered = Set((try JSONSerialization.jsonObject(with: JSONEncoder().encode(ChoirAuthState())) as! [String: Any]).keys)
        XCTAssertEqual(answered, declared)
        let sub = ((iface["names"] as! [[String: Any]]).first { ($0["source"] as? String) == "choirAuth" }?["substitute"] as? [String: Any])?["default"] as? [String: Any]
        XCTAssertEqual(Set(sub?.keys.map { $0 } ?? []), declared, "the build's substitute answer has the same fields")
    }
}
