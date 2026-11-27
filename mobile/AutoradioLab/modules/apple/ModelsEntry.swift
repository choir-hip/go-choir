// The app's native module. It exists so that the app target compiles Swift of its own: Exact links
// `modules/apple/*.xcframework` (static third-party libraries) into this module, and the model stages
// (Models_*.swift, copied in from AutoradioKit/Sources/AutoradioModels by scripts/sync-exact-package.sh) import them.
// The kit (package module ExactPkg_autoradio) finds AutoradioModelsRegistrar by name; see ExternalModels.swift there.
import Foundation
import ExactPkg_autoradio
import Security

// `keystate` (read only, no data returned): present | absent | unreadable for the sign-in key. Shared code (host.ts) uses it as the probe "is the app module here", and the sign-out reconcile does not (it reads the Exact
// secret store). The key itself is kept by shared code as the Exact secrets choir.key, choir.key-id and choir.computer-id (AutoradioLab/app-signin.ts); the voice code reads the same Keychain items (KeychainKeyStore).
//
// The mail cipher (Choir/MailSeal.swift): `mailSeal`, `mailOpen`, `mailForget`, a platform cipher shared code has none of; the key never leaves the Keychain.
//
// The edition picks' calls (`editionMirror`, `editionMirrorReset`, `editionHeard`, `editionSaid`): the voice's in-memory copy of the picks, and the words the voice heard at the setup screen (docs/reference/AUTORADIO-EDITION-PICKS.md).
//
// The second native call, `{op: "signOut"}` (later only), answers `{done: Bool}` once the native half of sign-out has finished. Sign-out is led
// by shared code (AutoradioLab/account.ts); the reason it is native is that the Swift stores
// (AccountWipe: the voice's and the pages not yet moved) are wiped by Swift, and the voice code deletes the Keychain items there. It goes with them when those stores move.
// The sign-out reconcile (AutoradioLab/account.ts) needs the stored key as THREE answers, which the reader's `nil` cannot give: `keystate` -> present (the item is
// there), absent (the Keychain answered item-not-found) or unreadable (locked before the first unlock, or any other status). Read only: no data is returned, nothing
// is written, moved or re-keyed. `wipe` is the native half of an ACCOUNT CHANGE: the old account's Swift stores and mail key go, the stored key stays.
//
// The take recorder's ops (`takeState` ... `takeAudio`, AutoradioModule+Takes.swift `takeOp`): shared code (AutoradioLab/take-session.ts) leads the Takes section; the reason they are native is the
// microphone, the radio's gate, the recogniser and the player (the voice), and the steer queue. Nothing is carried from earlier builds' files: they are never read.
//
// The page in front, `{op: "pageText", address, title, dek, kind, blocks}` (later only): the shared reader hands the voice the page it is showing (HeldPages); nothing is read back. It goes when the voice's page text is its own.
//
final class ModelsModule: ExactModule {
    static func keyState() -> String {
        let q: [String: Any] = [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: KeychainKeyStore.defaultService, kSecAttrAccount as String: KeyAccount.apiKey, kSecMatchLimit as String: kSecMatchLimitOne]
        switch SecItemCopyMatching(q as CFDictionary, nil) { case errSecSuccess: return "present"; case errSecItemNotFound: return "absent"; default: return "unreadable" }
    }
    override func call(_ request: [String: Any]) -> [String: Any] {
        #if targetEnvironment(simulator)
        // The real-data proof only (compiled out of device builds): the `proof-key` file is imported into the Keychain by the package module's init, which can run AFTER the first
        // shared-code read on a fresh install (the first real drive after an install showed Settings, Files and Mail signed out for the whole drive, the answer being kept). Importing
        // here, before any read of the key, removes that race; the file is deleted by the import, so every later call finds none.
        if let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first { ProofKey.importIfPresent(documents: docs, into: KeychainKeyStore()) }
        #endif
        if (request["op"] as? String) == "keystate" { return ["state": Self.keyState()] }
        return MailSeal().reply(request) ?? [:]
    }
    override func later(_ request: [String: Any], reply plain: ExactReply) {
        // The ledger (NativeOpLog.swift): op name only, never an argument. A reply never called is named.
        let op = request["op"] as? String
        let reply = LoggedReply(NativeOpLog.shared.wrap(op: op ?? "none", reply: { plain.send($0) }))
        if op == "generation" {      // shared code says the generation it is at: the first call of a JS context is a boot (it may be below the newest seen: a reload), the wipe's last act a floor
            let g = (request["gen"] as? NSNumber)?.intValue ?? 0
            if request["boot"] as? Bool == true { SharedGeneration.boot(g) } else { SharedGeneration.announce(g) }
            reply.send(["ok": true]); return
        }
        if op == "signOut" || op == "wipe" {
            if op == "signOut" { TurnGate.shared.closeForSignOut() }; SharedGeneration.announce((request["gen"] as? NSNumber)?.intValue ?? 0)      // the gate shuts first, synchronously, the first moment native knows (ChoirApp.signOut shuts it again, idempotently); shared code's generation moved when the sign-out began: what it pushed under an older one is refused from now on
            Task { @MainActor in
                guard let m = AutoradioModule.current else { reply.send(["done": false]); return }
                if op == "signOut", let keyId = request["keyId"] as? String, !keyId.isEmpty, KeychainKeyStore().get(KeyAccount.keyID) != keyId { reply.send(["done": false, "superseded": true]); return }      // a sign-in kept another key meanwhile: it is not the one being signed out, and nothing of it is deleted
                await m.signOutForShared(keepKey: op == "wipe")
                MailSeal().forget()      // the mail copy's key goes with the account, whatever shared code does next
                // Truthful: the Keychain delete ignores its status (ChoirClient.swift), so read the item again. Still there: not done. The Account screen's sign-out then wipes
                // nothing shared (the key remains: the marker goes); at a launch (account.ts reconcile) the shared stores of the account being left are wiped regardless of this answer and the marker stays. Idempotent: with the key already gone it only runs the Swift wipe again.
                reply.send(["done": op == "wipe" || !KeychainKeyStore.keyRemains()])
            }
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
        // The content sync's one native fact (AutoradioModule+Notebook.swift `notebookOp`, with its reason).
        if op == "contentBusy" {
            Task { @MainActor in reply.send(await AutoradioModule.current?.notebookOp(request) ?? [:]) }
            return
        }
        if let op = op, op.hasPrefix("take") {
            Task { @MainActor in reply.send(await AutoradioModule.current?.takeOp(request) ?? [:]) }
            return
        }
        if op == "pagesMirror" {
            // Shared code (AutoradioLab/pages.ts) owns the open pages and hands the voice their facts after every change (AutoradioModule+Spine.swift `holdPages`). Only native code can use them: the
            // voice engine and the local model's warm-up are native (the Texture editor's flush is shared code's: texture-edit.ts). Answers {ok}.
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
