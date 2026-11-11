import XCTest
@testable import AutoradioKit

// The failure list is at the top of Interview.swift (numbers below refer to it).
final class InterviewTests: XCTestCase {
    func fresh() -> Interview { Interview() }

    func texts(_ l: [InterviewLine]) -> String { l.map(\.text).joined(separator: " | ") }

    func testBankFileEqualsCompiledCopy() throws {
        // failure 2
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("interview/questions.md")
        let text = try String(contentsOf: url, encoding: .utf8).trimmingCharacters(in: .newlines)
        XCTAssertEqual(InterviewBank.parse(text), InterviewBank.default)
        XCTAssertEqual(text, InterviewBank.defaultText.trimmingCharacters(in: .newlines), "run scripts/sync-interview-bank.sh")
    }

    func testBankParsesAndSurvivesGarbage() {
        // failure 1
        let b = InterviewBank.default
        XCTAssertEqual(b.questions.map(\.id), ["topics", "follow", "tired", "sources", "places", "listening"])
        XCTAssertFalse(b.line("closing").isEmpty)
        XCTAssertFalse(b.faq.isEmpty)
        let junk = InterviewBank.parse("nothing here\n## a\nask:\n## b\nask: ok\nbad key!: x\n## @say\nz: y\n")
        XCTAssertEqual(junk.questions.map(\.id), ["b"], "an entry without an ask is dropped; a bad key is skipped")
        XCTAssertEqual(junk.say["z"], "y")
        XCTAssertEqual(InterviewBank.parse("").questions.count, 0)
    }

    func testHappyPathToReadbackAndEdition() {
        var i = fresh()
        let first = i.start()
        XCTAssertEqual(first.last?.kind, "question")
        XCTAssertTrue(first.last!.text.contains("every day"), texts(first))
        XCTAssertTrue(i.start().isEmpty, "starting twice does nothing")
        var l = i.hear("tech and the harbor council")                          // specific: follow-up skipped
        XCTAssertTrue(l.contains { $0.kind == "aside" && $0.text.contains("edition") }, "the aside is said in passing: " + texts(l))
        XCTAssertEqual(l.last?.questionID, "tired", texts(l))
        l = i.hear("celebrity gossip")
        XCTAssertEqual(l.last?.questionID, "sources")
        XCTAssertTrue(l.contains { $0.kind == "aside" && $0.text.contains("learns") })
        l = i.hear("I trust the public radio but not the tabloids")
        XCTAssertEqual(i.state.notes.trusted, ["public radio"]); XCTAssertEqual(i.state.notes.avoided, ["tabloids"])
        l = i.hear("Dunmere, and I follow the harbor mayor")
        XCTAssertEqual(i.state.notes.places, ["dunmere"]); XCTAssertEqual(i.state.notes.people, ["harbor mayor"])
        l = i.hear("mornings, about twenty minutes")
        XCTAssertEqual(i.status, .readback)
        XCTAssertEqual(l.last?.kind, "readback")
        XCTAssertTrue(l.last!.text.contains("tech and harbor council"), texts(l))
        XCTAssertTrue(l.contains { $0.kind == "aside" && $0.text.contains("documents") }, "the last aside comes before the read-back")
        l = i.hear("that's it")
        XCTAssertEqual(i.status, .done); XCTAssertEqual(l.last?.kind, "closing")
        XCTAssertTrue(i.documentMarkdown(title: "Interview, 11 Nov").contains("**You:** mornings"))
        XCTAssertTrue(i.editionDraft().first!.contains("tech"))
        XCTAssertTrue(i.hear("hello").isEmpty, "failure 14: a turn after the end is ignored")
    }

    func testVagueOffersSectionsThenTakesTheWholePaper() {
        // failure 3
        var i = fresh(); _ = i.start()
        var l = i.hear("the news")
        XCTAssertTrue(l.last!.text.contains("World, Local, Culture"), texts(l))
        XCTAssertEqual(i.current?.id, "topics")
        l = i.hear("everything")
        XCTAssertEqual(i.state.notes.topics, ["the whole paper"])
        XCTAssertNotEqual(i.current?.id, "topics")
    }

