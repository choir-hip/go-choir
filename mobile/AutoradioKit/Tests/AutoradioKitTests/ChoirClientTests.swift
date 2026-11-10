import XCTest
@testable import AutoradioKit

/// A local stand-in for choir.news: every request goes to `handler`; no network.
final class StubProtocol: URLProtocol, @unchecked Sendable {
    struct Req { let method: String; let path: String; let query: [String: String]; let headers: [String: String]; let body: [String: Any] }
    struct Resp { var status = 200; var headers: [String: String] = [:]; var json: Any = [String: Any]() }
    nonisolated(unsafe) static var handler: (Req) -> Resp = { _ in Resp() }
    nonisolated(unsafe) static var seen: [Req] = []
    static let lock = NSLock()

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        var data = request.httpBody ?? Data()
        if let s = request.httpBodyStream {
            s.open(); var buf = [UInt8](repeating: 0, count: 4096)
            while s.hasBytesAvailable { let n = s.read(&buf, maxLength: buf.count); if n <= 0 { break }; data.append(buf, count: n) }
            s.close()
        }
        let comps = URLComponents(url: request.url!, resolvingAgainstBaseURL: false)!
        var q: [String: String] = [:]; for i in comps.queryItems ?? [] { q[i.name] = i.value ?? "" }
        var h: [String: String] = [:]; for (k, v) in request.allHTTPHeaderFields ?? [:] { h[k.lowercased()] = v }
        let req = Req(method: request.httpMethod ?? "GET", path: comps.path, query: q, headers: h,
                      body: (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] ?? [:])
        Self.lock.lock(); Self.seen.append(req); Self.lock.unlock()
        let r = Self.handler(req)
        let resp = HTTPURLResponse(url: request.url!, statusCode: r.status, httpVersion: "HTTP/1.1", headerFields: r.headers)!
        client?.urlProtocol(self, didReceive: resp, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: (try? JSONSerialization.data(withJSONObject: r.json)) ?? Data())
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}

let secretKey = "choir_sk_TESTKEY_do_not_leak"

