import XCTest
@testable import AutoradioKit

/// What demo mode left on a phone that was in it (demo mode itself is gone).
final class DemoLeftoversTests: XCTestCase {
    private func scratch() -> UserDefaults {
        let name = "demo-test-\(UUID().uuidString)"
        let d = UserDefaults(suiteName: name)!
        addTeardownBlock { d.removePersistentDomain(forName: name) }
        return d
    }

    /// The host this test runs on is the Mac: the cleanup must be skipped here (an inverted guard would run it on every proof lane).
    func testTheCleanupIsSkippedOnTheHost() { XCTAssertTrue(Demo.allowedHere) }

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
    private let pagesJSON = """
    {"pages":[{"address":"radio:","kind":"radio","title":"Autoradio","opener":"","pinned":false},{"address":"pub:demo-harbor-barrier","kind":"pub","title":"Harbor","opener":"","pinned":false},{"address":"pub:/real-story","kind":"pub","title":"Real","opener":"","pinned":false},{"address":"file:/demo-notes.pdf","kind":"file","title":"Notes","opener":"","pinned":false},{"address":"pub:demo-not-a-fixture","kind":"pub","title":"Other","opener":"","pinned":false}],"recency":["pub:demo-harbor-barrier","pub:/real-story","file:/demo-notes.pdf","pub:demo-not-a-fixture","radio:"]}
    """

    func testADeviceCleansOnceWhateverTheFlagWas() throws {
        let d = scratch(); let s = support()           // no stored flag, no marker: the owner's phone already ran the build that removed it
        Demo.clearLeftovers(support: s, allowed: false, defaults: d)
        XCTAssertTrue(d.bool(forKey: Demo.clearedKey))
    }

    func testNothingIsClearedWhereDemoIsAllowed() throws {
        let d = scratch(); let s = support()
        put(s, "open-pages.json", pagesJSON)
        Demo.clearLeftovers(support: s, allowed: true, defaults: d)
        XCTAssertTrue(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("demo-harbor-barrier"))
        XCTAssertFalse(d.bool(forKey: Demo.clearedKey))
    }



