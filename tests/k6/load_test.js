import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('error_rate');

export const options = {
  stages: [
    { duration: '30s', target: 100 }, // Ramp up to 100 users
    { duration: '1m', target: 1000 }, // Ramp up to 1000 users (stress test)
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must complete below 500ms
    error_rate: ['rate<0.01'],        // Error rate must be less than 1%
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'sk-test-key';

export default function () {
  const payload = JSON.stringify({
    model: 'llama-3-8b',
    messages: [
      { role: 'user', content: 'Hello, world!' }
    ],
    stream: false,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${API_KEY}`,
    },
  };

  const res = http.post(`${BASE_URL}/v1/chat/completions`, payload, params);

  // Check for success
  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'response has usage': (r) => r.json('usage') !== undefined,
  });

  // Check for rate limiting
  check(res, {
    'rate limited': (r) => r.status === 429,
  });

  if (!success && res.status !== 429) {
    errorRate.add(1);
  }

  sleep(1);
}

