import XCTest
@testable import AutoradioKit

// Failure list (2026-11-17, owner: "Read my book" answered "the page is empty and I have no access to any files"):
//  1. The server desk is sent an empty page when the page in front is the Autoradio page, and none of what the phone retrieved.
//  2. "that" and "this" search for the title of the Autoradio page instead of what is on air.
//  3. Any phrase is handled on the phone outside the one path: nothing is (owner, 2026-11-17).
//  5. The prompt the on-device model is shown cannot be seen: a test prints it for a store of books, stories and podcasts.
//  6. The turn leaves no line saying who answered and what was retrieved (counts only, never content).

final class TurnContextTests: XCTestCase {
    static let now = Date(timeIntervalSince1970: 1_794_700_000)
    func fixtureStore() -> (Int, SavedCorpus) {
        let c = SavedCorpus(readerStore: nil)
        func day(_ n: Double) -> Date { Self.now.addingTimeInterval(-n * 86400) }
        var list: [SavedCorpus.Document] = []     // what the content store held (shared code now): the same documents, handed to the index directly
        list.append(SavedCorpus.Document(id: "pub:/p/water", title: "Senate passes the water bill", address: "pub:/p/water", text: "The Senate voted 61 to 38 on Tuesday to pass the water bill after a long debate on drought funding.", kind: "story", source: "Autopaper", date: day(1)))
        list.append(SavedCorpus.Document(id: "pub:/p/budget", title: "Senate committee delays budget hearing", address: "pub:/p/budget", text: "A Senate committee postponed its budget hearing until next week.", kind: "story", source: "Autopaper", date: day(3)))
        list.append(SavedCorpus.Document(id: "pub:/p/harbor", title: "Harbor strike ends", address: "pub:/p/harbor", text: "Dock workers ended the strike on Tuesday.", kind: "story", source: "Autopaper", date: day(2)))
        list.append(SavedCorpus.Document(id: "pc:ep1", title: "World News: The Senate week", address: "podcast:episode/x/ep1", text: "This episode covers the Senate and the budget.", kind: "podcast", source: "Podcasts", date: day(1)))
        let stored = list
        c.setSource("store") { stored }
        c.setSource("library") { [SavedCorpus.Document(id: "book:b1", title: "The Keeper of Lamps", address: "doc:b1", text: "", kind: "book", source: "Reader", date: day(30),
                                                         parts: [.init(label: "chapter 1: The shore", text: "The keeper walked the shore at dusk and counted the boats coming in.", block: 3)])] }
        return (0, c)
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
        let line = DeskContext.turnLine(desk: "server", retrieved: c.retrieve("tell me about the Senate"))
        XCTAssertTrue(line.hasPrefix("AUTORADIO turn desk=server retrieved=3 sources=podcast,story context_chars="), line)
        XCTAssertFalse(line.contains("Senate"))
        XCTAssertEqual(DeskContext.turnLine(desk: "local", retrieved: nil), "AUTORADIO turn desk=local retrieved=0 sources=- context_chars=0")
    }
}

extension SavedAnswerTests {
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

final class MailTurnTests: XCTestCase {
    func ctx(_ docs: [SavedCorpus.Document], _ q: String) -> RetrievalContext {
        let c = SavedCorpus(readerStore: nil); c.setSource("t") { docs }; return c.retrieve(q)
    }
    let story = SavedCorpus.Document(id: "pub:s", title: "Tunnel referendum result", address: "pub:/s", text: "The tunnel referendum passed.", kind: "story", source: "Autopaper", date: Date(timeIntervalSince1970: 1_794_000_000))
    let mail = SavedCorpus.Document(id: "mail:m", title: "Tunnel referendum notes", address: "mail:m", text: "From Ana. The tunnel referendum notes and the private plan for the tunnel.", kind: "mail", source: "Mail", date: Date(timeIntervalSince1970: 1_794_100_000))

