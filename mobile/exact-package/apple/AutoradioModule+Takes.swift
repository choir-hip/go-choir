// Takes (station L7; design 11): the Takes section of the Autoradio page. One source, `takesPage`, and one send, `takesAct`, declared in
// interface/takes.json. The state machine is the Kit's (TakeSession.swift); this file holds what only the app can do: take the microphone from
// the radio (the one voice), record to a file, read it with the recogniser, and fetch another listener's take for the run sheet.
//   takesAct op  record | stop | preview | rerecord | discard | publish | delete(arg id) | mute(arg handle) | unmute(arg handle) | refresh
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
                              client: { [weak module] in module == nil ? nil : TakesClient(client: ChoirEnvironment.shared.client()) },
                              micAllowed: { LiveAudio.micPermissionGranted() ? true : await LiveAudio.requestMicPermission() },
                              responds: { TakeResponds.from(entry: RunSheetPlayer.shared.store.onAir()) }, own: .shared, dir: dir)
        preview.onEnd = { [weak self] in self?.session.previewEnded() }
        session.onChange = { [weak self, weak module] in
            module?.touch(["takesPage", "radioPage"], now: true)
            self?.keepTicking()
        }
    }
    private func keepTicking() {
        guard session.state == .recording, ticker == nil else { return }
        ticker = Task { @MainActor [weak self] in
            while let s = self?.session, s.state == .recording {
                await s.tick()
                self?.session.onChangeTick()
                try? await Task.sleep(nanoseconds: 250_000_000)
            }
            self?.ticker = nil
        }
    }
}

extension TakeSession { func onChangeTick() { onChange?() } }

extension AutoradioModule {
    func takesPageInfo() -> TakesPageInfo {
        onMainThread { TakesPage.make(session: TakesHost.shared!.session, mutes: TakeMutes.shared.all, own: .shared) }
    }

    func registerTakes() {
        onMainThread {
            TakesHost.shared = TakesHost(module: self)
            // Another listener's take is fetched like a clip: the publisher's bytes, signed in, into a file the player deletes when the entry ends.
            RunSheetPlayer.shared.takeAudio = { id in
                let path = TakeStore.shared.take(id)?.audioPath ?? Take.audioPath(id)
                guard let data = try? await TakesClient(client: ChoirEnvironment.shared.client()).audio(path: path) else { return nil }
                let f = FileManager.default.temporaryDirectory.appendingPathComponent("take-play-\(id).m4a")
                guard (try? data.write(to: f, options: .atomic)) != nil else { return nil }
                return f
            }
        }
        handle("takesPage") { [unowned self] _, reply in reply(json(takesPageInfo())) }
        handle("takesAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", arg = args["arg"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self, let host = TakesHost.shared else { return }
                let s = host.session
                switch op {
                case "record", "rerecord": await s.start()
                case "stop": await s.stop()
                case "preview": s.togglePreview()
                case "discard": s.discard()
                case "publish": await s.publish()
                case "delete": await s.delete(arg)
                case "mute":
                    if TakeMutes.shared.mute(arg) { SteerQueue.shared.enqueue(.mute(handle: arg)); await SteerQueue.shared.flush() }
                case "unmute": TakeMutes.shared.unmute(arg)
                case "refresh": await s.refresh()
                default: break
                }
                self.touch(["takesPage", "radioPage"], now: true)
                reply(self.json(self.takesPageInfo()))
            }
        }
        Task { @MainActor in await TakesHost.shared?.session.refresh() }
    }
}
