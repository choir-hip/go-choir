import XCTest
@testable import AutoradioKit

// Failure list (2026-11-17, owner: "Read my book" answered "the page is empty and I have no access to any files"):
//  1. The server desk is sent an empty page when the page in front is the Autoradio page, and none of what the phone retrieved.
//  2. "that" and "this" search for the title of the Autoradio page instead of what is on air.
//  3. "Read my book", "play the latest <show>", "what is in my mail" are taken for questions and go to a model.
//  4. A question that contains those words ("who wrote the book I should read") is taken for a command.
//  5. The prompt the on-device model is shown cannot be seen: a test prints it for a store of books, stories and podcasts.
//  6. The turn leaves no line saying who answered and what was retrieved (counts only, never content).

final class TurnContextTests: XCTestCase {
    static let now = Date(timeIntervalSince1970: 1_794_700_000)
    func fixtureStore() -> (ContentStore, SavedCorpus) {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("tc-\(UUID().uuidString)")
        let s = ContentStore(root: root, clock: { Self.now }); let c = SavedCorpus(readerStore: nil)
        c.setDeltaSource("store") { s.drain() }
        func day(_ n: Double) -> Date { Self.now.addingTimeInterval(-n * 86400) }
        s.put(StoredItem(id: "pub:/p/water", kind: .story, title: "Senate passes the water bill", text: "The Senate voted 61 to 38 on Tuesday to pass the water bill after a long debate on drought funding.", date: day(1), address: "pub:/p/water", source: "Autopaper", syncedAt: Self.now))
        s.put(StoredItem(id: "pub:/p/budget", kind: .story, title: "Senate committee delays budget hearing", text: "A Senate committee postponed its budget hearing until next week.", date: day(3), address: "pub:/p/budget", source: "Autopaper", syncedAt: Self.now))
        s.put(StoredItem(id: "pub:/p/harbor", kind: .story, title: "Harbor strike ends", text: "Dock workers ended the strike on Tuesday.", date: day(2), address: "pub:/p/harbor", source: "Autopaper", syncedAt: Self.now))
        s.put(StoredItem(id: "pc:ep1", kind: .podcast, title: "World News: The Senate week", text: "This episode covers the Senate and the budget.", date: day(1), address: "podcast:episode/x/ep1", source: "Podcasts", syncedAt: Self.now))
        c.setSource("library") { [SavedCorpus.Document(id: "book:b1", title: "The Keeper of Lamps", address: "doc:b1", text: "", kind: "book", source: "Reader", date: day(30),
                                                         parts: [.init(label: "chapter 1: The shore", text: "The keeper walked the shore at dusk and counted the boats coming in.", block: 3)])] }
        return (s, c)
    }

