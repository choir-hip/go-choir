# Staging Evidence: Competing Parent Work Items Cause Assignment Supersession Loop

- Date: 2026-08-21
- Mutation class: Red
- Computer: `computer-03335285269bdba4f94377e56879f9e6`
- Guest realization epoch: 361
- Guest build: `e42c65c04f0681e4dd695853ed1396fed736a467`

## Observation

After the capsule memory-budget reclaim fix (`ad8b1d73` + `5b046651` +
`3ccdefa7` + `e42c65c0`), the reclaim mechanism works correctly — capsules are
revoked and budget released. However, a new loop emerged:

Two distinct parent work items (`4671d318…` and `69558aae…`) each hold a
pending Texture execution request for a *different* self-development operation.
Each persistent-Super cycle processes both requests, opening a fresh assignment
for one and then the other. Each new assignment supersedes the previous one via
the reclaim path, cancelling the prior CoSuper before it can complete any work.

Timeline evidence (all 2026-08-21):

```
17:38:45 assignment-…72b91dbd parent=4671d318
17:36:04 assignment-…4fde7f7e parent=4671d318
17:32:59 assignment-…89756b87 parent=69558aae
17:31:24 assignment-…fe5c7dc3 parent=4671d318
17:28:14 assignment-…276c4a8a parent=69558aae
17:26:07 assignment-…38615eb2 parent=4671d318
17:23:43 assignment-…f69379aa parent=69558aae
17:22:21 assignment-…0df3f5f4 parent=4671d318
```

Every CoSuper is cancelled with "superseded by a fresh implementation
assignment" before it can author, build, or freeze anything.

## Root cause

Multiple stale Texture execution-request controls (for operations
`selfdev-3f842968…`, `selfdev-4379920e…`, `selfdev-8dcdd2c5…`,
`selfdev-1b9489ea…`) remain pending in the lifecycle mailbox. Each Super cycle
drains them and calls `assign_co_super` for each, but the one-live-capsule
reclaim path cancels the previous assignment to free budget. With two competing
requests, the system ping-pongs indefinitely.

## Required repair direction

The Super must select exactly one pending execution request and drain/complete
it before opening another, rather than opening assignments for every pending
request in sequence. Alternatively, stale duplicate execution requests for
operations that already have terminal CoSuper attempts should be settled as
late evidence without spawning new assignments.

Effects remain OFF; no candidate artifact, bundle, proposal, promotion, or
live state write exists. Rollback: revert through origin/main + CI/deploy;
checkpoint `99949fe2` remains untouched.
