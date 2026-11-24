import XCTest
@testable import AutoradioKit

// The quick-decision job offers more than one model (LFM2.5-350M Q4_K_M beside Qwen3-0.6B). Failure list (each line a test):
//  Q1 the pin is the host's own: the file, its size and its SHA-256 are the Hugging Face API's listing (read 2026-11-24, no download), at the repository commit named in the address.
//  Q2 it is offered under the decision job with its licence limit and without being the recommendation (Qwen3 stays first), and a phone that has none picked keeps Qwen3.
//  Q3 a pick that names a model this build has no file for never becomes the decision model (the default stays).
//  Q4 the prompt is the model's own chat layout: Qwen3 keeps the empty think block it always had; LFM2.5 starts with its start token, has no think block and ends at the assistant turn.
//  Q5 the offer carries the licence the maker asks for, as a credit line, so the settings page shows it (the LFM Open License names no credit, it names a revenue limit: the limit is the note).
final class DecisionModelTests: XCTestCase {
    static let lfm = "lfm2.5-350m-q4_k_m"

    func testThePinIsTheHostsListing() throws {                                                         // Q1
        let e = try XCTUnwrap(KnownModels.extraGGUF.first { $0.spec.id == Self.lfm })
        XCTAssertEqual(e.spec.sha256, "7e6f72643caafc9a68256686638c4d7916f2cec76d1df478d4c3ddcd95a6aed4")
        XCTAssertEqual(e.spec.bytes, 229_312_224)
        XCTAssertEqual(e.spec.fileName, "LFM2.5-350M-Q4_K_M.gguf")
        XCTAssertEqual(e.spec.url.absoluteString, "https://huggingface.co/LiquidAI/LFM2.5-350M-GGUF/resolve/657e078c94084481950a2d555a941481f715536b/LFM2.5-350M-Q4_K_M.gguf")
        XCTAssertTrue(KnownModels.isPinned(Self.lfm))
        XCTAssertTrue(e.license.contains("LFM Open License"))
    }
    func testOfferedUnderDecisionNotRecommended() throws {                                              // Q2, Q5
        let jobs = ModelOffer.jobs(profile: DeviceProfile(chip: "iPhone15,2", memoryGB: 5.6, freeDiskGB: 40, onWiFi: true))
        let d = try XCTUnwrap(jobs.first { $0.job == "decision" })
        let o = try XCTUnwrap(d.options.first { $0.id == Self.lfm })
        XCTAssertEqual(o.bytes, 229_312_224)
        XCTAssertFalse(o.recommended)
        XCTAssertTrue(o.note.contains("US$10M"), "the licence limit is shown beside it: \(o.note)")
        XCTAssertEqual(d.options.first { $0.recommended }?.id, "qwen3-0.6b-q8_0", "Qwen3 stays the recommendation")
        XCTAssertTrue(ModelList.builtIn().models.contains { $0.id == Self.lfm && $0.job == "decision" && !($0.license).isEmpty })
    }
    func testAnUnknownPickNeverBecomesTheDecisionModel() {                                              // Q3
        let d = UserDefaults(suiteName: "decision-tests-\(UUID().uuidString)")!
        XCTAssertEqual(DecisionChoice.resolve(saved: nil), DecisionChoice.defaultID)
        XCTAssertEqual(DecisionChoice.resolve(saved: "no-such-model"), DecisionChoice.defaultID)
        XCTAssertEqual(DecisionChoice.resolve(saved: "lfm3-audio-2b"), DecisionChoice.defaultID, "a model that is not a pinned decision model is refused")
        XCTAssertEqual(DecisionChoice.resolve(saved: Self.lfm), Self.lfm)
        _ = d
    }
    func testPromptsAreEachModelsOwnLayout() {                                                          // Q4
        let args = (context: "A story about a park.", words: "Tell me more", question: "Is it a request?", options: ["yes", "no"], letters: ["A", "B"])
        let qwen = DecisionPrompt.text(modelID: "qwen3-0.6b-q8_0", context: args.context, listenerWords: args.words, question: args.question, options: args.options, letters: args.letters)
        XCTAssertEqual(qwen, "<|im_start|>system\nYou classify what a radio listener said. Reply with the letter of the best option only.<|im_end|>\n<|im_start|>user\nContext: A story about a park.\nListener said: \"Tell me more\"\nQuestion: Is it a request?\nA. yes\nB. no\nAnswer with the letter only.<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n\n")
        let lfm = DecisionPrompt.text(modelID: Self.lfm, context: args.context, listenerWords: args.words, question: args.question, options: args.options, letters: args.letters)
        XCTAssertTrue(lfm.hasPrefix("<|startoftext|><|im_start|>system\n"))
        XCTAssertFalse(lfm.contains("<think>"))
        XCTAssertTrue(lfm.hasSuffix("Answer with the letter only.<|im_end|>\n<|im_start|>assistant\n"))
    }
}
