import XCTest
@testable import AutoradioKit

/// The mail cipher: AES-GCM with a native key that never leaves. A made-up Keychain service stands in for the real one.
final class MailSealTests: XCTestCase {
    private let service = "news.choir.autoradio.mailseal.test-\(UUID().uuidString.prefix(8))"

    func testSealThenOpenAndNothingElseOpens() {
        let m = MailSeal(service: service); defer { m.forget() }
        let plain = Data("Thursday dinner, maya.ortiz@example.com".utf8)
        let sealed = m.seal(plain)!
        XCTAssertNil(String(data: sealed, encoding: .utf8)?.range(of: "Thursday"), "no plain word in the sealed bytes")
        XCTAssertEqual(m.open(sealed), plain)
        XCTAssertNotEqual(m.seal(plain), sealed, "a fresh nonce every time")
        var bent = sealed; bent[bent.count / 2] ^= 1
        XCTAssertNil(m.open(bent)); XCTAssertNil(m.open(sealed.prefix(10))); XCTAssertNil(m.open(Data("plain".utf8)))
    }
    func testTheKeyIsForgottenAndOldDataNeverOpensWithTheNextKey() {
        let m = MailSeal(service: service); defer { m.forget() }
        let sealed = m.seal(Data("one".utf8))!
        m.forget()
        XCTAssertNil(m.open(sealed), "no key: nothing opens, and open never makes one")
        _ = m.seal(Data("two".utf8))                       // a new key is made on the next seal
        XCTAssertNil(m.open(sealed), "another key: the old copy does not open")
    }
    func testTheReplyIsJSONShapedAndNeverCarriesTheKey() {
        let m = MailSeal(service: service); defer { m.forget() }
        let b64 = Data("body".utf8).base64EncodedString()
        let s = m.reply(["op": "mailSeal", "b64": b64])!
        XCTAssertEqual(s["ok"] as? Bool, true); XCTAssertEqual(Set(s.keys), ["ok", "b64"])
        let o = m.reply(["op": "mailOpen", "b64": s["b64"] as! String])!
        XCTAssertEqual(o["b64"] as? String, b64)
        XCTAssertEqual(m.reply(["op": "mailOpen", "b64": "!!!"])?["ok"] as? Bool, false)
        XCTAssertEqual(m.reply(["op": "mailForget"])?["ok"] as? Bool, true)
        XCTAssertNil(m.reply(["op": "get"]), "not ours")
    }
    func testTheMailKeyIsNotTheSignInItem() {
        XCTAssertNotEqual(MailSeal.service, KeychainKeyStore().service)
    }
}
