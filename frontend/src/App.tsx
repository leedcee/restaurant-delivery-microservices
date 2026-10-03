import { useCallback, useEffect, useMemo, useState } from 'react'
import { createOrder, fetchMenu, fetchOrder, fetchOrders, fetchRestaurants, quoteOrder, streamOrder } from './api'
import { demoMenu, demoRestaurants } from './mock'
import type { Cart, Menu, Order, OrderQuote, Product, Restaurant, Screen } from './types'

const money = (minor: number) => `${new Intl.NumberFormat('ru-RU').format(minor / 100)} ₽`
const CART_KEY = 'looch-cart-v1'

function routeFromPath(): { screen: Screen; id?: string } {
  const parts = window.location.pathname.split('/').filter(Boolean)
  if (parts[0] === 'restaurants' && parts[1]) return { screen: 'menu', id: parts[1] }
  if (parts[0] === 'checkout') return { screen: 'checkout' }
  if (parts[0] === 'orders' && parts[1]) return { screen: 'tracking', id: parts[1] }
  if (parts[0] === 'orders') return { screen: 'orders' }
  return { screen: 'catalog' }
}

function readSavedCart(): { restaurantId?: string; cart: Cart } {
  try {
    const saved = JSON.parse(localStorage.getItem(CART_KEY) ?? '{}')
    return { restaurantId: saved.restaurantId, cart: saved.cart ?? {} }
  } catch { return { cart: {} } }
}

function FoodArt({ kind = 'round', restaurant = false }: { kind?: string; restaurant?: boolean }) {
  return <div className={`food-art food-art--${kind} ${restaurant ? 'food-art--restaurant' : ''}`} aria-hidden="true"><span /><i /><b /></div>
}

function Header({ screen, go }: { screen: Screen; go: (screen: Screen) => void }) {
  return <header className="header shell">
    <button className="brand" onClick={() => go('catalog')} aria-label="На главную"><strong>LOOCH</strong><span>еда рядом</span></button>
    <nav aria-label="Основная навигация">
      <button className={screen === 'catalog' || screen === 'menu' ? 'active' : ''} onClick={() => go('catalog')}>Рестораны</button>
      <button className={screen === 'checkout' || screen === 'orders' || screen === 'tracking' ? 'active' : ''} onClick={() => go('orders')}>Заказы</button>
      <button>Профиль</button>
    </nav><div className="avatar">Л</div>
  </header>
}

function Notice({ children, retry }: { children: React.ReactNode; retry?: () => void }) {
  return <div className="notice"><span>{children}</span>{retry && <button onClick={retry}>Повторить</button>}</div>
}

function Catalog({ restaurants, choose, loading, error, retry }: { restaurants: Restaurant[]; choose: (restaurant: Restaurant) => void; loading: boolean; error: string; retry: () => void }) {
  const [query, setQuery] = useState('')
  const filtered = restaurants.filter((item) => `${item.name} ${item.cuisine}`.toLowerCase().includes(query.toLowerCase()))
  return <main className="shell page catalog-page">
    <section className="hero"><p className="eyebrow">Доставка из ресторанов рядом</p><h1>Что хочется сегодня?</h1><p>Собрали хорошие места и проверили, что у них есть в наличии прямо сейчас.</p><div className="search-row"><label className="search"><span>⌕</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Название или кухня" /></label><button className="button button--coral">Найти</button><button className="button button--outline">Фильтры <b>2</b></button></div></section>
    {error && <Notice retry={retry}>Backend временно недоступен — показываем демонстрационный каталог.</Notice>}
    <div className="section-heading"><div><p className="eyebrow coral">Самара · Московское шоссе</p><h2>{loading ? 'Загружаем рестораны…' : 'Популярно рядом'}</h2></div><button className="text-link">Смотреть все →</button></div>
    <section className="restaurant-grid">{filtered.map((restaurant) => <button className="restaurant-card offset-card" key={restaurant.id} onClick={() => choose(restaurant)}><div className="restaurant-card__art"><FoodArt kind={restaurant.art} restaurant /><span className="rating">★ {restaurant.rating}</span></div><div className="restaurant-card__body"><h3>{restaurant.name}</h3><p>{restaurant.cuisine}</p><span className="eta">{restaurant.eta}</span><span className="card-arrow">→</span></div></button>)}</section>
    {!filtered.length && <div className="empty"><h3>Ничего не нашли</h3><p>Попробуйте изменить запрос.</p></div>}
  </main>
}

