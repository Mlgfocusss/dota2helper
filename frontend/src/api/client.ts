import { t } from '@/i18n/ru'
import { API_BASE } from '@/lib/config'
import type { ApiErrorBody } from './types'

export class ApiError extends Error {
  /** HTTP‑статус; 0 — запрос не дошёл до сервера */
  readonly status: number
  readonly code: string
  /** Ошибки по полям формы (код validation_failed): имя поля → текст */
  readonly fields: Record<string, string>

  constructor(status: number, code: string, message: string, fields: Record<string, string> = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields
  }

  get isNetwork(): boolean {
    return this.status === 0
  }
}

type Query = Record<string, string | number | boolean | null | undefined>

export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  query?: Query
  signal?: AbortSignal
}

function buildUrl(path: string, query?: Query): string {
  const url = API_BASE + path
  if (!query) return url
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  }
  const qs = params.toString()
  return qs ? `${url}?${qs}` : url
}

/** Запрос к нашему API. Ошибки приводятся к ApiError с текстом для пользователя. */
export async function api<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, query, signal } = options
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  let res: Response
  try {
    res = await fetch(buildUrl(path, query), {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
      signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network', t.states.network)
  }

  if (res.status === 204) return undefined as T

  const text = await res.text()
  let data: unknown
  try {
    data = text ? JSON.parse(text) : undefined
  } catch {
    data = undefined
  }

  if (!res.ok) {
    const info = (data as ApiErrorBody | undefined)?.error
    if (info) throw new ApiError(res.status, info.code, info.message, info.fields)
    // не JSON — обычно nginx отвечает 502/504, пока бэкенд перезапускается
    const message = res.status >= 500 ? t.states.serverDown : t.states.unknown
    throw new ApiError(res.status, `http_${res.status}`, message)
  }
  return data as T
}
