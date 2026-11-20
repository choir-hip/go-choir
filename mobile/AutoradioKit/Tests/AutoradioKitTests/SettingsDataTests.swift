import XCTest
@testable import AutoradioKit

// Lane 7 tests. The failure lists are at the top of ThemeLoader.swift (F1-F8), Accounts.swift (A, C), SettingsData.swift (S, L)
// and ActivityFeed.swift (V); each test names the failure it covers.

private func freshDefaults() -> UserDefaults { UserDefaults(suiteName: "l7-\(UUID().uuidString)")! }
/// Invented sample data the tests read: the mobile folder's fixtures (a theme Texture, a changelog).
private func fixture(_ path: String) -> Data {
    let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    return try! Data(contentsOf: mobile.appendingPathComponent("fixtures").appendingPathComponent(path))
}
private func tempDir() -> URL {
    let u = FileManager.default.temporaryDirectory.appendingPathComponent("l7-\(UUID().uuidString)")
    try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true)
    return u
}

final class ThemeLoaderTests: XCTestCase {
    func testBundledThemesAllLoadAndPassTheGate() {                            // F1
        let s = ThemeStore(userDir: nil, defaults: freshDefaults())
        XCTAssertEqual(s.problems, [])
        XCTAssertEqual(s.list().map(\.id).sorted(), ["chrome-pastel", "depth", "lantern", "liquid-metal", "newsprint", "signal"])
    }

    func testNewsprintDarkOnAccentPassesTheGate() {                            // F1
        let s = ThemeStore(userDir: nil, defaults: freshDefaults())
        s.setTheme("newsprint"); s.setMode("dark")
        let t = s.current()
        XCTAssertEqual(t.scheme, "dark")
        XCTAssertGreaterThanOrEqual(ThemeStore.ratio(ThemeStore.rgb(t.onAccent)!, ThemeStore.rgb(t.accent)!), 3)
    }

    func testSignalIsHighContrastAndTheOthersAreAA() {
        let s = ThemeStore(userDir: nil, defaults: freshDefaults())
        let by = Dictionary(uniqueKeysWithValues: s.cards().map { ($0.id, $0) })
        XCTAssertEqual(by["signal"]?.contrast, "AAA")
        XCTAssertEqual(by["lantern"]?.contrast, "AA")
    }

    func testStacksReduceToFacesExactResolves() {                              // F2
        XCTAssertEqual(ThemeStore.face("ui-serif, 'Iowan Old Style', Charter, Georgia, serif"), "ui-serif")
        XCTAssertEqual(ThemeStore.face("Inter, Helvetica, sans-serif"), "system-ui")
        XCTAssertEqual(ThemeStore.face("'SF Mono', monospace"), "ui-monospace")
        XCTAssertEqual(ThemeStore.face("ui-rounded"), "ui-rounded")
        XCTAssertNil(ThemeStore.face("Papyrus, Comic Sans MS"))
        let s = ThemeStore(userDir: nil, defaults: freshDefaults())
        s.setTheme("depth")
        XCTAssertEqual(s.current().fontRead, "ui-serif"); XCTAssertEqual(s.current().fontUi, "system-ui")
        s.setTheme("lantern")
        XCTAssertEqual(s.current().fontUi, "ui-rounded")
        s.setTheme("signal")
        XCTAssertEqual(s.current().fontRead, "system-ui", "a theme may read in a sans")
    }

    func testAThemeNamingNoFaceIsRefused() {                                   // F2
        let bad = ##"{"schema_version":3,"id":"odd","name":"Odd","colors":{"paper":"#FFFFFF","fg":"#000000","muted":"#444444","accent":"#0000AA","onAccent":"#FFFFFF"},"type":{"read":"Papyrus"}}"##
        let s = ThemeStore(bundled: ["odd": bad], userDir: nil, defaults: freshDefaults())
        XCTAssertTrue(s.list().isEmpty); XCTAssertTrue(s.problems.first?.contains("names no face") == true)
    }

    func testCSSInAValueIsRefused() {                                          // F3
        let bad = ##"{"schema_version":3,"id":"inj","name":"Inj","colors":{"paper":"#FFFFFF","fg":"#000000","muted":"#444444","accent":"#0000AA","onAccent":"#FFFFFF"},"elevation":{"1":{"shadow":"0 0 0 url(x)"}}}"##
        let s = ThemeStore(bundled: ["inj": bad], userDir: nil, defaults: freshDefaults())
        XCTAssertTrue(s.list().isEmpty)
        let semi = ##"{"schema_version":3,"id":"semi","name":"Semi","colors":{"paper":"#FFFFFF;x","fg":"#000000","muted":"#444444","accent":"#0000AA","onAccent":"#FFFFFF"}}"##
        XCTAssertTrue(ThemeStore(bundled: ["semi": semi], userDir: nil, defaults: freshDefaults()).list().isEmpty)
    }

