import XCTest
@testable import AutoradioKit

/// The mail cipher: AES-GCM with a native key that never leaves. A made-up Keychain service stands in for the real one.
final class MailSealTests: KeychainFakeCase {
    private let service = "news.choir.autoradio.mailseal.test-\(UUID().uuidString.prefix(8))"

    func testSealThenOpenAndNothingElseOpens() {
        let m = MailSeal(service: service, signedIn: { true }); defer { m.forget() }
        let plain = Data("Thursday dinner, maya.ortiz@example.com".utf8)
        let sealed = m.seal(plain)!
        XCTAssertNil(String(data: sealed, encoding: .utf8)?.range(of: "Thursday"), "no plain word in the sealed bytes")
        XCTAssertEqual(m.open(sealed), plain)
        XCTAssertNotEqual(m.seal(plain), sealed, "a fresh nonce every time")
        var bent = sealed; bent[bent.count / 2] ^= 1
        XCTAssertNil(m.open(bent)); XCTAssertNil(m.open(sealed.prefix(10))); XCTAssertNil(m.open(Data("plain".utf8)))
    }
    func testTheKeyIsForgottenAndOldDataNeverOpensWithTheNextKey() {
        let m = MailSeal(service: service, signedIn: { true }); defer { m.forget() }
        let sealed = m.seal(Data("one".utf8))!
        m.forget()
        XCTAssertNil(m.open(sealed), "no key: nothing opens, and open never makes one")
        _ = m.seal(Data("two".utf8))                       // a new key is made on the next seal
        XCTAssertNil(m.open(sealed), "another key: the old copy does not open")
    }
    func testTheReplyIsJSONShapedAndNeverCarriesTheKey() {
        let m = MailSeal(service: service, signedIn: { true }); defer { m.forget() }
        let b64 = Data("body".utf8).base64EncodedString()
        let s = m.reply(["op": "mailSeal", "b64": b64])!
        XCTAssertEqual(s["ok"] as? Bool, true); XCTAssertEqual(Set(s.keys), ["ok", "b64"])
        let o = m.reply(["op": "mailOpen", "b64": s["b64"] as! String])!
        XCTAssertEqual(o["b64"] as? String, b64)
        XCTAssertEqual(m.reply(["op": "mailOpen", "b64": "!!!"])?["ok"] as? Bool, false)
        XCTAssertEqual(m.reply(["op": "mailForget"])?["ok"] as? Bool, true)
        XCTAssertNil(m.reply(["op": "get"]), "not ours")
    }
    // Review 2026-11-19 (mail, seat 1): K1 the record's identity is authenticated (a swapped file fails); K2 "cannot" (no key now) is not "bad" (the tag failed);
    // K3 two first uses at once end with ONE key and no lost write; K4 no key is made after the sign-in key is gone; K5 old data opens only with its aad.
    func testK1AFileSwappedToAnotherRecordDoesNotOpen() {
        let m = MailSeal(service: service, signedIn: { true }); defer { m.forget() }
        let sealed = m.seal(Data("one".utf8), aad: Data("msg|a".utf8))!
        XCTAssertEqual(m.open(sealed, aad: Data("msg|a".utf8)), Data("one".utf8))
        XCTAssertNil(m.open(sealed, aad: Data("msg|b".utf8)))
        XCTAssertEqual(m.openResult(sealed, aad: Data("msg|b".utf8)), .bad)
    }
    func testK2NoKeyIsCannotAndATamperedFileIsBad() {
        let m = MailSeal(service: service, signedIn: { true }); defer { m.forget() }
        let sealed = m.seal(Data("x".utf8))!
        var bent = sealed; bent[bent.count - 1] ^= 1
        XCTAssertEqual(m.openResult(bent), .bad)
        XCTAssertEqual(m.reply(["op": "mailOpen", "b64": "AAAA"])?["why"] as? String, "bad")
        m.forget()
        XCTAssertEqual(m.openResult(sealed), .cannot, "a missing or locked key does not condemn the file")
        XCTAssertEqual(m.reply(["op": "mailOpen", "b64": sealed.base64EncodedString()])?["why"] as? String, "cannot")
    }
    func testK3TwoFirstUsesAtOnceShareOneKey() {
        let m = MailSeal(service: service, signedIn: { true }); defer { m.forget() }
        let n = 8; let results = NSMutableArray(); let lock = NSLock()
        DispatchQueue.concurrentPerform(iterations: n) { i in
            let s = m.seal(Data("m\(i)".utf8))
            lock.lock(); results.add(s ?? NSNull()); lock.unlock()
        }
        XCTAssertEqual(results.compactMap { $0 as? Data }.count, n, "no write lost to the race")
        for case let d as Data in results { XCTAssertNotNil(m.open(d), "every copy opens with the one key") }
    }
    func testK4NoKeyIsMadeWhenNoSignInKeyIsStored() {
        let m = MailSeal(service: service, signedIn: { false }); defer { m.forget() }
        XCTAssertNil(m.seal(Data("late".utf8)), "a seal after sign-out makes no key")
        XCTAssertEqual(m.reply(["op": "mailSeal", "b64": Data("x".utf8).base64EncodedString()])?["ok"] as? Bool, false)
        XCTAssertEqual(m.reply(["op": "mailCan"])?["ok"] as? Bool, true)
    }

    func testTheMailKeyIsNotTheSignInItem() {
        XCTAssertNotEqual(MailSeal.service, KeychainKeyStore().service)
    }
}

extension MailSeal { func open(_ d: Data, aad: Data = Data()) -> Data? { if case .plain(let p) = openResult(d, aad: aad) { return p }; return nil } }
