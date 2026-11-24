// First run, native half: THE VOICE. The step machine, the page and what is kept are shared code (AutoradioLab/onboarding.ts); this file is only what the voice side can say and do,
// and each line below has the reason only native code can meet it:
//   facts   `voiceSetupFacts` (one source): the models' setup rows and progress (VoiceSetup.swift, ModelStore: the on-device models), whether the phone is on Wi-Fi, the microphone
//           check's state and level, what it heard, the audio route, whether the voice is listening at the topic screen, how many times the voice cards reported a pick and how many
//           times the voice said "continue" (counters, so shared code can tell a new one: Exact has no call from native code into shared code).
//   calls   `later({op})` from onboarding.ts, answered by `onboardingOp` (ModelsEntry.swift routes the `ob*` ops here):
//             obFacts, obSetupBegin, obSetupRetry, obSetupFallback   download and measure this phone's voice models (VoiceSetup)
//             obMicStart, obMicStop                                  the microphone permission and the one live check
//             obHearingPlay                                          the sentence at the hearing check, in the voice that was set up (the speaker route)
//             obTalk                                                 the voice at the topic screen: the engine's stages (EditionSetupProgramme/Desk) and the open microphone
//             obMirror                                               whether the first run is over: the station may then start by itself (Station.isReady)
//             obNotebookTopics                                       the topics become the first lines of the notebook (NotebookStore: Notebook.swift)
//             obReset, obStop                                        sign-out; leaving the first run
// Nothing is kept here: no file, no step. What a setup record and the responder choice need stays where the models are (voice-setup.json beside the models).
import AVFoundation
import Foundation

final class OnboardingController: @unchecked Sendable {
    static var shared: OnboardingController?
    private static let synth = AVSpeechSynthesizer()

    unowned let module: AutoradioModule
    let support: URL
    private let lock = NSLock()
    private var setup: VoiceSetup?
    /// The stages the running setup was asked to do (empty: all of them).
    private var runStages: [String] = []
    /// stage -> the model id the running (or last) setup was asked to fetch: what a row on Settings > Voice and models is downloading, by its own id.
    private var runModels: [String: String] = [:]
    /// The voice-print model's own fetch is running (it is not a setup stage).
    private var speakerFetching = false
    /// The Test lines (per account) and the last failure of each model's download (`ModelRunBook`). Native because the runs are.
    private let book = ModelRunBook()
    /// The running Tests, so a sign-out cancels them (a Test must not open the microphone or play audio for the next account).
    /// Keyed by (account generation, row): one account's cleanup never removes another account's task.
    private var trialTasks: [String: Task<Void, Never>] = [:]
    /// Learning the person's voice (voice onboarding; Voice/VoiceEnrolment.swift): the sentences, the profile, the check.
    let capture = EnrolCapture(gate: .shared)
    /// The ONE owner of the microphone for enrolment (AutoradioKit Voice/SetupMicLease.swift): every open has its close on every exit, and between sentences the microphone is off and the engine's frames go to the tap alone.
    private var takeSerial = 0
    private lazy var lease: SetupMicLease = {
        let m = module
        let l = SetupMicLease(.init(
            ask: { await LiveAudio.requestMicPermission() },      // the one system prompt: shared code showed its plain line first (the learn page: Continue / Not now)
            openEngine: { let rt = m.runtime!; await rt.start(virtual: m.virtualEngine); rt.audio.micMayRetry() },
            setTap: { tap in await m.runtime!.setEnrolmentTap(tap) },
            micOn: { on in m.runtime!.setHandsFree(on, persist: false) },
            restore: { m.runtime!.restoreHandsFree() }))
        SetupMicLease.live = l
        return l
    }()
    /// The first run is over (done or skipped): the station may start by itself. Told by shared code (obMirror).
    private var over = false
    /// A voice setup is running OR has run and not yet applied its choices (a Remove waits for both: the model about to become the voice must not be erased in the gap).
    private var applying = false
    var setupRunning: Bool { lock.lock(); defer { lock.unlock() }; return applying }
    var finished: Bool { lock.lock(); defer { lock.unlock() }; return over }
    private var mic = "unasked"          // unasked | checking | heard | refused | silent
    private var micActive = false
    private var level = 0.0
    private var heard = ""
    private var route = ""
    private var talking = false
    private var picked = 0
    private var finishAsked = 0

