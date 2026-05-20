let deviceId = '';

export function currentDeviceId() {
  if (!deviceId) {
    deviceId = globalThis.crypto?.randomUUID?.() || `device-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }
  return deviceId;
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
  return liveEventPayload(message).source_device_id === currentDeviceId();
}
