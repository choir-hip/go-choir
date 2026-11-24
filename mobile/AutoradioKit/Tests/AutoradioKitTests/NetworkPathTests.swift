import XCTest
@testable import AutoradioKit

// The app learns what network it is on (review round 2, B1): the device profile every voice download reads is the real path, not a constant true.
final class NetworkPathTests: XCTestCase {
    func testN1ClassifiesThePath() {
        XCTAssertTrue(WiFiMonitor.classify(satisfied: true, wifi: true, wired: false, expensive: false))
        XCTAssertTrue(WiFiMonitor.classify(satisfied: true, wifi: false, wired: true, expensive: false))
        XCTAssertFalse(WiFiMonitor.classify(satisfied: true, wifi: false, wired: false, expensive: false), "cellular")
        XCTAssertFalse(WiFiMonitor.classify(satisfied: true, wifi: true, wired: false, expensive: true), "a personal hotspot is mobile data")
        XCTAssertFalse(WiFiMonitor.classify(satisfied: false, wifi: true, wired: false, expensive: false), "no connection is not Wi-Fi")
    }
    func testN2TheDeviceProfileFollowsThePathNotAConstant() {
        defer { WiFiMonitor.shared.testOverride = nil }
        WiFiMonitor.shared.testOverride = false
        XCTAssertFalse(DeviceProfile.current().onWiFi, "on mobile data the profile says so: the page asks before anything is fetched")
        WiFiMonitor.shared.testOverride = true
        XCTAssertTrue(DeviceProfile.current().onWiFi)
        XCTAssertFalse(DeviceProfile.current(onWiFi: false).onWiFi, "an explicit answer still wins")
    }
    func testN3APathChangeIsReported() {
        let m = WiFiMonitor(testing: true)                                    // not the live monitor: a real path update cannot fire in the middle
        final class C: @unchecked Sendable { var n = 0 }; let c = C()
        m.onChange = { c.n += 1 }
        m.report(wifi: true); XCTAssertEqual(c.n, 1); XCTAssertTrue(m.onWiFi)
        m.report(wifi: true); XCTAssertEqual(c.n, 1, "no change, no report")
        m.report(wifi: false); XCTAssertEqual(c.n, 2)
    }
}