    var recordFile: URL { support.appendingPathComponent("voice-setup.json") }

    init(module: AutoradioModule, support: URL) {
        self.module = module; self.support = support
        capture.onChange = { [weak self] in self?.changed() }
        TurnGate.shared.keyPresent = { ChoirEnvironment.shared.keys.get(KeyAccount.apiKey) != nil }      // a turn needs an account's key: signed out the radio only listens
        WiFiMonitor.shared.onChange = { [weak self] in self?.changed() }      // the path moved: shared code asks the facts again
        EditionTalk.shared.onFinish = { [weak self] in self?.set { self?.finishAsked += 1 } }
    }

    func changed() { module.touch(["voiceSetupFacts"]) }
    private func set(_ f: () -> Void) { lock.lock(); f(); lock.unlock(); changed() }

    // MARK: Voice setup

    /// `only` false: the whole voice setup (the configurator plans the pipeline together, unless the person picked models). `only` true: just `stages` (`["voice"]` is the radio's speaking voice, asked for first and alone: owner, 2026-11-23), each on its own ladder.
    /// `picks` (stage -> model id): the models the person chose in the download step (the list's recommendation when they chose none), tried first on their stage; with picks the configurator, which would choose other models, is not used (the person's choice is what is fetched).
    /// `speaker`: also fetch the voice-print model (about 8 MB) the person agreed to with the rest, which is what learning their voice needs.
    /// `row`: the press was ONE model's Download on Settings > Voice and models: it fails as itself, at once (no wait for a choice, no built-in floor marked ready in its place; `VoiceSetup.rowDownload`).
    func begin(stages: [String] = [], only: Bool = false, picks: [String: String] = [:], speaker: Bool = false, row: Bool = false) {
        if speaker { fetchSpeakerModel() }
        if only && stages.isEmpty { return }
        lock.lock()
        if let s = setup, s.isStarted, !s.isFinished { lock.unlock(); return }
        lock.unlock()
        // What was shown is what is fetched: a pick this build cannot fetch is not run, and its stage does not fall back to a model nobody was shown (ModelOffer.plan).
        let planned = ModelOffer.plan(stages: stages, picks: picks)
        let run = planned.run
        if only && run.isEmpty { return }
        // The one shared store, with the whole catalog in it from the start.
        let backend: SetupBackend = LiveSetupBackend(store: .shared)
        let profile = DeviceProfile.current()
        let stall = 20.0, patience = 45.0
        // The configurator picks one model per role for THIS phone, the whole pipeline resident together, by measuring it: only when the person did not choose.
        let configurator: ConfigBackend? = (only || !picks.isEmpty) ? nil : LiveConfigBackend()
        var s = VoiceSetup(backend: backend, profile: profile, smaller: false, stallSeconds: stall, patience: patience, rowDownload: row, configBackend: configurator)
        if !s.fitsOnDisk { s = VoiceSetup(backend: backend, profile: profile, smaller: true, stallSeconds: stall, patience: patience, rowDownload: row, configBackend: configurator) }
        if only { for (st, c) in planned.chosen { s.prefer(st, c) } }
        else { for (job, id) in picks { if let st = SetupStage(rawValue: job), let c = SetupCatalog.ladder(st).first(where: { $0.id == id }) ?? ModelOffer.candidate(job: job, id: id) { s.prefer(st, c) } } }
        s.onChange = { [weak self] in self?.changed() }
        set { setup = s; runStages = only ? stages : []; runModels = Dictionary(uniqueKeysWithValues: planned.chosen.map { ($0.key.rawValue, $0.value.id) }); applying = true }
        let gen = AccountWipe.generation
        Task { [weak self, s] in
            await s.run(only: only ? run : nil)
            self?.setupFinished(s, gen: gen, merging: only)
        }
    }

