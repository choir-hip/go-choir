import XCTest
@testable import AutoradioKit

// Round 3 merge: the bar's `aim` and the lens as theme data.
// Failure list (written first):
//  A1 a talk press carries `aim` (choirTalkDown) but the runtime resolves the front page instead: aimedPage() must name the aimed page.
//  A2 no aim (an empty string) must fall back to the page in front now (the resolver gets nil), never to a page of "".
//  A3 an aim the resolver does not know yields no page (the turn runs without one), not a crash or a stale page.
//  L1 a theme without a "lens" key shows the native glass (style A, wash 0); every bundled theme says so explicitly.
//  L2 a theme's lens style and wash reach `Theme` for the scheme in force.
//  L3 an unknown style, or a wash outside 0...1, is refused with a reason (nothing is silently clamped).
final class AimTests: XCTestCase {
    func makeRuntime() -> LabRuntime {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("aim-\(UUID().uuidString)")
        return LabRuntime(supportDir: dir, session: NullSessionOwner())
    }

    func testTheAimedPageIsTheOneTheTalkPressNamed() {                          // A1, A2, A3
        let rt = makeRuntime()
        rt.frontPageResolver = { a in
            guard let a else { return FrontPage(address: "paper:front", title: "Front page", kind: "paper") }
            return a == "pub:aimed" ? FrontPage(address: "pub:aimed", title: "Aimed story", kind: "story") : nil
        }
        XCTAssertEqual(rt.aimedPage()?.address, "paper:front", "no aim: the page in front now")
        rt.aimAddress = "pub:aimed"
        XCTAssertEqual(rt.aimedPage()?.address, "pub:aimed"); XCTAssertEqual(rt.aimedPage()?.title, "Aimed story")
        rt.aimAddress = "pub:unknown"
        XCTAssertNil(rt.aimedPage())
        rt.aimAddress = ""
        XCTAssertEqual(rt.aimedPage()?.address, "paper:front")
    }
}

final class ThemeLensTests: XCTestCase {
    private func themeJSON(lens: Any?, id: String = "lens-test") throws -> Data {
        var o = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(try XCTUnwrap(BundledThemes.json["depth"]).utf8)) as? [String: Any])
        o["id"] = id; o["name"] = "Lens test"
        if let lens { o["lens"] = lens } else { o.removeValue(forKey: "lens") }
        return try JSONSerialization.data(withJSONObject: o)
    }
    private func store() -> ThemeStore {
        ThemeStore(userDir: nil, defaults: UserDefaults(suiteName: "lens-\(UUID().uuidString)")!, systemScheme: { "light" })
    }

    func testEveryBundledThemeSaysNativeGlassAndNoWash() {                      // L1
        for (name, text) in BundledThemes.json {
            let o = try? JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any]
            let lens = o?["lens"] as? [String: Any]
            XCTAssertEqual(lens?["style"] as? String, "A", name); XCTAssertEqual(lens?["wash"] as? Double, 0, name)
        }
        let s = store()
        XCTAssertEqual(s.current().lensStyle, "A"); XCTAssertEqual(s.current().lensWash, 0)
    }

    func testAThemeWithoutALensKeyDefaultsToNativeGlass() throws {              // L1
        let s = store()
        try s.installFromTexture(themeJSON(lens: nil))
        s.setTheme("lens-test")
        XCTAssertEqual(s.current().lensStyle, "A"); XCTAssertEqual(s.current().lensWash, 0)
    }

    func testALensStyleAndWashReachTheTheme() throws {                          // L2
        let s = store()
        try s.installFromTexture(themeJSON(lens: ["style": "B", "wash": 0.55]))
        s.setTheme("lens-test")
        XCTAssertEqual(s.current().lensStyle, "B"); XCTAssertEqual(s.current().lensWash, 0.55)
    }

    func testABadLensIsRefusedWithAReason() throws {                            // L3
        let s = store()
        XCTAssertThrowsError(try s.installFromTexture(themeJSON(lens: ["style": "Z", "wash": 0]))) { XCTAssertTrue("\($0)".contains("lens.style")) }
        XCTAssertThrowsError(try s.installFromTexture(themeJSON(lens: ["style": "A", "wash": 2]))) { XCTAssertTrue("\($0)".contains("lens.wash")) }
        XCTAssertFalse(s.list().contains { $0.id == "lens-test" })
    }
}
