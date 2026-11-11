// Lane file: settings, themes, accounts and Activity (lane 7). Sources and sends are declared in interface/settings.json.
//   themeCards / themeExtras      the themes as preview cards, and the type scale and mark tokens of the one in force
//   settingsAccount               who am I (GET /auth/whoami; demo: fixture)
//   settingsComputer              status, Wake, Restart to update
//   settingRows(group)            the talk mode and the gesture meanings (a catalog in the kit, SettingsCatalog)
//   learnedInfo                   "What the paper has learned" (derived from the gesture events; lane 4 may supply its own)
//   activityInfo                  Activity: now, the computer, the desk, the paper, the radio's turns
//   aboutInfo                     the app and server builds
// The spine's themeSet and schemeSet are wrapped (not replaced) so the cards and the scale repaint with them.
import Foundation
import UIKit

/// The lane's state, held here because an extension cannot add stored properties to the module.
final class SettingsLane: @unchecked Sendable {
    let account: AccountModel
    let computer: ComputerModel
    let activity: ActivityModel
    let themeSource: ThemeSource
    let learned: LearnedProvider
    let settings = AppSettings.shared
    private let lock = NSLock()
    private var _themeStatus = ""
    private var _server = ""
    var themeStatus: String { get { lock.lock(); defer { lock.unlock() }; return _themeStatus } set { lock.lock(); _themeStatus = newValue; lock.unlock() } }
    var server: String { get { lock.lock(); defer { lock.unlock() }; return _server } set { lock.lock(); _server = newValue; lock.unlock() } }

    init(account: AccountModel, computer: ComputerModel, activity: ActivityModel, themeSource: ThemeSource, learned: LearnedProvider) {
        self.account = account; self.computer = computer; self.activity = activity; self.themeSource = themeSource; self.learned = learned
    }
}