    /// The voice-print model (WeSpeaker, about 8 MB) is fetched only after the person agreed on the download step; once loaded the gate can listen.
    func fetchSpeakerModel() {
        set { speakerFetching = true }
        let gen = AccountWipe.generation
        Task { let e = WeSpeakerEmbedder.standard(); if await e.ensureReady() { SpeakerGate.shared.embedder = e }      /* the model is the phone's, not the account's (the print is forgotten at sign-out): a late success is kept */ else if gen == AccountWipe.generation { self.book.speakerFailed("The voice-print model could not be fetched. Press Download to try again.", generation: gen) }; self.set { self.speakerFetching = false } }
    }

    private func setupFinished(_ s: VoiceSetup, gen: Int, merging: Bool = false) {
        defer { lock.lock(); applying = false; lock.unlock() }          // the choices are applied (or the account changed): a Remove may run again
        // (The models are on the phone, not in the account: a sign-out while they download does not undo them, so the record is kept whatever the generation says; the choices below are applied only for the account that began it.)
        var record = s.record
        if merging, let old = ModelOffer.readRecord(recordFile) {          // a voice-only run keeps what an earlier full run recorded for the other stages
            record.items = old.items.filter { o in !record.items.contains { $0.stage == o.stage } } + record.items
        }
        if let d = try? JSONEncoder().encode(record) {
            try? FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
            try? d.write(to: recordFile, options: .atomic)
        }
        // Readiness was written by setup itself, after each check passed (VoiceSetup.runStage); the launch choice and this one both read it.
        guard gen == AccountWipe.generation else { changed(); return }
        // The first run's choices become the running ones; a row's Download applies none (Download only fetches and verifies, only Use changes what is in use): `LabRuntime.applySetupChoices`.
        module.runtime.applySetupChoices(s)
        // A row that failed keeps its reason on its row until it is pressed again (`ModelRunBook`).
        for r in s.rows where r.state == "failed" { lock.lock(); let id = runModels[r.stage]; lock.unlock(); if let id { book.failed(id, r.statusLine, generation: gen) } }
        changed()
    }

    func retry(_ stage: String) { if let st = SetupStage(rawValue: stage) { setup?.retry(st) } }
    func fallback(_ stage: String) { if let st = SetupStage(rawValue: stage) { setup?.useFallback(st) } }

    // MARK: Microphone, in context, with one live check

