export type Screen = 'catalog' | 'menu' | 'checkout' | 'tracking'

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
  status: string
  deliveryAddress: string
  items: QuoteItem[]
  totalMinor: number
  currency: string
}

export type Cart = Record<string, number>