    func testAskedIsAnsweredThenRephrasedOnceThenSkipped() {
        // failure 4
        var i = fresh(); _ = i.start()
        var l = i.hear("why do you want to know?")
        XCTAssertTrue(texts(l).contains("start with what you care about"), texts(l))
        XCTAssertEqual(l.last?.questionID, "topics")
        XCTAssertEqual(l.last?.text, "Tell me one or two things you would like your paper to cover for you.")
        l = i.hear("what is this?")
        XCTAssertTrue(l.contains { $0.text.contains("leave that one") }, texts(l))
        XCTAssertNotEqual(i.current?.id, "topics")
    }

    func testInterruptionThatAnswersDropsTheQuestionAndReframes() {
        // failure 5
        var i = fresh(); _ = i.start()
        _ = i.hear("the news")                                                // the sections offer is now being said
        let l = i.hear("actually, tech", interrupted: true)
        XCTAssertTrue(i.state.turns.contains { $0.who == "radio" && $0.interrupted }, "the cut-off line is marked")
        XCTAssertTrue(l.first!.text.contains("tech"), "the next line answers what was said: " + texts(l))
        XCTAssertFalse(l.contains { $0.kind == "aside" }, "no aside over an interruption")
        XCTAssertEqual(l.last?.questionID, "follow", "a single topic gets the prepared follow-up")
        XCTAssertTrue(l.last!.text.contains("companies, the science"), texts(l))
        // the held-back aside is said at the next uninterrupted beat
        let m = i.hear("the science")
        XCTAssertTrue(m.contains { $0.kind == "aside" && $0.text.contains("edition") }, texts(m))
        XCTAssertTrue(i.documentMarkdown(title: "t").contains("(cut off)"))
    }

    func testInterruptionThatAsksIsAnsweredAndReframed() {
        var i = fresh(); _ = i.start()
        let l = i.hear("wait, is this recorded?", interrupted: true)
        XCTAssertTrue(texts(l).contains("stays on this phone"), texts(l))
        XCTAssertEqual(l.last?.questionID, "topics")
    }

    func testSilenceLadder() {
        // failure 6
        var i = fresh(); _ = i.start()
        XCTAssertTrue(i.silence().first!.text.contains("Take your time"))
        var l = i.silence()                                                    // second silence: skipped
        XCTAssertTrue(l.first!.text.contains("leave that one"), texts(l))
        XCTAssertEqual(i.status, .running)
        _ = i.silence()                                                        // the next question, first silence
        l = i.silence()                                                        // second silence on a second question in a row
        XCTAssertEqual(i.status, .paused); XCTAssertEqual(i.state.pauseReason, "silent")
        XCTAssertTrue(l.first!.text.contains("be here when you are ready"))
        XCTAssertTrue(i.hear("hello").isEmpty, "paused: no lines")
        l = i.resume()
        XCTAssertEqual(i.status, .running); XCTAssertEqual(l.first?.text, "Welcome back.")
    }

    func testTooLoudPausesAndKeepsWhatWasSaid() {
        // failure 7
        var i = fresh(); _ = i.start(); _ = i.hear("tech and local")
        let l = i.tooLoud()
        XCTAssertEqual(i.status, .paused); XCTAssertEqual(i.state.pauseReason, "loud")
        XCTAssertTrue(l.first!.text.contains("too loud"))
        XCTAssertEqual(i.state.notes.topics, ["tech", "local"])
    }

    func testEnoughGoesToReadbackEvenWithNothingSaid() {
        // failure 8
        var i = fresh(); _ = i.start()
        let l = i.hear("that's enough")
        XCTAssertEqual(i.status, .readback)
        XCTAssertTrue(l.last!.text.contains("whole paper"), texts(l))
        XCTAssertEqual(InterviewHeuristic.verdict("skip the celebrity stuff", topics: false), .answered, "skip plus a topic is an answer")
        XCTAssertEqual(InterviewHeuristic.verdict("skip", topics: false), .done)
    }