    func askMicrophone() {
        set { mic = "checking"; micActive = true; level = 0; heard = "" }
        Task { [weak self] in
            guard let self else { return }
            let granted = await LiveAudio.requestMicPermission()              // the one system prompt
            self.lock.lock(); let here = self.micActive; self.lock.unlock()
            if !here { return }                                               // the page was left while the prompt was up: the engine and the switch are not touched
            if !granted { self.set { self.mic = "refused"; self.micActive = false }; return }
            // One live check: start the engine, listen on this screen only (hands-free is off by default and the engine drops every frame
            // while it is off), feed the meter from the real input level, and pass on the first transcript or on a sustained level
            // (`MicCheck`). When it ends, hands-free goes back to the stored state. UNPROVEN on a device.
            Task { [weak self] in
                guard let self else { return }
                let rt = self.module.runtime!
                let meter = MicCheckMeter()
                let before = rt.snapshot().transcript
                meter.begin(transcript: before)                  // an earlier turn's words are not this check's
                let frames = rt.bus.mic()
                let feeder = Task { for await f in frames { meter.frame(db: f.rmsDb, seconds: f.duration) } }
                await rt.start(virtual: self.module.virtualEngine)
                rt.audio.micMayRetry()                           // the listener asks to be heard now: a give-up from an earlier run must not read as this check's "silent" (review N1)
                if !HandsFreeSwitch.shared.isOn { rt.setHandsFree(true, persist: false) }
                TurnGate.shared.hold()                           // what is said to prove the microphone works is never sent as a turn
                defer { feeder.cancel(); TurnGate.shared.release(); rt.restoreHandsFree() }
                while meter.verdict == .listening {
                    self.lock.lock(); let here = self.micActive; self.lock.unlock()
                    if !here { return }                                                  // shared code left the step: end the check (defer restores)
                    try? await Task.sleep(nanoseconds: 100_000_000)
                    meter.tick(0.1)
                    // The microphone could not be opened (not allowed, or no input after the bounded retries): say so now, not after twenty seconds of nothing.
                    if let why = MicLog.onboardingState(rt.audio.micState) { self.set { self.mic = why; self.micActive = false }; return }
                    meter.transcript(rt.snapshot().transcript)
                    let l = meter.level
                    self.set { self.level = l }
                }
                if meter.verdict == .heard {
                    rt.cancelTalk()                                                      // a phrase spoken here must not become a turn to the desk
                    let t = rt.snapshot().transcript
                    self.set { self.heard = (t.isEmpty || t == before) ? "" : t; self.mic = "heard"; self.micActive = false }
                } else {
                    self.set { self.mic = "silent"; self.micActive = false }
                }
            }
        }
    }

    // MARK: Learning the person's voice (native only for the microphone's frames and the embedding model; the page, the sentences and the order of it all are shared code, voicelearn.ts)

    /// One sentence is about to be read. The lease opens the microphone for it alone and shuts it when it ends; the frames go to the capture and the engine looks for no turn in them (`TurnEngine.enrolmentTap`), so nothing read here can become a request,
    /// with hands-free on or off. If the page is left, or the take given up, while the system prompt is still up, the lease opens nothing. UNPROVEN on a device.
    func beginCapture() {
        guard capture.begin() else { changed(); return }
        lock.lock(); takeSerial += 1; let mine = takeSerial; lock.unlock()
        Task { [weak self] in
            guard let self else { return }
            let out = await self.lease.begin(wanted: { [weak self] in self?.capture.state == .listening }, onFrame: { [weak self] f in
                guard let self else { return }
                self.capture.feed(f)
                if self.capture.state != .listening { Task { [weak self] in guard let self else { return }; self.lock.lock(); let current = self.takeSerial == mine; self.lock.unlock(); if current { await self.lease.endTake() } } }
            })
            switch out {
            case .refused: self.set { self.mic = "refused" }; self.capture.fail("The microphone is off for Autoradio 1.")
            case .stale: break
            case .open:
                // A take that hears nothing (no sound, the engine did not start) ends by itself as a take that was not heard.
                try? await Task.sleep(nanoseconds: 9_000_000_000)
                self.lock.lock(); let current = self.takeSerial == mine; self.lock.unlock()
                if current, self.capture.state == .listening { self.capture.expire(); await self.lease.endTake() }
            }
        }
    }

    /// Leaving the page, "Not now", a stored or failed enrolment and sign-out: nothing in progress is kept (the stored voice is not touched) and the microphone is given back, unconditionally.
    func discardEnrolment() async {
        lock.lock(); takeSerial += 1; lock.unlock()
        capture.discard()
        await lease.release()
    }

    func stopMicrophone() { lock.lock(); micActive = false; lock.unlock() }