function Quantity({ value, add, remove }: { value: number; add: () => void; remove: () => void }) {
  return value ? <div className="quantity"><button onClick={remove}>−</button><b>{value}</b><button onClick={add}>+</button></div> : <button className="add-button" onClick={add}>+</button>
}

function MenuPage({ menu, cart, setCart, checkout, loading, error, retry }: { menu: Menu; cart: Cart; setCart: (cart: Cart) => void; checkout: () => void; loading: boolean; error: string; retry: () => void }) {
  const [category, setCategory] = useState(menu.categories[0]?.id)
  useEffect(() => setCategory(menu.categories[0]?.id), [menu])
  const products = menu.categories.find((item) => item.id === category)?.products ?? menu.categories.flatMap((item) => item.products)
  const allProducts = menu.categories.flatMap((item) => item.products)
  const count = Object.values(cart).reduce((sum, quantity) => sum + quantity, 0)
  const total = allProducts.reduce((sum, product) => sum + product.priceMinor * (cart[product.id] ?? 0), 0)
  const update = (product: Product, delta: number) => setCart({ ...cart, [product.id]: Math.max(0, (cart[product.id] ?? 0) + delta) })
  if (loading) return <main className="shell page"><div className="loading-card"><span /><h2>Загружаем меню</h2><p>Проверяем цены и наличие блюд.</p></div></main>
  return <main className="shell page menu-page"><button className="back" onClick={() => history.back()}>← Все рестораны</button>{error && <Notice retry={retry}>Не удалось получить актуальное меню — доступна демонстрационная версия.</Notice>}<div className="menu-title"><div><h1>{menu.restaurant.name}</h1><p>25–35 минут&nbsp; · &nbsp;рейтинг 4,8</p></div><span className="open-pill">Открыт до 22:00</span></div><div className="tabs" role="tablist">{menu.categories.map((item, index) => <button key={item.id} className={item.id === category ? 'active' : ''} onClick={() => setCategory(item.id)}>{index === 0 ? 'Популярное' : item.name}</button>)}</div><section className="product-grid">{products.map((product, index) => <article className="product-card offset-card" key={product.id}><div className="product-art"><FoodArt kind={product.art ?? ['round', 'square', 'triangle'][index % 3]} /></div><div className="product-body"><h3>{product.name}</h3><p>{product.description}</p><div><strong>{money(product.priceMinor)}</strong><Quantity value={cart[product.id] ?? 0} add={() => update(product, 1)} remove={() => update(product, -1)} /></div></div></article>)}{!products.length && <div className="empty"><h3>Здесь скоро появятся блюда</h3><p>Загляните в раздел «Популярное».</p></div>}</section>{count > 0 && <button className="cart-dock" onClick={checkout}><span>Корзина&nbsp; • &nbsp;{count} {count === 1 ? 'блюдо' : 'блюда'}</span><strong>{money(total)}</strong></button>}</main>
}

function Checkout({ quote, submit, busy, error, quoteLoading, quoteError, retryQuote }: { quote: OrderQuote | null; submit: (address: string) => void; busy: boolean; error: string; quoteLoading: boolean; quoteError: string; retryQuote: () => void }) {
  const [address, setAddress] = useState('Самара, Московское шоссе, 15')
  return <main className="shell page checkout-page"><h1>Оформление заказа</h1><div className="steps"><span className="done"><b>Корзина</b></span><i /><span className="active"><b>Доставка</b></span><i /><span><b>Оплата</b></span></div>{quoteError && <Notice retry={retryQuote}>{quoteError}</Notice>}<div className="checkout-layout"><form className="checkout-form" onSubmit={(event) => { event.preventDefault(); submit(address) }} id="checkout-form"><label>Адрес доставки<input required value={address} onChange={(event) => setAddress(event.target.value)} /></label><label>Время<select defaultValue="asap"><option value="asap">Как можно скорее · 25–35 минут</option><option value="later">Выбрать время</option></select></label><label>Оплата<select defaultValue="card"><option value="card">Картой онлайн ···· 4821</option><option value="cash">При получении</option></select></label><label>Комментарий<textarea placeholder="Код домофона, ориентир" rows={2} /></label></form><aside className="order-summary"><h2>Ваш заказ</h2>{quoteLoading && <p className="summary-loading">Проверяем цены и остатки…</p>}<div className="summary-lines">{quote?.items.map((item) => <p key={item.productId}><span>{item.name} × {item.quantity}</span><b>{money(item.totalMinor)}</b></p>)}{quote && <p><span>Доставка</span><b>{money(quote.deliveryFeeMinor)}</b></p>}</div><div className="summary-total"><span>Итого</span><strong>{quote ? money(quote.totalMinor) : '—'}</strong></div>{error && <p className="form-error">{error}</p>}<button className="button button--coral summary-button" form="checkout-form" disabled={busy || quoteLoading || !quote}>{busy ? 'Оформляем…' : 'Оформить заказ'}</button><small>Нажимая кнопку, вы соглашаетесь с условиями сервиса</small></aside></div></main>
}

