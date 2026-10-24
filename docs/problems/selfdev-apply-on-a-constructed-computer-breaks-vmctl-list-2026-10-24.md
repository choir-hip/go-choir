# A self-development apply on a constructed computer breaks vmctl's list, and that computer's routing (2026-10-24)

Date: 2026-10-24, 04:4x UTC. Found by: FP0, during the overnight app rewrites on
b@c.com's computer. Class of this record: green. Class of the fix: red
(route/evidence classification, which deploy refresh selection and the proxy's
ownership lookup read). Status: recorded; the fix follows in the next commit.

## Evidence

- 04:27:04: the owner approval (an owner:acts key, minted by the owner for this
  run) of operation selfdev-41ea6ea6668ad096b24b28fdc626ed75 (the Slides rewrite)
  on computer-6c50d6e566143ae32cdb660faa60e24d. Its VM is
  candidate-fleet-bf04bbc7215c4bad1b47fb47, an ownership constructed on
  2026-07-17 (`snapshot_kind: constructed-computer-version`, divergence `canary`).
  This was the first self-development apply on that computer. Tonight's passing drill
  computers are minted computers, which have no route slot.
- From 04:32:20, `GET /internal/vmctl/list` answers 503 for every computer:
  `classify immutable active ownership candidate-fleet-bf04bbc7215c4bad1b47fb47:
  ownership candidate-fleet-bf04bbc7215c4bad1b47fb47 has unrecognized route
  execution evidence`. The store compaction pressure sweep logs the same error.
- `choir computer status` for that computer through the proxy: 503 `computer
  ownership authority unavailable`. Other computers route normally: J1 and the
  drill computer answered at 04:4x. The public site answers 200, and the
  paper's liveness probe passes.

## Cause (verified in source)

`RouteAuthority.ApplySelfDevelopmentProjection` and
`ApplyPlatformFollowRouteProjection` (`internal/vmctl/self_development_route.go`)
both write a promote receipt through `applySignedProjection`. Its gate payload is
a `selfdevprotocol.AcceptedEventAuthorizationEvidence` and its certificate a
`PromotionJoinEvidence`. The self-development writer requires
`DecisionScope` to start with `computer:self_development:`.
`constructedOwnershipIdentity` (`internal/vmctl/route_authority.go`) recognizes
the constructed bootstrap/promotion class and the platform-follow class. It
recognizes the second only when `DecisionActor == "platform"` and
`DecisionScope == "computer:platform_follow:update"`. A self-development
promotion (actor `owner`, scope `computer:self_development:approve`) on a
constructed slot falls through to "unrecognized route execution evidence".

`listOwnershipResponses` fails the whole list on one classification error. That
is deliberate fail-closed behaviour, pinned by
`TestHandleListClassifiesConstructedOwnershipFromRouteEvidence`. Its two readers
are the operator list and the deploy refresh selection. The resolve and
authorize paths for that ownership refuse the same way. So the next deploy's
refresh selection fails, and the owner cannot open that computer.
`TestSelfDevelopmentRouteProjectionRequiresExactPlatformCertificate` applies a
self-development projection but never classifies the slot afterwards. That
is the gap the bug went through.

## Fix

Recognize the self-development promotion class in `constructedOwnershipIdentity`
with the same precondition its writer enforces: a non-empty actor and a
`computer:self_development:` scope. Then apply the same `PromotionJoinEvidence`
join as platform-follow. The list keeps failing closed for evidence that is
really unknown. The evidence the classifier now accepts was already verified
against the platform-control signatures when it was written.

## Not in this fix (recorded)

- One computer's unclassifiable evidence takes down the whole list and the
  deploy refresh selection for every computer. That fail-closed choice is
  deliberate. Its blast radius is now a measured cost: about 20 minutes so far,
  and it was ready to fail the 06:52 deploy. Whether a per-entry refusal is
  safer is a separate design question for the owner.
- Constructed computers are rare: b@c's, and possibly the owner's older
  computers. Any of them would break the same way on its first
  self-development apply.

## Heresy delta

discovered: a self-development apply on a constructed route slot makes vmctl's
list fail closed for every computer and makes that computer unroutable.
introduced: none. repaired: none in this commit.