    func testThePromptTheOnDeviceModelIsShownForTheSenate() async {
        let (_, c) = fixtureStore()
        let m = FakeLocalModel(pieces: ["The Senate passed the water bill. "])
        let h = SavedAnswerHolder(StageParams(), corpus: c, model: { m })
        for await _ in h.lines(for: HolderInput(listenerWords: "tell me about the Senate", answers: [], pageTitle: "Autoradio", pageAddress: "autoradio:home")) {}
        let prefix = m.asked.first?.prefix ?? "", suffix = m.asked.first?.suffix ?? ""
        print("---- PROMPT (system) ----\n\(m.asked.first?.system ?? "")\n---- PROMPT (context) ----\n\(prefix)---- PROMPT (question) ----\n\(suffix)")
        XCTAssertTrue(prefix.contains("Senate passes the water bill") && prefix.contains("Senate committee delays"), "the Senate stories are in the context")
        XCTAssertTrue(prefix.contains("World News: The Senate week"), "and the podcast episode")
        XCTAssertFalse(prefix.contains("Harbor strike"), "and nothing unrelated")
        XCTAssertLessThan(prefix.range(of: "water bill")!.lowerBound, prefix.range(of: "budget hearing")!.lowerBound, "newest first")
    }
    func testReadMyBookIsACommandNotAQuestionAndTheQuestionPathStillSeesTheBook() {
        let (_, c) = fixtureStore()
        let book = ShelfDoc(id: "b1", title: "The Keeper of Lamps", author: "", kind: "epub", added: Self.now, position: 42, length: 100, sections: 3, fileName: "k.epub")
        let cat = LocalCatalog(books: { [book] })
        XCTAssertEqual(LocalCommands.match("Read my book", catalog: cat), .readBook(id: "b1", title: "The Keeper of Lamps", block: 42))
        XCTAssertEqual(LocalCommands.match("read my book to me please", catalog: cat)?.name, "read-book")
        XCTAssertEqual(LocalCommands.match("continue the keeper of lamps", catalog: cat)?.name, "read-book")
        XCTAssertNil(LocalCommands.match("who wrote the book I should read", catalog: cat), "a question is not a command")
        XCTAssertNil(LocalCommands.match("read the news", catalog: cat))
        XCTAssertNil(LocalCommands.match("read my book", catalog: LocalCatalog()), "no book on the phone: the desk answers")
        print("---- 'Read my book' as a question would be given: ----\n\(c.retrieve("Read my book", onAir: "").text)")
    }
    func testSeveralBooksChooseTheOneBeingReadThenTheNewest() {
        func b(_ id: String, _ t: String, added: Double, pos: Int) -> ShelfDoc { ShelfDoc(id: id, title: t, author: "", kind: "epub", added: Date(timeIntervalSince1970: added), position: pos, length: 10, sections: 1, fileName: id) }
        let cat = LocalCatalog(books: { [b("a", "Alpha Tides", added: 1, pos: 5), b("b", "Beta Winds", added: 9, pos: 0), b("c", "Gamma Seas", added: 5, pos: 3)] })
        XCTAssertEqual(LocalCommands.match("read my book", catalog: cat), .readBook(id: "c", title: "Gamma Seas", block: 3), "the most recent of those with progress")
        XCTAssertEqual(LocalCommands.match("read beta winds", catalog: cat)?.name, "read-book")
        XCTAssertEqual(LocalCommands.match("read my book", catalog: LocalCatalog(books: { [b("b", "Beta Winds", added: 9, pos: 0), b("a", "Alpha", added: 1, pos: 0)] })), .readBook(id: "b", title: "Beta Winds", block: 0))
    }
    func testPlayTheLatestShowAndMailCommandsResolveFromTheStores() {
        let show = PodcastShow(feedURL: "https://example.com/f.xml", title: "Reuters World News")
        let feed = PodcastFeed(show: show, episodes: [PodcastEpisode(id: "old", title: "Old", published: 100, enclosureURL: "https://example.com/a.mp3"), PodcastEpisode(id: "new", title: "New", published: 200, enclosureURL: "https://example.com/b.mp3")])
        var row = MailSummary(); row.id = "m1"; row.subject = "Tunnel vote"; row.fromDisplay = "Ana Example"; row.at = "2026-11-14T09:00:00Z"
        var read = MailSummary(); read.id = "m0"; read.subject = "Old"; read.at = "2026-11-01T09:00:00Z"; read.readAt = "2026-11-02T00:00:00Z"
        let cat = LocalCatalog(shows: { [show] }, feed: { _ in feed }, inbox: { [read, row] })
        XCTAssertEqual(LocalCommands.match("play the latest Reuters", catalog: cat), .playEpisode(address: PodcastService.episodeAddress(feed: show.feedURL, id: "new"), title: "Reuters World News: New"))
        XCTAssertNil(LocalCommands.match("play the latest zebra show", catalog: cat))
        let mail = LocalCommands.match("what is in my mail", catalog: cat)
        XCTAssertEqual(mail, .readMail(address: "mail:m1", unread: 1, total: 2, from: "Ana Example", subject: "Tunnel vote"))
        XCTAssertTrue(mail!.line.hasPrefix("You have 1 unread of 2"), mail!.line)
        XCTAssertEqual(LocalCommands.match("read my email", catalog: LocalCatalog())?.line, "I have no mail saved on this phone yet.")
        XCTAssertNil(LocalCommands.match("tell me why my email provider changed", catalog: cat))
    }
    func testOnAirIsTheContentPageOrWhatTheRunSheetHasOnAir() {
        XCTAssertEqual(OnAir.title(pageAddress: "pub:/p/x", pageTitle: "Water bill", hint: "Harbor"), "Water bill")
        XCTAssertEqual(OnAir.title(pageAddress: "autoradio:home", pageTitle: "Autoradio", hint: "Harbor strike ends"), "Harbor strike ends")
        XCTAssertEqual(OnAir.title(pageAddress: "autoradio:home", pageTitle: "Autoradio", hint: ""), "", "never the Autoradio page's own name")
    }
    func testTheDeskIsNeverSentAnEmptyPage() {
        let (_, c) = fixtureStore()
        let ctx = c.retrieve("tell me about the Senate")
        let auto = DeskContext.page(front: FrontPage(address: "autoradio:home", title: "Autoradio", kind: "autoradio"), pageText: "", brief: "On air now: Harbor strike ends. Up next: Water.", retrieved: ctx)!
        XCTAssertTrue(auto.text.hasPrefix("On air now: Harbor strike ends."), auto.text); XCTAssertTrue(auto.text.contains("Senate passes the water bill")); XCTAssertEqual(auto.address, "autoradio:home")
        let story = DeskContext.page(front: FrontPage(address: "pub:/p/x", title: "X", kind: "story"), pageText: "Own words of the page.", brief: "On air now: Y.", retrieved: ctx)!
        XCTAssertTrue(story.text.hasPrefix("Own words of the page.")); XCTAssertFalse(story.text.contains("On air now"), "a content page speaks for itself"); XCTAssertTrue(story.text.contains("Found on the listener's phone"))
        XCTAssertNil(DeskContext.page(front: nil, pageText: "", brief: "", retrieved: nil))
        XCTAssertEqual(DeskContext.page(front: FrontPage(address: "autoradio:home", title: "A"), pageText: "", brief: "", retrieved: nil)?.text, "", "nothing to add: unchanged")
    }
    func testTheTurnLineHoldsCountsAndKindsOnly() {
        let (_, c) = fixtureStore()
        let line = DeskContext.turnLine(desk: "server", retrieved: c.retrieve("tell me about the Senate"), command: "none")
        XCTAssertTrue(line.hasPrefix("AUTORADIO turn desk=server retrieved=3 sources=podcast,story context_chars="), line)
        XCTAssertTrue(line.hasSuffix("command=none")); XCTAssertFalse(line.contains("Senate"))
        XCTAssertEqual(DeskContext.turnLine(desk: "local", retrieved: nil, command: "read-book"), "AUTORADIO turn desk=local retrieved=0 sources=- context_chars=0 command=read-book")
    }
}

