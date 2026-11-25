// Settings, native half. The pages are shared code: the Computer page (AutoradioLab/computer.ts), Activity (activity.ts), About (settings.ts) and the themes (themes.ts).
// What only native code can say, in ONE source:
//   settingsFacts   the radio engine's phase and its kept turns (the voice runtime holds them). Facts only: the rows, their words and the page are shared code; the clock is
//                   Exact's (`exactTime`, members-settings.contract) and the app's version is the build's (build-info.gen.ts).
import Foundation

extension AutoradioModule {
    func registerSettings() {

        handle("settingsFacts") { [unowned self] _, reply in      // (self: the runtime)
            var radio: [String: Any] = ["known": false, "running": false, "phase": ""]
            if let s = runtime?.snapshot() { radio = ["known": true, "running": s.running, "phase": s.phase] }
            let turns: [[String: Any]] = (runtime?.turns() ?? []).map { ["id": "\($0.id)", "outcome": $0.outcome, "transcript": $0.transcript, "configName": $0.configName] }
            reply(["radio": radio, "turns": turns])
        }

    }
}
