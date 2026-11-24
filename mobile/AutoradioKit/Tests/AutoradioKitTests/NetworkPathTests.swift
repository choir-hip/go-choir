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
        defer { WiFiMonitor.shared.onChange = nil }
        final class C: @unchecked Sendable { var n = 0 }; let c = C()
        WiFiMonitor.shared.report(wifi: false)
        WiFiMonitor.shared.onChange = { c.n += 1 }
        WiFiMonitor.shared.report(wifi: true); XCTAssertEqual(c.n, 1); XCTAssertTrue(WiFiMonitor.shared.onWiFi)
        WiFiMonitor.shared.report(wifi: true); XCTAssertEqual(c.n, 1, "no change, no report")
        WiFiMonitor.shared.report(wifi: false); XCTAssertEqual(c.n, 2)
    }
}
