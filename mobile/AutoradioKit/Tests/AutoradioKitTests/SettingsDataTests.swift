import XCTest
@testable import AutoradioKit

// Lane 7 tests. The theme failure list (F1-F8) is now AutoradioLab/themes.test.ts; the others are at the top of Accounts.swift (A, C), SettingsData.swift (S, L)
// and ActivityFeed.swift (V); each test names the failure it covers.

/// Invented sample data the tests read: the mobile folder's fixtures (a theme Texture, a changelog).
private func fixture(_ path: String) -> Data {
    let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    return try! Data(contentsOf: mobile.appendingPathComponent("fixtures").appendingPathComponent(path))
}
final class AccountsTests: XCTestCase {
    final class FakeComputer: ComputerClient, @unchecked Sendable {
        var outcome: UpdateOutcome = .started
        var snapshots: [ComputerSnapshot]
        var wakes = 0
        init(_ s: [ComputerSnapshot]) { snapshots = s }
        func snapshot() async throws -> ComputerSnapshot { snapshots.count > 1 ? snapshots.removeFirst() : snapshots[0] }
        func wake() async throws -> String { wakes += 1; return "woke" }
        func restartToUpdate() async throws -> UpdateOutcome { outcome }
    }

    func testRestartToUpdateEndsWhenTheSignalClears() async {                  // C3
        let fake = FakeComputer([ComputerSnapshot(id: "c", state: "running", epoch: 1, updateTarget: "t2"),
                                 ComputerSnapshot(id: "c", state: "restarting", epoch: 1, updateTarget: "t2"),
                                 ComputerSnapshot(id: "c", state: "running", epoch: 2, updateTarget: "")])
        let m = ComputerModel(client: fake); m.sleep = { _ in }
        await m.refresh()
        XCTAssertTrue(m.view().updateAvailable)
        await m.restartToUpdate()
        XCTAssertEqual(m.view().phase, "current"); XCTAssertFalse(m.view().updateAvailable); XCTAssertEqual(m.view().epoch, 2)
    }

    func testABusyComputerIsToldItWillUpdateWhenIdleAndNeverCalledUpdated() async {   // C2
        let fake = FakeComputer([ComputerSnapshot(id: "c", state: "running", updateTarget: "t2")]); fake.outcome = .paused("capsule_live")
        let m = ComputerModel(client: fake)
        await m.refresh(); await m.restartToUpdate()
        XCTAssertEqual(m.view().phase, "paused"); XCTAssertTrue(m.view().message.contains("when it is idle")); XCTAssertTrue(m.view().updateAvailable)
    }

    func testAlreadyCurrentAndWake() async {
        let fake = FakeComputer([ComputerSnapshot(id: "c", state: "running")]); fake.outcome = .current
        let m = ComputerModel(client: fake)
        await m.restartToUpdate(); XCTAssertEqual(m.view().phase, "current")
        await m.wake(); XCTAssertEqual(fake.wakes, 1); XCTAssertEqual(m.view().phase, "idle")
    }

}

final class ActivityFeedTests: XCTestCase {
    func testTheSampleChangelogParsesNewestFirstWithGroupsTargetsAndUnseen() throws {
        let items = try ActivityParse.items(from: fixture("activity/changelog.json"), now: Date(timeIntervalSince1970: 1_800_000_000))
        XCTAssertEqual(items.count, 5)
        XCTAssertEqual(items.map(\.at), items.map(\.at).sorted(by: >))
        XCTAssertEqual(items[0].kind, "revision_committed"); XCTAssertEqual(items[0].target, "texture:sample-doc-edition")
        XCTAssertEqual(items[0].label, "Revision saved"); XCTAssertEqual(items[0].group, "desk")
        let used = items.first { $0.kind == "parent_used" }!
        XCTAssertEqual(used.target, "pub:sample-river-festival"); XCTAssertEqual(used.group, "paper")
        XCTAssertEqual(items.first { $0.kind == "update_finished" }?.group, "computer")
        XCTAssertEqual(items.filter(\.unseen).count, 2, "seq 41 and 40 are major and newer than seen_seq 38")
    }

    func testAnUnknownKindIsShownNotDropped() throws {                         // V2
        let d = Data(##"{"events":[{"inbox_seq":1,"level":"major","changelog":{"event_id":"e","kind":"brand_new_thing","occurred_at":"2026-11-10T10:00:00Z","summary":"s","subject":{}}}],"seen_seq":0}"##.utf8)
        let it = try ActivityParse.items(from: d)
        XCTAssertEqual(it.count, 1); XCTAssertEqual(it[0].label, "Brand New Thing"); XCTAssertEqual(it[0].target, "")
    }

    func testTargetsRefuseAbsoluteAddressesAndSchemes() {                      // V6, V3
        XCTAssertEqual(ActivityParse.target(docID: "", route: "https://evil.example/x"), "")
        XCTAssertEqual(ActivityParse.target(docID: "", route: "/a:b"), "")
        XCTAssertEqual(ActivityParse.target(docID: "", route: "/"), "")
        XCTAssertEqual(ActivityParse.target(docID: "d", route: "/r"), "texture:d")
    }

    func testALongSummaryIsCutAndABadDateSaysNothing() throws {                // V1, V4
        let long = String(repeating: "x", count: 500)
        let d = Data(#"{"events":[{"inbox_seq":1,"level":"major","changelog":{"event_id":"e","kind":"wake_decision","occurred_at":"garbage","summary":"\#(long)","subject":{}}}]}"#.utf8)
        let it = try ActivityParse.items(from: d)
        XCTAssertEqual(it[0].detail.count, 240); XCTAssertEqual(it[0].ago, "")
    }

    func testRelativeTimes() {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        XCTAssertEqual(ActivityParse.when(now.timeIntervalSince1970 - 300, now: now), "5 minutes ago")
        XCTAssertEqual(ActivityParse.when(now.timeIntervalSince1970 - 7300, now: now), "2 hours ago")
    }

    struct FailingActivity: ActivityClient { let e: Error; func events() async throws -> [ActivityItem] { throw e } }

    func testAMissingRouteLeavesTheRadiosTurnsVisible() async {                // V5
        let m = ActivityModel(client: FailingActivity(e: ChoirError.server(status: 404, message: "")))
        var turn = ActivityItem(); turn.id = "t1"; turn.group = "radio"; turn.label = "Answered"
        m.radioTurns = { [turn] in [turn] }
        await m.refresh()
        let v = m.view()
        XCTAssertEqual(v.status, "failed"); XCTAssertTrue(v.message.contains("no activity feed")); XCTAssertEqual(v.radio.count, 1); XCTAssertFalse(v.blank)
    }
}
