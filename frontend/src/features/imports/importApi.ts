import { apiRequest } from '../../api'

export function uploadTransactionFile(batchId: string, source: string, file: File) {
  const form = new FormData()
  form.set('file', file)
  form.set('source_type', source)
  return apiRequest(`/api/v1/batches/${encodeURIComponent(batchId)}/imports`, { method: 'POST', idempotent: true, body: form })
}
