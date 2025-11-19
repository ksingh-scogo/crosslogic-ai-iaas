import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 100 }, // Ramp up to 100 users
    { duration: '1m', target: 1000 }, // Ramp up to 1000 users (target load)
    { duration: '30s', target: 1000 }, // Stay at 1000 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must complete below 500ms
    errors: ['rate<0.01'],            // Error rate must be less than 1%
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY || 'test-key';

export default function () {
  const payload = JSON.stringify({
    model: 'llama-3-8b',
    messages: [
      { role: 'user', content: 'Hello, world!' }
    ],
    max_tokens: 50
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${API_KEY}`,
    },
  };

  const res = http.post(`${BASE_URL}/v1/chat/completions`, payload, params);

  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'status is 429 (rate limit)': (r) => r.status === 429, // Accept rate limits as valid behavior under load
  });

  if (!success && res.status !== 429) {
    errorRate.add(1);
  }

  sleep(1);
}
