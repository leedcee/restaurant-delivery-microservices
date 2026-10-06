import { expect, test } from '@playwright/test'

const restaurantId = '11111111-1111-1111-1111-111111111111'
const productId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1'
const orderId = '99999999-9999-9999-9999-999999999999'
const addressId = '77777777-7777-7777-7777-777777777777'
const user = { id: '55555555-5555-5555-5555-555555555555', name: 'Лидси', email: 'leedcee@example.com', createdAt: '2026-10-06T09:00:00Z' }
const session = { user, accessToken: 'access-token', refreshToken: 'refresh-token', expiresIn: 900 }

test.beforeEach(async ({ page }) => {
  let addresses: Array<{ id: string; userId: string; label: string; address: string; isDefault: boolean; createdAt: string; updatedAt: string }> = []
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
      return route.fulfill({ json: { items: [{ id: restaurantId, name: 'Тёплый хлеб', description: 'Завтраки', isOpen: true }] } })
    }
    if (path.endsWith('/menu')) {
      return route.fulfill({ json: { restaurant: { id: restaurantId, name: 'Тёплый хлеб', description: 'Завтраки', isOpen: true }, categories: [{ id: 'popular', name: 'Популярное', products: [{ id: productId, name: 'Паста с курицей', description: 'Фирменный соус', priceMinor: 49000, currency: 'RUB', available: true, quantity: 20 }] }] } })
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
