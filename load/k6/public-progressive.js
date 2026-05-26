import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = (__ENV.CHOIR_BASE_URL || 'https://choir.news').replace(/\/$/, '');

export const options = {
  scenarios: {
    public_progressive: {
      executor: 'ramping-arrival-rate',
      startRate: Number(__ENV.CHOIR_K6_START_RATE || 1),
      timeUnit: '1s',
      preAllocatedVUs: Number(__ENV.CHOIR_K6_PREALLOCATED_VUS || 20),
      maxVUs: Number(__ENV.CHOIR_K6_MAX_VUS || 80),
      stages: [
        { target: Number(__ENV.CHOIR_K6_RAMP_TARGET || 5), duration: __ENV.CHOIR_K6_RAMP_DURATION || '2m' },
        { target: Number(__ENV.CHOIR_K6_RAMP_TARGET || 5), duration: __ENV.CHOIR_K6_HOLD_DURATION || '2m' },
        { target: 0, duration: __ENV.CHOIR_K6_COOLDOWN_DURATION || '30s' },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{surface:public_root}': ['p(95)<1200'],
    'http_req_duration{surface:health}': ['p(95)<800'],
  },
};

export default function () {
  const root = http.get(`${BASE_URL}/`, { tags: { surface: 'public_root' } });
  check(root, {
    'public root is ok': (res) => res.status >= 200 && res.status < 400,
    'public root has html': (res) => String(res.headers['Content-Type'] || '').includes('text/html'),
  });

  const health = http.get(`${BASE_URL}/health`, { tags: { surface: 'health' } });
  check(health, {
    'health is ok or degraded JSON': (res) => [200].includes(res.status) && String(res.headers['Content-Type'] || '').includes('application/json'),
  });

  sleep(Math.random() * 0.4);
}
