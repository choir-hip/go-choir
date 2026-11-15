import XCTest
@testable import AutoradioKit

final class SourceLinkTests: XCTestCase {
    func testWebPagesOpenInsideTheApp() {
        XCTAssertEqual(SourceLink.route("https://example.org/a?b=1#c"), .inApp(URL(string: "https://example.org/a?b=1#c")!))
        XCTAssertEqual(SourceLink.route("http://example.org/x"), .inApp(URL(string: "http://example.org/x")!))
        XCTAssertEqual(SourceLink.route("  HTTPS://Example.org/x \n"), .inApp(URL(string: "HTTPS://Example.org/x")!))
    }
    func testMailPhoneAndMessageLinksLeaveTheApp() {
        for s in ["mailto:a@example.org", "tel:+15551234", "sms:+15551234"] {
            guard case .external = SourceLink.route(s) else { return XCTFail("\(s) should hand off to the system") }
        }
    }
    func testAnythingElseIsRefused() {
        for s in ["", "   ", "not a url", "javascript:alert(1)", "file:///etc/hosts", "data:text/html,hi", "ftp://example.org/x", "https://", "https:///path", "/relative/path"] {
            XCTAssertEqual(SourceLink.route(s), .refused, s)
        }
    }
}