    func testAMissingRoleAndAWrongVersionAreRefused() {
        let noMuted = ##"{"schema_version":3,"id":"nm","name":"NM","colors":{"paper":"#FFFFFF","fg":"#000000","accent":"#0000AA","onAccent":"#FFFFFF"}}"##
        let v2 = ##"{"schema_version":2,"id":"v2","name":"V2","colors":{"paper":"#FFFFFF","fg":"#000000","muted":"#444444","accent":"#0000AA","onAccent":"#FFFFFF"}}"##
        let s = ThemeStore(bundled: ["nm": noMuted, "v2": v2], userDir: nil, defaults: freshDefaults())
        XCTAssertTrue(s.list().isEmpty); XCTAssertEqual(s.problems.count, 2)
    }

    func testTypeScaleResolvesPerTheme() {
        let s = ThemeStore(userDir: nil, defaults: freshDefaults())
        s.setTheme("depth"); XCTAssertEqual(s.extras().sizeBody, 18)
        s.setTheme("lantern"); XCTAssertEqual(s.extras().sizeBody, 20); XCTAssertEqual(s.current().scale, "large")
        s.setTheme("signal"); XCTAssertTrue(s.extras().pressTint)
    }

    func testReducedMotionStopsTheMarkAndTheEntrance() {                       // F6
        let on = ThemeStore(userDir: nil, defaults: freshDefaults(), reduceMotion: { true })
        on.setTheme("depth")
        let t = on.current()
        XCTAssertTrue(t.reducedMotion); XCTAssertEqual(t.motionEnterMs, 0); XCTAssertEqual(t.markDriftS, 0)
        XCTAssertEqual(t.behindScale3, 1); XCTAssertTrue(on.extras().markStatic)
        XCTAssertEqual(t.springDamping, 2 * (t.springStiffness * t.springMass).squareRoot(), accuracy: 0.01, "critically damped")
        let off = ThemeStore(userDir: nil, defaults: freshDefaults()); off.setTheme("depth")
        XCTAssertFalse(off.current().reducedMotion); XCTAssertEqual(off.current().markDriftS, 60)
    }

    func testADarkOnlyThemeIsNeverShownLight() throws {                        // F4
        let dir = tempDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let s = ThemeStore(userDir: dir, defaults: freshDefaults())
        let tt = try TextureTheme.parse(fixture("texture/theme-ember.json"))
        let id = try s.installFromTexture(tt.themeJSON)
        XCTAssertEqual(id, "ember")
        s.setTheme("ember"); s.setMode("light")
        XCTAssertEqual(s.effectiveScheme(), "dark"); XCTAssertEqual(s.current().scheme, "dark")
    }

    func testInstallFromTextureSurvivesARelaunchAndCanBeRemoved() throws {    // F7
        let dir = tempDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let s = ThemeStore(userDir: dir, defaults: freshDefaults())
        let tt = try TextureTheme.parse(fixture("texture/theme-ember.json"))
        XCTAssertEqual(tt.docID, "sample-theme"); XCTAssertEqual(tt.revisionID, "sample-theme-r3")
        try s.installFromTexture(tt.themeJSON)
        let again = ThemeStore(userDir: dir, defaults: freshDefaults())
        let card = again.cards().first { $0.id == "ember" }
        XCTAssertEqual(card?.origin, "texture"); XCTAssertEqual(card?.schemes, "dark")
        again.remove("ember")
        XCTAssertNil(again.cards().first { $0.id == "ember" })
        XCTAssertEqual(ThemeStore(userDir: dir, defaults: freshDefaults()).cards().first { $0.id == "ember" }?.id, nil)
        again.remove("depth"); XCTAssertNotNil(again.cards().first { $0.id == "depth" }, "a bundled theme stays")
    }

    func testInstallRefusesAnUnreadableThemeAndWritesNothing() throws {        // F7
        let dir = tempDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let s = ThemeStore(userDir: dir, defaults: freshDefaults())
        let bad = Data(##"{"schema_version":3,"id":"faint","name":"Faint","colors":{"paper":"#FFFFFF","fg":"#EEEEEE","muted":"#EEEEEE","accent":"#0000FF","onAccent":"#FFFFFF"}}"##.utf8)
        XCTAssertThrowsError(try s.installFromTexture(bad))
        XCTAssertEqual((try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? ["?"], [])
        XCTAssertNil(s.cards().first { $0.id == "faint" })
    }

    func testReloadDoesNotDuplicateProblems() throws {                         // F5
        let dir = tempDir(); defer { try? FileManager.default.removeItem(at: dir) }
        try Data("not json".utf8).write(to: dir.appendingPathComponent("broken.json"))
        let s = ThemeStore(userDir: dir, defaults: freshDefaults())
        s.reload(); s.reload()
        XCTAssertEqual(s.problems.count, 1)
    }

    func testTextureThemeParseTakesTheRoutesAnswerAndABareTheme() throws {
        let answer = Data(##"{"doc_id":"d1","revision_id":"r9","theme":{"schema_version":3,"id":"x","name":"X"},"ignored":["layout"]}"##.utf8)
        let a = try TextureTheme.parse(answer)
        XCTAssertEqual(a.docID, "d1"); XCTAssertEqual(a.ignored, ["layout"])
        let bare = try TextureTheme.parse(Data(##"{"schema_version":3,"id":"y","name":"Y"}"##.utf8))
        XCTAssertEqual(bare.docID, "")
        XCTAssertThrowsError(try TextureTheme.parse(Data(##"{"error":"nope"}"##.utf8)))
    }

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
