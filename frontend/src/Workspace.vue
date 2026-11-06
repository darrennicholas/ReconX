<script setup lang="ts">
import { navGroups, titles, subtitles, routeRoles, type Page } from './features/navigation/navigation'
import SettingsView from './features/settings/SettingsView.vue'
import RoleMatrix from './features/settings/RoleMatrix.vue'
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import DataTable from './components/DataTable.vue'
import GlobalDropdown from './components/GlobalDropdown.vue'
import ReportView from './features/reports/ReportView.vue'
import DashboardView from './features/dashboard/DashboardView.vue'
import ImportView from './features/imports/ImportView.vue'
import ExceptionDetail from './features/exceptions/ExceptionDetail.vue'
import BatchDetail from './features/batches/BatchDetail.vue'
import { fetchBatch } from './features/batches/batchApi'
import { useBatchWorkflow } from './features/batches/useBatchWorkflow'
import { fetchApproval, fetchException } from './features/exceptions/exceptionApi'
import { useExceptionActions } from './features/exceptions/useExceptionActions'
import { useImportWorkflow, importValidationMessage } from './features/imports/useImportWorkflow'
import MatchingBatchList from './features/matching/MatchingBatchList.vue'
import { useMatchingWorkflow } from './features/matching/useMatchingWorkflow'
import RecordDetail from './features/details/RecordDetail.vue'
import DetailDialog from './features/details/DetailDialog.vue'
import { useReportWorkflow } from './features/reports/useReportWorkflow'
import { ApiError, apiRequest, downloadFile, isActionLoading } from './api'
import { useSessionStore, type SessionUser } from './stores/session'

type Row = Record<string, string>

const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const { currentUser } = storeToRefs(session)
const page = computed<Page>(() => route.name === 'login' ? 'dashboard' : (route.name as Page) || 'dashboard')
const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')
const search = ref('')
const statusFilter = ref('All statuses')
const sourceFilter = ref('All sources')
const typeFilter = ref('All types')
const batchFilter = ref('')
const exceptionAccountFilter = ref('')
const exceptionAgeFilter = ref('')
const exceptionFiltersOpen = ref(false)
const assigneeFilter = ref('')
const auditEntityFilter = ref('')
const auditActorFilter = ref('')
const batchAccountFilter = ref('')
const transactionBatchFilter = ref('')
const userRoleFilter = ref('')
const filterFrom = ref('')
const filterTo = ref('')
const exceptionActiveFilterCount = computed(() => [typeFilter.value !== 'All types', !!batchFilter.value, !!exceptionAccountFilter.value, !!assigneeFilter.value, !!filterFrom.value, !!filterTo.value, !!exceptionAgeFilter.value].filter(Boolean).length)
const activeTab = ref('All')
const modal = ref('')
const detailPopup = ref<'batch' | 'exception' | ''>('')
const previousDetailPopup = ref<'batch' | 'exception' | ''>('')
const showLogoutConfirm = ref(false)
const successMessage = ref('')
const toast = ref('')
const showNotifications = ref(false)
const selectedFile = ref('')
const importError = ref('')
const importErrors = ref<{ line: number; field: string; message: string }[]>([])
const showImportError = ref(false)
const fileToUpload = ref<File | null>(null)
const importStage = ref(1)
const loginEmail = ref('')
const loginPassword = ref('')
const showLoginPassword = ref(false)
const loginError = ref('')
const busy = ref(false)
const matchingProgress = ref('')
const interactionLoading = ref(false)
const showGlobalLoading = ref(false)
let interactionTimer: number | undefined
let loadingHideTimer: number | undefined
let notificationTimer: number | undefined
let loadingStartedAt = 0
let openingBatchView = false
const loadingRequested = computed(() => busy.value || isActionLoading.value || interactionLoading.value)

watch(loadingRequested, active => {
  window.clearTimeout(loadingHideTimer)
  if (active) {
    if (!showGlobalLoading.value) loadingStartedAt = Date.now()
    showGlobalLoading.value = true
    return
  }
  const remaining = Math.max(0, 1000 - (Date.now() - loadingStartedAt))
  loadingHideTimer = window.setTimeout(() => { showGlobalLoading.value = false }, remaining)
}, { flush: 'sync' })

function startInteractionLoading(event: MouseEvent) {
  const target = event.target
  if (!(target instanceof Element)) return
  const button = target.closest<HTMLButtonElement>('button[data-loading]')
  if (!button || button.disabled || button.matches('[data-no-loading]')) return
  window.clearTimeout(interactionTimer)
  interactionLoading.value = true
  interactionTimer = window.setTimeout(() => { interactionLoading.value = false }, 1000)
}

function finishLoadingForOpenedView() {
  window.clearTimeout(interactionTimer)
  interactionLoading.value = false
  window.clearTimeout(loadingHideTimer)
  showGlobalLoading.value = false
  loadingStartedAt = 0
}

watch([modal, showImportError, showLogoutConfirm], ([currentModal, importErrorOpen, logoutOpen], [previousModal, previousImportError, previousLogout]) => {
  if ((currentModal && currentModal !== previousModal) || (importErrorOpen && !previousImportError) || (logoutOpen && !previousLogout)) {
    if (!openingBatchView) finishLoadingForOpenedView()
  }
}, { flush: 'sync' })

watch(() => route.fullPath, (current, previous) => {
  if (current !== previous && !openingBatchView) finishLoadingForOpenedView()
}, { flush: 'sync' })

onUnmounted(() => {
  window.clearTimeout(interactionTimer)
  window.clearTimeout(loadingHideTimer)
  window.clearInterval(notificationTimer)
  window.removeEventListener('reconx:notifications-changed', refreshNotifications)
  window.removeEventListener('keydown', handleDetailEscape)
})
const selectedBatchID = ref('')
const matchingBatchSearch = ref('')
const matchingBatchOptions = ref<Record<string, any>[]>([])
const matchingBatchPage = ref(1)
const matchingBatchTotal = ref(0)
const matchingBatchLoading = ref(false)
const matchingBatchError = ref('')
let matchingBatchRequest = 0
let listPageRequest = 0
const matchingBatchRows = computed<Row[]>(() => matchingBatchOptions.value.map(batch => ({ id: batch.id, batch: batch.batch_no, account: `${batch.account_code} · ${batch.bank_display_name}`, period: `${formatDate(batch.period_from)} – ${formatDate(batch.period_to)}`, status: batch.state })))
const selectedExceptionID = ref('')
const selectedApprovalID = ref('')
const selectedUserID = ref('')
const selectedAccountID = ref('')
const accountActiveForm = ref(true)
const selectedTransaction = ref<Record<string, any> | null>(null)
const transactionBatchLabel = ref('')
const resetPasswordValue = ref('')
const rawBatches = ref<Record<string, any>[]>([])
const rawAccounts = ref<Record<string, any>[]>([])
const rawUsers = ref<Record<string, any>[]>([])
const rawExceptions = ref<Record<string, any>[]>([])
const rawApprovals = ref<Record<string, any>[]>([])
const rawMatches = ref<Record<string, any>[]>([])
const rawImports = ref<Record<string, any>[]>([])
const notifications = ref<Record<string, any>[]>([])
const dashboardSummary = ref<Record<string, any>>({})
const dashboardError = ref('')
const dashboardForbidden = ref(false)
const dashboardLoading = ref(false)
const batchForm = ref({ account_id: '', period_from: new Date().toISOString().slice(0, 10), period_to: new Date().toISOString().slice(0, 10), remarks: '' })
const accountForm = ref({ account_code: '', bank_display_name: '', masked_account_no: '•••• 0000', account_holder_name: '' })
const userForm = ref({ full_name: '', email: '', password: '', role: 'OPERATOR' })
const userEditForm = ref({ full_name: '', email: '', role: 'OPERATOR' })
const proposalReason = ref('')
const decisionComment = ref('')
const importSource = ref('INTERNAL')
const timezoneSetting = ref('Asia/Jakarta')
const maxImportSetting = ref(10485760)
const csvDelimiterSetting = ref(',')
const investigationNote = ref('')
const evidenceFile = ref<File | null>(null)
const reportFrom = ref('')
const reportTo = ref('')
const reportAccountID = ref('')
const reportQuery = ref('')
const reportStatus = ref('')
const reportExportType = ref('batches')
const dashboardFrom = ref('')
const dashboardTo = ref('')
const dashboardAccountID = ref('')
const paginationTotal = ref(0)
const listLoading = ref(false)
const listError = ref('')
const listForbidden = ref(false)
const reportSummary = ref<Record<string, any>>({})
const reportLoading = ref(false)
const reportError = ref('')
const reportForbidden = ref(false)
const auditDetail = ref<Record<string, any> | null>(null)

const batchRows = ref<Row[]>([])
const transactionRows = ref<Row[]>([])
const exceptionRows = ref<Row[]>([])
const approvalRows = ref<Row[]>([])
const matchingRows = ref<Row[]>([])
const accountRows = ref<Row[]>([])
const userRows = ref<Row[]>([])
const auditRows = ref<Row[]>([])
const batchTransactionRows = ref<Row[]>([])
const batchExceptionRows = ref<Row[]>([])
const batchAuditRows = ref<Row[]>([])
const batchApprovalRows = ref<Row[]>([])
const batchTabs = computed(() => {
  const batch = rawBatches.value.find(item => item.id === selectedBatchID.value)
  if (currentUser.value?.role === 'OPERATOR' && batch && batch.created_by !== currentUser.value.id) return ['Ringkasan']
  const tabs = ['Ringkasan', 'Imports', 'Detail Transaksi', 'Matching Results', 'Exceptions', 'Approvals', 'Riwayat Aktivitas']
  return ['CHECKER', 'AUDITOR'].includes(currentUser.value?.role ?? '') ? tabs : tabs.filter(tab => tab !== 'Approvals')
})

