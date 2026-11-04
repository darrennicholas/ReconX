import { computed, ref } from 'vue'

const pendingActions = ref(0)
export const isActionLoading = computed(() => pendingActions.value > 0)

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
  if (mutating) pendingActions.value++
  try {
    const response = await fetch(path, { ...options, headers, credentials: 'include' })
    const envelope = await response.json().catch(() => ({})) as ApiEnvelope<T>
    if (!response.ok || envelope.success === false) {
      throw new ApiError(envelope.error?.message ?? `Request gagal (${response.status})`, response.status, envelope.data)
    }
    return { data: envelope.data as T, response }
  } finally {
    if (mutating) {
      pendingActions.value--
      if (!path.startsWith('/api/v1/notifications/') && path !== '/api/v1/auth/login' && path !== '/api/v1/auth/logout') {
        window.dispatchEvent(new Event('reconx:notifications-changed'))
      }
    }
  }
}

export async function downloadFile(path: string, filename: string): Promise<void> {
  pendingActions.value++
  try {
    const response = await fetch(path, { credentials: 'include', headers: { 'X-Request-ID': crypto.randomUUID() } })
    if (!response.ok) throw new ApiError('Gagal mengunduh file laporan.', response.status)
    const link = document.createElement('a')
    link.href = URL.createObjectURL(await response.blob())
    link.download = filename
    link.click()
    URL.revokeObjectURL(link.href)
  } finally {
    pendingActions.value--
  }
}
