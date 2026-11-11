import XCTest
@testable import AutoradioKit

// The failure list is at the top of Sources/AutoradioKit/ChoirMail.swift; each test names the number it covers.

final class OfflineProtocol: URLProtocol, @unchecked Sendable {
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() { client?.urlProtocol(self, didFailWithError: URLError(.notConnectedToInternet)) }
    override func stopLoading() {}
}

final class MailTests: XCTestCase {
    var env: ChoirEnvironment!
    var cal: Calendar { var c = Calendar(identifier: .gregorian); c.timeZone = TimeZone(identifier: "UTC")!; return c }

    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env = ChoirEnvironment()
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: "choir_sk_MAILTEST", KeyAccount.computerID: "comp-1"])
        env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init() }
    }

    // 8. threading
    func testNormalizedSubject() {
        XCTAssertEqual(MailText.normalizedSubject("Re: Re: Fwd: Q4 Plan"), "q4 plan")
        XCTAssertEqual(MailText.normalizedSubject("[list] RE: Q4 plan"), "q4 plan")
        XCTAssertEqual(MailText.normalizedSubject("   "), "")
        XCTAssertEqual(MailText.replySubject("Re: Hello"), "Re: Hello")
        XCTAssertEqual(MailText.replySubject("Hello"), "Re: Hello")
        XCTAssertEqual(MailText.replySubject(""), "Re:")
    }

    func summary(_ id: String, _ subject: String, at: String, read: Bool = true, dir: String = "inbound") -> MailSummary {
        var m = MailSummary(); m.id = id; m.subject = subject; m.at = at; m.direction = dir; m.fromAddress = "a@example.com"
        if read { m.readAt = at }
        return m
    }

    func testThreadsGroupAndEmptySubjectsNeverGroup() {
        let list = [summary("3", "Re: Plan", at: "2026-11-09T10:00:00Z", read: false), summary("2", "Plan", at: "2026-11-08T10:00:00Z"),
                    summary("1", "", at: "2026-11-07T10:00:00Z"), summary("0", "", at: "2026-11-06T10:00:00Z")]
        let rows = MailThreads.rows(list, now: ISO8601DateFormatter().date(from: "2026-11-10T00:00:00Z")!, calendar: cal)
        XCTAssertEqual(rows.count, 3)
        XCTAssertEqual(rows[0].id, "3"); XCTAssertEqual(rows[0].count, 2); XCTAssertTrue(rows[0].unread)
        XCTAssertEqual(rows[1].subject, "(no subject)")
        let t = MailThreads.thread(of: list[1], in: list)
        XCTAssertEqual(t.map(\.id), ["2", "3"])
        XCTAssertEqual(MailThreads.thread(of: list[2], in: list).count, 1)
    }

    // 5, 6. readable text
    func testHtmlToText() {
        let t = MailText.htmlToText("<html><head><style>p{}</style><script>x()</script></head><body><p>Hello &amp; <b>welcome</b></p><ul><li>One</li><li>Two&nbsp;</li></ul>a<br>b&#33;</body></html>")
        XCTAssertFalse(t.contains("x()")); XCTAssertFalse(t.contains("p{}"))
        XCTAssertTrue(t.contains("Hello & welcome")); XCTAssertTrue(t.contains("• One")); XCTAssertTrue(t.contains("a\nb!"))
    }

    func testBlocks() {
        let r = MailText.blocks(text: "Hello there,\nthis wraps over two long lines of text, which is how plain mail is written when it is sent.\n\nOn Monday Ana wrote:\n> first\n> second\n\n-- \nAna\nHarbour", html: "")
        XCTAssertFalse(r.htmlOnly)
        XCTAssertEqual(r.blocks.map(\.kind), ["p", "p", "quote", "sig"])
        XCTAssertFalse(r.blocks[0].text.contains("\n"))                       // re-flowed
        XCTAssertEqual(r.blocks[2].text, "first\nsecond")                      // short lines keep their breaks
        XCTAssertTrue(MailText.hasQuoted(r.blocks))
        let h = MailText.blocks(text: "", html: "<p>Only html</p>")
        XCTAssertTrue(h.htmlOnly); XCTAssertEqual(h.blocks.first?.text, "Only html")
        XCTAssertEqual(MailText.blocks(text: "", html: "").blocks, [MailBlock(kind: "p", text: "(no text)")])
        let big = MailText.blocks(text: String(repeating: "x ", count: 150_000), html: "")
        XCTAssertTrue(big.cut)
    }

    // 7. hostile HTML
    func testSanitizerStripsEverythingThatRunsOrLoads() {
        let hostile = """
        <html><head><meta http-equiv="refresh" content="0;url=https://x.example.com"><link rel="stylesheet" href="https://x.example.com/a.css"><script>alert(1)</script></head>
        <body onload="bad()"><iframe src="https://x.example.com"></iframe><object data="x"></object><embed src="y">
        <a href="javascript:alert(1)" onclick="bad()">Click</a><a href="https://example.com/ok">Fine</a>
        <img src="https://x.example.com/p.gif" alt="pixel"><img src="data:image/png;base64,AAAA" alt="inline">
        <div style="background:url(https://x.example.com/b.png)">styled</div><style>@import url(https://x.example.com/c.css); p{color:red}</style><form action="https://x.example.com"><input name=a></form></body></html>
        """
        let out = MailHTML.sanitized(hostile)
        for bad in ["<script", "alert(", "<iframe", "<object", "<embed", "<link", "http-equiv=\"refresh\"", "onload", "onclick", "javascript:",
                    "x.example.com", "@import", "<form", "<input"] {
            XCTAssertFalse(out.contains(bad), "survived: \(bad)")
        }
        XCTAssertTrue(out.contains("Content-Security-Policy"))
        XCTAssertTrue(out.contains("data:image/png;base64,AAAA"))
        XCTAssertTrue(out.contains("Fine")); XCTAssertTrue(out.contains("styled"))
    }

    // 12. dates
    func testDates() {
        let now = ISO8601DateFormatter().date(from: "2026-11-10T12:00:00Z")!
        XCTAssertEqual(MailDates.short("2026-11-10T08:12:00Z", now: now, calendar: cal), "08:12")
        XCTAssertEqual(MailDates.short("2026-11-08T08:12:00.250Z", now: now, calendar: cal), "Sun")
        XCTAssertEqual(MailDates.short("2026-10-30T08:12:00Z", now: now, calendar: cal), "30 Oct")
        XCTAssertEqual(MailDates.short("not a date at all, really long", now: now, calendar: cal), "not a date at al")
    }

    // 11. the draft refusal, 10. quoting
    func testProblemAndReplyBody() {
        XCTAssertEqual(MailService.problem(to: "", body: "hi"), "Add who it is for.")
        XCTAssertEqual(MailService.problem(to: "nobody", body: "hi"), "Check the address: nobody")
        XCTAssertEqual(MailService.problem(to: "ana@example.com", body: "\n\nOn Mon, X wrote:\n> q"), "Write something first.")
        XCTAssertNil(MailService.problem(to: "Ana <ana@example.com>, ben@example.com", body: "Thanks"))
        let body = MailText.replyBody(date: "d", who: "Ana", blocks: [MailBlock(kind: "p", text: (1...40).map { "line \($0)" }.joined(separator: "\n"))])
        XCTAssertEqual(body.components(separatedBy: "\n").filter { $0.hasPrefix("> ") }.count, 30)
    }

    func testAttachmentHelpers() {
        XCTAssertEqual(MailText.safeFilename("../../etc/passwd"), "passwd")
        XCTAssertEqual(MailText.safeFilename(""), "attachment")
        XCTAssertEqual(MailText.fileSize(482113), "471 KB")
        XCTAssertEqual(MailText.attachmentKind(filename: "a.PDF", contentType: ""), "pdf")
    }

    // 13. a sparse JSON message
    func testParseTolerant() {
        let d = MailParse.detail(["message": ["id": "x"]])
        XCTAssertEqual(d.summary.id, "x"); XCTAssertEqual(d.text, ""); XCTAssertTrue(d.to.isEmpty)
        XCTAssertEqual(MailThreads.row(d.summary).sender, "(unknown sender)")
    }

    // The service, against a stub of the computer
    func inboxJSON() -> [String: Any] {
        ["messages": [["id": "m1", "direction": "inbound", "from_address": "ana@example.com", "from_display": "Ana", "subject": "Hello", "snippet": "hi",
                       "trust_status": "trusted", "received_at": "2026-11-10T08:00:00Z", "created_at": "2026-11-10T08:00:00Z"]], "total": 1, "unread": 1]
    }

    func testBoxReadyEmptyAndError() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { r in r.path == "/api/email/messages" ? .init(json: self.inboxJSON()) : .init() }
        var b = await svc.box(address: "mailbox:inbox")
        XCTAssertEqual(b.status, "ready"); XCTAssertEqual(b.rows.count, 1); XCTAssertEqual(b.folders.first { $0.current }?.name, "inbox"); XCTAssertEqual(b.folders.first?.badge, "1")
        XCTAssertEqual(StubProtocol.seen.first?.query["folder"], "inbox")
        StubProtocol.handler = { _ in .init(json: ["messages": [], "total": 0, "unread": 0]) }
        b = await svc.box(address: "mailbox:sent")
        XCTAssertEqual(b.status, "empty"); XCTAssertFalse(b.message.isEmpty)
        StubProtocol.handler = { _ in .init(status: 400, json: ["error": "unsupported folder \"junk\""]) }
        b = await svc.box(address: "mailbox:junk")
        XCTAssertEqual(b.status, "error"); XCTAssertTrue(b.message.contains("unsupported folder"))
        StubProtocol.handler = { _ in .init(status: 401) }
        b = await svc.box(address: "mailbox:inbox")
        XCTAssertEqual(b.message, "Sign in again to read your mail.")
    }

    // 3. offline keeps the last good rows
    func testOfflineKeepsCachedRows() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { _ in .init(json: self.inboxJSON()) }
        _ = await svc.box(address: "mailbox:inbox")
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [OfflineProtocol.self]
        env.session = URLSession(configuration: cfg)
        let b = await svc.box(address: "mailbox:inbox")
        XCTAssertEqual(b.status, "offline"); XCTAssertTrue(b.cached); XCTAssertEqual(b.rows.count, 1)
        let fresh = await MailService(environment: env).box(address: "mailbox:inbox")
        XCTAssertEqual(fresh.status, "offline"); XCTAssertTrue(fresh.rows.isEmpty)
    }

    func detailJSON(dir: String = "inbound", replyTo: String? = nil) -> [String: Any] {
        var o: [String: Any] = ["message": ["id": "m1", "direction": dir, "from_address": "ana@example.com", "from_display": "Ana R.", "subject": "Hello", "trust_status": "trusted",
                                            "received_at": "2026-11-10T08:00:00Z", "created_at": "2026-11-10T08:00:00Z"],
                                "text_body": "Can you come on Thursday?", "recipients": ["to": [["address": "listener@example.com"]]],
                                "attachments": [["id": "a", "filename": "../plan.pdf", "content_type": "application/pdf", "size_bytes": 2048]]]
        if let replyTo { o["raw_headers"] = ["Reply-To": replyTo] }
        if dir == "outbound" { o["recipients"] = ["to": [["address": "ben@example.com", "display": "Ben"]]] }
        return o
    }

    func testMessageViewAndMarkRead() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { r in
            if r.path == "/api/email/messages/m1" { return .init(json: self.detailJSON()) }
            return .init(json: self.inboxJSON())
        }
        let m = await svc.message(address: "mail:m1")
        XCTAssertEqual(m.status, "ready"); XCTAssertEqual(m.subject, "Hello"); XCTAssertEqual(m.replyLabel, "Reply to Ana R.")
        XCTAssertEqual(m.attachments.first?.filename, "plan.pdf"); XCTAssertFalse(m.attachments.first?.available ?? true)   // 9
        XCTAssertEqual(m.thread.count, 1); XCTAssertTrue(m.spoken.hasPrefix("Message from Ana R.. Subject: Hello."))
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(StubProtocol.seen.contains { $0.method == "POST" && $0.path == "/api/email/messages/m1/read" })
        StubProtocol.handler = { _ in .init(status: 404, json: ["error": "not found"]) }
        let gone = await svc.message(address: "mail:zzz")
        XCTAssertEqual(gone.status, "error"); XCTAssertEqual(gone.message, "This message is gone.")
        XCTAssertEqual(await svc.message(address: "mail:new").status, "idle")
    }

    // 10. reply addressing
    func testReplyAddressing() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { r in r.path == "/api/email/messages/m1" ? .init(json: self.detailJSON(replyTo: "Ana R. <ana.replies@example.com>")) : .init(json: self.inboxJSON()) }
        var c = await svc.compose(address: "mail:reply/m1")
        XCTAssertEqual(c.status, "seed"); XCTAssertEqual(c.toLine, "ana.replies@example.com"); XCTAssertEqual(c.subject, "Re: Hello"); XCTAssertEqual(c.replyTo, "m1")
        XCTAssertTrue(c.body.contains("> Can you come on Thursday?"))
        StubProtocol.handler = { r in r.path == "/api/email/messages/m1" ? .init(json: self.detailJSON(dir: "outbound")) : .init(json: self.inboxJSON()) }
        c = await MailService(environment: env).compose(address: "mail:reply/m1")
        XCTAssertEqual(c.toLine, "ben@example.com")
    }

    // 11. saving a draft, the request, the missing alias, the approval link
    func testSaveDraftAndApproval() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { r in
            switch (r.method, r.path) {
            case ("GET", "/api/email/aliases"): return .init(json: ["aliases": [["address": "listener@example.com"]]])
            case ("POST", "/api/email/drafts"): return .init(status: 201, json: ["id": "d1", "status": "draft", "to_addresses": r.body["to_addresses"] ?? [String]()])
            case ("POST", "/api/email/drafts/d1/approval-email"): return .init(json: ["status": "sent", "review_url": "https://choir.news/mail/review/x"])
            default: return .init(status: 404)
            }
        }
        let refused = await svc.saveDraft(address: "mail:new", to: "", subject: "s", body: "b")
        XCTAssertEqual(refused.status, "error"); XCTAssertTrue(StubProtocol.seen.isEmpty, "a refused draft must make no request")
        let d = await svc.saveDraft(address: "mail:reply/m1", to: "Ana <ana@example.com>", subject: "Re: Hello", body: "Yes")
        XCTAssertEqual(d.status, "saved"); XCTAssertEqual(d.draftId, "d1")
        let post = StubProtocol.seen.first { $0.method == "POST" && $0.path == "/api/email/drafts" }
        XCTAssertEqual(post?.body["from_address"] as? String, "listener@example.com")
        XCTAssertEqual(post?.body["to_addresses"] as? [String], ["ana@example.com"])
        XCTAssertEqual(post?.body["reply_to_message_id"] as? String, "m1")
        XCTAssertFalse(StubProtocol.seen.contains { $0.path.hasSuffix("/send") }, "sending is an owner act; the phone never calls it")
        let a = await svc.approvalEmail(address: "mail:reply/m1", draft: "d1")
        XCTAssertEqual(a.status, "approval"); XCTAssertEqual(a.reviewURL, "https://choir.news/mail/review/x")
        StubProtocol.handler = { _ in .init(status: 403) }
        let denied = await svc.approvalEmail(address: "mail:reply/m1", draft: "d1")
        XCTAssertEqual(denied.status, "saved"); XCTAssertTrue(denied.note.contains("cannot ask"))
        StubProtocol.handler = { _ in .init(json: ["aliases": []]) }
        let noAlias = await MailService(environment: env).saveDraft(address: "mail:new", to: "a@example.com", subject: "s", body: "b")
        XCTAssertEqual(noAlias.status, "error"); XCTAssertTrue(noAlias.note.contains("no sending address"))
    }

    // 14. nothing private in the event log
    func testNoBodyOrKeyInTheLog() async {
        let log = EventLog(clock: LabClock()); env.log = log
        StubProtocol.handler = { r in r.path == "/api/email/messages/m1" ? .init(json: self.detailJSON()) : .init(json: self.inboxJSON()) }
        _ = await MailService(environment: env).message(address: "mail:m1")
        let dump = log.events.map { LabRuntime.describe($0.event) }.joined(separator: "\n")
        for secret in ["choir_sk_MAILTEST", "Can you come on Thursday", "ana@example.com", "Hello"] { XCTAssertFalse(dump.contains(secret), secret) }
    }

    // Demo routes answer from the fixtures
    func testDemoFixturesParse() {
        let inbox = MailDemo.answer(method: "GET", path: "/api/email/messages", query: [URLQueryItem(name: "folder", value: "inbox")], body: [:])
        XCTAssertEqual(inbox?.0, 200)
        let l = MailParse.list((try? JSONSerialization.jsonObject(with: inbox!.1)) as? [String: Any] ?? [:])
        XCTAssertEqual(l.rows.count, 6); XCTAssertEqual(l.unread, 2)
        for id in ["m1", "m2", "m3", "m4", "m5", "m6", "s1", "q1"] {
            let r = MailDemo.answer(method: "GET", path: "/api/email/messages/\(id)", query: [], body: [:])
            XCTAssertEqual(r?.0, 200, id)
        }
        XCTAssertEqual(MailDemo.answer(method: "GET", path: "/api/email/messages", query: [URLQueryItem(name: "folder", value: "junk")], body: [:])?.0, 400)
        XCTAssertNil(MailDemo.answer(method: "GET", path: "/api/platform/publications/front", query: [], body: [:]))
    }
}
