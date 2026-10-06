import { useCallback, useEffect, useMemo, useState } from 'react'
import { createAddress, createOrder, deleteAddress, fetchAddresses, fetchMenu, fetchOrder, fetchOrders, fetchRestaurants, quoteOrder, streamOrder, updateAddress } from './api'
import { getSession, login, register, saveSession, subscribeToSession } from './auth'
import type { Address, AuthSession, Cart, Menu, Order, OrderQuote, Product, Restaurant, Screen } from './types'

const money = (minor: number) => `${new Intl.NumberFormat('ru-RU').format(minor / 100)} ₽`
const CART_KEY = 'looch-cart-v1'

function routeFromPath(): { screen: Screen; id?: string } {
  const parts = window.location.pathname.split('/').filter(Boolean)
  if (parts[0] === 'restaurants' && parts[1]) return { screen: 'menu', id: parts[1] }
  if (parts[0] === 'checkout') return { screen: 'checkout' }
  if (parts[0] === 'orders' && parts[1]) return { screen: 'tracking', id: parts[1] }
  if (parts[0] === 'orders') return { screen: 'orders' }
  if (parts[0] === 'login') return { screen: 'auth' }
  if (parts[0] === 'profile') return { screen: 'profile' }
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

function Header({ screen, session, go }: { screen: Screen; session: AuthSession | null; go: (screen: Screen) => void }) {
  return <header className="header shell">
    <button className="brand" onClick={() => go('catalog')} aria-label="На главную"><strong>LOOCH</strong><span>еда рядом</span></button>
    <nav aria-label="Основная навигация">
      <button className={screen === 'catalog' || screen === 'menu' ? 'active' : ''} onClick={() => go('catalog')}>Рестораны</button>
      <button className={screen === 'checkout' || screen === 'orders' || screen === 'tracking' ? 'active' : ''} onClick={() => go('orders')}>Заказы</button>
      <button className={screen === 'profile' || screen === 'auth' ? 'active' : ''} onClick={() => go('profile')}>Профиль</button>
    </nav><button className="avatar" onClick={() => go('profile')} aria-label={session ? `Профиль ${session.user.name}` : 'Войти'}>{session?.user.name.trim().charAt(0).toUpperCase() || '→'}</button>
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
    {error && <Notice retry={retry}>{error}</Notice>}
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
  return <main className="shell page menu-page"><button className="back" onClick={() => history.back()}>← Все рестораны</button>{error && <Notice retry={retry}>{error}</Notice>}<div className="menu-title"><div><h1>{menu.restaurant.name}</h1><p>{menu.restaurant.eta}&nbsp; · &nbsp;рейтинг {menu.restaurant.rating}</p></div><span className="open-pill">{menu.restaurant.isOpen ? 'Открыт' : 'Закрыт'}</span></div><div className="tabs" role="tablist">{menu.categories.map((item) => <button key={item.id} className={item.id === category ? 'active' : ''} onClick={() => setCategory(item.id)}>{item.name}</button>)}</div><section className="product-grid">{products.map((product, index) => <article className="product-card offset-card" key={product.id}><div className="product-art"><FoodArt kind={product.art ?? ['round', 'square', 'triangle'][index % 3]} /></div><div className="product-body"><h3>{product.name}</h3><p>{product.description}</p><div><strong>{money(product.priceMinor)}</strong><Quantity value={cart[product.id] ?? 0} add={() => update(product, 1)} remove={() => update(product, -1)} /></div></div></article>)}{!products.length && <div className="empty"><h3>Меню пока пусто</h3><p>Ресторан ещё не опубликовал доступные блюда.</p></div>}</section>{count > 0 && <button className="cart-dock" onClick={checkout}><span>Корзина&nbsp; • &nbsp;{count} {count === 1 ? 'блюдо' : 'блюда'}</span><strong>{money(total)}</strong></button>}</main>
}

function Checkout({ quote, addresses, submit, busy, error, quoteLoading, quoteError, retryQuote }: { quote: OrderQuote | null; addresses: Address[]; submit: (address: string) => void; busy: boolean; error: string; quoteLoading: boolean; quoteError: string; retryQuote: () => void }) {
  const defaultAddress = addresses.find((item) => item.isDefault)?.address ?? addresses[0]?.address ?? 'Самара, Московское шоссе, 15'
  const [address, setAddress] = useState(defaultAddress)
  useEffect(() => { if (addresses.length) setAddress(defaultAddress) }, [defaultAddress, addresses.length])
  return <main className="shell page checkout-page"><h1>Оформление заказа</h1><div className="steps"><span className="done"><b>Корзина</b></span><i /><span className="active"><b>Доставка</b></span><i /><span><b>Оплата</b></span></div>{quoteError && <Notice retry={retryQuote}>{quoteError}</Notice>}<div className="checkout-layout"><form className="checkout-form" onSubmit={(event) => { event.preventDefault(); submit(address) }} id="checkout-form">{addresses.length > 0 && <label>Сохранённый адрес<select value={addresses.some((item) => item.address === address) ? address : ''} onChange={(event) => event.target.value && setAddress(event.target.value)}><option value="">Другой адрес</option>{addresses.map((item) => <option key={item.id} value={item.address}>{item.label}{item.isDefault ? ' · основной' : ''}</option>)}</select></label>}<label>Адрес доставки<input required value={address} onChange={(event) => setAddress(event.target.value)} /></label><label>Время<select defaultValue="asap"><option value="asap">Как можно скорее · 25–35 минут</option><option value="later">Выбрать время</option></select></label><label>Оплата<select defaultValue="card"><option value="card">Картой онлайн ···· 4821</option><option value="cash">При получении</option></select></label><label>Комментарий<textarea placeholder="Код домофона, ориентир" rows={2} /></label></form><aside className="order-summary"><h2>Ваш заказ</h2>{quoteLoading && <p className="summary-loading">Проверяем цены и остатки…</p>}<div className="summary-lines">{quote?.items.map((item) => <p key={item.productId}><span>{item.name} × {item.quantity}</span><b>{money(item.totalMinor)}</b></p>)}{quote && <p><span>Доставка</span><b>{money(quote.deliveryFeeMinor)}</b></p>}</div><div className="summary-total"><span>Итого</span><strong>{quote ? money(quote.totalMinor) : '—'}</strong></div>{error && <p className="form-error">{error}</p>}<button className="button button--coral summary-button" form="checkout-form" disabled={busy || quoteLoading || !quote}>{busy ? 'Оформляем…' : 'Оформить заказ'}</button><small>Нажимая кнопку, вы соглашаетесь с условиями сервиса</small></aside></div></main>
}

const statusIndex: Record<string, number> = { pending: 0, accepted: 0, preparing: 1, ready: 1, delivering: 2, delivered: 3 }
const statusLabel: Record<string, string> = { pending: 'Ожидает подтверждения', accepted: 'Принят', preparing: 'Готовится', ready: 'Готов к выдаче', delivering: 'У курьера', delivered: 'Доставлен', rejected: 'Отклонён', cancelled: 'Отменён' }

function OrdersPage({ orders, loading, error, retry, open }: { orders: Order[]; loading: boolean; error: string; retry: () => void; open: (order: Order) => void }) {
  return <main className="shell page orders-page"><div className="section-heading orders-heading"><div><p className="eyebrow coral">Личный кабинет</p><h1>Мои заказы</h1></div></div>{error && <Notice retry={retry}>{error}</Notice>}{loading ? <div className="loading-card"><span /><h2>Загружаем заказы</h2></div> : <section className="orders-list">{orders.map((item) => <button key={item.id} className="order-card" onClick={() => open(item)}><div><small>{item.createdAt ? new Date(item.createdAt).toLocaleString('ru-RU') : 'Текущий заказ'}</small><h3>Заказ № {item.id.slice(0, 5).toUpperCase()}</h3><p>{item.items.map((product) => `${product.name} × ${product.quantity}`).join(', ') || item.deliveryAddress}</p></div><div><span className={`order-status order-status--${item.status}`}>{statusLabel[item.status] ?? item.status}</span><strong>{money(item.totalMinor)}</strong></div></button>)}{!orders.length && !error && <div className="empty"><h3>Заказов пока нет</h3><p>Выберите ресторан и соберите первую корзину.</p></div>}</section>}</main>
}

function Tracking({ order, streamError, retry }: { order: Order; streamError: string; retry: () => void }) {
  const active = statusIndex[order.status] ?? 0
  const shortId = order.id.slice(0, 5).toUpperCase()
  return <main className="shell page tracking-page"><div className="tracking-heading"><div><p className="eyebrow coral">{statusLabel[order.status] ?? 'Заказ подтверждён'}</p><h1>Заказ № {shortId}</h1></div><span className="delivery-time">Доставим к 19:35</span></div>{streamError && <Notice retry={retry}>{streamError}</Notice>}<div className="status-track">{['Принят', 'Готовится', 'У курьера', 'Доставлен'].map((label, index) => <div className={index <= active ? 'done' : ''} key={label}><span>{index < active ? '✓' : ''}</span><b>{label}</b></div>)}</div><div className="tracking-layout"><div className="map-card"><div className="street-lines" /><svg viewBox="0 0 700 360" preserveAspectRatio="none" aria-label="Маршрут курьера"><polyline points="65,310 240,105 610,58" /><circle cx="65" cy="310" r="15" className="map-start" /><circle cx="610" cy="58" r="18" className="map-courier" /></svg><span className="map-label">Курьер Алексей</span></div><aside className="courier-card"><span className="courier-icon">А</span><div><h2>{order.status === 'delivered' ? 'Заказ доставлен' : `Курьер ${active >= 2 ? 'в пути' : 'скоро заберёт заказ'}`}</h2><p>Алексей&nbsp; · &nbsp;рейтинг 4,9</p></div><hr /><p className="muted">Адрес</p><strong>{order.deliveryAddress}</strong><p className="muted">Осталось примерно</p><b className="minutes">{order.status === 'delivered' ? 'Готово' : '12 минут'}</b><button className="button button--outline">Связаться с курьером</button></aside></div></main>
}

function AuthPage({ complete }: { complete: (session: AuthSession) => void }) {
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const submit = async (event: React.FormEvent) => {
    event.preventDefault(); setBusy(true); setError('')
    try { complete(mode === 'register' ? await register({ name, email, password }) : await login({ email, password })) }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Не удалось войти') }
    finally { setBusy(false) }
  }
  return <main className="shell page auth-page"><section className="auth-copy"><p className="eyebrow coral">Личный кабинет</p><h1>{mode === 'login' ? 'С возвращением' : 'Начнём знакомство'}</h1><p>Войдите, чтобы оформлять заказы, отслеживать доставку и хранить историю в одном месте.</p><div className="auth-points"><span><b>01</b> Заказы защищены вашим аккаунтом</span><span><b>02</b> Сессия обновляется автоматически</span><span><b>03</b> История доступна на любом устройстве</span></div></section><form className="auth-card offset-card" onSubmit={submit}><div className="auth-switch" role="tablist"><button type="button" className={mode === 'login' ? 'active' : ''} onClick={() => { setMode('login'); setError('') }}>Вход</button><button type="button" className={mode === 'register' ? 'active' : ''} onClick={() => { setMode('register'); setError('') }}>Регистрация</button></div>{mode === 'register' && <label>Имя<input name="name" autoComplete="name" minLength={2} required value={name} onChange={(event) => setName(event.target.value)} placeholder="Как к вам обращаться" /></label>}<label>Электронная почта<input name="email" type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} placeholder="name@example.com" /></label><label>Пароль<input name="password" type="password" autoComplete={mode === 'login' ? 'current-password' : 'new-password'} minLength={8} required value={password} onChange={(event) => setPassword(event.target.value)} placeholder="Минимум 8 символов" /></label>{error && <p className="auth-error" role="alert">{error}</p>}<button className="button button--coral auth-submit" disabled={busy}>{busy ? 'Подождите…' : mode === 'login' ? 'Войти' : 'Создать аккаунт'}</button><small>Продолжая, вы соглашаетесь с условиями сервиса.</small></form></main>
}