    func testReadbackCorrectedAtMostTwice() {
        // failure 9
        var i = fresh(); _ = i.start(); _ = i.hear("tech, sports and local"); _ = i.hear("none")
        _ = i.hear("none"); _ = i.hear("Dunmere"); _ = i.hear("evenings")
        XCTAssertEqual(i.status, .readback)
        var l = i.hear("drop sports")
        XCTAssertFalse(i.state.notes.topics.contains("sports")); XCTAssertEqual(i.status, .readback); XCTAssertTrue(l.last!.text.hasPrefix("Okay, I changed that"))
        l = i.hear("also the harbor")
        XCTAssertTrue(i.state.notes.topics.contains("harbor")); XCTAssertEqual(i.status, .readback)
        l = i.hear("and one more thing, ferries")
        XCTAssertEqual(i.status, .done, "the third change is applied and the interview ends")
        XCTAssertTrue(l.last!.text.contains("change it in the document"))
    }

    func testTimeBudgetGoesToReadback() {
        // failure 10
        var i = fresh(); _ = i.start()
        i.tick(elapsed: 361)
        let l = i.hear("tech and local")
        XCTAssertEqual(i.status, .readback)
        XCTAssertTrue(texts(l).contains("plenty for now"))
    }

    func testRestartComesBackPausedNotRunning() throws {
        // failure 11
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("interview-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let s = InterviewSession(interview: fresh(), file: f)
        _ = s.start()
        let exp = expectation(description: "heard")
        Task { _ = await s.hear("tech and local", interrupted: false); exp.fulfill() }
        wait(for: [exp], timeout: 5)
        let loaded = InterviewSession.load(file: f, sections: ["World"])
        XCTAssertEqual(loaded?.status, .paused)
        XCTAssertEqual(loaded?.state.pauseReason, "restart")
        XCTAssertEqual(loaded?.state.notes.topics, ["tech", "local"])
        var j = loaded!
        XCTAssertTrue(j.resume().last!.kind == "question")
    }

    func testSystem1QuestionsAndVerdicts() {
        let q = InterviewBank.default.questions[0]
        let qs = InterviewQuestions.system1(for: q)
        XCTAssertEqual(qs.map(\.id), ["reply_kind", "follow"])
        XCTAssertTrue(qs[1].options.contains("tech") && qs[1].options.contains("none"))
        let (v, f) = InterviewQuestions.verdict(from: [System1Answer(questionID: "reply_kind", option: "vague", score: 0.9, ms: 5),
                                                       System1Answer(questionID: "follow", option: "ai", score: 0.7, ms: 5)])
        XCTAssertEqual(v, .vague); XCTAssertEqual(f, "ai")
        let (v2, f2) = InterviewQuestions.verdict(from: [System1Answer(questionID: "reply_kind", option: "nonsense", score: 1, ms: 1)])
        XCTAssertNil(v2); XCTAssertNil(f2, "failure 12: an option the interview does not know is ignored")
    }

    func testSystem1VerdictOverridesTheHeuristic() {
        var i = fresh(); _ = i.start()
        let l = i.hear("hmm the harbor thing", verdict: .vague)               // System 1 judged it vague
        XCTAssertEqual(i.current?.id, "topics"); XCTAssertTrue(l.last!.text.contains("Today's paper has"))
    }

    func testDeskStageAnswersATurnWithARevisionOfTheNextLines() async throws {
        let session = InterviewSession(interview: fresh(), file: nil)
        InterviewSession.shared = session
        defer { InterviewSession.shared = nil }
        let programme = try await InterviewProgramme(StageParams()).load()
        XCTAssertEqual(programme.id, "interview"); XCTAssertEqual(programme.blocks.count, 2)
        let desk = InterviewDesk(StageParams())
        var got: [Revision] = []
        for try await r in desk.submit(DeskTurn(turnID: "t1", programmeID: "interview", blockIndex: 1, listenerWords: "tech and local news")) { got.append(r) }
        XCTAssertEqual(got.count, 1)
        XCTAssertEqual(got[0].fromBlock, 2); XCTAssertFalse(got[0].keepRest)
        XCTAssertTrue(got[0].blocks.last!.spokenText!.contains("tired"), "the next question follows: \(got[0].blocks)")
        let doc = programme.applying(got[0])
        XCTAssertEqual(doc.blocks.count, 2 + got[0].blocks.count)
    }

    func testExtractLists() {
        XCTAssertEqual(InterviewExtract.list("I want tech, the harbor council and also climate."), ["tech", "harbor council", "climate"])
        XCTAssertEqual(InterviewExtract.list("nothing"), [])
        XCTAssertEqual(InterviewHeuristic.verdict("everything", topics: true), .vague)
        XCTAssertEqual(InterviewHeuristic.verdict("what is a texture?", topics: true), .asked)
        XCTAssertEqual(InterviewHeuristic.verdict("nope", topics: false), .refused)
        XCTAssertEqual(InterviewHeuristic.readbackVerdict("yeah that's right"), .answered, "only exact confirmations end the read-back")
        XCTAssertEqual(InterviewHeuristic.readbackVerdict("looks good"), .done)
    }
}

// The demo's recorded turns (fixtures/demo/*.json) drive the proof; these tests keep them honest without a simulator.
final class InterviewDemoFixtureTests: XCTestCase {
    func fixture(_ name: String) throws -> Data {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("fixtures/demo/\(name).json")
        return try Data(contentsOf: url)
    }

