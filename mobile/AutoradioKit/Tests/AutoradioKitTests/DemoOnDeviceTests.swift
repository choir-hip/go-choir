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
    //  3 the sample book goes from the shelf, a real book stays
    //  4 onboarding, edition topics and voice setup go only when no real key is stored
    //  5 open pages and signals lose only the demo addresses; the radio page and real addresses stay
    //  6 a second call is a no-op

    private func support() -> URL {
        let u = FileManager.default.temporaryDirectory.appendingPathComponent("demo-left-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: u, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: u) }
        return u
    }
    private func put(_ dir: URL, _ name: String, _ text: String) { try? Data(text.utf8).write(to: dir.appendingPathComponent(name)) }
    private func exists(_ dir: URL, _ name: String) -> Bool { FileManager.default.fileExists(atPath: dir.appendingPathComponent(name).path) }

    func testADeviceCleansOnceWhateverTheFlagWas() throws {
        let d = scratch(); let s = support()           // no stored flag, no marker: the owner's phone already ran the build that removed it
        let shelf = ReaderShelf(root: s.appendingPathComponent("books")); _ = shelf.seedDemo(demo: true)
        Demo.clearLeftovers(support: s, hasRealKey: false, allowed: false, defaults: d)
        XCTAssertTrue(d.bool(forKey: Demo.clearedKey))
        XCTAssertTrue(ReaderShelf(root: s.appendingPathComponent("books")).list().isEmpty)
        _ = ReaderShelf(root: s.appendingPathComponent("books")).seedDemo(demo: true)        // a second launch does nothing
        Demo.clearLeftovers(support: s, hasRealKey: false, allowed: false, defaults: d)
        XCTAssertEqual(ReaderShelf(root: s.appendingPathComponent("books")).list().count, 1)
    }

    func testNothingIsClearedWhereDemoIsAllowed() throws {
        let d = scratch(); let s = support()
        let shelf = ReaderShelf(root: s.appendingPathComponent("books")); _ = shelf.seedDemo(demo: true)
        put(s, "onboarding.json", "{}"); put(s, "edition-topics.json", "{}"); put(s, "voice-setup.json", "{}")
        Demo.clearLeftovers(support: s, hasRealKey: false, allowed: true, defaults: d)
        XCTAssertEqual(ReaderShelf(root: s.appendingPathComponent("books")).list().count, 1)
        XCTAssertTrue(exists(s, "onboarding.json") && exists(s, "edition-topics.json") && exists(s, "voice-setup.json"))
        XCTAssertFalse(d.bool(forKey: Demo.clearedKey))
    }

    func testTheSampleBookGoesAndARealBookStays() throws {
        let d = scratch(); let s = support()
        let shelf = ReaderShelf(root: s.appendingPathComponent("books"))
        _ = shelf.seedDemo(demo: true)
        let real = s.appendingPathComponent("My_Notes.pdf")
        try makePDF(["Moon over water."]).write(to: real)
        let doc = try shelf.importFile(at: real)
        XCTAssertNotEqual(doc.id, ReaderSample.id)
        Demo.clearLeftovers(support: s, hasRealKey: true, allowed: false, defaults: d)
        let after = ReaderShelf(root: s.appendingPathComponent("books")).list().map(\.id)
        XCTAssertEqual(after, [doc.id])
    }

    func testTheFilesGoOnlyWhenNoRealKeyIsStored() throws {
        for (hasKey, gone) in [(false, true), (true, false)] {
            let d = scratch(); let s = support()
                put(s, "onboarding.json", "{}"); put(s, "edition-topics.json", "{}"); put(s, "voice-setup.json", "{}"); put(s, "recent-files.json", "{}")
            Demo.clearLeftovers(support: s, hasRealKey: hasKey, allowed: false, defaults: d)
            for n in ["onboarding.json", "edition-topics.json", "voice-setup.json"] { XCTAssertEqual(exists(s, n), !gone, "\(n) hasKey=\(hasKey)") }
            XCTAssertTrue(exists(s, "recent-files.json"), "not known to be demo's: left alone")
        }
    }

    func testOnlyDemoAddressesLeaveThePagesAndTheSignals() throws {
        let d = scratch(); let s = support()
        put(s, "open-pages.json", """
        {"pages":[{"address":"radio:","kind":"radio","title":"Autoradio","opener":"","pinned":false},{"address":"pub:demo-harbor-barrier","kind":"pub","title":"Harbor","opener":"","pinned":false},{"address":"pub:/real-story","kind":"pub","title":"Real","opener":"","pinned":false}],"recency":["pub:demo-harbor-barrier","pub:/real-story","radio:"]}
        """)
        let lines = [SignalEvent(kind: "open", subject: "pub:demo-grid-storage"), SignalEvent(kind: "open", subject: "pub:/real-story"),
                     SignalEvent(kind: "close", subject: "pub:/demo-river-festival#b2")]
            .map { String(data: try! JSONEncoder().encode($0), encoding: .utf8)! }.joined(separator: "\n") + "\n"
        put(s, "signals.jsonl", lines)
        Demo.clearLeftovers(support: s, hasRealKey: true, allowed: false, defaults: d)
        let pages = try JSONSerialization.jsonObject(with: Data(contentsOf: s.appendingPathComponent("open-pages.json"))) as! [String: Any]
        XCTAssertEqual((pages["pages"] as! [[String: Any]]).map { $0["address"] as! String }, ["radio:", "pub:/real-story"])
        XCTAssertEqual(pages["recency"] as! [String], ["pub:/real-story", "radio:"])
        let kept = try String(contentsOf: s.appendingPathComponent("signals.jsonl"), encoding: .utf8).split(separator: "\n")
        XCTAssertEqual(kept.count, 1)
        XCTAssertTrue(kept[0].contains("real-story"))
    }
}
