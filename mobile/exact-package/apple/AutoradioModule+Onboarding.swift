// Lane 8: first run. Voice setup (configuring the voice and audio models), the edition's topics (chips the reader taps, types or
// says) and the edition. Owned by the onboarding lane. The logic is in the kit (VoiceSetup.swift, EditionSetup.swift, with tests); this
// file only holds the listener's progress, the platform calls (microphone, route, speech) and the sources
// and sends the contract binds.
//
// What runs where:
//   a phone                      : setup uses the ModelStore downloader and (when the models lane supplies them) the
//                                  library fetch and on-device checkers; "Talk to it" runs on the engine: programme
//                                  "edition-setup", desk "edition-setup-desk", open addressing, until the reader stops it.
//                                  UNPROVEN on a device.
import AVFoundation
import Foundation

private struct OnboardingProgress: Codable {
    var offered = false
    var skipped = false
    var setupDone = false
    var micRefused = false
    var topicsStarted: Bool?
    var editionDone = false
    var step = "welcome"
}

final class OnboardingController: @unchecked Sendable {
    static var shared: OnboardingController?
    private static let synth = AVSpeechSynthesizer()

    unowned let module: AutoradioModule
    let support: URL
    private let lock = NSLock()
    private var progress = OnboardingProgress()
    private var setup: VoiceSetup?
    private var step = "welcome"
    /// Onboarding is over (the edition is made, or the reader skipped it): the station may start by itself (lane autoradio-queue).
    var finished: Bool { step == "done" || step == "skipped" }
    private var askMobile = false
    private var message = ""
    // microphone and hearing
    private var mic = "unasked"          // unasked | checking | heard | refused | silent
    private var level = 0.0
    private var heard = ""
    private var route = ""
    private var hearingOK = false
    // edition topics
    private var session: EditionSetupSession?
    private var talking = false
    /// The edit page is open: Done rewrites the edition without touching the first-run step.
    private var editing = false
    // edition
    private var edition: Edition?
    private var editionStatus = ""       // "" | writing | failed | ready
    private var writingLine = ""
    private var editionError = ""
    private var writer: EditionWriter

    var progressFile: URL { support.appendingPathComponent("onboarding.json") }
    var topicsFile: URL { support.appendingPathComponent("edition-topics.json") }
    var recordFile: URL { support.appendingPathComponent("voice-setup.json") }

    init(module: AutoradioModule, support: URL) {
        self.module = module; self.support = support
        writer = LocalEditionWriter()
        if let d = try? Data(contentsOf: progressFile), let p = try? JSONDecoder().decode(OnboardingProgress.self, from: d) { progress = p }
        // Coming back to the topics step shows what was saved. The microphone never opens by itself (restart rule).
        if progress.topicsStarted == true, !progress.editionDone, !progress.skipped {
            let s = EditionSetupSession(suggestions: EditionTopics.suggestions(), file: topicsFile, liveSections: [])
            session = s; EditionSetupSession.shared = s
            step = "topics"
        } else if progress.editionDone { step = "done" }
        else if progress.skipped { step = "skipped" }
        else if progress.setupDone { step = "ready" }
        // The saved picks are always there for the edit page, whether or not the first-run step was ever shown.
        if session == nil {
            let s = EditionSetupSession(suggestions: EditionTopics.suggestions(), file: topicsFile, liveSections: [])
            session = s; EditionSetupSession.shared = s
        }
        wire(session)
    }

    private func wire(_ s: EditionSetupSession?) {
        s?.onChange = { [weak self] in self?.changed() }
        s?.onFinish = { [weak self] in self?.finishTopics() }
    }

    func changed() { module.touch(["onboardingState"]) }

    private func save() {
        lock.lock(); let p = progress; lock.unlock()
        try? FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(p) { try? d.write(to: progressFile, options: .atomic) }
    }

    private func set(_ f: () -> Void) { lock.lock(); f(); lock.unlock(); changed() }

    /// Section ids from today's paper; merged into the suggestions so a section the house list adds shows up as a chip.
    private func sections() -> [String] {
        var seen: [String] = []
        for r in module.choir.storyRows() where !r.section.isEmpty && !seen.contains(r.section) { seen.append(r.section) }
        return seen
    }

    // MARK: Voice setup

