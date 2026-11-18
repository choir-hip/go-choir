import XCTest
@testable import AutoradioKit

// The failure list is at the top of Sources/AutoradioKit/MailboxTabs.swift; each test names the number it covers.
final class MailboxTabsTests: XCTestCase {
    var env: ChoirEnvironment!
    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env = ChoirEnvironment()
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: "choir_sk_MAILTEST", KeyAccount.computerID: "comp-1"])
        env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init() }
    }
    func list(_ n: Int, unread: Int = 0) -> [String: Any] {
        ["messages": (0..<n).map { ["id": "m\($0)", "direction": "inbound", "from_address": "ana@example.com", "from_display": "Ana", "subject": "Subject \($0)", "snippet": "hi",
                                    "trust_status": "trusted", "received_at": "2026-11-10T08:00:00Z", "created_at": "2026-11-10T08:00:00Z"] }, "total": n, "unread": unread]
    }
    func tabs(_ s: MailboxTabsState) -> [String] { s.frame.folders.map(\.label) }
    func assertNoWaitingWords(_ s: MailboxTabsState, file: StaticString = #filePath, line: UInt = #line) {
        for w in s.words { XCTAssertFalse(w.lowercased().contains("loading") || w.lowercased().contains("opening"), w, file: file, line: line) }
    }

    // 1: the frame is the same before, during and after a load; the counts only arrive.
    func testFrameIsPresentBeforeDuringAndAfter() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { r in .init(json: self.list(r.query["folder"] == "inbox" ? 3 : 1, unread: r.query["folder"] == "inbox" ? 98 : 0)) }
        await MailboxTabs(service: svc).open("inbox")                       // the inbox count is known
        let m = MailboxTabs(service: svc, folder: "inbox")
        let before = m.state
        XCTAssertEqual(tabs(before), ["Inbox", "Drafts", "Sent", "Quarantine"])
        XCTAssertEqual(before.frame.folders.first?.badge, "98")
        let g = m.select("quarantine")
        let during = m.state
        XCTAssertEqual(tabs(during), tabs(before)); XCTAssertEqual(during.frame.folders.first?.badge, "98")
        XCTAssertEqual(during.frame.folders.first { $0.current }?.name, "quarantine")
        let qv = await svc.box(address: "mailbox:quarantine")
        XCTAssertTrue(m.accept(generation: g, view: qv))
        let after = m.state
        XCTAssertEqual(tabs(after), tabs(before)); XCTAssertEqual(after.frame.folders.first?.badge, "98"); XCTAssertEqual(after.frame.title, "Quarantine")
        XCTAssertEqual(after.rows.count, 1)
        for s in [before, during, after] { assertNoWaitingWords(s) }
    }

    // 1: the frame needs no fetch at all, so a first visit has it too.
    func testFrameWithNothingFetched() {
        let m = MailboxTabs(service: MailService(environment: env), folder: "sent")
        XCTAssertEqual(tabs(m.state), ["Inbox", "Drafts", "Sent", "Quarantine"])
        XCTAssertTrue(m.state.showsSkeleton); XCTAssertTrue(m.state.rows.isEmpty); assertNoWaitingWords(m.state)
        XCTAssertEqual(StubProtocol.seen.count, 0)
    }

    // 2: a tab tapped during a load wins; the slower answer is discarded.
    func testTapDuringLoadWinsAndStaleAnswerIsDiscarded() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { r in .init(json: self.list(r.query["folder"] == "sent" ? 5 : 2)) }
        let m = MailboxTabs(service: svc)
        let g1 = m.select("sent")
        let g2 = m.select("quarantine")
        let sentView = await svc.box(address: "mailbox:sent")
        XCTAssertFalse(m.accept(generation: g1, view: sentView), "the stale answer is refused")
        XCTAssertEqual(m.state.selected, "quarantine"); XCTAssertTrue(m.state.showsSkeleton); XCTAssertTrue(m.state.rows.isEmpty)
        let qv = await svc.box(address: "mailbox:quarantine")
        XCTAssertTrue(m.accept(generation: g2, view: qv))
        XCTAssertEqual(m.state.rows.count, 2); XCTAssertEqual(m.state.frame.title, "Quarantine")
    }

    // 3: the transition state changes on the action itself, before any fetch completes (no network is touched by `select`).
    func testTransitionStartsSynchronouslyOnTheTap() {
        let m = MailboxTabs(service: MailService(environment: env), folder: "inbox")
        XCTAssertEqual(m.state.phase, .settled); XCTAssertEqual(m.state.direction, 0)
        m.select("sent")
        XCTAssertEqual(m.state.phase, .leaving, "the outgoing content is already leaving")
        XCTAssertEqual(m.state.direction, 1, "to the right of the inbox")
        XCTAssertTrue(m.state.showsSkeleton)
        XCTAssertEqual(StubProtocol.seen.count, 0, "no fetch has finished, none has even been needed")
        m.select("drafts")
        XCTAssertEqual(m.state.direction, -1)
    }

    // 3 + 4: a folder seen before runs the whole transition at once, and refreshes underneath.
    func testCachedTabEntersAtOnce() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { _ in .init(json: self.list(4)) }
        await MailboxTabs(service: svc).open("sent")
        let m = MailboxTabs(service: svc, folder: "inbox")
        m.select("sent")
        XCTAssertEqual(m.state.phase, .entering); XCTAssertFalse(m.state.showsSkeleton); XCTAssertEqual(m.state.rows.count, 4)
        m.settle(); XCTAssertEqual(m.state.phase, .settled)
    }

    // An error is still said in words, with the frame kept.
    func testErrorIsWordsAndTheFrameStays() async {
        let svc = MailService(environment: env)
        StubProtocol.handler = { _ in .init(status: 500, json: ["error": "boom"]) }
        let m = MailboxTabs(service: svc)
        await m.open("sent")
        XCTAssertEqual(m.state.status, "error"); XCTAssertFalse(m.state.message.isEmpty)
        XCTAssertEqual(tabs(m.state), ["Inbox", "Drafts", "Sent", "Quarantine"]); assertNoWaitingWords(m.state)
    }
}
