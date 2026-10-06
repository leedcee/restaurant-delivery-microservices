export type Screen = 'catalog' | 'menu' | 'checkout' | 'orders' | 'tracking' | 'auth' | 'profile' | 'partnerLogin' | 'partnerDashboard'

export interface Restaurant {
  id: string
  name: string
  description: string
  isOpen: boolean
  cuisine?: string
  eta?: string
  rating?: string
  art?: 'bread' | 'pasta' | 'sushi'
}

export interface Product {
  id: string
  name: string
  description: string
  priceMinor: number
  currency: string
  available: boolean
  quantity: number
  art?: 'round' | 'square' | 'triangle'
}

export interface MenuCategory {
  id: string
  name: string
  products: Product[]
}

export interface Menu {
  restaurant: Restaurant
  categories: MenuCategory[]
}

export interface QuoteItem {
  productId: string
  name: string
  quantity: number
  unitPriceMinor: number
  totalMinor: number
}

export interface Order {
  id: string
  userId?: string
  restaurantId?: string
  status: string
  deliveryAddress: string
  items: QuoteItem[]
  deliveryFeeMinor: number
  totalMinor: number
  currency: string
  createdAt?: string
}

export interface OrderQuote {
  restaurantId: string
  items: QuoteItem[]
  deliveryFeeMinor: number
  totalMinor: number
  currency: string
}

export type Cart = Record<string, number>

export interface User {
  id: string
  email: string
  name: string
  createdAt: string
}

export interface AuthSession {
  user: User
  accessToken: string
  refreshToken: string
  expiresIn: number
}

export interface Address {
  id: string
  userId: string
  label: string
  address: string
  isDefault: boolean
  createdAt: string
  updatedAt: string
}
