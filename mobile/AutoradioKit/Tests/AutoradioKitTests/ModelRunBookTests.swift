import XCTest
@testable import AutoradioKit

// What Voice and models remembers between presses (ModelRunBook): the Test lines and the last failure of each model's download.
//  B1 a Test line written for an account that has since signed out is dropped, and the sign-out clears what was kept: account B never sees account A's "Heard: ...".
//  B2 a failed download's reason is kept until that model is pressed again or is on the phone: another row's run does not erase it.
//  B3 the speaker fetch has a failure state of its own.
final class ModelRunBookTests: XCTestCase {
    func testB1ATestLineOfASignedOutAccountIsNeverShownToTheNext() {
        let b = ModelRunBook()
        let gen = AccountWipe.generation
        b.test("transcriber:m", state: "done", line: "Heard: \"private words\"", ms: 900, generation: gen)
        XCTAssertEqual(b.tests().map(\.key), ["transcriber:m"])
        b.reset()                                                                           // sign-out
        XCTAssertTrue(b.tests().isEmpty, "the sign-out cleared it")
        AccountWipe.bumpGeneration()                                                         // the wipe moved the generation
        b.test("transcriber:m", state: "done", line: "Heard: \"late report\"", ms: 1, generation: gen)    // a test still running when the account left
        XCTAssertTrue(b.tests().isEmpty, "a report from before the wipe is dropped")
        b.test("transcriber:m", state: "done", line: "mine", ms: 1, generation: AccountWipe.generation)
        XCTAssertEqual(b.tests().first?.line, "mine")
    }
    func testB2AFailureOutlivesTheNextRowsRun() {
        let b = ModelRunBook()
        b.failed("fluid-speech-pro", "The download failed (offline).")
        var runs = b.runs(current: [["id": "silero-vad-v6.2.1", "state": "downloading", "progress": 0.2, "line": ""]], isReady: { _ in false })
        XCTAssertEqual(runs.first { ($0["id"] as? String) == "fluid-speech-pro" }?["line"] as? String, "The download failed (offline).")
        XCTAssertEqual(runs.first { ($0["id"] as? String) == "fluid-speech-pro" }?["state"] as? String, "failed")
        runs = b.runs(current: [["id": "fluid-speech-pro", "state": "downloading", "progress": 0.1, "line": ""]], isReady: { _ in false })
        XCTAssertEqual(runs.filter { ($0["id"] as? String) == "fluid-speech-pro" }.count, 1, "pressed again: the live run replaces the old failure")
        XCTAssertTrue(b.runs(current: [], isReady: { _ in false }).isEmpty, "the press cleared the failure it replaced")
        b.failed("a", "x"); XCTAssertTrue(b.runs(current: [], isReady: { $0 == "a" }).isEmpty, "a model that is on the phone has no failure")
    }
    func testB3TheSpeakerFetchHasAFailureState() {
        let b = ModelRunBook()
        b.speakerFailed("The voice-print model could not be fetched.")
        let r = b.runs(current: [], isReady: { _ in false }).first
        XCTAssertEqual(r?["id"] as? String, ModelOffer.speakerID); XCTAssertEqual(r?["state"] as? String, "failed")
    }
}
