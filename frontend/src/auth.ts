import type { AuthSession } from './types'

const AUTH_KEY = 'looch-auth-v1'
const AUTH_EVENT = 'looch-auth-changed'
let refreshPromise: Promise<AuthSession | null> | null = null

export function getSession(): AuthSession | null {
  try {
	const stored = JSON.parse(localStorage.getItem(AUTH_KEY) ?? 'null') as (AuthSession & { refreshToken?: string }) | null
	if (stored?.refreshToken) {
		const sanitized = { user: stored.user, accessToken: stored.accessToken, expiresIn: stored.expiresIn }
		localStorage.setItem(AUTH_KEY, JSON.stringify(sanitized))
		return sanitized
	}
	return stored
  } catch { return null }
}

export function saveSession(session: AuthSession | null) {
	if (session) localStorage.setItem(AUTH_KEY, JSON.stringify({ user: session.user, accessToken: session.accessToken, expiresIn: session.expiresIn }))
  else localStorage.removeItem(AUTH_KEY)
  window.dispatchEvent(new Event(AUTH_EVENT))
}

export function subscribeToSession(listener: () => void) {
  window.addEventListener(AUTH_EVENT, listener)
  return () => window.removeEventListener(AUTH_EVENT, listener)
}

async function parse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const problem = await response.json().catch(() => null)
    throw new Error(problem?.detail || problem?.title || `HTTP ${response.status}`)
  }
  return response.json() as Promise<T>
}

export async function register(input: { name: string; email: string; password: string }): Promise<AuthSession> {
  const response = await fetch('/api/v1/auth/register', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) })
  const session = await parse<AuthSession>(response)
  saveSession(session)
  return session
}

export async function login(input: { email: string; password: string }): Promise<AuthSession> {
  const response = await fetch('/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(input) })
  const session = await parse<AuthSession>(response)
  saveSession(session)
  return session
}

async function refreshSession(): Promise<AuthSession | null> {
	const response = await fetch('/api/v1/auth/refresh', { method: 'POST' })
  if (!response.ok) { saveSession(null); return null }
  const next = await response.json() as AuthSession
  saveSession(next)
  return next
}

function refreshSessionOnce(): Promise<AuthSession | null> {
	if (!refreshPromise) refreshPromise = refreshSession().finally(() => { refreshPromise = null })
	return refreshPromise
}

export async function logout(): Promise<void> {
	try { await fetch('/api/v1/auth/logout', { method: 'POST' }) }
	finally { saveSession(null) }
}

export async function authorizedFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const send = (accessToken?: string) => fetch(input, { ...init, headers: { ...Object.fromEntries(new Headers(init.headers).entries()), ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}) } })
  const current = getSession()
  if (!current) throw new Error('Требуется вход в аккаунт')
  let response = await send(current.accessToken)
  if (response.status !== 401) return response
	const refreshed = await refreshSessionOnce()
  if (!refreshed) throw new Error('Сессия истекла. Войдите снова.')
  response = await send(refreshed.accessToken)
  return response
}