const tableConfig: Partial<Record<Page, { columns: { key: string; label: string }[]; rows: Row[] }>> = {
  batches: { columns: [{ key: 'batch', label: 'No. Batch' }, { key: 'name', label: 'Nama Batch' }, { key: 'source', label: 'Sumber' }, { key: 'period', label: 'Periode' }, { key: 'total', label: 'Total Tx' }, { key: 'rate', label: 'Match Rate' }, { key: 'status', label: 'Status' }, { key: 'updated', label: 'Diperbarui' }], rows: batchRows.value },
  transactions: { columns: [{ key: 'ref', label: 'Ref. No.' }, { key: 'date', label: 'Tanggal' }, { key: 'source', label: 'Sumber' }, { key: 'description', label: 'Deskripsi' }, { key: 'debit', label: 'Debit' }, { key: 'credit', label: 'Kredit' }, { key: 'status', label: 'Status' }, { key: 'batch', label: 'Batch' }], rows: transactionRows.value },
  matching: { columns: [{ key: 'ref', label: 'Ref. No.' }, { key: 'date', label: 'Tanggal' }, { key: 'internal', label: 'Data Internal' }, { key: 'bank', label: 'Data Bank' }, { key: 'amount', label: 'Nominal' }, { key: 'rule', label: 'Rule' }, { key: 'status', label: 'Status' }], rows: matchingRows.value },
  exceptions: { columns: [{ key: 'ref', label: 'No. Ref.' }, { key: 'type', label: 'Tipe Selisih' }, { key: 'batch', label: 'Batch' }, { key: 'nominalA', label: 'Nominal A' }, { key: 'nominalB', label: 'Nominal B' }, { key: 'difference', label: 'Selisih' }, { key: 'owner', label: 'PIC' }, { key: 'status', label: 'Status' }], rows: exceptionRows.value },
  approvals: { columns: [{ key: 'request', label: 'No. Pengajuan' }, { key: 'type', label: 'Jenis' }, { key: 'batch', label: 'Batch' }, { key: 'amount', label: 'Nominal' }, { key: 'maker', label: 'Maker' }, { key: 'submitted', label: 'Diajukan' }, { key: 'status', label: 'Status' }], rows: approvalRows.value },
  accounts: { columns: [{ key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama Sumber' }, { key: 'type', label: 'Tipe' }, { key: 'account', label: 'Rekening' }, { key: 'currency', label: 'Mata Uang' }, { key: 'status', label: 'Status' }], rows: accountRows.value },
  users: { columns: [{ key: 'name', label: 'Nama' }, { key: 'email', label: 'Email' }, { key: 'role', label: 'Role' }, { key: 'lastActive', label: 'Aktif Terakhir' }, { key: 'status', label: 'Status' }], rows: userRows.value },
  audit: { columns: [{ key: 'time', label: 'Waktu' }, { key: 'actor', label: 'Pengguna' }, { key: 'activity', label: 'Aktivitas' }, { key: 'module', label: 'Modul' }, { key: 'detail', label: 'Detail' }, { key: 'status', label: 'Hasil' }], rows: auditRows.value },
}

const currentTitle = computed(() => titles[page.value])
const visibleNavGroups = computed(() => {
  const role = currentUser.value?.role
  return navGroups.map(group => ({
    ...group,
    items: group.items.filter(item => (routeRoles[item.id] as readonly string[]).includes(role ?? '')).map(item => {
      const count = item.id === 'batches' ? rawBatches.value.length
        : item.id === 'exceptions' ? rawExceptions.value.filter(exception => !['RESOLVED', 'ARCHIVED'].includes(exception.state)).length
          : item.id === 'approvals' ? rawApprovals.value.filter(approval => approval.state === 'PENDING').length
            : undefined
      return { ...item, count: count === undefined ? undefined : String(count) }
    }),
  })).filter(group => group.items.length)
})
const currentNavGroup = computed(() => visibleNavGroups.value.find(group => group.items.some(item => item.id === page.value))?.title ?? 'OPERATIONS')
const currentTable = computed(() => tableConfig[page.value])
const currentTableTabs = computed(() => {
  if (page.value === 'batches') return ['Semua', 'Aktif', 'Selesai', 'Dibatalkan']
  if (page.value === 'matching') return [`Matched (${paginationTotal.value})`, 'Semua']
  if (page.value === 'approvals') return ['Manual Matches', 'Batch Approvals', `Menunggu (${rawApprovals.value.filter(item => item.state === 'PENDING').length})`, 'Semua', 'Riwayat']
  if (page.value === 'exceptions') return [`Semua (${paginationTotal.value})`, 'Open', 'Investigating', 'Resolved']
  if (page.value === 'transactions') return ['Semua', 'Matched', 'Exception', 'Unmatched']
  if (page.value === 'accounts' || page.value === 'users') return ['Semua', 'Aktif', 'Nonaktif']
  return ['Semua']
})
const currentStatusOptions = computed(() => {
  if (page.value === 'batches') return ['DRAFT', 'READY', 'REVIEW', 'PENDING_APPROVAL', 'APPROVED', 'CLOSED', 'CANCELLED', 'FAILED']
  if (page.value === 'transactions') return ['MATCHED', 'EXCEPTION', 'UNMATCHED']
  if (page.value === 'matching') return ['AUTO_MATCHED', 'MANUAL_MATCHED']
  if (page.value === 'exceptions') return ['OPEN', 'INVESTIGATING', 'PENDING_ACTION_APPROVAL', 'RESOLVED', 'ARCHIVED']
  if (page.value === 'approvals') return ['PENDING', 'APPROVED', 'REJECTED', 'INVALIDATED']
  if (page.value === 'accounts' || page.value === 'users') return ['Aktif', 'Nonaktif']
  return []
})
const currentStatusOptionsForReport = ['DRAFT', 'READY', 'REVIEW', 'PENDING_APPROVAL', 'APPROVED', 'CLOSED', 'CANCELLED', 'FAILED']
const pageSize = 25
const currentPage = ref(1)
const paginatedRows = computed(() => filteredRows.value)
const pageCount = computed(() => Math.max(1, Math.ceil(paginationTotal.value / pageSize)))
const firstVisibleRow = computed(() => paginationTotal.value ? (currentPage.value - 1) * pageSize + 1 : 0)
const lastVisibleRow = computed(() => Math.min(currentPage.value * pageSize, paginationTotal.value))
watch([page, search, statusFilter, sourceFilter, activeTab, typeFilter, batchFilter, exceptionAccountFilter, exceptionAgeFilter, assigneeFilter, auditEntityFilter, auditActorFilter, batchAccountFilter, transactionBatchFilter, userRoleFilter, filterFrom, filterTo, currentPage], async () => {
  if (['batches', 'transactions', 'exceptions', 'audit', 'matching', 'approvals', 'accounts', 'users'].includes(page.value)) await loadCurrentPage()
})
watch(selectedBatchID, async () => { if (page.value === 'matching') { currentPage.value = 1; await loadCurrentPage() } })
watch(matchingBatchSearch, () => { matchingBatchPage.value = 1 })
watch([page, matchingBatchSearch, matchingBatchPage], async () => { if (page.value === 'matching') await loadMatchingBatchOptions() })
watch([page, reportFrom, reportTo, reportAccountID, reportQuery, reportStatus], async () => { if (page.value === 'reports') { currentPage.value = 1; await loadReport() } })
watch(currentPage, async () => { if (page.value === 'reports') await loadReport() })
watch([search, statusFilter, sourceFilter, activeTab, typeFilter, batchFilter, exceptionAccountFilter, exceptionAgeFilter, assigneeFilter, auditEntityFilter, auditActorFilter, batchAccountFilter, transactionBatchFilter, userRoleFilter, filterFrom, filterTo], () => { currentPage.value = 1 })
const filteredRows = computed(() => {
  const rows = currentTable.value?.rows ?? []
  const q = search.value.trim().toLowerCase()
  return rows.filter(row => {
    const matchesSearch = !q || Object.values(row).some(value => value.toLowerCase().includes(q))
    const matchesStatus = statusFilter.value === 'All statuses' || row.status === statusFilter.value
    const matchesSource = page.value === 'matching' || sourceFilter.value === 'All sources' || row.source === sourceFilter.value
    const tab = activeTab.value
    const matchesTab = page.value === 'approvals'
      ? (tab === 'Manual Matches' ? row.type === 'MANUAL_MATCH' : tab === 'Batch Approvals' ? row.type === 'BATCH_CLOSE' : tab.startsWith('Menunggu') ? row.status === 'PENDING' : tab === 'Riwayat' ? row.status !== 'PENDING' : true)
      : page.value === 'exceptions'
        ? (tab === 'All' || tab.startsWith('Semua') || row.status === tab.toUpperCase())
        : page.value === 'matching'
          ? (tab === 'All' || tab === 'Semua' || !tab.startsWith('Matched') || row.status.includes('MATCHED'))
          : page.value === 'transactions'
            ? (tab === 'All' || tab === 'Semua' || row.status === tab.toUpperCase())
          : page.value === 'batches'
            ? (tab === 'All' || tab.startsWith('Semua') || (tab === 'Aktif' && !['CLOSED', 'CANCELLED'].includes(row.status)) || (tab === 'Selesai' && row.status === 'CLOSED') || (tab === 'Dibatalkan' && row.status === 'CANCELLED'))
            : page.value === 'accounts' || page.value === 'users'
              ? (tab === 'All' || tab.startsWith('Semua') || row.status === tab)
              : true
    return matchesSearch && matchesStatus && matchesSource && matchesTab
  })
})
const pageAction = computed(() => {
  const role = currentUser.value?.role
  if ((page.value === 'dashboard' || page.value === 'batches') && role === 'OPERATOR') return 'Buat Batch'
  if (page.value === 'exceptions' && role === 'OPERATOR') return 'Investigasi'
  if (page.value === 'accounts' && role === 'ADMIN') return 'Tambah Sumber'
  if (page.value === 'users' && role === 'ADMIN') return 'Tambah Pengguna'
  return ''
})

onMounted(async () => {
  window.addEventListener('reconx:notifications-changed', refreshNotifications)
  window.addEventListener('keydown', handleDetailEscape)
  notificationTimer = window.setInterval(refreshNotifications, 15000)
  try {
    const response = await fetch('/api/v1/health')
    apiStatus.value = response.ok ? 'online' : 'offline'
    if (response.ok) {
      try {
        await session.load()
        await loadWorkspace()
      } catch { /* Login is required, or database setup has not been completed. */ }
    }
  } catch {
    apiStatus.value = 'offline'
  }
})

watch(() => [page.value, route.params.id] as const, async ([currentPage, routeID]) => {
  const id = typeof routeID === 'string' ? routeID : ''
  if (currentPage === 'batchDetail' && id) {
    selectedBatchID.value = id
    activeTab.value = 'Ringkasan'
    try {
      const { data } = await fetchBatch(id)
      const index = rawBatches.value.findIndex(batch => batch.id === id)
      if (index >= 0) rawBatches.value.splice(index, 1, data)
      else rawBatches.value.unshift(data)
      if (currentUser.value?.role !== 'OPERATOR' || data.created_by === currentUser.value.id) await Promise.all([loadMatches(), loadBatchDetails(), loadImports()])
      detailPopup.value = 'batch'
    } catch (error) { showToast(error instanceof Error ? error.message : 'Batch tidak dapat dibuka.') }
  } else if (currentPage === 'exceptionDetail' && id) {
    selectedExceptionID.value = id
    await loadExceptionDetail()
    if (exceptionDetail.value?.id === id) detailPopup.value = 'exception'
  }
}, { immediate: true })

async function loadWorkspace() {
  const dashboardParams = new URLSearchParams()
  if (dashboardFrom.value) dashboardParams.set('from', dashboardFrom.value)
  if (dashboardTo.value) dashboardParams.set('to', dashboardTo.value)
  if (dashboardAccountID.value) dashboardParams.set('account_id', dashboardAccountID.value)
  const results = await Promise.allSettled([
    apiRequest<Record<string, any>>(`/api/v1/dashboard/summary${dashboardParams.size ? `?${dashboardParams}` : ''}`),
    apiRequest<Record<string, any>[]>('/api/v1/batches?page_size=100'),
    apiRequest<Record<string, any>[]>('/api/v1/bank-accounts'),
    apiRequest<Record<string, any>[]>('/api/v1/transactions?page_size=100'),
    apiRequest<Record<string, any>[]>('/api/v1/exceptions?page_size=100'),
    apiRequest<Record<string, any>[]>('/api/v1/approvals'),
    apiRequest<Record<string, any>[]>('/api/v1/users'),
    apiRequest<Record<string, any>[]>('/api/v1/audit-logs?page_size=100'),
    apiRequest<Record<string, any>[]>('/api/v1/notifications?page_size=10'),
    apiRequest<Record<string, any>[]>('/api/v1/settings'),
  ])
  const value = <T>(index: number): T | null => results[index].status === 'fulfilled' ? results[index].value.data as T : null
  const summary = value<Record<string, any>>(0); if (summary) dashboardSummary.value = summary
  if (results[0].status === 'rejected') {
    const error = results[0].reason
    dashboardForbidden.value = error instanceof ApiError && error.status === 403
    dashboardError.value = error instanceof Error ? error.message : 'Dashboard tidak dapat dimuat.'
  } else { dashboardError.value = ''; dashboardForbidden.value = false }
  const batches = value<Record<string, any>[]>(1); if (batches) {
    const selected = (page.value === 'batchDetail' || detailPopup.value === 'batch') ? rawBatches.value.find(item => item.id === selectedBatchID.value) : undefined
    rawBatches.value = selected && !batches.some(item => item.id === selected.id) ? [selected, ...batches] : batches
    batchRows.value.splice(0, batchRows.value.length, ...batches.map((item: any) => ({ id: item.id, batch: item.batch_no, name: item.remarks || item.bank_display_name, source: item.account_code, period: `${formatDate(item.period_from)} – ${formatDate(item.period_to)}`, total: `${item.internal_count} + ${item.bank_count}`, rate: `${(item.internal_count ? item.matched_pairs * 100 / item.internal_count : 0).toFixed(1)}%`, status: item.state, updated: formatDate(item.updated_at) })))
    if (!selectedBatchID.value) selectedBatchID.value = (page.value === 'matching' ? batches.find(item => item.matched_pairs > 0) : undefined)?.id ?? batches[0]?.id ?? ''
  }
  const accounts = value<Record<string, any>[]>(2); if (accounts) {
    rawAccounts.value = accounts
    accountRows.value.splice(0, accountRows.value.length, ...accounts.map((item: any) => ({ id: item.id, code: item.account_code, name: item.bank_display_name, type: 'Rekening', account: item.masked_account_no, currency: item.currency, status: item.active ? 'Aktif' : 'Nonaktif' })))
  }
  const transactions = value<Record<string, any>[]>(3); if (transactions) transactionRows.value.splice(0, transactionRows.value.length, ...transactions.map((item: any) => ({ id: item.id, ref: item.reference_id, date: formatDate(item.date), source: item.source_type, description: item.direction, debit: item.direction === 'DEBIT' ? money(item.amount_rupiah) : '—', credit: item.direction === 'CREDIT' ? money(item.amount_rupiah) : '—', status: item.status, batch: rawBatches.value.find(batch => batch.id === item.batch_id)?.batch_no ?? item.batch_id })))
  const exceptions = value<Record<string, any>[]>(4); if (exceptions) {
    rawExceptions.value = exceptions
    exceptionRows.value.splice(0, exceptionRows.value.length, ...exceptions.map((item: any) => ({ id: item.id, ref: item.reference_id, type: item.type, batch: item.batch_no, nominalA: money(item.internal_amount_rupiah), nominalB: money(item.bank_amount_rupiah), difference: money(item.difference_rupiah), owner: item.assignee_name || 'Belum ditugaskan', status: item.state })))
  }
  const approvals = value<Record<string, any>[]>(5); if (approvals) {
    rawApprovals.value = approvals
    approvalRows.value.splice(0, approvalRows.value.length, ...approvals.map((item: any) => ({ id: item.id, request: `APR-${item.id.slice(0, 8)}`, type: item.kind, batch: item.batch_no, amount: item.internal ? money(item.internal.amount_rupiah) : '—', maker: item.maker_name, submitted: formatDate(item.created_at), status: item.state })))
  }
  const users = value<Record<string, any>[]>(6); if (users) {
    rawUsers.value = users
    userRows.value.splice(0, userRows.value.length, ...users.map((item: any) => ({ id: item.id, name: item.full_name, email: item.email, role: item.role, lastActive: '—', status: item.active ? 'Aktif' : 'Nonaktif' })))
  }
  const logs = value<Record<string, any>[]>(7); if (logs) auditRows.value.splice(0, auditRows.value.length, ...logs.map((item: any) => ({ time: formatDate(item.created_at), actor: item.actor, activity: item.action, module: item.entity, detail: item.entity_id, status: 'Success' })))
  const notices = value<Record<string, any>[]>(8); if (notices) notifications.value = notices
  const settings = value<Record<string, any>[]>(9); if (settings) {
    for (const item of settings) {
      if (item.key === 'timezone') timezoneSetting.value = item.value
      if (item.key === 'max_import_bytes') maxImportSetting.value = Number(item.value)
      if (item.key === 'csv_delimiter') csvDelimiterSetting.value = item.value
    }
  }
  await loadMatches()
  await loadBatchDetails()
  if (page.value === 'matching') await loadMatchingBatchOptions()
  if (['batches', 'transactions', 'exceptions', 'audit', 'matching', 'approvals'].includes(page.value)) await loadCurrentPage()
  if (page.value === 'reports') await loadReport()
}

function listParams() {
  const params = new URLSearchParams({ page: String(currentPage.value), page_size: String(pageSize) })
  if (search.value.trim()) params.set('q', search.value.trim())
  const selectedStatus = statusFilter.value !== 'All statuses' ? statusFilter.value : ''
  if (selectedStatus) params.set('status', selectedStatus)
  if (sourceFilter.value !== 'All sources') params.set('source', sourceFilter.value)
  if (filterFrom.value) params.set('from', filterFrom.value)
  if (filterTo.value) params.set('to', filterTo.value)
  if (page.value === 'batches' && batchAccountFilter.value) params.set('account_id', batchAccountFilter.value)
  if (page.value === 'transactions' && transactionBatchFilter.value) params.set('batch_id', transactionBatchFilter.value)
  if (page.value === 'accounts' || page.value === 'users') {
    const activeState = statusFilter.value !== 'All statuses' ? statusFilter.value : activeTab.value
    if (activeState === 'Aktif') params.set(page.value === 'accounts' ? 'active' : 'status', page.value === 'accounts' ? 'true' : 'ACTIVE')
    if (activeState === 'Nonaktif') params.set(page.value === 'accounts' ? 'active' : 'status', page.value === 'accounts' ? 'false' : 'INACTIVE')
    if (page.value === 'users' && userRoleFilter.value) params.set('role', userRoleFilter.value)
  }
  if (page.value === 'approvals') {
    const kinds: Record<string, string> = { 'Manual Matches': 'MANUAL_MATCH', 'Batch Approvals': 'BATCH_CLOSE' }
    const kind = kinds[activeTab.value]
    if (kind) params.set('kind', kind)
    if (!selectedStatus && activeTab.value.startsWith('Menunggu')) params.set('status', 'PENDING')
    if (!selectedStatus && activeTab.value === 'Riwayat') params.set('status', 'HISTORY')
  }
  if (page.value === 'batches' && activeTab.value === 'Aktif' && !selectedStatus) params.set('status', 'ACTIVE')
  if (page.value === 'transactions') {
    const tabStatus: Record<string, string> = { Matched: 'MATCHED', Exception: 'EXCEPTION', Unmatched: 'UNMATCHED' }
    if (tabStatus[activeTab.value] && !selectedStatus) params.set('status', tabStatus[activeTab.value])
  }
  if (page.value === 'exceptions') {
    if (typeFilter.value !== 'All types') params.set('type', typeFilter.value)
    if (batchFilter.value) params.set('batch_id', batchFilter.value)
    if (exceptionAccountFilter.value) params.set('account_id', exceptionAccountFilter.value)
    if (exceptionAgeFilter.value) params.set('age', exceptionAgeFilter.value)
    if (assigneeFilter.value) params.set('assignee_id', assigneeFilter.value)
    const tabStatus: Record<string, string> = { Open: 'OPEN', Investigating: 'INVESTIGATING', Resolved: 'RESOLVED' }
    if (tabStatus[activeTab.value] && !selectedStatus) params.set('status', tabStatus[activeTab.value])
  }
  if (page.value === 'audit') {
    if (auditEntityFilter.value) params.set('entity', auditEntityFilter.value)
    if (auditActorFilter.value.trim()) params.set('actor', auditActorFilter.value.trim())
  }
  return params
}

async function loadCurrentPage() {
  const requestID = ++listPageRequest
  const requestedPage = page.value
  const requestedBatchID = selectedBatchID.value
  listLoading.value = true
  listError.value = ''
  listForbidden.value = false
  try {
    const params = listParams()
    const endpoint = page.value === 'batches' ? '/api/v1/batches'
      : page.value === 'transactions' ? '/api/v1/transactions'
        : page.value === 'exceptions' ? '/api/v1/exceptions'
          : page.value === 'audit' ? '/api/v1/audit-logs'
            : page.value === 'approvals' ? '/api/v1/approvals'
              : page.value === 'accounts' ? '/api/v1/bank-accounts'
                : page.value === 'users' ? '/api/v1/users'
            : selectedBatchID.value ? `/api/v1/batches/${selectedBatchID.value}/matches` : ''
    if (!endpoint) { matchingRows.value.splice(0); paginationTotal.value = 0; return }
    const { data, meta } = await apiRequest<Record<string, any>[]>(`${endpoint}?${params}`)
    if (requestID !== listPageRequest || page.value !== requestedPage || (requestedPage === 'matching' && selectedBatchID.value !== requestedBatchID)) return
    paginationTotal.value = Number(meta?.total ?? data.length)
    if (page.value === 'batches') {
      batchRows.value.splice(0, batchRows.value.length, ...data.map(item => ({ id: item.id, batch: item.batch_no, name: item.remarks || item.bank_display_name, source: item.account_code, period: `${formatDate(item.period_from)} – ${formatDate(item.period_to)}`, total: `${item.internal_count} + ${item.bank_count}`, rate: `${(item.internal_count ? item.matched_pairs * 100 / item.internal_count : 0).toFixed(1)}%`, status: item.state, updated: formatDate(item.updated_at) })))
      rawBatches.value = data
      if (!selectedBatchID.value && data.length) selectedBatchID.value = data[0].id
    } else if (page.value === 'transactions') {
      transactionRows.value.splice(0, transactionRows.value.length, ...data.map(item => ({ id: item.id, ref: item.reference_id, date: formatDate(item.date), source: item.source_type, description: item.direction, debit: item.direction === 'DEBIT' ? money(item.amount_rupiah) : '—', credit: item.direction === 'CREDIT' ? money(item.amount_rupiah) : '—', status: item.status, batch: rawBatches.value.find(batch => batch.id === item.batch_id)?.batch_no ?? item.batch_id })))
    } else if (page.value === 'exceptions') {
      exceptionRows.value.splice(0, exceptionRows.value.length, ...data.map(item => ({ id: item.id, ref: item.reference_id, type: item.type, batch: item.batch_no, nominalA: money(item.internal_amount_rupiah), nominalB: money(item.bank_amount_rupiah), difference: money(item.difference_rupiah), owner: item.assignee_name || 'Belum ditugaskan', status: item.state })))
      rawExceptions.value = data
      const summaryParams = new URLSearchParams(params)
      summaryParams.delete('page'); summaryParams.delete('page_size'); summaryParams.delete('status')
      const counts = (await apiRequest<{ name: string; count: number }[]>(`/api/v1/exceptions/summary?${summaryParams}`)).data
      exceptionSummaryCounts.value = Object.fromEntries(counts.map(item => [item.name, item.count]))
    } else if (page.value === 'audit') {
      auditLogs.value = data
      auditRows.value.splice(0, auditRows.value.length, ...data.map(item => ({ id: item.id, time: formatDate(item.created_at), actor: item.actor, activity: item.action, module: item.entity, detail: item.entity_id, status: 'Success' })))
    } else if (page.value === 'approvals') {
      rawApprovals.value = data
      approvalRows.value.splice(0, approvalRows.value.length, ...data.map(item => ({ id: item.id, request: `APR-${item.id.slice(0, 8)}`, type: item.kind, batch: item.batch_no, amount: item.internal ? money(item.internal.amount_rupiah) : '—', maker: item.maker_name, submitted: formatDate(item.created_at), status: item.state })))
    } else if (page.value === 'accounts') {
      rawAccounts.value = data
      accountRows.value.splice(0, accountRows.value.length, ...data.map(item => ({ id: item.id, code: item.account_code, name: item.bank_display_name, type: 'Rekening', account: item.masked_account_no, currency: item.currency, status: item.active ? 'Aktif' : 'Nonaktif' })))
    } else if (page.value === 'users') {
      rawUsers.value = data
      userRows.value.splice(0, userRows.value.length, ...data.map(item => ({ id: item.id, name: item.full_name, email: item.email, role: item.role, lastActive: '—', status: item.active ? 'Aktif' : 'Nonaktif' })))
    } else {
      rawMatches.value = data
      matchingRows.value.splice(0, matchingRows.value.length, ...data.map(item => ({ id: item.id, ref: item.reference_id, date: formatDate(item.internal_date), internal: `${item.internal_direction} · ${money(item.internal_amount_rupiah)}`, bank: `${item.bank_direction} · ${money(item.bank_amount_rupiah)}`, amount: money(item.internal_amount_rupiah), rule: item.rule_id, status: item.method })))
    }
  } catch (error) {
    if (requestID !== listPageRequest || page.value !== requestedPage) return
    listForbidden.value = error instanceof ApiError && error.status === 403
    listError.value = error instanceof Error ? error.message : 'Data tidak dapat dimuat.'
  } finally { if (requestID === listPageRequest) listLoading.value = false }
}

async function loadMatchingBatchOptions() {
  const requestID = ++matchingBatchRequest
  matchingBatchLoading.value = true
  matchingBatchError.value = ''
  const params = new URLSearchParams({ page: String(matchingBatchPage.value), page_size: '25' })
  if (matchingBatchSearch.value.trim()) params.set('q', matchingBatchSearch.value.trim())
  try {
    const { data, meta } = await apiRequest<Record<string, any>[]>(`/api/v1/batches?${params}`)
    if (requestID !== matchingBatchRequest || page.value !== 'matching') return
    matchingBatchOptions.value = data
    matchingBatchTotal.value = Number(meta?.total ?? data.length)
    if (!selectedBatchID.value && data.length) selectedBatchID.value = (data.find(item => item.matched_pairs > 0) ?? data[0]).id
    const selected = rawBatches.value.find(batch => batch.id === selectedBatchID.value)
    rawBatches.value = selected && !data.some(batch => batch.id === selected.id) ? [selected, ...data] : data
  } catch (error) { if (requestID === matchingBatchRequest) matchingBatchError.value = error instanceof Error ? error.message : 'Daftar batch tidak dapat dimuat.' }
  finally { if (requestID === matchingBatchRequest) matchingBatchLoading.value = false }
}

const exceptionSummaryCounts = ref<Record<string, number>>({})
const auditLogs = ref<Record<string, any>[]>([])

const { loadReport, downloadReport } = useReportWorkflow({
  from: reportFrom, to: reportTo, accountId: reportAccountID, query: reportQuery, status: reportStatus, exportType: reportExportType,
  page: currentPage, pageSize, total: paginationTotal, rows: batchRows, summary: reportSummary,
  loading: reportLoading, forbidden: reportForbidden, error: reportError, formatDate, showToast,
})

async function loadDashboard() {
  if (dashboardFrom.value && dashboardTo.value && dashboardFrom.value > dashboardTo.value) {
    showToast('Tanggal mulai tidak boleh melewati tanggal akhir.')
    return
  }
  dashboardLoading.value = true
  const params = new URLSearchParams()
  if (dashboardFrom.value) params.set('from', dashboardFrom.value)
  if (dashboardTo.value) params.set('to', dashboardTo.value)
  if (dashboardAccountID.value) params.set('account_id', dashboardAccountID.value)
  try { dashboardSummary.value = (await apiRequest<Record<string, any>>(`/api/v1/dashboard/summary${params.size ? `?${params}` : ''}`)).data; dashboardError.value = ''; dashboardForbidden.value = false }
  catch (error) { dashboardForbidden.value = error instanceof ApiError && error.status === 403; dashboardError.value = error instanceof Error ? error.message : 'Ringkasan dashboard gagal dimuat.' }
  finally { dashboardLoading.value = false }
}

async function loadMatches() {
  if (!selectedBatchID.value) { matchingRows.value.splice(0); return }
  try {
    const params = new URLSearchParams({ page: String(page.value === 'matching' ? currentPage.value : 1), page_size: String(page.value === 'matching' ? pageSize : 100) })
    const { data, meta } = await apiRequest<Record<string, any>[]>(`/api/v1/batches/${selectedBatchID.value}/matches?${params}`)
    if (page.value === 'matching') paginationTotal.value = Number(meta?.total ?? data.length)
    rawMatches.value = data
    matchingRows.value.splice(0, matchingRows.value.length, ...data.map((item: any) => ({
      id: item.id, ref: item.reference_id, date: formatDate(item.internal_date),
      internal: `${item.internal_direction} · ${money(item.internal_amount_rupiah)}`,
      bank: `${item.bank_direction} · ${money(item.bank_amount_rupiah)}`,
      amount: money(item.internal_amount_rupiah), rule: item.rule_id, status: item.method,
    })))
  } catch { matchingRows.value.splice(0) }
}
async function loadImports() {
  if (!selectedBatchID.value) { rawImports.value = []; return }
  try { rawImports.value = (await apiRequest<Record<string, any>[]>(`/api/v1/batches/${selectedBatchID.value}/imports`)).data }
  catch { rawImports.value = [] }
}

async function loadBatchDetails() {
  if (!selectedBatchID.value) {
    batchTransactionRows.value = []
    batchExceptionRows.value = []
    batchAuditRows.value = []
    batchApprovalRows.value = []
    return
  }
  const batchID = encodeURIComponent(selectedBatchID.value)
  const requests = [
    apiRequest<Record<string, any>[]>(`/api/v1/transactions?batch_id=${batchID}&page_size=100`),
    apiRequest<Record<string, any>[]>(`/api/v1/exceptions?batch_id=${batchID}&page_size=100`),
    apiRequest<Record<string, any>[]>(`/api/v1/audit-logs?q=${batchID}&page_size=100`),
    apiRequest<Record<string, any>[]>(`/api/v1/batches/${batchID}/imports`),
    apiRequest<Record<string, any>[]>(`/api/v1/batches/${batchID}/matches`),
  ]
  if (['CHECKER', 'AUDITOR'].includes(currentUser.value?.role ?? '')) requests.push(apiRequest<Record<string, any>[]>('/api/v1/approvals'))
  const results = await Promise.allSettled(requests)
  const transactionResult = results[0]
  if (transactionResult.status === 'fulfilled') {
    batchTransactionRows.value = transactionResult.value.data.map((item: any) => ({
      id: item.id, ref: item.reference_id, date: formatDate(item.date), source: item.source_type,
      description: item.direction, debit: item.direction === 'DEBIT' ? money(item.amount_rupiah) : '—',
      credit: item.direction === 'CREDIT' ? money(item.amount_rupiah) : '—', status: item.status,
      batch: selectedBatch.value?.batch_no ?? batchID,
    }))
  } else batchTransactionRows.value = []
  const exceptionResult = results[1]
  if (exceptionResult.status === 'fulfilled') {
    batchExceptionRows.value = exceptionResult.value.data.map((item: any) => ({
      id: item.id, ref: item.reference_id, type: item.type, batch: item.batch_no,
      nominalA: money(item.internal_amount_rupiah), nominalB: money(item.bank_amount_rupiah),
      difference: money(item.difference_rupiah), owner: item.assignee_name || 'Belum ditugaskan', status: item.state,
    }))
  } else batchExceptionRows.value = []
  const importsResult = results[3]
  if (importsResult.status === 'fulfilled') rawImports.value = importsResult.value.data
  else rawImports.value = []
  const matchesResult = results[4]
  if (page.value !== 'matching' && matchesResult.status === 'fulfilled') {
    const data = matchesResult.value.data
    rawMatches.value = data
    matchingRows.value.splice(0, matchingRows.value.length, ...data.map((item: any) => ({
      id: item.id, ref: item.reference_id, date: formatDate(item.internal_date),
      internal: `${item.internal_direction} · ${money(item.internal_amount_rupiah)}`,
      bank: `${item.bank_direction} · ${money(item.bank_amount_rupiah)}`,
      amount: money(item.internal_amount_rupiah), rule: item.rule_id, status: item.method,
    })))
  } else if (page.value !== 'matching') matchingRows.value.splice(0)
  const auditResult = results[2]
  if (auditResult.status === 'fulfilled') {
    batchAuditRows.value = auditResult.value.data
      .filter((item: any) => item.entity_id === selectedBatchID.value)
      .map((item: any) => ({ time: formatDate(item.created_at), actor: item.actor, activity: item.action, module: item.entity, detail: item.entity_id, status: 'Success' }))
  } else batchAuditRows.value = []
  const approvalsResult = results[5]
  if (approvalsResult?.status === 'fulfilled') {
    batchApprovalRows.value = approvalsResult.value.data
      .filter((item: any) => item.batch_id === selectedBatchID.value)
      .map((item: any) => ({ id: item.id, request: `APR-${item.id.slice(0, 8)}`, type: item.kind, batch: item.batch_no, amount: item.internal ? money(item.internal.amount_rupiah) : '—', maker: item.maker_name, submitted: formatDate(item.created_at), status: item.state }))
  } else batchApprovalRows.value = []
}

function formatDate(value?: string) { return value ? new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeZone: 'Asia/Jakarta' }).format(new Date(value)) : '—' }
function money(value?: number | null) { return value == null ? '—' : new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(value) }
function showToast(message: string) { toast.value = message; window.setTimeout(() => (toast.value = ''), 3200) }
function showSuccess(message: string) {
  successMessage.value = message
  finishLoadingForOpenedView()
}
async function login() {
  busy.value = true; loginError.value = ''
  try {
    const { data } = await apiRequest<SessionUser>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email: loginEmail.value, password: loginPassword.value }) })
    session.setUser(data); loginPassword.value = ''; await loadWorkspace()
    if (route.name === 'login') await router.replace({ name: 'dashboard' })
  } catch (error) { loginError.value = error instanceof Error ? error.message : 'Login gagal.' }
  finally { busy.value = false }
}

