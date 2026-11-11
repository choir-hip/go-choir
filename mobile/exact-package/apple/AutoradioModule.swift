// The Exact side of the lab: AutoradioKit (copied in as Kit/*.swift by scripts/sync-exact-package.sh) behind
// the @choir/autoradio interface. Exact owns the audio session; the kit only asks this adapter for a claim.
import AVFoundation
import ExactPackage
import Foundation
import UIKit

/// Adapts Exact's session claims to the kit's AudioSessionOwner. One standing capture claim at a time:
/// listening (mode default, A2DP, speaker, no voice processing) or talking (voiceChat, HFP, speaker). A change of
/// state is release, then claim, because the first claim's mode wins in Exact (AudioSession.swift:238-262). Options
/// are always explicit: an empty list silently means defaultToSpeaker + allowBluetoothHFP (NativeHatches.swift:239-243).
final class ExactSessionOwner: AudioSessionOwner, @unchecked Sendable {
    private let context: ExactPackageContext
    private var claim: ExactSubscription?
    private var held: AudioState?
    private let note: (String) -> Void
    private let lock = NSLock()
    private var handler: (@Sendable (String, String) -> Void)?
    init(context: ExactPackageContext, note: @escaping (String) -> Void) { self.context = context; self.note = note }

    static func spec(_ s: AudioState) -> (mode: String, options: [String], expected: AVAudioSession.Mode) {
        switch s {
        case .listening: return ("default", ["defaultToSpeaker", "allowBluetoothA2DP"], .default)
        case .talking: return ("voiceChat", ["defaultToSpeaker", "allowBluetoothHFP"], .voiceChat)
        }
    }

    func observe(_ h: @escaping @Sendable (String, String) -> Void) { lock.lock(); handler = h; lock.unlock() }

    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        let ok: Bool = await MainActor.run {
            if claim != nil, held == state { return true }
            claim?.cancel(); claim = nil; held = nil          // release first: the first claim's mode wins
            let spec = Self.spec(state)
            claim = context.audioSession.claim(mode: spec.mode, options: spec.options, background: true, events: { [weak self] e in
                guard let self else { return }
                self.lock.lock(); let h = self.handler; self.lock.unlock()
                h?(e, self.describe())
            })
            guard claim != nil else { return false }
            held = state
            let s = AVAudioSession.sharedInstance()
            note("claimed \(state.rawValue) (\(reason)): \(describe())")
            if s.mode != spec.expected && !context.agent { note("MODE MISMATCH: asked \(spec.mode), session has \(s.mode.rawValue)") }
            return true
        }
        if !ok { throw LabError.unavailable("Exact refused the audio session claim for \(state.rawValue)") }
    }
    func release() {
        DispatchQueue.main.async { [self] in claim?.cancel(); claim = nil; held = nil; note("released") }
    }
    func describe() -> String {
        let s = AVAudioSession.sharedInstance()
        let cat = s.category.rawValue.replacingOccurrences(of: "AVAudioSessionCategory", with: "")
        let mode = s.mode.rawValue.replacingOccurrences(of: "AVAudioSessionMode", with: "")
        return "cat=\(cat) mode=\(mode) in=\(s.currentRoute.inputs.map { $0.portType.rawValue }) out=\(s.currentRoute.outputs.map { $0.portType.rawValue })"
    }
}

public final class AutoradioModule: ExactPackage {
    private let context: ExactPackageContext
    private var runtime: LabRuntime!
    private var owner: ExactSessionOwner!
    private var dirty = Set<String>()
    private var scheduled = false
    private var turnArg: [String: Any] = [:]
    private var choir: ChoirApp!

