// Lane 8: first run. Voice setup (configuring the voice and audio models), the interview on the existing turn engine, and
// the edition. Owned by the onboarding lane. The logic is in the kit (VoiceSetup.swift, Interview.swift, with tests); this
// file only holds the listener's progress, the demo drive, the platform calls (microphone, route, speech) and the sources
// and sends the contract binds.
//
// What runs where:
//   demo mode / simulator driver : setup is a scripted download and check (fixtures/demo/setup-script.json); the interview
//                                  is driven turn by turn from fixtures/demo/interview-script.json through the same
//                                  `InterviewSession.hear` the desk stage calls; the edition is fixtures/demo/edition.json.
//                                  No microphone, no model, no network.
//   a phone                      : setup uses the ModelStore downloader and (when the models lane supplies them) the
//                                  library fetch and on-device checkers; the interview runs on the engine: programme
//                                  "interview", desk "interview-desk", open addressing. UNPROVEN on a device.
import AVFoundation
import Foundation
import UIKit

private struct OnboardingProgress: Codable {
    var offered = false
    var skipped = false
    var setupDone = false
    var micRefused = false
    var interviewStarted = false
    var editionDone = false
    var step = "welcome"
}

final class OnboardingController: @unchecked Sendable {
    static var shared: OnboardingController?
    private static let synth = AVSpeechSynthesizer()

    unowned let module: AutoradioModule
    let support: URL
    let demo: Bool
    private let lock = NSLock()
    private var progress = OnboardingProgress()
    private var setup: VoiceSetup?
    private var step = "welcome"
    private var askMobile = false
    private var message = ""
    // microphone and hearing
    private var mic = "unasked"          // unasked | checking | heard | refused | silent
    private var level = 0.0
    private var heard = ""
    private var route = ""
    private var hearingOK = false
    // interview
    private var session: InterviewSession?
    private var demoScript: [[String: String]] = []
    private var demoIndex = 0
    private var demoSections: [String] = []
    // edition
    private var edition: Edition?
    private var editionStatus = ""       // "" | writing | failed | ready
    private var writingLine = ""
    private var editionError = ""
    private var writer: EditionWriter

    var progressFile: URL { support.appendingPathComponent("onboarding.json") }
    var interviewFile: URL { support.appendingPathComponent("interview.json") }
    var recordFile: URL { support.appendingPathComponent("voice-setup.json") }

    init(module: AutoradioModule, support: URL, demo: Bool) {
        self.module = module; self.support = support; self.demo = demo
        if demo, let d = Demo.fixture("edition"), let e = Edition.parse(d) { writer = DemoEditionWriter(edition: e) } else { writer = UnbuiltEditionWriter() }
        if let d = try? Data(contentsOf: progressFile), let p = try? JSONDecoder().decode(OnboardingProgress.self, from: d) { progress = p }
        // Nothing resumes by itself (restart rule): a saved interview comes back paused, behind a card.
        if let saved = InterviewSession.load(file: interviewFile, sections: []), !saved.state.turns.isEmpty, !progress.editionDone {
            let s = InterviewSession(interview: saved, file: interviewFile)
            session = s; InterviewSession.shared = s
            step = "interview"
        } else if progress.editionDone { step = "done" }
        else if progress.skipped { step = "skipped" }
        else if progress.setupDone { step = "ready" }
        if demo, let d = Demo.fixture("interview-script"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
            demoSections = o["sections"] as? [String] ?? []
            demoScript = (o["steps"] as? [[String: Any]] ?? []).map { st in st.reduce(into: [String: String]()) { $0[$1.key] = $1.value as? String ?? "" } }
        }
        wire(session)
    }

    private func wire(_ s: InterviewSession?) {
        s?.onChange = { [weak self] in self?.changed() }
    }

    func changed() { module.touch(["onboardingState"]) }

