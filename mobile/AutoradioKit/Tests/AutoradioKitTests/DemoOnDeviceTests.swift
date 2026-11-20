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

    func testSettingItOnWhereItIsNotAllowedStoresNothing() {
        let d = scratch()
        Demo.setOn(true, allowed: false, defaults: d)
        XCTAssertNil(d.object(forKey: Demo.defaultsKey))
    }

    func testTheSimulatorAndTheMacKeepDemoMode() {
        let d = scratch()
        XCTAssertFalse(Demo.isOn(allowed: true, defaults: d, env: [:]))
        Demo.setOn(true, allowed: true, defaults: d)
        XCTAssertTrue(Demo.isOn(allowed: true, defaults: d, env: [:]))
        XCTAssertTrue(Demo.isOn(allowed: true, defaults: scratch(), env: ["AUTORADIO_DEMO": "1"]))
        Demo.setOn(false, allowed: true, defaults: d)
        XCTAssertFalse(Demo.isOn(allowed: true, defaults: d, env: [:]))
    }
}
