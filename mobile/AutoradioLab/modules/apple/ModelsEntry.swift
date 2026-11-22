// The app's native module. It exists so that the app target compiles Swift of its own: Exact links
// `modules/apple/*.xcframework` (static third-party libraries) into this module, and the model stages
// (Models_*.swift, copied in from AutoradioKit/Sources/AutoradioModels by scripts/sync-exact-package.sh) import them.
// The kit (package module ExactPkg_autoradio) finds AutoradioModelsRegistrar by name; see ExternalModels.swift there.
import Foundation
import ExactPkg_autoradio
import Security

// THE KEY IS EXACT'S (`secret.keep`; docs/design/account-boundary-2026-11-21.md, "Round 12"): sign-in, the key and the launch verdict are shared code (AutoradioLab/account.ts, signin.ts). The native ops that are left, each with its reason: `hello` (is a native module here: a device fact),
// `ready` (build once the launch is clean), `holdKey` (the key in memory for the Swift clients that still call the server: pages still in Swift; goes with the last of them), `forgetAccountFiles` (deletion only, of the Swift stores' own files that Exact's `app:/` does not reach).
// The mail cipher (Choir/MailSeal.swift): `mailSeal`, `mailOpen`, `mailForget`, a platform cipher shared code has none of; the key never leaves the Keychain.
// The edition picks' calls (`editionMirror`, `editionMirrorReset`, `editionHeard`, `editionSaid`): the voice's in-memory copy of the picks, and the words the voice heard at the setup screen (docs/reference/AUTORADIO-EDITION-PICKS.md).
// The take recorder's ops (`takeState` ... `takeMirror`, AutoradioModule+Takes.swift `takeOp`): shared code (AutoradioLab/take-session.ts) leads the Takes section; the reason they are native is the
// microphone, the radio's gate, the recogniser and the player (the voice), the steer queue, and the Swift programme's copy of the listener's own ids and muted voices. Nothing is carried from the pre-release build's files: they are never read.
// The page in front, `{op: "pageText", address, title, dek, kind, blocks}` (later only): the shared reader hands the voice the page it is showing (HeldPages); nothing is read back. It goes when the voice's page text is its own.
// The themes' one-time carry-over (AutoradioLab/themes.ts `migrate`): `{op: "themeLegacy"}` READS what the Swift app kept for the themes: the choice (UserDefaults) and the user's theme files
// (Application Support/Autoradio/themes). The reason it is native: those are places shared code cannot see (Exact's `app:/data` is another folder). It only reads and removes nothing, so a reverted
// build still finds its old copy; a later change removes the old copy when phones have moved, and this op goes with it. At most 64 files of at most 64 KiB are sent; the rest are named in `skipped`
// (shared code shows each as a problem under its own name). Nothing else about the themes is native.
final class ModelsModule: ExactModule {
    static var legacyThemeDir: URL { FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio/themes") }
    static func legacyThemes() -> [String: Any] {
        var files: [String: String] = [:], skipped: [String] = []
        for n in ((try? FileManager.default.contentsOfDirectory(atPath: legacyThemeDir.path)) ?? []).sorted() where n.hasSuffix(".json") {
            if files.count < 64, let d = try? Data(contentsOf: legacyThemeDir.appendingPathComponent(n)), d.count <= 65536, let t = String(data: d, encoding: .utf8) { files[n] = t } else { skipped.append(n) }
        }
        let theme = UserDefaults.standard.string(forKey: "AutoradioThemeID") ?? "", mode = UserDefaults.standard.string(forKey: "AutoradioThemeMode") ?? ""
        return ["found": !theme.isEmpty || !mode.isEmpty || !files.isEmpty || !skipped.isEmpty, "theme": theme, "mode": mode, "files": files, "skipped": skipped]
    }
    override func call(_ request: [String: Any]) -> [String: Any] {
        if (request["op"] as? String) == "hello" { return ["state": "native"] }      // shared code asks whether a native module is here (host.ts): a device fact
        if (request["op"] as? String) == "themeLegacy" { return Self.legacyThemes() }
        return MailSeal().reply(request) ?? ["found": false]
    }
    override func later(_ request: [String: Any], reply: ExactReply) {
        let op = request["op"] as? String
        // THE KEY IS EXACT'S (docs/design/account-boundary-2026-11-21.md, "Round 12"): shared code makes the launch verdict. `ready`: the launch is clean, build the module. `holdKey`: the key in memory for the Swift clients that still call the server (HeldKeys); an empty key is the
        // sign-out (the key is gone at once, then playback stops). `forgetAccountFiles`: DELETION ONLY of the Swift stores' files (Application Support, which Exact's `app:/` does not reach), answering whether none is left.
        if op == "ready" { Task { @MainActor in AutoradioModule.pending?.begin(); reply.send(["ok": true]) }; return }
        if op == "holdKey" {
            let key = request["key"] as? String ?? ""
            HeldKeys.hold(key: key, keyId: request["keyId"] as? String ?? "", computerId: request["computerId"] as? String ?? ""); reply.send(["ok": true])
            Task { @MainActor in if key.isEmpty { await AutoradioModule.current?.keyDropped() } else { AutoradioModule.current?.keyHeld() } }
            return
        }
        if op == "forgetAccountFiles" { Task.detached { reply.send(["done": AccountWipe.forgetAll()]) }; return }
        // The deck (shared code's radio runtime commands it): one op, `{op: "deck", name: deckState|deckPlay|deckEnqueue|deckAct|deckAck, args}`, answering the deck's state. Native because the deck is
        // the audio: the session, the engine and the journal that survives a kill (AutoradioModule+Deck.swift `deckCall`). It decides nothing.
        if op == "deck" {
            Task { @MainActor in reply.send(await AutoradioModule.current?.deckCall(name: request["name"] as? String ?? "", args: request["args"] as? [String: Any] ?? [:]) ?? [:]) }
            return
        }
        // Open a page by its address (the shell's: shared code names the address, only the shell shows a page): the clip's episode page from the queue's "Open episode".
        if op == "openPage" {
            Task { @MainActor in AutoradioModule.current?.requestPage(request["address"] as? String ?? "", title: request["title"] as? String ?? ""); reply.send([:]) }
            return
        }
        // The radio's mailbox (AutoradioModule+Queue.swift): `radioFacts` (what only the audio code knows, and the inbox of what native handed over), `radioSet` (the turn route's queue and the entry on air, pushed by shared
        // code). Native because the voice turn and the audio are; they decide nothing.
        if op == "radioFacts" || op == "radioSet" {
            Task { @MainActor in
                guard let m = AutoradioModule.current else { reply.send([:]); return }
                switch op { case "radioFacts": reply.send(m.radioFacts(peek: (request["peek"] as? Bool) ?? false)); default: reply.send(m.radioSet(request)) }
            }
            return
        }
        // First run's voice side (AutoradioModule+Onboarding.swift `onboardingOp`; each op's reason is in that file's header): the models' setup, the microphone check, the hearing sentence, the voice at the topic screen.
        if let op = op, op.hasPrefix("ob") {
            Task { @MainActor in reply.send(await AutoradioModule.current?.onboardingOp(request) ?? [:]) }
            return
        }
        // Retrieval (AutoradioKit/RetrievalFacts.swift, EmbedDevice.swift): shared code's answer to the voice turn's ask, and the embedding model. Neither touches the module,
        // and the model's work stays off the main thread.
        if op == "retrievalAnswer" {
            RetrievalBridge.shared.answer(id: request["id"] as? String ?? "", context: request["context"] as? [String: Any] ?? [:])
            reply.send([:]); return
        }
        if let op = op, op.hasPrefix("embed") {
            Task.detached { reply.send(await EmbedDevice.op(request)) }
            return
        }
        // The notebook's and the content sync's native facts (AutoradioModule+Notebook.swift `notebookOp`, with the reason for each).
        if let op = op, op.hasPrefix("notebook") || op == "contentBusy" {
            Task { @MainActor in reply.send(await AutoradioModule.current?.notebookOp(request) ?? [:]) }
            return
        }
        if let op = op, op.hasPrefix("take") {
            Task { @MainActor in reply.send(await AutoradioModule.current?.takeOp(request) ?? [:]) }
            return
        }
        if op == "pagesMirror" {
            // Shared code (AutoradioLab/pages.ts) owns the open pages and hands the voice their facts after every change (AutoradioModule+Spine.swift `holdPages`). Only native code can use them: the
            // voice engine, the local model's warm-up and the Texture editor's flush on leaving a page are native. Answers {ok}.
            Task { @MainActor in reply.send(["ok": await AutoradioModule.current?.holdPages(request) ?? false]) }
            return
        }
        if op == "pageText" {
            // The shared reader hands over the page in front (AutoradioLab/reader.ts `hand`); the voice turn reads it. Only native code can: the voice engine is native. Answers {ok}.
            reply.send(["ok": AutoradioModule.current?.holdPage(request) ?? false])
            return
        }
        // The edition picks' native calls (EditionSetup.swift `EditionNative`), each with the reason only native code can meet: the voice's copy of the picks and the words the voice heard.
        if let op = op, op.hasPrefix("edition") { reply.send(EditionNative.op(request)); return }
        reply.send(call(request))
    }
}
let exactModule: ExactModule.Type = ModelsModule.self
