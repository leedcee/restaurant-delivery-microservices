import { expect, test } from '@playwright/test'

const restaurantId = '11111111-1111-1111-1111-111111111111'
const pastaRestaurantId = '22222222-2222-2222-2222-222222222222'
const sushiRestaurantId = '33333333-3333-3333-3333-333333333333'
const productId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1'
const orderId = '99999999-9999-9999-9999-999999999999'
const addressId = '77777777-7777-7777-7777-777777777777'
const user = { id: '55555555-5555-5555-5555-555555555555', name: 'Лидси', email: 'leedcee@example.com', createdAt: '2026-10-06T09:00:00Z' }
const session = { user, accessToken: 'access-token', refreshToken: 'refresh-token', expiresIn: 900 }

test.beforeEach(async ({ page }) => {
  let addresses: Array<{ id: string; userId: string; label: string; address: string; isDefault: boolean; createdAt: string; updatedAt: string }> = []
  let partnerMenu = { categories: [{ externalId: 'popular', name: 'Популярное', position: 1, products: [{ externalId: 'chicken-pasta', name: 'Паста с курицей', description: 'Фирменный соус', priceMinor: 49000, quantity: 20, available: true }] }] }
  let partnerOrder = { id: orderId, userId: user.id, restaurantId, status: 'pending', deliveryAddress: 'Самара, Московское шоссе, 15', items: [{ productId, name: 'Паста с курицей', quantity: 1, unitPriceMinor: 49000, totalMinor: 49000 }], deliveryFeeMinor: 13000, totalMinor: 62000, currency: 'RUB', createdAt: new Date().toISOString() }
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    if (path === '/api/v1/auth/register') {
      return route.fulfill({ status: 201, json: session })
    }
    if (path === '/api/v1/auth/login') {
      return route.fulfill({ json: session })
    }
    if (path === '/api/v1/addresses' && request.method() === 'GET') {
      return route.fulfill({ json: { items: addresses } })
    }
    if (path === '/api/v1/addresses' && request.method() === 'POST') {
      const input = request.postDataJSON() as { label: string; address: string }
      const created = { id: addressId, userId: user.id, ...input, isDefault: addresses.length === 0, createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
      addresses = [created, ...addresses]
      return route.fulfill({ status: 201, json: created })
    }
    if (path === `/api/v1/addresses/${addressId}` && request.method() === 'PATCH') {
      addresses = addresses.map((item) => ({ ...item, isDefault: item.id === addressId }))
      return route.fulfill({ json: addresses.find((item) => item.id === addressId) })
    }
    if (path === `/api/v1/addresses/${addressId}` && request.method() === 'DELETE') {
      addresses = addresses.filter((item) => item.id !== addressId)
      return route.fulfill({ status: 204 })
    }
    if (path === '/api/v1/restaurants') {
      return route.fulfill({ json: { items: [
        { id: restaurantId, name: 'Тёплый хлеб', description: 'Завтраки', isOpen: true, cuisine: 'Европейская · Завтраки', eta: '25–35 минут', rating: '4.8', art: 'bread' },
        { id: pastaRestaurantId, name: 'Паста Лаб', description: 'Свежая паста', isOpen: false, cuisine: 'Итальянская · Паста', eta: '30–40 минут', rating: '4.7', art: 'pasta' },
        { id: sushiRestaurantId, name: 'Рис и рыба', description: 'Роллы и поке', isOpen: true, cuisine: 'Японская · Суши', eta: '35–45 минут', rating: '4.9', art: 'sushi' },
      ] } })
    }
    if (path.endsWith('/menu')) {
      return route.fulfill({ json: { restaurant: { id: restaurantId, name: 'Тёплый хлеб', description: 'Завтраки', isOpen: true, cuisine: 'Европейская · Завтраки', eta: '25–35 минут', rating: '4.8', art: 'bread' }, categories: [{ id: 'popular', name: 'Популярное', products: [{ id: productId, name: 'Паста с курицей', description: 'Фирменный соус', priceMinor: 49000, currency: 'RUB', available: true, quantity: 20 }] }] } })
    }
    if (path === '/api/v1/orders/quote') {
      return route.fulfill({ json: { restaurantId, items: [{ productId, name: 'Паста с курицей', quantity: 1, unitPriceMinor: 49000, totalMinor: 49000 }], deliveryFeeMinor: 13000, totalMinor: 62000, currency: 'RUB' } })
    }
    if (path === '/api/v1/orders' && request.method() === 'POST') {
      return route.fulfill({ status: 201, json: { id: orderId, userId: '55555555-5555-5555-5555-555555555555', restaurantId, status: 'pending', deliveryAddress: 'Самара, Московское шоссе, 15', items: [{ productId, name: 'Паста с курицей', quantity: 1, unitPriceMinor: 49000, totalMinor: 49000 }], deliveryFeeMinor: 13000, totalMinor: 62000, currency: 'RUB', createdAt: new Date().toISOString() } })
    }
    if (path.endsWith('/events')) {
      const delivered = { id: orderId, status: 'delivered', deliveryAddress: 'Самара, Московское шоссе, 15', items: [], deliveryFeeMinor: 13000, totalMinor: 62000, currency: 'RUB' }
      return route.fulfill({ contentType: 'text/event-stream', body: `event: order\ndata: ${JSON.stringify(delivered)}\n\n` })
    }
    if (path === `/api/v1/orders/${orderId}`) {
      return route.fulfill({ json: { id: orderId, status: 'pending', deliveryAddress: 'Самара, Московское шоссе, 15', items: [], deliveryFeeMinor: 13000, totalMinor: 62000, currency: 'RUB' } })
    }
    return route.fulfill({ json: { items: [] } })
  })
  await page.route('**/partner/v1/**', async (route) => {
    const request = route.request()
    if (request.headers()['x-api-key'] !== 'demo-secret') return route.fulfill({ status: 401, json: { title: 'Unauthorized' } })
    const path = new URL(request.url()).pathname
    if (path === '/partner/v1/menu' && request.method() === 'GET') return route.fulfill({ json: partnerMenu })
    if (path === '/partner/v1/menu' && request.method() === 'PUT') {
      partnerMenu = request.postDataJSON() as typeof partnerMenu
      return route.fulfill({ status: 204 })
    }
    if (path === '/partner/v1/orders/events' && request.method() === 'GET') return route.fulfill({ contentType: 'text/event-stream', body: `event: orders\ndata: ${JSON.stringify({ items: [partnerOrder] })}\n\n` })
    if (path === '/partner/v1/orders' && request.method() === 'GET') return route.fulfill({ json: { items: [partnerOrder] } })
    if (path === `/partner/v1/orders/${orderId}/status` && request.method() === 'PATCH') {
      const input = request.postDataJSON() as { status: string }
      partnerOrder = { ...partnerOrder, status: input.status }
      return route.fulfill({ json: partnerOrder })
    }
    return route.fulfill({ status: 404, json: { title: 'Not found' } })
  })
})

