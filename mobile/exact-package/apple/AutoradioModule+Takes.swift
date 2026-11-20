// The take RECORDER's native side (the voice): the radio's gate, the microphone, the file, the recogniser and the preview player. The Takes section itself is shared code (AutoradioLab/take-session.ts,
// takes.ts); this file holds what only the app can do: take the microphone from the radio (the one voice), record to a file, read it with the recogniser, hand the shared code one snapshot
// (the source `takesRecorder`, declared in interface/takes.json) and answer its ops (`takeOp`, public: called across the module boundary from AutoradioLab/modules/apple/ModelsEntry.swift), and fetch another listener's take for the run sheet.
//   ops: takeState | takeStart(minSeconds, maxSeconds, maxBytes) | takeStop | takePreview(mode on|off|toggle) | takePublishing(on) | takeDiscard | takeDone | takeAudio
//        takeSteerMute(handle): the learning engine's steer queue is native (SteerQueue)
//        takeMirror(ids, handle, muted): the Swift programme (RadioProgramme.candidates) still filters the takes it offers by the listener's own ids and muted voices; shared code owns them and pushes a copy
//        takeLegacy: READS the old Swift files (takes-own.json, takes-muted.json) for the one-time carry-over; removes nothing
import AVFoundation
import Foundation

/// The radio gives the microphone to a take: the radio is paused (the same pause a turn makes), hands-free is off for the while, and the
/// session is the capture one. When the take is over everything goes back as it was; a listener's own pause stays a pause.
@MainActor final class TakeGate: TakeRadioGate {
    unowned let module: AutoradioModule
    private var handsFree = false
    init(_ m: AutoradioModule) { module = m }
    func yieldForTake() async -> String? {
        handsFree = HandsFreeSwitch.shared.isOn
        if handsFree { HandsFreeSwitch.shared.set(false) }
        RunSheetPlayer.shared.setLevel(.pause)
        await module.runtime.player.setLevel(.pause)
        do { try await module.runtime.coordinator.setRecordingTake(true) }
        catch { giveBack(); return "the microphone could not be taken (\(error))" }
        return nil
    }
    func giveBack() {
        let hf = handsFree; handsFree = false
        Task { @MainActor in
            try? await module.runtime.coordinator.setRecordingTake(false)
            RunSheetPlayer.shared.setLevel(.play)
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
                              responds: { TakeResponds.from(entry: RunSheetPlayer.shared.store.onAir()) }, dir: dir)
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
            // Another listener's take is fetched like a clip: the publisher's bytes, signed in, into a file the player deletes when the entry ends. Native because the run sheet player is.
            RunSheetPlayer.shared.takeAudio = { id in
                let path = TakeStore.shared.take(id)?.audioPath ?? Take.audioPath(id)
                guard let data = try? await TakesClient(client: ChoirEnvironment.shared.client()).audio(path: path) else { return nil }
                let f = FileManager.default.temporaryDirectory.appendingPathComponent("take-play-\(id).m4a")
                guard (try? data.write(to: f, options: .atomic)) != nil else { return nil }
                return f
            }
        }
        handle("takesRecorder") { [unowned self] _, reply in reply(takesSnapshot()) }
    }

    /// The shared code's calls on the recorder (take-session.ts `nativeDevice`). Each answers the snapshot, except the ones that answer their own thing.
    @MainActor public func takeOp(_ r: [String: Any]) async -> [String: Any] {
        guard let s = TakesHost.shared?.session else { return (r["op"] as? String) == "takeStart" ? ["ended": "open"] : [:] }
        func num(_ k: String, _ d: Double) -> Double { (r[k] as? NSNumber)?.doubleValue ?? d }
        switch r["op"] as? String ?? "" {
        case "takeStart": await s.start(TakeLimits(minSeconds: num("minSeconds", 3), maxSeconds: num("maxSeconds", 59.5), maxBytes: Int(num("maxBytes", 512 * 1024))))
        case "takeStop": await s.stop()
        case "takePreview":
            switch r["mode"] as? String ?? "toggle" {
            case "on": s.setPreview(true)
            case "off": s.setPreview(false)
            default: s.togglePreview()
            }
        case "takePublishing": s.setPublishing((r["on"] as? Bool) ?? false)
        case "takeDiscard": s.discard()
        case "takeDone": s.done()
        case "takeAudio":
            return s.audio.map { ["b64": $0.base64EncodedString() as Any] } ?? [:]
        case "takeSteerMute":
            if let h = r["handle"] as? String, !h.isEmpty { SteerQueue.shared.enqueue(.mute(handle: h)); await SteerQueue.shared.flush() }
            return [:]
        case "takeMirror":
            TakeOwn.shared.replace(ids: r["ids"] as? [String] ?? [], handle: r["handle"] as? String ?? "")
            TakeMutes.shared.replace(r["muted"] as? [String] ?? [])
            return [:]
        case "takeLegacy":
            let ids = TakeOwn.shared.allIDs, handle = TakeOwn.shared.handle, muted = TakeMutes.shared.all
            return ["found": !ids.isEmpty || !handle.isEmpty || !muted.isEmpty, "ids": ids, "handle": handle, "muted": muted]
        default: break
        }
        return s.snapshot
    }
}