async function logout() {
  showLogoutConfirm.value = false
  try { await apiRequest('/api/v1/auth/logout', { method: 'POST' }) } catch { /* Session may already be expired. */ }
  session.clearUser(); notifications.value = []; selectedBatchID.value = ''; selectedExceptionID.value = ''; exceptionDetail.value = null; modal.value = ''
  await router.replace({ name: 'login' })
}

async function refreshNotifications() {
  const userID = currentUser.value?.id
  if (!userID) return
  try {
    const { data } = await apiRequest<Record<string, any>[]>('/api/v1/notifications?page_size=10')
    if (currentUser.value?.id === userID) notifications.value = data
  } catch { /* A later refresh will retry if the server is unavailable. */ }
}

function navigate(id: string) {
  detailPopup.value = ''
  previousDetailPopup.value = ''
  const params: Record<string, string> = {}
  if (id === 'batchDetail' && selectedBatchID.value) params.id = selectedBatchID.value
  if (id === 'exceptionDetail' && selectedExceptionID.value) params.id = selectedExceptionID.value
  const navigation = router.push({ name: id, params })
  search.value = ''
  statusFilter.value = 'All statuses'
  activeTab.value = id === 'batchDetail' ? 'Ringkasan' : id === 'settings' ? 'Umum' : id === 'approvals' ? 'Menunggu' : 'All'
  if (id === 'imports' && !selectedBatchID.value && rawBatches.value.length) selectedBatchID.value = rawBatches.value[0].id
  if (id === 'imports') void loadImports()
  return navigation
}

