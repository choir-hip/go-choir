// Lane file: reader. Owned by the reader lane (mobile/README.md, "File ownership"). The reader's sources and sends:
//   readerDoc(address)    the document at a pub: or texture: address, as blocks of runs, citations, sources, versions
//   texturesList(at)      the listener's documents (reloaded each time the Textures page is the page in front)
//   readerFollow          the block and sentence the radio is reading (set by lane 1 through ReaderFollow.shared.set)
//   readerAim             what the bar's prompt is aimed at: a document or one block of it
// Sends: readerAimSet, readerStop (stop the desk's revision), readerRestore (a version as latest), readerReload, readerPrefetch, readerFollowSet.
import Foundation
import SafariServices
import UIKit

/// The in-app browser: SFSafariViewController over whatever is on screen, with Done to return.
enum InAppBrowser {
    static func present(_ url: URL) {
        let scenes = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }
        let window = scenes.first { $0.activationState == .foregroundActive }?.windows.first { $0.isKeyWindow } ?? scenes.flatMap { $0.windows }.first { $0.isKeyWindow }
        guard var top = window?.rootViewController else { return }
        while let next = top.presentedViewController, !next.isBeingDismissed { top = next }
        let vc = SFSafariViewController(url: url)
        vc.dismissButtonStyle = .done
        top.present(vc, animated: true)
    }
}

extension AutoradioModule {
    func registerReader() {
        pageSources.register(ReaderPageSource())
        ReaderFollow.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["readerFollow"]) } }
        // Only ever read and written on the main thread: Exact runs every handler there, and the write below hops to it.
        var lastList = TextureList()

        handle("readerDoc") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            guard TextureAddress.parse(address).kind != .none else { return reply(json(ReaderDoc.loading(""))) }
            Task { [weak self] in
                // A document the desk is revising is read again each time it is asked for (and asked for again every few seconds).
                let again = ReaderStore.shared.cached(address)?.revising ?? false
                let d = await ReaderStore.shared.doc(address, force: again)
                DispatchQueue.main.async {
                    reply(self?.json(d) ?? [:])
                    if d.revising { DispatchQueue.main.asyncAfter(deadline: .now() + 5) { self?.touch(["readerDoc"]) } }
                }
            }
        }
        handle("texturesList") { [unowned self] args, reply in
            guard (args["at"] as? String ?? "") == "textures:" else { return reply(json(lastList)) }
            Task { [weak self] in
                let l = await TextureList.load()
                DispatchQueue.main.async { lastList = l; reply(self?.json(l) ?? [:]) }
            }
        }
        handle("readerFollow") { [unowned self] _, reply in reply(json(ReaderFollow.shared.info)) }
        handle("readerAim") { [unowned self] _, reply in reply(json(ReaderAim.shared.info)) }

        handle("readerAimSet") { [unowned self] args, reply in
            let info = ReaderAimInfo(address: args["address"] as? String ?? "", blockId: args["blockId"] as? String ?? "", label: args["label"] as? String ?? "")
            if info.address.isEmpty { ReaderAim.shared.clear() } else {
                ReaderAim.shared.set(info)
                signals.record(SignalEvent(kind: "aim", subject: info.address, detail: info.blockId))
            }
            touch(["readerAim"])
            reply(json(ReaderAim.shared.info))
        }
        handle("readerStop") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            Task { [weak self] in
                await ReaderStore.shared.cancelRevision(document: address)
                DispatchQueue.main.async { self?.touch(["readerDoc", "texturesList"]); reply(["ok": true]) }
            }
        }
        handle("readerRestore") { [unowned self] args, reply in
            let address = args["address"] as? String ?? "", version = args["version"] as? String ?? ""
            Task { [weak self] in
                let ok = await ReaderStore.shared.restore(document: address, revisionAddress: version)
                DispatchQueue.main.async { self?.touch(["readerDoc", "texturesList"]); reply(["ok": ok]) }
            }
        }
        // The stack's neighbours: the card above and below the one on top are fetched and decoded now, so a swipe finds them held.
        handle("readerPrefetch") { args, reply in
            ReaderStore.shared.prefetch([args["prev"] as? String ?? "", args["next"] as? String ?? ""])
            reply(["ok": true])
        }
        handle("readerReload") { [unowned self] args, reply in
            ReaderStore.shared.invalidate(args["address"] as? String ?? "")
            touch(["readerDoc"])
            reply(["ok": true])
        }
        // A source or article link: web pages open in an in-app browser sheet, mail and phone links go to the system, the rest is refused.
        // Presenting does not touch the audio session, so the radio keeps playing under the sheet.
        handle("readerOpenLink") { args, reply in
            switch SourceLink.route(args["url"] as? String ?? "") {
            case .inApp(let u): DispatchQueue.main.async { InAppBrowser.present(u) }; reply(["ok": true])
            case .external(let u): DispatchQueue.main.async { UIApplication.shared.open(u) }; reply(["ok": true])
            case .refused: reply(["ok": false])
            }
        }
        handle("readerFollowSet") { [unowned self] args, reply in
            let address = args["address"] as? String ?? ""
            Task { [weak self] in
                let known = await ReaderStore.shared.doc(address)       // makes sure the document is cached
                _ = known
                DispatchQueue.main.async {
                    guard let self else { return reply([:]) }
                    reply(self.json(ReaderFollow.shared.set(address: address, spoken: args["spoken"] as? String ?? "")))
                }
            }
        }
    }
}
