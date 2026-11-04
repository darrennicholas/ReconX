export interface ApiEnvelope<T> {
  success: boolean
  data?: T
  meta?: Record<string, unknown>
  error?: { code: string; message: string }
}

export class ApiError extends Error {
  constructor(message: string, public readonly status: number, public readonly data?: unknown) {
    super(message)
  }
}

function cookie(name: string): string {
  const prefix = `${name}=`
  const item = document.cookie.split('; ').find(value => value.startsWith(prefix))
  return item ? decodeURIComponent(item.slice(prefix.length)) : ''
}

export async function apiRequest<T>(path: string, options: RequestInit & { idempotent?: boolean } = {}): Promise<{ data: T; response: Response }> {
  const headers = new Headers(options.headers)
  const method = (options.method ?? 'GET').toUpperCase()
  const mutating = !['GET', 'HEAD', 'OPTIONS'].includes(method)
  const isForm = options.body instanceof FormData
  if (options.body && !isForm && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  if (mutating) {
    const csrf = cookie('reconx_csrf')
    if (csrf) headers.set('X-CSRF-Token', csrf)
    if (options.idempotent) headers.set('Idempotency-Key', crypto.randomUUID())
  }
  headers.set('X-Request-ID', crypto.randomUUID())
  const response = await fetch(path, { ...options, headers, credentials: 'include' })
  const envelope = await response.json().catch(() => ({})) as ApiEnvelope<T>
  if (!response.ok || envelope.success === false) {
    throw new ApiError(envelope.error?.message ?? `Request gagal (${response.status})`, response.status, envelope.data)
  }
  return { data: envelope.data as T, response }
}

export async function downloadFile(path: string, filename: string): Promise<void> {
  const response = await fetch(path, { credentials: 'include', headers: { 'X-Request-ID': crypto.randomUUID() } })
  if (!response.ok) throw new ApiError('Gagal mengunduh file laporan.', response.status)
  const link = document.createElement('a')
  link.href = URL.createObjectURL(await response.blob())
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}
