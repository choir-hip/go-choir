// The voice cards: tap a voice, hear a line in it, and that is the voice the radio speaks with (first-run's voice step and Settings).
// The logic is the kit's VoiceSampler (with tests); this file holds what only the app can do: play the sample through the radio's own
// audio engine, fetch a model through the model provider, and answer the contract.
import AVFoundation
import Foundation

/// A failure the sampler's `say` reports after the audio has played what there was.
private final class SampleOutcome: @unchecked Sendable {
    private let lock = NSLock()
    private var error: Error?
    private var frames = 0
    func fail(_ e: Error) { lock.lock(); error = e; lock.unlock() }
    func got(_ n: Int) { lock.lock(); frames += n; lock.unlock() }
    func result() -> (Error?, Int) { lock.lock(); defer { lock.unlock() }; return (error, frames) }
}

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
            fetch: { model, onBytes in
                if virtual {                              // no models and no network under the agent: a short, honest-looking wait
                    for i in 1...4 { try await Task.sleep(nanoseconds: 300_000_000); onBytes(Int64(i) * 50_000_000) }
                    return
                }
                guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
                try await p.fetch(model, role: "", onBytes: onBytes)
            },
            verify: { model in
                let r = await verifier.verify(model: model, check: {
                    if virtual { return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: true) }
                    guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }
                    return try await p.check(model, role: "")
                })
                return r.outputOK ? nil : (r.note.isEmpty ? "That voice did not work on this phone. The radio will use the phone's voice." : r.note)
            },
            say: { option in
                if virtual { try await Task.sleep(nanoseconds: 1_200_000_000); return }
                try await VoiceController.play(option, on: rt)
            },
            halt: { rt.audio.stopVoice() },
            choose: { rt.setVoice($0) })
        sampler.onChange = { [weak module] in module?.touch(["voiceCards"]) }
    }

    /// Say the option's sample in THAT voice. There is no fall back here: a card for Heart that played the phone's voice would be a lie.
    static func play(_ option: VoiceOption, on rt: LabRuntime) async throws {
        await rt.start(virtual: false)                            // does nothing when the radio is already running
        var cfg = rt.configuration
        cfg.set(.voice, option.setting)
        let voice = try rt.registry.make(.voice, as: Voice.self, cfg)
        let outcome = SampleOutcome()
        let frames = AsyncThrowingStream<AudioFrame, Error> { c in
            let t = Task {
                do {
                    for try await f in voice.speak(AsyncStream<String> { $0.yield(option.sample); $0.finish() }) { outcome.got(f.samples.count); c.yield(f) }
                } catch { outcome.fail(error) }
                c.finish()
            }
            c.onTermination = { _ in t.cancel() }
        }
        await rt.audio.playVoice(frames)
        let (error, n) = outcome.result()
        if let error { throw error }
        if n == 0 && !Task.isCancelled { throw LabError.unavailable("the voice \(option.id) made no sound") }
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
            await rt.audio.playVoice(frames)
        } catch { rt.log.emit(.error(stage: "voice", message: "\(error)")) }
    }

    func view() -> [String: Any] {
        let rt = module.runtime!
        if let o = VoiceCatalog.option(matching: rt.configuration.setting(.voice) ?? VoiceCatalog.defaultOption.setting) { sampler.syncSelected(o.id) }
        let cards: [[String: Any]] = sampler.cards().map { c in
            ["id": c.id, "name": c.name, "blurb": c.blurb, "sizeLabel": c.sizeLabel, "state": c.state, "note": c.note,
             "progress": c.progress, "selected": c.selected, "playing": c.playing]
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
        handle("voiceStop") { _, reply in c.sampler.stop(); reply(c.view()) }
        // First-run: the voice stays as chosen, the sample stops, the interview is next.
        handle("voiceContinue") { _, reply in
            c.sampler.stop()
            OnboardingController.shared?.voiceNext()
            reply(c.view())
        }
    }
}