function ProfilePage({ session, orders, addresses, addressError, openOrders, addAddress, makeDefault, removeAddress, logout }: { session: AuthSession; orders: Order[]; addresses: Address[]; addressError: string; openOrders: () => void; addAddress: (label: string, address: string) => Promise<void>; makeDefault: (id: string) => Promise<void>; removeAddress: (id: string) => Promise<void>; logout: () => void }) {
  const [label, setLabel] = useState('Дом')
  const [address, setAddress] = useState('')
  const [busy, setBusy] = useState(false)
  const submit = async (event: React.FormEvent) => { event.preventDefault(); setBusy(true); try { await addAddress(label, address); setAddress('') } catch { /* error is rendered by the parent */ } finally { setBusy(false) } }
  return <main className="shell page profile-page"><div className="profile-heading"><div><p className="eyebrow coral">Личный кабинет</p><h1>Профиль</h1></div><button className="button button--outline" onClick={logout}>Выйти</button></div><section className="profile-grid"><article className="profile-card profile-card--identity"><span className="profile-avatar">{session.user.name.trim().charAt(0).toUpperCase()}</span><div><h2>{session.user.name}</h2><p>{session.user.email}</p><small>С нами с {new Date(session.user.createdAt).toLocaleDateString('ru-RU')}</small></div></article><button className="profile-card profile-card--action" onClick={openOrders}><span>История заказов</span><strong>{orders.length ? `${orders.length} ${orders.length === 1 ? 'заказ' : 'заказа'}` : 'Открыть'} →</strong></button><section className="address-book"><div className="address-book__heading"><div><span>Адреса доставки</span><strong>{addresses.length}</strong></div><p>Основной адрес подставляется при оформлении заказа.</p></div>{addressError && <p className="auth-error" role="alert">{addressError}</p>}<div className="address-list">{addresses.map((item) => <article className={item.isDefault ? 'address-row is-default' : 'address-row'} key={item.id}><div><b>{item.label}</b>{item.isDefault && <span>Основной</span>}<p>{item.address}</p></div><div>{!item.isDefault && <button type="button" onClick={() => void makeDefault(item.id)}>Сделать основным</button>}<button type="button" className="danger-link" onClick={() => void removeAddress(item.id)}>Удалить</button></div></article>)}</div><form className="address-form" onSubmit={submit}><label>Метка<input required maxLength={50} value={label} onChange={(event) => setLabel(event.target.value)} placeholder="Дом" /></label><label>Новый адрес<input required minLength={3} maxLength={500} value={address} onChange={(event) => setAddress(event.target.value)} placeholder="Самара, улица, дом, квартира" /></label><button className="button button--coral" disabled={busy}>{busy ? 'Сохраняем…' : 'Сохранить адрес'}</button></form></section></section></main>
}

