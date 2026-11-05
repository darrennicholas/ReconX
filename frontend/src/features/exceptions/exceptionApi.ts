import { apiRequest } from '../../api'

export function fetchException(id: string) {
  return apiRequest<Record<string, any>>(`/api/v1/exceptions/${encodeURIComponent(id)}`)
}

export function fetchApproval(id: string) {
  return apiRequest<Record<string, any>>(`/api/v1/approvals/${encodeURIComponent(id)}`)
}
