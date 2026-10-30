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

4. **A failed actor commit loses the successor wake** (GPT-6.1 Sol and
   GPT-6 Luna). The handler saves the advanced schedule outside the actor
   transaction; the successor wake is buffered into that transaction. If
   the commit fails, the successor is dropped and the original wake is
   retried. Checking the due time alone would refuse that retry and leave
   the schedule active with no wake; not checking it polls early.

5. **A schedule can sit active with no wake** if re-arming keeps failing
   until the actor poisons the wake (Claude, GLM, DeepSeek). Nothing shows
   it except a log line.

6. **A large poll can exceed corpusd's request bound** (GLM): up to 100
   items of up to 1 MiB each can make a request larger than 64 MiB, which
   corpusd refuses on every poll.

Guests reach corpusd directly on port 8086 (`CHOIR_PLATFORM_URL`), not
only through the proxy (DeepSeek), so corpusd's own checks carry the
generic-route guard as well as the proxy's.

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
- A wake of the current generation whose due time already ran does not
  poll; it re-arms the schedule's saved next poll, which restores a
  successor lost to a failed commit.
- The schedule list marks an active schedule whose next poll is long past
  as overdue.
- The capture sink splits a poll's items into requests well under
  corpusd's bound.

## Second round (panel on 1ddb0606)

7. **Two computers of one owner can race to create the same capture**
   (Codex, GPT-6 Luna). `WriteCaptures` (`internal/sourcegraph/captures.go`)
   checks that the capture id is absent, then creates it; Store B's
   `PutObject` (`internal/platform/objectgraph_store.go`) is an upsert that
   replaces `computer_id`, the body reference and the metadata. If two
   computers of the same owner post the same new item at once, both see no
   row, and the later write replaces the earlier computer's capture, while
   both report it as new. The capture id is per owner, so computers of
   different owners cannot collide. Not reproduced on staging, where one
   computer polls.

8. **Suggestion: a large error page is saved into every poll's usage**
   (Claude). The capture sink copies up to 4 MiB of corpusd's error
   response into the error text, which the schedule keeps in its last 24
   polls and writes on every poll.

Named for W2, not W1 blockers: the capture route has no per-guest quota;
a capture is what the writing guest says it is, so shared readers should
filter by a trusted owner or computer; items beyond a poll's `max_items`
are never fetched once the validators are kept; a body file written before
a failed row write is orphaned.

### Test (second round)

- Two computers of one owner create the same capture at once: exactly one
  row exists, it keeps the first writer's computer and body, and the other
  request counts a repeat.
- A poll's saved error text is bounded.
