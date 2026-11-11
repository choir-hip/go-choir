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

// MARK: The mail sanitiser: an allowlist, judged against hostile input

final class MailSanitizerTests: XCTestCase {
    /// (name, hostile input, strings that must not be in the output)
    let hostile: [(String, String, [String])] = [
        ("unquoted javascript href", "<a href=javascript:alert(1)>x</a>", ["javascript", "alert", "href"]),
        ("unquoted vbscript href", "<a href=vbscript:msgbox(1)>x</a>", ["vbscript", "msgbox"]),
        ("data html href", "<a href=\"data:text/html,<script>alert(1)</script>\">x</a>", ["data:", "alert", "<script"]),
        ("entity-encoded scheme (decimal)", "<a href=\"&#106;avascript:alert(1)\">x</a>", ["javascript", "&#106", "alert"]),
        ("entity-encoded scheme (hex, no semicolon)", "<a href=\"&#x6A avascript:alert(1)\">x</a>", ["alert"]),
        ("entity colon", "<a href=\"javascript&colon;alert(1)\">x</a>", ["javascript", "alert"]),
        ("entity tab inside", "<a href=\"jav&Tab;ascript:alert(1)\">x</a>", ["alert", "href"]),
        ("entity newline inside", "<a href=\"java&NewLine;script:alert(1)\">x</a>", ["alert", "href"]),
        ("raw tab inside", "<a href=\"jav\tascript:alert(1)\">x</a>", ["alert", "href"]),
        ("raw newline inside", "<a href=\"java\nscript:alert(1)\">x</a>", ["alert", "href"]),
        ("control char before", "<a href=\"\u{1}javascript:alert(1)\">x</a>", ["alert", "href"]),
        ("leading space", "<a href=\"  javascript:alert(1)\">x</a>", ["alert", "href"]),
        ("upper case", "<a HREF=\"JaVaScRiPt:alert(1)\">x</a>", ["alert", "href", "HREF"]),
        ("zero width inside", "<a href=\"java\u{200B}script:alert(1)\">x</a>", ["alert", "href"]),
        ("unknown entity in scheme", "<a href=\"java&bogus;script:alert(1)\">x</a>", ["alert", "href"]),
        ("relative url", "<a href=\"//evil.example.com/x\">x</a>", ["evil.example.com", "href"]),
        ("img onerror", "<img src=x onerror=alert(1)>", ["onerror", "alert"]),
        ("img onerror upper", "<IMG SRC=\"x\" ONERROR=\"alert(1)\">", ["onerror", "ONERROR", "alert"]),
        ("img onerror with slash separator", "<img/src=x/onerror=alert(1)>", ["onerror", "alert"]),
        ("handler after quoted value without space", "<a href=\"https://example.com/\"onclick=\"alert(1)\">x</a>", ["onclick", "alert"]),
        ("handler name with NUL", "<p on\u{0}click=alert(1)>x</p>", ["onclick", "alert"]),
        ("body onload", "<body onload=alert(1)>hello</body>", ["onload", "alert", "<body"]),
        ("script", "<script>alert(1)</script>after", ["script", "alert"]),
        ("script upper, spaced close", "<SCRIPT >alert(1)</SCRIPT >after", ["alert", "SCRIPT"]),
        ("script without end", "before<script>alert(1)", ["script", "alert"]),
        ("script src", "<script src=https://evil.example.com/x.js></script>", ["evil.example.com", "script"]),
        ("style with import", "<style>@import url(https://evil.example.com/x.css);</style>", ["evil.example.com", "@import", "<style"]),
        ("iframe", "<iframe src=https://evil.example.com></iframe>", ["evil.example.com", "iframe"]),
        ("object", "<object data=https://evil.example.com></object>", ["evil.example.com", "object"]),
        ("embed", "<embed src=https://evil.example.com>", ["evil.example.com", "embed"]),
        ("base", "<base href=https://evil.example.com/><a href=/x>x</a>", ["evil.example.com", "<base"]),
        ("meta refresh", "<meta http-equiv=refresh content=\"0;url=https://evil.example.com\">", ["evil.example.com", "refresh"]),
        ("link stylesheet", "<link rel=stylesheet href=https://evil.example.com/a.css>", ["evil.example.com", "<link"]),
        ("svg onload", "<svg onload=alert(1)><circle/></svg>", ["svg", "onload", "alert"]),
        ("svg with script", "<svg><script>alert(1)</script></svg>after", ["svg", "script", "alert"]),
        ("nested svg", "<svg><svg><a xlink:href=javascript:alert(1)>x</a></svg></svg>after", ["svg", "alert", "xlink"]),
        ("svg unclosed", "<svg><a xlink:href=javascript:alert(1)>x</a>", ["svg", "alert"]),
        ("math", "<math><mtext><script>alert(1)</script></mtext></math>after", ["math", "alert"]),
        ("form action", "<form action=https://evil.example.com><input name=a><button formaction=https://evil.example.com>go</button></form>",
         ["evil.example.com", "<form", "<input", "formaction", "<button"]),
        ("srcset", "<img src=\"data:image/png;base64,AAAA\" srcset=\"https://evil.example.com/a.png 2x\">", ["srcset", "evil.example.com"]),
        ("remote image", "<img src=\"https://evil.example.com/p.gif\" alt=pixel>", ["evil.example.com", "src="]),
        ("svg image data uri", "<img src=\"data:image/svg+xml;base64,PHN2ZyBvbmxvYWQ9YWxlcnQoMSk+\">", ["svg", "data:image"]),
        ("image data uri with html", "<img src=\"data:image/png;base64,AAAA\" onerror=alert(1)>", ["onerror", "alert"]),
        ("css url in style", "<div style=\"background:url(https://evil.example.com/b.png)\">x</div>", ["evil.example.com", "url("]),
        ("css expression", "<div style=\"width:expression(alert(1))\">x</div>", ["expression", "alert"]),
        ("css escaped url", "<div style=\"background:\\75rl(https://evil.example.com)\">x</div>", ["evil.example.com"]),
        ("css comment split", "<div style=\"background:ur/**/l(https://evil.example.com)\">x</div>", ["evil.example.com"]),
        ("css import in style attr", "<div style=\"@import 'https://evil.example.com'\">x</div>", ["evil.example.com"]),
        ("css entity-encoded url", "<div style=\"background:&#117;rl(https://evil.example.com)\">x</div>", ["evil.example.com"]),
        ("background attribute", "<table background=https://evil.example.com/b.png><tr><td>x</td></tr></table>", ["evil.example.com", "background"]),
        ("comment hides tag", "<!--><script>alert(1)</script>-->after", ["alert", "<script"]),
        ("conditional comment", "<!--[if mso]><script>alert(1)</script><![endif]-->after", ["alert"]),
        ("cdata", "<![CDATA[<script>alert(1)</script>]]>after", ["<script"]),
        ("tag with no end", "text<a href=\"https://example.com/\" ", ["<a"]),
        ("lt in quoted attr", "<a title=\"<script>alert(1)</script>\" href=https://example.com/>x</a>", ["<script", "alert(1)</script"]),
        ("noscript", "<noscript><img src=x onerror=alert(1)></noscript>", ["onerror", "alert"]),
        ("template", "<template><script>alert(1)</script></template>", ["template", "alert"]),
        ("textarea closes tag", "<textarea></textarea><img src=x onerror=alert(1)>", ["onerror", "alert"]),
        ("title closes", "<title></title><script>alert(1)</script>", ["alert"]),
        ("id and name for clobbering", "<a id=cookie name=x href=https://example.com/>x</a>", ["id=", "name="]),
        ("class attribute", "<p class=\"x\">x</p>", ["class"]),
        ("target blank", "<a href=https://example.com/ target=_blank>x</a>", ["target"]),
    ]

