import { fetchWithRenewal } from './auth.js';

const DURABLE_WORK_SCHEMA = 'choir.durable_work.v1';

function requireDurableWorkSchema(value, label) {
  if (value?.schema !== DURABLE_WORK_SCHEMA) {
    throw new Error(`${label} returned unsupported schema`);
  }
  return value;
}



export async function getLifecycleEvents(trajectoryId, options = {}) {
  if (!trajectoryId) throw new Error('Lifecycle trajectory ID is required')
  const after = Number.isSafeInteger(options.after) && options.after >= 0 ? options.after : 0
  const limit = Number.isSafeInteger(options.limit) && options.limit > 0 ? options.limit : 100
  const response = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}/events?after=${after}&limit=${limit}`)
  if (!response.ok) {
    const error = await response.json().catch(() => ({}))
    throw new Error(error.reason || error.error || `Lifecycle events failed (${response.status})`)
  }
  return requireDurableWorkSchema(await response.json(), 'Lifecycle events');
}





export async function getLifecycleSnapshot(trajectoryId) {
  if (!trajectoryId) {
    throw new Error('Trajectory ID is required');
  }
  const response = await fetchWithRenewal(`/api/trajectories/${encodeURIComponent(trajectoryId)}`, { method: 'GET' });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.reason || error.error || `Lifecycle snapshot failed (${response.status})`);
  }
  return requireDurableWorkSchema(await response.json(), 'Lifecycle snapshot');
}

// observeLifecycle opens the event stream before fetching the snapshot, then
// discards buffered events covered by the snapshot cursor and delivers the
// remainder in reducer order. Overflow and expired cursors force replay.
// Post-snapshot stream errors reconnect at the last cursor (?after=cursor)
// with bounded exponential backoff; exhaustion surfaces onError.
const MAX_RECONNECT_ATTEMPTS = 8;

export async function observeLifecycle(trajectoryId, handlers = {}) {
  if (!trajectoryId) throw new Error('Trajectory ID is required');
  let cursor = 0;
  let closed = false;
  let stream = null;

  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  const attach = (es) => {
    let ready = false;
    const buffer = [];
    es.addEventListener('lifecycle', (message) => {
      try {
        const event = JSON.parse(message.data);
        requireDurableWorkSchema(event, 'Lifecycle stream event');
        if (!ready) {
          buffer.push(event);
          if (buffer.length > 1000) {
            es.close();
            closed = true;
            handlers.onReplayRequired?.({ reason: 'buffer_overflow' });
          }
          return;
        }
        if (event.reducer_seq > cursor) {
          cursor = event.reducer_seq;
          handlers.onEvent?.(event);
        }
      } catch (error) {
        handlers.onError?.(error);
      }
    });
    es.addEventListener('replay_required', (message) => {
      es.close();
      closed = true;
      handlers.onReplayRequired?.(JSON.parse(message.data));
    });
    return {
      markReady() {
        ready = true;
        buffer.sort((left, right) => left.reducer_seq - right.reducer_seq);
        for (const event of buffer) {
          if (event.reducer_seq > cursor) {
            cursor = event.reducer_seq;
            handlers.onEvent?.(event);
          }
        }
      },
    };
  };

  const connect = async (after) => {
    const es = new EventSource(`/api/trajectories/${encodeURIComponent(trajectoryId)}/stream?after=${after}`, { withCredentials: true });
    stream = es;
    const gate = attach(es);
    try {
      await new Promise((resolve, reject) => {
        es.onopen = resolve;
        es.onerror = () => reject(new Error('Lifecycle stream failed to open'));
      });
      const snapshot = await getLifecycleSnapshot(trajectoryId);
      if (snapshot.snapshot_cursor > cursor) cursor = snapshot.snapshot_cursor;
      gate.markReady();
      handlers.onSnapshot?.(snapshot);
    } catch (error) {
      es.close();
      throw error;
    }
    // Post-open errors arm the reconnect loop at the last durable cursor.
    es.onerror = () => {
      es.close();
      if (!closed) void reconnect();
    };
  };

  const reconnect = async () => {
    for (let attempt = 0; attempt < MAX_RECONNECT_ATTEMPTS && !closed; attempt += 1) {
      await sleep(Math.min(1000 * 2 ** attempt, 15000));
      if (closed) return;
      try {
        await connect(cursor);
        return;
      } catch {
        // keep backing off
      }
    }
    if (!closed) handlers.onError?.(new Error('Lifecycle stream disconnected'));
  };

  try {
    await connect(0);
  } catch (error) {
    closed = true;
    if (stream) stream.close();
    throw error;
  }
  return () => {
    closed = true;
    if (stream) stream.close();
  };
}
