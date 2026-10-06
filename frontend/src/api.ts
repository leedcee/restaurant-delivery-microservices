import type { Address, Cart, Menu, Order, OrderQuote, Restaurant } from './types'
import { authorizedFetch } from './auth'

async function json<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const problem = await response.json().catch(() => null)
    throw new Error(problem?.detail || `HTTP ${response.status}`)
  }
  return response.json() as Promise<T>
}

export async function fetchRestaurants(): Promise<Restaurant[]> {
  const response = await fetch('/api/v1/restaurants')
  return (await json<{ items: Restaurant[] }>(response)).items
}

export async function fetchMenu(restaurantId: string): Promise<Menu> {
  return json<Menu>(await fetch(`/api/v1/restaurants/${restaurantId}/menu`))
}

function draft(restaurantId: string, cart: Cart, deliveryAddress?: string) {
  return {
    restaurantId,
    items: Object.entries(cart).filter(([, quantity]) => quantity > 0).map(([productId, quantity]) => ({ productId, quantity })),
    ...(deliveryAddress ? { deliveryAddress } : {}),
  }
}

export async function createOrder(restaurantId: string, cart: Cart, deliveryAddress: string): Promise<Order> {
  const response = await authorizedFetch('/api/v1/orders', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': crypto.randomUUID(),
    },
    body: JSON.stringify(draft(restaurantId, cart, deliveryAddress)),
  })
  return json<Order>(response)
}

export async function quoteOrder(restaurantId: string, cart: Cart): Promise<OrderQuote> {
  const response = await fetch('/api/v1/orders/quote', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(draft(restaurantId, cart)),
  })
  return json<OrderQuote>(response)
}

export async function fetchOrders(): Promise<Order[]> {
  const response = await authorizedFetch('/api/v1/orders')
  return (await json<{ items: Order[] }>(response)).items
}

export async function fetchOrder(orderId: string): Promise<Order> {
  return json<Order>(await authorizedFetch(`/api/v1/orders/${orderId}`))
}

export async function fetchAddresses(): Promise<Address[]> {
  return (await json<{ items: Address[] }>(await authorizedFetch('/api/v1/addresses'))).items
}

export async function createAddress(input: { label: string; address: string; isDefault?: boolean }): Promise<Address> {
  return json<Address>(await authorizedFetch('/api/v1/addresses', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) }))
}

export async function updateAddress(id: string, input: { label?: string; address?: string; isDefault?: boolean }): Promise<Address> {
  return json<Address>(await authorizedFetch(`/api/v1/addresses/${id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) }))
}

export async function deleteAddress(id: string): Promise<void> {
  const response = await authorizedFetch(`/api/v1/addresses/${id}`, { method: 'DELETE' })
  if (!response.ok) await json(response)
}

export async function fetchPartnerOrders(apiKey: string): Promise<Order[]> {
  const response = await fetch('/partner/v1/orders', { headers: { 'X-API-Key': apiKey } })
  return (await json<{ items: Order[] }>(response)).items
}

export async function updatePartnerOrderStatus(apiKey: string, orderId: string, status: string, reason?: string): Promise<Order> {
  return json<Order>(await fetch(`/partner/v1/orders/${orderId}/status`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json', 'X-API-Key': apiKey },
    body: JSON.stringify({ status, ...(reason ? { reason } : {}) }),
  }))
}

export async function streamOrder(orderId: string, onOrder: (order: Order) => void, signal: AbortSignal): Promise<void> {
  const response = await authorizedFetch(`/api/v1/orders/${orderId}/events`, { signal })
  if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`)
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { value, done } = await reader.read()
    if (done) return
    buffer += decoder.decode(value, { stream: true })
    const events = buffer.split('\n\n')
    buffer = events.pop() ?? ''
    for (const event of events) {
      const data = event.split('\n').find((line) => line.startsWith('data: '))
      if (data) onOrder(JSON.parse(data.slice(6)) as Order)
    }
  }
}
