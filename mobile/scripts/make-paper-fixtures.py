#!/usr/bin/env python3
"""Writes mobile/fixtures/demo/front-big.json, front-even.json, front-thin.json: invented stories, three kinds of day."""
import json, sys, os
from datetime import datetime, timedelta, timezone

OUT = sys.argv[1]
BASE = datetime(2026, 11, 10, 9, 0, tzinfo=timezone.utc)

def E(*pairs):
    return [{"id": i, "label": l} for i, l in pairs]

def S(route, title, dek, section, imp, src, ver, hours_ago, ents=(), words=300, kind=""):
    d = {"title": title, "dek": dek, "route": {"bare": route}, "section": section, "importance": imp, "source_count": src,
         "version_count": ver, "words": words, "updated_at": (BASE - timedelta(hours=hours_ago)).strftime("%Y-%m-%dT%H:%M:%SZ"),
         "entities": E(*ents)}
    if kind: d["kind"] = kind
    return d

HB = ("harbor-barrier", "Harbor tide barrier")
HC = ("harbor-council", "Harbor council")

# A big-news day: one dominant story with follow-ups, a long-headline pair that must stack, briefs, a river, opinion.
big = [
    S("demo-harbor-barrier", "The harbor council approves a tide barrier",
      "After a year of hearings, the council voted seven to two to fund a movable barrier across the harbor mouth, the largest public works contract in the city's history.",
      "World", 10, 27, 22, 0.5, [HB, HC]),
    S("demo-big-barrier-cost", "Barrier contract set at 1.4 billion over nine years, the council's budget office says", "", "World", 6, 8, 4, 1, [HB]),
    S("demo-big-barrier-fishers", "Fishing crews ask for a guaranteed gap in the barrier during the autumn run", "", "Local", 6, 5, 3, 2, [HB]),
    S("demo-big-barrier-vote", "Two dissenting members say the vote came too fast", "", "World", 6, 6, 2, 1.5, [HC]),
    S("demo-big-barrier-timeline", "Construction could begin in the spring if permits clear by January", "", "World", 6, 4, 2, 3, [HB]),
    S("demo-big-pair-a", "Regional rail operator proposes cutting off-peak service on eleven lines to close a widening budget gap, citing fewer weekday riders",
      "The proposal goes to a public comment period next week.", "Local", 7, 6, 3, 2),
    S("demo-big-pair-b", "National statistics office revises last quarter's output upward after late reports from construction and farm cooperatives arrive",
      "", "Business", 7, 6, 2, 3),
    S("demo-grid-storage", "A regional grid adds its largest battery yet", "The new installation can hold enough power for a small city through an evening peak.",
      "Tech and AI", 6, 4, 3, 4, [("grid-battery", "Grid storage")]),
    S("demo-big-b1", "Port authority tests quieter cranes overnight", "", "Local", 5, 3, 1, 3),
    S("demo-big-b2", "Hospital group opens a late clinic for walk-ins", "", "Local", 5, 3, 1, 5),
    S("demo-big-b3", "Winter fares announced for the ferry service", "", "Local", 5, 3, 1, 6),
    S("demo-big-b4", "Chip maker delays its next processor by a quarter", "", "Tech and AI", 5, 3, 1, 7),
    S("demo-big-b5", "City choir announces a winter season of free concerts", "", "Culture", 5, 3, 1, 8),
    S("demo-big-b6", "Wheat prices ease as the harvest report lands", "", "Business", 5, 3, 1, 9),
    S("demo-library-hours", "City libraries extend their evening hours", "Branches in six neighbourhoods will stay open until ten on weekdays.", "Culture", 5, 4, 1, 10),
    S("demo-river-festival", "The river festival returns with a quieter schedule", "Organisers cut the fireworks and added a morning of boat tours.", "Local", 5, 4, 2, 11),
    S("demo-big-t1", "Open-source toolkit adds offline mode for field teams", "", "Tech and AI", 5, 3, 1, 12),
    S("demo-big-t2", "Researchers publish a small, fast model for transcribing radio", "", "Tech and AI", 5, 3, 1, 13),
    S("demo-big-op", "The barrier is a bet on a century we cannot see", "The case for patience in the harbor debate.", "Opinion", 5, 1, 1, 6, [HB], kind="opinion"),
]

# An even day: two stories of near weight (colead), a pair that fits side by side only wide, briefs, river.
even = [
    S("demo-grid-storage", "A regional grid adds its largest battery yet", "The new installation can hold enough power for a small city through an evening peak.",
      "Tech and AI", 8, 12, 9, 1, [("grid-battery", "Grid storage")]),
    S("demo-even-ai", "A university lab releases open weights for a small speech model", "Engineers say it runs on a mid-range phone with no network.",
      "Tech and AI", 8, 12, 8, 1.5, [("speech-models", "Speech models")]),
    S("demo-harbor-barrier", "The harbor council approves a tide barrier", "After a year of hearings, the council voted seven to two to fund a movable barrier across the harbor mouth.",
      "World", 6, 8, 5, 2, [HB]),
    S("demo-even-schools", "Teachers and the district reach a tentative agreement", "", "Local", 6, 6, 3, 2.5),
    S("demo-library-hours", "City libraries extend their evening hours", "Branches in six neighbourhoods will stay open until ten on weekdays.", "Culture", 5, 4, 1, 3),
    S("demo-river-festival", "The river festival returns with a quieter schedule", "Organisers cut the fireworks and added a morning of boat tours.", "Local", 5, 4, 2, 4),
    S("demo-even-1", "Bakers' guild marks two centuries with a public oven day", "", "Culture", 4, 3, 1, 3),
    S("demo-even-2", "Bridge repairs move to nights next month", "", "Local", 4, 3, 1, 4),
    S("demo-even-3", "Shipping rates drift lower for a third week", "", "Business", 5, 3, 1, 5),
    S("demo-even-4", "Insurer pauses new flood policies near the estuary", "", "Business", 5, 4, 2, 5),
    S("demo-even-5", "Museum acquires a collection of hand-lettered signs", "", "Culture", 4, 3, 1, 6),
    S("demo-even-6", "New bus lane opens on the east ring road", "", "Local", 4, 3, 1, 6),
    S("demo-even-op", "Small models, large consequences", "Why the open speech release matters.", "Opinion", 5, 1, 1, 3, [("speech-models", "Speech models")], kind="opinion"),
]

# A thin day: few stories, none clearly first, few fresh: a pair and a river, no lead.
thin = [
    S("demo-library-hours", "City libraries extend their evening hours", "Branches in six neighbourhoods will stay open until ten on weekdays.", "Culture", 5, 4, 1, 15),
    S("demo-river-festival", "The river festival returns with a quieter schedule", "Organisers cut the fireworks and added a morning of boat tours.", "Local", 5, 4, 2, 15.5),
    S("demo-grid-storage", "A regional grid adds its largest battery yet", "The new installation can hold enough power for a small city through an evening peak.", "Tech and AI", 5, 3, 2, 17),
    S("demo-thin-1", "Allotment waiting list falls below a hundred", "", "Local", 4, 3, 1, 19),
    S("demo-thin-2", "Tide tables for the week are published", "", "Local", 4, 3, 1, 20),
]

for name, stories in (("big", big), ("even", even), ("thin", thin)):
    with open(os.path.join(OUT, f"front-{name}.json"), "w") as f:
        json.dump({"stories": stories}, f, indent=1)
        f.write("\n")
print("ok")
