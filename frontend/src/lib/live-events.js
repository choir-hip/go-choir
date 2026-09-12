let deviceId = '';
let sessionId = '';
let driverLeaseUntil = 0;
const DRIVER_LEASE_MS = 60_000;

export function currentDeviceId() {
  if (!deviceId) {
    deviceId = globalThis.crypto?.randomUUID?.() || `device-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
  return deviceId;
}

export function currentSessionId() {
  if (!sessionId) {
    sessionId = globalThis.crypto?.randomUUID?.() || `session-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
  return sessionId;
}

export function currentViewportProfile() {
  if (typeof window === 'undefined') return 'server';
  const width = Number(window.innerWidth || 0);
  if (width > 0 && width < 768) return 'compact';
  return 'desktop';
}

export function renewDriverLease() {
  currentSessionId();
  driverLeaseUntil = Date.now() + DRIVER_LEASE_MS;
}

export function isDrivingSession() {
  return Date.now() < driverLeaseUntil;
}

export function observeRemoteDriverSession(remoteSessionId = '') {
  const normalized = String(remoteSessionId || '').trim();
  if (normalized && normalized !== currentSessionId()) {
    driverLeaseUntil = 0;
  }
}

export function dispatchLiveEvent(message) {
  if (typeof window === 'undefined' || !message) return;
  window.dispatchEvent(new CustomEvent('choir-live-event', { detail: message }));
}

export function addLiveEventListener(handler) {
  if (typeof window === 'undefined' || typeof handler !== 'function') return () => {};
  const wrapped = (event) => handler(event.detail || {});
  window.addEventListener('choir-live-event', wrapped);
  return () => window.removeEventListener('choir-live-event', wrapped);
}

export function liveEventKind(message) {
  return String(message?.kind || '');
}

export function liveEventPayload(message) {
  return message?.payload && typeof message.payload === 'object' ? message.payload : {};
}

export function isOwnLiveEvent(message) {
  const payload = liveEventPayload(message);
  if (payload.source_session_id) {
    return payload.source_session_id === currentSessionId();
  }
  return payload.source_device_id === currentDeviceId();
}
