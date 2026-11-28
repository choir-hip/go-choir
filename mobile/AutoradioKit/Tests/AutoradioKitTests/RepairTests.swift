import XCTest
@testable import AutoradioKit

// Tests for the repair of the Autoradio app review (2026-11-11). Each test names the finding it covers; the failure lists are
// comment blocks in Concurrency.swift, MailSanitizer.swift and the files they belong to.

// MARK: Once, TaskBag, withDeadline (continuations resumed exactly once, tasks that end with their owner)

final class ConcurrencyTests: XCTestCase {
    func testOnceResumesOnceEvenWhenTwoPathsFire() async {
        let once = Once<Int>()
        let v: Int = await withCheckedContinuation { c in
            once.arm(c)
            XCTAssertTrue(once.resume(1))
            XCTAssertFalse(once.resume(2), "the second path must not resume again")
        }
        XCTAssertEqual(v, 1)
    }

    func testOnceResumedBeforeArmedStillDelivers() async {
        let once = Once<String>()
        once.resume("early")                      // a stop path that ran before the wait began
        let v: String = await withCheckedContinuation { c in once.arm(c) }
        XCTAssertEqual(v, "early")
    }

    func testOnceDeliversAnOptionalNil() async {
        let once = Once<Int?>()
        once.resume(nil)
        let v: Int? = await withCheckedContinuation { c in once.arm(c) }
        XCTAssertNil(v)
    }

    func testReleaseWaitersResumesAWaitOnAStoppedPlayerNode() async {
        // The audio engine does not call a scheduled buffer's completion once its node is stopped: every stop path calls
        // releaseWaiters, which must wake what is waiting (and only once).
        let audio = LiveAudio()
        let once = Once<Void>()
        let id = UUID()
        audio.waiters.write { $0[id] = once }
        let woke = expectation(description: "wait resumed")
        Task { await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in once.arm(c) }; woke.fulfill() }
        try? await Task.sleep(nanoseconds: 50_000_000)
        audio.releaseWaiters()
        audio.releaseWaiters()                    // a second stop finds nothing to resume
        await fulfillment(of: [woke], timeout: 2)
        XCTAssertTrue(audio.waiters.read { $0.isEmpty })
    }

    func testTaskBagCancelsAndCancelsLateAdditions() async {
        let bag = TaskBag()
        let t1 = Task { () -> Bool in (try? await Task.sleep(nanoseconds: 5_000_000_000)) == nil }
        bag.add(t1)
        bag.cancelAll()
        let t2 = Task { () -> Bool in (try? await Task.sleep(nanoseconds: 5_000_000_000)) == nil }
        bag.add(t2)                                // added after: cancelled on the spot
        let a = await t1.value, b = await t2.value
        XCTAssertTrue(a); XCTAssertTrue(b)
    }

    func testWithDeadlineReturnsWhenTheBodyFinishes() async {
        let v = await withDeadline(5) { 7 }
        XCTAssertEqual(v, 7)
    }

    func testWithDeadlineTimesOutAndCancelsTheBody() async {
        let saw = Locked(false)
        let v: Int? = await withDeadline(0.1) {
            do { try await Task.sleep(nanoseconds: 5_000_000_000) } catch { saw.value = true }
            return 1
        }
        XCTAssertNil(v)
        try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertTrue(saw.value, "the body was cancelled")
    }

    func testWithDeadlineWakesAtOnceWhenTheCallerIsCancelled() async {
        let t0 = Date()
        let task = Task { () -> Int? in await withDeadline(30) { try? await Task.sleep(nanoseconds: 30_000_000_000); return 1 } }
        try? await Task.sleep(nanoseconds: 100_000_000)
        task.cancel()
        let v = await task.value
        XCTAssertNil(v)
        XCTAssertLessThan(Date().timeIntervalSince(t0), 5, "must not wait out the 30 s deadline")
    }

    func testLockedIsSafeUnderContention() async {
        let n = Locked(0)
        await withTaskGroup(of: Void.self) { g in
            for _ in 0..<50 { g.addTask { for _ in 0..<200 { n.write { $0 += 1 } } } }
        }
        XCTAssertEqual(n.value, 10_000)
    }
}

// MARK: The key goes only to the configured host

final class HostGuardTests: XCTestCase {
    let base = URL(string: "https://choir.news")!

    func testSameOriginTable() {
        let yes = ["https://choir.news/x", "https://CHOIR.news/x?y=1", "https://choir.news:443/x"]
        let no = ["http://choir.news/x", "https://choir.news:8443/x", "https://evil.example.com/x", "https://choir.news.evil.example.com/x",
                  "https://choir.news@evil.example.com/x", "https://user:pw@choir.news/x", "ftp://choir.news/x", "file:///etc/passwd", "choir.news", ""]
        for s in yes { XCTAssertTrue(SameOrigin.matches(s, base), s) }
        for s in no { XCTAssertFalse(SameOrigin.matches(s, base), s) }
    }

    func testAClientForAnotherHostRefusesToCarryTheKey() async {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        let keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        let env = ChoirEnvironment()
        env.keys = keys; env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init() }
        let c = env.client(baseURL: "https://evil.example.com")
        do { _ = try await c.send("GET", "/api/anything"); XCTFail("must refuse") }
        catch { XCTAssertEqual(error as? ChoirError, .wrongHost("evil.example.com")) }
        XCTAssertTrue(StubProtocol.seen.isEmpty, "nothing was sent anywhere")
        // The same client without the key (sign-in begin) is not the key's business.
        _ = try? await c.send("GET", "/auth/ping", authenticated: false)
        XCTAssertEqual(StubProtocol.seen.count, 1)
        XCTAssertNil(StubProtocol.seen.first?.headers["authorization"])
        // The configured host works.
        StubProtocol.seen = []
        _ = try? await env.client().send("GET", "/api/anything")
        XCTAssertEqual(StubProtocol.seen.first?.headers["authorization"], "Bearer \(secretKey)")
    }
}
