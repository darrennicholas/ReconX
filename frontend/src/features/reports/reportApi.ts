import { apiRequest, downloadFile } from '../../api'

export interface ReportFilters {
  from: string
  to: string
  accountId: string
  query: string
  status: string
}

function reportParams(filters: ReportFilters): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.from) params.set('from', filters.from)
  if (filters.to) params.set('to', filters.to)
  if (filters.accountId) params.set('account_id', filters.accountId)
  if (filters.query.trim()) params.set('q', filters.query.trim())
  if (filters.status) params.set('status', filters.status)
  return params
}

export function fetchReport(filters: ReportFilters, page: number, pageSize: number) {
  const params = reportParams(filters)
  params.set('page', String(page))
  params.set('page_size', String(pageSize))
  return apiRequest<{ items: Record<string, any>[]; summary: Record<string, any> }>(`/api/v1/reports/reconciliations?${params}`)
}

export function exportReport(filters: ReportFilters, type: string) {
  const params = reportParams(filters)
  params.set('type', type)
  return downloadFile(`/api/v1/reports/reconciliations/export?${params}`, `reconx-${type}.csv`)
}
