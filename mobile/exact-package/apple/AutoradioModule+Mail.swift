// Lane file: mail. Owned by the mail lane (mobile/README.md, "File ownership"). Register this lane's sources and sends here:
//   handle("someSource") { args, reply in reply(json(...)) }          // the names are declared in exact-package/interface/mail.json
// Reach the shared kit objects through self: pages, signals, themes, pageSources (register a PageSource for your
// kinds), runtime, choir; call touch(["someSource"]) when a live source changes. The spine calls registerMail() once.
//
// The phone never talks to a mail provider: every call goes to the listener's computer (the Mail agent is the sole
// custodian of email there) through MailService in the kit.
import Foundation
import UIKit

extension AutoradioModule {
    func registerMail() {
        let service = MailService()
        pageSources.register(MailPageSource(service: service))

        handle("mailBox") { [weak self] args, reply in
            let address = args["address"] as? String ?? ""
            Task { let v = await service.box(address: address); reply(self?.json(v) ?? [:]) }
        }

        // The message, and beside it the sanitised original document ("Show original") written where an iframe can read it.
        handle("mailMessage") { [weak self] args, reply in
            let address = args["address"] as? String ?? ""
            Task {
                let v = await service.message(address: address)
                if v.hasOriginal, let html = service.originalDocument(id: v.id), let url = self?.context.file(v.originalPath) {
                    try? FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
                    try? Data(html.utf8).write(to: url, options: .atomic)
                }
                if v.status == "ready" { DispatchQueue.main.async { self?.touch(["mailBox"]) } }   // the read mark changed the counts
                reply(self?.json(v) ?? [:])
            }
        }

        handle("mailCompose") { [weak self] args, reply in
            let address = args["address"] as? String ?? ""
            Task { let v = await service.compose(address: address); reply(self?.json(v) ?? [:]) }
        }

        handle("mailSaveDraft") { [weak self] args, reply in
            Task {
                let v = await service.saveDraft(address: args["address"] as? String ?? "", to: args["recipients"] as? String ?? "",
                                                subject: args["subject"] as? String ?? "", body: args["body"] as? String ?? "")
                if v.status == "saved" { DispatchQueue.main.async { self?.touch(["mailBox"]) } }
                reply(self?.json(v) ?? [:])
            }
        }

        // Sending is an owner act. The server refuses it from a key, so the phone asks for the approval link by email.
        handle("mailApprovalEmail") { [weak self] args, reply in
            Task {
                let v = await service.approvalEmail(address: args["address"] as? String ?? "", draft: args["draft"] as? String ?? "")
                reply(self?.json(v) ?? [:])
            }
        }

        // Open the approval page in the browser, only on the configured Choir host (same scheme, host and port), over https.
        handle("mailOpenReview") { [weak self] args, reply in
            if let u = URL(string: args["url"] as? String ?? ""), u.scheme == "https", RemoteHost.allows(u) {
                DispatchQueue.main.async { UIApplication.shared.open(u) }
                reply(["status": "opened", "message": ""])
            } else {
                reply(["status": "refused", "message": "That link is not on your computer's address."])
            }
            _ = self
        }

        // "Read this to me": the radio reads the message as a programme item (LabRuntime.speakText through the handler that
        // MailReading holds). The signal records that it was asked, without the text.
        handle("mailRead") { [weak self] args, reply in
            let address = args["address"] as? String ?? ""
            Task {
                let v = await service.message(address: address)
                guard v.status == "ready" else { reply(["status": "error", "message": v.message]); return }
                MailReading.shared.request(address: address, text: v.spoken)
                self?.signals.record(SignalEvent(kind: "read-aloud", subject: address, detail: "mail"))
                reply(["status": "requested", "message": "Reading it now."])
            }
        }

        // "Summarise this thread", "Reply to ...": the prompt aimed at a message. Recorded as an ask signal about the page, the
        // same kind the bar's Ask records, so whoever answers asks (the desk) takes it from there.
        handle("mailAsk") { [weak self] args, reply in
            let address = args["address"] as? String ?? "", q = args["question"] as? String ?? ""
            self?.signals.record(SignalEvent(kind: "ask", subject: address, detail: q))
            reply(["status": "asked", "message": "Asked: \(q)"])
        }

        handle("mailMarkUnread") { [weak self] args, reply in
            let address = args["address"] as? String ?? ""
            Task {
                let ok = await service.setRead(id: PageAddress.rest(of: address), read: false)
                DispatchQueue.main.async { self?.touch(["mailBox", "mailMessage"]) }
                reply(["status": ok ? "ok" : "error", "message": ok ? "" : "Could not mark it unread."])
            }
        }
    }
}