extension SavedAnswerTests {
    func testATypedReadMyBookRunsTheCommandAndNeverReachesTheDesk() async throws {
        ScriptedStreamingDesk.requests = []; ScriptedStreamingDesk.sentences = ["I cannot read your book."]
        let book = ShelfDoc(id: "b1", title: "The Keeper of Lamps", author: "", kind: "epub", added: Date(), position: 7, length: 100, sections: 1, fileName: "k.epub")
        let did = Locked<[LocalCommand]>([])
        var hooks = QueueVoice.hooks
        hooks.catalog = { LocalCatalog(books: { [book] }) }
        hooks.actOnCommand = { cmd in did.value.append(cmd); return true }
        hooks.perform = { _, _ in false }
        let saved = QueueVoice.hooks; QueueVoice.hooks = hooks; defer { QueueVoice.hooks = saved }
        let (e, _, _) = try engine(desk: "scripted-stream-desk", model: nil, corpus: corpus())
        _ = await e.typedTurn("Read my book", front: FrontPage(address: "autoradio:home", title: "Autoradio", kind: "autoradio"))
        XCTAssertEqual(did.value, [.readBook(id: "b1", title: "The Keeper of Lamps", block: 7)])
        XCTAssertTrue(ScriptedStreamingDesk.requests.isEmpty, "no model and no server desk for a command")
        XCTAssertEqual(Conversation.shared.entries().last?.answer, "Okay, reading The Keeper of Lamps from where you left off.")
    }
    func testTheServerDeskIsSentWhatIsOnAirAndWhatThePhoneRetrieved() async throws {
        ScriptedStreamingDesk.requests = []; ScriptedStreamingDesk.sentences = ["Ok."]; ScriptedStreamingDesk.hang = false
        let (e, _, got) = try engine(desk: "scripted-stream-desk", model: nil, corpus: corpus(), deskParams: ["delay": .number(0.05)])
        _ = await e.typedTurn("what happened with the harbor strike", front: FrontPage(address: "autoradio:home", title: "Autoradio", kind: "autoradio"))
        try await waitDone(got)
        let page = try XCTUnwrap(ScriptedStreamingDesk.requests.last?.page)
        XCTAssertFalse(page.text.isEmpty, "never an empty page"); XCTAssertTrue(page.text.contains("Harbor strike ends after nine days"), page.text)
        XCTAssertEqual(page.address, "autoradio:home")
    }
}

final class PrivateTurnTests: XCTestCase {
    func ctx(_ docs: [SavedCorpus.Document], _ q: String) -> RetrievalContext {
        let c = SavedCorpus(readerStore: nil); c.setSource("t") { docs }; return c.retrieve(q)
    }
    let story = SavedCorpus.Document(id: "pub:s", title: "Tunnel referendum result", address: "pub:/s", text: "The tunnel referendum passed.", kind: "story", source: "Autopaper", date: Date(timeIntervalSince1970: 1_794_000_000))
    let mail = SavedCorpus.Document(id: "mail:m", title: "Tunnel referendum notes", address: "mail:m", text: "From Ana. The tunnel referendum notes and the private plan for the tunnel.", kind: "mail", source: "Mail", date: Date(timeIntervalSince1970: 1_794_100_000))

