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

// A story loaded as a playing programme, for the desk stage's tests (the app never loads a story this way: the paper's stories are shared code's).
enum ChoirFront {
    /// Resolve one story into a playable document and remember it for the desk.
    public static func load(route: String, client: ChoirClient, maxBlocks: Int = 200, env: ChoirEnvironment = .shared) async throws -> ProgrammeDocument {
        let r = try await client.send("GET", "/api/platform/publications/resolve", query: ["route": route], authenticated: false, route: "/api/platform/publications/resolve")
        let o = try r.object()
        let payload = o["payload"] as? [String: Any]
        let artifact = o["artifact"] as? [String: Any]
        let version = o["version"] as? [String: Any]
        let text = (payload?["text"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? (artifact?["content"] as? String) ?? ""
        let title = ((o["publication"] as? [String: Any])?["title"] as? String) ?? ""
        let blocks = ChoirText.blocks(title: title, text: text, maxBlocks: maxBlocks)
        if ChoirText.paragraphs(text).isEmpty { throw ChoirError.empty("the story \"\(title)\"") }
        let versionID = (version?["id"] as? String) ?? ""
        let manifest = (version?["manifest_hash"] as? String) ?? (payload?["manifest_hash"] as? String) ?? ""
        let id = "choir:" + (versionID.isEmpty ? route : versionID)
        env.setPlaying(PlayingProgramme(programmeID: id, title: title, route: route, publicationVersionID: versionID, manifestHash: manifest, blocks: blocks))
        return ProgrammeDocument(id: id, title: title, blocks: blocks.map { .speech(text: $0) })
    }
}
