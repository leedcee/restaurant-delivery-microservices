import { useEffect, useRef, useState } from 'react'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
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
  const updateCourier = useRef<((point: [number, number]) => void) | null>(null)
  const [failed, setFailed] = useState(false)
  const [provider, setProvider] = useState<'vector' | 'raster'>('vector')
  const restaurant = restaurantLocations[order.restaurantId ?? ''] ?? DEFAULT_RESTAURANT
  const destination = deliveryLocation(order.deliveryAddress, restaurant)
  const courierPoint = pointBetween(restaurant, destination, courierProgress(order.status))

  useEffect(() => {
    if (!container.current) return
    const element = container.current
    let vectorMap: maplibregl.Map | null = null
    let rasterMap: L.Map | null = null
    let fallbackTimer: number | undefined
    let disposed = false
    setFailed(false)

    const startRasterMap = () => {
      if (disposed || rasterMap) return
      if (fallbackTimer) window.clearTimeout(fallbackTimer)
      if (vectorMap) {
        vectorMap.remove()
        vectorMap = null
      }
      element.replaceChildren()
      try {
        rasterMap = L.map(element, { zoomControl: true, attributionControl: true, scrollWheelZoom: false })
        L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
          maxZoom: 19,
          attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
        }).addTo(rasterMap)
        const icon = (className: string, label: string) => L.divIcon({
          className: '',
          html: `<span class="delivery-marker ${className}" role="img" aria-label="${label}" title="${label}"></span>`,
          iconSize: className.endsWith('courier') ? [34, 34] : [28, 28],
          iconAnchor: className.endsWith('courier') ? [17, 17] : [14, 14],
        })
        const from: L.LatLngExpression = [restaurant[1], restaurant[0]]
        const to: L.LatLngExpression = [destination[1], destination[0]]
        L.polyline([from, to], { color: '#ef6248', weight: 6, opacity: 0.88 }).addTo(rasterMap)
        L.marker(from, { icon: icon('delivery-marker--restaurant', 'Ресторан') }).addTo(rasterMap)
        L.marker(to, { icon: icon('delivery-marker--destination', 'Адрес доставки') }).addTo(rasterMap)
        const courierMarker = L.marker([courierPoint[1], courierPoint[0]], { icon: icon('delivery-marker--courier', 'Курьер') }).addTo(rasterMap)
        updateCourier.current = (point) => courierMarker.setLatLng([point[1], point[0]])
        rasterMap.fitBounds(L.latLngBounds(from, to), { padding: [55, 55], maxZoom: 14 })
        setProvider('raster')
        setFailed(false)
      } catch {
        setFailed(true)
      }
    }

    try {
      if (!document.createElement('canvas').getContext('webgl2')) {
        startRasterMap()
      } else {
        vectorMap = new maplibregl.Map({
          container: element,
          style: STYLE_URL,
          center: restaurant,
          zoom: 13,
          attributionControl: { compact: true },
          cooperativeGestures: true,
        })
        vectorMap.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right')
        vectorMap.on('webglcontextlost', startRasterMap)
        vectorMap.on('load', () => {
          if (!vectorMap || disposed) return
          if (fallbackTimer) window.clearTimeout(fallbackTimer)
          vectorMap.addSource('delivery-route', {
            type: 'geojson',
            data: {
              type: 'Feature',
              properties: {},
              geometry: { type: 'LineString', coordinates: [restaurant, destination] },
            },
          })
          vectorMap.addLayer({
            id: 'delivery-route-line',
            type: 'line',
            source: 'delivery-route',
            layout: { 'line-cap': 'round', 'line-join': 'round' },
            paint: { 'line-color': '#ef6248', 'line-width': 6, 'line-opacity': 0.88 },
          })
          new maplibregl.Marker({ element: marker('delivery-marker--restaurant', 'Ресторан') }).setLngLat(restaurant).addTo(vectorMap)
          new maplibregl.Marker({ element: marker('delivery-marker--destination', 'Адрес доставки') }).setLngLat(destination).addTo(vectorMap)
          const courierMarker = new maplibregl.Marker({ element: marker('delivery-marker--courier', 'Курьер') }).setLngLat(courierPoint).addTo(vectorMap)
          updateCourier.current = (point) => courierMarker.setLngLat(point)
          vectorMap.fitBounds(new LngLatBounds(restaurant, destination), { padding: 70, maxZoom: 14 })
          setProvider('vector')
        })
        fallbackTimer = window.setTimeout(startRasterMap, 5000)
      }
    } catch {
      startRasterMap()
    }
    return () => {
      disposed = true
      if (fallbackTimer) window.clearTimeout(fallbackTimer)
      updateCourier.current = null
      vectorMap?.remove()
      rasterMap?.remove()
    }
  }, [order.id])

  useEffect(() => {
    updateCourier.current?.(courierPoint)
  }, [courierPoint[0], courierPoint[1]])

  return <div className="map-card map-card--interactive">
    <div ref={container} className="delivery-map" aria-label="Интерактивная карта доставки" />
    <span className="map-label">{order.status === 'delivered' ? 'Доставлено' : 'Курьер Алексей'}</span>
    <span className="map-demo-label">Демо-маршрут · {provider === 'vector' ? 'OpenFreeMap' : 'OpenStreetMap'}</span>
    {failed && <div className="map-fallback"><b>Карта временно недоступна</b><span>Заказ продолжает обновляться.</span></div>}
  </div>
}