    func begin(mobileOK: Bool) {
        lock.lock()
        let doneBefore = progress.setupDone && setup == nil && step == "skipped"
        lock.unlock()
        // Skipped after a finished setup: nothing to download again; go on to the topics.
        if doneBefore { set { step = "ready"; progress.skipped = false }; save(); return }
        lock.lock()
        if step == "setup", setup?.isStarted == true, setup?.isFinished == false { lock.unlock(); return }
        lock.unlock()
        // The one shared store, with the whole catalog in it from the start (the first phone walk's "model ... not registered"
        // came from a store of its own that nothing had registered into).
        let backend: SetupBackend = LiveSetupBackend(store: .shared)
        let profile = DeviceProfile.current()
        let stall = 20.0, patience = 45.0
        // The configurator picks one model per role for THIS phone, the whole pipeline resident together, by measuring it.
        let configurator: ConfigBackend? = LiveConfigBackend()
        var s = VoiceSetup(backend: backend, profile: profile, smaller: false, stallSeconds: stall, patience: patience, configBackend: configurator)
        if !s.fitsOnDisk { s = VoiceSetup(backend: backend, profile: profile, smaller: true, stallSeconds: stall, patience: patience, configBackend: configurator) }
        if !profile.onWiFi && !mobileOK {
            set { setup = s; step = "setup"; askMobile = true; message = "" }
            return
        }
        s.onChange = { [weak self] in self?.changed() }
        set { setup = s; step = "setup"; askMobile = false; message = ""; progress.skipped = false }
        Task { [weak self, s] in
            await s.run()
            self?.setupFinished(s)
        }
    }

    private func setupFinished(_ s: VoiceSetup) {
        if let d = try? JSONEncoder().encode(s.record) {
            try? FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
            try? d.write(to: recordFile, options: .atomic)
        }
        // The listener's configuration: select what passed, where the kit has that stage registered.
        for (kind, id) in s.choices where module.runtime.registry.stages(kind).contains(where: { $0.id == id }) {
            module.runtime.setStage(kind, id: id)
        }
        // The responder model setup chose is the one the phone talks with (read again at every start).
        if let r = s.record.items.first(where: { $0.stage == SetupStage.responder.rawValue }), KnownModels.isPinned(r.chosen) { ResponderChoice.current = r.chosen }
        set { progress.setupDone = true; step = "mic" }
        save()
    }

    func retry(_ stage: String) { if let st = SetupStage(rawValue: stage) { setup?.retry(st) } }
    func fallback(_ stage: String) { if let st = SetupStage(rawValue: stage) { setup?.useFallback(st) } }

    // MARK: Microphone, in context, with one live check; then a hearing check

