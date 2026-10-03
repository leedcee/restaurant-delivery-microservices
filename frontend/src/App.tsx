import { useEffect, useMemo, useState } from 'react'
import { createOrder, fetchMenu, fetchOrder, fetchRestaurants } from './api'
import { demoMenu, demoRestaurants } from './mock'
import type { Cart, Menu, Order, Product, Restaurant, Screen } from './types'

const money = (minor: number) => `${new Intl.NumberFormat('ru-RU').format(minor / 100)} ₽`

function FoodArt({ kind = 'round', restaurant = false }: { kind?: string; restaurant?: boolean }) {
  return <div className={`food-art food-art--${kind} ${restaurant ? 'food-art--restaurant' : ''}`} aria-hidden="true"><span /><i /><b /></div>
}

function Header({ screen, go }: { screen: Screen; go: (screen: Screen) => void }) {
  return (
    <header className="header shell">
      <button className="brand" onClick={() => go('catalog')} aria-label="На главную">
        <strong>LOOCH</strong><span>еда рядом</span>
      </button>
      <nav aria-label="Основная навигация">
        <button className={screen === 'catalog' || screen === 'menu' ? 'active' : ''} onClick={() => go('catalog')}>Рестораны</button>
        <button className={screen === 'checkout' || screen === 'tracking' ? 'active' : ''} onClick={() => go('tracking')}>Заказы</button>
        <button>Профиль</button>
      </nav>
      <div className="avatar">Л</div>
    </header>
  )
}

function Catalog({ restaurants, choose }: { restaurants: Restaurant[]; choose: (restaurant: Restaurant) => void }) {
  const [query, setQuery] = useState('')
  const filtered = restaurants.filter((item) => `${item.name} ${item.cuisine}`.toLowerCase().includes(query.toLowerCase()))
  return (
    <main className="shell page catalog-page">
      <section className="hero">
        <p className="eyebrow">Доставка из ресторанов рядом</p>
        <h1>Что хочется сегодня?</h1>
        <p>Собрали хорошие места и проверили, что у них есть в наличии прямо сейчас.</p>
        <div className="search-row">
          <label className="search"><span>⌕</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Название или кухня" /></label>
          <button className="button button--coral">Найти</button>
          <button className="button button--outline">Фильтры <b>2</b></button>
        </div>
      </section>
      <div className="section-heading"><div><p className="eyebrow coral">Самара · Московское шоссе</p><h2>Популярно рядом</h2></div><button className="text-link">Смотреть все →</button></div>
      <section className="restaurant-grid">
        {filtered.map((restaurant) => (
          <button className="restaurant-card offset-card" key={restaurant.id} onClick={() => choose(restaurant)}>
            <div className="restaurant-card__art"><FoodArt kind={restaurant.art} restaurant /><span className="rating">★ {restaurant.rating}</span></div>
            <div className="restaurant-card__body">
              <h3>{restaurant.name}</h3>
              <p>{restaurant.cuisine}</p>
              <span className="eta">{restaurant.eta}</span>
              <span className="card-arrow">→</span>
            </div>
          </button>
        ))}
      </section>
      {!filtered.length && <div className="empty"><h3>Ничего не нашли</h3><p>Попробуйте изменить запрос.</p></div>}
    </main>
  )
}

function Quantity({ value, add, remove }: { value: number; add: () => void; remove: () => void }) {
  return value ? <div className="quantity"><button onClick={remove}>−</button><b>{value}</b><button onClick={add}>+</button></div> : <button className="add-button" onClick={add}>+</button>
}

