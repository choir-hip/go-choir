# The owner sees a blank screen during a staging deploy

Reported by the owner 2026-10-13 about 14:51 UTC: choir.news, logged in,
showed a blank screen; a few minutes later it loaded. The staging deploy of
761d94c5 ran 14:48:24-14:51:32 UTC (CI run for 761d94c5, "Deploy to Staging
(Node B)"), and staging reported 761d94c5 right after. No frontend change was
in that push (it fixed the guest runtime config), so this is the deploy
window, not a code regression.

Hypothesis (not yet confirmed from a trace): during the deploy the proxy
restarts and the frontend's hashed assets are replaced; a page that loads in
that window either gets no response or an index.html whose script chunks are
not served yet, and the app shell renders nothing instead of a retry or a
"updating" notice.

Next observation: load choir.news (logged in and out) through a deploy and
record the HTTP status of the index and its chunk requests and any console
error. Candidate fixes after that: keep the previous asset set servable
across a deploy, and have the shell show a reload notice on a chunk-load
failure rather than a blank page.
