# W1 completion panel: captures can bypass their route, and a failed write loses items

Found 2026-10-11 by the W1 completion panel (codex, reviewing 010b22f8,
code identical to the deployed ebe02b17). Evidence class: code reading,
confirmed against the files below. No staging exploit was attempted.

## What the panel found

1. **A guest can write a capture through the generic object route.**
   `/internal/platform/objects` accepts any registered kind. The proxy
   checks only that the submitted object names the guest's own owner and
   computer (`internal/proxy/guest_authority.go`), and corpusd sees the
   proxied request as an internal caller, then upserts
   (`internal/platform/objectgraph_handlers.go` putObject,
   `internal/platform/objectgraph_store.go` PutObject). So:
   - a guest can overwrite a capture another computer of the same owner
     wrote, by resubmitting its canonical id with its own computer id;
   - a guest can write a `choir.web_capture` whose body is under 1 KiB,
     which the store keeps inline in the Dolt row, breaking the rule that
     no capture body is in a row.
   The capture route itself is sound; the generic route is the bypass.

2. **A capture-write failure can lose a feed's items.** `PollSourceOnce`
   (`internal/researchtools/poll_source.go`) saves the feed's validators
   (ETag, Last-Modified, cursor) as soon as the fetch succeeds, before
   corpusd accepts the items. If the write fails, the next poll sends the
   same validators, gets 304, writes nothing, and the schedule records a
   success. The items are lost until the feed changes or the process
   restarts.

3. **Suggestion: a duplicate or retried wake can poll twice.** The
   handler checks the wake's generation but not its due time, so two
   wakes of one generation (a replay, or an actor retry after the poll ran
   but before the commit) both poll.

## Discovered, wider than W1

The same-owner, other-computer overwrite in finding 1 applies to every
object kind written through `/internal/platform/objects`, not only
captures: corpusd cannot tell a proxied guest from an internal caller, and
neither the proxy nor corpusd compares a write with the existing object's
computer. W1 closes it for captures; the general boundary needs the proxy
to pass the guest's binding to corpusd (or corpusd to check existing
ownership), which is a separate auth-path change.

## Test

- A guest's generic write of a `choir.web_capture` is refused, through the
  proxy and directly at corpusd; captures are written only through the
  capture route.
- A poll whose capture write fails keeps the previous validators, so the
  next poll fetches the items again and writes them.
- A wake whose due time is not the schedule's next due time does nothing.
