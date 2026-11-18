import XCTest
@testable import AutoradioKit

// The microphone is asked for only with voice on (owner: voice is off unless the listener turns it on), on the phone and on the Mac.
final class MicAskTests: XCTestCase {
    func testVoiceOffNeverAsks() {
        XCTAssertFalse(MicAsk.shouldRequest(voiceOn: false, granted: false))
        XCTAssertFalse(MicAsk.shouldRequest(voiceOn: false, granted: true))
    }
    func testVoiceOnAsksOnlyWhenNotGranted() {
        XCTAssertTrue(MicAsk.shouldRequest(voiceOn: true, granted: false))
        XCTAssertFalse(MicAsk.shouldRequest(voiceOn: true, granted: true))
    }
}