    /// The hearing check: the route it plays on, and the sentence, in the voice that was just set up. The radio's engine is running after the microphone check; if it is not,
    /// the phone's own voice says it (the check is about the speaker, and it must not stay silent).
    func playHearing() {
        stopMicrophone()
        set { route = "Playing on " + Platform.outputName() }
        if !module.virtualEngine {
            if module.runtime.snapshot().running, let v = VoiceController.shared {
                Task { await v.speak("Can you hear this?") }
            } else {
                DispatchQueue.main.async { Self.synth.speak(AVSpeechUtterance(string: "Can you hear this?")) }
            }
        }
    }

    /// The voice cards report a pick (the voice step is done): shared code sees the counter move.
    func voiceNext() { set { picked += 1 } }

    // MARK: The voice at the topic screen

    /// Say it instead of tapping: the engine listens until shared code stops it. The words are heard by EditionTalk and applied by shared code (edition-picks.ts).
    func talk(_ on: Bool) {
        lock.lock(); let was = talking; lock.unlock()
        if !on { if was { stopVoice() }; return }
        if was { return }
        let rt = module.runtime!
        rt.setStage(.programme, id: EditionSetupProgramme.stageID)
        rt.setStage(.desk, id: EditionSetupDesk.stageID)
        rt.setAudioPolicy(.alwaysTalking)
        set { talking = true }
        Task { [weak self] in
            guard let self else { return }
            await rt.start(virtual: self.module.virtualEngine)
            if !HandsFreeSwitch.shared.isOn { rt.setHandsFree(true, persist: false) }       // off: the reader tapped it, so it listens on this screen only
            await self.module.choir.loadProgramme()
        }
    }

    /// Back to the radio's own stages, and the setup's one-line programme out of the player.
    private func stopVoice() {
        lock.lock(); let was = talking; talking = false; lock.unlock()
        if was {
            let rt = module.runtime!
            rt.setStage(.desk, id: AutoradioDeskStage.stageID)
            rt.setStage(.programme, id: RadioProgramme.stageID)
            rt.player.pause()
            rt.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
            rt.restoreHandsFree()                                                           // back to the stored state
            EditionTalk.shared.clear()                                                      // words heard and not yet applied are not applied later
        }
        changed()
    }

    func stop() { stopVoice(); stopMicrophone(); Task { await discardEnrolment() } }

    /// Sign-out: nothing of the voice side's first run reaches the next account.
    func reset() {
        stop(); DownloadDoor.shared.reset(); SpeakerGate.shared.forget(); module.runtime.setHandsFree(false)           // sign-out: the microphone is closed and the switch stays off for the next person
        book.reset()
        lock.lock(); let running = trialTasks; trialTasks = [:]; lock.unlock(); for (_, t) in running { t.cancel() }
        module.runtime.audio.stopVoice(); VoiceController.shared?.sampler.stop()          // a sample or a Test voice still sounding when the account leaves is stopped
        set { setup = nil; runModels = [:]; runStages = []; speakerFetching = false; mic = "unasked"; level = 0; heard = ""; route = ""; talking = false; over = false; picked = 0; finishAsked = 0 }
    }

    func mirror(finished f: Bool) { lock.lock(); over = f; lock.unlock() }

    // MARK: Voice and models: the download's progress, Use and Test (native because the files, the choices the runtime reads, and the models themselves are)

    /// The progress (0...1) of the download the row of this model is running, or nil. The voice cards show it for every voice of the model.
    func runningProgress(model: String) -> Double? {
        lock.lock(); defer { lock.unlock() }
        guard let stage = runModels.first(where: { $0.value == model })?.key, let row = setup?.rows.first(where: { $0.stage == stage }) else { return model == ModelOffer.speakerID && speakerFetching ? 0 : nil }
        return ["downloading", "waiting", "preparing", "stalled"].contains(row.state) ? row.progress : nil
    }

    /// Make a model on the phone the one in use for its job (`LabRuntime.useModel`): the choice is kept and the stage that runs it is set. A model that is not on the phone changes nothing.
    func use(job: String, id: String, live: Bool) {
        if module.runtime.useModel(job: job, id: id, live: live), job == "voice", let o = VoiceCatalog.options.first(where: { $0.modelID == id }) { VoiceController.shared?.sampler.syncSelected(o.id) }
        changed()
    }