function MenuPage({ menu, cart, setCart, checkout }: { menu: Menu; cart: Cart; setCart: (cart: Cart) => void; checkout: () => void }) {
  const [category, setCategory] = useState(menu.categories[0]?.id)
  const products = menu.categories.find((item) => item.id === category)?.products ?? menu.categories.flatMap((item) => item.products)
  const allProducts = menu.categories.flatMap((item) => item.products)
  const count = Object.values(cart).reduce((sum, quantity) => sum + quantity, 0)
  const total = allProducts.reduce((sum, product) => sum + product.priceMinor * (cart[product.id] ?? 0), 0)
  const update = (product: Product, delta: number) => setCart({ ...cart, [product.id]: Math.max(0, (cart[product.id] ?? 0) + delta) })
  return (
    <main className="shell page menu-page">
      <button className="back" onClick={() => history.back()}>← Все рестораны</button>
      <div className="menu-title"><div><h1>{menu.restaurant.name}</h1><p>25–35 минут&nbsp; · &nbsp;рейтинг 4,8</p></div><span className="open-pill">Открыт до 22:00</span></div>
      <div className="tabs" role="tablist">
        {menu.categories.map((item, index) => <button key={item.id} className={item.id === category ? 'active' : ''} onClick={() => setCategory(item.id)}>{index === 0 ? 'Популярное' : item.name}</button>)}
      </div>
      <section className="product-grid">
        {products.map((product, index) => (
          <article className="product-card offset-card" key={product.id}>
            <div className="product-art"><FoodArt kind={product.art ?? ['round', 'square', 'triangle'][index % 3]} /></div>
            <div className="product-body"><h3>{product.name}</h3><p>{product.description}</p><div><strong>{money(product.priceMinor)}</strong><Quantity value={cart[product.id] ?? 0} add={() => update(product, 1)} remove={() => update(product, -1)} /></div></div>
          </article>
        ))}
        {!products.length && <div className="empty"><h3>Здесь скоро появятся блюда</h3><p>Загляните в раздел «Популярное».</p></div>}
      </section>
      {count > 0 && <button className="cart-dock" onClick={checkout}><span>Корзина&nbsp; • &nbsp;{count} {count === 1 ? 'блюдо' : 'блюда'}</span><strong>{money(total)}</strong></button>}
    </main>
  )
}

function Checkout({ menu, cart, submit, busy, error }: { menu: Menu; cart: Cart; submit: (address: string) => void; busy: boolean; error: string }) {
  const [address, setAddress] = useState('Самара, Московское шоссе, 15')
  const products = menu.categories.flatMap((item) => item.products)
  const selected = products.filter((product) => cart[product.id]).map((product) => ({ ...product, cartQuantity: cart[product.id] }))
  const subtotal = selected.reduce((sum, product) => sum + product.priceMinor * product.cartQuantity, 0)
  const delivery = 13000
  return (
    <main className="shell page checkout-page">
      <h1>Оформление заказа</h1>
      <div className="steps"><span className="done"><b>Корзина</b></span><i /><span className="active"><b>Доставка</b></span><i /><span><b>Оплата</b></span></div>
      <div className="checkout-layout">
        <form className="checkout-form" onSubmit={(event) => { event.preventDefault(); submit(address) }} id="checkout-form">
          <label>Адрес доставки<input required value={address} onChange={(event) => setAddress(event.target.value)} /></label>
          <label>Время<select defaultValue="asap"><option value="asap">Как можно скорее · 25–35 минут</option><option value="later">Выбрать время</option></select></label>
          <label>Оплата<select defaultValue="card"><option value="card">Картой онлайн ···· 4821</option><option value="cash">При получении</option></select></label>
          <label>Комментарий<textarea placeholder="Код домофона, ориентир" rows={2} /></label>
        </form>
        <aside className="order-summary">
          <h2>Ваш заказ</h2>
          <div className="summary-lines">{selected.map((product) => <p key={product.id}><span>{product.name} × {product.cartQuantity}</span><b>{money(product.priceMinor * product.cartQuantity)}</b></p>)}<p><span>Доставка</span><b>{money(delivery)}</b></p></div>
          <div className="summary-total"><span>Итого</span><strong>{money(subtotal + delivery)}</strong></div>
          {error && <p className="form-error">{error}</p>}
          <button className="button button--coral summary-button" form="checkout-form" disabled={busy}>{busy ? 'Оформляем…' : 'Оформить заказ'}</button>
          <small>Нажимая кнопку, вы соглашаетесь с условиями сервиса</small>
        </aside>
      </div>
    </main>
  )
}

const statusIndex: Record<string, number> = { pending: 0, accepted: 0, preparing: 1, ready: 1, delivering: 2, delivered: 3 }

function Tracking({ order }: { order: Order }) {
  const active = statusIndex[order.status] ?? 2
  const shortId = order.id.startsWith('demo-') ? '18452' : order.id.slice(0, 5).toUpperCase()
  return (
    <main className="shell page tracking-page">
      <div className="tracking-heading"><div><p className="eyebrow coral">Заказ подтверждён</p><h1>Заказ № {shortId}</h1></div><span className="delivery-time">Доставим к 19:35</span></div>
      <div className="status-track">{['Принят', 'Готовится', 'У курьера', 'Доставлен'].map((label, index) => <div className={index <= active ? 'done' : ''} key={label}><span>{index < active ? '✓' : ''}</span><b>{label}</b></div>)}</div>
      <div className="tracking-layout">
        <div className="map-card"><div className="street-lines" /><svg viewBox="0 0 700 360" preserveAspectRatio="none" aria-label="Маршрут курьера"><polyline points="65,310 240,105 610,58" /><circle cx="65" cy="310" r="15" className="map-start" /><circle cx="610" cy="58" r="18" className="map-courier" /></svg><span className="map-label">Курьер Алексей</span></div>
        <aside className="courier-card"><span className="courier-icon">А</span><div><h2>Курьер {active >= 2 ? 'в пути' : 'скоро заберёт заказ'}</h2><p>Алексей&nbsp; · &nbsp;рейтинг 4,9</p></div><hr /><p className="muted">Адрес</p><strong>{order.deliveryAddress}</strong><p className="muted">Осталось примерно</p><b className="minutes">12 минут</b><button className="button button--outline">Связаться с курьером</button></aside>
      </div>
    </main>
  )
}

