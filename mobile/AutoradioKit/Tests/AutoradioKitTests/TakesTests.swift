import XCTest
@testable import AutoradioKit

/// Takes, what stays native (failure list: Takes.swift; the page, the wire for publishing and the listener's own data are shared code, takes.test.ts and take-session.test.ts).
final class TakesTests: XCTestCase {
    func testT6TheProgrammesCopyOfOwnIdsAndMutesIsWhatSharedCodePushedAndSurvivesARelaunch() throws {
        let d = FileManager.default.temporaryDirectory.appendingPathComponent("mirror-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: d) }
        let fo = d.appendingPathComponent("takes-copy-own.json"), fm = d.appendingPathComponent("takes-copy-muted.json")
        let own = TakeOwn(file: fo), mutes = TakeMutes(file: fm)
        own.replace(ids: ["take-2", "take-1"], handle: "voice-me0001"); mutes.replace(["voice-3fa2c1", "bad handle"])
        XCTAssertEqual(TakeOwn(file: fo).handle, "voice-me0001")
        XCTAssertEqual(TakeMutes(file: fm).all, ["voice-3fa2c1"], "a handle that is not one is not kept")
        own.replace(ids: [], handle: ""); mutes.replace([])
        XCTAssertEqual(TakeMutes(file: fm).all, []); XCTAssertEqual(TakeOwn(file: fo).handle, "")
        XCTAssertEqual(TakeFiles.url("takes-copy-own.json")?.lastPathComponent, "takes-copy-own.json")
        XCTAssertNotEqual(TakeFiles.url("takes-own.json"), TakeFiles.url("takes-copy-own.json"), "the programme's copy is not the old file: a push never recreates it")
    }

    /// The programme's copy takes a push only from the account generation that is current: an older one (in flight when the account changed) is refused, and so is any push of the generation the wipe found,
    /// even one that lands after the wipe and before the new account's first push.
    func testTheMirrorRefusesAStalePushAndAfterAWipeAnythingButANewerGeneration() {
        TakeMirror.reset()
        XCTAssertTrue(TakeMirror.accept(5)); XCTAssertTrue(TakeMirror.accept(5), "the same generation again is the same account"); XCTAssertFalse(TakeMirror.accept(4), "older: refused")
        TakeMirror.invalidate()                                  // the account wipe
        XCTAssertFalse(TakeMirror.accept(5), "the old account's push landing after the wipe")
        XCTAssertFalse(TakeMirror.accept(4)); XCTAssertTrue(TakeMirror.accept(6), "the new account's first push")
        XCTAssertTrue(TakeMirror.accept(6)); TakeMirror.reset()
    }

    func testRespondsToFromTheEntryOnAir() async {
        await MainActor.run {
            var story = RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "Harbor fees rise", text: "x", estSeconds: 3)
            story.cites = [RunSheetCite(ref: "pub:harbor-fees@v2", role: .subject, label: "Harbor fees rise")]
            XCTAssertEqual(TakeResponds.from(entry: story), TakeResponds(route: "harbor-fees", title: "Harbor fees rise"))
            XCTAssertNil(TakeResponds.from(entry: RunSheetEntry(id: "o", kind: .opening, placedBy: .phone, title: "Opening", text: "x", estSeconds: 3)))
            XCTAssertNil(TakeResponds.from(entry: nil))
        }
    }
}
