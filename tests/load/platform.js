import http from 'k6/http'
import { check, fail, sleep } from 'k6'
import { Trend, Rate } from 'k6/metrics'
import exec from 'k6/execution'

const baseURL = __ENV.BASE_URL || 'http://127.0.0.1:8080'
const partnerKey = __ENV.PARTNER_API_KEY || 'demo-secret'
const profile = __ENV.PROFILE || 'smoke'

const quoteDuration = new Trend('order_quote_duration', true)
const createDuration = new Trend('order_create_duration', true)
const businessFailures = new Rate('business_failures')

const sharedThresholds = {
  http_req_failed: ['rate<0.005'],
  business_failures: ['rate<0.005'],
  order_quote_duration: ['p(95)<300'],
  order_create_duration: ['p(95)<500'],
}

const profiles = {
  smoke: {
    scenarios: {
      smoke: { executor: 'shared-iterations', vus: 1, iterations: 5, maxDuration: '30s', exec: 'smokeFlow' },
    },
    thresholds: sharedThresholds,
  },
  average: {
    scenarios: {
      browse: { executor: 'constant-arrival-rate', rate: 5, timeUnit: '1s', duration: '5m', preAllocatedVUs: 10, exec: 'browse' },
      quote: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '1s', duration: '5m', preAllocatedVUs: 5, exec: 'quote' },
      create: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '4s', duration: '5m', preAllocatedVUs: 3, exec: 'create' },
    },
    thresholds: sharedThresholds,
  },
  peak: {
    scenarios: {
      browse: {
        executor: 'ramping-arrival-rate', startRate: 5, timeUnit: '1s', preAllocatedVUs: 30, maxVUs: 100, exec: 'browse',
        stages: [{ target: 25, duration: '1m' }, { target: 50, duration: '2m' }, { target: 5, duration: '1m' }],
      },
      quote: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '1s', duration: '4m', preAllocatedVUs: 8, exec: 'quote' },
      create: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '4s', duration: '4m', preAllocatedVUs: 4, exec: 'create' },
    },
    thresholds: sharedThresholds,
  },
  stress: {
    scenarios: {
      browse: {
        executor: 'ramping-arrival-rate', startRate: 25, timeUnit: '1s', preAllocatedVUs: 100, maxVUs: 300, exec: 'browse',
        stages: [{ target: 100, duration: '2m' }, { target: 200, duration: '3m' }, { target: 0, duration: '1m' }],
      },
      quote: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '1s', duration: '6m', preAllocatedVUs: 10, exec: 'quote' },
      create: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '4s', duration: '6m', preAllocatedVUs: 5, exec: 'create' },
    },
    thresholds: sharedThresholds,
  },
  degraded: {
    scenarios: {
      create: { executor: 'constant-arrival-rate', rate: 1, timeUnit: '4s', duration: '2m', preAllocatedVUs: 4, exec: 'create' },
      browse: { executor: 'constant-arrival-rate', rate: 5, timeUnit: '1s', duration: '2m', preAllocatedVUs: 10, exec: 'browse' },
    },
    thresholds: sharedThresholds,
  },
}

if (!profiles[profile]) {
  throw new Error(`unknown PROFILE ${profile}`)
}
export const options = profiles[profile]

