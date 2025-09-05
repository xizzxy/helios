// k6 Load Test for Helios Gateway
// Usage: k6 run k6-basic.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 20 },   // Warm up
    { duration: '1m', target: 100 },   // Ramp up
    { duration: '2m', target: 500 },   // Stay at 500 RPS
    { duration: '1m', target: 1000 },  // Peak load
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'], // 95% < 100ms, 99% < 200ms
    http_req_failed: ['rate<0.01'],                // Error rate < 1%
    errors: ['rate<0.05'],                         // Custom error rate < 5%
  },
};

// Test scenarios
const scenarios = {
  // Basic rate limiting test
  basic: {
    weight: 70,
    exec: 'basicRateLimitTest',
  },
  // High-frequency single tenant
  burst: {
    weight: 20,
    exec: 'burstTest',
  },
  // Multi-tenant test
  multiTenant: {
    weight: 10,
    exec: 'multiTenantTest',
  },
};

// Base URL configuration
const BASE_URL = __ENV.HELIOS_URL || 'http://localhost:8080';

// Test data
const tenants = ['acme', 'globex', 'initech', 'umbrella', 'cyberdyne'];
const resources = ['api', 'upload', 'download', 'search'];

export function basicRateLimitTest() {
  const tenant = tenants[Math.floor(Math.random() * tenants.length)];
  const resource = resources[Math.floor(Math.random() * resources.length)];
  
  const params = {
    headers: {
      'X-API-Key': `test-key-${tenant}`,
      'Content-Type': 'application/json',
    },
    timeout: '10s',
  };

  const url = `${BASE_URL}/allow?tenant=${tenant}&resource=${resource}&cost=1`;
  const response = http.get(url, params);

  const success = check(response, {
    'status is 200 or 429': (r) => r.status === 200 || r.status === 429,
    'response time < 100ms': (r) => r.timings.duration < 100,
    'has rate limit headers': (r) => 
      r.headers['X-Ratelimit-Limit'] !== undefined &&
      r.headers['X-Ratelimit-Remaining'] !== undefined,
  });

  errorRate.add(!success);

  // Brief pause to simulate realistic usage
  sleep(0.1);
}

export function burstTest() {
  const tenant = 'burst-test';
  const params = {
    headers: {
      'X-API-Key': `test-key-${tenant}`,
    },
    timeout: '5s',
  };

  // Send burst of requests
  for (let i = 0; i < 10; i++) {
    const url = `${BASE_URL}/allow?tenant=${tenant}&cost=5`;
    const response = http.get(url, params);

    check(response, {
      'burst request handled': (r) => r.status === 200 || r.status === 429,
      'response time < 50ms': (r) => r.timings.duration < 50,
    });
  }

  sleep(0.5);
}

export function multiTenantTest() {
  // Test isolation between tenants
  const tenant1 = 'tenant-a';
  const tenant2 = 'tenant-b';
  
  const params1 = {
    headers: { 'X-API-Key': `key-${tenant1}` },
    timeout: '5s',
  };
  
  const params2 = {
    headers: { 'X-API-Key': `key-${tenant2}` },
    timeout: '5s',
  };

  // Exhaust tenant1's limit
  for (let i = 0; i < 50; i++) {
    http.get(`${BASE_URL}/allow?tenant=${tenant1}&cost=2`, params1);
  }

  // tenant2 should still be available
  const response2 = http.get(`${BASE_URL}/allow?tenant=${tenant2}&cost=1`, params2);
  
  check(response2, {
    'tenant isolation works': (r) => r.status === 200,
  });

  sleep(0.2);
}

// Setup function (runs once)
export function setup() {
  console.log('Starting Helios load test...');
  console.log(`Target URL: ${BASE_URL}`);
  
  // Health check
  const healthResponse = http.get(`${BASE_URL}/health`);
  if (healthResponse.status !== 200) {
    throw new Error(`Health check failed: ${healthResponse.status}`);
  }
  
  console.log('Health check passed, starting load test');
  return { healthOk: true };
}

// Teardown function (runs once after all iterations)
export function teardown(data) {
  console.log('Load test completed');
  
  // Final health check
  const healthResponse = http.get(`${BASE_URL}/health`);
  console.log(`Final health check: ${healthResponse.status}`);
}