    /// The Test button: one run, reported through the facts. A second press while it runs does nothing; a report from before a sign-out is dropped (`ModelRunBook`).
    func test(job: String, id: String) {
        let key = "\(job):\(id)"
        if book.isRunning(key) { return }
        let gen = AccountWipe.generation; let gate = TrialGate(generation: gen); let taskKey = "\(gen)|\(key)"
        book.test(key, state: "working", line: "", ms: 0, generation: gen); changed()
        let task = Task { [weak self] in
            guard let self else { return }
            await ModelTrial.run(job: job, id: id, backend: LiveTrialBackend(controller: self), gate: gate, report: { state, line, ms in self.book.test(key, state: state, line: line, ms: ms, generation: gen); self.changed() })
            self.lock.lock(); self.trialTasks[taskKey] = nil; self.lock.unlock()
        }
        lock.lock(); trialTasks[taskKey] = task; lock.unlock()
        if !gate.live { task.cancel() }                  // the account left between the press and here
    }

    /// The microphone for `seconds`, through the same lease the voice learning uses (`SetupMicLease.record`: the frames go to this recording alone, and the lease is given back whole on every exit). nil when the microphone is refused.
    func recordAudio(seconds: Double, gate: TrialGate) async -> [Float]? {
        await lease.record(seconds: seconds, gate: gate)
    }

    // MARK: What shared code sees

    func facts() -> [String: Any] {
        lock.lock(); defer { lock.unlock() }
        let rows: [[String: Any]] = (setup?.rows ?? []).map { r in
            ["stage": r.stage, "name": r.name, "model": r.model, "sizeLabel": r.sizeLabel, "state": r.state, "progress": r.progress, "statusLine": r.statusLine,
             "isFallback": r.isFallback, "canRetry": r.canRetry, "canFallback": r.canFallback, "numbers": r.numbers]
        }
        var profile = setup?.profile ?? DeviceProfile.current()
        profile.onWiFi = WiFiMonitor.shared.onWiFi                      // always the live path: a profile made when the press began is old
        // What voice needs from the network, read from the model lists (ModelOffer): per file, its name and size, and whether it is on this phone. Shared code states the sizes and the host; no string has a model's name or size in it.
        let readyChoices0 = ModelOffer.readyChoices(isReady: { Voices.readiness.isReady(model: $0) })      // the ONE readiness record, the engine's too
        let ready = ModelOffer.readyStages(choices: readyChoices0)
        var readyChoices = readyChoices0
        if WeSpeakerEmbedder.standard().onDisk { readyChoices["speaker"] = ModelOffer.speakerID }
        let inUse = ModelInUse.ids(config: module.runtime.configuration, registry: module.runtime.registry, responder: ResponderChoice.current, decision: DecisionChoice.current)
        let offers = ModelOffer.jobs(profile: profile, ready: readyChoices, inUse: inUse, chosen: ["responder": ResponderChoice.current, "decision": DecisionChoice.current], isReady: { Voices.readiness.isReady(model: $0) }).map { $0.dict }
        var runs: [[String: Any]] = (setup?.rows ?? []).compactMap { r in
            guard let id = runModels[r.stage], r.state != "ready" else { return nil }
            return ["id": id, "state": r.state == "stalled" ? "downloading" : r.state, "progress": r.progress, "line": r.statusLine]
        }
        if speakerFetching { runs.append(["id": ModelOffer.speakerID, "state": "downloading", "progress": 0.0, "line": ""]) }
        runs = book.runs(current: runs, isReady: { $0 == ModelOffer.speakerID ? WeSpeakerEmbedder.standard().onDisk : Voices.readiness.isReady(model: $0) })
        let trials: [[String: Any]] = book.tests().map { ["key": $0.key, "state": $0.state, "line": $0.line, "ms": $0.ms] }
        var device = ""
        if let p = setup?.profile { device = p.chip + ", " + String(format: "%.0f", p.memoryGB) + " GB" }
        return ["rows": rows, "summary": setup?.summary ?? [],
                "totalLabel": setup?.totalLabel ?? SetupCatalog.totalLabel(bytes: SetupCatalog.totalBytes(SetupCatalog.plan(for: profile)), onWiFi: profile.onWiFi),
                "offers": offers, "modelRuns": runs, "modelTests": trials, "enrol": capture.facts(), "runStages": runStages, "voiceReady": ready.contains("voice"), "hearingReady": ready.contains("transcriber") && ready.contains("bounds"),
                "device": device, "onWiFi": profile.onWiFi, "setupStarted": setup?.isStarted ?? false, "setupFinished": setup?.isFinished ?? false,
                "mic": mic, "level": level, "heard": heard, "route": route, "talking": talking, "voicePicked": picked, "finishAsked": finishAsked]
    }
}