    func testMailAndFilePassagesAreInWhatTheDeskIsSent() {
        let c = ctx([story, mail, SavedCorpus.Document(id: "f", title: "Tunnel diary", address: "file:/d", text: "tunnel referendum diary", kind: "file", source: "Local files", date: nil)], "tunnel referendum")
        XCTAssertEqual(c.items.count, 3)
        let page = DeskContext.page(front: FrontPage(address: "autoradio:home", title: "Autoradio"), pageText: "", brief: "", retrieved: c)!
        XCTAssertTrue(page.text.contains("Tunnel referendum result")); XCTAssertTrue(page.text.contains("private plan")); XCTAssertTrue(page.text.contains("Tunnel diary"))
        let only = ctx([mail], "tunnel referendum")
        XCTAssertTrue(DeskContext.page(front: nil, pageText: "", brief: "", retrieved: only)?.text.contains("private plan") ?? false, "a mail passage alone is sent")
    }
    func testAMailPageInFrontIsSentWithItsSubjectAndBody() {
        let p = DeskContext.page(front: FrontPage(address: "mail:m", title: "Tunnel referendum notes", kind: "mail"), pageText: "From Ana. private plan", brief: "On air now: X.", retrieved: nil)!
        XCTAssertEqual(p.title, "Tunnel referendum notes"); XCTAssertTrue(p.text.contains("private plan")); XCTAssertEqual(p.address, "mail:m")
    }
    func testTheTurnLineNamesKindsNeverWords() {
        let line = DeskContext.turnLine(desk: "server", retrieved: ctx([mail], "tunnel referendum notes plan"))
        XCTAssertTrue(line.hasPrefix("AUTORADIO turn desk=server retri"), line); XCTAssertFalse(line.contains("Ana")); XCTAssertTrue(line.contains("sources=mail"))
    }
}

extension SavedAnswerTests {
    func privateCorpus() -> SavedCorpus {
        let c = SavedCorpus(readerStore: nil)
        c.setSource("t") { [SavedCorpus.Document(id: "mail:m", title: "Tunnel referendum notes", address: "mail:m", text: "From Ana. The tunnel referendum notes and the plan for the tunnel.", kind: "mail", source: "Mail", date: Date())] }
        return c
    }
    func testAMailQuestionIsAskedOfTheDeskWithTheMailAndItsRevisionReplacesTheLocalAnswer() async throws {
        ScriptedStreamingDesk.requests = []; ScriptedStreamingDesk.sentences = ["The server's answer."]; ScriptedStreamingDesk.hang = false
        let m = FakeLocalModel(pieces: ["The notes say the tunnel plan is private. "])
        let (e, _, got) = try engine(desk: "scripted-stream-desk", model: m, corpus: privateCorpus(), deskParams: ["delay": .number(0.05)])
        _ = await e.typedTurn("what do the tunnel referendum notes say about the plan")
        try await waitDone(got)
        XCTAssertFalse(ScriptedStreamingDesk.requests.isEmpty, "the desk is asked for a mail turn like any other")
        XCTAssertTrue((ScriptedStreamingDesk.requests.last?.page?.text ?? "").contains("plan for the tunnel"), "and is sent the mail passage")
        XCTAssertEqual(Conversation.shared.entries().last?.answer ?? "", "The server's answer.", "the desk's revision replaces the local answer")
    }
    func testWithNoLocalModelTheDeskIsSentTheMail() async throws {
        ScriptedStreamingDesk.requests = []; ScriptedStreamingDesk.sentences = ["Ok."]; ScriptedStreamingDesk.hang = false
        let (e, _, got) = try engine(desk: "scripted-stream-desk", model: nil, corpus: privateCorpus(), deskParams: ["delay": .number(0.05)])
        _ = await e.typedTurn("what do the tunnel referendum notes say about the plan")
        try await waitDone(got)
        let page = ScriptedStreamingDesk.requests.last?.page
        XCTAssertFalse(ScriptedStreamingDesk.requests.isEmpty); XCTAssertTrue((page?.text ?? "").contains("plan for the tunnel"))
    }
}

final class OnePathTests: XCTestCase {
    static let now = Date(timeIntervalSince1970: 1_794_700_000)
    func store() -> SavedCorpus {
        let c = SavedCorpus(readerStore: nil)
        func d(_ n: Double) -> Date { Self.now.addingTimeInterval(-n * 86400) }
        c.setSource("t") { [
            SavedCorpus.Document(id: "pub:a", title: "Senate passes the water bill", address: "pub:/a", text: "The Senate voted on Tuesday to pass the water bill.", kind: "story", source: "Autopaper", date: d(1)),
            SavedCorpus.Document(id: "pub:b", title: "Senate committee delays budget hearing", address: "pub:/b", text: "A Senate committee postponed its budget hearing.", kind: "story", source: "Autopaper", date: d(3)),
            SavedCorpus.Document(id: "pub:c", title: "Harbor strike ends after nine days", address: "pub:/c", text: "Dock workers voted to end the strike over pay.", kind: "story", source: "Autopaper", date: d(2))] }
        c.onAirHint = "Harbor strike ends after nine days"
        c.onAirBrief = "On air now: Harbor strike ends after nine days. Up next: Water."
        return c
    }
    func testTheSenateReachesBothDesksNewestFirstWithNothingSpecial() async {
        let c = store()
        let ctx = c.retrieve("tell me about the Senate", onAir: "Harbor strike ends after nine days", now: Self.now)
        XCTAssertEqual(ctx.items.map(\.address), ["pub:/a", "pub:/b"], "newest first, no harbor story")
        let m = FakeLocalModel(pieces: ["The Senate passed it. "])
        for await _ in SavedAnswerHolder(StageParams(), corpus: c, model: { m }).lines(for: HolderInput(listenerWords: "tell me about the Senate", answers: [], retrieved: ctx)) {}
        let local = m.asked.first?.prefix ?? ""
        let server = DeskContext.page(front: FrontPage(address: "autoradio:home", title: "Autoradio"), pageText: "", brief: ctx.brief, retrieved: ctx)!.text
        for seen in [local, server] {
            XCTAssertTrue(seen.contains("On air now: Harbor strike ends after nine days"), seen)
            XCTAssertLessThan(seen.range(of: "water bill")!.lowerBound, seen.range(of: "budget hearing")!.lowerBound)
            XCTAssertFalse(seen.contains("Dock workers"))
        }
    }
    func testAPlainFollowUpOnWhatIsOnAirGoesThroughTheSamePath() async {
        let c = store()
        let ctx = c.retrieve("more on this", onAir: OnAir.title(pageAddress: "autoradio:home", pageTitle: "Autoradio", hint: c.onAirHint), now: Self.now)
        XCTAssertEqual(ctx.items.first?.address, "pub:/c"); XCTAssertTrue(ctx.record.usedOnAir)
        let server = DeskContext.page(front: FrontPage(address: "autoradio:home", title: "Autoradio"), pageText: "", brief: ctx.brief, retrieved: ctx)!.text
        XCTAssertTrue(server.hasPrefix("On air now: Harbor strike ends after nine days.")); XCTAssertTrue(server.contains("Dock workers voted"))
    }
    func testNoPhraseIsMatchedOutsideTheOnePath() throws {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Sources/AutoradioKit")
        let engine = try String(contentsOf: root.appendingPathComponent("TurnEngine.swift"), encoding: .utf8)
        for gone in ["SpokenCommand", "LocalCommands", "RetrievedCommand", "finishCommand"] { XCTAssertFalse(engine.contains(gone), gone) }
        XCTAssertFalse(FileManager.default.fileExists(atPath: root.appendingPathComponent("LocalCommands.swift").path))
    }
}
