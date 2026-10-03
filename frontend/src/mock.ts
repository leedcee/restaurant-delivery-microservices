import type { Menu, Product, Restaurant } from './types'

export const demoRestaurants: Restaurant[] = [
  { id: '11111111-1111-1111-1111-111111111111', name: 'Тёплый хлеб', description: 'Завтраки и домашняя кухня', isOpen: true, cuisine: 'Европейская · Завтраки', eta: '25–35 минут', rating: '4,8', art: 'bread' },
  { id: '22222222-2222-2222-2222-222222222222', name: 'Паста и соус', description: 'Свежая паста каждый день', isOpen: true, cuisine: 'Итальянская · Паста', eta: '30–40 минут', rating: '4,7', art: 'pasta' },
  { id: '33333333-3333-3333-3333-333333333333', name: 'Рис и рыба', description: 'Роллы и горячие блюда', isOpen: true, cuisine: 'Азиатская · Суши', eta: '35–45 минут', rating: '4,9', art: 'sushi' },
]

export const demoProducts: Product[] = [
  { id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1', name: 'Паста с курицей', description: 'Свежие продукты и фирменный соус', priceMinor: 49000, currency: 'RUB', available: true, quantity: 20, art: 'round' },
  { id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2', name: 'Томлёная говядина', description: 'Свежие продукты и фирменный соус', priceMinor: 62000, currency: 'RUB', available: true, quantity: 15, art: 'square' },
  { id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3', name: 'Сырники', description: 'Свежие продукты и фирменный соус', priceMinor: 35000, currency: 'RUB', available: true, quantity: 30, art: 'triangle' },
]

export function demoMenu(restaurant = demoRestaurants[0]): Menu {
  return {
    restaurant,
    categories: [
      { id: 'popular', name: 'Популярное', products: demoProducts },
      { id: 'breakfast', name: 'Завтраки', products: [] },
      { id: 'hot', name: 'Горячее', products: [] },
      { id: 'drinks', name: 'Напитки', products: [] },
    ],
  }
}
