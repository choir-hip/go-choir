import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = (__ENV.CHOIR_BASE_URL || 'https://choir.news').replace(/\/$/, '');
const AUTH_STATE = __ENV.CHOIR_AUTH_STATE || '';

function cookiesFromStorageState(path) {
  if (!path) {
    throw new Error('CHOIR_AUTH_STATE is required for authenticated bootstrap load');
  }
  const state = JSON.parse(open(path));
  const host = new URL(BASE_URL).hostname;
  return (state.cookies || [])
    .filter((cookie) => {
      const domain = String(cookie.domain || '').replace(/^\./, '');
      return domain === host || host.endsWith(`.${domain}`);
    })
    .map((cookie) => `${cookie.name}=${cookie.value}`)
    .join('; ');
}

const cookieHeader = cookiesFromStorageState(AUTH_STATE);

export const options = {
  scenarios: {
    authenticated_bootstrap_progressive: {
      executor: 'ramping-arrival-rate',
      startRate: Number(__ENV.CHOIR_K6_AUTH_START_RATE || 1),
      timeUnit: '1s',
      preAllocatedVUs: Number(__ENV.CHOIR_K6_AUTH_PREALLOCATED_VUS || 8),
      maxVUs: Number(__ENV.CHOIR_K6_AUTH_MAX_VUS || 30),
      stages: [
        { target: Number(__ENV.CHOIR_K6_AUTH_RAMP_TARGET || 2), duration: __ENV.CHOIR_K6_AUTH_RAMP_DURATION || '90s' },
        { target: Number(__ENV.CHOIR_K6_AUTH_RAMP_TARGET || 2), duration: __ENV.CHOIR_K6_AUTH_HOLD_DURATION || '2m' },
        { target: 0, duration: __ENV.CHOIR_K6_AUTH_COOLDOWN_DURATION || '30s' },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.02'],
    'http_req_duration{surface:bootstrap}': ['p(95)<5000'],
  },
};

export default function () {
  const res = http.get(`${BASE_URL}/api/shell/bootstrap`, {
    headers: {
      Cookie: cookieHeader,
      'X-Choir-Load-Scenario': 'authenticated-bootstrap-progressive',
    },
    tags: { surface: 'bootstrap' },
  });
  check(res, {
    'bootstrap authenticated': (r) => r.status !== 401,
    'bootstrap ok': (r) => r.status >= 200 && r.status < 300,
    'bootstrap has sandbox id': (r) => {
      try {
        return Boolean(JSON.parse(r.body).sandbox_id);
      } catch (_err) {
        return false;
      }
    },
  });
  sleep(Math.random() * 0.5);
}