    func testMailAndLocalOnlyFilesAreNeverInWhatTheDeskIsSent() {
        let c = ctx([story, mail, SavedCorpus.Document(id: "f", title: "Tunnel diary", address: "file:/d", text: "tunnel referendum diary", kind: "file", source: "Local files", date: nil)], "tunnel referendum")
        XCTAssertEqual(c.items.count, 3, "the local model gets everything")
        let page = DeskContext.page(front: FrontPage(address: "autoradio:home", title: "Autoradio"), pageText: "", brief: "", retrieved: c)!
        XCTAssertTrue(page.text.contains("Tunnel referendum result")); XCTAssertFalse(page.text.contains("private plan")); XCTAssertFalse(page.text.contains("Tunnel diary")); XCTAssertFalse(page.text.contains("notes"))
        let only = ctx([mail], "tunnel referendum")
        XCTAssertNil(DeskContext.page(front: nil, pageText: "", brief: "", retrieved: only), "nothing sendable: no page at all")
    }
    func testAMailPageInFrontIsSentWithoutItsSubjectOrBody() {
        let p = DeskContext.page(front: FrontPage(address: "mail:m", title: "Tunnel referendum notes", kind: "mail"), pageText: "From Ana. private plan", brief: "On air now: X.", retrieved: nil)!
        XCTAssertEqual(p.title, "A mail message"); XCTAssertFalse(p.text.contains("private plan")); XCTAssertEqual(p.address, "mail:m")
    }
    func testTheBestPassageDecidesWhetherTheLocalAnswerStands() {
        XCTAssertTrue(DeskContext.keepsLocal(ctx([mail], "tunnel referendum notes plan")))
        XCTAssertFalse(DeskContext.keepsLocal(ctx([story], "tunnel referendum")))
        XCTAssertFalse(DeskContext.keepsLocal(nil))
    }
    func testTheTurnLineSaysPrivateAndNothingElse() {
        let line = DeskContext.turnLine(desk: "local", retrieved: ctx([mail], "tunnel referendum notes plan"), command: "none", reason: "private")
        XCTAssertEqual(line.prefix(46), "AUTORADIO turn desk=local reason=private retri"); XCTAssertFalse(line.contains("Ana")); XCTAssertTrue(line.contains("sources=mail"))
    }
}

extension SavedAnswerTests {
    func privateCorpus() -> SavedCorpus {
        let c = SavedCorpus(readerStore: nil)
        c.setSource("t") { [SavedCorpus.Document(id: "mail:m", title: "Tunnel referendum notes", address: "mail:m", text: "From Ana. The tunnel referendum notes and the plan for the tunnel.", kind: "mail", source: "Mail", date: Date())] }
        return c
    }
    func testAMailQuestionIsAnsweredLocallyAndTheDeskIsNotAsked() async throws {
        ScriptedStreamingDesk.requests = []; ScriptedStreamingDesk.sentences = ["The server's answer."]; ScriptedStreamingDesk.hang = false
        let m = FakeLocalModel(pieces: ["The notes say the tunnel plan is private. "])
        let (e, _, got) = try engine(desk: "scripted-stream-desk", model: m, corpus: privateCorpus(), deskParams: ["delay": .number(0.05)])
        _ = await e.typedTurn("what do the tunnel referendum notes say about the plan")
        try await waitDone(got)
        XCTAssertTrue(ScriptedStreamingDesk.requests.isEmpty, "mail never reaches the server desk")
        XCTAssertEqual(Conversation.shared.entries().last?.answer ?? "", "", "the answer is the local one: first, not a desk revision")
        XCTAssertEqual(Conversation.shared.entries().last?.first, "The notes say the tunnel plan is private.")
    }
    func testWithNoLocalModelTheDeskIsAskedButNeverSentTheMail() async throws {
        ScriptedStreamingDesk.requests = []; ScriptedStreamingDesk.sentences = ["Ok."]; ScriptedStreamingDesk.hang = false
        let (e, _, got) = try engine(desk: "scripted-stream-desk", model: nil, corpus: privateCorpus(), deskParams: ["delay": .number(0.05)])
        _ = await e.typedTurn("what do the tunnel referendum notes say about the plan")
        try await waitDone(got)
        let page = ScriptedStreamingDesk.requests.last?.page
        XCTAssertFalse(ScriptedStreamingDesk.requests.isEmpty); XCTAssertFalse((page?.text ?? "").contains("plan for the tunnel"))
    }
}