    public init(context: ExactPackageContext) {
        self.context = context
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        owner = ExactSessionOwner(context: context) { [weak self] s in self?.context.log("AUTORADIO \(s)"); NSLog("AUTORADIO %@", s) }
        runtime = LabRuntime(supportDir: support, session: owner)
        runtime.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["labState", "labAudio", "labEvents", "labTrace", "labTurns", "labStages", "labParams", "labConfigs", "choirPlayer", "choirStories", "choirAuth"]) } }
        // The listener path: sign-in (Keychain), the front page, the real stages.
        choir = ChoirApp(runtime: runtime, keys: KeychainKeyStore(), openURL: { url in DispatchQueue.main.async { UIApplication.shared.open(url) } })
        choir.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["choirPlayer", "choirStories", "choirAuth", "labState"]) } }
        Task { [choir] in await choir?.refresh() }
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
        case "labAudio": reply(json(runtime.audioInfo()))
        case "labStages": reply(["rows": runtime.stageRows().map { json($0) }])
        case "labParams": reply(["rows": runtime.paramRows().map { json($0) }])
        case "labEvents": reply(["lines": runtime.eventLines()])
        case "labTurns": reply(["rows": runtime.turns().map { json($0) }])
        case "labConfigs": reply(["names": runtime.configs.names()])
        case "labTrace":
            let id = (args["turn"] as? String).flatMap { $0.isEmpty ? nil : $0 }
            reply(json(runtime.trace(id: id)))
        case "labStart":
            // Never start the engine inside the reply. Under the agent no device is opened: the engine is virtual.
            // On a phone driven by the agent the real engine runs (that is how the device numbers are measured).
            #if targetEnvironment(simulator)
            let virtual = context.agent
            #else
            let virtual = false
            #endif
            DispatchQueue.main.async { [weak self] in Task { await self?.runtime.start(virtual: virtual) } }
            reply(state())
        case "labInject":
            Task { [weak self] in await self?.runtime.injectFixtureTurn() }
            reply(state())
        case "labSetAckTiming":
            if let t = AckTiming(rawValue: args["timing"] as? String ?? "") { runtime.setAckTiming(t) }
            reply(state())
        case "labSetAudioPolicy":
            if let p = AudioPolicy(rawValue: args["policy"] as? String ?? "") { runtime.setAudioPolicy(p) }
            reply(state())
        case "labExportTurn":
            let id = (args["turn"] as? String).flatMap { $0.isEmpty ? nil : $0 }
            guard let u = runtime.export(id: id) else { return reply(["path": "", "documents": "", "name": ""]) }
            // Also as an app:/ file, for the share sheet (the host resolves the path against the app's grants).
            var shared = ""
            if let d = context.file("app:/data/exports/\(u.lastPathComponent)") {
                try? FileManager.default.createDirectory(at: d.deletingLastPathComponent(), withIntermediateDirectories: true)
                try? FileManager.default.removeItem(at: d)
                if (try? FileManager.default.copyItem(at: u, to: d)) != nil { shared = "app:/data/exports/\(u.lastPathComponent)" }
            }
            reply(["path": shared, "documents": u.path, "name": u.lastPathComponent])
        case "labStop": runtime.stop(); reply(state())
        case "labTalkDown": runtime.talkDown(); reply(state())
        case "labTalkUp": runtime.talkUp(); reply(state())
        case "labPlayPause": if runtime.player.userPaused { runtime.player.play() } else { runtime.player.pause() }; reply(state())
        case "labNext": runtime.player.next(); reply(state())
        case "labPrevious": runtime.player.previous(); reply(state())
        case "labSelectStage":
            if let k = StageKind(rawValue: args["kind"] as? String ?? ""), let id = args["id"] as? String {
                runtime.setStage(k, id: id)
                if k == .programme { Task { [choir] in await choir?.loadProgramme() } }   // a programme picked in the Lab loads now
            }
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
        case "choirAuth": reply(json(choir.authView()))
        case "choirPlayer": reply(json(choir.playerView()))
        case "choirStories": reply(["rows": choir.storyRows().map { json($0) }])
        case "choirSignIn": Task { [choir] in _ = await choir?.signIn.begin() }; reply(state())
        case "choirCancelSignIn": choir.signIn.cancel(); reply(state())
        case "choirSignOut": Task { [choir] in await choir?.signOut() }; reply(state())
        case "choirOpenApproval":
            if let u = URL(string: choir.authView().verificationURL) { DispatchQueue.main.async { UIApplication.shared.open(u) } }
            reply(state())
        case "choirRefresh": Task { [choir] in await choir?.refresh() }; reply(state())
        case "choirPlayPause": Task { [choir] in await choir?.playPause() }; reply(state())
        case "choirTalkDown": Task { [choir] in await choir?.talkDown() }; reply(state())
        case "choirTalkUp": runtime.talkUp(); reply(state())
        case "choirPlay": let r = args["route"] as? String ?? ""; Task { [choir] in await choir?.play(route: r) }; reply(state())
        default: reply(["error": "NotSupportedError"])
        }
    }
}
