import XCTest
@testable import AutoradioKit

/// Demo mode never turns on where it is not allowed (a physical phone), and a stored flag there is removed.
final class DemoOnDeviceTests: XCTestCase {
    private func scratch() -> UserDefaults {
        let name = "demo-test-\(UUID().uuidString)"
        let d = UserDefaults(suiteName: name)!
        addTeardownBlock { d.removePersistentDomain(forName: name) }
        return d
    }

    func testAStoredFlagIsIgnoredAndRemovedWhereDemoIsNotAllowed() {
        let d = scratch()
        d.set(true, forKey: Demo.defaultsKey)
        XCTAssertFalse(Demo.isOn(allowed: false, defaults: d, env: [:]))
        XCTAssertNil(d.object(forKey: Demo.defaultsKey), "the flag must not survive to the next launch")
    }

    func testTheEnvironmentCannotTurnItOnWhereItIsNotAllowed() {
        XCTAssertFalse(Demo.isOn(allowed: false, defaults: scratch(), env: ["AUTORADIO_DEMO": "1"]))
    }

    func testTheSimulatorAndTheMacKeepDemoMode() {
        let d = scratch()
        XCTAssertFalse(Demo.isOn(allowed: true, defaults: d, env: [:]))
        d.set(true, forKey: Demo.defaultsKey)
        XCTAssertTrue(Demo.isOn(allowed: true, defaults: d, env: [:]))
        XCTAssertTrue(Demo.isOn(allowed: true, defaults: scratch(), env: ["AUTORADIO_DEMO": "1"]))
        d.removeObject(forKey: Demo.defaultsKey)
        XCTAssertFalse(Demo.isOn(allowed: true, defaults: d, env: [:]))
    }

    /// The host this test runs on is the Mac: demo mode must be allowed here (an inverted guard would turn it off on every proof lane).
    func testDemoIsAllowedOnTheHost() { XCTAssertTrue(Demo.allowedHere) }

    // MARK: what demo left behind (written before the code)
    // Failure list:
    //  1 a device cleans once whatever the flag was (no flag, no marker still cleans), and a second launch does not
    //  2 where demo is allowed (simulator, Mac) nothing is cleared
    //  3 the sample book goes only when the shelf lists it; a real book stays; an unreadable index is never rewritten
    //  4 onboarding, edition topics and voice setup are never removed
    //  5 only the addresses demo mode produced (pub: plus a fixture id) leave pages and signals; a real file:/demo-notes.pdf stays
    //  6 the cleared marker is written after the removals: a failed write leaves it unset, and the next launch retries

