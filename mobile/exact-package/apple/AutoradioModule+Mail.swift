// Lane file: mail. What is left of Swift mail (2026-11-19: the mailbox, the messages, drafts and the phone's copy are shared code, AutoradioLab/mail.ts).
// Two sends stay here because only native code can reach what they touch: the app's own web view (the approval page) and the radio's voice
// (read it aloud). The phone never talks to a mail provider: that is all in mail.ts.
import Foundation

extension AutoradioModule {
    func registerMail() {
        // (Sign-out of the mail copy is shared code's: mail.ts `wipeMail`, run by account.ts after the native sign-out; the native key goes in `signOut`.)
        // Open the approval page in the app's own web view like any web link (SourceLink: http and https are in-app; mailto, tel, sms and
        // facetime go to the system; anything else is refused), only on the configured Choir host (same scheme, host and port), over https.
        // Native because the in-app web view is a native view.
        handle("mailOpenReview") { [weak self] args, reply in
            if let u = URL(string: args["url"] as? String ?? ""), u.scheme == "https", RemoteHost.allows(u) {
                DispatchQueue.main.async { [weak self] in
                    self?.barWebURL = u.absoluteString; EmbeddedPage.shared.opened(u.absoluteString)
                    self?.touch(["queueView", "barFacts"], now: true)
                }
                reply(["status": "opened", "message": ""])
            } else {
                reply(["status": "refused", "message": "That link is not on your computer's address."])
            }
            _ = self
        }

        // "Read this to me": the radio reads the text the page hands over (the message's `spoken`, made by mail.ts) as a programme item
        // (LabRuntime.speakText through the handler MailReading holds). Native because it is the voice. The signal records that it was asked, without the text.
        handle("mailRead") { [weak self] args, reply in
            let address = args["address"] as? String ?? "", text = args["text"] as? String ?? ""
            guard !text.isEmpty else { reply(["status": "error", "message": "There is nothing to read yet."]); return }
            MailReading.shared.request(address: address, text: text)
            reply(["status": "requested", "message": "Reading it now."])
        }
    }
}