    func askMicrophone() {
        set { mic = "checking"; message = "" }
        AVAudioApplication.requestRecordPermission { [weak self] granted in
            guard let self else { return }
            if !granted {
                self.set { self.mic = "refused"; self.progress.micRefused = true; self.message = "Talking to the radio needs the microphone. You can allow it in Settings; you can always tap instead." }
                self.save(); return
            }
            self.set { self.progress.micRefused = false }
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
                if !HandsFreeSwitch.shared.isOn { rt.setHandsFree(true, persist: false) }
                defer { feeder.cancel(); rt.restoreHandsFree() }
                while meter.verdict == .listening {
                    self.lock.lock(); let here = self.step == "mic"; self.lock.unlock()
                    if !here { return }                                                  // the listener left the step: end the check (defer restores)
                    try? await Task.sleep(nanoseconds: 100_000_000)
                    meter.tick(0.1)
                    meter.transcript(rt.snapshot().transcript)
                    let l = meter.level
                    self.set { self.level = l }
                }
                if meter.verdict == .heard {
                    rt.cancelTalk()                                                      // a phrase spoken here must not become a turn to the desk
                    let t = rt.snapshot().transcript
                    self.set { self.heard = (t.isEmpty || t == before) ? "Your microphone is working." : t; self.mic = "heard" }
                } else {
                    self.set { self.mic = "silent"; self.message = "I could not hear anything. The paper keeps working, and you can tap your topics instead." }
                }
            }
        }
    }

    func microphoneNext() {
        let r = currentRoute()
        set { route = r; step = "hearing"; hearingOK = false; message = "" }
        if !module.virtualEngine {
            // In the voice that was just set up. The radio's engine is running after the microphone check; if it is not, the phone's
            // own voice says it (the check is about the speaker, and it must not stay silent).
            if module.runtime.snapshot().running, let v = VoiceController.shared {
                Task { await v.speak("Can you hear this?") }
            } else {
                DispatchQueue.main.async { Self.synth.speak(AVSpeechUtterance(string: "Can you hear this?")) }
            }
        }
    }

    /// The voice step is done (the voice cards keep the choice): on to the interview's door.
    func voiceNext() {
        set { if step == "voice" { step = "ready" }; message = "" }
        save()
    }

    private func currentRoute() -> String {
        return "Playing on " + Platform.outputName()
    }

    func hearing(ok: Bool) {
        if ok { set { hearingOK = true; step = "voice"; progress.setupDone = true; message = "" }; save() }
        else {
            microphoneNext()
            set { message = "Try the volume, or another speaker or headphones, then play it again." }
        }
    }

    // MARK: The edition's topics

    /// The step: chips the reader taps, a field to type their own, and "Talk to it". Nothing here needs the microphone or the network.
    func startTopics() {
        lock.lock(); let existing = session; lock.unlock()
        let live = sections()
        let s = existing ?? EditionSetupSession(suggestions: EditionTopics.suggestions(liveSections: live), file: topicsFile, liveSections: live)
        s.setLiveSections(live)
        wire(s)
        EditionSetupSession.shared = s
        set { session = s; step = "topics"; progress.topicsStarted = true; progress.skipped = false; message = "" }
        save()
    }

    func toggleTopic(_ label: String) { session?.toggle(label) }

    /// The disclosure control: open or close a category. Not a selection.
    func expandTopic(_ label: String) { session?.toggleExpanded(label) }

    func addTopic(_ text: String) {
        guard let s = session else { return }
        if s.addTyped(text).isEmpty, !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { set { message = "That one is already there." } }
        else { set { message = "" } }
    }

    /// Say it instead of tapping: the engine listens until the reader stops it or taps Continue.
    func toggleTalk() {
        lock.lock(); let on = talking; lock.unlock()
        if on { stopVoice(); return }
        if progress.micRefused { set { message = "Talking to the radio needs the microphone. You can allow it in Settings, or tap topics instead." }; return }
        guard let s = session else { return }
        EditionSetupSession.shared = s
        let rt = module.runtime!
        rt.setStage(.programme, id: EditionSetupProgramme.stageID)
        rt.setStage(.desk, id: EditionSetupDesk.stageID)
        rt.setAudioPolicy(.alwaysTalking)
        set { talking = true; message = "" }
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
        }
        changed()
    }

    /// Continue: the selection is already saved (every change writes it); the edition is written from it.
    func finishTopics() {
        guard session != nil else { return }
        lock.lock(); let on = step == "topics"; lock.unlock()
        guard on else { return }
        stopVoice()
        writeEdition()
    }

    // MARK: The edition

    /// The edit page (`onboarding:edit`) opened: the saved picks are there, preselected, whether or not onboarding ever ran.
    func prepareEdit() {
        let live = sections()
        lock.lock(); let s = session; lock.unlock()
        s?.setLiveSections(live)
        EditionSetupSession.shared = s
        wire(s)
        set { editing = true; message = "" }
    }

    /// Done on the edit page: the same completion path as onboarding's Continue (deadline, progress, Retry).
    func finishEdit() {
        stopVoice()
        set { editing = true }  // also when the page's load never ran
        writeEdition()
    }

    /// Leave the edit page without rewriting (after a failure): the picks stay saved, the edition stays as it was.
    func leaveEdit() {
        stopVoice()
        set { editing = false; editionStatus = (edition != nil || progress.editionDone) ? "ready" : ""; editionError = "" }
        module.pages.close("onboarding:edit")
        module.touch(["openPages"])
    }

    func writeEdition() {
        guard let s = session else { return }
        lock.lock(); let busy = editionStatus == "writing"; lock.unlock()
        guard !busy else { return }
        let topics = s.topics
        lock.lock(); let edit = editing; lock.unlock()
        set { if !edit { step = "writing" }; editionStatus = "writing"; writingLine = "Writing your edition…"; editionError = "" }
        let w = writer
        Task { [weak self, w] in
            guard let self else { return }
            // Always ends: the edition, a failure, or the deadline (EditionCompletion). The topics were saved before this ran.
            let outcome = await EditionCompletion.run(topics: topics, writer: w) { line in self.set { self.writingLine = line } }
            switch outcome {
            case .written(let e):
                self.editionReady(e)
                // The topics also become the first lines of the notebook; the local file stays as the cache. A failure here is quiet: the page retries.
                Task { @MainActor in await NotebookStore.shared.writeTopics(topics) }
            case .failed(let m): self.set { self.editionStatus = "failed"; self.editionError = m }
            case .timedOut: self.set { self.editionStatus = "failed"; self.editionError = EditionCompletion.timedOutLine }
            }
        }
    }

    /// "Continue anyway": the general edition, from nothing but this phone. The saved topics stay in the file.
    func continueAnyway() {
        editionReady(EditionCompletion.general())
    }

    private func editionReady(_ e: Edition) {
        lock.lock(); let edit = editing; lock.unlock()
        if edit {
            // Editing after the first run: the step stays as it was; the edit page closes and the reader is back where they came from.
            set { edition = e; editionStatus = "ready"; progress.editionDone = true; editing = false }
            save()
            module.pages.close("onboarding:edit")
            module.touch(["openPages"])
            return
        }
        set { edition = e; editionStatus = "ready"; step = "done"; progress.editionDone = true }
        save()
        // The edition is a local page (onboarding:edit), not a server document: nothing is opened here, the done screen is shown.
        module.touch(["openPages"])
    }

    /// The computer already holds the listener's edition: no card, no offer.
    func markEditionFound() {
        set { progress.editionDone = true; step = "done" }
        save()
    }

    func skip() {
        stopVoice()
        set { progress.skipped = true; step = "skipped" }
        save()
    }

    func reset() {
        stopVoice()
        set { progress = OnboardingProgress(); step = "welcome"; setup = nil; edition = nil; editionStatus = ""; mic = "unasked"; message = ""; editing = false }
        try? FileManager.default.removeItem(at: topicsFile)
        let fresh = EditionSetupSession(suggestions: EditionTopics.suggestions(), file: topicsFile, liveSections: [])
        wire(fresh); EditionSetupSession.shared = fresh
        set { session = fresh }
        save()
    }

    // MARK: What the contract sees

    func view() -> [String: Any] {
        lock.lock(); defer { lock.unlock() }
        let rows: [[String: Any]] = (setup?.rows ?? []).map { r in
            ["stage": r.stage, "name": r.name, "model": r.model, "sizeLabel": r.sizeLabel, "state": r.state, "progress": r.progress, "statusLine": r.statusLine,
             "isFallback": r.isFallback, "canRetry": r.canRetry, "canFallback": r.canFallback, "numbers": r.numbers]
        }
        let tree: [[String: Any]] = (session?.treeRows() ?? TopicTree.rows(expanded: [], selection: EditionSelection(), own: [])).map {
            ["key": $0.key, "label": $0.label, "kind": $0.kind, "selected": $0.selected, "partial": $0.partial,
             "expandable": $0.expandable, "expanded": $0.expanded, "note": $0.note]
        }
        let picked = session?.topics ?? []
        var card = "", cardBody = ""
        // the prompt is gone once the edition is made or the reader has skipped it ("Not now": a saved skip), and starting setup again brings it back
        if progress.editionDone || progress.skipped { card = "" }
        else if progress.topicsStarted == true { card = "Choose your topics"; cardBody = "Pick up where you left off. What you chose is saved." }
        else if setup?.isStarted == true && setup?.isFinished == false { card = "Finish voice setup"; cardBody = "Setting up the voice on this phone takes a few minutes." }
        else { card = "Make your edition"; cardBody = "Pick the topics you care about, then your paper is yours." }
        let titles: [String: String] = ["welcome": "Make it yours", "setup": "Set up your voice", "mic": "Can I hear you?", "hearing": "Can you hear me?", "voice": "Pick a voice", "ready": "Voice is ready",
                                        "topics": "Your edition", "writing": "Writing your edition", "done": "Your edition", "skipped": "Make it yours"]
        var d = [String: Any]()
        d["step"] = step; d["title"] = titles[step] ?? "Make it yours"; d["message"] = message
        d["totalLabel"] = setup?.totalLabel ?? SetupCatalog.totalLabel(bytes: SetupCatalog.totalBytes(SetupCatalog.plan(for: .current())), onWiFi: true)
        d["askMobile"] = askMobile; d["rows"] = rows; d["summary"] = setup?.summary ?? []
        d["setupFinished"] = setup?.isFinished ?? false; d["setupStarted"] = setup?.isStarted ?? false
        var device = ""
        if let p = setup?.profile { device = p.chip + ", " + String(format: "%.0f", p.memoryGB) + " GB" }
        d["device"] = device
        d["mic"] = mic; d["level"] = level; d["heard"] = heard; d["route"] = route; d["hearingOK"] = hearingOK
        d["tree"] = tree; d["pickedCount"] = picked.count; d["selectionLine"] = EditionSetupText.selectionLine(picked)
        d["explainer"] = EditionSetupText.explainer; d["voiceHint"] = EditionSetupText.voiceHint; d["talking"] = talking
        d["editionStatus"] = editionStatus; d["writingLine"] = writingLine; d["editionError"] = editionError
        d["editionTitle"] = edition?.title ?? "My edition"; d["editionIntro"] = edition?.intro ?? ""
        d["editionLines"] = edition?.lines ?? []; d["editionAddress"] = PageAddress.edition
        d["card"] = card; d["cardBody"] = cardBody; d["aim"] = progress.editionDone ? "" : "Make it yours"
        d["skipped"] = progress.skipped; d["editionDone"] = progress.editionDone; d["editing"] = editing
        return d
    }
}