test('customer completes the order journey and receives a live status', async ({ page }) => {
  await page.addInitScript((authSession) => localStorage.setItem('looch-auth-v1', JSON.stringify(authSession)), session)
  await page.goto('/')
  await page.getByRole('button', { name: /Тёплый хлеб/ }).click()
  await expect(page).toHaveURL(`/restaurants/${restaurantId}`)
  await page.locator('.add-button').first().click()
  await page.getByRole('button', { name: /Корзина/ }).click()
  await expect(page).toHaveURL('/checkout')
  await expect(page.getByText('620 ₽')).toBeVisible()
  await page.getByRole('button', { name: 'Оформить заказ' }).click()
  await expect(page).toHaveURL(`/orders/${orderId}`)
  await expect(page.getByText('Заказ доставлен')).toBeVisible()
})

test('customer registers and opens the profile', async ({ page }) => {
  await page.goto('/login')
  await page.getByRole('button', { name: 'Регистрация' }).click()
  await page.getByLabel('Имя').fill('Лидси')
  await page.getByLabel('Электронная почта').fill('leedcee@example.com')
  await page.getByLabel('Пароль').fill('strong-password')
  await page.getByRole('button', { name: 'Создать аккаунт' }).click()
  await expect(page).toHaveURL('/profile')
  await expect(page.getByRole('heading', { name: 'Лидси' })).toBeVisible()
  await expect(page.getByText('leedcee@example.com')).toBeVisible()
})

test('private order history redirects a guest to login', async ({ page }) => {
  await page.goto('/orders')
  await expect(page).toHaveURL('/login?return=%2Forders')
  await expect(page.getByRole('heading', { name: 'С возвращением' })).toBeVisible()
})

test('customer saves a delivery address in the profile', async ({ page }) => {
  await page.addInitScript((authSession) => localStorage.setItem('looch-auth-v1', JSON.stringify(authSession)), session)
  await page.goto('/profile')
  await page.getByLabel('Новый адрес').fill('Самара, Ново-Садовая улица, 106')
  await page.getByRole('button', { name: 'Сохранить адрес' }).click()
  await expect(page.getByText('Самара, Ново-Садовая улица, 106')).toBeVisible()
  await expect(page.getByText('Основной', { exact: true })).toBeVisible()
})

