import type { Cart, Menu, Order, Restaurant } from './types'

const USER_ID = '55555555-5555-5555-5555-555555555555'

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
  const response = await fetch('/api/v1/orders', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-User-ID': USER_ID,
      'Idempotency-Key': crypto.randomUUID(),
    },
    body: JSON.stringify(draft(restaurantId, cart, deliveryAddress)),
  })
  return json<Order>(response)
}

export async function fetchOrder(orderId: string): Promise<Order> {
  return json<Order>(await fetch(`/api/v1/orders/${orderId}`, { headers: { 'X-User-ID': USER_ID } }))
}