    func testTheThreeFilesAreNeverRemoved() throws {
        let d = scratch(); let s = support()
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

    // MARK: the wider cleanup (version 2: its own marker, because phones that ran the first pass have its marker set)
    // Failure list, written before the code:
    //  M1 it runs on a phone whose first-pass marker is already set, once, with its own marker set after the removals; where the cleanup is not allowed (Mac, simulator) nothing happens
    //  M2 open pages and signals lose every kind of demo address (pub, texture, mailbox, mail, the sample book's doc); a real address of a similar name stays
    //  M3 the told-the-paper acts lose only the acts on the five demo stories
    //  M4 the demo theme file goes only when its content is the demo theme, and the chosen-theme default only then; a real theme with the same id stays
    //  M5 own takes lose only the demo ids and the demo handle
    //  M6 the launch flags demo used go (AutoradioDemo, AutoradioDemoSheet, AutoradioOpenOnboarding)
    //  M7 an unreadable file is never rewritten; a failed write leaves the marker unset and the next launch retries

    private let demoTheme = ##"{"schema_version":3,"id":"ember","name":"Ember","schemes":["dark"],"colors":{"paper":"#14100E","paper2":"#211A16","fg":"#F4EBE3","muted":"#C2B1A2","accent":"#FF7A45","accent2":"#FFB48F","success":"#7ED6A0","danger":"#FF8A80","onAccent":"#14100E","rule":"rgba(244,235,227,.18)"},"type":{"read":"ui-serif","ui":"system-ui","mono":"ui-monospace","scale":"compact"},"radii":{"card":"10px","control":"8px","pill":"999px","rules":"0px"},"elevation":{"1":{"shadow":"0 1px 0 rgba(255,122,69,.10), 0 8px 20px rgba(0,0,0,.45)","edge":"inset 0 1px 0 rgba(255,235,220,.10)","press":0.98},"2":{"material":"glass","shadow":"0 10px 28px rgba(0,0,0,.55)","edge":"inset 0 1px 0 rgba(255,235,220,.14)"},"3":{"material":"glass","shadow":"0 24px 60px rgba(0,0,0,.65)","behind":{"scale":0.95,"dim":0.18}}},"motion":{"fast":"110ms ease","enter":"300ms ease-out","sheet":"240ms ease-out","spring":{"stiffness":260,"damping":24,"mass":1},"stagger":"35ms","reduced":{"enter":"0ms","spring":"none"}},"mark":{"drift":"75s","tumble":["3.7s","6.1s"],"flip":{"stiffness":320,"damping":21},"listenScale":1.18,"reduced":"static"}}"##
    private func more(_ s: URL, _ d: UserDefaults, allowed: Bool = false) { Demo.clearMoreLeftovers(support: s, allowed: allowed, defaults: d) }

    func testTheWiderPassRunsOnceOnAPhoneWhoseFirstPassMarkerIsSet() throws {
        let d = scratch(); let s = support()
        d.set(true, forKey: Demo.clearedKey)                                  // the owner's phone: the first pass already ran
        put(s, "open-pages.json", #"{"pages":[{"address":"texture:demo-notes","kind":"reader","title":"N","opener":"","pinned":false}],"recency":["texture:demo-notes"]}"#)
        more(s, d)
        XCTAssertTrue(d.bool(forKey: Demo.clearedMoreKey))
        XCTAssertFalse(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("demo-notes"))
        put(s, "open-pages.json", #"{"pages":[{"address":"texture:demo-notes","kind":"reader","title":"N","opener":"","pinned":false}],"recency":[]}"#)
        more(s, d)                                                              // a second launch does nothing
        XCTAssertTrue(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("demo-notes"))
    }

    func testNothingHappensWhereTheCleanupIsNotAllowed() throws {
        let d = scratch(); let s = support()
        d.set(true, forKey: "AutoradioDemo")
        put(s, "open-pages.json", #"{"pages":[{"address":"mail:m1","kind":"mail","title":"M","opener":"","pinned":false}],"recency":["mail:m1"]}"#)
        more(s, d, allowed: true)
        XCTAssertFalse(d.bool(forKey: Demo.clearedMoreKey)); XCTAssertTrue(d.bool(forKey: "AutoradioDemo"))
        XCTAssertTrue(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("mail:m1"))
    }

    func testEveryKindOfDemoAddressLeavesAndARealOneStays() throws {
        let d = scratch(); let s = support()
        let demo = ["pub:demo-harbor-barrier", "texture:demo-edition@ed-r4", "texture:demo-letter", "mailbox:demo-offline", "mail:m6", "mail:demo-draft-new", "doc:6916a7c31cd3", "file:/Mail/q1/x.pdf"]
        let real = ["radio:", "pub:demo-not-a-fixture", "texture:demo-notes-2", "texture:abc", "mailbox:inbox", "mailbox:demo-spam", "mail:em_01hz08", "mail:m10", "doc:aaaaaaaaaaaa", "file:/Documents/Council minutes.pdf"]
        let pages = (demo + real).map { #"{"address":"\#($0)","kind":"x","title":"t","opener":"","pinned":false}"# }.joined(separator: ",")
        put(s, "open-pages.json", #"{"pages":[\#(pages)],"recency":[\#((demo + real).map { "\"\($0)\"" }.joined(separator: ","))]}"#)
        put(s, "signals.jsonl", (demo + real).map { String(data: try! JSONEncoder().encode(SignalEvent(kind: "open", subject: $0)), encoding: .utf8)! + "\n" }.joined())
        more(s, d)
        let o = try JSONSerialization.jsonObject(with: Data(contentsOf: s.appendingPathComponent("open-pages.json"))) as! [String: Any]
        XCTAssertEqual((o["pages"] as! [[String: Any]]).map { $0["address"] as! String }, real)
        XCTAssertEqual(o["recency"] as! [String], real)
        let kept = try String(contentsOf: s.appendingPathComponent("signals.jsonl"), encoding: .utf8).split(separator: "\n").map { try JSONDecoder().decode(SignalEvent.self, from: Data($0.utf8)).subject }
        XCTAssertEqual(kept, real)
    }

    func testOnlyTheActsOnDemoStoriesLeaveTheToldPaperFile() throws {
        let d = scratch(); let s = support()
        let acts = [("demo-grid-storage", "more"), ("/demo-river-festival", "less"), ("demo-not-a-fixture", "more"), ("real-story", "less")]
            .map { #"{"at":1,"kind":"\#($0.1)","route":"\#($0.0)","title":"T","entities":[],"topic":"x"}"# }.joined(separator: ",")
        put(s, "what-i-told-the-paper.json", #"{"acts":[\#(acts)],"left":"close","right":"more"}"#)
        more(s, d)
        let o = try JSONSerialization.jsonObject(with: Data(contentsOf: s.appendingPathComponent("what-i-told-the-paper.json"))) as! [String: Any]
        XCTAssertEqual((o["acts"] as! [[String: Any]]).map { $0["route"] as! String }, ["demo-not-a-fixture", "real-story"])
        XCTAssertEqual(o["left"] as? String, "close", "the gesture meanings are kept")
    }

    func testTheDemoThemeGoesButARealThemeWithTheSameIdStays() throws {
        for isDemo in [true, false] {
            let d = scratch(); let s = support()
            try FileManager.default.createDirectory(at: s.appendingPathComponent("themes"), withIntermediateDirectories: true)
            put(s, "themes/texture-ember.json", isDemo ? demoTheme : demoTheme.replacingOccurrences(of: "#FF7A45", with: "#00AA00"))
            d.set("ember", forKey: "AutoradioThemeID")
            more(s, d)
            XCTAssertEqual(exists(s, "themes/texture-ember.json"), !isDemo, "demo=\(isDemo)")
            XCTAssertEqual(d.string(forKey: "AutoradioThemeID"), isDemo ? nil : "ember")
        }
        let d = scratch(); let s = support(); d.set("depth", forKey: "AutoradioThemeID")
        put(s, "themes/texture-ember.json", demoTheme)
        more(s, d)
        XCTAssertEqual(d.string(forKey: "AutoradioThemeID"), "depth", "a chosen theme that is not the demo one stays")
    }

    func testTheLaunchFlagsDemoUsedGo() {
        let d = scratch(); let s = support()
        for k in ["AutoradioDemo", "AutoradioDemoSheet", "AutoradioOpenOnboarding"] { d.set("x", forKey: k) }
        d.set("keep", forKey: "AutoradioThemeMode")
        more(s, d)
        for k in ["AutoradioDemo", "AutoradioDemoSheet", "AutoradioOpenOnboarding"] { XCTAssertNil(d.object(forKey: k), k) }
        XCTAssertEqual(d.string(forKey: "AutoradioThemeMode"), "keep")
    }

    func testUnreadableFilesAreNeverRewrittenAndAFailedWriteRetries() throws {
        let d = scratch(); let s = support()
        put(s, "open-pages.json", "not json {"); put(s, "what-i-told-the-paper.json", "[1"); put(s, "takes-own.json", "{")
        more(s, d)
        XCTAssertEqual(try String(contentsOf: s.appendingPathComponent("open-pages.json"), encoding: .utf8), "not json {")
        XCTAssertEqual(try String(contentsOf: s.appendingPathComponent("what-i-told-the-paper.json"), encoding: .utf8), "[1")
        XCTAssertTrue(d.bool(forKey: Demo.clearedMoreKey))
        let d2 = scratch(); let s2 = support()
        put(s2, "open-pages.json", #"{"pages":[{"address":"mail:m1","kind":"mail","title":"M","opener":"","pinned":false}],"recency":[]}"#)
        try FileManager.default.setAttributes([.posixPermissions: 0o555], ofItemAtPath: s2.path)
        more(s2, d2)
        XCTAssertFalse(d2.bool(forKey: Demo.clearedMoreKey))
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: s2.path)
        more(s2, d2)
        XCTAssertTrue(d2.bool(forKey: Demo.clearedMoreKey))
        XCTAssertFalse(try String(contentsOf: s2.appendingPathComponent("open-pages.json"), encoding: .utf8).contains("mail:m1"))
    }
}
