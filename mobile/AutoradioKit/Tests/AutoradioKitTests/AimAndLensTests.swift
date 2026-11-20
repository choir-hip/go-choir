import XCTest
@testable import AutoradioKit

// Round 3 merge: the bar's `aim`. (The lens as theme data, L1-L3, is tested in AutoradioLab/themes.test.ts.)
// Failure list (written first):
//  A1 a talk press carries `aim` (choirTalkDown) but the runtime resolves the front page instead: aimedPage() must name the aimed page.
//  A2 no aim (an empty string) must fall back to the page in front now (the resolver gets nil), never to a page of "".
//  A3 an aim the resolver does not know yields no page (the turn runs without one), not a crash or a stale page.
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