function openFilePicker() {
  document.querySelector<HTMLInputElement>('#file-input')?.click()
}

async function openTableRow(row?: Row) {
  if (page.value === 'batches' || page.value === 'dashboard' || page.value === 'reports') { if (row?.id) await openBatchPopup(row.id) }
  else if (page.value === 'transactions' && row?.id) await openTransaction(row)
  else if (page.value === 'exceptions' && row?.id) await openExceptionPopup(row.id)
  else if (page.value === 'approvals' && row?.id) await openApproval(row.id)
  else if (page.value === 'audit' && row?.id) { auditDetail.value = auditLogs.value.find(item => item.id === row.id) ?? null; if (auditDetail.value) modal.value = 'auditDetail' }
  else if (page.value === 'users' && row?.id) {
    selectedUserID.value = row.id; resetPasswordValue.value = ''
    const user = rawUsers.value.find(item => item.id === row.id)
    if (user) userEditForm.value = { full_name: user.full_name, email: user.email, role: user.role }
    modal.value = 'userActions'
  } else if (page.value === 'accounts' && row?.id) {
    const account = rawAccounts.value.find(item => item.id === row.id)
    if (!account) return
    selectedAccountID.value = account.id
    accountForm.value = { account_code: account.account_code, bank_display_name: account.bank_display_name, masked_account_no: account.masked_account_no, account_holder_name: account.account_holder_name }
    accountActiveForm.value = account.active
    modal.value = currentUser.value?.role === 'ADMIN' ? 'accountEdit' : 'accountView'
  }
}
async function openBatchPopup(id: string) {
  try {
    const { data } = await fetchBatch(id)
    const index = rawBatches.value.findIndex(batch => batch.id === id)
    if (index >= 0) rawBatches.value.splice(index, 1, data); else rawBatches.value.unshift(data)
    selectedBatchID.value = id
    activeTab.value = 'Ringkasan'
    previousDetailPopup.value = detailPopup.value
    if (currentUser.value?.role !== 'OPERATOR' || data.created_by === currentUser.value.id) await Promise.all([loadBatchDetails(), loadImports()])
    detailPopup.value = 'batch'
    await nextTick()
    finishLoadingForOpenedView()
  } catch (error) { showToast(error instanceof Error ? error.message : 'Batch tidak dapat dibuka.') }
}
async function openMatchingBatchPopup(id: string) {
  selectedBatchID.value = id
  await loadMatches()
  await openBatchPopup(id)
  if (detailPopup.value === 'batch') activeTab.value = 'Matching Results'
}
async function openExceptionPopup(id: string) {
  selectedExceptionID.value = id
  previousDetailPopup.value = detailPopup.value
  await loadExceptionDetail()
  if (exceptionDetail.value?.id === id) { detailPopup.value = 'exception'; await nextTick(); finishLoadingForOpenedView() }
}
function handleDetailEscape(event: KeyboardEvent) {
  if (event.key !== 'Escape' || successMessage.value || showImportError.value || showLogoutConfirm.value) return
  if (modal.value) modal.value = ''
  else if (detailPopup.value) closeDetailPopup()
}
function closeDetailPopup() {
  const previous = previousDetailPopup.value
  detailPopup.value = previous
  previousDetailPopup.value = ''
  if (!previous && page.value === 'batchDetail') void navigate('batches')
  else if (!previous && page.value === 'exceptionDetail') void navigate('exceptions')
}
async function openTransaction(row: Row) {
  if (!row.id) return
  try {
    selectedTransaction.value = (await apiRequest<Record<string, any>>(`/api/v1/transactions/${row.id}`)).data
    const batchID = selectedTransaction.value?.batch_id
    transactionBatchLabel.value = rawBatches.value.find(batch => batch.id === batchID)?.batch_no || ''
    if (!transactionBatchLabel.value && batchID) {
      try { transactionBatchLabel.value = (await apiRequest<Record<string, any>>(`/api/v1/batches/${batchID}`)).data.batch_no || '' } catch { /* Batch details may be unavailable for this role. */ }
    }
    modal.value = 'transaction'
  } catch (error) { showToast(error instanceof Error ? error.message : 'Detail transaksi tidak dapat dibuka.') }
}