const statusIndex: Record<string, number> = { pending: 0, accepted: 0, preparing: 1, ready: 1, delivering: 2, delivered: 3 }
const statusLabel: Record<string, string> = { pending: 'Ожидает подтверждения', accepted: 'Принят', preparing: 'Готовится', ready: 'Готов к выдаче', delivering: 'У курьера', delivered: 'Доставлен', rejected: 'Отклонён', cancelled: 'Отменён' }

function OrdersPage({ orders, loading, error, retry, open }: { orders: Order[]; loading: boolean; error: string; retry: () => void; open: (order: Order) => void }) {
  return <main className="shell page orders-page"><div className="section-heading orders-heading"><div><p className="eyebrow coral">Личный кабинет</p><h1>Мои заказы</h1></div></div>{error && <Notice retry={retry}>{error}</Notice>}{loading ? <div className="loading-card"><span /><h2>Загружаем заказы</h2></div> : <section className="orders-list">{orders.map((item) => <button key={item.id} className="order-card" onClick={() => open(item)}><div><small>{item.createdAt ? new Date(item.createdAt).toLocaleString('ru-RU') : 'Текущий заказ'}</small><h3>Заказ № {item.id.slice(0, 5).toUpperCase()}</h3><p>{item.items.map((product) => `${product.name} × ${product.quantity}`).join(', ') || item.deliveryAddress}</p></div><div><span className={`order-status order-status--${item.status}`}>{statusLabel[item.status] ?? item.status}</span><strong>{money(item.totalMinor)}</strong></div></button>)}{!orders.length && !error && <div className="empty"><h3>Заказов пока нет</h3><p>Выберите ресторан и соберите первую корзину.</p></div>}</section>}</main>
}

function Tracking({ order, streamError, retry }: { order: Order; streamError: string; retry: () => void }) {
  const active = statusIndex[order.status] ?? 0
  const shortId = order.id.startsWith('demo-') ? '18452' : order.id.slice(0, 5).toUpperCase()
  return <main className="shell page tracking-page"><div className="tracking-heading"><div><p className="eyebrow coral">{statusLabel[order.status] ?? 'Заказ подтверждён'}</p><h1>Заказ № {shortId}</h1></div><span className="delivery-time">Доставим к 19:35</span></div>{streamError && <Notice retry={retry}>{streamError}</Notice>}<div className="status-track">{['Принят', 'Готовится', 'У курьера', 'Доставлен'].map((label, index) => <div className={index <= active ? 'done' : ''} key={label}><span>{index < active ? '✓' : ''}</span><b>{label}</b></div>)}</div><div className="tracking-layout"><div className="map-card"><div className="street-lines" /><svg viewBox="0 0 700 360" preserveAspectRatio="none" aria-label="Маршрут курьера"><polyline points="65,310 240,105 610,58" /><circle cx="65" cy="310" r="15" className="map-start" /><circle cx="610" cy="58" r="18" className="map-courier" /></svg><span className="map-label">Курьер Алексей</span></div><aside className="courier-card"><span className="courier-icon">А</span><div><h2>{order.status === 'delivered' ? 'Заказ доставлен' : `Курьер ${active >= 2 ? 'в пути' : 'скоро заберёт заказ'}`}</h2><p>Алексей&nbsp; · &nbsp;рейтинг 4,9</p></div><hr /><p className="muted">Адрес</p><strong>{order.deliveryAddress}</strong><p className="muted">Осталось примерно</p><b className="minutes">{order.status === 'delivered' ? 'Готово' : '12 минут'}</b><button className="button button--outline">Связаться с курьером</button></aside></div></main>
}

