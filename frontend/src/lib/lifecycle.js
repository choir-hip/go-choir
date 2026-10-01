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

// reconnect backs off at the durable cursor. When attempts are exhausted the
// stream is marked dead (onStreamLost) rather than silently abandoned: resume
// events (tab visible again, pageshow, network back online) re-arm a fresh
// reconnect loop so a laptop sleep or network flap does not leave the view
// dead until reload.
let streamDead = false;
let reconnecting = false;
const reconnect = async () => {
  if (reconnecting) return;
  reconnecting = true;
  try {
    for (let attempt = 0; attempt < MAX_RECONNECT_ATTEMPTS && !closed; attempt += 1) {
      await sleep(Math.min(1000 * 2 ** attempt, 15000));
      if (closed) return;
      try {
        await connect(cursor);
        if (streamDead) {
          streamDead = false;
          handlers.onStreamRestored?.();
        }
        return;
      } catch {
        // keep backing off
      }
    }
    if (!closed && !streamDead) {
      streamDead = true;
      handlers.onStreamLost?.(new Error('Lifecycle stream disconnected'));
      handlers.onError?.(new Error('Lifecycle stream disconnected'));
    }
  } finally {
    reconnecting = false;
  }
};

const onResume = () => {
  if (closed) return;
  if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return;
  const esClosed = typeof EventSource !== 'undefined' && stream?.readyState === EventSource.CLOSED;
  if (streamDead || esClosed) void reconnect();
};
if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', onResume);
}
if (typeof window !== 'undefined') {
  window.addEventListener('pageshow', onResume);
  window.addEventListener('online', onResume);
}

let initialConnectError = null;
try {
  await connect(0);
} catch (error) {
  // Initial connect failed before the stream opened. Do NOT close the
  // subscription: keep resume listeners armed so visibility/online events
  // can retry, and mark the stream dead so the error surfaces and the
  // caller still gets a cleanup function. Re-throw only for the caller's
  // immediate feedback — the instance stays reconnectable.
  initialConnectError = error;
  streamDead = true;
  handlers.onStreamLost?.(error);
  void reconnect();
}
return () => {
  closed = true;
  if (stream) stream.close();
  if (typeof document !== 'undefined') {
    document.removeEventListener('visibilitychange', onResume);
  }
  if (typeof window !== 'undefined') {
    window.removeEventListener('pageshow', onResume);
    window.removeEventListener('online', onResume);
  }
};
}
