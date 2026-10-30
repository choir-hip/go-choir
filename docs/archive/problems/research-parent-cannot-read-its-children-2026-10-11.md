# The research parent cannot read most of what its children return (2026-10-11)

Mission: [research fan-out](../definitions/choir-research-fanout-2026-10-10.md),
slice S1. Problem first; no fix here.

## Evidence

S1 demo on staging 0b7250ef, document ee7e8c2b. Both research runs used
`choir.Research` unprompted.

1. **Child findings cut at 4,000 characters.** Run 7f1242cf listed `u` and
   `r` (three children's results). The cell showed
   `=> r: [{... Findings: ...} ... [6707 bytes truncated]`: the first
   child's findings and the start of the second's. The second child had
   found an Automation Anywhere / Boost.ai acquisition (October 7) and a
   Rein Security funding round (October 8), both flagged unverified; the
   third child's findings were not shown at all. The run then verified
   and reported only the two items in what it could see (Mistral Large 4,
   OpenAI GPT-6). Texture's revision 3 carried those two items.
2. **Raw JSON printed as byte numbers.** Run 0abcac12 wrote
   `a, ea := choir.WebSearch(...)` and `println(a)`. The output was
   `[123 34 95 95 99 104 ...]` for about 10 KB per search. Go prints a
   `json.RawMessage` (a byte slice) that way under `println`, `fmt.Println`
   and `%v` alike. The earlier display fix (431ca252) renders bytes as
   text only for the cell's final value and listed names, not for prints.
3. **Role name taken for a model name.** Run 0abcac12 first asked for
   children with `"model": "research"`. The refusal listed the 19 catalog
   models; the run picked `gpt-5.4-mini` from that list, which this
   computer cannot serve
   (`research-child-picks-unserved-model-2026-10-11.md`, fixed in
   8aa8559b by gating on the model policy). The model reached for the
   policy's own vocabulary (a role) and was refused.

## Cause

1. `maxShownListedValue = 4000` (`internal/yaegikernel/session.go`) was
   sized for probing state, not for a verb whose whole result is several
   multi-thousand-character reports. The cell's output limit is 2 MiB.
2. The world verbs return `json.RawMessage`, which has no `String`
   method, so every print path formats it as a byte slice.
3. The child model gate accepts model IDs only.

## Effect

The fan-out did its work and the parent discarded most of it: the parent
reported two items when its children had found at least four candidates.
In the other run, the parent could not read its own searches.

## Fix direction

1. Show a listed value in full up to a bound sized to the cell's output
   limit, not a probe-sized one.
2. Return world-verb JSON as a byte-slice type that prints as text and
   marshals as raw JSON, so `println`, `fmt.Println`, `%v` and
   `json.Marshal` all show the JSON. Parameters that take JSON accept any
   byte slice.
3. Accept a model policy role name as a child model: it means that role's
   model, on that role's provider.
