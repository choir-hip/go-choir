// The voice cards: tap a voice, hear a line in it, and that is the voice the radio speaks with (first-run's voice step and Settings).
// The logic is the kit's VoiceSampler (with tests); this file holds what only the app can do: play the sample through the radio's own
// audio engine, and answer the contract. (Every download is Settings > Voice and models' own: a card only shows it.)
import AVFoundation
import Foundation

/// Models marked ready for the simulator driver (the virtual engine), which fetch nothing.
final class VoiceController: @unchecked Sendable {
    static var shared: VoiceController?
    unowned let module: AutoradioModule
    let sampler: VoiceSampler

    init(module: AutoradioModule) {
        self.module = module
        let rt = module.runtime!
        let virtual = module.virtualEngine
        // The ONE readiness writer (`ReadinessVerifier`): the sampler reads readiness and runs a fetched model through the verifier's check; a virtual engine has its own in-memory record.
        let verifier = virtual ? ReadinessVerifier(record: MemoryVoiceReadiness()) : Voices.verifier
        let readiness: VoiceReadiness = verifier.reading
        let current = VoiceCatalog.option(matching: rt.configuration.setting(.voice) ?? VoiceCatalog.defaultOption.setting)?.id ?? VoiceCatalog.defaultOption.id
        sampler = VoiceSampler(
            options: VoiceCatalog.offered(),            // a voice no download group contains is not shown
            selected: current, readiness: readiness,
            say: { option, gate in
                if virtual { try await Task.sleep(nanoseconds: 1_200_000_000); return }
                try await rt.playSample(option, gate: gate)                   // the gate is the press's: LabRuntime.playSample reads it at every step
            },
            halt: { rt.audio.stopVoice() },
            choose: { rt.setVoice($0) },
            running: { OnboardingController.shared?.runningProgress(model: $0) })      // Voice and models downloads every model from its own row; a card only shows that row's progress
        sampler.onChange = { [weak module] in module?.touch(["voiceCards"]) }
    }

    /// Speak one line with the voice the listener has chosen (the neural one; the phone's voice if it is not ready). The radio must be running.
    func speak(_ text: String) async {
        let rt = module.runtime!
        do {
            let voice = try rt.registry.makeSpeaker(rt.configuration, log: rt.log)
            let frames = AsyncThrowingStream<AudioFrame, Error> { c in
                let t = Task {
                    do { for try await f in voice.speak(AsyncStream<String> { $0.yield(text); $0.finish() }) { c.yield(f) } } catch {}
                    c.finish()
                }
                c.onTermination = { _ in t.cancel() }
            }
            await rt.audio.playVoice(frames, gate: .alwaysLive(because: "the radio speaks the listener's chosen voice itself (the hearing check): no Test owns this line"))
        } catch { rt.log.emit(.error(stage: "voice", message: "\(error)")) }
    }

    func view() -> [String: Any] {
        let rt = module.runtime!
        if let o = VoiceCatalog.option(matching: rt.configuration.setting(.voice) ?? VoiceCatalog.defaultOption.setting) { sampler.syncSelected(o.id) }
        let cards: [[String: Any]] = sampler.cards().map { c in
            ["id": c.id, "name": c.name, "blurb": c.blurb, "sizeLabel": c.sizeLabel, "state": c.state, "note": c.note,
             "progress": c.progress, "selected": c.selected, "playing": c.playing, "model": c.model, "group": c.group, "credit": c.credit]
        }
        return ["selected": sampler.selectedID, "message": "", "cards": cards]
    }
}

extension AutoradioModule {
    func registerVoice() {
        let c = VoiceController(module: self)
        VoiceController.shared = c
        handle("voiceCards") { _, reply in reply(c.view()) }
        handle("voicePick") { args, reply in c.sampler.pick(args["id"] as? String ?? ""); reply(c.view()) }
        // Test on a voice (Voice and models): its sample plays, nothing is chosen. Native because the audio engine plays it.
        handle("voiceHear") { args, reply in c.sampler.hear(args["id"] as? String ?? ""); reply(c.view()) }
        handle("voiceStop") { _, reply in c.sampler.stop(); reply(c.view()) }
        // First-run: the voice stays as chosen, the sample stops, the interview is next.
        handle("voiceContinue") { _, reply in
            c.sampler.stop()
            OnboardingController.shared?.voiceNext()
            reply(c.view())
        }
    }
}