/// `onboarding:*` pages: the first-run setup and the edition's edit page. The page itself is shared code (onboarding.contract); this only names it for the native page list.
struct OnboardingPageSource: PageSource {
    func handles(_ address: String) -> Bool { address.hasPrefix("onboarding:") }
    func load(_ address: String) async throws -> PageModel {
        PageModel(address: address, kind: "onboarding", title: address == "onboarding:edit" ? "Edit my edition" : "Make it yours", status: "ready")
    }
}

extension AutoradioModule {
    func registerOnboarding() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let c = OnboardingController(module: self, support: support)
        OnboardingController.shared = c
        StageRegistry.registerEditionSetupStages(runtime.registry)
        pageSources.register(OnboardingPageSource())
        choir.accountWipe.register("onboarding") { [weak c] in c?.reset() }
        handle("voiceSetupFacts") { _, reply in reply(c.facts()) }
    }

    /// The narrow calls shared code (onboarding.ts) makes; see the header of this file for each one's reason.
    @MainActor public func onboardingOp(_ r: [String: Any]) async -> [String: Any] {
        guard let c = OnboardingController.shared else { return [:] }
        switch r["op"] as? String ?? "" {
        case "obFacts": return c.facts()
        case "obSetupBegin": c.begin(stages: (r["stages"] as? [String]) ?? [], only: (r["only"] as? Bool) ?? false, picks: (r["picks"] as? [String: String]) ?? [:], speaker: (r["speaker"] as? Bool) ?? false, row: (r["row"] as? Bool) ?? false)
        // The system's microphone permission, read and nothing asked (shared code decides whether to show its plain line first: micask.ts). Native because only the platform knows it.
        // Shared code's consent record, as it stands: native's ONE download door refuses every model fetch without it (DownloadDoor). Native because the transport is.
        case "obConsent": DownloadDoor.shared.sync(agreed: (r["agreed"] as? [String: String]) ?? [:])
        // The person's Remove on a downloaded model: everything of it goes, except the model in use (ModelDeletion.remove). Native because the files are.
        case "obRemoveModel":
            let id = r["id"] as? String ?? ""
            let m = c.module
            let removal = await ModelDeletion.remove(model: id, pin: ModelPins.current(id),
                                                     inUse: { ModelInUse.ids(config: m.runtime.configuration, registry: m.runtime.registry, responder: ResponderChoice.current, decision: DecisionChoice.current) },
                                                     busy: { OnboardingController.shared?.setupRunning ?? false })
            c.changed()
            return ["result": "\(removal)"]
        // Voice and models, Use: the model the person chose for a job is the one the phone uses (native keeps the choice with the runtime that reads it).
        case "obUseModel": c.use(job: r["job"] as? String ?? "", id: r["id"] as? String ?? "", live: (r["live"] as? Bool) ?? false)
        // Voice and models, Test: one run of one model, which needs the model itself (it loads it, listens, speaks, answers). The result comes back through the facts (modelTests).
        case "obModelTest": c.test(job: r["job"] as? String ?? "", id: r["id"] as? String ?? "")
        case "obMicState": return ["state": MicPermissionWord.current()]
        case "obEnrolBegin": c.beginCapture()
        case "obEnrolKeep": return ["kept": c.capture.keep(), "usable": c.capture.usable]
        case "obEnrolBuild": return ["ok": await c.capture.build()]
        case "obEnrolCheck": return ["call": await c.capture.check()]
        case "obEnrolStore": let ok = c.capture.store(); await c.discardEnrolment(); return ["ok": ok]
        case "obEnrolDiscardTake": c.capture.dropTake()
        case "obEnrolDiscard": await c.discardEnrolment()
        case "obVoiceForget": await c.discardEnrolment(); SpeakerGate.shared.forget()
        case "obSetupRetry": c.retry(r["stage"] as? String ?? "")
        case "obSetupFallback": c.fallback(r["stage"] as? String ?? "")
        case "obMicStart": c.askMicrophone()
        case "obMicStop": c.stopMicrophone()
        case "obHearingPlay": c.playHearing()
        case "obTalk": c.talk((r["on"] as? Bool) ?? false)
        case "obStop": c.stop()
        case "obReset": c.reset()
        case "obMirror": c.mirror(finished: (r["finished"] as? Bool) ?? false)
        case "obNotebookTopics":
            let gen = AccountWipe.generation
            _ = await NotebookStore.shared.writeTopics((r["topics"] as? [String]) ?? [], generation: gen)       // a failure here is quiet: the page retries
        default: break
        }
        return [:]
    }
}


