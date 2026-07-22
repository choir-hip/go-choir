import { fetchWithRenewal } from './auth.js';

const DURABLE_WORK_SCHEMA = 'choir.durable_work.v1';

function requireDurableWorkSchema(value, label) {
  if (value?.schema !== DURABLE_WORK_SCHEMA) {
    throw new Error(`${label} returned unsupported schema`);
  }
  return value;
}

async function lifecycleRequest(path, request) {
  if (!request?.command_id) {
    throw new Error('Lifecycle command ID is required');
  }
  if (!request.command_digest) {
    throw new Error('Lifecycle command digest is required');
  }
  const response = await fetchWithRenewal(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  if (!response.ok) {
    const error = await response.json().catch(() => ({}));
    throw new Error(error.reason || error.error || `Lifecycle request failed (${response.status})`);
  }
  return requireDurableWorkSchema(await response.json(), 'Lifecycle command');
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

export function openLifecycleWork(request) {
  return lifecycleRequest('/api/lifecycle/work/open', request);
}

export function amendLifecycleWork(request) {
  return lifecycleRequest('/api/lifecycle/work/amend', request);
}

export function recordLifecycleRefs(request) {
  return lifecycleRequest('/api/lifecycle/refs/record', request);
}

export function queueLifecycleUpdate(request) {
  return lifecycleRequest('/api/lifecycle/updates/queue', request);
}

export function applyLifecycleUpdate(request) {
  return lifecycleRequest('/api/lifecycle/updates/apply', request);
}

export function settleLifecycleWork(request) {
  return lifecycleRequest('/api/lifecycle/work/settle', request);
}

export function refuseLifecycleWork(request) {
  return lifecycleRequest('/api/lifecycle/work/refuse', request);
}

export function settleLifecycleTrajectory(request) {
  return lifecycleRequest('/api/lifecycle/trajectories/settle', request);
}

export function cancelLifecycleTrajectory(request) {
  return lifecycleRequest('/api/lifecycle/trajectories/cancel', request);
}

export function archiveLifecycleArtifact(request) {
  return lifecycleRequest('/api/lifecycle/artifacts/archive', request);
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
export async function observeLifecycle(trajectoryId, handlers = {}) {
  if (!trajectoryId) throw new Error('Trajectory ID is required');
  const buffer = [];
  let snapshotReady = false;
  let cursor = 0;
  let closed = false;
  const stream = new EventSource(`/api/trajectories/${encodeURIComponent(trajectoryId)}/stream?after=0`, { withCredentials: true });
  const opened = new Promise((resolve, reject) => {
    stream.onopen = resolve;
    stream.onerror = () => {
      if (!snapshotReady) reject(new Error('Lifecycle stream failed before snapshot'));
      else handlers.onError?.(new Error('Lifecycle stream disconnected'));
    };
  });
  stream.addEventListener('lifecycle', (message) => {
    try {
      const event = JSON.parse(message.data);
      requireDurableWorkSchema(event, 'Lifecycle stream event');
      if (!snapshotReady) {
        buffer.push(event);
        if (buffer.length > 1000) {
          stream.close();
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
  stream.addEventListener('replay_required', (message) => {
    stream.close();
    closed = true;
    handlers.onReplayRequired?.(JSON.parse(message.data));
  });
  try {
    await opened;
    const snapshot = await getLifecycleSnapshot(trajectoryId);
    cursor = snapshot.snapshot_cursor || 0;
    snapshotReady = true;
    handlers.onSnapshot?.(snapshot);
    buffer.sort((left, right) => left.reducer_seq - right.reducer_seq);
    for (const event of buffer) {
      if (event.reducer_seq > cursor) {
        cursor = event.reducer_seq;
        handlers.onEvent?.(event);
      }
    }
  } catch (error) {
    stream.close();
    closed = true;
    throw error;
  }
  return () => {
    if (!closed) stream.close();
  };
}