    func testHostileTable() {
        for (name, input, banned) in hostile {
            let out = MailHTML.sanitized(input)
            let body = String(out[(out.range(of: "<body>")?.upperBound ?? out.startIndex)...])
            for b in banned { XCTAssertFalse(body.contains(b), "\(name): '\(b)' survived in: \(body)") }
        }
    }

    func testWhatIsKept() {
        let out = MailHTML.sanitized("""
        <html><head><title>t</title><style>p{}</style></head><body><h1>Hello</h1><p style="color:red;font-weight:bold">Some <b>bold</b> &amp; <i>fine</i> text, 1 < 2 > 0.</p>
        <a href="https://example.com/ok?a=1&amp;b=2">link</a> <a href="mailto:a@example.com">mail</a> <a href="#top">top</a>
        <img src="data:image/png;base64,AAAA" alt="inline" width="10"><table><tr><td colspan="2">cell</td></tr></table></body></html>
        """)
        for keep in ["<h1>Hello</h1>", "<b>bold</b>", "<i>fine</i>", "color:red;font-weight:bold", "&amp;", "href=\"https://example.com/ok?a=1&amp;b=2\"",
                     "href=\"mailto:a@example.com\"", "href=\"#top\"", "src=\"data:image/png;base64,AAAA\"", "alt=\"inline\"", "colspan=\"2\"", "cell",
                     "1 &lt; 2 &gt; 0", "Content-Security-Policy"] {
            XCTAssertTrue(out.contains(keep), "lost: \(keep)\n\(out)")
        }
        XCTAssertFalse(out.contains("<title"), "head content is dropped")
        XCTAssertFalse(out.contains("p{}"))
    }

