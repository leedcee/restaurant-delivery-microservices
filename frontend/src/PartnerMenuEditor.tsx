import { useEffect, useMemo, useState } from 'react'
import type { PartnerMenu } from './types'

type Props = {
  menu: PartnerMenu | null
  loading: boolean
  saving: boolean
  error: string
  reload: () => void
  save: (menu: PartnerMenu) => Promise<void>
  openOrders: () => void
  logout: () => void
}

const blankProduct = () => ({ externalId: `product-${crypto.randomUUID()}`, name: 'Новое блюдо', description: '', priceMinor: 0, quantity: 0, available: false })
const copyMenu = (menu: PartnerMenu): PartnerMenu => ({ categories: menu.categories.map((category) => ({ ...category, products: category.products.map((product) => ({ ...product })) })) })

export default function PartnerMenuEditor({ menu, loading, saving, error, reload, save, openOrders, logout }: Props) {
  const [draft, setDraft] = useState<PartnerMenu>({ categories: [] })
  const [saved, setSaved] = useState(false)
  useEffect(() => { if (menu) setDraft(copyMenu(menu)) }, [menu])
  const productCount = useMemo(() => draft.categories.reduce((sum, category) => sum + category.products.length, 0), [draft])
  const updateCategory = (index: number, patch: Partial<PartnerMenu['categories'][number]>) => setDraft((current) => ({ ...current, categories: current.categories.map((category, categoryIndex) => categoryIndex === index ? { ...category, ...patch } : category) }))
  const updateProduct = (categoryIndex: number, productIndex: number, patch: Partial<PartnerMenu['categories'][number]['products'][number]>) => setDraft((current) => ({ ...current, categories: current.categories.map((category, currentCategoryIndex) => currentCategoryIndex === categoryIndex ? { ...category, products: category.products.map((product, currentProductIndex) => currentProductIndex === productIndex ? { ...product, ...patch } : product) } : category) }))
  const addCategory = () => setDraft((current) => ({ ...current, categories: [...current.categories, { externalId: `category-${crypto.randomUUID()}`, name: 'Новая категория', position: current.categories.length + 1, products: [] }] }))
  const removeCategory = (index: number) => setDraft((current) => ({ ...current, categories: current.categories.filter((_, categoryIndex) => categoryIndex !== index) }))
  const addProduct = (index: number) => updateCategory(index, { products: [...draft.categories[index].products, blankProduct()] })
  const removeProduct = (categoryIndex: number, productIndex: number) => updateCategory(categoryIndex, { products: draft.categories[categoryIndex].products.filter((_, index) => index !== productIndex) })
  const submit = async () => {
    setSaved(false)
    const normalized = { categories: draft.categories.map((category, index) => ({ ...category, position: index + 1 })) }
    try {
      await save(normalized)
      setDraft(copyMenu(normalized))
      setSaved(true)
    } catch {
      // The parent exposes the API error in the page notice.
    }
  }

  return <main className="shell page partner-page menu-editor-page">
    <div className="partner-top"><div><p className="eyebrow coral">Кабинет ресторана</p><h1>Меню и остатки</h1><p>{draft.categories.length} категорий · {productCount} блюд</p></div><div><button className="button button--outline" onClick={openOrders}>Заказы</button><button className="partner-logout" onClick={logout}>Выйти</button></div></div>
    {error && <NoticeMessage message={error} retry={reload} />}
    {saved && <p className="menu-editor-success" role="status">Меню сохранено и уже доступно клиентам.</p>}
    {loading && !menu ? <div className="loading-card"><span /><h2>Загружаем меню</h2></div> : <>
      <section className="menu-editor-toolbar"><div><strong>Структура меню</strong><span>Изменения публикуются после сохранения.</span></div><div><button className="button button--outline" type="button" onClick={addCategory}>Добавить категорию</button><button className="button button--coral" type="button" disabled={saving} onClick={() => void submit()}>{saving ? 'Сохраняем…' : 'Сохранить меню'}</button></div></section>
      <section className="menu-editor-categories">{draft.categories.map((category, categoryIndex) => <article className="menu-editor-category" key={category.externalId}><header><label>Название категории<input required maxLength={100} aria-label={`Название категории ${categoryIndex + 1}`} value={category.name} onChange={(event) => updateCategory(categoryIndex, { name: event.target.value })} /></label><button className="danger-link" type="button" onClick={() => removeCategory(categoryIndex)}>Удалить категорию</button></header><div className="menu-editor-products">{category.products.map((product, productIndex) => <section className="menu-editor-product" key={product.externalId}><div className="menu-editor-product__heading"><strong>{product.name || 'Без названия'}</strong><button className="danger-link" type="button" onClick={() => removeProduct(categoryIndex, productIndex)}>Удалить блюдо</button></div><div className="menu-editor-fields"><label>Название<input required maxLength={200} aria-label={`Название блюда ${categoryIndex + 1}-${productIndex + 1}`} value={product.name} onChange={(event) => updateProduct(categoryIndex, productIndex, { name: event.target.value })} /></label><label className="wide">Описание<textarea maxLength={1000} rows={2} aria-label={`Описание блюда ${categoryIndex + 1}-${productIndex + 1}`} value={product.description} onChange={(event) => updateProduct(categoryIndex, productIndex, { description: event.target.value })} /></label><label>Цена, ₽<input type="number" min="0" max="1000000" step="1" aria-label={`Цена блюда ${categoryIndex + 1}-${productIndex + 1}`} value={product.priceMinor / 100} onChange={(event) => updateProduct(categoryIndex, productIndex, { priceMinor: Math.max(0, Math.round(Number(event.target.value) * 100)) })} /></label><label>Остаток<input type="number" min="0" max="1000000" step="1" aria-label={`Остаток блюда ${categoryIndex + 1}-${productIndex + 1}`} value={product.quantity} onChange={(event) => updateProduct(categoryIndex, productIndex, { quantity: Math.max(0, Math.round(Number(event.target.value))) })} /></label><label className="availability"><input type="checkbox" aria-label={`Доступность блюда ${categoryIndex + 1}-${productIndex + 1}`} checked={product.available} onChange={(event) => updateProduct(categoryIndex, productIndex, { available: event.target.checked })} /><span>{product.available ? 'В продаже' : 'В стоп-листе'}</span></label></div></section>)}</div><button className="menu-editor-add" type="button" onClick={() => addProduct(categoryIndex)}>+ Добавить блюдо</button></article>)}{!draft.categories.length && <div className="empty"><h3>Меню пока пусто</h3><p>Создайте первую категорию и добавьте в неё блюда.</p></div>}</section>
    </>}
  </main>
}

function NoticeMessage({ message, retry }: { message: string; retry: () => void }) {
  return <div className="notice"><span>{message}</span><button onClick={retry}>Повторить</button></div>
}
