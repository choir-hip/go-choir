// Lane file: reader. Owned by the reader lane (mobile/README.md, "File ownership"). The reader's sources and sends:
//   readerDoc(address)    the document at a pub: or texture: address, as blocks of runs, citations, sources, versions
//   texturesList(at)      the listener's documents (reloaded each time the Textures page is the page in front)
//   readerFollow          the block and sentence the radio is reading (set by lane 1 through ReaderFollow.shared.set)
//   readerAim             what the bar's prompt is aimed at: a document or one block of it
// Sends: readerAimSet, readerStop (stop the desk's revision), readerRestore (a version as latest), readerReload, readerFollowSet.
import Foundation

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
        handle("readerReload") { [unowned self] args, reply in
            ReaderStore.shared.invalidate(args["address"] as? String ?? "")
            touch(["readerDoc"])
            reply(["ok": true])
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