async function runAction() {
  if (page.value === 'imports') {
    openFilePicker()
    return
  }
  if (page.value === 'reports') {
    downloadReport()
    return
  }
  if (page.value === 'exceptions') {
    const first = rawExceptions.value[0]?.id
    if (first) await openExceptionPopup(first)
    else showToast('Belum ada exception untuk diperiksa.')
    return
  }
  if (page.value === 'approvals') {
    toast.value = 'Pilih permintaan dalam senarai untuk semakan.'
    window.setTimeout(() => (toast.value = ''), 2800)
    return
  }
  modal.value = page.value === 'accounts' ? 'account' : page.value === 'users' ? 'user' : 'batch'
}

const { onFile, clearImportFile, changeImportBatch, uploadImport } = useImportWorkflow({
  selectedBatchId: selectedBatchID, source: importSource, selectedFile, file: fileToUpload, stage: importStage,
  busy, error: importError, errors: importErrors, showError: showImportError, loadImports, loadWorkspace, showSuccess,
})

async function downloadImportErrors(item: Record<string, any>) {
  try { await downloadFile(`/api/v1/imports/${item.id}/errors`, `${item.original_name}.errors.csv`) }
  catch (error) { showToast(error instanceof Error ? error.message : 'Laporan error tidak dapat diunduh.') }
}
async function downloadOriginalImport(item: Record<string, any>) {
  try { await downloadFile(`/api/v1/imports/${item.id}/file`, item.original_name || `import-${item.id}.csv`) }
  catch (error) { showToast(error instanceof Error ? error.message : 'File sumber tidak dapat diunduh.') }
}

