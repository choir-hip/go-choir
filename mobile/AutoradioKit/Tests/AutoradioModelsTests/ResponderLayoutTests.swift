import XCTest
@testable import AutoradioKit
@testable import AutoradioModels

// LFM3-Omni-2B's text path reads the same chat layout as LFM3-1.7B (its chat_template.jinja ends `<|im_start|>assistant\n<think></think>` with
// thinking off, and starts with `<|startoftext|>`): a model id that starts with "lfm" gets it (failure O5).
final class ResponderLayoutTests: XCTestCase {
    func testOmniReadsTheLFMLayout() {
        let omni = ChatLayout.forModel("lfm3-omni-2b-q8_0"), lfm = ChatLayout.forModel("lfm3-1.7b-qad-q4_0")
        XCTAssertEqual(omni.begin, "<|startoftext|>")
        XCTAssertEqual(omni.prefix(system: "s", userPrefix: "u"), lfm.prefix(system: "s", userPrefix: "u"))
        XCTAssertEqual(omni.suffix(userSuffix: "q"), "q<|im_end|>\n<|im_start|>assistant\n<think></think>")
    }
}