extension AutoradioModule {
    func registerSettings() {
        let demo = Demo.isOn
        let account = AccountModel(client: demo ? DemoAccountClient() : ChoirAccountClient(), demo: demo,
                                   signedIn: { [choir] in choir?.authView().signedIn ?? false }, baseURL: { [choir] in choir?.signIn.baseURL ?? "" })
        let computer = ComputerModel(client: demo ? DemoComputerClient() : ChoirComputerClient())
        let activity = ActivityModel(client: demo ? DemoActivityClient() : ChoirActivityClient())
        let lane = SettingsLane(account: account, computer: computer, activity: activity,
                                themeSource: demo ? DemoThemeSource() : ChoirThemeSource(),
                                learned: SignalsLearned(signals: signals, demoSeed: demo))
        account.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["settingsAccount"]) } }
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

        // Reduce Motion is the system's: the theme's reduced values apply while it is on.
        themes.reduceMotion = {
            let read: () -> Bool = { UIAccessibility.isReduceMotionEnabled }
            return Thread.isMainThread ? read() : DispatchQueue.main.sync(execute: read)
        }
        NotificationCenter.default.addObserver(forName: UIAccessibility.reduceMotionStatusDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            self?.touch(["theme", "themeExtras", "themeCards"])
        }

        // Signing in or out changes who the account is.
        let previous = choir.onChange
        choir.onChange = { [weak self] in
            previous?()
            DispatchQueue.main.async {
                guard let self else { return }
                if self.choir.authView().signedIn { Task { await account.refresh() } } else { account.clear() }
                self.touch(["settingsAccount"])
            }
        }

        // The talk mode applies at launch and when set.
        lane.settings.apply(to: runtime)

        // ---- themes
        func cards(_ status: String = lane.themeStatus) -> [String: Any] {
            ["rows": themes.cards().map { json($0) }, "mode": themes.schemeMode, "scheme": themes.effectiveScheme(),
             "themeID": themes.themeID, "problems": themes.problems, "status": status]
        }
        handle("themeCards") { _, reply in reply(cards()) }
        handle("themeExtras") { [unowned self] _, reply in reply(json(themes.extras())) }
        for name in ["themeSet", "schemeSet"] {
            if let original = handlers[name] {
                handle(name) { [weak self] args, reply in original(args, reply); self?.touch(["themeCards", "themeExtras"]) }
            }
        }
        func ack(_ ok: Bool, _ message: String = "") -> [String: Any] { ["ok": ok, "message": message] }
        handle("themeFromTexture") { [unowned self] args, reply in
            let doc = (args["doc"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
            guard !doc.isEmpty else { lane.themeStatus = "Enter the id of a theme Texture."; touch(["themeCards"]); return reply(ack(false, lane.themeStatus)) }
            Task { [weak self] in
                guard let self else { return }
                var okay = false
                do {
                    let t = try await lane.themeSource.fetch(doc: doc)
                    let id = try themes.installFromTexture(t.themeJSON)
                    let name = themes.list().first { $0.id == id }?.name ?? id
                    lane.themeStatus = "Added \(name) from the Texture" + (t.ignored.isEmpty ? "." : " (the phone ignored: \(t.ignored.joined(separator: ", "))).")
                    okay = true
                } catch ChoirError.server(let status, let message) {
                    lane.themeStatus = status == 404 ? "No theme Texture with that id." : status == 422 ? "That Texture is not a usable theme: \(message.prefix(120))" : "The server refused (\(status))."
                } catch ChoirError.unauthorized {
                    lane.themeStatus = "This phone's sign-in is no longer accepted."
                } catch { lane.themeStatus = "\(error)".prefix(160).description }
                DispatchQueue.main.async { self.touch(["themeCards", "themeList"]); reply(ack(okay, lane.themeStatus)) }
            }
        }
        handle("themeRemove") { [unowned self] args, reply in
            themes.remove(args["id"] as? String ?? ""); lane.themeStatus = ""; touch(["themeCards", "themeList", "theme", "themeExtras"]); reply(ack(true))
        }

        // ---- account
        handle("settingsAccount") { [unowned self] _, reply in
            let v = account.view()
            if v.signedIn, v.status == "idle" { Task { await account.refresh() } }
            reply(json(v))
        }
        handle("accountRefresh") { _, reply in Task { await account.refresh() }; reply(ack(true)) }

        // ---- computer
        handle("settingsComputer") { [unowned self] _, reply in
            let v = computer.view()
            if !v.checked, choir.authView().signedIn { Task { await computer.refresh() } }
            reply(json(v))
        }
        handle("computerRefresh") { _, reply in Task { await computer.refresh() }; reply(ack(true)) }
        handle("computerWake") { _, reply in Task { await computer.wake() }; reply(ack(true)) }
        handle("computerUpdate") { _, reply in Task { await computer.restartToUpdate() }; reply(ack(true)) }

        // ---- talk mode and gesture meanings
        handle("settingRows") { [unowned self] args, reply in
            reply(["rows": lane.settings.rows(group: args["group"] as? String ?? "").map { json($0) }])
        }
        handle("settingSet") { [unowned self] args, reply in
            let id = args["id"] as? String ?? "", value = args["value"] as? String ?? ""
            let ok = lane.settings.set(id, value)
            if ok, id == "talkMode" { lane.settings.apply(to: runtime) }
            touch(["settingRows"]); reply(ack(ok, ok ? "" : "That is not one of the choices."))
        }

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
            if lane.server.isEmpty, !demo { Task { [weak self] in await self?.readServerBuild(lane) } }
            reply(["app": (info["CFBundleShortVersionString"] as? String) ?? "dev", "build": (info["CFBundleVersion"] as? String) ?? "",
                   "server": demo ? "demo fixtures" : lane.server, "base": choir.signIn.baseURL, "demo": demo, "signedIn": choir.authView().signedIn])
        }
    }

    /// GET /health (no key): the build the server reports. Plain text only; failures leave it empty.
    func readServerBuild(_ lane: SettingsLane) async {
        lane.server = "reading…"
        do {
            let r = try await ChoirEnvironment.shared.client().send("GET", "/health", authenticated: false, route: "/health")
            let o = (try? r.object()) ?? [:]
            let v = ["commit", "build", "git_sha", "sha", "version"].compactMap { o[$0] as? String }.first ?? ""
            lane.server = String(v.prefix(40))
        } catch { lane.server = "" }
        touch(["aboutInfo"])
    }
}