async function submitModal() {
  busy.value = true
  try {
    if (modal.value === 'editBatch') {
      if (!selectedBatchID.value) throw new Error('Batch tidak ditemukan.')
      const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${selectedBatchID.value}`)
      await apiRequest(`/api/v1/batches/${selectedBatchID.value}`, { method: 'PATCH', headers: { 'If-Match': `"${batch.row_version}"` }, body: JSON.stringify(batchForm.value) })
    } else if (modal.value === 'batch') {
      const result = await apiRequest<Record<string, any>>('/api/v1/batches', { method: 'POST', idempotent: true, body: JSON.stringify(batchForm.value) })
      selectedBatchID.value = result.data.id
    } else if (modal.value === 'account') {
      await apiRequest('/api/v1/bank-accounts', { method: 'POST', idempotent: true, body: JSON.stringify(accountForm.value) })
      accountForm.value = { account_code: '', bank_display_name: '', masked_account_no: '•••• 0000', account_holder_name: '' }
    } else if (modal.value === 'accountEdit') {
      const current = rawAccounts.value.find(item => item.id === selectedAccountID.value)
      if (!current) throw new Error('Rekening tidak ditemukan.')
      if (current.active && !accountActiveForm.value && !window.confirm(`Nonaktifkan ${current.account_code}?`)) return
      await apiRequest(`/api/v1/bank-accounts/${selectedAccountID.value}`, { method: 'PATCH', body: JSON.stringify({ ...accountForm.value, active: accountActiveForm.value }) })
    } else if (modal.value === 'user') {
      await apiRequest('/api/v1/users', { method: 'POST', idempotent: true, body: JSON.stringify(userForm.value) })
      userForm.value = { full_name: '', email: '', password: '', role: 'OPERATOR' }
    } else if (modal.value === 'proposal') {
      if (!selectedExceptionID.value) throw new Error('Pilih exception terlebih dahulu.')
      await apiRequest(`/api/v1/exceptions/${selectedExceptionID.value}/manual-match-proposals`, { method: 'POST', idempotent: true, body: JSON.stringify({ reason: proposalReason.value }) })
      proposalReason.value = ''
    } else if (modal.value === 'approval') {
      if (!selectedApprovalID.value) throw new Error('Pilih approval terlebih dahulu.')
      await decideApproval('approve')
    }
    modal.value = ''
    await loadWorkspace()
    showSuccess('Perubahan berhasil disimpan.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Permintaan gagal.') }
  finally { busy.value = false }
}

const { runMatching } = useMatchingWorkflow({ selectedBatchId: selectedBatchID, busy, progress: matchingProgress, loadWorkspace, loadMatchingBatchOptions, showToast, showSuccess })

const exceptionDetail = ref<Record<string, any> | null>(null)
const exceptionOperators = ref<{ id: string; full_name: string }[]>([])
const operatorSearch = ref('')
const assignmentTarget = ref('')
let operatorRequest = 0
const approvalDetail = computed(() => rawApprovals.value.find(item => item.id === selectedApprovalID.value) ?? null)
const selectedBatch = computed(() => rawBatches.value.find(item => item.id === selectedBatchID.value) ?? null)
const selectedUser = computed(() => rawUsers.value.find(item => item.id === selectedUserID.value) ?? null)
const exceptionTypeCounts = computed(() => exceptionSummaryCounts.value)
async function loadExceptionDetail() {
  if (!selectedExceptionID.value) { exceptionDetail.value = null; return }
  try {
    exceptionDetail.value = (await fetchException(selectedExceptionID.value)).data
    assignmentTarget.value = exceptionDetail.value?.assigned_to || currentUser.value?.id || ''
    if (exceptionDetail.value?.can_assign_others && currentUser.value?.role === 'OPERATOR') await loadExceptionOperators()
  } catch (error) { exceptionDetail.value = null; showToast(error instanceof Error ? error.message : 'Exception tidak dapat dibuka.') }
}
async function loadExceptionOperators() {
  const request = ++operatorRequest
  const params = new URLSearchParams({ page_size: '100' })
  if (operatorSearch.value.trim()) params.set('q', operatorSearch.value.trim())
  try { const { data } = await apiRequest<{ id: string; full_name: string }[]>(`/api/v1/exception-operators?${params}`); if (request === operatorRequest) exceptionOperators.value = data }
  catch (error) { if (request === operatorRequest) showToast(error instanceof Error ? error.message : 'Daftar Operator tidak dapat dimuat.') }
}
watch(operatorSearch, () => { if ((page.value === 'exceptionDetail' || detailPopup.value === 'exception') && exceptionDetail.value?.can_assign_others) { assignmentTarget.value = ''; void loadExceptionOperators() } })
async function openBatchException(row: Row) {
  if (row.id) await openExceptionPopup(row.id)
}
async function openExceptionBatch() {
  const batchID = exceptionDetail.value?.batch_id
  if (!batchID) return
  try {
    await openBatchPopup(batchID)
  } catch (error) { showToast(error instanceof Error ? error.message : 'Batch tidak dapat dibuka.') }
}
async function openApproval(id: string) {
  try {
    const { data } = await fetchApproval(id)
    const index = rawApprovals.value.findIndex(item => item.id === id)
    if (index >= 0) rawApprovals.value.splice(index, 1, data); else rawApprovals.value.unshift(data)
    selectedApprovalID.value = id
    decisionComment.value = ''
    modal.value = 'approval'
  } catch (error) { showToast(error instanceof Error ? error.message : 'Approval tidak dapat dibuka.') }
}
async function openBatchApproval(row: Row) { if (row.id) await openApproval(row.id) }
const { assignException, addExceptionNote, onEvidenceFile, chooseEvidenceFile, uploadExceptionEvidence, downloadExceptionEvidence } = useExceptionActions({
  id: selectedExceptionID, assignmentTarget, currentUser, note: investigationNote, evidenceFile, detailPopup,
  loadWorkspace, loadDetail: loadExceptionDetail, showSuccess, showToast,
})
const { submitBatch, closeBatch, transitionBatch } = useBatchWorkflow({ selectedBatchId: selectedBatchID, busy, loadWorkspace, showToast, showSuccess })

function editBatch() {
  const batch = selectedBatch.value
  if (!batch) return
  batchForm.value = { account_id: batch.account_id, period_from: String(batch.period_from).slice(0, 10), period_to: String(batch.period_to).slice(0, 10), remarks: batch.remarks || '' }
  modal.value = 'editBatch'
}
async function showBatchViewLoading() {
  openingBatchView = true
  window.clearTimeout(interactionTimer)
  interactionLoading.value = true
  await nextTick()
  await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
}
async function openBatchEditor() {
  if (openingBatchView) return
  try {
    await showBatchViewLoading()
    if (!selectedBatch.value) { showToast('Data batch tidak ditemukan.'); return }
    editBatch()
    await nextTick()
  } finally {
    openingBatchView = false
    finishLoadingForOpenedView()
  }
}
async function openMatchingResults() {
  detailPopup.value = ''
  previousDetailPopup.value = ''
  if (openingBatchView) return
  try {
    await showBatchViewLoading()
    await navigate('matching')
    if (page.value === 'matching') await Promise.all([loadMatchingBatchOptions(), loadCurrentPage()])
    await nextTick()
  } catch (error) {
    showToast(error instanceof Error ? error.message : 'Hasil matching tidak dapat dibuka.')
  } finally {
    openingBatchView = false
    finishLoadingForOpenedView()
  }
}
async function saveSettings() {
  try { await apiRequest('/api/v1/settings', { method: 'PATCH', body: JSON.stringify({ timezone: timezoneSetting.value, max_import_bytes: Number(maxImportSetting.value), csv_delimiter: csvDelimiterSetting.value }) }); showSuccess('Pengaturan berhasil disimpan.') }
  catch (error) { showToast(error instanceof Error ? error.message : 'Pengaturan gagal disimpan.') }
}
async function toggleUserActive() {
  const user = selectedUser.value
  if (!user) return
  try {
    await apiRequest(`/api/v1/users/${user.id}`, { method: 'PATCH', body: JSON.stringify({ full_name: user.full_name, email: user.email, role: user.role, active: !user.active }) })
    modal.value = ''; await loadWorkspace(); showSuccess(`${user.full_name} ${user.active ? 'dinonaktifkan' : 'diaktifkan'}.`)
  } catch (error) { showToast(error instanceof Error ? error.message : 'Status user gagal diubah.') }
}
async function saveUserChanges() {
  const user = selectedUser.value
  if (!user) return
  try {
    await apiRequest(`/api/v1/users/${user.id}`, { method: 'PATCH', body: JSON.stringify({ ...userEditForm.value, active: user.active }) })
    modal.value = ''; await loadWorkspace(); showSuccess('Role dan data pengguna diperbarui.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Perubahan pengguna gagal disimpan.') }
}
async function resetUserPassword() {
  if (!selectedUserID.value || resetPasswordValue.value.length < 12) { showToast('Password minimal 12 karakter.'); return }
  try {
    await apiRequest(`/api/v1/users/${selectedUserID.value}/reset-password`, { method: 'POST', body: JSON.stringify({ password: resetPasswordValue.value }) })
    modal.value = ''; resetPasswordValue.value = ''; showSuccess('Password direset dan sesi user lama dicabut.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Password gagal direset.') }
}
async function decideApproval(decision: 'approve' | 'reject') {
  const comment = decisionComment.value.trim()
  if (decision === 'reject' && !comment) { showToast('Komentar wajib diisi untuk penolakan.'); return }
  busy.value = true
  try {
    await apiRequest(`/api/v1/approvals/${selectedApprovalID.value}/${decision}`, { method: 'POST', idempotent: true, body: JSON.stringify({ comment }) })
    modal.value = ''; decisionComment.value = ''; await loadWorkspace(); showSuccess(decision === 'approve' ? 'Approval disetujui.' : 'Approval ditolak.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Keputusan approval gagal disimpan.') }
  finally { busy.value = false }
}
function markNotificationsRead() {
  for (const item of notifications.value.filter(item => !item.read_at)) {
    void apiRequest(`/api/v1/notifications/${item.id}/read`, { method: 'PATCH', body: '{}' })
    item.read_at = new Date().toISOString()
  }
}
async function openNotification(item: Record<string, any>) {
  if (!item.read_at) {
    try { await apiRequest(`/api/v1/notifications/${item.id}/read`, { method: 'PATCH', body: '{}' }); item.read_at = new Date().toISOString() }
    catch (error) { showToast(error instanceof Error ? error.message : 'Status notifikasi gagal diperbarui.') }
  }
  showNotifications.value = false
  if (item.target_entity === 'batch') {
    try {
      const { data } = await apiRequest<Record<string, any>>(`/api/v1/batches/${item.target_entity_id}`)
      const index = rawBatches.value.findIndex(batch => batch.id === data.id)
      if (index >= 0) rawBatches.value.splice(index, 1, data)
      else rawBatches.value.unshift(data)
    } catch (error) { showToast(error instanceof Error ? error.message : 'Batch tidak dapat dibuka.'); return }
    selectedBatchID.value = item.target_entity_id
    navigate('batchDetail')
    await Promise.all([loadMatches(), loadBatchDetails()])
  } else if (item.target_entity === 'exception') {
    selectedExceptionID.value = item.target_entity_id
    navigate('exceptionDetail')
    await loadExceptionDetail()
  } else if (item.target_entity === 'approval') {
    if (['CHECKER', 'AUDITOR'].includes(currentUser.value?.role ?? '')) {
      await openApproval(item.target_entity_id)
    } else {
      navigate('batches')
    }
  } else if (item.target_entity === 'bank_account') {
    navigate('accounts')
  } else if (item.target_entity === 'user' && currentUser.value?.role === 'ADMIN') {
    navigate('users')
  } else if (item.target_entity === 'settings' && ['ADMIN', 'CHECKER', 'AUDITOR'].includes(currentUser.value?.role ?? '')) {
    navigate('settings')
  }
}


</script>

<template>
  <div class="app-shell" @click.capture="startInteractionLoading">
    <template v-if="!currentUser">
      <main class="login-shell">
        <section class="login-card">
          <div class="login-brand"><span class="brand-symbol"><i></i><b></b></span><span>Recon<span class="brand-x">X</span></span></div>
          <p class="eyebrow">RECONCILIATION WORKSPACE</p>
          <h1>Masuk ke ReconX</h1>
          <p class="login-copy">Gunakan akun Admin yang disiapkan melalui perintah seed lokal.</p>
          <form @submit.prevent="login">
            <label class="field-label">Email<input v-model="loginEmail" class="form-control" type="email" autocomplete="username" required placeholder="admin@perusahaan.co.id" /></label>
            <label class="field-label">Password<div class="password-input-wrap"><input v-model="loginPassword" class="form-control" :type="showLoginPassword ? 'text' : 'password'" autocomplete="current-password" required placeholder="Password akun" /><button type="button" class="password-visibility-button" :aria-label="showLoginPassword ? 'Sembunyikan password' : 'Lihat password'" :title="showLoginPassword ? 'Sembunyikan password' : 'Lihat password'" @click="showLoginPassword = !showLoginPassword"><svg v-if="!showLoginPassword" viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z"/><circle cx="12" cy="12" r="2.5"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="m3 3 18 18M10.6 6.2A10.8 10.8 0 0 1 12 6c6 0 9.5 6 9.5 6a16 16 0 0 1-2.7 3.4M6.2 6.2C3.8 7.8 2.5 12 2.5 12s3.5 6 9.5 6c1.1 0 2.1-.2 3-.5"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/></svg></button></div></label>
            <p v-if="loginError" class="login-error">{{ loginError }}</p>
            <button class="button primary login-button" :disabled="busy || apiStatus !== 'online'">{{ busy ? 'Memproses…' : 'Masuk' }}</button>
          </form>
          <p v-if="apiStatus === 'offline'" class="login-error">Layanan belum tersedia. Periksa koneksi backend.</p>
        </section>
      </main>
    </template>
    <template v-else>
    <aside class="sidebar">
      <button class="brand" aria-label="ReconX dashboard" @click="navigate('dashboard')">
        <span class="brand-symbol"><i></i><b></b></span><span>Recon<span class="brand-x">X</span></span>
      </button>
      <div class="workspace-switch"><span class="workspace-avatar">R</span><span class="workspace-name">ReconX Workspace<small>Development</small></span><span class="chevron">⌄</span></div>
      <nav class="navigation" aria-label="Navigasi utama">
        <section v-for="group in visibleNavGroups" :key="group.title" class="nav-group">
          <p class="nav-heading">{{ group.title }}</p>
          <button v-for="item in group.items" :key="item.id" class="nav-item" :class="{ active: page === item.id }" @click="navigate(item.id)">
            <svg viewBox="0 0 24 24" aria-hidden="true"><template v-if="item.icon === 'grid'"><rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.5"/></template><template v-else-if="item.icon === 'layers'"><path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5M3 16l9 5 9-5"/></template><template v-else-if="item.icon === 'upload'"><path d="M12 16V4m0 0L7 9m5-5 5 5"/><path d="M4 15v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4"/></template><template v-else-if="item.icon === 'rows'"><path d="M4 5h16M4 10h16M4 15h16M4 20h16"/><path d="M2 5h.01M2 10h.01M2 15h.01M2 20h.01"/></template><template v-else-if="item.icon === 'shuffle'"><path d="m16 3 4 4-4 4M4 7h3c5 0 5 10 10 10h3M16 13l4 4-4 4M4 17h3c1.5 0 2.5-.9 3.5-2"/></template><template v-else-if="item.icon === 'alert'"><path d="M10.3 3.9 2.8 17a2 2 0 0 0 1.7 3h15a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4m0 4h.01"/></template><template v-else-if="item.icon === 'check'"><path d="M9 12.5 11.5 15 16 9.5"/><circle cx="12" cy="12" r="9"/></template><template v-else-if="item.icon === 'chart'"><path d="M4 19V5m0 14h17"/><path d="m7 15 4-4 3 2 6-7"/></template><template v-else-if="item.icon === 'bank'"><path d="m3 9 9-6 9 6"/><path d="M4 10h16M5 10v9m4-9v9m6-9v9m4-9v9M3 21h18"/></template><template v-else-if="item.icon === 'users'"><circle cx="9" cy="8" r="3"/><path d="M3 20v-1a6 6 0 0 1 12 0v1M16 5.5a3 3 0 0 1 0 5.8M18 14a5 5 0 0 1 3 4.5V20"/></template><template v-else-if="item.icon === 'history'"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5m4-1v5l3 2"/></template><template v-else><circle cx="12" cy="12" r="3"/><path d="m19.4 15 .1.1 1.1.9-1 1.7-1.4-.5a8 8 0 0 1-1.5.9l-.3 1.5h-2l-.4-1.5a8 8 0 0 1-1.6-.9l-1.4.5-1-1.7 1.2-1a7 7 0 0 1 0-1.8l-1.2-1 1-1.7 1.4.5a8 8 0 0 1 1.6-.9l.4-1.5h2l.3 1.5a8 8 0 0 1 1.5.9l1.4-.5 1 1.7-1.1.9a7 7 0 0 1 0 1.9Z" transform="translate(-2 -2)"/></template></svg>
            <span>{{ item.label }}</span>
          </button>
        </section>
      </nav>
      <div class="sidebar-bottom"><div class="user-avatar">{{ currentUser.full_name.slice(0, 2).toUpperCase() }}</div><button class="user-info user-info-button" data-loading @click="modal = 'profile'"><strong>{{ currentUser.full_name }}</strong><small>{{ currentUser.role }}</small></button><button class="more-button" aria-label="Keluar" title="Keluar" data-no-loading @click="showLogoutConfirm = true">↪</button></div>
    </aside>

    <main class="main-area">
      <header class="topbar">
        <div class="crumb"><span>{{ currentNavGroup }}</span><span class="crumb-slash">/</span><strong>{{ currentTitle }}</strong></div>
        <div class="topbar-actions">
          <div class="top-search"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" aria-label="Cari" placeholder="Cari apa saja..." /></div>
          <div class="notification-wrap"><button class="top-icon" aria-label="Notifikasi" @click="showNotifications = !showNotifications"><svg viewBox="0 0 24 24"><path d="M18 9a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></svg><i v-if="notifications.some(item => !item.read_at)"></i></button><div v-if="showNotifications" class="notification-popover"><strong>Notifikasi</strong><button v-for="item in notifications" :key="item.id" class="notification-item" :class="{ unread: !item.read_at }" @click="openNotification(item)"><span class="notice-dot" :class="{ 'amber-dot': item.type.includes('EXCEPTION') }"></span> {{ item.title }} · {{ item.body }}</button><p v-if="!notifications.length">Belum ada notifikasi.</p></div></div>
          <span class="top-divider"></span><button class="user-avatar small-avatar account-icon-button" aria-label="Menu logout" data-no-loading @click="showLogoutConfirm = true">{{ currentUser.full_name.slice(0, 2).toUpperCase() }}</button><button class="profile-button" data-loading @click="modal = 'profile'"><span class="profile-name">{{ currentUser.full_name }}<small>{{ currentUser.role }}</small></span><span class="chevron">⌄</span></button>
        </div>
      </header>

      <div class="content-area">
        <div class="page-header"><div><div class="breadcrumb-mobile">Workspace <span>/</span> {{ currentTitle }}</div><h1>{{ currentTitle }}</h1><p>{{ subtitles[page] }}</p></div><div class="header-actions"><div v-if="page === 'dashboard'" class="dashboard-filter-controls"><input v-model="dashboardFrom" aria-label="Dari tanggal" type="date" /><input v-model="dashboardTo" aria-label="Sampai tanggal" type="date" /><GlobalDropdown v-model="dashboardAccountID" aria-label="Rekening"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }}</option></GlobalDropdown><button class="button secondary" data-loading @click="loadDashboard">Terapkan</button></div><button v-if="pageAction" class="button primary" data-loading @click="runAction"><span class="button-plus">＋</span>{{ pageAction }}</button></div></div>

        <DashboardView v-if="page === 'dashboard'" :summary="dashboardSummary" :loading="dashboardLoading" :forbidden="dashboardForbidden" :error="dashboardError" :rows="batchRows" :columns="tableConfig.batches!.columns" @reload="loadDashboard" @view-batches="navigate('batches')" @open="openTableRow" />

        <ImportView v-else-if="page === 'imports'" v-model:source="importSource" v-model:batch-id="selectedBatchID" :import-stage="importStage" :selected-file="selectedFile" :batches="rawBatches" :imports="rawImports" :file-selected="!!fileToUpload" :busy="busy" @pick="openFilePicker" @change-batch="changeImportBatch" @file="onFile" @clear="clearImportFile" @upload="uploadImport" @download="downloadOriginalImport" @download-errors="downloadImportErrors" />

        <div v-else-if="page === 'batchDetail' || page === 'exceptionDetail'" aria-hidden="true"></div>

        <ReportView v-else-if="page === 'reports'" v-model:from="reportFrom" v-model:to="reportTo" v-model:account-id="reportAccountID" v-model:query="reportQuery" v-model:status="reportStatus" v-model:export-type="reportExportType" :accounts="rawAccounts" :status-options="currentStatusOptionsForReport" :summary="reportSummary" :loading="reportLoading" :forbidden="reportForbidden" :error="reportError" :rows="batchRows" :columns="tableConfig.batches!.columns" :current-page="currentPage" :page-count="pageCount" :first-visible-row="firstVisibleRow" :last-visible-row="lastVisibleRow" :pagination-total="paginationTotal" @download="downloadReport" @reload="loadReport" @open="openTableRow" @page="currentPage = $event" />

        <SettingsView v-else-if="page === 'settings'" v-model:tab="activeTab" v-model:timezone="timezoneSetting" v-model:max-import="maxImportSetting" v-model:delimiter="csvDelimiterSetting" :role="currentUser?.role" @save="saveSettings" @users="navigate('users')" />
        <section v-else class="list-page"><MatchingBatchList v-if="page === 'matching'" v-model:search="matchingBatchSearch" :rows="matchingBatchRows" :selected-id="selectedBatchID" :total="matchingBatchTotal" :current-page="matchingBatchPage" :loading="matchingBatchLoading" :error="matchingBatchError" :busy="busy" :role="currentUser?.role" @reload="loadMatchingBatchOptions" @select="selectedBatchID = $event" @view="openMatchingBatchPopup" @run="runMatching" @page="matchingBatchPage = $event" /><div v-if="page === 'matching'" class="matching-results-heading"><h2>Hasil batch: {{ selectedBatch?.batch_no || 'Belum dipilih' }}</h2><p>Pilih batch pada tabel di atas untuk menampilkan hasilnya.</p></div><div v-if="page === 'matching'" class="kpi-grid matching-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>{{ selectedBatch?.internal_count ?? 0 }}</strong><small class="trend neutral-text">Batch terpilih</small></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>{{ selectedBatch?.bank_count ?? 0 }}</strong><small class="trend neutral-text">Batch terpilih</small></article><article class="kpi-card"><div class="kpi-top">Matched Pairs</div><strong>{{ paginationTotal }}</strong><small class="trend positive">{{ matchingProgress === "PROCESSING" ? "Matching berjalan..." : "Rule M01 · exact match" }}</small></article><article class="kpi-card"><div class="kpi-top">Exceptions</div><strong>{{ selectedBatch?.active_exceptions ?? 0 }}</strong><small class="trend warning-text">{{ selectedBatch?.batch_no ?? 'Pilih batch' }}</small></article></div><div v-if="page === 'exceptions'" class="exception-summary" aria-label="Ringkasan exception aktif"><article v-for="item in [{ kind: 'AMOUNT_MISMATCH', label: 'Selisih nominal', description: 'Nominal transaksi berbeda', icon: '≠', tone: 'amber' }, { kind: 'DIRECTION_MISMATCH', label: 'Perbedaan arah', description: 'Arah transaksi berbeda', icon: '↔', tone: 'blue' }, { kind: 'DATE_VARIANCE', label: 'Selisih tanggal', description: 'Tanggal transaksi berbeda', icon: '◷', tone: 'violet' }, { kind: 'AMBIGUOUS_MATCH', label: 'Kecocokan ganda', description: 'Lebih dari satu kemungkinan', icon: '⌘', tone: 'orange' }, { kind: 'INTERNAL_ONLY', label: 'Hanya internal', description: 'Tidak ditemukan pada data bank', icon: '↗', tone: 'teal' }, { kind: 'BANK_ONLY', label: 'Hanya bank', description: 'Tidak ditemukan pada data internal', icon: '↙', tone: 'slate' }]" :key="item.kind" class="exception-summary-card" :class="`tone-${item.tone}`"><div class="exception-card-heading"><span class="exception-card-icon" aria-hidden="true">{{ item.icon }}</span><span class="exception-card-count">{{ exceptionTypeCounts[item.kind] ?? 0 }}</span></div><div class="exception-card-content"><h3>{{ item.label }}</h3><p>{{ item.description }}</p></div><div class="exception-card-footer"><span>{{ item.kind }}</span><span>Aktif</span></div></article></div><article class="panel table-panel"><div class="table-toolbar"><div class="tabs"><button v-for="tab in currentTableTabs" :key="tab" :class="{ selected: activeTab === tab || (activeTab === 'All' && tab.startsWith('Semua')) || (activeTab === 'Menunggu' && tab.startsWith('Menunggu')) }" @click="activeTab = tab">{{ tab }}</button></div><div class="filters"><label class="search-field"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" :placeholder="page === 'exceptions' ? 'Cari reference / batch...' : 'Cari batch / reference...'" /></label><GlobalDropdown v-if="['batches','transactions'].includes(page)" v-model="sourceFilter" class="filter-select"><option>All sources</option><option>INTERNAL</option><option>BANK</option></GlobalDropdown><GlobalDropdown v-model="statusFilter" class="filter-select"><option>All statuses</option><option v-for="option in currentStatusOptions" :key="option">{{ option }}</option></GlobalDropdown><template v-if="page === 'users'"><GlobalDropdown v-model="userRoleFilter" class="filter-select"><option value="">Semua role</option><option value="ADMIN">Admin</option><option value="OPERATOR">Operator</option><option value="CHECKER">Checker</option><option value="AUDITOR">Auditor</option></GlobalDropdown></template><template v-if="page === 'batches'"><GlobalDropdown v-model="batchAccountFilter" class="filter-select"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }}</option></GlobalDropdown><input v-model="filterFrom" class="filter-select" type="date" aria-label="Periode dari"/><input v-model="filterTo" class="filter-select" type="date" aria-label="Periode sampai"/></template><template v-if="page === 'transactions'"><GlobalDropdown v-model="transactionBatchFilter" class="filter-select"><option value="">Semua batch</option><option v-for="batch in rawBatches" :key="batch.id" :value="batch.id">{{ batch.batch_no }}</option></GlobalDropdown></template><template v-if="page === 'audit'"><GlobalDropdown v-model="auditEntityFilter" class="filter-select"><option value="">Semua entitas</option><option value="batch">Batch</option><option value="exception">Exception</option><option value="transaction">Transaction</option><option value="user">User</option><option value="approval">Approval</option></GlobalDropdown><input v-model="auditActorFilter" class="filter-select" placeholder="Pengguna"/><input v-model="filterFrom" class="filter-select" type="date" aria-label="Dari tanggal"/><input v-model="filterTo" class="filter-select" type="date" aria-label="Sampai tanggal"/></template><button v-if="page === 'exceptions'" type="button" class="exception-filter-toggle" :class="{ active: exceptionFiltersOpen }" :aria-expanded="exceptionFiltersOpen" aria-controls="exception-filters" @click="exceptionFiltersOpen = !exceptionFiltersOpen"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M7 12h10M10 17h4" /></svg><span>{{ exceptionFiltersOpen ? 'Tutup filter' : 'Filter' }}</span><span v-if="exceptionActiveFilterCount" class="exception-filter-count">{{ exceptionActiveFilterCount }}</span></button><div v-if="page === 'exceptions'" v-show="exceptionFiltersOpen" id="exception-filters" class="exception-filter-panel"><GlobalDropdown v-model="typeFilter" class="filter-select"><option>All types</option><option v-for="kind in ['AMOUNT_MISMATCH','DIRECTION_MISMATCH','DATE_VARIANCE','AMBIGUOUS_MATCH','INTERNAL_ONLY','BANK_ONLY']" :key="kind">{{ kind }}</option></GlobalDropdown><GlobalDropdown v-model="batchFilter" class="filter-select"><option value="">Semua batch</option><option v-for="batch in rawBatches" :key="batch.id" :value="batch.id">{{ batch.batch_no }}</option></GlobalDropdown><GlobalDropdown v-model="exceptionAccountFilter" class="filter-select"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }}</option></GlobalDropdown><GlobalDropdown v-model="assigneeFilter" class="filter-select"><option value="">Semua assignee</option><option v-for="user in rawUsers" :key="user.id" :value="user.id">{{ user.full_name }}</option></GlobalDropdown><input v-model="filterFrom" class="filter-select" type="date" aria-label="Dari tanggal"/><input v-model="filterTo" class="filter-select" type="date" aria-label="Sampai tanggal"/><GlobalDropdown v-model="exceptionAgeFilter" class="filter-select"><option value="">Semua umur</option><option value="UNDER_24H">Kurang dari 24 jam</option><option value="24_72H">24–72 jam</option><option value="OVER_72H">Lebih dari 72 jam</option></GlobalDropdown></div><button v-if="page !== 'exceptions'" class="filter-icon" aria-label="Filter">☷</button></div></div><p v-if="listLoading" class="table-state">Memuat data…</p><p v-else-if="listForbidden" class="table-state error-state">Akses ke menu ini tidak diizinkan.</p><p v-else-if="listError" class="table-state error-state">{{ listError }} <button class="button secondary" @click="loadCurrentPage">Coba lagi</button></p><DataTable v-else-if="currentTable" :columns="currentTable.columns" :rows="paginatedRows" :hide-actions="page === 'matching'" :action-icon="page === 'users' || (page === 'accounts' && currentUser?.role === 'ADMIN') ? 'edit' : page === 'approvals' ? 'review' : page === 'audit' ? 'history' : 'view'" :action-label="page === 'users' || (page === 'accounts' && currentUser?.role === 'ADMIN') ? 'Ubah data' : page === 'approvals' ? 'Tinjau approval' : page === 'audit' ? 'Lihat riwayat' : 'Lihat detail'" :link-key="page === 'batches' ? 'batch' : page === 'exceptions' ? 'ref' : ''" @open="openTableRow" /><div class="table-footer"><span>Menampilkan {{ firstVisibleRow }}–{{ lastVisibleRow }} dari {{ paginationTotal }} baris</span><div><button :disabled="currentPage <= 1" @click="currentPage--">‹</button><button class="current-page">{{ currentPage }} / {{ pageCount }}</button><button :disabled="currentPage >= pageCount" @click="currentPage++">›</button></div></div></article></section>

        <RoleMatrix v-if="page === 'users'" />

        <footer class="app-footer">ReconX <span>·</span> Bank Transaction Reconciliation &amp; Exception Management <span class="footer-version">v0.2 · Modular monolith</span></footer>
      </div>
    </main>
    </template>

    <Transition name="modal"><div v-if="successMessage" class="modal-backdrop success-alert-backdrop"><section class="success-alert" role="alertdialog" aria-modal="true" aria-labelledby="success-alert-title" aria-describedby="success-alert-message"><div class="success-alert-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg></div><h2 id="success-alert-title">Berhasil</h2><p id="success-alert-message">{{ successMessage }}</p><button class="button primary" type="button" data-no-loading autofocus @click="successMessage = ''">OK</button></section></div></Transition>
    <Transition name="modal"><div v-if="showImportError" class="modal-backdrop import-error-backdrop" @click.self="showImportError = false"><section class="import-error-dialog" role="alertdialog" aria-modal="true" aria-labelledby="import-error-title" aria-describedby="import-error-description"><div class="import-error-icon" aria-hidden="true">!</div><h2 id="import-error-title">Import belum berhasil</h2><p id="import-error-description">{{ importError }}</p><div v-if="importErrors.length" class="import-error-details"><b>Rincian validasi</b><ul><li v-for="(error, index) in importErrors" :key="index"><strong>Baris {{ error.line }} · {{ error.field }}</strong><span>{{ importValidationMessage(error.message) }}</span></li></ul></div><p class="import-error-help">Perbaiki file dan unggah kembali. Laporan error dapat diunduh dari riwayat jika tersedia.</p><button class="button primary" data-no-loading @click="showImportError = false">Mengerti</button></section></div></Transition>
    <Transition name="modal"><div v-if="showLogoutConfirm" class="modal-backdrop" @click.self="showLogoutConfirm = false"><section class="logout-confirm" role="dialog" aria-modal="true" aria-labelledby="logout-title"><h2 id="logout-title">Keluar dari akun?</h2><p>Anda akan kembali ke halaman login.</p><div class="modal-actions"><button class="button secondary" data-no-loading @click="showLogoutConfirm = false">Batal</button><button class="button primary" data-no-loading @click="logout">Keluar</button></div></section></div></Transition>
    <DetailDialog :kind="detailPopup" @close="closeDetailPopup">
        <BatchDetail v-if="detailPopup === 'batch'" :selected-batch="selectedBatch" :current-user="currentUser" :busy="busy" :batch-tabs="batchTabs" :active-tab="activeTab" :raw-imports="rawImports" :batch-transaction-rows="batchTransactionRows" :matching-rows="matchingRows" :batch-exception-rows="batchExceptionRows" :batch-approval-rows="batchApprovalRows" :batch-audit-rows="batchAuditRows" :table-config="tableConfig" @matching="openMatchingResults" @edit="openBatchEditor" @transition="transitionBatch" @submit="submitBatch" @close-batch="closeBatch" @change-tab="activeTab = $event" @download="downloadOriginalImport" @download-errors="downloadImportErrors" @transaction="openTransaction" @exception="openBatchException" @approval="openBatchApproval" />

        <ExceptionDetail v-else-if="detailPopup === 'exception'" v-model:operator-search="operatorSearch" v-model:assignment-target="assignmentTarget" v-model:investigation-note="investigationNote" :exception-detail="exceptionDetail" :current-user="currentUser" :exception-operators="exceptionOperators" :evidence-file="evidenceFile" @open-batch="openExceptionBatch" @assign="assignException" @propose="modal = 'proposal'" @download-evidence="downloadExceptionEvidence" @add-note="addExceptionNote" @file="onEvidenceFile" @choose-file="chooseEvidenceFile" @upload-evidence="uploadExceptionEvidence" />
</DetailDialog>
    <Transition name="modal"><div v-if="modal" class="modal-backdrop" @click.self="modal = ''"><section class="modal-card" :class="`modal-${modal}`"><div class="modal-heading"><div><h2>{{ modal === 'batch' ? 'Buat Batch Rekonsiliasi' : modal === 'editBatch' ? 'Edit Batch'  : modal === 'account' ? 'Tambah Sumber Data' : modal === 'accountEdit' ? 'Edit Rekening' : modal === 'accountView' ? 'Detail Rekening' : modal === 'proposal' ? 'Ajukan Manual Match' : modal === 'approval' ? 'Keputusan Approval' : modal === 'profile' ? 'Profil Saya' : modal === 'transaction' ? 'Detail Transaksi' : modal === 'auditDetail' ? 'Detail Audit Log' : modal === 'userActions' ? 'Akses Pengguna' : 'Tambah Pengguna' }}</h2><p>{{ modal === 'batch' || modal === 'editBatch' ? 'Tentukan rekening dan periode rekonsiliasi.' : modal === 'accountView' ? 'Informasi rekening sumber.' : modal === 'accountEdit' ? 'Perbarui rekening sumber. Rekening dengan batch aktif tidak dapat dinonaktifkan.' : modal === 'proposal' ? 'Permintaan ini akan ditinjau oleh Checker.' : modal === 'approval' ? approvalDetail?.reason : modal === 'profile' ? 'Informasi akun yang sedang digunakan.' : modal === 'transaction' ? 'Informasi transaksi dan hasil rekonsiliasi.' : modal === 'auditDetail' ? 'Ringkasan aktivitas dan perubahan yang tercatat.' : 'Lengkapi informasi untuk melanjutkan.' }}</p></div><button class="close-button" data-no-loading aria-label="Tutup" @click="modal = ''">×</button></div>
      <div class="modal-body">
      <RecordDetail v-if="modal === 'transaction' || modal === 'auditDetail'" :kind="modal === 'transaction' ? 'transaction' : 'audit'" :selected-transaction="selectedTransaction" :audit-detail="auditDetail" :transaction-batch-label="transactionBatchLabel" /><template v-else-if="modal === 'profile'"><label class="field-label">Nama lengkap<input class="form-control" :value="currentUser?.full_name" readonly /></label><label class="field-label">Email<input class="form-control" :value="currentUser?.email" readonly /></label><label class="field-label">Role<input class="form-control" :value="currentUser?.role" readonly /></label><div class="modal-actions"><button class="button secondary" data-no-loading @click="modal = ''">Tutup</button><button class="button primary" @click="logout">Keluar dari akun</button></div></template><template v-else-if="modal === 'batch' || modal === 'editBatch'"><label class="field-label">Rekening Sumber<GlobalDropdown v-model="batchForm.account_id" class="form-control" required><option value="" disabled>Pilih rekening</option><option v-for="account in rawAccounts.filter(item => item.active)" :key="account.id" :value="account.id">{{ account.account_code }} · {{ account.bank_display_name }}</option></GlobalDropdown></label><label class="field-label">Periode Batch<div class="date-pair"><input v-model="batchForm.period_from" class="form-control" type="date" /><input v-model="batchForm.period_to" class="form-control" type="date" /></div></label><label class="field-label">Catatan<input v-model="batchForm.remarks" class="form-control" placeholder="Opsional" /></label></template>
      <template v-else-if="modal === 'accountView'"><div class="data-pair"><span>Kode</span><b>{{ accountForm.account_code }}</b></div><div class="data-pair"><span>Nama sumber</span><b>{{ accountForm.bank_display_name }}</b></div><div class="data-pair"><span>Rekening</span><b>{{ accountForm.masked_account_no }}</b></div><div class="data-pair"><span>Pemilik</span><b>{{ accountForm.account_holder_name }}</b></div><div class="data-pair"><span>Status</span><b>{{ accountActiveForm ? 'Aktif' : 'Nonaktif' }}</b></div></template><template v-else-if="modal === 'account' || modal === 'accountEdit'"><label class="field-label">Kode sumber<input v-model="accountForm.account_code" class="form-control" placeholder="QRIS" /></label><label class="field-label">Nama sumber<input v-model="accountForm.bank_display_name" class="form-control" placeholder="QRIS Utama" /></label><label class="field-label">Nomor rekening tersamarkan<input v-model="accountForm.masked_account_no" class="form-control" /></label><label class="field-label">Pemilik rekening<input v-model="accountForm.account_holder_name" class="form-control" /></label><label v-if="modal === 'accountEdit'" class="field-label">Status<GlobalDropdown v-model="accountActiveForm" class="form-control"><option :value="true">Aktif</option><option :value="false">Nonaktif</option></GlobalDropdown></label></template>
      <template v-else-if="modal === 'user'"><label class="field-label">Nama Lengkap<input v-model="userForm.full_name" class="form-control" placeholder="Nama pengguna" /></label><label class="field-label">Email<input v-model="userForm.email" class="form-control" type="email" placeholder="nama@perusahaan.co.id" /></label><label class="field-label">Password awal<input v-model="userForm.password" class="form-control" type="password" minlength="12" /></label><label class="field-label">Role<GlobalDropdown v-model="userForm.role" class="form-control"><option value="OPERATOR">Operator</option><option value="CHECKER">Checker</option><option value="ADMIN">Admin</option><option value="AUDITOR">Auditor</option></GlobalDropdown></label></template>
      <label v-else-if="modal === 'proposal'" class="field-label">Alasan Pengajuan<textarea v-model="proposalReason" class="form-control" rows="4" placeholder="Minimum 20 karakter; hanya DATE_VARIANCE yang dapat diusulkan."></textarea></label>
      <template v-else-if="modal === 'approval'"><p class="approval-summary">{{ approvalDetail?.kind }} · {{ approvalDetail?.batch_no }} · Maker: {{ approvalDetail?.maker_name }}</p><div v-if="approvalDetail?.kind === 'MANUAL_MATCH' && approvalDetail.internal && approvalDetail.bank" class="comparison-grid approval-comparison"><article class="panel"><h3>Internal</h3><p>{{ approvalDetail.internal.reference_id }} · {{ formatDate(approvalDetail.internal.date) }}</p><p>{{ approvalDetail.internal.direction }} · {{ money(approvalDetail.internal.amount_rupiah) }}</p></article><article class="panel"><h3>Bank</h3><p>{{ approvalDetail.bank.reference_id }} · {{ formatDate(approvalDetail.bank.date) }}</p><p>{{ approvalDetail.bank.direction }} · {{ money(approvalDetail.bank.amount_rupiah) }}</p></article></div><div v-if="approvalDetail?.kind === 'MANUAL_MATCH'" class="approval-source-details"><p><b>Aturan:</b> {{ approvalDetail.rule_version || 'M01' }} · Manual match selisih tanggal ≤ 3 hari</p><p><b>Sumber internal:</b> {{ approvalDetail.internal_file || '—' }} <button v-if="approvalDetail.internal?.import_id" class="link-button" @click="downloadOriginalImport({id: approvalDetail.internal.import_id, original_name: approvalDetail.internal_file})">Unduh</button></p><p><b>Sumber bank:</b> {{ approvalDetail.bank_file || '—' }} <button v-if="approvalDetail.bank?.import_id" class="link-button" @click="downloadOriginalImport({id: approvalDetail.bank.import_id, original_name: approvalDetail.bank_file})">Unduh</button></p></div><p v-if="approvalDetail?.reason" class="approval-summary">Alasan maker: {{ approvalDetail.reason }}</p><label class="field-label">Komentar keputusan<textarea v-model="decisionComment" class="form-control" rows="3" placeholder="Wajib diisi jika menolak"></textarea></label><div class="modal-actions"><template v-if="currentUser?.role === 'CHECKER'"><button class="button secondary" :disabled="busy" @click="decideApproval('reject')">Tolak</button><button class="button primary" :disabled="busy" @click="decideApproval('approve')">Setujui</button></template><span v-else class="login-footnote">Mode auditor: hanya baca.</span></div></template>
      <template v-else-if="modal === 'userActions'"><label class="field-label">Nama<input v-model="userEditForm.full_name" class="form-control" /></label><label class="field-label">Email<input v-model="userEditForm.email" class="form-control" type="email" /></label><label class="field-label">Role<GlobalDropdown v-model="userEditForm.role" class="form-control"><option value="OPERATOR">Operator</option><option value="CHECKER">Checker</option><option value="ADMIN">Admin</option><option value="AUDITOR">Auditor</option></GlobalDropdown></label><div class="modal-actions"><button class="button secondary" :disabled="busy" @click="toggleUserActive">{{ selectedUser?.active ? 'Nonaktifkan Akun' : 'Aktifkan Akun' }}</button><button class="button secondary" :disabled="busy" @click="saveUserChanges">Simpan Role</button></div><label class="field-label">Password baru<input v-model="resetPasswordValue" class="form-control" type="password" minlength="12" placeholder="Minimum 12 karakter" /></label><div class="modal-actions"><button class="button primary" :disabled="busy" @click="resetUserPassword">Reset Password</button></div></template>
      </div>
      <div v-if="modal !== 'approval' && modal !== 'userActions' && modal !== 'profile' && modal !== 'transaction' && modal !== 'auditDetail' && modal !== 'accountView'" class="modal-actions"><button class="button secondary" data-no-loading @click="modal = ''">Batal</button><button class="button primary" :disabled="busy" @click="submitModal">{{ busy ? 'Menyimpan…' : modal === 'proposal' ? 'Kirim untuk Approval' : modal === 'editBatch' ? 'Simpan Perubahan' : 'Simpan' }}</button></div>
    </section></div></Transition>
    <Transition name="loading"><div v-if="showGlobalLoading" class="loading-overlay" role="status" aria-live="polite" aria-label="Sedang memproses"><div class="loading-card"><span class="loading-spinner" aria-hidden="true"></span><span><b>Sedang memproses</b><small>Mohon tunggu sebentar</small></span></div></div></Transition>
    <div v-if="toast" class="toast-message"><span>✓</span>{{ toast }}</div>
  </div>
</template>