    private func support() -> URL {
        let u = FileManager.default.temporaryDirectory.appendingPathComponent("demo-left-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: u.path); try? FileManager.default.removeItem(at: u) }
        return u
    }
    private func put(_ dir: URL, _ name: String, _ text: String) { try? Data(text.utf8).write(to: dir.appendingPathComponent(name)) }
    private func exists(_ dir: URL, _ name: String) -> Bool { FileManager.default.fileExists(atPath: dir.appendingPathComponent(name).path) }
    private func books(_ s: URL) -> ReaderShelf { ReaderShelf(root: s.appendingPathComponent("books")) }
    private let pagesJSON = """
    {"pages":[{"address":"radio:","kind":"radio","title":"Autoradio","opener":"","pinned":false},{"address":"pub:demo-harbor-barrier","kind":"pub","title":"Harbor","opener":"","pinned":false},{"address":"pub:/real-story","kind":"pub","title":"Real","opener":"","pinned":false},{"address":"file:/demo-notes.pdf","kind":"file","title":"Notes","opener":"","pinned":false},{"address":"pub:demo-not-a-fixture","kind":"pub","title":"Other","opener":"","pinned":false}],"recency":["pub:demo-harbor-barrier","pub:/real-story","file:/demo-notes.pdf","pub:demo-not-a-fixture","radio:"]}
    """

    func testADeviceCleansOnceWhateverTheFlagWas() throws {
        let d = scratch(); let s = support()           // no stored flag, no marker: the owner's phone already ran the build that removed it
        _ = books(s).seedDemo(demo: true)
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertTrue(d.bool(forKey: Demo.clearedKey))
        XCTAssertTrue(books(s).list().isEmpty)
        _ = books(s).seedDemo(demo: true)              // a second launch does nothing
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertEqual(books(s).list().count, 1)
    }

    func testNothingIsClearedWhereDemoIsAllowed() throws {
        let d = scratch(); let s = support()
        _ = books(s).seedDemo(demo: true); put(s, "open-pages.json", pagesJSON)
        Demo.clearLeftovers(support: s, allowed: true, defaults: d)
        XCTAssertEqual(books(s).list().count, 1)
        XCTAssertTrue(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("demo-harbor-barrier"))
        XCTAssertFalse(d.bool(forKey: Demo.clearedKey))
    }

    func testTheSampleBookGoesAndARealBookStays() throws {
        let d = scratch(); let s = support()
        let shelf = books(s); _ = shelf.seedDemo(demo: true)
        let real = s.appendingPathComponent("My_Notes.pdf")
        try makePDF(["Moon over water."]).write(to: real)
        let doc = try shelf.importFile(at: real)
        XCTAssertNotEqual(doc.id, ReaderSample.id)
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertEqual(books(s).list().map(\.id), [doc.id])
    }

    func testAnUnreadableIndexIsNeverRewritten() throws {
        let d = scratch(); let s = support()
        try FileManager.default.createDirectory(at: s.appendingPathComponent("books"), withIntermediateDirectories: true)
        let index = s.appendingPathComponent("books/index.json")
        try Data("not json {".utf8).write(to: index)
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertEqual(try String(contentsOf: index, encoding: .utf8), "not json {", "no demo trace was read, so nothing is written")
        XCTAssertTrue(d.bool(forKey: Demo.clearedKey))
    }

    func testTheThreeFilesAreNeverRemoved() throws {
        let d = scratch(); let s = support()
        _ = books(s).seedDemo(demo: true)
        for n in ["onboarding.json", "edition-topics.json", "voice-setup.json", "recent-files.json"] { put(s, n, "{}") }
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        for n in ["onboarding.json", "edition-topics.json", "voice-setup.json", "recent-files.json"] { XCTAssertTrue(exists(s, n), n) }
    }

    func testOnlyTheAddressesDemoProducedLeaveThePagesAndTheSignals() throws {
        let d = scratch(); let s = support()
        put(s, "open-pages.json", pagesJSON)
        let subjects = ["pub:demo-grid-storage", "pub:/real-story", "pub:/demo-river-festival#b2", "file:/demo-notes.pdf", "pub:demo-not-a-fixture"]
        put(s, "signals.jsonl", subjects.map { String(data: try! JSONEncoder().encode(SignalEvent(kind: "open", subject: $0)), encoding: .utf8)! + "\n" }.joined())
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        let pages = try JSONSerialization.jsonObject(with: Data(contentsOf: s.appendingPathComponent("open-pages.json"))) as! [String: Any]
        XCTAssertEqual((pages["pages"] as! [[String: Any]]).map { $0["address"] as! String }, ["radio:", "pub:/real-story", "file:/demo-notes.pdf", "pub:demo-not-a-fixture"])
        XCTAssertEqual(pages["recency"] as! [String], ["pub:/real-story", "file:/demo-notes.pdf", "pub:demo-not-a-fixture", "radio:"])
        let kept = try String(contentsOf: s.appendingPathComponent("signals.jsonl"), encoding: .utf8).split(separator: "\n").map { try JSONDecoder().decode(SignalEvent.self, from: Data($0.utf8)).subject }
        XCTAssertEqual(kept, ["pub:/real-story", "file:/demo-notes.pdf", "pub:demo-not-a-fixture"])
    }

    func testAFailedRemovalLeavesTheMarkerUnsetAndTheNextLaunchRetries() throws {
        let d = scratch(); let s = support()
        put(s, "open-pages.json", pagesJSON)
        try FileManager.default.setAttributes([.posixPermissions: 0o555], ofItemAtPath: s.path)      // the rewrite cannot be written
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertFalse(d.bool(forKey: Demo.clearedKey))
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: s.path)
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertTrue(d.bool(forKey: Demo.clearedKey))
        XCTAssertFalse(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("demo-harbor-barrier"))
    }
}