/// The live half of ModelTrial (the Test button on a model's row). Native only because each call needs the model itself or the microphone and the speaker.
struct LiveTrialBackend: TrialBackend {
    let controller: OnboardingController
    private var provider: ModelProvider { get throws { guard let p = ModelProviders.current else { throw LabError.unavailable(ModelProviders.missingNote) }; return p } }
    func record(seconds: Double, gate: TrialGate) async -> [Float]? { await controller.recordAudio(seconds: seconds, gate: gate) }
    func transcribe(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> String { guard gate.live else { throw CancellationError() }; return try await provider.transcribe(id, audio) }
    func speechSpan(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> (start: Double?, end: Double?) { guard gate.live else { throw CancellationError() }; return try await provider.speechSpan(id, audio) }
    func answer(_ id: String, job: String, prompt: String, gate: TrialGate) async throws -> String { guard gate.live else { throw CancellationError() }; return try await provider.answer(id, job: job, prompt: prompt) }
    func speak(_ id: String, _ sentence: String, gate: TrialGate) async throws -> Double {
        guard let option = VoiceCatalog.options.first(where: { $0.modelID == id }) else { throw LabError.badInput("this model has no voice to speak with") }
        var o = option; o.sample = sentence
        let rt = controller.module.runtime!
        // A cancelled Test (the account left) stops the sound at once: the audio engine's own cancel only lets go of the waiter, so the voice is stopped here.
        return try await withTaskCancellationHandler { try await VoiceController.play(o, on: rt, gate: gate) } onCancel: { rt.audio.stopVoice() }
    }
    func voiceprint(_ audio: [Float], gate trial: TrialGate) async throws -> String {
        guard trial.live else { throw CancellationError() }
        let gate = SpeakerGate.shared
        guard gate.canListen else { return "The voice-print model is not loaded yet. Try again in a moment." }
        switch await gate.judge(audio) {
        case .match: return "That is the voice you taught it."
        case .mismatch: return "That does not sound like the voice you taught it."
        case .unknown: return "The voice-print model works, but it has not been taught your voice yet. Use Learn my voice."
        }
    }
}
