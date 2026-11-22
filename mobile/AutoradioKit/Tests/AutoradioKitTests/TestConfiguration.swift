import Foundation
@testable import AutoradioKit

extension StageRegistry {
    /// The baseline configuration the engine tests start from (the lab stand-ins, the on-device speech stages, the system voice). Test-only.
    static func defaultConfiguration() -> LabConfiguration {
        LabConfiguration(name: "baseline", stages: [
            .micSource: .init(id: "builtin"), .addressing: .init(id: "hybrid"), .speechBounds: .init(id: "energy-vad"),
            .speakerCheck: .init(id: "none"), .transcriber: .init(id: "apple-speech"), .yieldPolicy: .init(id: "pause-at-once"),
            .system1: .init(id: "fixed"), .holder: .init(id: "local-ack"), .voice: .init(id: "avspeech"),
            .desk: .init(id: "stub-echo"), .programme: .init(id: "fixture"),
        ])
    }
}