export function setup() {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1000000)}`
  const externalID = `load-${suffix}`
  const menuResponse = http.put(`${baseURL}/partner/v1/menu`, JSON.stringify({
    categories: [{
      externalId: externalID,
      name: 'Load test',
      position: 1,
      products: [{ externalId: externalID, name: 'Load test product', description: '', priceMinor: 10000, quantity: 1000000, available: true }],
    }],
  }), { headers: jsonHeaders({ 'X-API-Key': partnerKey }), tags: { operation: 'load_setup_menu' } })
  requireStatus(menuResponse, 204, 'publish load-test menu')

  const restaurants = http.get(`${baseURL}/api/v1/restaurants`, { tags: { operation: 'load_setup_restaurants' } })
  requireStatus(restaurants, 200, 'load restaurants')
  const restaurant = restaurants.json('items').find((item) => item.id === '11111111-1111-1111-1111-111111111111')
  if (!restaurant) fail('demo restaurant is absent')

  const menu = http.get(`${baseURL}/api/v1/restaurants/${restaurant.id}/menu`, { tags: { operation: 'load_setup_menu_read' } })
  requireStatus(menu, 200, 'load public menu')
  const product = menu.json('categories.0.products.0')
  if (!product || !product.id) fail('load-test product is absent')

  const registration = http.post(`${baseURL}/api/v1/auth/register`, JSON.stringify({
    email: `load-${suffix}@example.test`, password: `Load-${suffix}-password`, name: 'Load Test',
  }), { headers: jsonHeaders(), tags: { operation: 'load_setup_register' } })
  requireStatus(registration, 201, 'register load-test customer')
  const token = registration.json('accessToken')
  if (!token) fail('registration response did not include an access token')

  return { restaurantID: restaurant.id, productID: product.id, token }
}

export function browse(data) {
  const responses = http.batch([
    ['GET', `${baseURL}/api/v1/restaurants`, null, { tags: { operation: 'list_restaurants' } }],
    ['GET', `${baseURL}/api/v1/restaurants/${data.restaurantID}/menu`, null, { tags: { operation: 'get_menu' } }],
  ])
  const ok = check(responses[0], { 'restaurants status is 200': (r) => r.status === 200 }) &&
    check(responses[1], { 'menu status is 200': (r) => r.status === 200 })
  businessFailures.add(!ok)
}

export function quote(data) {
  const response = http.post(`${baseURL}/api/v1/orders/quote`, JSON.stringify(draft(data)), {
    headers: jsonHeaders(), tags: { operation: 'quote_order' },
  })
  quoteDuration.add(response.timings.duration)
  businessFailures.add(!check(response, { 'quote status is 200': (r) => r.status === 200 }))
}

export function create(data) {
  const response = http.post(`${baseURL}/api/v1/orders`, JSON.stringify(draft(data)), {
    headers: jsonHeaders({
      Authorization: `Bearer ${data.token}`,
      'Idempotency-Key': `load-${exec.scenario.name}-${exec.vu.idInTest}-${exec.scenario.iterationInTest}-${Date.now()}`,
    }),
    tags: { operation: 'create_order' },
  })
  createDuration.add(response.timings.duration)
  businessFailures.add(!check(response, { 'create status is 201': (r) => r.status === 201 }))
}

export function smokeFlow(data) {
  browse(data)
  quote(data)
  create(data)
  sleep(0.2)
}

export function handleSummary(data) {
  const metricNames = [
    'http_req_failed', 'business_failures', 'http_req_duration',
    'order_quote_duration', 'order_create_duration', 'http_reqs',
    'iterations', 'dropped_iterations', 'checks',
  ]
  const metrics = {}
  for (const name of metricNames) {
    if (data.metrics[name]) {
      metrics[name] = {
        type: data.metrics[name].type,
        contains: data.metrics[name].contains,
        values: data.metrics[name].values,
        thresholds: data.metrics[name].thresholds,
      }
    }
  }
  const sanitized = { profile, generatedAt: new Date().toISOString(), metrics }
  const lines = [
    `profile=${profile}`,
    `http_failed_rate=${value(metrics, 'http_req_failed', 'rate')}`,
    `business_failed_rate=${value(metrics, 'business_failures', 'rate')}`,
    `quote_p95_ms=${value(metrics, 'order_quote_duration', 'p(95)')}`,
    `create_p95_ms=${value(metrics, 'order_create_duration', 'p(95)')}`,
    `requests=${value(metrics, 'http_reqs', 'count')}`,
  ]
  const output = { stdout: `${lines.join('\n')}\n` }
  if (__ENV.SUMMARY_PATH) {
    output[__ENV.SUMMARY_PATH] = JSON.stringify(sanitized, null, 2)
  }
  return output
}

function draft(data) {
  return { restaurantId: data.restaurantID, deliveryAddress: 'Самара, нагрузочный тест', items: [{ productId: data.productID, quantity: 1 }] }
}

function jsonHeaders(extra = {}) {
  return { 'Content-Type': 'application/json', ...extra }
}

function requireStatus(response, status, action) {
  if (response.status !== status) {
    fail(`${action}: got ${response.status}: ${response.body}`)
  }
}

function value(metrics, metric, field) {
  return metrics[metric] && metrics[metric].values[field] !== undefined ? metrics[metric].values[field] : 'n/a'
}
