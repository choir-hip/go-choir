import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = (__ENV.CHOIR_BASE_URL || 'https://draft.choir-ip.com').replace(/\/$/, '');

export const options = {
  scenarios: {
    public_stochastic: {
      executor: 'constant-vus',
      vus: Number(__ENV.CHOIR_K6_VUS || 12),
      duration: __ENV.CHOIR_K6_DURATION || '5m',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.02'],
    'http_req_duration{surface:public_root}': ['p(95)<1500'],
    'http_req_duration{surface:auth_session}': ['p(95)<1000'],
  },
};

function jitter(maxMs) {
  return Math.random() * (maxMs / 1000);
}

export default function () {
  const roll = Math.random();
  if (roll < 0.7) {
    const root = http.get(`${BASE_URL}/`, { tags: { surface: 'public_root' } });
    check(root, { 'public root available': (res) => res.status >= 200 && res.status < 400 });
  } else {
    const session = http.get(`${BASE_URL}/auth/session`, { tags: { surface: 'auth_session' } });
    check(session, {
      'signed-out session endpoint available': (res) => res.status === 200 && String(res.body || '').includes('authenticated'),
    });
  }
  sleep(jitter(Number(__ENV.CHOIR_K6_MAX_JITTER_MS || 3000)));
}
