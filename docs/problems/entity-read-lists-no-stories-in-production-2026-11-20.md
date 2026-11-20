# The public entity read lists no stories in production, and every candidate weighs 0 (2026-11-20)

**Problem.** After station E2 deployed (staging `e33d9df5f`, 12:43Z),
`GET /api/platform/entities/ent:fbf45c40b1a78ea4afa18223?stories=5` answered 200
with `stories: 0`, `story_list: []`, although the entity is listed on a front
story. The same defect zeroes the ready list's candidate weights (`mentions`,
`central`).

**Evidence.** The production edges are as the writer makes them: canonical ids
on both ends (`obj:choir.publication:<b64 owner>:key-…` to
`obj:choir.entity:<b64 owner>:key-…`, metadata `{"role":"central"}`). The two
reads found them with a range, `from_id >= 'obj:choir.publication:' AND from_id <
'obj:choir.publication;'`. In the paper's database collation (not C) `:` sorts
after `;`, so the range is empty. The test databases use C, where it is not.
Reproduced by moving `og_edges.from_id` to the `und-x-icu` collation in a test
database (`internal/platform/edge_collation_test.go`): the page answers
`Stories:0`, the weights `Mentions:0 Central:0`, exactly the production
symptom; with the prefix test `starts_with(from_id, …)` they are right. The
earlier tests passed because they ran in C.

**Belief.** The cause is the collation-dependent range, not the id form: the
ids are the production writer's (the entity page tests go through
`RecordStoryEntities`). The ready list's neighbour links
(`from_id IN (canonical capture ids)`, equality) are not affected, and a test
under the same collation confirms it. I have not read the production
collation; the reproduction is the evidence.

**Remaining error.** Other reads that range over a text id column
(`objectIDPrefix` ranges on `og_objects`) may share the defect where the column
is not `COLLATE "C"`; not audited here.

**Fix.** A prefix test (`starts_with`) in the entity read's edge query and the
weight probe; the tests run under a non-C collation; the deployed proof now
fails when a front story's entity does not list that story.