    func testAnUnclosedHeadDoesNotSwallowTheMessage() {
        let out = MailHTML.sanitized("<html><head><title>x</title><body><p>the message</p></body></html>")
        XCTAssertTrue(out.contains("the message"))
    }

    func testOutputAttributesAreEscaped() {
        let out = MailHTML.sanitized("<p title='a\"b<c>&d'>x</p>")
        XCTAssertTrue(out.contains("title=\"a&quot;b&lt;c&gt;&amp;d\""), out)
    }

    func testEntityDecoding() {
        XCTAssertEqual(HTMLSanitizer.decodeEntities("&#106;&#x61;&colon;&Tab;&amp;&lt;&bogus;&#;&"), "ja:\t&<&bogus;&#;&")
        XCTAssertEqual(HTMLSanitizer.decodeEntities("&#0;&#x110000;"), "\u{FFFD}\u{FFFD}")
    }
}

// MARK: Small ones

final class SmallRepairTests: XCTestCase {
    func testUTF8TrimDropsOnlyAnUnfinishedSequence() {
        let e = Array("é".utf8), smile = Array("😀".utf8)       // 2 and 4 bytes
        func trim(_ b: [UInt8]) -> [UInt8] { Array(FilesService.withoutPartialUTF8(Data(b))) }
        XCTAssertEqual(trim(Array("ab".utf8) + [e[0]]), Array("ab".utf8), "a lone lead byte goes")
        XCTAssertEqual(trim(Array("ab".utf8) + e), Array("ab".utf8) + e, "a complete character at the cut stays")
        for k in 1..<4 { XCTAssertEqual(trim(Array("ab".utf8) + smile.prefix(k)), Array("ab".utf8), "\(k) of 4 bytes goes") }
        XCTAssertEqual(trim(Array("ab".utf8) + smile), Array("ab".utf8) + smile)
        XCTAssertEqual(trim(Array("abc".utf8)), Array("abc".utf8))
        XCTAssertEqual(trim([]), [])
        XCTAssertEqual(trim([0x80, 0x80]), [0x80, 0x80], "orphan continuation bytes with no lead are left to the decoder")
    }

    func testReadTextCutsMidCharacterWithoutReplacementCharacter() throws {
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("utf8-\(UUID().uuidString).txt")
        defer { try? FileManager.default.removeItem(at: url) }
        try Data((String(repeating: "a", count: 9) + "😀😀").utf8).write(to: url)     // 9 + 8 bytes
        for limit in 10...12 {
            let (s, cut) = FilesService.readText(url, limit: limit)
            XCTAssertTrue(cut)
            XCTAssertFalse(s.contains("\u{FFFD}"), "limit \(limit): \(s.unicodeScalars.map { $0.value })")
            XCTAssertEqual(s, String(repeating: "a", count: 9))
        }
        XCTAssertEqual(FilesService.readText(url, limit: 13).0, String(repeating: "a", count: 9) + "😀")
    }

    func testReaderVersionCountIsBounded() {
        let o = (try! JSONSerialization.jsonObject(with: Data(#"""
        {"publication":{"title":"T","kind":"story"},"route":{"bare":"t"},"version":{},"artifact":{"content":"# T\n\nBody."},
         "nav":{"revision":3,"latest":2000000000},"payload":{}}
        """#.utf8))) as! [String: Any]
        let d = TextureDocParser.parseBundle(o, address: "pub:t")
        XCTAssertLessThanOrEqual(d.versions.count, TextureDocParser.maxVersions)
        XCTAssertLessThanOrEqual(d.latest, TextureDocParser.maxVersions)
    }

    func testBadSampleRatesAreRefusedNotTrapped() {
        XCTAssertNil(monoBuffer([0.1, 0.2], rate: 0))
        XCTAssertNil(monoBuffer([0.1, 0.2], rate: -1))
        XCTAssertNil(monoBuffer([0.1, 0.2], rate: .nan))
        XCTAssertNil(pcmBuffer(AudioFrame(samples: [0.1], sampleRate: 0, t: 0)))
        XCTAssertNotNil(monoBuffer([], rate: 16_000), "an empty buffer is fine and does not trap on baseAddress")
    }
}