export default function App() {
  const initialRoute = useMemo(routeFromPath, [])
  const savedCart = useMemo(readSavedCart, [])
  const [screen, setScreen] = useState<Screen>(initialRoute.screen)
  const [routeId, setRouteId] = useState(initialRoute.id)
  const [restaurants, setRestaurants] = useState<Restaurant[]>([])
  const [selected, setSelected] = useState<Restaurant | null>(null)
  const [menu, setMenu] = useState<Menu | null>(null)
  const [cart, setCart] = useState<Cart>(savedCart.cart)
  const [order, setOrder] = useState<Order | null>(null)
  const [orders, setOrders] = useState<Order[]>([])
  const [addresses, setAddresses] = useState<Address[]>([])
  const [addressError, setAddressError] = useState('')
  const [quote, setQuote] = useState<OrderQuote | null>(null)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState({ catalog: true, menu: false, orders: false, quote: false })
  const [errors, setErrors] = useState({ catalog: '', menu: '', orders: '', quote: '', submit: '', stream: '' })
  const [catalogRetry, setCatalogRetry] = useState(0)
  const [ordersRetry, setOrdersRetry] = useState(0)
  const [quoteRetry, setQuoteRetry] = useState(0)
  const [streamRetry, setStreamRetry] = useState(0)
  const [session, setSession] = useState<AuthSession | null>(() => getSession())

  const navigate = useCallback((path: string) => { history.pushState({}, '', path); const next = routeFromPath(); setScreen(next.screen); setRouteId(next.id); window.scrollTo(0, 0) }, [])
  const go = (next: Screen) => {
    const path = next === 'catalog' ? '/' : next === 'orders' ? '/orders' : next === 'checkout' ? '/checkout' : next === 'profile' ? '/profile' : next === 'auth' ? '/login' : '/'
    if (!session && (next === 'orders' || next === 'profile')) { navigate(`/login?return=${encodeURIComponent(path)}`); return }
    navigate(path)
  }
  useEffect(() => { const listener = () => { const next = routeFromPath(); setScreen(next.screen); setRouteId(next.id) }; addEventListener('popstate', listener); return () => removeEventListener('popstate', listener) }, [])
  useEffect(() => subscribeToSession(() => setSession(getSession())), [])
  useEffect(() => localStorage.setItem(CART_KEY, JSON.stringify({ restaurantId: selected?.id, cart })), [selected?.id, cart])

  useEffect(() => {
    if (!session) { setAddresses([]); return }
    setAddressError('')
    fetchAddresses().then(setAddresses).catch((reason) => setAddressError(reason instanceof Error ? reason.message : 'Не удалось загрузить адреса'))
  }, [session])

  useEffect(() => {
    if (session || (screen !== 'orders' && screen !== 'tracking' && screen !== 'profile')) return
    const returnPath = screen === 'orders' ? '/orders' : screen === 'profile' ? '/profile' : `/orders/${routeId ?? ''}`
    navigate(`/login?return=${encodeURIComponent(returnPath)}`)
  }, [screen, routeId, session, navigate])

  useEffect(() => {
    setLoading((value) => ({ ...value, catalog: true })); setErrors((value) => ({ ...value, catalog: '' }))
    fetchRestaurants().then((live) => { const restored = live.find((item) => item.id === savedCart.restaurantId); if (savedCart.restaurantId && !restored) setCart({}); setRestaurants(live); setSelected((current) => live.find((item) => item.id === current?.id) ?? restored ?? live[0] ?? null) }).catch(() => { setRestaurants([]); setErrors((value) => ({ ...value, catalog: 'Сервис ресторанов не отвечает. Повторите запрос, когда соединение восстановится.' })) }).finally(() => setLoading((value) => ({ ...value, catalog: false })))
  }, [catalogRetry])

  const loadMenu = useCallback(async (restaurant: Restaurant) => {
    setLoading((value) => ({ ...value, menu: true })); setErrors((value) => ({ ...value, menu: '' }))
    try { const live = await fetchMenu(restaurant.id); let productIndex = 0; const categories = live.categories.map((item) => ({ ...item, products: item.products.map((product) => ({ ...product, art: ['round', 'square', 'triangle'][productIndex++ % 3] as Product['art'] })) })); setMenu({ ...live, restaurant: { ...restaurant, ...live.restaurant }, categories }) }
    catch { setMenu({ restaurant, categories: [] }); setErrors((value) => ({ ...value, menu: 'Актуальное меню недоступно. Попробуйте ещё раз.' })) }
    finally { setLoading((value) => ({ ...value, menu: false })) }
  }, [])

  useEffect(() => { if (screen !== 'menu' || !routeId || !restaurants.length) return; const restaurant = restaurants.find((item) => item.id === routeId); if (!restaurant) { navigate('/'); return }; setSelected(restaurant); void loadMenu(restaurant) }, [screen, routeId, restaurants, loadMenu, navigate])
  useEffect(() => { if (screen === 'checkout' && selected) void loadMenu(selected) }, [screen, selected, loadMenu])
  useEffect(() => {
    if (screen !== 'checkout' || !selected || !menu || !Object.values(cart).some(Boolean)) return
    setLoading((value) => ({ ...value, quote: true })); setErrors((value) => ({ ...value, quote: '' }))
    quoteOrder(selected.id, cart).then(setQuote).catch((reason) => { setQuote(null); setErrors((value) => ({ ...value, quote: reason instanceof Error ? reason.message : 'Не удалось рассчитать заказ.' })) }).finally(() => setLoading((value) => ({ ...value, quote: false })))
  }, [screen, cart, menu, selected, quoteRetry])

  useEffect(() => {
    if (screen !== 'orders' || !session) return
    setLoading((value) => ({ ...value, orders: true })); setErrors((value) => ({ ...value, orders: '' }))
    fetchOrders().then((items) => setOrders(order && !items.some((item) => item.id === order.id) ? [order, ...items] : items)).catch(() => { if (order) setOrders([order]); else setErrors((value) => ({ ...value, orders: 'Не удалось загрузить историю заказов.' })) }).finally(() => setLoading((value) => ({ ...value, orders: false })))
  }, [screen, ordersRetry, order, session])

  useEffect(() => {
    if (screen !== 'tracking' || !routeId || !session) return
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
  }, [screen, routeId, streamRetry, session])

  const choose = (restaurant: Restaurant) => { if (selected?.id !== restaurant.id) setCart({}); setSelected(restaurant); navigate(`/restaurants/${restaurant.id}`) }
  const submit = async (address: string) => {
    if (!session) { navigate('/login?return=%2Fcheckout'); return }
    setBusy(true); setErrors((value) => ({ ...value, submit: '' }))
    try { if (!selected || !menu) throw new Error('Ресторан или меню не загружены'); const created = await createOrder(selected.id, cart, address); setOrder(created); setOrders((items) => [created, ...items.filter((item) => item.id !== created.id)]); setCart({}); navigate(`/orders/${created.id}`) }
    catch (reason) { setErrors((value) => ({ ...value, submit: reason instanceof Error ? reason.message : 'Не удалось оформить заказ' })) }
    finally { setBusy(false) }
  }

  const completeAuth = (nextSession: AuthSession) => {
    setSession(nextSession)
    const requested = new URLSearchParams(window.location.search).get('return')
    navigate(requested?.startsWith('/') && !requested.startsWith('//') ? requested : '/profile')
  }
  const logout = () => { saveSession(null); setSession(null); setOrders([]); setOrder(null); navigate('/') }
  const addAddress = async (label: string, address: string) => {
    setAddressError('')
    try { const created = await createAddress({ label, address, isDefault: addresses.length === 0 }); setAddresses((items) => created.isDefault ? [created, ...items.map((item) => ({ ...item, isDefault: false }))] : [...items, created]) }
    catch (reason) { setAddressError(reason instanceof Error ? reason.message : 'Не удалось сохранить адрес'); throw reason }
  }
  const makeDefault = async (id: string) => {
    setAddressError('')
    try { const updated = await updateAddress(id, { isDefault: true }); setAddresses((items) => [updated, ...items.filter((item) => item.id !== id).map((item) => ({ ...item, isDefault: false }))]) }
    catch (reason) { setAddressError(reason instanceof Error ? reason.message : 'Не удалось обновить адрес') }
  }
  const removeAddress = async (id: string) => {
    setAddressError('')
    try { await deleteAddress(id); setAddresses(await fetchAddresses()) }
    catch (reason) { setAddressError(reason instanceof Error ? reason.message : 'Не удалось удалить адрес') }
  }

  return <><Header screen={screen} session={session} go={go} />{screen === 'catalog' && <Catalog restaurants={restaurants} choose={choose} loading={loading.catalog} error={errors.catalog} retry={() => setCatalogRetry((value) => value + 1)} />}{screen === 'menu' && menu && <MenuPage menu={menu} cart={cart} setCart={setCart} checkout={() => go('checkout')} loading={loading.menu} error={errors.menu} retry={() => selected && void loadMenu(selected)} />}{screen === 'menu' && !menu && <main className="shell page"><div className="loading-card"><span /><h2>Загружаем меню</h2></div></main>}{screen === 'checkout' && <Checkout quote={quote} addresses={addresses} submit={submit} busy={busy} error={errors.submit} quoteLoading={loading.quote} quoteError={errors.quote} retryQuote={() => setQuoteRetry((value) => value + 1)} />}{screen === 'auth' && <AuthPage complete={completeAuth} />}{screen === 'profile' && session && <ProfilePage session={session} orders={orders} addresses={addresses} addressError={addressError} openOrders={() => go('orders')} addAddress={addAddress} makeDefault={makeDefault} removeAddress={removeAddress} logout={logout} />}{screen === 'orders' && session && <OrdersPage orders={orders} loading={loading.orders} error={errors.orders} retry={() => setOrdersRetry((value) => value + 1)} open={(item) => { setOrder(item); navigate(`/orders/${item.id}`) }} />}{screen === 'tracking' && session && order && <Tracking order={order} streamError={errors.stream} retry={() => setStreamRetry((value) => value + 1)} />}{screen === 'tracking' && session && !order && <main className="shell page"><div className="loading-card"><span /><h2>Загружаем заказ</h2></div></main>}<footer className="footer shell"><strong>LOOCH</strong><span>Еда рядом, когда она нужна.</span><small>Web-клиент платформы доставки</small></footer></>
}
