import { useEffect, useRef, useState } from 'react'
import * as maplibregl from 'maplibre-gl'
import { LngLatBounds } from 'maplibre-gl'
import 'maplibre-gl/dist/maplibre-gl.css'
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url'
import type { Order } from './types'

const STYLE_URL = 'https://tiles.openfreemap.org/styles/positron'
maplibregl.setWorkerUrl(workerUrl)
const DEFAULT_RESTAURANT: [number, number] = [50.1267, 53.1959]
const restaurantLocations: Record<string, [number, number]> = {
  '11111111-1111-1111-1111-111111111111': [50.1008, 53.1952],
  '22222222-2222-2222-2222-222222222222': [50.1184, 53.1896],
  '33333333-3333-3333-3333-333333333333': [50.1513, 53.2051],
}

function deliveryLocation(address: string, origin: [number, number]): [number, number] {
  let hash = 2166136261
  for (const char of address) hash = Math.imul(hash ^ char.charCodeAt(0), 16777619)
  const east = ((hash >>> 0) % 1800) / 100000 + 0.018
  const north = (((hash >>> 11) >>> 0) % 1400) / 100000 - 0.007
  return [origin[0] + east, origin[1] + north]
}

function courierProgress(status: string) {
  if (status === 'delivered') return 1
  if (status === 'delivering') return 0.62
  if (status === 'ready') return 0.12
  return 0
}

function pointBetween(from: [number, number], to: [number, number], progress: number): [number, number] {
  return [from[0] + (to[0] - from[0]) * progress, from[1] + (to[1] - from[1]) * progress]
}

function marker(className: string, label: string) {
  const element = document.createElement('div')
  element.className = `delivery-marker ${className}`
  element.setAttribute('aria-label', label)
  element.title = label
  return element
}

export default function DeliveryMap({ order }: { order: Order }) {
  const container = useRef<HTMLDivElement>(null)
  const map = useRef<maplibregl.Map | null>(null)
  const courier = useRef<maplibregl.Marker | null>(null)
  const [failed, setFailed] = useState(false)
  const restaurant = restaurantLocations[order.restaurantId ?? ''] ?? DEFAULT_RESTAURANT
  const destination = deliveryLocation(order.deliveryAddress, restaurant)
  const courierPoint = pointBetween(restaurant, destination, courierProgress(order.status))

  useEffect(() => {
    if (!container.current) return
    setFailed(false)
    const instance = new maplibregl.Map({
      container: container.current,
      style: STYLE_URL,
      center: restaurant,
      zoom: 13,
      attributionControl: { compact: true },
      cooperativeGestures: true,
    })
    map.current = instance
    instance.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right')
    instance.on('error', () => setFailed(true))
    instance.on('load', () => {
      instance.addSource('delivery-route', {
        type: 'geojson',
        data: {
          type: 'Feature',
          properties: {},
          geometry: { type: 'LineString', coordinates: [restaurant, destination] },
        },
      })
      instance.addLayer({
        id: 'delivery-route-line',
        type: 'line',
        source: 'delivery-route',
        layout: { 'line-cap': 'round', 'line-join': 'round' },
        paint: { 'line-color': '#ef6248', 'line-width': 6, 'line-opacity': 0.88 },
      })
      new maplibregl.Marker({ element: marker('delivery-marker--restaurant', 'Ресторан') }).setLngLat(restaurant).addTo(instance)
      new maplibregl.Marker({ element: marker('delivery-marker--destination', 'Адрес доставки') }).setLngLat(destination).addTo(instance)
      courier.current = new maplibregl.Marker({ element: marker('delivery-marker--courier', 'Курьер') }).setLngLat(courierPoint).addTo(instance)
      instance.fitBounds(new LngLatBounds(restaurant, destination), { padding: 70, maxZoom: 14 })
    })
    return () => {
      courier.current = null
      map.current = null
      instance.remove()
    }
  }, [order.id])

  useEffect(() => {
    courier.current?.setLngLat(courierPoint)
  }, [courierPoint[0], courierPoint[1]])

  return <div className="map-card map-card--interactive">
    <div ref={container} className="delivery-map" aria-label="Интерактивная карта доставки" />
    <span className="map-label">{order.status === 'delivered' ? 'Доставлено' : 'Курьер Алексей'}</span>
    <span className="map-demo-label">Демо-маршрут · OpenFreeMap</span>
    {failed && <div className="map-fallback"><b>Карта временно недоступна</b><span>Заказ продолжает обновляться.</span></div>}
  </div>
}
