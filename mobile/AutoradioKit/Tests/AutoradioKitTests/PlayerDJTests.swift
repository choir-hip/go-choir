import XCTest
@testable import AutoradioKit

// The DJ's wiring: the programme request carries the paper's stories, a stretch of only station breaks is not a programme, and starting the
// radio never plays the Lab's test edition or says there is nothing to report (owner, 2026-11-15). Fixtures and helpers are PlayerAppTests'.
// The failure list is mobile/README.md, "Player controls", items 19 to 24.

extension PlayerAppTests {
    // P24 (owner, 2026-11-15: after onboarding "start the radio" said "there's nothing to report ... I'll let the music play"): a radio
    // start with stories in the paper never yields the empty announcement, whatever the programme route answers.
    func idents(_ texts: [String]) -> String {
        ": autoradio\n\nevent: stretch\ndata: {\"stretch_id\":\"st-9\"}\n\n"
        + texts.enumerated().map { i, t in "event: segment\ndata: " + segmentLine(["stretch_id": "st-9", "segment_id": "st-9.s\(i + 1)", "kind": "ident", "title": "", "text": t]) + "\n\n" }.joined()
        + "event: done\ndata: {\"stretch_id\":\"st-9\"}\n\n"
    }
    func spoken(_ d: ProgrammeDocument) -> String { d.blocks.compactMap(\.spokenText).joined(separator: " ").lowercased() }

    func testP24_WhenThePaperCannotBeReachedItIsSaidPlainly() {
        XCTAssertTrue(ChoirApp.plain(stage: "programme", message: "network connection lost").contains("can't reach the paper"))
    }
}
