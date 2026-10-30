// Reads a trajectory's run events through served product routes:
// GET /api/runs?trajectory_id= lists the trajectory's runs (including
// Management lifecycle-control runs bound by assignment_trajectory_id), and
// GET /api/runs/{id}/events pages each run's events in seq order. These
// replace the /api/trace/trajectories/{id}/moments/* reads, which no route
// ever served (docs/archive/definitions/choir-reeng-cut0-eyes-2026-10-12.md).
//
// fetchJSON is the calling spec's own (page, path) => body helper, so the
// spec keeps its auth and error handling.

export async function trajectoryRunEvents(fetchJSON, page, trajectoryId) {
  const list = await fetchJSON(
    page,
    `/api/runs?trajectory_id=${encodeURIComponent(trajectoryId)}&limit=500`
  );
  const events = [];
  for (const run of list.runs || []) {
    let after = 0;
    for (let pages = 0; pages < 1000; pages += 1) {
      const body = await fetchJSON(
        page,
        `/api/runs/${encodeURIComponent(run.run_id)}/events?after=${after}&limit=1000`
      );
      events.push(...(body.events || []));
      if (!body.next_after) break;
      after = body.next_after;
    }
  }
  return events;
}

// Every tool.result run event of the trajectory whose payload names the tool
// (or every tool.result event when toolName is omitted).
export async function trajectoryToolResultEvents(fetchJSON, page, trajectoryId, toolName) {
  const events = await trajectoryRunEvents(fetchJSON, page, trajectoryId);
  return events.filter((event) => {
    if (event.kind !== 'tool.result') return false;
    if (!toolName) return true;
    const payload = typeof event.payload === 'string' ? safeParse(event.payload) : (event.payload || {});
    return payload.tool === toolName;
  });
}

function safeParse(raw) {
  try {
    return JSON.parse(raw);
  } catch {
    return {};
  }
}