/// `onboarding:*` pages: the first-run setup and the edition's edit page (`onboarding:edit`; the old `texture:my-edition` address
/// is rewritten to it by `PageAddress.canonical`, because no server document of that name exists).
struct OnboardingPageSource: PageSource {
    let controller: OnboardingController
    func handles(_ address: String) -> Bool {
        address.hasPrefix("onboarding:")
    }
    func load(_ address: String) async throws -> PageModel {
        if address == "onboarding:edit" { controller.prepareEdit(); return PageModel(address: address, kind: "onboarding", title: "Edit my edition", status: "ready") }
        return PageModel(address: address, kind: "onboarding", title: "Make it yours", status: "ready")
    }
}

extension AutoradioModule {
    func registerOnboarding() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let c = OnboardingController(module: self, support: support)
        OnboardingController.shared = c
        StageRegistry.registerEditionSetupStages(runtime.registry)
        pageSources.register(OnboardingPageSource(controller: c))

        handle("onboardingState") { args, reply in reply(c.view()) }
        // Sends answer the same shape the page reads.
        handle("onboardingBegin") { args, reply in c.begin(mobileOK: (args["mobile"] as? String) == "true"); reply(c.view()) }
        handle("onboardingRetry") { args, reply in c.retry(args["stage"] as? String ?? ""); reply(c.view()) }
        handle("onboardingFallback") { args, reply in c.fallback(args["stage"] as? String ?? ""); reply(c.view()) }
        handle("onboardingMic") { args, reply in c.askMicrophone(); reply(c.view()) }
        handle("onboardingMicNext") { args, reply in c.microphoneNext(); reply(c.view()) }
        handle("onboardingHearing") { args, reply in c.hearing(ok: (args["ok"] as? String) == "true"); reply(c.view()) }
        handle("onboardingTopics") { args, reply in c.startTopics(); reply(c.view()) }
        handle("onboardingTopicToggle") { args, reply in c.toggleTopic(args["label"] as? String ?? ""); reply(c.view()) }
        handle("onboardingTopicExpand") { args, reply in c.expandTopic(args["label"] as? String ?? ""); reply(c.view()) }
        handle("onboardingEditDone") { args, reply in c.finishEdit(); reply(c.view()) }
        handle("onboardingEditLeave") { args, reply in c.leaveEdit(); reply(c.view()) }
        handle("onboardingTopicAdd") { args, reply in c.addTopic(args["text"] as? String ?? ""); reply(c.view()) }
        handle("onboardingTalk") { args, reply in c.toggleTalk(); reply(c.view()) }
        handle("onboardingTopicsDone") { args, reply in c.finishTopics(); reply(c.view()) }
        handle("onboardingEditionRetry") { args, reply in c.writeEdition(); reply(c.view()) }
        handle("onboardingContinueAnyway") { args, reply in c.continueAnyway(); reply(c.view()) }
        handle("onboardingSkip") { args, reply in c.skip(); reply(c.view()) }
        handle("onboardingReset") { args, reply in c.reset(); reply(c.view()) }

        // Fresh install, signed in: offer the path once as the open page (closing it is a choice that sticks).
        // The computer is asked first (GET /api/texture/edition): a listener who already has an edition (a second phone, a reinstall) is
        // not offered the topics step. No answer (offline, signed out, an older server) offers it as before.
        Task { [weak c, pages] in
            if let ed = await EditionLookup.fetch(), ed.found { c?.markEditionFound() } else if let pages { c?.offerOnce(pages: pages) }
        }
    }
}

extension OnboardingController {
    func offerOnce(pages: OpenPages) {
        lock.lock(); let go = !progress.offered && !progress.editionDone && !progress.skipped; if go { progress.offered = true }; lock.unlock()
        guard go else { return }
        save()
        pages.open("onboarding:setup", title: "Make it yours")
    }
}
