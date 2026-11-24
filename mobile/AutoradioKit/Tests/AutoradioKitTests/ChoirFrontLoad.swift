import Foundation
@testable import AutoradioKit

// The desk tests' way to a playing programme: resolve a paper story into a document and remember it for the desk (the Texture desk stage reads `env.playing`). Production no longer loads a story this way (the radio's stories are
// run sheet entries whose words shared code fetches); only these tests and the kept Texture desk stage use the programme record.
enum ChoirFront {
    /// Resolve one story into a playable document and remember it for the desk.
    static func load(route: String, client: ChoirClient, maxBlocks: Int = 200, env: ChoirEnvironment = .shared) async throws -> ProgrammeDocument {
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