    private func save() {
        lock.lock(); let p = progress; lock.unlock()
        try? FileManager.default.createDirectory(at: support, withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(p) { try? d.write(to: progressFile, options: .atomic) }
    }

    private func set(_ f: () -> Void) { lock.lock(); f(); lock.unlock(); changed() }

    /// Sections from today's paper, offered when the first answer is vague.
    private func sections() -> [String] {
        if demo, !demoSections.isEmpty { return demoSections }
        var seen: [String] = []
        for r in module.choir.storyRows() where !r.section.isEmpty && !seen.contains(r.section) { seen.append(r.section) }
        return seen
    }

    // MARK: Voice setup

    func begin(mobileOK: Bool) {
        lock.lock()
        if step == "setup", setup?.isStarted == true, setup?.isFinished == false { lock.unlock(); return }
        lock.unlock()
        let backend: SetupBackend
        var profile = DeviceProfile.current()
        var stall = 20.0, patience = 45.0
        if demo, let d = Demo.fixture("setup-script"), let sc = SimulatedSetupBackend.parse(d) {
            backend = SimulatedSetupBackend(script: sc); profile = sc.profile; stall = sc.stallSeconds ?? 1.6; patience = 3600
        } else {
            backend = LiveSetupBackend(store: ModelStore(root: support.appendingPathComponent("models")))
        }
        var s = VoiceSetup(backend: backend, profile: profile, smaller: false, stallSeconds: stall, patience: patience)
        if !s.fitsOnDisk { s = VoiceSetup(backend: backend, profile: profile, smaller: true, stallSeconds: stall, patience: patience) }
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
        if !demo {
            // The listener's configuration: select what passed, where the kit has that stage registered.
            for (kind, id) in s.choices where module.runtime.registry.stages(kind).contains(where: { $0.id == id }) {
                module.runtime.setStage(kind, id: id)
            }
        }
        set { progress.setupDone = true; step = "mic" }
        save()
    }

    func retry(_ stage: String) { if let st = SetupStage(rawValue: stage) { setup?.retry(st) } }
    func fallback(_ stage: String) { if let st = SetupStage(rawValue: stage) { setup?.useFallback(st) } }

    // MARK: Microphone, in context, with one live check; then a hearing check

    func askMicrophone() {
        if demo {
            set { mic = "checking"; level = 0.15; heard = "" ; message = "" }
            Task { [weak self] in
                for l in [0.35, 0.7, 0.5, 0.8] {
                    try? await Task.sleep(nanoseconds: 220_000_000)
                    guard let self else { return }
                    self.set { self.level = l }
                }
                guard let self else { return }
                self.set { self.heard = "Tell me about the harbor."; self.mic = "heard"; self.level = 0.3 }
            }
            return
        }
        set { mic = "checking"; message = "" }
        AVAudioApplication.requestRecordPermission { [weak self] granted in
            guard let self else { return }
            if !granted {
                self.set { self.mic = "refused"; self.progress.micRefused = true; self.message = "The interview is a spoken conversation, so it needs the microphone. You can allow it in Settings." }
                self.save(); return
            }
            self.set { self.progress.micRefused = false }
            // One live check: start the engine and wait for words (the engine's transcript is the evidence). UNPROVEN on a device.
            Task { [weak self] in
                guard let self else { return }
                await self.module.runtime.start(virtual: self.module.virtualEngine)
                for _ in 0..<40 {
                    try? await Task.sleep(nanoseconds: 500_000_000)
                    let t = self.module.runtime.snapshot().transcript
                    if !t.isEmpty { self.set { self.heard = t; self.mic = "heard" }; return }
                }
                self.set { self.mic = "silent"; self.message = "I could not hear anything. The paper keeps working; the interview waits until I can hear you." }
            }
        }
    }

    func microphoneNext() {
        let r = demo ? "Playing on the speaker (demo)" : currentRoute()
        set { route = r; step = "hearing"; hearingOK = false; message = "" }
        if !demo && !module.virtualEngine {
            DispatchQueue.main.async { Self.synth.speak(AVSpeechUtterance(string: "Can you hear this?")) }
        }
    }

    private func currentRoute() -> String {
        let outs = AVAudioSession.sharedInstance().currentRoute.outputs
        return "Playing on " + (outs.first?.portName ?? "this phone")
    }

    func hearing(ok: Bool) {
        if ok { set { hearingOK = true; step = "ready"; progress.setupDone = true; message = "" }; save() }
        else { set { message = "Try the volume, or another speaker or headphones, then play it again." } ; if !demo { microphoneNext() } }
    }

    // MARK: The interview

    func startInterview() {
        if !demo, mic != "heard" && mic != "checking" && progress.micRefused {
            set { message = "The interview is a spoken conversation, so it needs the microphone." }; return
        }
        let bankSections = sections()
        let s = InterviewSession(interview: Interview(bank: .default, sections: bankSections.isEmpty ? ["World", "Local", "Culture", "Tech and AI"] : bankSections), file: interviewFile)
        wire(s)
        InterviewSession.shared = s
        set { session = s; step = "interview"; progress.interviewStarted = true; progress.skipped = false; demoIndex = 0; message = "" }
        save()
        _ = s.start()
        if !demo { startOnEngine() }
    }

    /// The interview on the radio's own engine: programme, desk and open addressing come from the kit's registry.
    /// UNPROVEN on a device (the simulator proof drives the session directly).
    private func startOnEngine() {
        let rt = module.runtime!
        rt.setStage(.programme, id: InterviewProgramme.stageID)
        rt.setStage(.desk, id: InterviewDesk.stageID)
        if rt.registry.stages(.addressing).contains(where: { $0.id == "open" }) { rt.setStage(.addressing, id: "open") }
        rt.setAudioPolicy(.alwaysTalking)
        Task { [weak self] in
            guard let self else { return }
            await rt.start(virtual: self.module.virtualEngine)
            await self.module.choir.loadProgramme()
        }
    }

    func continueInterview() {
        guard let s = session else { startInterview(); return }
        InterviewSession.shared = s
        set { step = "interview"; message = "" }
        _ = s.resume()
        if !demo { startOnEngine() }
    }

    /// Demo: the next recorded turn, through the same `hear` the desk stage calls.
    func demoNext() {
        guard demo, let s = session else { return }
        lock.lock(); let i = demoIndex; let script = demoScript; lock.unlock()
        guard i < script.count else { return }
        set { demoIndex = i + 1 }
        let st = script[i]
        Task { [weak self] in
            guard let self else { return }
            switch st["kind"] ?? "say" {
            case "interrupt": _ = await s.hear(st["words"] ?? "", interrupted: true)
            case "loud": _ = s.tooLoud()
            case "resume": _ = s.resume()
            case "silence": _ = s.silence()
            default: _ = await s.hear(st["words"] ?? "", interrupted: false)
            }
            if s.snapshot.status == .done { self.writeEdition() }
            self.changed()
        }
    }

    // MARK: The edition

    func writeEdition() {
        guard let s = session else { return }
        let iv = s.snapshot
        let title = "Interview, " + Self.dayLabel()
        set { step = "writing"; editionStatus = "writing"; writingLine = "Reading what you said"; editionError = "" }
        try? iv.documentMarkdown(title: title).write(to: support.appendingPathComponent("interview.md"), atomically: true, encoding: .utf8)
        let w = writer
        Task { [weak self, w] in
            guard let self else { return }
            do {
                let e = try await w.write(from: iv, interviewTitle: title) { line in self.set { self.writingLine = line } }
                self.set { self.edition = e; self.editionStatus = "ready"; self.step = "done"; self.progress.editionDone = true }
                self.save()
                self.module.pages.open("texture:my-edition", title: e.title)
                self.module.pages.focus("onboarding:setup")
                self.module.touch(["openPages"])
            } catch {
                // The interview document is kept; the listener can retry. The default edition goes on meanwhile.
                self.set { self.editionStatus = "failed"; self.editionError = "I couldn't write your edition just now. I'll keep what you said." }
            }
        }
    }

    static func dayLabel() -> String {
        let f = DateFormatter(); f.dateFormat = "d MMM"; f.locale = Locale(identifier: "en_US_POSIX"); return f.string(from: Date())
    }

    func skip() {
        set { progress.skipped = true; step = "skipped" }
        save()
    }

    func reset() {
        set { progress = OnboardingProgress(); step = "welcome"; setup = nil; session = nil; InterviewSession.shared = nil; edition = nil; editionStatus = ""; mic = "unasked"; message = ""; demoIndex = 0 }
        try? FileManager.default.removeItem(at: interviewFile)
        save()
    }

    func editionLines() -> [String]? { lock.lock(); defer { lock.unlock() }; return edition?.lines }
    func editionTitle() -> String { lock.lock(); defer { lock.unlock() }; return edition?.title ?? "My edition" }

    // MARK: What the contract sees

    func view() -> [String: Any] {
        lock.lock(); defer { lock.unlock() }
        let rows: [[String: Any]] = (setup?.rows ?? []).map { r in
            ["stage": r.stage, "name": r.name, "model": r.model, "sizeLabel": r.sizeLabel, "state": r.state, "progress": r.progress, "statusLine": r.statusLine,
             "isFallback": r.isFallback, "canRetry": r.canRetry, "canFallback": r.canFallback, "numbers": r.numbers]
        }
        let iv = session?.snapshot
        let turns = iv?.state.turns ?? []
        let talk: [[String: Any]] = turns.suffix(7).map { ["who": $0.who, "text": $0.text, "interrupted": $0.interrupted] }
        var interviewStatus = iv?.status.rawValue ?? "notStarted"
        if iv?.status == .readback { interviewStatus = "readback" }
        let ivPaused = iv?.status == .paused
        let lastRadio = turns.last(where: { $0.who == "radio" })?.text ?? ""
        var card = "", cardBody = ""
        if progress.editionDone { card = "" }
        else if progress.micRefused { card = "Make your edition"; cardBody = "The interview is a spoken conversation, so it needs the microphone." }
        else if iv != nil, !turns.isEmpty { card = "Continue your interview"; cardBody = "Pick up where you left off. Nothing starts by itself." }
        else if progress.interviewStarted || (setup?.isStarted == true && setup?.isFinished == false) { card = "Finish voice setup"; cardBody = "Setting up the voice on this phone takes a few minutes." }
        else { card = "Make your edition"; cardBody = "A short spoken conversation, then your paper is yours." }
        let nextNote = demo && demoIndex < demoScript.count ? (demoScript[demoIndex]["note"] ?? "") : ""
        let nextKind = demo && demoIndex < demoScript.count ? (demoScript[demoIndex]["kind"] ?? "") : ""
        let nextWords = demo && demoIndex < demoScript.count ? (demoScript[demoIndex]["words"] ?? "") : ""
        let titles: [String: String] = ["welcome": "Make it yours", "setup": "Set up your voice", "mic": "Can I hear you?", "hearing": "Can you hear me?", "ready": "Ready to talk",
                                        "interview": "Your interview", "writing": "Writing your edition", "done": "Your edition", "skipped": "Make it yours"]
        var d = [String: Any]()
        d["step"] = step; d["title"] = titles[step] ?? "Make it yours"; d["message"] = message
        d["totalLabel"] = setup?.totalLabel ?? SetupCatalog.totalLabel(bytes: SetupCatalog.totalBytes(SetupCatalog.plan(for: .current())), onWiFi: true)
        d["askMobile"] = askMobile; d["rows"] = rows; d["summary"] = setup?.summary ?? []
        d["setupFinished"] = setup?.isFinished ?? false; d["setupStarted"] = setup?.isStarted ?? false
        var device = ""
        if let p = setup?.profile { device = p.chip + ", " + String(format: "%.0f", p.memoryGB) + " GB" }
        d["device"] = device
        d["mic"] = mic; d["level"] = level; d["heard"] = heard; d["route"] = route; d["hearingOK"] = hearingOK
        d["interviewStatus"] = interviewStatus; d["paused"] = ivPaused; d["pauseReason"] = iv?.state.pauseReason ?? ""
        d["talk"] = talk; d["nowSaying"] = lastRadio
        d["listening"] = (iv?.status == .running || iv?.status == .readback)
        d["demo"] = demo; d["demoNote"] = nextNote; d["demoKind"] = nextKind; d["demoWords"] = nextWords; d["demoLeft"] = max(0, demoScript.count - demoIndex)
        d["editionStatus"] = editionStatus; d["writingLine"] = writingLine; d["editionError"] = editionError
        d["editionTitle"] = edition?.title ?? "My edition"; d["editionIntro"] = edition?.intro ?? ""
        d["editionLines"] = edition?.lines ?? []; d["editionAddress"] = "texture:my-edition"
        d["card"] = card; d["cardBody"] = cardBody; d["aim"] = progress.editionDone ? "" : "Make it yours"
        d["skipped"] = progress.skipped; d["editionDone"] = progress.editionDone
        return d
    }
}

/// `onboarding:*` pages and the edition Texture the interview wrote (`texture:my-edition`, until the server has an edition marker).
struct OnboardingPageSource: PageSource {
    let controller: OnboardingController
    func handles(_ address: String) -> Bool {
        address.hasPrefix("onboarding:") || (address == "texture:my-edition" && controller.editionLines() != nil)
    }
    func load(_ address: String) async throws -> PageModel {
        if address == "texture:my-edition" {
            return PageModel(address: address, kind: "reader", title: controller.editionTitle(), status: "ready", lines: controller.editionLines() ?? [])
        }
        return PageModel(address: address, kind: "onboarding", title: "Make it yours", status: "ready")
    }
}

extension AutoradioModule {
    func registerOnboarding() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let c = OnboardingController(module: self, support: support, demo: Demo.isOn)
        OnboardingController.shared = c
        StageRegistry.registerInterviewStages(runtime.registry)
        pageSources.register(OnboardingPageSource(controller: c))

