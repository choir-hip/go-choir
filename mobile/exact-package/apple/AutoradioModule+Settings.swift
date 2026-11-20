// Lane file: settings, accounts and Activity (lane 7). Sources and sends are declared in interface/settings.json.
//   (the themes and settingsAccount are answered by app.ts: themes.ts, settings.ts)
//   settingsComputer              status, Wake, Restart to update
//   learnedInfo                   "What the paper has learned" (derived from the gesture events; lane 4 may supply its own)
//   activityInfo                  Activity: now, the computer, the desk, the paper, the radio's turns
//   aboutInfo                     the app version and build number (Bundle: only native reads it); the commits are about.ts
import Foundation

/// The lane's state, held here because an extension cannot add stored properties to the module.
final class SettingsLane: @unchecked Sendable {
    let computer: ComputerModel
    let activity: ActivityModel
    let learned: LearnedProvider
    private let lock = NSLock()

    init(computer: ComputerModel, activity: ActivityModel, learned: LearnedProvider) {
        self.computer = computer; self.activity = activity; self.learned = learned
    }
}

extension AutoradioModule {
    func registerSettings() {
        let computer = ComputerModel(client: ChoirComputerClient())
        let activity = ActivityModel(client: ChoirActivityClient())
        let lane = SettingsLane(computer: computer, activity: activity,
                                learned: PreferencesLearned(prefs: { CardsState.shared.prefs }))
        computer.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["settingsComputer", "activityInfo"]) } }
        activity.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["activityInfo"]) } }
        activity.signedIn = { [choir] in choir?.authView().signedIn ?? false }
        activity.radioTurns = { [runtime] in
            (runtime?.turns() ?? []).map { t in
                var it = ActivityItem()
                it.id = "turn-\(t.id)"; it.kind = "turn"; it.group = "radio"; it.label = t.outcome.isEmpty ? "A turn" : t.outcome.capitalized
                it.detail = t.transcript.isEmpty ? "(no words heard)" : "\u{201C}\(t.transcript)\u{201D}"
                it.title = t.configName
                return it
            }
        }
        activity.nowLines = { [runtime, computer] in
            var out: [ActivityItem] = []
            if let s = runtime?.snapshot() {
                var it = ActivityItem(); it.id = "now-radio"; it.kind = "radio"; it.group = "now"; it.label = "The radio"
                it.detail = !s.running ? "The radio is stopped." : (s.phase == "playing" ? "The radio is playing." : "The radio is in a turn (\(s.phase)).")
                out.append(it)
            }
            let c = computer.view()
            if c.checked || !c.state.isEmpty {
                var it = ActivityItem(); it.id = "now-computer"; it.kind = "computer"; it.group = "now"; it.label = "Your computer"
                it.detail = (c.state.isEmpty ? "Status is not known yet." : "Your computer is \(c.state).") + (c.updateAvailable ? " An update is waiting." : "")
                out.append(it)
            }
            return out
        }

        func ack(_ ok: Bool, _ message: String = "") -> [String: Any] { ["ok": ok, "message": message] }

        // ---- computer
        handle("settingsComputer") { [unowned self] _, reply in
            let v = computer.view()
            if !v.checked, choir.authView().signedIn { Task { await computer.refresh() } }
            reply(json(v))
        }
        handle("computerRefresh") { _, reply in Task { await computer.refresh() }; reply(ack(true)) }
        handle("computerWake") { _, reply in Task { await computer.wake() }; reply(ack(true)) }
        handle("computerUpdate") { _, reply in Task { await computer.restartToUpdate() }; reply(ack(true)) }

        // ---- what the paper has learned
        handle("learnedInfo") { [unowned self] _, reply in
            Task { [weak self] in
                let lines = await lane.learned.lines()
                DispatchQueue.main.async { reply(self?.json(LearnedView(lines: lines)) ?? [:]) }
            }
        }
        handle("learnedReset") { [unowned self] _, reply in
            Task { [weak self] in await lane.learned.reset(); DispatchQueue.main.async { self?.touch(["learnedInfo", "signalList"]); reply(ack(true)) } }
        }

        // ---- activity
        handle("activityInfo") { [unowned self] _, reply in
            if choir.authView().signedIn, activity.view().status == "idle" { Task { await activity.refresh() } }
            reply(json(activity.view()))
        }
        handle("activityRefresh") { _, reply in Task { await activity.refresh() }; Task { await computer.refresh() }; reply(ack(true)) }

        // ---- about
        handle("aboutInfo") { [unowned self] _, reply in
            let info = Bundle.main.infoDictionary ?? [:]
            reply(["app": (info["CFBundleShortVersionString"] as? String) ?? "dev", "build": (info["CFBundleVersion"] as? String) ?? "",
                   "base": choir.signIn.baseURL, "signedIn": choir.authView().signedIn])
        }
    }
}
