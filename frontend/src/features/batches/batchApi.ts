import { apiRequest } from '../../api'

export function fetchBatch(id: string) {
  return apiRequest<Record<string, any>>(`/api/v1/batches/${encodeURIComponent(id)}`)
}
