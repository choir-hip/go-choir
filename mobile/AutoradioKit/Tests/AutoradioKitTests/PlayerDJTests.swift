import XCTest
@testable import AutoradioKit

// The DJ's wiring: the programme request carries the paper's stories, a stretch of only station breaks is not a programme, and starting the
// radio never plays the Lab's test edition or says there is nothing to report (owner, 2026-11-15). Fixtures and helpers are PlayerAppTests'.
// The failure list is mobile/README.md, "Player controls", items 19 to 24.

extension PlayerAppTests {
    // P19, P22: the programme request carries the stories; with none it is not made.
    func testP19_TheProgrammeRequestCarriesTheFrontListWithTextOnTheFirstTen() async throws {
        StubProtocol.handler = { [self] req in
            if req.path == "/api/autoradio/programme" { return .init(headers: ["Content-Type": "text/event-stream"], raw: ": autoradio\n\nevent: done\ndata: {}\n\n") }
            return front(req)
        }
        _ = try? await RadioProgramme(StageParams()).open()
        let req = try XCTUnwrap(programmeCalls.first)
        let cands = try XCTUnwrap(req.body["candidates"] as? [[String: Any]])
        XCTAssertEqual(cands.compactMap { $0["route"] as? String }, ["a", "b", "c"], "the paper's weight first")
        XCTAssertEqual(cands.compactMap { $0["text"] as? String }, ["Text of a.", "Text of b.", "Text of c."])
        XCTAssertEqual(cands[0]["title"] as? String, "Story a"); XCTAssertEqual(cands[0]["dek"] as? String, "Dek a.")
        XCTAssertNotNil(cands[0]["version"] as? Int)
        XCTAssertEqual(programmeCalls.count, 1, "one request for the whole job")
    }
    func testP21_AStoryWhoseTextIsLateGoesWithoutItAndTheRequestStillGoes() async throws {
        failing.write { $0 = ["b"] }
        StubProtocol.handler = { [self] req in
            if req.path == "/api/autoradio/programme" { return .init(headers: ["Content-Type": "text/event-stream"], raw: "event: done\ndata: {}\n\n") }
            return front(req)
        }
        _ = try? await RadioProgramme(StageParams()).open()
        let cands = try XCTUnwrap(programmeCalls.first?.body["candidates"] as? [[String: Any]])
        XCTAssertEqual(cands.count, 3)
        XCTAssertNil(cands[1]["text"]); XCTAssertNotNil(cands[0]["text"]); XCTAssertNotNil(cands[2]["text"])
    }
    func testP22_NoStoriesNoRequestAndTheFrontPageStoryPlaysAsBefore() async throws {
        StubProtocol.handler = { req in
            if req.path == "/api/platform/publications/front" { return .init(json: ["stories": []]) }
            return .init(status: 404, json: ["error": "nope"])
        }
        do { _ = try await RadioProgramme(StageParams()).open(); XCTFail("nothing to play") } catch {}
        XCTAssertEqual(programmeCalls.count, 0, "a request with no candidates only buys station breaks: it is not made")
    }

    // P24 (owner, 2026-11-15: after onboarding "start the radio" said "there's nothing to report ... I'll let the music play"): a radio
    // start with stories in the paper never yields the empty announcement, whatever the programme route answers.
    func idents(_ texts: [String]) -> String {
        ": autoradio\n\nevent: stretch\ndata: {\"stretch_id\":\"st-9\"}\n\n"
        + texts.enumerated().map { i, t in "event: segment\ndata: " + segmentLine(["stretch_id": "st-9", "segment_id": "st-9.s\(i + 1)", "kind": "ident", "title": "", "text": t]) + "\n\n" }.joined()
        + "event: done\ndata: {\"stretch_id\":\"st-9\"}\n\n"
    }
    func spoken(_ d: ProgrammeDocument) -> String { d.blocks.compactMap(\.spokenText).joined(separator: " ").lowercased() }

    func testP24_StartingTheRadioWithStoriesInThePaperNeverSaysThereIsNothing() async {
        let line = "You're with the Autoradio desk, and there's nothing to report now, so I'm just going to let the music play."
        StubProtocol.handler = { [self] req in
            req.path == "/api/autoradio/programme" ? .init(headers: ["Content-Type": "text/event-stream"], raw: idents([line])) : front(req)
        }
        let (app, rt) = await makeApp()
        XCTAssertEqual(rt.player.document.id, "fixture-1", "the player starts with the Lab's test edition")
        await app.startRadio()
        XCTAssertEqual(rt.player.document.id, "choir:pv-a", "the paper's first story plays, not the fixture and not a station break")
        XCTAssertFalse(spoken(rt.player.document).contains("nothing to report")); XCTAssertFalse(spoken(rt.player.document).contains("music play"))
        XCTAssertFalse(rt.player.userPaused)
        XCTAssertEqual(programmeCalls.count, 1); XCTAssertEqual((programmeCalls[0].body["candidates"] as? [[String: Any]])?.count, 3, "and the request carried the stories")
        rt.player.pause()
    }
    func testP24_BreaksAtTheHeadWaitForTheFirstStoryAndThenPlayBeforeIt() async throws {
        let raw = idents(["You are with the Autoradio desk."]) .replacingOccurrences(of: "event: done", with: "event: segment\ndata: " + segmentLine(["stretch_id": "st-9", "segment_id": "st-9.s2", "kind": "story", "title": "Harbour", "text": "The cranes moved inland."]) + "\n\nevent: done")
        StubProtocol.handler = { [self] req in req.path == "/api/autoradio/programme" ? .init(headers: ["Content-Type": "text/event-stream"], raw: raw) : front(req) }
        let opened = try await RadioProgramme(StageParams()).open()
        XCTAssertFalse(opened.usedFallback)
        XCTAssertEqual(opened.document.segments.map(\.kind), ["ident", "story"])
    }
    func testP24_AStretchOfOnlyBreaksIsNotAProgramme() async throws {
        StubProtocol.handler = { [self] req in req.path == "/api/autoradio/programme" ? .init(headers: ["Content-Type": "text/event-stream"], raw: idents(["Nothing to report.", "Back soon."])) : front(req) }
        let opened = try await RadioProgramme(StageParams()).open()
        XCTAssertTrue(opened.usedFallback, "the story plays instead")
        XCTAssertEqual(opened.document.id, "choir:pv-a")
    }
    func testP24_WhenThePaperCannotBeReachedItIsSaidPlainly() {
        XCTAssertTrue(ChoirApp.plain(stage: "programme", message: "network connection lost").contains("can't reach the paper"))
    }
}
