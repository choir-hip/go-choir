// The take RECORDER's native side (the voice): the radio's gate, the microphone, the file, the recogniser, the preview. The Takes section is shared code (AutoradioLab/take-session.ts); this file hands it
// one snapshot (source `takesRecorder`, interface/takes.json) and answers its ops (`takeOp`, public: ModelsEntry.swift calls it) and fetches another listener's take for the run sheet.
//   ops: takeState | takeStart(min/maxSeconds, maxBytes) | takeStop | takePreview(mode) | takePublishing(on) | takeDiscard | takeDone | takeAudio
//        takeMirror (the Swift programme still filters takes by the listener's own ids and muted voices; shared code owns them and pushes a copy, with its account generation: a push older than one already taken was in flight when the account changed and is dropped). The pre-release build's old files are never read.
import AVFoundation
import Foundation

/// The radio gives the microphone to a take: paused (as a turn does), hands-free off, the capture session; when the take is over everything goes back (a listener's own pause stays a pause).
@MainActor final class TakeGate: TakeRadioGate {
    unowned let module: AutoradioModule
    private var handsFree = false
    init(_ m: AutoradioModule) { module = m }
    func yieldForTake() async -> String? {
        handsFree = HandsFreeSwitch.shared.isOn
        if handsFree { HandsFreeSwitch.shared.set(false) }
        RadioBridge.shared.send("level", ["value": "pause"])
        await module.runtime.player.setLevel(.pause)
        do { try await module.runtime.coordinator.setRecordingTake(true) }
        catch { giveBack(); return "the microphone could not be taken (\(error))" }
        return nil
    }
    func giveBack() {
        let hf = handsFree; handsFree = false
        Task { @MainActor in
            try? await module.runtime.coordinator.setRecordingTake(false)
            RadioBridge.shared.send("level", ["value": "play"])
            await module.runtime.player.setLevel(.play)
            if hf { HandsFreeSwitch.shared.set(true) }
        }
    }
}

@MainActor final class TakesHost {
    static var shared: TakesHost?
    let session: TakeSession
    private var ticker: Task<Void, Never>?
    let preview = AVTakePreview()
    init(module: AutoradioModule) {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("takes", isDirectory: true)
        let make: @Sendable () -> Transcriber = { SFSpeechTranscriber(StageParams([:])) }
        session = TakeSession(recorder: AVTakeRecorder(), transcriber: FileTranscriber(make: make), gate: TakeGate(module), preview: preview,
                              micAllowed: { LiveAudio.micPermissionGranted() ? true : await LiveAudio.requestMicPermission() },
                              responds: { TakeResponds.from(entry: RadioBridge.shared.onAirEntry()) }, dir: dir)
        TakeSession.live = session
        preview.onEnd = { [weak self] in self?.session.previewEnded() }
        session.onChange = { [weak self, weak module] in
            module?.touch(["takesRecorder", "radioPage"], now: true)
            self?.keepTicking()
        }
    }
    private func keepTicking() {
        guard session.state == .recording, ticker == nil else { return }
        ticker = Task { @MainActor [weak self] in
            while let s = self?.session, s.state == .recording {
                await s.tick()
                self?.session.onChange?()
                try? await Task.sleep(nanoseconds: 250_000_000)
            }
            self?.ticker = nil
        }
    }
}

extension AutoradioModule {
    func takesSnapshot() -> [String: Any] { onMainThread { TakesHost.shared?.session.snapshot ?? [:] } }

    func registerTakes() {
        onMainThread {
            TakesHost.shared = TakesHost(module: self)
        }
        handle("takesRecorder") { [unowned self] _, reply in reply(takesSnapshot()) }
    }

    /// The shared code's calls on the recorder (take-session.ts `nativeDevice`). Each answers the snapshot, except the ones that answer their own thing.
    @MainActor public func takeOp(_ r: [String: Any]) async -> [String: Any] {
        switch r["op"] as? String ?? "" {      // the ops that need no recorder never depend on it being up
        case "takeMirror": if TakeMirror.accept((r["gen"] as? NSNumber)?.intValue ?? 0) { TakeOwn.shared.replace(ids: r["ids"] as? [String] ?? [], handle: r["handle"] as? String ?? ""); TakeMutes.shared.replace(r["muted"] as? [String] ?? []) }; return [:]
        default: break
        }
        guard let s = TakesHost.shared?.session else { return (r["op"] as? String) == "takeStart" ? ["ended": "open"] : [:] }
        func num(_ k: String, _ d: Double) -> Double { (r[k] as? NSNumber)?.doubleValue ?? d }
        switch r["op"] as? String ?? "" {
        case "takeStart": await s.start(TakeLimits(minSeconds: num("minSeconds", 3), maxSeconds: num("maxSeconds", 59.5), maxBytes: Int(num("maxBytes", 512 * 1024))))
        case "takeStop": await s.stop()
        case "takePreview": if let m = r["mode"] as? String, m != "toggle" { s.setPreview(m == "on") } else { s.togglePreview() }
        case "takePublishing": s.setPublishing((r["on"] as? Bool) ?? false)
        case "takeDiscard": s.discard()
        case "takeDone": s.done()
        case "takeAudio": return s.audio.map { ["b64": $0.base64EncodedString() as Any] } ?? [:]
        default: break
        }
        return s.snapshot
    }
}