export default function App() {
  const initialRoute = useMemo(routeFromPath, [])
  const savedCart = useMemo(readSavedCart, [])
  const [screen, setScreen] = useState<Screen>(initialRoute.screen)
  const [routeId, setRouteId] = useState(initialRoute.id)
  const [restaurants, setRestaurants] = useState(demoRestaurants)
  const [selected, setSelected] = useState<Restaurant>(() => demoRestaurants.find((item) => item.id === savedCart.restaurantId) ?? demoRestaurants[0])
  const [menu, setMenu] = useState<Menu>(() => demoMenu(selected))
  const [cart, setCart] = useState<Cart>(savedCart.cart)
  const [order, setOrder] = useState<Order | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [quote, setQuote] = useState<OrderQuote | null>(null)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState({ catalog: true, menu: false, orders: false, quote: false })
  const [errors, setErrors] = useState({ catalog: '', menu: '', orders: '', quote: '', submit: '', stream: '' })
  const [catalogRetry, setCatalogRetry] = useState(0)
  const [ordersRetry, setOrdersRetry] = useState(0)
  const [quoteRetry, setQuoteRetry] = useState(0)
  const [streamRetry, setStreamRetry] = useState(0)

  const navigate = useCallback((path: string) => { history.pushState({}, '', path); const next = routeFromPath(); setScreen(next.screen); setRouteId(next.id); window.scrollTo(0, 0) }, [])
  const go = (next: Screen) => navigate(next === 'catalog' ? '/' : next === 'orders' ? '/orders' : next === 'checkout' ? '/checkout' : '/')
  useEffect(() => { const listener = () => { const next = routeFromPath(); setScreen(next.screen); setRouteId(next.id) }; addEventListener('popstate', listener); return () => removeEventListener('popstate', listener) }, [])
  useEffect(() => localStorage.setItem(CART_KEY, JSON.stringify({ restaurantId: selected.id, cart })), [selected.id, cart])

  useEffect(() => {
    setLoading((value) => ({ ...value, catalog: true })); setErrors((value) => ({ ...value, catalog: '' }))
    fetchRestaurants().then((live) => { if (!live.length) return; const merged = demoRestaurants.map((item, index) => index === 0 ? { ...item, id: live[0].id, isOpen: live[0].isOpen } : item); setRestaurants(merged); setSelected((current) => current.id === demoRestaurants[0].id || current.id === live[0].id ? merged[0] : current) }).catch(() => setErrors((value) => ({ ...value, catalog: 'Сервис ресторанов не отвечает.' }))).finally(() => setLoading((value) => ({ ...value, catalog: false })))
  }, [catalogRetry])

  const loadMenu = useCallback(async (restaurant: Restaurant) => {
    setLoading((value) => ({ ...value, menu: true })); setErrors((value) => ({ ...value, menu: '' }))
    if (restaurant.id !== restaurants[0].id) { setMenu(demoMenu(restaurant)); setLoading((value) => ({ ...value, menu: false })); return }
    try { const live = await fetchMenu(restaurant.id); const decorated = live.categories.flatMap((item) => item.products).map((product, index) => ({ ...product, art: ['round', 'square', 'triangle'][index % 3] as Product['art'] })); setMenu({ ...live, restaurant: { ...restaurant, ...live.restaurant }, categories: [{ id: 'all', name: 'Популярное', products: decorated }, ...live.categories.map((item) => ({ ...item, products: [] }))] }) }
    catch { setMenu(demoMenu(restaurant)); setErrors((value) => ({ ...value, menu: 'Актуальное меню недоступно.' })) }
    finally { setLoading((value) => ({ ...value, menu: false })) }
  }, [restaurants])

  useEffect(() => { if (screen !== 'menu' || !routeId) return; const restaurant = restaurants.find((item) => item.id === routeId) ?? selected; setSelected(restaurant); void loadMenu(restaurant) }, [screen, routeId, restaurants, loadMenu])
  useEffect(() => { if (screen === 'checkout') void loadMenu(restaurants.find((item) => item.id === selected.id) ?? selected) }, [screen, selected.id, restaurants, loadMenu])
  useEffect(() => {
    if (screen !== 'checkout' || !Object.values(cart).some(Boolean)) return
    setLoading((value) => ({ ...value, quote: true })); setErrors((value) => ({ ...value, quote: '' }))
    const products = menu.categories.flatMap((item) => item.products); const isDemo = products.some((item) => item.id.startsWith('aaaaaaaa'))
    if (isDemo) { const items = products.filter((item) => cart[item.id]).map((item) => ({ productId: item.id, name: item.name, quantity: cart[item.id], unitPriceMinor: item.priceMinor, totalMinor: item.priceMinor * cart[item.id] })); const deliveryFeeMinor = 13000; setQuote({ restaurantId: selected.id, items, deliveryFeeMinor, totalMinor: items.reduce((sum, item) => sum + item.totalMinor, deliveryFeeMinor), currency: 'RUB' }); setLoading((value) => ({ ...value, quote: false })); return }
    quoteOrder(selected.id, cart).then(setQuote).catch((reason) => { setQuote(null); setErrors((value) => ({ ...value, quote: reason instanceof Error ? reason.message : 'Не удалось рассчитать заказ.' })) }).finally(() => setLoading((value) => ({ ...value, quote: false })))
  }, [screen, cart, menu, selected.id, quoteRetry])

  useEffect(() => {
    if (screen !== 'orders') return
    setLoading((value) => ({ ...value, orders: true })); setErrors((value) => ({ ...value, orders: '' }))
    fetchOrders().then((items) => setOrders(order && !items.some((item) => item.id === order.id) ? [order, ...items] : items)).catch(() => { if (order) setOrders([order]); else setErrors((value) => ({ ...value, orders: 'Не удалось загрузить историю заказов.' })) }).finally(() => setLoading((value) => ({ ...value, orders: false })))
  }, [screen, ordersRetry, order])

  useEffect(() => {
    if (screen !== 'tracking' || !routeId || routeId.startsWith('demo-')) return
    const controller = new AbortController(); setErrors((value) => ({ ...value, stream: '' }))
    const connect = async () => {
      try {
        setOrder(await fetchOrder(routeId))
        await streamOrder(routeId, setOrder, controller.signal)
      } catch {
        if (!controller.signal.aborted) setErrors((value) => ({ ...value, stream: 'Поток статусов прерван.' }))
      }
    }
    void connect()
    return () => controller.abort()
  }, [screen, routeId, streamRetry])

  const choose = (restaurant: Restaurant) => { if (selected.id !== restaurant.id) setCart({}); setSelected(restaurant); navigate(`/restaurants/${restaurant.id}`) }
  const submit = async (address: string) => {
    setBusy(true); setErrors((value) => ({ ...value, submit: '' }))
    try { const products = menu.categories.flatMap((item) => item.products); const isDemo = products.some((item) => item.id.startsWith('aaaaaaaa')); const created: Order = isDemo ? { id: `demo-${Date.now()}`, status: 'delivering', deliveryAddress: address, currency: 'RUB', items: quote?.items ?? [], deliveryFeeMinor: quote?.deliveryFeeMinor ?? 13000, totalMinor: quote?.totalMinor ?? 0, createdAt: new Date().toISOString() } : await createOrder(selected.id, cart, address); setOrder(created); setOrders((items) => [created, ...items.filter((item) => item.id !== created.id)]); setCart({}); navigate(`/orders/${created.id}`) }
    catch (reason) { setErrors((value) => ({ ...value, submit: reason instanceof Error ? reason.message : 'Не удалось оформить заказ' })) }
    finally { setBusy(false) }
  }

  return <><Header screen={screen} go={go} />{screen === 'catalog' && <Catalog restaurants={restaurants} choose={choose} loading={loading.catalog} error={errors.catalog} retry={() => setCatalogRetry((value) => value + 1)} />}{screen === 'menu' && <MenuPage menu={menu} cart={cart} setCart={setCart} checkout={() => go('checkout')} loading={loading.menu} error={errors.menu} retry={() => void loadMenu(selected)} />}{screen === 'checkout' && <Checkout quote={quote} submit={submit} busy={busy} error={errors.submit} quoteLoading={loading.quote} quoteError={errors.quote} retryQuote={() => setQuoteRetry((value) => value + 1)} />}{screen === 'orders' && <OrdersPage orders={orders} loading={loading.orders} error={errors.orders} retry={() => setOrdersRetry((value) => value + 1)} open={(item) => { setOrder(item); navigate(`/orders/${item.id}`) }} />}{screen === 'tracking' && order && <Tracking order={order} streamError={errors.stream} retry={() => setStreamRetry((value) => value + 1)} />}{screen === 'tracking' && !order && <main className="shell page"><div className="loading-card"><span /><h2>Загружаем заказ</h2></div></main>}<footer className="footer shell"><strong>LOOCH</strong><span>Еда рядом, когда она нужна.</span><small>Web-клиент платформы доставки</small></footer></>
}
