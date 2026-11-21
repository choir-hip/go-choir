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

    func testRemoteStagesRefuseAUrlOffTheConfiguredHost() async {
        let env = ChoirEnvironment()                            // the default host
        XCTAssertThrowsError(try RemoteHost.require(URL(string: "https://evil.example.com/s1")!, env: env))
        XCTAssertNoThrow(try RemoteHost.require(URL(string: "https://choir.news/api/s1")!, env: env))
        XCTAssertTrue(RemoteHost.allows(URL(string: "https://choir.news/review/1")!, env: env))
        env.configuredBaseURL = "https://staging.choir.example:8443"
        XCTAssertFalse(RemoteHost.allows(URL(string: "https://choir.news/review/1")!, env: env), "compares with the configured host, not the default")
        XCTAssertTrue(RemoteHost.allows(URL(string: "https://staging.choir.example:8443/review/1")!, env: env))
        do {
            let p = StageParams(["url": .string("https://evil.example.com/s1")])
            _ = try await RemoteSystem1(p).answer(System1Input(context: "c", listenerWords: "w", questions: []))
            XCTFail("must refuse")
        } catch { XCTAssertEqual((error as? ChoirError), .wrongHost("evil.example.com")) }
        do {
            _ = try await RemoteProgramme(StageParams(["url": .string("https://evil.example.com/p")])).load()
            XCTFail("must refuse")
        } catch { XCTAssertEqual((error as? ChoirError), .wrongHost("evil.example.com")) }
    }
}

// MARK: Ids as one path segment

final class PathSegmentTests: XCTestCase {
    func testEncodeTable() {
        XCTAssertEqual(PathSegment.encode("m6"), "m6")
        XCTAssertEqual(PathSegment.encode("a/b"), "a%2Fb")
        XCTAssertEqual(PathSegment.encode("a?b#c"), "a%3Fb%23c")
        XCTAssertEqual(PathSegment.encode("100%"), "100%25")
        XCTAssertEqual(PathSegment.encode(".."), "%2E%2E")
        XCTAssertEqual(PathSegment.encode("."), "%2E")
        XCTAssertEqual(PathSegment.encode("../../etc"), "..%2F..%2Fetc")
        XCTAssertEqual(PathSegment.encode("<id@host>"), "%3Cid%40host%3E")
        XCTAssertEqual(PathSegment.encode("é"), "%C3%A9")
        XCTAssertEqual(PathSegment.encode(""), "")
    }
}

// MARK: Computer discovery is one call, whoever waits

final class SingleFlightTests: XCTestCase {
    func testAGivingUpCallerDoesNotLetTheNextOneStartASecondDiscovery() async {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        let keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey])
        let env = ChoirEnvironment()
        env.keys = keys; env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { req in
            if req.path == "/api/compute/status" { Thread.sleep(forTimeInterval: 0.4) }
            return .init(json: ["computers": [["computer_id": "comp-7", "kind": "interactive", "current": true]]])
        }
        let c = env.client()
        let a = Task { try? await c.computerID() }
        try? await Task.sleep(nanoseconds: 80_000_000)
        a.cancel()
        _ = await a.value
        let b = try? await c.computerID()
        XCTAssertEqual(b, "comp-7")
        XCTAssertEqual(StubProtocol.seen.filter { $0.path == "/api/compute/status" }.count, 1, "one discovery")
        // After it ended, the entry is gone: a fresh client with no stored id discovers again.
        keys.delete(KeyAccount.computerID)
        _ = try? await c.computerID()
        XCTAssertEqual(StubProtocol.seen.filter { $0.path == "/api/compute/status" }.count, 2)
    }
}

// MARK: Small ones

final class SmallRepairTests: XCTestCase {
    // (The UTF-8 trim and the cut-text reads moved with the Files page: AutoradioLab/files.test.ts, F6.)

    func testBadSampleRatesAreRefusedNotTrapped() {
        XCTAssertNil(monoBuffer([0.1, 0.2], rate: 0))
        XCTAssertNil(monoBuffer([0.1, 0.2], rate: -1))
        XCTAssertNil(monoBuffer([0.1, 0.2], rate: .nan))
        XCTAssertNil(pcmBuffer(AudioFrame(samples: [0.1], sampleRate: 0, t: 0)))
        XCTAssertNotNil(monoBuffer([], rate: 16_000), "an empty buffer is fine and does not trap on baseAddress")
    }
}