final class ChoirClientTests: XCTestCase {
    var env: ChoirEnvironment!
    var log: EventLog!
    var keys: InMemoryKeyStore!

    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        log = EventLog(clock: LabClock())
        env = ChoirEnvironment.shared
        env.keys = keys; env.log = log; env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init() }
    }
    override func tearDown() { env.session = .shared }

    func noLeak(file: StaticString = #filePath, line: UInt = #line) {
        let dump = log.events.map { LabRuntime.describe($0.event) }.joined(separator: "\n")
        XCTAssertFalse(dump.contains(secretKey), "the key reached the event log", file: file, line: line)
        let enc = String(data: (try? JSONEncoder().encode(log.events)) ?? Data(), encoding: .utf8) ?? ""
        XCTAssertFalse(enc.contains(secretKey), file: file, line: line)
    }

    // MARK: System 1

    func testSystem1OneCallAllQuestions() async throws {
        StubProtocol.handler = { req in
            let names = (req.body["questions"] as? [String: Any])?.keys.sorted() ?? []
            var answers: [String: Any] = [:]
            for n in names { answers[n] = ["choice": "yes", "confidence": 0.8, "probabilities": ["yes": 0.8, "no": 0.2]] }
            return .init(json: ["answers": answers, "upstream_ms": 300, "total_ms": 320, "record_id": "rec-1"])
        }
        let s1 = ChoirSystem1(StageParams())
        let input = System1Input(context: "ctx", listenerWords: "what happened there", questions: Question.defaults)
        let out = try await s1.answer(input)
        XCTAssertEqual(out.map(\.questionID), Question.defaults.map(\.id))   // input order, not map order
        XCTAssertEqual(out.first?.score, 0.8)
        let calls = StubProtocol.seen.filter { $0.path == "/api/system1/judgments" }
        XCTAssertEqual(calls.count, 1, "one call for all questions")
        XCTAssertEqual((calls[0].body["questions"] as? [String: Any])?.count, 3)
        XCTAssertEqual(calls[0].headers["authorization"], "Bearer \(secretKey)")
        XCTAssertEqual(calls[0].headers["x-choir-computer"], "comp-1")
        XCTAssertNotNil((calls[0].body["state"] as? [String: Any])?["listener_words"])
        XCTAssertTrue(log.events.contains { if case .stageTiming(let st, _, _) = $0.event { return st == "choir POST /api/system1/judgments" }; return false })
        noLeak()
    }

    func test429IsAFailedStageNoRetry() async {
        StubProtocol.handler = { _ in .init(status: 429, headers: ["Retry-After": "1"], json: ["error": "judgment rate limit exceeded"]) }
        do { _ = try await ChoirSystem1(StageParams()).answer(System1Input(context: "", listenerWords: "x", questions: Question.defaults)); XCTFail("should throw") }
        catch { XCTAssertEqual(error as? ChoirError, .rateLimited(retryAfter: 1)) }
        XCTAssertEqual(StubProtocol.seen.filter { $0.path == "/api/system1/judgments" }.count, 1, "no retry")
    }

    func test502AndNoKey() async {
        StubProtocol.handler = { _ in .init(status: 502, json: ["error": "upstream rejected"]) }
        do { _ = try await ChoirSystem1(StageParams()).answer(System1Input(context: "", listenerWords: "x", questions: Question.defaults)); XCTFail() }
        catch { XCTAssertEqual(error as? ChoirError, .server(status: 502, message: "upstream rejected")) }
        keys.delete(KeyAccount.apiKey)
        do { _ = try await ChoirSystem1(StageParams()).answer(System1Input(context: "", listenerWords: "x", questions: Question.defaults)); XCTFail() }
        catch { XCTAssertEqual(error as? ChoirError, .notSignedIn) }
    }

    func testQuestionsClippedToRouteLimits() throws {
        let many = (0..<20).map { Question(id: "q\($0)", text: String(repeating: "t", count: 3000), options: (0..<12).map { "o\($0)" }) }
        let (body, names) = try ChoirSystem1.body(for: System1Input(context: String(repeating: "c", count: 100_000), listenerWords: "w", questions: many), label: "autoradio")
        XCTAssertEqual(names.count, 12)
        let qs = body["questions"] as! [String: [String: Any]]
        XCTAssertTrue(qs.values.allSatisfy { ($0["choices"] as! [String]).count == 8 && ($0["instructions"] as! String).utf8.count <= 2000 })
        XCTAssertLessThan(try JSONSerialization.data(withJSONObject: body["state"]!).count, 48 << 10)
    }

    // MARK: Computer discovery

    func testOwnerWideKeyDiscoversItsComputer() async throws {
        keys.delete(KeyAccount.computerID)
        StubProtocol.handler = { req in
            if req.path == "/api/compute/status" { return .init(json: ["computers": [["computer_id": "comp-9", "kind": "interactive", "current": true]]]) }
            return .init(json: ["answers": [:]])
        }
        let c = env.client()
        try await c.send("GET", "/api/anything")
        XCTAssertEqual(keys.get(KeyAccount.computerID), "comp-9")
        XCTAssertEqual(StubProtocol.seen.last?.headers["x-choir-computer"], "comp-9")
        XCTAssertNil(StubProtocol.seen.first?.headers["x-choir-computer"], "status is read without naming a computer")
    }

    // MARK: Device flow

    func testDeviceFlowStoresKeyAndNeverShowsIt() async throws {
        keys.delete(KeyAccount.apiKey); keys.delete(KeyAccount.computerID)
        nonisolated(unsafe) var polls = 0
        StubProtocol.handler = { req in
            switch req.path {
            case "/auth/device/begin": return .init(json: ["device_code": "choir_dc_x", "user_code": "ABCD-EFGH", "verification_uri": "https://choir.news/device",
                                                           "verification_uri_complete": "https://choir.news/device?user_code=ABCD-EFGH", "expires_in": 600, "interval": 5])
            case "/auth/device/token":
                polls += 1
                if polls == 1 { return .init(status: 400, json: ["error": "authorization_pending"]) }
                if polls == 2 { return .init(status: 400, json: ["error": "slow_down"]) }
                return .init(json: ["api_key": secretKey, "key_id": "k1", "scopes": ["read:runtime"]])
            case "/api/compute/status": return .init(json: ["computers": [["computer_id": "comp-2", "kind": "interactive"]]])
            default: return .init()
            }
        }
        nonisolated(unsafe) var opened: URL?
        let si = ChoirSignIn(env: env)
        si.sleep = { _ in }
        si.openURL = { opened = $0 }
        let st = await si.begin()
        XCTAssertEqual(st.userCode, "ABCD-EFGH"); XCTAssertEqual(opened?.absoluteString, "https://choir.news/device?user_code=ABCD-EFGH")
        await si.wait()
        XCTAssertEqual(keys.get(KeyAccount.apiKey), secretKey)
        XCTAssertEqual(keys.get(KeyAccount.computerID), "comp-2")
        XCTAssertTrue(si.current.signedIn)
        let shown = String(data: try JSONEncoder().encode(si.current), encoding: .utf8)!
        XCTAssertFalse(shown.contains(secretKey))
        noLeak()
        XCTAssertEqual(polls, 3)
    }

    func testDeviceFlowDenied() async {
        keys.delete(KeyAccount.apiKey)
        StubProtocol.handler = { req in
            req.path == "/auth/device/begin" ? .init(json: ["device_code": "d", "user_code": "U", "verification_uri_complete": "https://choir.news/device?u=U", "expires_in": 600, "interval": 1])
                                             : .init(status: 400, json: ["error": "access_denied"])
        }
        let si = ChoirSignIn(env: env); si.sleep = { _ in }
        _ = await si.begin(); await si.wait()
        XCTAssertEqual(si.current.status, "failed"); XCTAssertFalse(si.current.signedIn)
    }

    func testSignOutRevokesAndForgets() async {
        try? keys.set("k1", account: KeyAccount.keyID)
        let si = ChoirSignIn(env: env)
        await si.signOut()
        XCTAssertNil(keys.get(KeyAccount.apiKey)); XCTAssertNil(keys.get(KeyAccount.computerID))
        XCTAssertEqual(StubProtocol.seen.first?.method, "DELETE")
        XCTAssertEqual(StubProtocol.seen.first?.path, "/auth/api-keys/k1")
    }

    // MARK: Programme

    func frontHandler(_ req: StubProtocol.Req) -> StubProtocol.Resp {
        switch req.path {
        case "/api/platform/publications/front":
            return .init(json: ["stories": [["title": "Harbour", "dek": "d", "route": ["bare": "/p/harbour"], "section": "world", "words": 40]]])
        case "/api/platform/publications/resolve":
            return .init(json: ["publication": ["title": "Harbour"], "version": ["id": "pv-1", "manifest_hash": "mh-1"],
                                "payload": ["text": "# Harbour\n\nThe cranes moved inland [1].\n\nResidents said the **quiet** was strange. Then they stopped noticing it."]])
        default: return .init(status: 404, json: ["error": "nope"])
        }
    }

    func testFrontProgrammeLoadsBlocksWithoutAKey() async throws {
        keys.delete(KeyAccount.apiKey)
        StubProtocol.handler = frontHandler
        let doc = try await ChoirFrontProgramme(StageParams()).load()
        XCTAssertEqual(doc.blocks.map { $0.spokenText ?? "" }, ["Harbour", "The cranes moved inland.", "Residents said the quiet was strange. Then they stopped noticing it."])
        XCTAssertEqual(env.playing(doc.id)?.publicationVersionID, "pv-1")
        XCTAssertTrue(StubProtocol.seen.allSatisfy { $0.headers["authorization"] == nil })
    }

    func testEmptyFrontPage() async {
        StubProtocol.handler = { _ in .init(json: ["stories": []]) }
        do { _ = try await ChoirFrontProgramme(StageParams()).load(); XCTFail() } catch { XCTAssertEqual(error as? ChoirError, .empty("the front page")) }
    }

    // MARK: Desk

    func testDeskForksThenFollowsStreamAndSpeaksOnlyTheDesksRevision() async throws {
        StubProtocol.handler = frontHandler
        let doc = try await ChoirFrontProgramme(StageParams()).load()
        nonisolated(unsafe) var streamPolls = 0
        StubProtocol.handler = { req in
            switch req.path {
            case "/api/texture/fork": return .init(status: 202, json: ["doc_id": "doc-1", "revision_id": "rev-1"])
            case "/api/texture/revisions/rev-1": return .init(json: ["revision_id": "rev-1", "author_kind": "user", "version_number": 2, "content": "x"])
            case "/api/texture/revisions/rev-2":
                return .init(json: ["revision_id": "rev-2", "author_kind": "appagent", "version_number": 3,
                                    "content": "Residents said the quiet was strange.\n\nA new paragraph the desk wrote."])
            case "/api/texture/documents/doc-1/stream":
                streamPolls += 1
                if streamPolls == 1 { return .init(json: ["events": [["event_type": "version", "revision_id": "rev-1", "work_state": "working"]], "next_cursor": 1]) }
                return .init(json: ["events": [["event_type": "version", "revision_id": "rev-2", "work_state": "terminal"]], "next_cursor": 2])
            default: return .init(status: 404, json: ["error": "x"])
            }
        }
        let desk = ChoirDeskStage(StageParams(["pollSeconds": .number(0.05)]))
        var revs: [Revision] = []
        for try await r in desk.submit(DeskTurn(turnID: "t1", programmeID: doc.id, blockIndex: 1, listenerWords: "Tell me more about the residents")) { revs.append(r) }
        XCTAssertEqual(revs.count, 1)
        XCTAssertEqual(revs[0].id, "rev-2"); XCTAssertFalse(revs[0].keepRest)
        XCTAssertGreaterThanOrEqual(revs[0].fromBlock, 2)
        let fork = StubProtocol.seen.first { $0.path == "/api/texture/fork" }!
        XCTAssertTrue((fork.body["text"] as! String).hasPrefix("Tell me more about the residents"))   // raw words first
        XCTAssertEqual((fork.body["source"] as? [String: String])?["manifest_hash"], "mh-1")
        XCTAssertEqual(env.playing(doc.id)?.docID, "doc-1")
        // Second turn: prompt bar with the document as focus.
        StubProtocol.handler = { req in
            if req.path == "/api/prompt-bar" { return .init(status: 202, json: ["doc_id": "doc-1", "revision_id": "rev-3"]) }
            return .init(json: ["events": [], "next_cursor": 2, "revision_id": "x", "author_kind": "user", "version_number": 4])
        }
        let d2 = ChoirDeskStage(StageParams(["pollSeconds": .number(0.02), "timeoutSeconds": .number(5)]))
        let t = Task { var n = 0; do { for try await _ in d2.submit(DeskTurn(turnID: "t2", programmeID: doc.id, blockIndex: 0, listenerWords: "again")) { n += 1 } } catch {}; return n }
        try await Task.sleep(nanoseconds: 300_000_000); t.cancel()
        let pb = StubProtocol.seen.first { $0.path == "/api/prompt-bar" }!
        XCTAssertEqual((pb.body["focus"] as? [String: String])?["object_ref"], "doc-1")
        XCTAssertEqual((pb.body["focus"] as? [String: String])?["app_agent_id"], "texture")
        noLeak()
    }

    func testDeskStreamDropsThreeTimesThenFails() async {
        env.setPlaying(PlayingProgramme(programmeID: "p", title: "T", route: "/r", publicationVersionID: "", manifestHash: "", blocks: ["a"], docID: "doc-1"))
        StubProtocol.handler = { req in
            if req.path == "/api/prompt-bar" { return .init(status: 202, json: ["doc_id": "doc-1", "revision_id": "r1"]) }
            if req.path.hasSuffix("/stream") { return .init(status: 502, json: ["error": "bad gateway"]) }
            return .init(json: ["version_number": 1])
        }
        let desk = ChoirDeskStage(StageParams(["pollSeconds": .number(0.01)]))
        do { for try await _ in desk.submit(DeskTurn(turnID: "t", programmeID: "p", blockIndex: 0, listenerWords: "hi")) {}; XCTFail() }
        catch { XCTAssertEqual(error as? ChoirError, .server(status: 502, message: "bad gateway")) }
    }

    func testTextSplitting() {
        XCTAssertEqual(ChoirText.paragraphs("## Head\n\n- one\n- two\n\n[link](http://x) and [3] text"), ["Head", "one two", "link and text"])
        let long = (0..<30).map { "This is sentence number \($0)." }.joined(separator: " ")
        XCTAssertTrue(ChoirText.paragraphs(long).allSatisfy { $0.count <= 600 })
    }

    func testFileKeyStoreReadOnlyKeyFile() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("k-\(UUID().uuidString)")
        try "  choir_sk_abc \n".write(to: f, atomically: true, encoding: .utf8)
        defer { try? FileManager.default.removeItem(at: f) }
        let ks = FileKeyStore(url: f, readOnlyKeyFile: true)
        XCTAssertEqual(ks.get(KeyAccount.apiKey), "choir_sk_abc")
        XCTAssertNil(ks.get(KeyAccount.computerID))
        XCTAssertThrowsError(try ks.set("x", account: KeyAccount.computerID))
    }
}
