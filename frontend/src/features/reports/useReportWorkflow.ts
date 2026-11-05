import type { Ref } from 'vue'
import { ApiError } from '../../api'
import { exportReport, fetchReport, type ReportFilters } from './reportApi'

type Row = Record<string, string>
interface ReportContext {
  from: Ref<string>; to: Ref<string>; accountId: Ref<string>; query: Ref<string>; status: Ref<string>; exportType: Ref<string>
  page: Ref<number>; pageSize: number; total: Ref<number>; rows: Ref<Row[]>; summary: Ref<Record<string, any>>
  loading: Ref<boolean>; forbidden: Ref<boolean>; error: Ref<string>
  formatDate: (value?: string) => string
  showToast: (message: string) => void
}

export function useReportWorkflow(context: ReportContext) {
  function filters(): ReportFilters {
    return { from: context.from.value, to: context.to.value, accountId: context.accountId.value, query: context.query.value, status: context.status.value }
  }
  async function loadReport() {
    context.loading.value = true; context.error.value = ''; context.forbidden.value = false
    try {
      const { data, meta } = await fetchReport(filters(), context.page.value, context.pageSize)
      context.summary.value = data.summary
      context.total.value = Number(meta?.total ?? data.items.length)
      context.rows.value.splice(0, context.rows.value.length, ...data.items.map(item => ({
        id: item.id, batch: item.batch_no, name: item.remarks || item.bank_display_name, source: item.account_code,
        period: `${context.formatDate(item.period_from)} – ${context.formatDate(item.period_to)}`,
        total: `${item.internal_count} + ${item.bank_count}`,
        rate: `${(item.internal_count ? item.matched_pairs * 100 / item.internal_count : 0).toFixed(1)}%`,
        status: item.state, updated: context.formatDate(item.updated_at),
      })))
    } catch (error) {
      context.forbidden.value = error instanceof ApiError && error.status === 403
      context.error.value = error instanceof Error ? error.message : 'Laporan tidak dapat dimuat.'
    } finally { context.loading.value = false }
  }
  async function downloadReport() {
    try { await exportReport(filters(), context.exportType.value) }
    catch (error) { context.showToast(error instanceof Error ? error.message : 'Gagal mengunduh laporan.') }
  }
  return { loadReport, downloadReport }
}