test('cart survives page reload', async ({ page }) => {
  await page.goto(`/restaurants/${restaurantId}`)
  await page.locator('.add-button').first().click()
  await page.reload()
  await expect(page.getByRole('button', { name: /Корзина.*1 блюдо/ })).toBeVisible()
})

test('catalog does not invent restaurants when API is unavailable', async ({ page }) => {
  await page.unroute('**/api/v1/**')
  await page.route('**/api/v1/restaurants', (route) => route.fulfill({ status: 503, json: { title: 'Unavailable' } }))
  await page.goto('/')
  await expect(page.getByText(/Сервис ресторанов не отвечает/)).toBeVisible()
  await expect(page.getByRole('button', { name: /Тёплый хлеб/ })).toHaveCount(0)
})

test('customer searches and filters the restaurant catalog', async ({ page }) => {
  await page.goto('/')
  await expect(page.locator('.restaurant-card')).toHaveCount(3)
  await page.getByRole('button', { name: /^Фильтры/ }).click()
  await page.getByRole('button', { name: 'Только открытые' }).click()
  await expect(page.getByRole('button', { name: /Паста Лаб/ })).toHaveCount(0)
  await expect(page.locator('.restaurant-card')).toHaveCount(2)
  await page.getByRole('button', { name: 'Только открытые' }).click()
  await page.getByPlaceholder('Название или кухня').fill('итальянская')
  await page.getByRole('button', { name: 'Найти' }).click()
  await expect(page.getByRole('button', { name: /Паста Лаб/ })).toBeVisible()
  await expect(page.locator('.restaurant-card')).toHaveCount(1)
})

test('restaurant partner signs in and accepts an order', async ({ page }) => {
  await page.goto('/partner')
  await page.getByLabel('API-ключ').fill('demo-secret')
  await page.getByRole('button', { name: 'Открыть очередь' }).click()
  await expect(page).toHaveURL('/partner/orders')
  await expect(page.getByRole('heading', { name: /Заказ №/ })).toBeVisible()
  await page.getByRole('button', { name: 'Принять' }).click()
  await expect(page.getByText('Принят', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Начать готовить' })).toBeVisible()
})
test('restaurant partner receives a new order through realtime stream', async ({ page }) => {
  const realtimeOrder = { id: '88888888-8888-8888-8888-888888888888', userId: user.id, restaurantId, status: 'pending', deliveryAddress: 'Самара, улица Нового заказа, 8', items: [{ productId, name: 'Паста с курицей', quantity: 2, unitPriceMinor: 49000, totalMinor: 98000 }], deliveryFeeMinor: 13000, totalMinor: 111000, currency: 'RUB', createdAt: new Date().toISOString() }
  await page.route('**/partner/v1/orders/events', (route) => route.fulfill({
    contentType: 'text/event-stream',
    body: `event: orders\ndata: ${JSON.stringify({ items: [realtimeOrder] })}\n\n`,
  }))
  await page.goto('/partner')
  await page.getByLabel('API-ключ').fill('demo-secret')
  await page.getByRole('button', { name: 'Открыть очередь' }).click()
  await expect(page).toHaveURL('/partner/orders')
  await expect(page.getByText('Самара, улица Нового заказа, 8')).toBeVisible()
})
test('restaurant partner edits prices, stock and stop-list', async ({ page }) => {
  await page.goto('/partner')
  await page.getByLabel('API-ключ').fill('demo-secret')
  await page.getByRole('button', { name: 'Открыть очередь' }).click()
  await page.getByRole('button', { name: 'Меню' }).click()
  await expect(page).toHaveURL('/partner/menu')
  await page.getByLabel('Цена блюда 1-1').fill('555')
  await page.getByLabel('Остаток блюда 1-1').fill('12')
  await page.getByLabel('Доступность блюда 1-1').uncheck()
  await page.getByRole('button', { name: '+ Добавить блюдо' }).click()
  await page.getByLabel('Название блюда 1-2').fill('Новая позиция')
  await page.getByLabel('Цена блюда 1-2').fill('320')
  await page.getByLabel('Остаток блюда 1-2').fill('8')
  await page.getByLabel('Доступность блюда 1-2').check()
  await page.getByRole('button', { name: 'Сохранить меню' }).click()
  await expect(page.getByRole('status')).toContainText('Меню сохранено')
  await expect(page.getByText('В стоп-листе')).toBeVisible()
  await page.getByRole('button', { name: 'Заказы' }).click()
  await expect(page).toHaveURL('/partner/orders')
})
