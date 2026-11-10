// The Exact side of the lab: AutoradioKit (copied in as Kit/*.swift by scripts/sync-exact-package.sh) behind
// the @choir/autoradio interface. Exact owns the audio session; the kit only asks this adapter for a claim.
import AVFoundation
import ExactPackage
import Foundation
import UIKit

/// Adapts Exact's session claims to the kit's AudioSessionOwner. One standing capture claim holds
/// playAndRecord for the whole run; listening/talking are recorded as events (lane K does not switch
/// voice processing, so both states are the same configuration).
final class ExactSessionOwner: AudioSessionOwner, @unchecked Sendable {
    private let context: ExactPackageContext
    private var claim: ExactSubscription?
    private let note: (String) -> Void
    init(context: ExactPackageContext, note: @escaping (String) -> Void) { self.context = context; self.note = note }

    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        await MainActor.run {
            if claim == nil {
                claim = context.audioSession.claim(
                    mode: "default",
                    options: ["defaultToSpeaker", "allowBluetoothA2DP"],
                    background: true,
                    events: { [note] e in note("session event \(e)") })
                let s = AVAudioSession.sharedInstance()
                note("claimed \(state.rawValue) cat=\(s.category.rawValue) mode=\(s.mode.rawValue) out=\(s.currentRoute.outputs.map { $0.portType.rawValue }) in=\(s.currentRoute.inputs.map { $0.portType.rawValue })")
            }
        }
        if await MainActor.run(body: { claim == nil }) { throw LabError.unavailable("Exact refused the audio session claim") }
    }
    func release() { claim?.cancel(); claim = nil }
}

public final class AutoradioModule: ExactPackage {
    private let context: ExactPackageContext
    private var runtime: LabRuntime!
    private var owner: ExactSessionOwner!
    private var dirty = Set<String>()
    private var scheduled = false
    private var turnArg: [String: Any] = [:]

    public init(context: ExactPackageContext) {
        self.context = context
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        owner = ExactSessionOwner(context: context) { [weak self] s in self?.context.log("AUTORADIO \(s)"); NSLog("AUTORADIO %@", s) }
        runtime = LabRuntime(supportDir: support, session: owner)
        runtime.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["labState", "labEvents", "labTrace", "labTurns", "labStages", "labParams", "labConfigs"]) } }
    }

    /// Coalesce change notifications to a few per second.
    private func touch(_ names: [String]) {
        dirty.formUnion(names)
        guard !scheduled else { return }
        scheduled = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) { [weak self] in
            guard let self else { return }
            self.scheduled = false
            let d = self.dirty; self.dirty = []
            for n in d { self.context.changed("autoradio.\(n)") }
        }
    }

    private func json<T: Encodable>(_ v: T) -> [String: Any] {
        guard let d = try? JSONEncoder().encode(v), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else { return [:] }
        return o
    }

    public func later(_ name: String, _ args: [String: Any], preferred: Any?, reply: @escaping ([String: Any]) -> Void) {
        let state = { [unowned self] in self.json(self.runtime.snapshot()) }
        switch name {
        case "labState": reply(state())
        case "labStages": reply(["rows": runtime.stageRows().map { json($0) }])
        case "labParams": reply(["rows": runtime.paramRows().map { json($0) }])
        case "labEvents": reply(["lines": runtime.eventLines()])
        case "labTurns": reply(["rows": runtime.turns().map { json($0) }])
        case "labConfigs": reply(["names": runtime.configs.names()])
        case "labTrace":
            let id = (args["turn"] as? String).flatMap { $0.isEmpty ? nil : $0 }
            reply(json(runtime.trace(id: id)))
        case "labStart": Task { await runtime.start(); DispatchQueue.main.async { reply(state()) } }
        case "labStop": runtime.stop(); reply(state())
        case "labTalkDown": runtime.talkDown(); reply(state())
        case "labTalkUp": runtime.talkUp(); reply(state())
        case "labPlayPause": if runtime.player.userPaused { runtime.player.play() } else { runtime.player.pause() }; reply(state())
        case "labNext": runtime.player.next(); reply(state())
        case "labPrevious": runtime.player.previous(); reply(state())
        case "labSelectStage":
            if let k = StageKind(rawValue: args["kind"] as? String ?? ""), let id = args["id"] as? String { runtime.setStage(k, id: id) }
            reply(state())
        case "labSetParam":
            if let k = StageKind(rawValue: args["kind"] as? String ?? "") {
                runtime.setParam(k, name: args["name"] as? String ?? "", value: args["value"] as? String ?? "")
            }
            reply(state())
        case "labSaveConfig": try? runtime.saveConfiguration(name: args["name"] as? String ?? "unnamed"); touch(["labConfigs"]); reply(state())
        case "labLoadConfig": try? runtime.loadConfiguration(name: args["name"] as? String ?? ""); reply(state())
        case "labLoadConfigURL":
            if let u = URL(string: args["url"] as? String ?? "") { Task { try? await runtime.loadConfiguration(url: u); DispatchQueue.main.async { reply(state()) } } } else { reply(state()) }
        case "labReplay":
            let id = args["turn"] as? String ?? ""
            Task { _ = try? await runtime.replay(id: id); DispatchQueue.main.async { reply(state()) } }
        default: reply(["error": "NotSupportedError"])
        }
    }
}