export default function App() {
  const [screen, setScreen] = useState<Screen>('catalog')
  const [restaurants, setRestaurants] = useState(demoRestaurants)
  const [selected, setSelected] = useState<Restaurant>(demoRestaurants[0])
  const [menu, setMenu] = useState<Menu>(demoMenu())
  const [cart, setCart] = useState<Cart>({})
  const [order, setOrder] = useState<Order | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    fetchRestaurants().then((live) => {
      if (!live.length) return
      const merged = demoRestaurants.map((item, index) => index === 0 ? { ...item, id: live[0].id, isOpen: live[0].isOpen } : item)
      setRestaurants(merged)
      setSelected(merged[0])
    }).catch(() => undefined)
  }, [])

  useEffect(() => {
    if (!order || screen !== 'tracking' || order.id.startsWith('demo-')) return
    const timer = window.setInterval(() => fetchOrder(order.id).then(setOrder).catch(() => undefined), 1000)
    return () => window.clearInterval(timer)
  }, [order, screen])

  const choose = async (restaurant: Restaurant) => {
    setSelected(restaurant)
    setCart({})
    if (restaurant === restaurants[0]) {
      try {
        const liveMenu = await fetchMenu(restaurant.id)
        const decorated = liveMenu.categories.flatMap((item) => item.products).map((product, index) => ({ ...product, art: ['round', 'square', 'triangle'][index % 3] as Product['art'] }))
        setMenu({ ...liveMenu, restaurant: { ...restaurant, ...liveMenu.restaurant }, categories: [{ id: 'popular', name: 'Популярное', products: decorated }, ...liveMenu.categories.map((item) => ({ ...item, products: [] }))] })
      } catch { setMenu(demoMenu(restaurant)) }
    } else setMenu(demoMenu(restaurant))
    setScreen('menu')
    window.scrollTo(0, 0)
  }

  const submit = async (address: string) => {
    setBusy(true); setError('')
    try {
      let created: Order
      if (selected.id === restaurants[0].id && !menu.categories.flatMap((item) => item.products).some((item) => item.id.startsWith('aaaaaaaa'))) {
        created = await createOrder(selected.id, cart, address)
      } else {
        const products = menu.categories.flatMap((item) => item.products)
        created = { id: `demo-${Date.now()}`, status: 'delivering', deliveryAddress: address, currency: 'RUB', items: products.filter((item) => cart[item.id]).map((item) => ({ productId: item.id, name: item.name, quantity: cart[item.id], unitPriceMinor: item.priceMinor, totalMinor: item.priceMinor * cart[item.id] })), totalMinor: products.reduce((sum, item) => sum + item.priceMinor * (cart[item.id] ?? 0), 0) }
      }
      setOrder(created); setScreen('tracking'); window.scrollTo(0, 0)
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'Не удалось оформить заказ') }
    finally { setBusy(false) }
  }

  const fallbackOrder = useMemo<Order>(() => ({ id: '18452-DEMO', status: 'delivering', deliveryAddress: 'Самара, Московское шоссе, 15', items: [], totalMinor: 146000, currency: 'RUB' }), [])
  const go = (next: Screen) => { if (next === 'tracking' && !order) setOrder(fallbackOrder); setScreen(next); window.scrollTo(0, 0) }

  return <><Header screen={screen} go={go} />{screen === 'catalog' && <Catalog restaurants={restaurants} choose={choose} />}{screen === 'menu' && <MenuPage menu={menu} cart={cart} setCart={setCart} checkout={() => go('checkout')} />}{screen === 'checkout' && <Checkout menu={menu} cart={cart} submit={submit} busy={busy} error={error} />}{screen === 'tracking' && <Tracking order={order ?? fallbackOrder} />}<footer className="footer shell"><strong>LOOCH</strong><span>Еда рядом, когда она нужна.</span><small>Демонстрационный интерфейс сервиса доставки</small></footer></>
}
