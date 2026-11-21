// Settings, native half. The pages are shared code: the Computer page (AutoradioLab/computer.ts), Activity (activity.ts), About (settings.ts) and the themes (themes.ts).
// What only native code can say, in ONE source:
//   settingsFacts   the clock (a data source may not read it), the app's version (the bundle: only native code reads it), the radio engine's phase and its kept turns (the voice
//                   runtime holds them). Facts only: the rows, their words and the page are shared code.
// and one that waits for the cards to move:
//   learnedInfo, learnedReset   "What the paper has learned": the cards' Preferences (AutoradioModule+Cards.swift) are native until the launcher and the story shading move; this
//                               page reads them there. It goes with them.
import Foundation

extension AutoradioModule {
    func registerSettings() {
        let learned = PreferencesLearned(prefs: { CardsState.shared.prefs })
        func ack(_ ok: Bool, _ message: String = "") -> [String: Any] { ["ok": ok, "message": message] }

        handle("settingsFacts") { [unowned self] _, reply in      // (self: the runtime)
            let info = Bundle.main.infoDictionary ?? [:]
            var radio: [String: Any] = ["known": false, "running": false, "phase": ""]
            if let s = runtime?.snapshot() { radio = ["known": true, "running": s.running, "phase": s.phase] }
            let turns: [[String: Any]] = (runtime?.turns() ?? []).map { ["id": "\($0.id)", "outcome": $0.outcome, "transcript": $0.transcript, "configName": $0.configName] }
            reply(["now": Date().timeIntervalSince1970 * 1000, "appVersion": (info["CFBundleShortVersionString"] as? String) ?? "", "radio": radio, "turns": turns])
        }

        handle("learnedInfo") { [unowned self] _, reply in
            Task { [weak self] in
                let lines = await learned.lines()
                DispatchQueue.main.async { reply(self?.json(LearnedView(lines: lines)) ?? [:]) }
            }
        }
        handle("learnedReset") { [unowned self] _, reply in
            Task { [weak self] in await learned.reset(); DispatchQueue.main.async { self?.touch(["learnedInfo", "signalList"]); reply(ack(true)) } }
        }
    }
}