        handle("onboardingState") { args, reply in reply(c.view()) }
        // Sends answer the same shape the page reads.
        handle("onboardingBegin") { args, reply in c.begin(mobileOK: (args["mobile"] as? String) == "true"); reply(c.view()) }
        handle("onboardingRetry") { args, reply in c.retry(args["stage"] as? String ?? ""); reply(c.view()) }
        handle("onboardingFallback") { args, reply in c.fallback(args["stage"] as? String ?? ""); reply(c.view()) }
        handle("onboardingMic") { args, reply in c.askMicrophone(); reply(c.view()) }
        handle("onboardingMicNext") { args, reply in c.microphoneNext(); reply(c.view()) }
        handle("onboardingHearing") { args, reply in c.hearing(ok: (args["ok"] as? String) == "true"); reply(c.view()) }
        handle("onboardingInterview") { args, reply in c.startInterview(); reply(c.view()) }
        handle("onboardingContinue") { args, reply in c.continueInterview(); reply(c.view()) }
        handle("onboardingDemoNext") { args, reply in c.demoNext(); reply(c.view()) }
        handle("onboardingEditionRetry") { args, reply in c.writeEdition(); reply(c.view()) }
        handle("onboardingSkip") { args, reply in c.skip(); reply(c.view()) }
        handle("onboardingReset") { args, reply in c.reset(); reply(c.view()) }

        // Fresh install, signed in, not demo: offer the path once as the open page (closing it is a choice that sticks).
        if !Demo.isOn { c.offerOnce(pages: pages) }
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
