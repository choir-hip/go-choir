import Foundation

/// The bundled utterance, read for the checks that run on the phone (voice setup) and on the Mac (the model proof).
/// A separate file because FixtureTurn.swift is generated.
public extension FixtureTurn {
    /// The words the synthesised utterance says (context.json `expectedWords`): "tell me more about the park".
    static let expectedWords = ["tell", "more", "park"]
    /// The microphone audio as 16 kHz mono samples.
    static func micSamples16k() -> [Float]? {
        guard let u = materialise() else { return nil }
        return try? AudioFiles.readMono(u, rate: 16_000)
    }
}