    func testRecordedInterviewRunsToTheEnd() throws {
        let o = try XCTUnwrap(try JSONSerialization.jsonObject(with: fixture("interview-script")) as? [String: Any])
        let steps = try XCTUnwrap(o["steps"] as? [[String: String]])
        var i = Interview(sections: o["sections"] as? [String] ?? [])
        _ = i.start()
        var seen: [String] = []
        for st in steps {
            let words = st["words"] ?? ""
            let lines: [InterviewLine]
            switch st["kind"] ?? "say" {
            case "interrupt": lines = i.hear(words, interrupted: true)
            case "loud": lines = i.tooLoud()
            case "resume": lines = i.resume()
            default: lines = i.hear(words)
            }
            seen += lines.map(\.text)
            XCTAssertFalse(lines.isEmpty, "step \(st) produced nothing (status \(i.status))")
        }
        XCTAssertEqual(i.status, .done)
        let said = seen.joined(separator: " | ")
        XCTAssertTrue(said.contains("Today's paper has World, Local, Culture"), said)
        XCTAssertTrue(said.contains("Mm, tech"), said)
        XCTAssertTrue(said.contains("too loud"), said)
        XCTAssertTrue(said.contains("Welcome back"), said)
        XCTAssertTrue(said.contains("Here is what I have"), said)
        XCTAssertTrue(said.contains("Okay, I changed that"), said)
        XCTAssertFalse(i.state.notes.less.contains("sports"), "the read-back correction took sports out")
        XCTAssertTrue(i.state.turns.contains { $0.who == "radio" && $0.interrupted })
        XCTAssertEqual(i.state.notes.avoided, ["tabloids"])
    }

    func testEditionFixtureAndOneDeskFailure() async throws {
        let e = try XCTUnwrap(Edition.parse(fixture("edition")))
        XCTAssertEqual(e.failFirst, true)
        XCTAssertTrue(e.lines.contains("What I follow"))
        let w = DemoEditionWriter(edition: e, tickMs: 1)
        do { _ = try await w.write(from: Interview(), interviewTitle: "t") { _ in }; XCTFail("the first write fails") } catch {}
        let ok = try await w.write(from: Interview(), interviewTitle: "t") { _ in }
        XCTAssertEqual(ok.title, "My edition")
        let off = UnbuiltEditionWriter()
        do { _ = try await off.write(from: Interview(), interviewTitle: "t") { _ in }; XCTFail("not built") } catch { XCTAssertTrue("\(error)".contains("not connected")) }
    }
}
