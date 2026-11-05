<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import DataTable from './components/DataTable.vue'
import GlobalDropdown from './components/GlobalDropdown.vue'
import { ApiError, apiRequest, downloadFile, isActionLoading } from './api'
import { useSessionStore, type SessionUser } from './stores/session'

type Page = 'dashboard' | 'batches' | 'batchDetail' | 'imports' | 'transactions' | 'matching' | 'exceptions' | 'exceptionDetail' | 'approvals' | 'reports' | 'accounts' | 'users' | 'audit' | 'settings'
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

const navGroups: { title: string; items: { id: Page; label: string; icon: string; count?: string }[] }[] = [
  { title: 'OVERVIEW', items: [{ id: 'dashboard', label: 'Dashboard', icon: 'grid' }] },
  { title: 'OPERATIONS', items: [
    { id: 'batches', label: 'Reconciliation Batches', icon: 'layers', count: '1' },
    { id: 'imports', label: 'Import Data', icon: 'upload' },
    { id: 'transactions', label: 'Transactions', icon: 'rows' },
    { id: 'matching', label: 'Matching', icon: 'shuffle' },
    { id: 'exceptions', label: 'Exceptions', icon: 'alert', count: '3' },
  ] },
  { title: 'GOVERNANCE', items: [{ id: 'approvals', label: 'Approval Queue', icon: 'check', count: '0' }] },
  { title: 'ANALYTICS', items: [{ id: 'reports', label: 'Reports', icon: 'chart' }] },
  { title: 'ADMINISTRATION', items: [
    { id: 'accounts', label: 'Bank Accounts', icon: 'bank' },
    { id: 'users', label: 'Users & Access', icon: 'users' },
    { id: 'audit', label: 'Audit Logs', icon: 'history' },
    { id: 'settings', label: 'Settings', icon: 'settings' },
  ] },
]

const titles: Record<Page, string> = {
  dashboard: 'Dashboard', batches: 'Reconciliation Batches', batchDetail: 'Batch Detail', imports: 'Import Data',
  transactions: 'Transactions', matching: 'Matching Results', exceptions: 'Exceptions', exceptionDetail: 'Exception Detail',
  approvals: 'Approval Queue', reports: 'Reports', accounts: 'Bank Accounts', users: 'User Management', audit: 'Audit Logs', settings: 'Settings',
}
const subtitles: Record<Page, string> = {
  dashboard: 'Overview of your reconciliation activity and workspace health.',
  batches: 'Manage reconciliation batches across all connected sources.',
  batchDetail: 'Review the batch summary, source files, matching and approvals.',
  imports: 'Upload transaction files and review validation results.',
  transactions: 'Search and review imported transactions across batches.',
  matching: 'Review automatic matching results for a reconciliation batch.',
  exceptions: 'Investigate unmatched transactions and data discrepancies.',
  exceptionDetail: 'Compare source records and track the investigation history.',
  approvals: 'Review pending manual matches and batch close requests.',
  reports: 'Monitor reconciliation performance and export summaries.',
  accounts: 'Manage bank and source account reference data.',
  users: 'Manage workspace access and role assignments.',
  audit: 'Trace important actions performed in this workspace.',
  settings: 'Review system defaults and matching policies.',
}

const batchRows = ref<Row[]>([])
const transactionRows = ref<Row[]>([])
const exceptionRows = ref<Row[]>([])
const approvalRows = ref<Row[]>([])
const matchingRows = ref<Row[]>([])
const accountRows = ref<Row[]>([])
const userRows = ref<Row[]>([])
const roleMatrixRows = [
  { feature: 'User & role', admin: 'CRUD', operator: '—', checker: '—', auditor: '—' },
  { feature: 'Bank account', admin: 'CRUD', operator: 'View', checker: 'View', auditor: 'View' },
  { feature: 'Dashboard', admin: 'View', operator: 'View', checker: 'View', auditor: 'View' },
  { feature: 'Buat/edit draft batch', admin: '—', operator: 'Own', checker: 'View', auditor: 'View' },
  { feature: 'Import dan run matching', admin: '—', operator: 'Own', checker: 'View', auditor: 'View' },
  { feature: 'Hasil & transaksi', admin: 'View', operator: 'Scoped', checker: 'View', auditor: 'View' },
  { feature: 'Investigasi exception', admin: '—', operator: 'Assigned/own', checker: 'View', auditor: 'View' },
  { feature: 'Propose manual match', admin: '—', operator: 'Yes', checker: '—', auditor: '—' },
  { feature: 'Approve/reject match', admin: '—', operator: '—', checker: 'Yes, not own', auditor: '—' },
  { feature: 'Submit batch approval', admin: '—', operator: 'Maker', checker: '—', auditor: '—' },
  { feature: 'Approve/reject/close batch', admin: '—', operator: '—', checker: 'Yes, not maker', auditor: '—' },
  { feature: 'Reports/export', admin: 'View', operator: 'View', checker: 'View', auditor: 'View' },
  { feature: 'Audit logs', admin: 'View', operator: '—', checker: 'View', auditor: 'View' },
  { feature: 'System settings', admin: 'CRUD', operator: '—', checker: 'View', auditor: 'View' },
]
const roleCards = [
  { name: 'Admin', description: 'Kelola pengguna, rekening referensi, dan konfigurasi sistem.', tone: 'purple' },
  { name: 'Operator', description: 'Buat batch, impor data, jalankan matching, dan investigasi.', tone: 'teal' },
  { name: 'Checker', description: 'Periksa hasil dan setujui secara independen dari maker.', tone: 'blue' },
  { name: 'Auditor', description: 'Akses read-only ke batch, transaksi, laporan, approval, dan log.', tone: 'amber' },
]
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
const reportStatusChart = computed(() => {
  const palette = ['#0d9488', '#488ec1', '#e5a23d', '#8e79c8', '#70b8a5', '#e5906a', '#97aab8', '#b96f78']
  const items = ((reportSummary.value.batch_statuses ?? []) as {name: string; count: number}[]).filter(item => item.count > 0)
  const total = items.reduce((sum, item) => sum + item.count, 0)
  let offset = 0
  const segments = items.map((item, index) => { const start = offset; offset += item.count / total * 100; return {...item, color: palette[index % palette.length], start, end: offset} })
  return {total, segments, background: `conic-gradient(${segments.map(item => `${item.color} ${item.start}% ${item.end}%`).join(', ')})`}
})
const reportExceptionChart = computed(() => {
  const items = (reportSummary.value.exception_reasons ?? []) as {name: string; count: number}[]
  const max = Math.max(1, ...items.map(item => item.count))
  return items.map(item => ({...item, width: `${item.count / max * 100}%`}))
})
const dashboardTrendPoints = computed(() => {
  const points = (dashboardSummary.value.daily_trend ?? []) as { date: string; matched_count: number; unmatched_count: number }[]
  const visible = points.slice(-14)
  const maximum = Math.max(1, ...visible.map(point => point.matched_count + point.unmatched_count))
  return visible.map(point => ({
    ...point,
    label: new Intl.DateTimeFormat('id-ID', { day: '2-digit', month: 'short', timeZone: 'Asia/Jakarta' }).format(new Date(point.date)),
    matchedHeight: `${point.matched_count / maximum * 100}%`,
    unmatchedHeight: `${point.unmatched_count / maximum * 100}%`,
  }))
})
const dashboardStatusPoints = computed(() => dashboardSummary.value.batch_statuses ?? [])
const dashboardExceptionPoints = computed(() => dashboardSummary.value.exception_reasons ?? [])
const trendChart = computed(() => {
  const points = dashboardTrendPoints.value
  const maximum = Math.max(1, ...points.flatMap(point => [point.matched_count, point.unmatched_count]))
  const ticks = Array.from(new Set([0, Math.floor((points.length - 1) / 4), Math.floor((points.length - 1) / 2), Math.floor((points.length - 1) * 3 / 4), points.length - 1])).filter(i => i >= 0)
  const chartPoints = points.map((point, i) => ({ ...point, x: points.length === 1 ? 500 : 30 + i * 940 / (points.length - 1), matchedY: 160 - point.matched_count / maximum * 140, unmatchedY: 160 - point.unmatched_count / maximum * 140 }))
  return { points: chartPoints, matched: chartPoints.map(p => `${p.x},${p.matchedY}`).join(' '), unmatched: chartPoints.map(p => `${p.x},${p.unmatchedY}`).join(' '), ticks: ticks.map(i => chartPoints[i]).filter(Boolean), maximum }
})
const statusChart = computed(() => {
  const palette: Record<string, string> = { DRAFT: '#94a9ba', READY: '#3e88c7', PROCESSING: '#637bd4', REVIEW: '#e3a73c', PENDING_APPROVAL: '#ec905b', APPROVED: '#54b996', CLOSED: '#0d9488', CANCELLED: '#c97076', FAILED: '#dc6767' }
  const items = (dashboardStatusPoints.value as { name: string; count: number }[]).filter(item => item.count > 0)
  const total = items.reduce((sum, item) => sum + item.count, 0)
  let offset = 0
  const segments = items.map(item => { const start = offset; offset += item.count / total * 100; return { ...item, color: palette[item.name] ?? '#708c9a', start, end: offset } })
  return { total, segments, background: `conic-gradient(${segments.map(item => `${item.color} ${item.start}% ${item.end}%`).join(', ')})` }
})
const exceptionChart = computed(() => {
  const items = dashboardExceptionPoints.value as { name: string; count: number }[]
  const maximum = Math.max(1, ...items.map(item => item.count))
  return items.map(item => ({ ...item, width: `${item.count / maximum * 100}%` }))
})
const visibleNavGroups = computed(() => {
  const role = currentUser.value?.role
  const allowed: Record<string, Page[]> = {
    ADMIN: ['dashboard', 'batches', 'transactions', 'matching', 'exceptions', 'reports', 'accounts', 'users', 'audit', 'settings'],
    OPERATOR: ['dashboard', 'batches', 'imports', 'transactions', 'matching', 'exceptions', 'reports', 'accounts'],
    CHECKER: ['dashboard', 'batches', 'transactions', 'matching', 'exceptions', 'approvals', 'reports', 'accounts', 'audit', 'settings'],
    AUDITOR: ['dashboard', 'batches', 'transactions', 'matching', 'exceptions', 'approvals', 'reports', 'accounts', 'audit', 'settings'],
  }
  return navGroups.map(group => ({
    ...group,
    items: group.items.filter(item => allowed[role ?? '']?.includes(item.id)).map(item => {
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
      const { data } = await apiRequest<Record<string, any>>(`/api/v1/batches/${id}`)
      const index = rawBatches.value.findIndex(batch => batch.id === id)
      if (index >= 0) rawBatches.value.splice(index, 1, data)
      else rawBatches.value.unshift(data)
      if (currentUser.value?.role !== 'OPERATOR' || data.created_by === currentUser.value.id) await Promise.all([loadMatches(), loadBatchDetails(), loadImports()])
    } catch (error) { showToast(error instanceof Error ? error.message : 'Batch tidak dapat dibuka.') }
  } else if (currentPage === 'exceptionDetail' && id && id !== selectedExceptionID.value) {
    selectedExceptionID.value = id
    await loadExceptionDetail()
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

async function loadReport() {
  reportLoading.value = true
  reportError.value = ''
  reportForbidden.value = false
  const params = new URLSearchParams({ page: String(currentPage.value), page_size: String(pageSize) })
  if (reportFrom.value) params.set('from', reportFrom.value)
  if (reportTo.value) params.set('to', reportTo.value)
  if (reportAccountID.value) params.set('account_id', reportAccountID.value)
  if (reportQuery.value.trim()) params.set('q', reportQuery.value.trim())
  if (reportStatus.value) params.set('status', reportStatus.value)
  try {
    const { data, meta } = await apiRequest<{ items: Record<string, any>[]; summary: Record<string, any> }>(`/api/v1/reports/reconciliations?${params}`)
    reportSummary.value = data.summary
    paginationTotal.value = Number(meta?.total ?? data.items.length)
    batchRows.value.splice(0, batchRows.value.length, ...data.items.map(item => ({ id: item.id, batch: item.batch_no, name: item.remarks || item.bank_display_name, source: item.account_code, period: `${formatDate(item.period_from)} – ${formatDate(item.period_to)}`, total: `${item.internal_count} + ${item.bank_count}`, rate: `${(item.internal_count ? item.matched_pairs * 100 / item.internal_count : 0).toFixed(1)}%`, status: item.state, updated: formatDate(item.updated_at) })))
  } catch (error) { reportForbidden.value = error instanceof ApiError && error.status === 403; reportError.value = error instanceof Error ? error.message : 'Laporan tidak dapat dimuat.' }
  finally { reportLoading.value = false }
}

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
  else if (page.value === 'approvals') { selectedApprovalID.value = row?.id || ''; modal.value = 'approval'; decisionComment.value = '' }
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
    const { data } = await apiRequest<Record<string, any>>(`/api/v1/batches/${id}`)
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
  detailPopup.value = previousDetailPopup.value
  previousDetailPopup.value = ''
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
const auditActionLabels: Record<string, string> = {
  LOGIN: 'Masuk ke aplikasi', LOGOUT: 'Keluar dari aplikasi', USER_CREATED: 'Pengguna ditambahkan', USER_UPDATED: 'Data pengguna diperbarui', USER_PASSWORD_RESET: 'Kata sandi diatur ulang',
  ACCOUNT_CREATED: 'Rekening ditambahkan', ACCOUNT_UPDATED: 'Rekening diperbarui', BATCH_CREATED: 'Batch dibuat', BATCH_UPDATED: 'Batch diperbarui', BATCH_CANCELLED: 'Batch dibatalkan', BATCH_REOPENED: 'Batch dibuka untuk koreksi', BATCH_SUBMITTED: 'Batch diajukan untuk persetujuan', BATCH_CLOSED: 'Batch ditutup',
  IMPORT_VALIDATED: 'Validasi impor selesai', MATCH_RUN_STARTED: 'Matching dimulai', MATCH_RUN_COMPLETED: 'Matching selesai', MATCH_RUN_FAILED: 'Matching gagal', MATCH_RUN_RETRY: 'Matching disiapkan untuk dicoba kembali', EXCEPTION_ASSIGN: 'Penanggung jawab ditugaskan', EXCEPTION_NOTE: 'Catatan investigasi ditambahkan', EXCEPTION_EVIDENCE_UPLOADED: 'Bukti diunggah', MANUAL_MATCH_PROPOSED: 'Manual match diajukan', APPROVAL_APPROVED: 'Pengajuan disetujui', APPROVAL_REJECTED: 'Pengajuan ditolak', SETTINGS_UPDATED: 'Pengaturan diperbarui',
}
const auditFieldLabels: Record<string, string> = { state: 'Status', status: 'Status', batch_no: 'Nomor batch', period_from: 'Periode mulai', period_to: 'Periode akhir', source_version: 'Versi data', rule_version: 'Aturan matching', matched_pairs: 'Pasangan cocok', exceptions: 'Jumlah exception', rows_count: 'Jumlah baris', source_type: 'Sumber data', file_name: 'Nama file', email: 'Email', role: 'Role', active: 'Status akun', full_name: 'Nama', bank_display_name: 'Nama rekening', account_code: 'Kode rekening', masked_account_no: 'Nomor rekening', account_holder_name: 'Pemilik rekening', note: 'Catatan', comment: 'Komentar', result: 'Hasil', error: 'Keterangan', timezone: 'Zona waktu', max_import_bytes: 'Batas ukuran impor', csv_delimiter: 'Pemisah CSV' }
const readableStatus: Record<string, string> = { DRAFT: 'Draft', READY: 'Siap diproses', PROCESSING: 'Sedang diproses', REVIEW: 'Dalam pemeriksaan', PENDING: 'Menunggu persetujuan', PENDING_APPROVAL: 'Menunggu persetujuan', APPROVED: 'Disetujui', REJECTED: 'Ditolak', CLOSED: 'Ditutup', CANCELLED: 'Dibatalkan', FAILED: 'Gagal', OPEN: 'Terbuka', INVESTIGATING: 'Dalam investigasi', PENDING_ACTION_APPROVAL: 'Menunggu persetujuan', RESOLVED: 'Selesai', ARCHIVED: 'Diarsipkan', VALID: 'Valid', INVALID: 'Tidak valid', MATCHED: 'Sudah cocok', UNMATCHED: 'Belum cocok', EXCEPTION: 'Perlu pemeriksaan', INTERNAL: 'Internal', BANK: 'Bank', CREDIT: 'Kredit', DEBIT: 'Debit', success: 'Berhasil', batch: 'Batch', run: 'Proses matching', user: 'Pengguna', bank_account: 'Rekening', exception: 'Exception', approval: 'Persetujuan', settings: 'Pengaturan', import: 'Impor' }
function humanLabel(value: unknown) { const text = String(value ?? ''); return auditActionLabels[text] || readableStatus[text] || text.replace(/_/g, ' ').toLowerCase().replace(/^./, letter => letter.toUpperCase()) || '—' }
function auditFields(value: unknown): { label: string; value: string }[] {
  let data = value
  if (typeof data === 'string') { try { data = JSON.parse(data) } catch { return [] } }
  if (!data || typeof data !== 'object' || Array.isArray(data)) return []
  return Object.entries(data as Record<string, unknown>).flatMap(([key, item]) => {
    if (key === 'id' || key.endsWith('_id') || key === 'assigned_to' || item === null || item === undefined || typeof item === 'object') return []
    const text = String(item)
    if (/^[0-9a-f]{8}-[0-9a-f-]{27,}$/i.test(text) || /^[0-9a-f]{32,}$/i.test(text)) return []
    let display = key === 'error' ? 'Terjadi kendala saat proses berlangsung.' : typeof item === 'boolean' ? (item ? 'Aktif' : 'Nonaktif') : (readableStatus[text] ?? text)
    if ((key === 'period_from' || key === 'period_to') && !Number.isNaN(Date.parse(text))) display = formatDate(text)
    return [{ label: auditFieldLabels[key] || humanLabel(key), value: display || '—' }]
  })
}
function formatDateTime(value?: string) { if (!value || Number.isNaN(Date.parse(value))) return '—'; return new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Jakarta' }).format(new Date(value)) + ' WIB' }
const auditBefore = computed(() => auditFields(auditDetail.value?.before))
const auditAfter = computed(() => auditFields(auditDetail.value?.after))

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

function onFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (file) { importError.value = ''; importErrors.value = []; fileToUpload.value = file; selectedFile.value = file.name; importStage.value = 3 }
}

function clearImportFile() {
  importError.value = ''; importErrors.value = []
  fileToUpload.value = null
  selectedFile.value = ''
  importStage.value = selectedBatchID.value ? 2 : 1
  const input = document.getElementById('file-input') as HTMLInputElement | null
  if (input) input.value = ''
}

function changeImportBatch() {
  importStage.value = selectedBatchID.value ? (fileToUpload.value ? 3 : 2) : 1
  void loadImports()
}

async function downloadReport() {
  const params = new URLSearchParams()
  if (reportFrom.value) params.set('from', reportFrom.value)
  if (reportTo.value) params.set('to', reportTo.value)
  if (reportAccountID.value) params.set('account_id', reportAccountID.value)
  if (reportQuery.value.trim()) params.set('q', reportQuery.value.trim())
  if (reportStatus.value) params.set('status', reportStatus.value)
  params.set('type', reportExportType.value)
  try { await downloadFile(`/api/v1/reports/reconciliations/export?${params}`, `reconx-${reportExportType.value}.csv`) }
  catch (error) { showToast(error instanceof Error ? error.message : 'Gagal mengunduh laporan.') }
}
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

async function uploadImport() {
  if (!fileToUpload.value || !selectedBatchID.value) { importError.value = 'Pilih batch dan file CSV terlebih dahulu.'; showImportError.value = true; return }
  importError.value = ''; importErrors.value = []
  if (!fileToUpload.value.name.toLowerCase().endsWith('.csv')) { importError.value = 'Format file tidak didukung. Simpan file Excel sebagai CSV UTF-8 terlebih dahulu.'; showImportError.value = true; return }
  if (fileToUpload.value.size > 10 * 1024 * 1024) { importError.value = 'Ukuran file melebihi batas 10 MB.'; showImportError.value = true; return }
  const form = new FormData(); form.set('file', fileToUpload.value); form.set('source_type', importSource.value)
  importStage.value = 3
  busy.value = true
  try {
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/imports`, { method: 'POST', idempotent: true, body: form })
    clearImportFile(); importStage.value = 4; await loadWorkspace(); await loadImports(); showSuccess('CSV tervalidasi dan diimpor.')
  } catch (error) {
    const data = error instanceof ApiError ? error.data as { errors?: { line: number; field: string; message: string }[] } | undefined : undefined
    importErrors.value = data?.errors ?? []
    importError.value = importErrors.value.length ? 'Data dalam file belum lolos validasi. Periksa rincian berikut.' : error instanceof Error ? error.message : 'Import gagal. Periksa format file dan coba lagi.'
    importStage.value = 2
    await loadImports()
    showImportError.value = true
  } finally { busy.value = false }
}

function importValidationMessage(message: string) {
  const translations: Record<string, string> = {
    'Reference cannot be empty': 'Reference tidak boleh kosong.',
    'Date must use YYYY-MM-DD': 'Tanggal harus menggunakan format YYYY-MM-DD.',
    'Date falls outside the batch period': 'Tanggal berada di luar periode batch.',
    'Direction must be CREDIT or DEBIT': 'Arah transaksi harus CREDIT atau DEBIT.',
    'Amount must be a positive integer in rupiah': 'Nominal harus berupa bilangan bulat positif dalam rupiah.',
    'Currency must be IDR': 'Mata uang harus IDR.',
    'Column count does not match header': 'Jumlah kolom tidak sesuai dengan header.',
    'CSV row format is invalid': 'Format baris CSV tidak valid.',
    'CSV must contain at least one transaction': 'File harus berisi minimal satu transaksi.',
  }
  return translations[message] ?? message
}

async function runMatching(batchID: string) {
  if (!batchID || busy.value) return
  selectedBatchID.value = batchID
  busy.value = true
  try {
    const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${batchID}`)
    const { data: started } = await apiRequest<Record<string, any>>(`/api/v1/batches/${batchID}/match-runs`, { method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}' })
    matchingProgress.value = 'PROCESSING'
    const deadline = Date.now() + 30 * 60 * 1000
    let completed: Record<string, any> | null = null
    while (Date.now() < deadline) {
      await new Promise(resolve => window.setTimeout(resolve, 750))
      try {
        const { data } = await apiRequest<Record<string, any>>(`/api/v1/batches/${batchID}/match-runs/latest`)
        if (data.id === started.id) {
          matchingProgress.value = data.state
          if (data.state === 'SUCCEEDED' || data.state === 'FAILED') { completed = data; break }
        }
      } catch { /* Keep polling through temporary request failures. */ }
    }
    await loadWorkspace()
    await loadMatchingBatchOptions()
    selectedBatchID.value = batchID
    if (!completed) showToast('Matching masih berjalan. Periksa kembali hasil batch sebentar lagi.')
    else if (completed.state === 'SUCCEEDED') showSuccess(`Matching selesai: ${completed.matched_pairs} pasangan cocok, ${completed.exceptions} exception.`)
    else showToast('Matching gagal. Periksa status batch sebelum mencoba lagi.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Matching gagal.') }
  finally { matchingProgress.value = ''; busy.value = false }
}

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
    exceptionDetail.value = (await apiRequest<Record<string, any>>(`/api/v1/exceptions/${selectedExceptionID.value}`)).data
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
function openBatchApproval(row: Row) {
  selectedApprovalID.value = row.id || ''
  decisionComment.value = ''
  modal.value = 'approval'
}
async function assignException() {
  if (!selectedExceptionID.value) return
  const assigneeID = assignmentTarget.value || currentUser.value?.id || ''
  if (!assigneeID) { showToast('Pilih Operator terlebih dahulu.'); return }
  try { await apiRequest(`/api/v1/exceptions/${selectedExceptionID.value}/actions`, { method: 'POST', body: JSON.stringify({ type: 'ASSIGN', assignee_id: assigneeID, note: '' }) }); await loadWorkspace(); await loadExceptionDetail(); showSuccess(assigneeID === currentUser.value?.id ? 'Exception ditugaskan ke Anda.' : 'Exception berhasil ditugaskan.') }
  catch (error) { showToast(error instanceof Error ? error.message : 'Aksi gagal.') }
}
async function addExceptionNote() {
  if (!selectedExceptionID.value || !investigationNote.value.trim()) return
  try {
    await apiRequest(`/api/v1/exceptions/${selectedExceptionID.value}/actions`, { method: 'POST', body: JSON.stringify({ type: 'NOTE', note: investigationNote.value.trim() }) })
    investigationNote.value = ''; await loadWorkspace(); await loadExceptionDetail(); showSuccess('Catatan investigation disimpan.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Catatan gagal disimpan.') }
}
function onEvidenceFile(event: Event) {
  evidenceFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
}
function chooseEvidenceFile() {
  const scope = detailPopup.value === 'exception' ? document.querySelector('.detail-popup') : document
  scope?.querySelector<HTMLInputElement>('#exception-evidence-input')?.click()
}
async function uploadExceptionEvidence() {
  if (!selectedExceptionID.value || !evidenceFile.value) { showToast('Pilih file evidence terlebih dahulu.'); return }
  if (evidenceFile.value.size > 5 * 1024 * 1024) { showToast('Ukuran evidence maksimal 5 MB.'); return }
  const form = new FormData()
  form.set('file', evidenceFile.value)
  try {
    await apiRequest(`/api/v1/exceptions/${selectedExceptionID.value}/evidence`, { method: 'POST', idempotent: true, body: form })
    evidenceFile.value = null
    await loadWorkspace()
    await loadExceptionDetail()
    showSuccess('Evidence tersimpan di penyimpanan private.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Evidence gagal diunggah.') }
}
async function downloadExceptionEvidence(action: Record<string, any>) {
  try { await downloadFile(`/api/v1/exceptions/${selectedExceptionID.value}/evidence/${action.evidence_id}`, action.file_name || 'evidence') }
  catch (error) { showToast(error instanceof Error ? error.message : 'Evidence tidak dapat diunduh.') }
}
async function submitBatch() {
  if (!selectedBatchID.value) { showToast('Pilih batch terlebih dahulu.'); return }
  busy.value = true
  try {
    const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${selectedBatchID.value}`)
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/submit`, { method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}' })
    await loadWorkspace(); showSuccess('Batch dikirim ke antrean approval.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Batch belum dapat diajukan.') }
  finally { busy.value = false }
}
async function closeBatch() {
  if (!selectedBatchID.value) return
  if (!window.confirm('Tutup batch yang sudah disetujui? Hasilnya akan dikunci dan tidak dapat diubah.')) return
  busy.value = true
  try {
    const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${selectedBatchID.value}`)
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/close`, { method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}' })
    await loadWorkspace(); showSuccess('Batch berhasil ditutup dan dikunci.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Batch belum dapat ditutup.') }
  finally { busy.value = false }
}
async function transitionBatch(action: 'cancel' | 'reopen' | 'retry') {
  if (!selectedBatchID.value) return
  busy.value = true
  try {
    const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${selectedBatchID.value}`)
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/${action}`, { method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}' })
    await loadWorkspace(); showSuccess(action === 'cancel' ? 'Batch dibatalkan.' : action === 'reopen' ? 'Batch dibuka untuk koreksi dan import ulang.' : 'Batch siap dicoba kembali.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Perubahan state batch gagal.') }
  finally { busy.value = false }
}
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
      try { rawApprovals.value = (await apiRequest<Record<string, any>[]>('/api/v1/approvals')).data }
      catch (error) { showToast(error instanceof Error ? error.message : 'Approval tidak dapat dibuka.'); return }
      selectedApprovalID.value = item.target_entity_id
      decisionComment.value = ''
      modal.value = 'approval'
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

function badgeClass(value: string) {
  const v = value.toLowerCase()
  if (v.includes('completed') || (v.includes('matched') && !v.includes('unmatched')) || v.includes('aktif') || v.includes('success') || v.includes('disetujui')) return 'badge success'
  if (v.includes('progress') || v.includes('review') || v.includes('menunggu') || v.includes('open')) return 'badge warning'
  if (v.includes('unmatched') || v.includes('only') || v.includes('mismatch') || v.includes('ditolak') || v.includes('failed')) return 'badge danger'
  return 'badge neutral'
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
            <label class="field-label">Password<div class="password-input-wrap"><input v-model="loginPassword" class="form-control" :type="showLoginPassword ? 'text' : 'password'" autocomplete="current-password" required placeholder="Password akun" /><button type="button" @click="showLoginPassword = !showLoginPassword">{{ showLoginPassword ? 'Sembunyikan' : 'Lihat' }}</button></div></label>
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

        <section v-if="page === 'dashboard'" class="dashboard-page"><p v-if="dashboardLoading" class="table-state">Memuat ringkasan…</p><p v-else-if="dashboardForbidden" class="table-state error-state">Akses dashboard tidak diizinkan.</p><p v-else-if="dashboardError" class="table-state error-state">{{ dashboardError }} <button class="button secondary" @click="loadDashboard">Coba lagi</button></p>
          <div class="kpi-grid">
            <article class="kpi-card"><div class="kpi-top"><span>Total Batch</span><span class="kpi-icon teal">▦</span></div><strong>{{ dashboardSummary.total_batches ?? 0 }}</strong><small class="trend neutral-text">{{ dashboardSummary.active_batches ?? 0 }} masih aktif</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Matched Pairs</span><span class="kpi-icon green">✓</span></div><strong>{{ dashboardSummary.matched_pairs ?? 0 }}</strong><small class="trend positive">{{ Number(dashboardSummary.match_rate ?? 0).toFixed(1) }}% dari transaksi internal</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Exceptions Aktif</span><span class="kpi-icon amber">⚑</span></div><strong>{{ dashboardSummary.open_exceptions ?? 0 }}</strong><small class="trend warning-text">{{ dashboardSummary.exceptions_older_24h ?? 0 }} lebih dari 24 jam</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Approval Menunggu</span><span class="kpi-icon blue">◷</span></div><strong>{{ dashboardSummary.pending_approvals ?? 0 }}</strong><small class="trend neutral-text">Perlu tindakan Checker</small></article>
          </div>
          <div class="dashboard-insights">
            <article class="panel dashboard-chart-panel"><div class="panel-heading"><div><h2>Tren Rekonsiliasi</h2><p>Transaksi internal per tanggal pada filter terpilih</p></div></div><div v-if="trendChart.points.length" class="trend-line-chart"><div class="trend-chart-scale"><span>{{ trendChart.maximum }}</span><span>0</span></div><div class="trend-chart-main"><svg viewBox="0 0 1000 180" preserveAspectRatio="none" role="img" aria-label="Tren transaksi matched dan unmatched"><line v-for="y in [20, 90, 160]" :key="y" x1="30" x2="970" :y1="y" :y2="y" class="trend-grid-line"/><polyline v-if="trendChart.points.length > 1" :points="trendChart.matched" class="trend-line matched"/><polyline v-if="trendChart.points.length > 1" :points="trendChart.unmatched" class="trend-line unmatched"/><g v-for="point in trendChart.points" :key="point.date"><circle :cx="point.x" :cy="point.matchedY" r="5" class="trend-dot matched"><title>{{ point.label }}: {{ point.matched_count }} matched</title></circle><circle :cx="point.x" :cy="point.unmatchedY" r="5" class="trend-dot unmatched"><title>{{ point.label }}: {{ point.unmatched_count }} unmatched</title></circle></g></svg><div class="trend-axis"><span v-for="point in trendChart.ticks" :key="point!.date">{{ point!.label }}</span></div></div></div><p v-else class="chart-empty">Belum ada transaksi pada periode ini.</p><div class="chart-legend"><span><i class="trend-matched"></i>Matched internal</span><span><i class="trend-unmatched"></i>Unmatched internal</span></div></article>
            <article class="panel dashboard-distribution"><div class="panel-heading"><div><h2>Status Batch</h2><p>Jumlah batch sesuai filter</p></div></div><div v-if="statusChart.total" class="status-chart"><div class="status-donut" :style="{ background: statusChart.background }" role="img" :aria-label="`Total ${statusChart.total} batch berdasarkan status`"><div><strong>{{ statusChart.total }}</strong><span>Batch</span></div></div><div class="status-legend"><div v-for="item in statusChart.segments" :key="item.name" class="status-legend-row"><i :style="{ background: item.color }"></i><span>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div></div></div><p v-else class="chart-empty">Belum ada batch.</p></article>
            <article class="panel dashboard-distribution"><div class="panel-heading"><div><h2>Top Selisih</h2><p>Exception yang masih aktif</p></div></div><div v-if="exceptionChart.length" class="exception-chart"><div v-for="item in exceptionChart" :key="item.name" class="exception-chart-row"><div><span>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div><div class="exception-chart-track"><i :style="{ width: item.width }"></i></div></div></div><p v-else class="chart-empty">Belum ada exception aktif.</p></article>
          </div><article class="panel table-panel recent-panel"><div class="panel-heading"><div><h2>Batch Rekonsiliasi</h2><p>Status dan ringkasan dari database workspace</p></div><button class="link-button" @click="navigate('batches')">Lihat semua batch <span>→</span></button></div><DataTable :columns="tableConfig.batches!.columns" :rows="batchRows.slice(0, 6)" @open="openTableRow" /></article>
          <div class="bottom-grid"><article class="panel process-panel"><div class="panel-heading"><div><h2>Alur Proses</h2><p>Batch berjalan melalui state machine yang tervalidasi di server</p></div></div><div class="process-flow"><div v-for="(step, i) in [{icon:'▱',title:'Buat Batch',text:'Pilih rekening & periode'}, {icon:'⇧',title:'Import Data',text:'Unggah dua CSV'}, {icon:'⇄',title:'Matching',text:'Pencocokan exact'}, {icon:'⌕',title:'Investigasi',text:'Tinjau exception'}, {icon:'✓',title:'Approval',text:'Pemeriksaan Checker'}, {icon:'▥',title:'Tutup Batch',text:'Kunci hasil disetujui'}]" :key="step.title" class="process-step"><span class="process-icon">{{ step.icon }}</span><div><b>{{ i + 1 }}. {{ step.title }}</b><small>{{ step.text }}</small></div><span v-if="i < 5" class="process-arrow">→</span></div></div></article></div>
        </section>

        <section v-else-if="page === 'imports'" class="page-grid import-layout">
          <article class="panel import-card"><div class="panel-heading"><div><h2>Unggah File Transaksi</h2><p>CSV divalidasi seluruhnya sebelum transaksi disimpan.</p></div><span class="step-count">CSV · 10 MB maks.</span></div>
            <ol class="import-stepper" aria-label="Tahapan import">
              <li v-for="(step, index) in ['Pilih batch', 'Pilih file', 'Validasi & import', 'Selesai']" :key="step" :class="{ active: importStage === index + 1, complete: importStage > index + 1 }"><span>{{ importStage > index + 1 ? '✓' : index + 1 }}</span><b>{{ step }}</b></li>
            </ol>
            <div class="upload-grid"><button class="drop-zone" @click="openFilePicker"><span class="upload-cloud">⇧</span><b>{{ selectedFile || 'Pilih file CSV' }}</b><small>Kolom wajib: reference_id, date, direction, amount, currency</small><span class="button primary small-button">Pilih File</span><small class="format-note">Data ditolak seluruhnya bila ada baris tidak valid.</small></button>
              <div class="upload-side"><div class="field-label">Sumber Data</div><GlobalDropdown v-model="importSource" class="form-control"><option value="INTERNAL">INTERNAL · Catatan perusahaan</option><option value="BANK">BANK · Mutasi bank</option></GlobalDropdown><div class="field-label spaced">Batch Rekonsiliasi</div><GlobalDropdown v-model="selectedBatchID" @change="changeImportBatch" class="form-control"><option value="" disabled>Pilih batch draft</option><option v-for="batch in rawBatches.filter(item => item.state === 'DRAFT')" :key="batch.id" :value="batch.id">{{ batch.batch_no }} · {{ formatDate(batch.period_from) }}</option></GlobalDropdown><div class="check-note"><span>✓</span><div><b>Validasi sebelum disimpan</b><small>Periksa batch, sumber, dan format file sebelum mengimpor transaksi.</small></div></div></div></div>
            <section class="import-guide"><h3>Format file untuk import</h3><p>Siapkan data di Excel, lalu pilih <b>Save As → CSV UTF-8 (Comma delimited) (*.csv)</b>. File .xlsx belum dapat diunggah langsung. Gunakan koma sebagai pemisah kolom.</p><p>Contoh referensi ganda untuk menguji ambiguous match: <a href="/contoh/internal-ambiguous.csv" download>Unduh CSV internal</a> · <a href="/contoh/bank-ambiguous.csv" download>Unduh CSV bank</a>. Sesuaikan tanggal dengan periode batch.</p><p>Baris pertama wajib berisi nama kolom berikut:</p><pre>reference_id,date,direction,amount,currency
TRX001,2026-01-15,DEBIT,150000,IDR
TRX002,2026-01-15,CREDIT,250000,IDR</pre><ul><li><b>reference_id:</b> wajib diisi; referensi ganda dapat menghasilkan exception ambiguous match.</li><li><b>date:</b> format YYYY-MM-DD dan harus berada dalam periode batch.</li><li><b>direction:</b> DEBIT atau CREDIT.</li><li><b>amount:</b> bilangan bulat positif dalam rupiah, tanpa titik, koma, atau simbol mata uang.</li><li><b>currency:</b> IDR. Maksimal 50.000 baris dan 10 MB.</li></ul><a class="link-button" download="template-transaksi-reconx.csv" href="data:text/csv;charset=utf-8,reference_id%2Cdate%2Cdirection%2Camount%2Ccurrency%0ATRX001%2C2026-01-15%2CDEBIT%2C150000%2CIDR%0A">Unduh contoh CSV</a></section>
            <input id="file-input" class="hidden-input" type="file" accept=".csv,text/csv" @change="onFile" /><div class="import-actions"><button class="button secondary" data-no-loading @click="clearImportFile">Batal</button><button class="button primary" :disabled="!fileToUpload || busy" @click="uploadImport">{{ busy ? 'Memproses…' : 'Validasi & Import' }} <span>→</span></button></div>
          </article>
          <article class="panel history-card"><div class="panel-heading"><div><h2>Riwayat Import</h2><p>File yang pernah diunggah untuk batch terpilih.</p></div></div><div v-for="item in rawImports" :key="item.id" class="import-history-entry"><div class="import-history-info"><strong>{{ item.original_name }}</strong><div class="import-history-meta"><span>{{ item.source_type }}</span><span :class="['import-history-status', item.state === 'VALID' ? 'valid' : 'invalid']">{{ item.state === 'VALID' ? 'Berhasil' : item.state === 'INVALID' ? 'Gagal' : item.state }}</span><span>{{ item.rows_count }} baris</span></div><small>{{ formatDate(item.created_at) }}</small></div><div class="import-history-actions"><button class="import-history-action" type="button" aria-label="Unduh file asli" title="Unduh file asli" @click="downloadOriginalImport(item)"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m-5-5 5 5 5-5M4 17v3h16v-3" /></svg></button><button v-if="item.has_error_report" class="import-history-action error" type="button" aria-label="Unduh laporan error" title="Unduh laporan error" @click="downloadImportErrors(item)"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 2.5 20h19L12 3Zm0 6v4m0 3h.01" /></svg></button></div></div><p v-if="!rawImports.length" class="login-footnote">Belum ada file pada batch terpilih.</p></article>
        </section>

        <section v-else-if="page === 'batchDetail'" class="detail-page batch-detail-page">
          <button class="back-link" @click="navigate('batches')">Kembali ke daftar batch</button>
          <div class="detail-title"><div><div class="title-line"><h2>{{ selectedBatch?.batch_no || 'Batch tidak dipilih' }}</h2><span class="badge" :class="badgeClass(selectedBatch?.state || '')">{{ selectedBatch?.state || '—' }}</span></div><p>{{ selectedBatch?.remarks || selectedBatch?.bank_display_name || 'Tanpa catatan' }} <span>·</span> {{ formatDate(selectedBatch?.period_from) }} – {{ formatDate(selectedBatch?.period_to) }} · {{ selectedBatch?.account_code }}</p></div><button v-if="currentUser?.role !== 'OPERATOR' || selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="openMatchingResults">Hasil Matching</button><button v-if="selectedBatch?.state === 'DRAFT' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="openBatchEditor">Edit Batch</button><button v-if="selectedBatch?.state === 'DRAFT' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="transitionBatch('cancel')">Batalkan</button><button v-if="selectedBatch?.state === 'REVIEW' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="transitionBatch('reopen')">Koreksi Sumber</button><button v-if="selectedBatch?.state === 'REVIEW' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="submitBatch">Ajukan Approval</button><button v-if="selectedBatch?.state === 'FAILED' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="transitionBatch('retry')">Coba Matching Lagi</button><button v-if="selectedBatch?.state === 'APPROVED' && currentUser?.role === 'CHECKER'" class="button primary" :disabled="busy" @click="closeBatch">Tutup Batch</button></div>
          <div class="kpi-grid detail-kpis"><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Transaksi Internal</span><span class="detail-metric-icon teal">↗</span></div><strong>{{ selectedBatch?.internal_count ?? 0 }}</strong><small>Data dari internal</small></article><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Transaksi Bank</span><span class="detail-metric-icon blue">↙</span></div><strong>{{ selectedBatch?.bank_count ?? 0 }}</strong><small>Data dari bank</small></article><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Matched</span><span class="detail-metric-icon green">✓</span></div><strong>{{ selectedBatch?.matched_pairs ?? 0 }}</strong><small>Pasangan transaksi</small></article><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Exceptions Aktif</span><span class="detail-metric-icon amber">!</span></div><strong>{{ selectedBatch?.active_exceptions ?? 0 }}</strong><small>Memerlukan tindak lanjut</small></article></div>
          <article class="panel detail-tabs"><div class="tabs"><button v-for="tab in batchTabs" :key="tab" :class="{ selected: activeTab === tab }" @click="activeTab = tab">{{ tab }}</button></div><div v-if="activeTab === 'Ringkasan'" class="detail-summary"><div><h3>Ringkasan</h3><p>Ringkasan hasil rekonsiliasi untuk batch ini.</p><div class="match-progress"><span>Matched</span><div><i :style="{ width: `${selectedBatch?.internal_count ? selectedBatch.matched_pairs * 100 / selectedBatch.internal_count : 0}%` }"></i></div><b>{{ selectedBatch?.matched_pairs ?? 0 }} pasangan</b></div><div class="match-progress unmatched-progress"><span>Exceptions</span><div><i :style="{ width: `${selectedBatch?.internal_count ? selectedBatch.active_exceptions * 100 / selectedBatch.internal_count : 0}%` }"></i></div><b>{{ selectedBatch?.active_exceptions ?? 0 }} aktif</b></div></div><div class="batch-state"><span class="state-check">!</span><b>{{ selectedBatch?.state === 'CLOSED' ? 'Batch ditutup' : selectedBatch?.state === 'CANCELLED' ? 'Batch dibatalkan' : selectedBatch?.state === 'DRAFT' ? 'Batch masih dalam persiapan' : selectedBatch?.state === 'PENDING_APPROVAL' ? 'Menunggu persetujuan' : selectedBatch?.state === 'APPROVED' ? 'Siap ditutup' : selectedBatch?.active_exceptions ? 'Belum dapat ditutup' : 'Periksa status batch' }}</b><small>{{ selectedBatch?.state === 'CANCELLED' ? 'Batch ini telah dibatalkan.' : selectedBatch?.state === 'CLOSED' ? 'Hasil rekonsiliasi telah dikunci.' : 'Batch dapat diajukan setelah seluruh data cocok dan tidak ada exception atau approval tertunda.' }}</small></div></div><div v-else-if="activeTab === 'Imports'"><div v-for="item in rawImports" :key="item.id" class="history-item"><span class="file-icon">CSV</span><span class="file-info"><b>{{ item.original_name }}</b><small>{{ item.source_type }} · {{ item.state }} · {{ item.rows_count }} baris · {{ formatDate(item.created_at) }}</small></span><button class="link-button" @click="downloadOriginalImport(item)">File</button><button v-if="item.has_error_report" class="link-button" @click="downloadImportErrors(item)">Unduh Error</button></div><div v-if="!rawImports.length" class="detail-empty"><span class="detail-empty-icon">⇧</span><strong>Belum ada file import</strong><p>File yang diunggah untuk batch ini akan ditampilkan di sini.</p></div></div><div v-else-if="activeTab === 'Detail Transaksi'"><DataTable :columns="[{ key: 'ref', label: 'Reference' }, { key: 'date', label: 'Tanggal' }, { key: 'source', label: 'Sumber' }, { key: 'description', label: 'Arah' }, { key: 'debit', label: 'Debit' }, { key: 'credit', label: 'Kredit' }, { key: 'status', label: 'Status' }]" :rows="batchTransactionRows" @open="openTransaction" /></div><div v-else-if="activeTab === 'Matching Results'"><DataTable :columns="tableConfig.matching!.columns" :rows="matchingRows" /></div><div v-else-if="activeTab === 'Exceptions'"><DataTable :columns="tableConfig.exceptions!.columns" :rows="batchExceptionRows" link-key="ref" @open="openBatchException" /></div><div v-else-if="activeTab === 'Approvals'"><DataTable :columns="tableConfig.approvals!.columns" :rows="batchApprovalRows" @open="openBatchApproval" /></div><div v-else><DataTable :columns="tableConfig.audit!.columns" :rows="batchAuditRows" /></div></article>
        </section>

        <section v-else-if="page === 'exceptionDetail'" class="detail-page exception-detail-page"><button class="back-link" @click="navigate('exceptions')">Kembali ke Exceptions</button>
          <div class="detail-title"><div><div class="title-line"><h2>{{ exceptionDetail?.reference_id || 'Exception' }}</h2><span class="badge warning">{{ exceptionDetail?.type || '—' }}</span></div><p>{{ exceptionDetail?.batch_no }} <span>·</span> {{ exceptionDetail?.state }} <span>·</span> PIC: {{ exceptionDetail?.assignee_name || 'Belum ditugaskan' }}</p></div><button v-if="exceptionDetail?.batch_id" class="button secondary" data-loading @click="openExceptionBatch">Lihat batch</button><div v-if="currentUser?.role === 'OPERATOR' && exceptionDetail?.can_assign_others && ['OPEN','INVESTIGATING'].includes(exceptionDetail?.state)" class="exception-assignment"><input v-model="operatorSearch" class="form-control" aria-label="Cari operator" placeholder="Cari nama Operator" /><GlobalDropdown v-model="assignmentTarget" class="form-control" aria-label="Pilih penanggung jawab"><option :value="currentUser?.id || ''">Saya · {{ currentUser?.full_name }}</option><option v-for="operator in exceptionOperators.filter(item => item.id !== currentUser?.id)" :key="operator.id" :value="operator.id">{{ operator.full_name }}</option></GlobalDropdown><button class="button secondary" :disabled="!assignmentTarget" @click="assignException">Tugaskan</button></div><button v-if="exceptionDetail?.type === 'DATE_VARIANCE' && currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING'].includes(exceptionDetail?.state)" class="button primary" data-loading @click="modal = 'proposal'">Ajukan Manual Match</button></div>
          <div class="comparison-grid"><article class="panel"><div class="panel-heading"><div><h2>Data Internal</h2><p>{{ exceptionDetail?.internal?.source_type || 'INTERNAL' }}</p></div></div><div class="data-pair"><span>Reference ID</span><b>{{ exceptionDetail?.internal?.reference_id || '—' }}</b></div><div class="data-pair"><span>Tanggal</span><b>{{ formatDate(exceptionDetail?.internal?.date) }}</b></div><div class="data-pair"><span>Arah</span><b>{{ exceptionDetail?.internal?.direction || '—' }}</b></div><div class="data-pair"><span>Nominal</span><b>{{ money(exceptionDetail?.internal?.amount_rupiah) }}</b></div></article><article class="panel"><div class="panel-heading"><div><h2>Data Bank</h2><p>{{ exceptionDetail?.bank?.source_type || 'BANK' }}</p></div></div><div class="data-pair"><span>Reference ID</span><b>{{ exceptionDetail?.bank?.reference_id || '—' }}</b></div><div class="data-pair"><span>Tanggal</span><b>{{ formatDate(exceptionDetail?.bank?.date) }}</b></div><div class="data-pair"><span>Arah</span><b>{{ exceptionDetail?.bank?.direction || '—' }}</b></div><div class="data-pair mismatch"><span>Nominal</span><b>{{ money(exceptionDetail?.bank?.amount_rupiah) }}</b></div></article></div><article v-if="exceptionDetail?.type === 'AMBIGUOUS_MATCH'" class="panel ambiguity-panel"><h3>Transaksi dengan reference yang sama</h3><p>Reference ini muncul lebih dari sekali. Pilihannya perlu diperiksa dan sumber diperbaiki sebelum batch diajukan.</p><div class="candidate-table-wrap"><table class="candidate-table"><thead><tr><th>Sumber</th><th>Referensi</th><th>Tanggal</th><th>Arah</th><th>Nominal</th><th>Mata uang</th></tr></thead><tbody><tr v-for="candidate in exceptionDetail.candidates || []" :key="candidate.id"><td>{{ candidate.source_type }}</td><td>{{ candidate.reference_id }}</td><td>{{ formatDate(candidate.date) }}</td><td>{{ candidate.direction }}</td><td>{{ money(candidate.amount_rupiah) }}</td><td>{{ candidate.currency }}</td></tr></tbody></table></div><p v-if="!exceptionDetail.candidates?.length">Tidak ada kandidat yang tersedia.</p></article>
          <article class="panel timeline-panel"><div class="panel-heading"><div><h2>Riwayat Pemeriksaan</h2><p>Catatan dan assignment exception</p></div></div><div class="timeline"><div v-for="action in exceptionDetail?.actions || []" :key="action.id"><i></i><span><b>{{ action.action_type }} · {{ action.actor_name }}</b><small>{{ action.note || 'Tidak ada catatan' }} · {{ formatDate(action.created_at) }}</small><button v-if="action.has_evidence" class="link-button" @click="downloadExceptionEvidence(action)">Unduh {{ action.file_name || 'evidence' }}</button></span></div><p v-if="!exceptionDetail?.actions?.length">Belum ada tindakan.</p></div><label v-if="currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL'].includes(exceptionDetail?.state)" class="field-label">Catatan investigasi<textarea v-model="investigationNote" class="form-control" rows="2" placeholder="Catatan untuk audit trail"></textarea></label><button v-if="currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL'].includes(exceptionDetail?.state)" class="button secondary" :disabled="!investigationNote.trim()" @click="addExceptionNote">Simpan Catatan</button><div v-if="currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL'].includes(exceptionDetail?.state)" class="evidence-upload"><input id="exception-evidence-input" class="hidden-input" type="file" accept=".pdf,.png,.jpg,.jpeg,.txt,application/pdf,image/png,image/jpeg,text/plain" @change="onEvidenceFile" /><span>{{ evidenceFile?.name || 'PDF, PNG, JPEG, TXT · maks. 5 MB' }}</span><button class="button secondary" @click="chooseEvidenceFile">Pilih Evidence</button><button class="button primary" :disabled="!evidenceFile" @click="uploadExceptionEvidence">Unggah</button></div></article>
        </section>

        <section v-else-if="page === 'reports'" class="reports-page">
          <article class="panel report-filters"><div class="filter-block"><label>Mulai</label><input v-model="reportFrom" class="form-control" type="date" /></div><div class="filter-block"><label>Sampai</label><input v-model="reportTo" class="form-control" type="date" /></div><div class="filter-block"><label>Rekening</label><GlobalDropdown v-model="reportAccountID" class="form-control"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }} · {{ account.bank_display_name }}</option></GlobalDropdown></div><div class="filter-block"><label>Cari batch</label><input v-model="reportQuery" class="form-control" placeholder="No. batch / rekening" /></div><div class="filter-block"><label>Status batch</label><GlobalDropdown v-model="reportStatus" class="form-control"><option value="">Semua status</option><option v-for="status in currentStatusOptionsForReport" :key="status" :value="status">{{ status }}</option></GlobalDropdown></div><div class="report-export"><label>Jenis laporan</label><div><GlobalDropdown v-model="reportExportType" aria-label="Jenis laporan"><option value="batches">Ringkasan batch</option><option value="transactions">Transaksi</option><option value="exceptions">Riwayat exception</option></GlobalDropdown><button class="button secondary" data-loading @click="downloadReport">Unduh CSV</button></div></div></article>
          <div class="report-summary-grid"><article v-for="card in [{label: 'Transaksi Internal', value: reportSummary.internal_count ?? 0, icon: '↗', tone: 'teal'}, {label: 'Transaksi Bank', value: reportSummary.bank_count ?? 0, icon: '↙', tone: 'blue'}, {label: 'Matched Pairs', value: reportSummary.matched_pairs ?? 0, icon: '✓', tone: 'teal'}, {label: 'Belum Cocok', value: reportSummary.unmatched_internal ?? 0, icon: '≠', tone: 'amber'}, {label: 'Exception Aktif', value: reportSummary.active_exceptions ?? 0, icon: '!', tone: 'orange'}, {label: 'Match Rate', value: `${Number(reportSummary.match_rate ?? 0).toFixed(1)}%`, icon: '%', tone: 'violet'}, {label: 'Rata-rata Matching', value: `${Number(reportSummary.average_match_seconds ?? 0).toFixed(1)} dtk`, icon: '◷', tone: 'slate'}]" :key="card.label" class="report-stat" :class="`tone-${card.tone}`"><div><span>{{ card.label }}</span><strong>{{ card.value }}</strong></div><span class="report-stat-icon">{{ card.icon }}</span></article></div>
          <div class="report-chart-grid"><article class="panel report-chart"><div class="panel-heading"><div><h2>Status Batch</h2><p>Distribusi batch sesuai filter</p></div></div><div v-if="reportStatusChart.total" class="report-donut-layout"><div class="report-donut" :style="{ background: reportStatusChart.background }"><div><strong>{{ reportStatusChart.total }}</strong><span>Batch</span></div></div><div class="report-legend"><div v-for="item in reportStatusChart.segments" :key="item.name"><span><i :style="{background:item.color}"></i>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div></div></div><p v-else class="report-empty">Belum ada batch pada filter ini.</p></article><article class="panel report-chart"><div class="panel-heading"><div><h2>Exception Aktif</h2><p>Jenis selisih pada batch terpilih</p></div></div><div v-if="reportExceptionChart.length" class="report-bars"><div v-for="item in reportExceptionChart" :key="item.name"><div><span>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div><div class="report-bar-track"><i :style="{width:item.width}"></i></div></div></div><p v-else class="report-empty">Tidak ada exception aktif.</p></article></div>
          <article class="panel table-panel"><div class="panel-heading"><div><h2>Batch dalam laporan</h2><p>{{ reportSummary.total_batches ?? 0 }} batch sesuai filter; ekspor memakai filter yang sama.</p></div></div><p v-if="reportLoading" class="table-state">Memuat laporan…</p><p v-else-if="reportForbidden" class="table-state error-state">Akses laporan tidak diizinkan.</p><p v-else-if="reportError" class="table-state error-state">{{ reportError }} <button class="button secondary" @click="loadReport">Coba lagi</button></p><DataTable v-else :columns="tableConfig.batches!.columns" :rows="batchRows" @open="openTableRow" /><div class="table-footer"><span>Menampilkan {{ firstVisibleRow }}–{{ lastVisibleRow }} dari {{ paginationTotal }} batch</span><div><button :disabled="currentPage <= 1" @click="currentPage--">‹</button><button class="current-page">{{ currentPage }} / {{ pageCount }}</button><button :disabled="currentPage >= pageCount" @click="currentPage++">›</button></div></div></article>
        </section>

        <section v-else-if="page === 'settings'" class="settings-page">
          <nav class="settings-nav panel" aria-label="Kategori pengaturan">
            <button v-for="s in ['Umum','Parameter Matching','Integrasi','Notifikasi','Manajemen Role']" :key="s" :class="{ selected: activeTab === s || (activeTab === 'All' && s === 'Umum') }" @click="activeTab = s">{{ s }} <span>›</span></button>
          </nav>
          <article class="panel settings-card">
            <div class="panel-heading"><div><h2>{{ activeTab === 'All' ? 'Umum' : activeTab }}</h2><p v-if="activeTab === 'Umum' || activeTab === 'All'">Atur format dan batas penggunaan aplikasi.</p><p v-else-if="activeTab === 'Parameter Matching'">Aturan pencocokan yang digunakan saat ini.</p><p v-else-if="activeTab === 'Integrasi'">Sumber data yang tersedia pada versi ini.</p><p v-else-if="activeTab === 'Notifikasi'">Peristiwa yang ditampilkan pada notifikasi aplikasi.</p><p v-else>Ringkasan peran dan akses pengguna.</p></div></div>
            <section v-if="activeTab === 'Umum' || activeTab === 'All'" class="settings-section">
              <div class="settings-row"><div><b>Zona Waktu Default</b><small>Digunakan untuk tanggal batch dan audit log.</small></div><GlobalDropdown v-model="timezoneSetting" :disabled="currentUser?.role !== 'ADMIN'" class="form-control compact"><option value="Asia/Jakarta">Asia/Jakarta (WIB, UTC+7)</option></GlobalDropdown></div>
              <div class="settings-row"><div><b>Maksimum Ukuran Import</b><small>Batas ukuran file transaksi yang diterima.</small></div><GlobalDropdown v-model.number="maxImportSetting" :disabled="currentUser?.role !== 'ADMIN'" class="form-control compact"><option :value="5242880">5 MB</option><option :value="10485760">10 MB</option></GlobalDropdown></div>
              <div class="settings-row"><div><b>Format CSV</b><small>Delimiter untuk file transaksi.</small></div><GlobalDropdown v-model="csvDelimiterSetting" :disabled="currentUser?.role !== 'ADMIN'" class="form-control compact"><option value=",">Koma (,)</option></GlobalDropdown></div>
              <button v-if="currentUser?.role === 'ADMIN'" class="button primary settings-save" @click="saveSettings">Simpan Perubahan</button>
            </section>
            <section v-else-if="activeTab === 'Parameter Matching'" class="settings-section">
              <div class="matching-rule"><span class="rule-code">M01</span><div><b>Auto Exact Match</b><small>Reference, tanggal, arah, mata uang, dan nominal harus sama.</small></div><span class="readonly-chip">Aktif · Read-only</span></div>
              <div class="settings-row"><div><b>Toleransi Selisih Tanggal</b><small>Pengajuan manual memerlukan persetujuan Checker.</small></div><span class="settings-value">3 hari kalender</span></div>
              <p class="settings-note">Aturan matching ditetapkan pada versi aplikasi ini dan ditampilkan sebagai informasi.</p>
            </section>
            <section v-else-if="activeTab === 'Integrasi'" class="settings-section">
              <div class="settings-row"><div><b>Import CSV</b><small>Unggah data internal dan bank melalui menu Import Data.</small></div><span class="settings-value">Tersedia</span></div>
              <div class="settings-row"><div><b>Koneksi ke sistem eksternal</b><small>Pengaturan koneksi bank atau payment gateway belum tersedia.</small></div><span class="settings-value muted">Belum tersedia</span></div>
            </section>
            <section v-else-if="activeTab === 'Notifikasi'" class="settings-section">
              <div class="settings-row"><div><b>Notifikasi dalam aplikasi</b><small>Ditampilkan melalui ikon lonceng pada bagian atas halaman.</small></div><span class="settings-value">Tersedia</span></div>
              <div class="settings-row"><div><b>Aktivitas yang diberitahukan</b><small>Import gagal, hasil matching, pengajuan dan keputusan approval, serta penutupan batch.</small></div></div>
              <div class="settings-row"><div><b>Email dan saluran eksternal</b><small>Pengiriman notifikasi melalui saluran eksternal belum tersedia.</small></div><span class="settings-value muted">Belum tersedia</span></div>
            </section>
            <section v-else class="settings-section">
              <div v-for="role in roleCards" :key="role.name" class="settings-row"><div><b>{{ role.name }}</b><small>{{ role.description }}</small></div></div>
              <p class="settings-note">Daftar role ditetapkan oleh aplikasi. Admin dapat mengatur role setiap pengguna.</p>
              <button v-if="currentUser?.role === 'ADMIN'" class="button secondary settings-save" @click="navigate('users')">Buka Users &amp; Access</button>
            </section>
          </article>
        </section>
        <section v-else class="list-page"><article v-if="page === 'matching'" class="panel matching-batch-panel"><div class="matching-batch-heading"><div><h2>Daftar batch</h2><p>Pilih batch untuk melihat hasil. Jalankan matching pada batch berstatus READY.</p></div><input v-model="matchingBatchSearch" class="form-control matching-batch-search" aria-label="Cari batch" placeholder="Cari nomor batch atau rekening" /></div><p v-if="matchingBatchLoading" class="table-state">Memuat batch…</p><p v-else-if="matchingBatchError" class="table-state error-state">{{ matchingBatchError }} <button class="button secondary" @click="loadMatchingBatchOptions">Coba lagi</button></p><DataTable v-else :columns="[{ key: 'batch', label: 'No. Batch' }, { key: 'account', label: 'Rekening' }, { key: 'period', label: 'Periode' }, { key: 'status', label: 'Status' }]" :rows="matchingBatchRows" link-key="batch" @open="selectedBatchID = $event.id"><template #actions="{ row }"><div class="matching-row-actions"><button class="matching-view-button" type="button" :aria-label="`Lihat hasil ${row.batch}`" @click="openMatchingBatchPopup(row.id)"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z"/><circle cx="12" cy="12" r="2.5"/></svg>{{ selectedBatchID === row.id ? 'Dipilih' : 'Lihat hasil' }}</button><button v-if="currentUser?.role === 'OPERATOR' && row.status === 'READY'" class="matching-run-button" type="button" :disabled="busy" :aria-label="`Jalankan matching batch ${row.batch}`" @click="runMatching(row.id)"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 5 11 7-11 7V5Z"/></svg>Jalankan Matching</button></div></template></DataTable><div class="table-footer"><span>{{ matchingBatchTotal }} batch ditemukan</span><div><button :disabled="matchingBatchPage <= 1 || matchingBatchLoading" @click="matchingBatchPage--">‹</button><button class="current-page">{{ matchingBatchPage }} / {{ Math.max(1, Math.ceil(matchingBatchTotal / 25)) }}</button><button :disabled="matchingBatchPage >= Math.ceil(matchingBatchTotal / 25) || matchingBatchLoading" @click="matchingBatchPage++">›</button></div></div></article><div v-if="page === 'matching'" class="matching-results-heading"><h2>Hasil batch: {{ selectedBatch?.batch_no || 'Belum dipilih' }}</h2><p>Pilih batch pada tabel di atas untuk menampilkan hasilnya.</p></div><div v-if="page === 'matching'" class="kpi-grid matching-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>{{ selectedBatch?.internal_count ?? 0 }}</strong><small class="trend neutral-text">Batch terpilih</small></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>{{ selectedBatch?.bank_count ?? 0 }}</strong><small class="trend neutral-text">Batch terpilih</small></article><article class="kpi-card"><div class="kpi-top">Matched Pairs</div><strong>{{ paginationTotal }}</strong><small class="trend positive">{{ matchingProgress === "PROCESSING" ? "Matching berjalan..." : "Rule M01 · exact match" }}</small></article><article class="kpi-card"><div class="kpi-top">Exceptions</div><strong>{{ selectedBatch?.active_exceptions ?? 0 }}</strong><small class="trend warning-text">{{ selectedBatch?.batch_no ?? 'Pilih batch' }}</small></article></div><div v-if="page === 'exceptions'" class="exception-summary" aria-label="Ringkasan exception aktif"><article v-for="item in [{ kind: 'AMOUNT_MISMATCH', label: 'Selisih nominal', description: 'Nominal transaksi berbeda', icon: '≠', tone: 'amber' }, { kind: 'DIRECTION_MISMATCH', label: 'Perbedaan arah', description: 'Arah transaksi berbeda', icon: '↔', tone: 'blue' }, { kind: 'DATE_VARIANCE', label: 'Selisih tanggal', description: 'Tanggal transaksi berbeda', icon: '◷', tone: 'violet' }, { kind: 'AMBIGUOUS_MATCH', label: 'Kecocokan ganda', description: 'Lebih dari satu kemungkinan', icon: '⌘', tone: 'orange' }, { kind: 'INTERNAL_ONLY', label: 'Hanya internal', description: 'Tidak ditemukan pada data bank', icon: '↗', tone: 'teal' }, { kind: 'BANK_ONLY', label: 'Hanya bank', description: 'Tidak ditemukan pada data internal', icon: '↙', tone: 'slate' }]" :key="item.kind" class="exception-summary-card" :class="`tone-${item.tone}`"><div class="exception-card-heading"><span class="exception-card-icon" aria-hidden="true">{{ item.icon }}</span><span class="exception-card-count">{{ exceptionTypeCounts[item.kind] ?? 0 }}</span></div><div class="exception-card-content"><h3>{{ item.label }}</h3><p>{{ item.description }}</p></div><div class="exception-card-footer"><span>{{ item.kind }}</span><span>Aktif</span></div></article></div><article class="panel table-panel"><div class="table-toolbar"><div class="tabs"><button v-for="tab in currentTableTabs" :key="tab" :class="{ selected: activeTab === tab || (activeTab === 'All' && tab.startsWith('Semua')) || (activeTab === 'Menunggu' && tab.startsWith('Menunggu')) }" @click="activeTab = tab">{{ tab }}</button></div><div class="filters"><label class="search-field"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" :placeholder="page === 'exceptions' ? 'Cari reference / batch...' : 'Cari batch / reference...'" /></label><GlobalDropdown v-if="['batches','transactions'].includes(page)" v-model="sourceFilter" class="filter-select"><option>All sources</option><option>INTERNAL</option><option>BANK</option></GlobalDropdown><GlobalDropdown v-model="statusFilter" class="filter-select"><option>All statuses</option><option v-for="option in currentStatusOptions" :key="option">{{ option }}</option></GlobalDropdown><template v-if="page === 'users'"><GlobalDropdown v-model="userRoleFilter" class="filter-select"><option value="">Semua role</option><option value="ADMIN">Admin</option><option value="OPERATOR">Operator</option><option value="CHECKER">Checker</option><option value="AUDITOR">Auditor</option></GlobalDropdown></template><template v-if="page === 'batches'"><GlobalDropdown v-model="batchAccountFilter" class="filter-select"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }}</option></GlobalDropdown><input v-model="filterFrom" class="filter-select" type="date" aria-label="Periode dari"/><input v-model="filterTo" class="filter-select" type="date" aria-label="Periode sampai"/></template><template v-if="page === 'transactions'"><GlobalDropdown v-model="transactionBatchFilter" class="filter-select"><option value="">Semua batch</option><option v-for="batch in rawBatches" :key="batch.id" :value="batch.id">{{ batch.batch_no }}</option></GlobalDropdown></template><template v-if="page === 'audit'"><GlobalDropdown v-model="auditEntityFilter" class="filter-select"><option value="">Semua entitas</option><option value="batch">Batch</option><option value="exception">Exception</option><option value="transaction">Transaction</option><option value="user">User</option><option value="approval">Approval</option></GlobalDropdown><input v-model="auditActorFilter" class="filter-select" placeholder="Pengguna"/><input v-model="filterFrom" class="filter-select" type="date" aria-label="Dari tanggal"/><input v-model="filterTo" class="filter-select" type="date" aria-label="Sampai tanggal"/></template><button v-if="page === 'exceptions'" type="button" class="exception-filter-toggle" :class="{ active: exceptionFiltersOpen }" :aria-expanded="exceptionFiltersOpen" aria-controls="exception-filters" @click="exceptionFiltersOpen = !exceptionFiltersOpen"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M7 12h10M10 17h4" /></svg><span>{{ exceptionFiltersOpen ? 'Tutup filter' : 'Filter' }}</span><span v-if="exceptionActiveFilterCount" class="exception-filter-count">{{ exceptionActiveFilterCount }}</span></button><div v-if="page === 'exceptions'" v-show="exceptionFiltersOpen" id="exception-filters" class="exception-filter-panel"><GlobalDropdown v-model="typeFilter" class="filter-select"><option>All types</option><option v-for="kind in ['AMOUNT_MISMATCH','DIRECTION_MISMATCH','DATE_VARIANCE','AMBIGUOUS_MATCH','INTERNAL_ONLY','BANK_ONLY']" :key="kind">{{ kind }}</option></GlobalDropdown><GlobalDropdown v-model="batchFilter" class="filter-select"><option value="">Semua batch</option><option v-for="batch in rawBatches" :key="batch.id" :value="batch.id">{{ batch.batch_no }}</option></GlobalDropdown><GlobalDropdown v-model="exceptionAccountFilter" class="filter-select"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }}</option></GlobalDropdown><GlobalDropdown v-model="assigneeFilter" class="filter-select"><option value="">Semua assignee</option><option v-for="user in rawUsers" :key="user.id" :value="user.id">{{ user.full_name }}</option></GlobalDropdown><input v-model="filterFrom" class="filter-select" type="date" aria-label="Dari tanggal"/><input v-model="filterTo" class="filter-select" type="date" aria-label="Sampai tanggal"/><GlobalDropdown v-model="exceptionAgeFilter" class="filter-select"><option value="">Semua umur</option><option value="UNDER_24H">Kurang dari 24 jam</option><option value="24_72H">24–72 jam</option><option value="OVER_72H">Lebih dari 72 jam</option></GlobalDropdown></div><button v-if="page !== 'exceptions'" class="filter-icon" aria-label="Filter">☷</button></div></div><p v-if="listLoading" class="table-state">Memuat data…</p><p v-else-if="listForbidden" class="table-state error-state">Akses ke menu ini tidak diizinkan.</p><p v-else-if="listError" class="table-state error-state">{{ listError }} <button class="button secondary" @click="loadCurrentPage">Coba lagi</button></p><DataTable v-else-if="currentTable" :columns="currentTable.columns" :rows="paginatedRows" :hide-actions="page === 'matching'" :action-icon="page === 'users' || (page === 'accounts' && currentUser?.role === 'ADMIN') ? 'edit' : page === 'approvals' ? 'review' : page === 'audit' ? 'history' : 'view'" :action-label="page === 'users' || (page === 'accounts' && currentUser?.role === 'ADMIN') ? 'Ubah data' : page === 'approvals' ? 'Tinjau approval' : page === 'audit' ? 'Lihat riwayat' : 'Lihat detail'" :link-key="page === 'batches' ? 'batch' : page === 'exceptions' ? 'ref' : ''" @open="openTableRow" /><div class="table-footer"><span>Menampilkan {{ firstVisibleRow }}–{{ lastVisibleRow }} dari {{ paginationTotal }} baris</span><div><button :disabled="currentPage <= 1" @click="currentPage--">‹</button><button class="current-page">{{ currentPage }} / {{ pageCount }}</button><button :disabled="currentPage >= pageCount" @click="currentPage++">›</button></div></div></article></section>

        <section v-if="page === 'users'" class="panel role-matrix-panel"><div class="panel-heading"><div><h2>Role &amp; Access Control</h2><p>Role tetap MVP · matrix baca-saja · bukan role builder dinamis.</p></div><span class="badge neutral">RBAC</span></div><div class="role-cards"><article v-for="role in roleCards" :key="role.name" class="role-card"><span :class="['role-symbol', role.tone]">{{ role.name.slice(0, 1) }}</span><div><b>{{ role.name }}</b><small>{{ role.description }}</small></div></article></div><div class="role-table-scroll"><table class="role-table"><thead><tr><th>Fitur</th><th>Admin</th><th>Operator</th><th>Checker</th><th>Auditor</th></tr></thead><tbody><tr v-for="row in roleMatrixRows" :key="row.feature"><td>{{ row.feature }}</td><td>{{ row.admin }}</td><td>{{ row.operator }}</td><td>{{ row.checker }}</td><td>{{ row.auditor }}</td></tr></tbody></table></div><div class="sod-note"><b>Segregation of duties</b><span>Checker tidak boleh menyetujui permintaan sendiri. Admin tidak mendapat bypass approval. Akun nonaktif kehilangan akses sesi.</span></div></section>

        <footer class="app-footer">ReconX <span>·</span> Bank Transaction Reconciliation &amp; Exception Management <span class="footer-version">v0.2 · Modular monolith</span></footer>
      </div>
    </main>
    </template>

    <Transition name="modal"><div v-if="successMessage" class="modal-backdrop success-alert-backdrop"><section class="success-alert" role="alertdialog" aria-modal="true" aria-labelledby="success-alert-title" aria-describedby="success-alert-message"><div class="success-alert-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg></div><h2 id="success-alert-title">Berhasil</h2><p id="success-alert-message">{{ successMessage }}</p><button class="button primary" type="button" data-no-loading autofocus @click="successMessage = ''">OK</button></section></div></Transition>
    <Transition name="modal"><div v-if="showImportError" class="modal-backdrop import-error-backdrop" @click.self="showImportError = false"><section class="import-error-dialog" role="alertdialog" aria-modal="true" aria-labelledby="import-error-title" aria-describedby="import-error-description"><div class="import-error-icon" aria-hidden="true">!</div><h2 id="import-error-title">Import belum berhasil</h2><p id="import-error-description">{{ importError }}</p><div v-if="importErrors.length" class="import-error-details"><b>Rincian validasi</b><ul><li v-for="(error, index) in importErrors" :key="index"><strong>Baris {{ error.line }} · {{ error.field }}</strong><span>{{ importValidationMessage(error.message) }}</span></li></ul></div><p class="import-error-help">Perbaiki file dan unggah kembali. Laporan error dapat diunduh dari riwayat jika tersedia.</p><button class="button primary" data-no-loading @click="showImportError = false">Mengerti</button></section></div></Transition>
    <Transition name="modal"><div v-if="showLogoutConfirm" class="modal-backdrop" @click.self="showLogoutConfirm = false"><section class="logout-confirm" role="dialog" aria-modal="true" aria-labelledby="logout-title"><h2 id="logout-title">Keluar dari akun?</h2><p>Anda akan kembali ke halaman login.</p><div class="modal-actions"><button class="button secondary" data-no-loading @click="showLogoutConfirm = false">Batal</button><button class="button primary" data-no-loading @click="logout">Keluar</button></div></section></div></Transition>
    <Transition name="modal"><div v-if="detailPopup" class="detail-popup-backdrop" @click.self="closeDetailPopup"><section class="detail-popup" role="dialog" aria-modal="true" :aria-label="detailPopup === 'batch' ? 'Detail batch' : 'Detail exception'"><header class="detail-popup-header"><div><h2>{{ detailPopup === 'batch' ? 'Detail Batch' : 'Detail Exception' }}</h2><p>{{ detailPopup === 'batch' ? 'Ringkasan dan tindakan batch rekonsiliasi' : 'Informasi dan penanganan selisih' }}</p></div><button type="button" class="close-button" data-no-loading aria-label="Tutup detail" @click="closeDetailPopup">×</button></header><div class="detail-popup-body">
        <section v-if="detailPopup === 'batch'" class="detail-page batch-detail-page">

          <div class="detail-title"><div><div class="title-line"><h2>{{ selectedBatch?.batch_no || 'Batch tidak dipilih' }}</h2><span class="badge" :class="badgeClass(selectedBatch?.state || '')">{{ selectedBatch?.state || '—' }}</span></div><p>{{ selectedBatch?.remarks || selectedBatch?.bank_display_name || 'Tanpa catatan' }} <span>·</span> {{ formatDate(selectedBatch?.period_from) }} – {{ formatDate(selectedBatch?.period_to) }} · {{ selectedBatch?.account_code }}</p></div><button v-if="currentUser?.role !== 'OPERATOR' || selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="openMatchingResults">Hasil Matching</button><button v-if="selectedBatch?.state === 'DRAFT' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="openBatchEditor">Edit Batch</button><button v-if="selectedBatch?.state === 'DRAFT' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="transitionBatch('cancel')">Batalkan</button><button v-if="selectedBatch?.state === 'REVIEW' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="transitionBatch('reopen')">Koreksi Sumber</button><button v-if="selectedBatch?.state === 'REVIEW' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="submitBatch">Ajukan Approval</button><button v-if="selectedBatch?.state === 'FAILED' && currentUser?.role === 'OPERATOR' && selectedBatch?.created_by === currentUser?.id" class="button secondary" :disabled="busy" @click="transitionBatch('retry')">Coba Matching Lagi</button><button v-if="selectedBatch?.state === 'APPROVED' && currentUser?.role === 'CHECKER'" class="button primary" :disabled="busy" @click="closeBatch">Tutup Batch</button></div>
          <div class="kpi-grid detail-kpis"><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Transaksi Internal</span><span class="detail-metric-icon teal">↗</span></div><strong>{{ selectedBatch?.internal_count ?? 0 }}</strong><small>Data dari internal</small></article><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Transaksi Bank</span><span class="detail-metric-icon blue">↙</span></div><strong>{{ selectedBatch?.bank_count ?? 0 }}</strong><small>Data dari bank</small></article><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Matched</span><span class="detail-metric-icon green">✓</span></div><strong>{{ selectedBatch?.matched_pairs ?? 0 }}</strong><small>Pasangan transaksi</small></article><article class="kpi-card"><div class="detail-metric-head"><span class="kpi-top">Exceptions Aktif</span><span class="detail-metric-icon amber">!</span></div><strong>{{ selectedBatch?.active_exceptions ?? 0 }}</strong><small>Memerlukan tindak lanjut</small></article></div>
          <article class="panel detail-tabs"><div class="tabs"><button v-for="tab in batchTabs" :key="tab" :class="{ selected: activeTab === tab }" @click="activeTab = tab">{{ tab }}</button></div><div v-if="activeTab === 'Ringkasan'" class="detail-summary"><div><h3>Ringkasan</h3><p>Ringkasan hasil rekonsiliasi untuk batch ini.</p><div class="match-progress"><span>Matched</span><div><i :style="{ width: `${selectedBatch?.internal_count ? selectedBatch.matched_pairs * 100 / selectedBatch.internal_count : 0}%` }"></i></div><b>{{ selectedBatch?.matched_pairs ?? 0 }} pasangan</b></div><div class="match-progress unmatched-progress"><span>Exceptions</span><div><i :style="{ width: `${selectedBatch?.internal_count ? selectedBatch.active_exceptions * 100 / selectedBatch.internal_count : 0}%` }"></i></div><b>{{ selectedBatch?.active_exceptions ?? 0 }} aktif</b></div></div><div class="batch-state"><span class="state-check">!</span><b>{{ selectedBatch?.state === 'CLOSED' ? 'Batch ditutup' : selectedBatch?.state === 'CANCELLED' ? 'Batch dibatalkan' : selectedBatch?.state === 'DRAFT' ? 'Batch masih dalam persiapan' : selectedBatch?.state === 'PENDING_APPROVAL' ? 'Menunggu persetujuan' : selectedBatch?.state === 'APPROVED' ? 'Siap ditutup' : selectedBatch?.active_exceptions ? 'Belum dapat ditutup' : 'Periksa status batch' }}</b><small>{{ selectedBatch?.state === 'CANCELLED' ? 'Batch ini telah dibatalkan.' : selectedBatch?.state === 'CLOSED' ? 'Hasil rekonsiliasi telah dikunci.' : 'Batch dapat diajukan setelah seluruh data cocok dan tidak ada exception atau approval tertunda.' }}</small></div></div><div v-else-if="activeTab === 'Imports'"><div v-for="item in rawImports" :key="item.id" class="history-item"><span class="file-icon">CSV</span><span class="file-info"><b>{{ item.original_name }}</b><small>{{ item.source_type }} · {{ item.state }} · {{ item.rows_count }} baris · {{ formatDate(item.created_at) }}</small></span><button class="link-button" @click="downloadOriginalImport(item)">File</button><button v-if="item.has_error_report" class="link-button" @click="downloadImportErrors(item)">Unduh Error</button></div><div v-if="!rawImports.length" class="detail-empty"><span class="detail-empty-icon">⇧</span><strong>Belum ada file import</strong><p>File yang diunggah untuk batch ini akan ditampilkan di sini.</p></div></div><div v-else-if="activeTab === 'Detail Transaksi'"><DataTable :columns="[{ key: 'ref', label: 'Reference' }, { key: 'date', label: 'Tanggal' }, { key: 'source', label: 'Sumber' }, { key: 'description', label: 'Arah' }, { key: 'debit', label: 'Debit' }, { key: 'credit', label: 'Kredit' }, { key: 'status', label: 'Status' }]" :rows="batchTransactionRows" @open="openTransaction" /></div><div v-else-if="activeTab === 'Matching Results'"><DataTable :columns="tableConfig.matching!.columns" :rows="matchingRows" /></div><div v-else-if="activeTab === 'Exceptions'"><DataTable :columns="tableConfig.exceptions!.columns" :rows="batchExceptionRows" link-key="ref" @open="openBatchException" /></div><div v-else-if="activeTab === 'Approvals'"><DataTable :columns="tableConfig.approvals!.columns" :rows="batchApprovalRows" @open="openBatchApproval" /></div><div v-else><DataTable :columns="tableConfig.audit!.columns" :rows="batchAuditRows" /></div></article>
        </section>

        <section v-else-if="detailPopup === 'exception'" class="detail-page exception-detail-page">
          <div class="detail-title"><div><div class="title-line"><h2>{{ exceptionDetail?.reference_id || 'Exception' }}</h2><span class="badge warning">{{ exceptionDetail?.type || '—' }}</span></div><p>{{ exceptionDetail?.batch_no }} <span>·</span> {{ exceptionDetail?.state }} <span>·</span> PIC: {{ exceptionDetail?.assignee_name || 'Belum ditugaskan' }}</p></div><button v-if="exceptionDetail?.batch_id" class="button secondary" data-loading @click="openExceptionBatch">Lihat batch</button><div v-if="currentUser?.role === 'OPERATOR' && exceptionDetail?.can_assign_others && ['OPEN','INVESTIGATING'].includes(exceptionDetail?.state)" class="exception-assignment"><input v-model="operatorSearch" class="form-control" aria-label="Cari operator" placeholder="Cari nama Operator" /><GlobalDropdown v-model="assignmentTarget" class="form-control" aria-label="Pilih penanggung jawab"><option :value="currentUser?.id || ''">Saya · {{ currentUser?.full_name }}</option><option v-for="operator in exceptionOperators.filter(item => item.id !== currentUser?.id)" :key="operator.id" :value="operator.id">{{ operator.full_name }}</option></GlobalDropdown><button class="button secondary" :disabled="!assignmentTarget" @click="assignException">Tugaskan</button></div><button v-if="exceptionDetail?.type === 'DATE_VARIANCE' && currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING'].includes(exceptionDetail?.state)" class="button primary" data-loading @click="modal = 'proposal'">Ajukan Manual Match</button></div>
          <div class="comparison-grid"><article class="panel"><div class="panel-heading"><div><h2>Data Internal</h2><p>{{ exceptionDetail?.internal?.source_type || 'INTERNAL' }}</p></div></div><div class="data-pair"><span>Reference ID</span><b>{{ exceptionDetail?.internal?.reference_id || '—' }}</b></div><div class="data-pair"><span>Tanggal</span><b>{{ formatDate(exceptionDetail?.internal?.date) }}</b></div><div class="data-pair"><span>Arah</span><b>{{ exceptionDetail?.internal?.direction || '—' }}</b></div><div class="data-pair"><span>Nominal</span><b>{{ money(exceptionDetail?.internal?.amount_rupiah) }}</b></div></article><article class="panel"><div class="panel-heading"><div><h2>Data Bank</h2><p>{{ exceptionDetail?.bank?.source_type || 'BANK' }}</p></div></div><div class="data-pair"><span>Reference ID</span><b>{{ exceptionDetail?.bank?.reference_id || '—' }}</b></div><div class="data-pair"><span>Tanggal</span><b>{{ formatDate(exceptionDetail?.bank?.date) }}</b></div><div class="data-pair"><span>Arah</span><b>{{ exceptionDetail?.bank?.direction || '—' }}</b></div><div class="data-pair mismatch"><span>Nominal</span><b>{{ money(exceptionDetail?.bank?.amount_rupiah) }}</b></div></article></div><article v-if="exceptionDetail?.type === 'AMBIGUOUS_MATCH'" class="panel ambiguity-panel"><h3>Transaksi dengan reference yang sama</h3><p>Reference ini muncul lebih dari sekali. Pilihannya perlu diperiksa dan sumber diperbaiki sebelum batch diajukan.</p><div class="candidate-table-wrap"><table class="candidate-table"><thead><tr><th>Sumber</th><th>Referensi</th><th>Tanggal</th><th>Arah</th><th>Nominal</th><th>Mata uang</th></tr></thead><tbody><tr v-for="candidate in exceptionDetail.candidates || []" :key="candidate.id"><td>{{ candidate.source_type }}</td><td>{{ candidate.reference_id }}</td><td>{{ formatDate(candidate.date) }}</td><td>{{ candidate.direction }}</td><td>{{ money(candidate.amount_rupiah) }}</td><td>{{ candidate.currency }}</td></tr></tbody></table></div><p v-if="!exceptionDetail.candidates?.length">Tidak ada kandidat yang tersedia.</p></article>
          <article class="panel timeline-panel"><div class="panel-heading"><div><h2>Riwayat Pemeriksaan</h2><p>Catatan dan assignment exception</p></div></div><div class="timeline"><div v-for="action in exceptionDetail?.actions || []" :key="action.id"><i></i><span><b>{{ action.action_type }} · {{ action.actor_name }}</b><small>{{ action.note || 'Tidak ada catatan' }} · {{ formatDate(action.created_at) }}</small><button v-if="action.has_evidence" class="link-button" @click="downloadExceptionEvidence(action)">Unduh {{ action.file_name || 'evidence' }}</button></span></div><p v-if="!exceptionDetail?.actions?.length">Belum ada tindakan.</p></div><label v-if="currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL'].includes(exceptionDetail?.state)" class="field-label">Catatan investigasi<textarea v-model="investigationNote" class="form-control" rows="2" placeholder="Catatan untuk audit trail"></textarea></label><button v-if="currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL'].includes(exceptionDetail?.state)" class="button secondary" :disabled="!investigationNote.trim()" @click="addExceptionNote">Simpan Catatan</button><div v-if="currentUser?.role === 'OPERATOR' && ['OPEN','INVESTIGATING','PENDING_ACTION_APPROVAL'].includes(exceptionDetail?.state)" class="evidence-upload"><input id="exception-evidence-input" class="hidden-input" type="file" accept=".pdf,.png,.jpg,.jpeg,.txt,application/pdf,image/png,image/jpeg,text/plain" @change="onEvidenceFile" /><span>{{ evidenceFile?.name || 'PDF, PNG, JPEG, TXT · maks. 5 MB' }}</span><button class="button secondary" @click="chooseEvidenceFile">Pilih Evidence</button><button class="button primary" :disabled="!evidenceFile" @click="uploadExceptionEvidence">Unggah</button></div></article>
        </section>

</div><footer class="detail-popup-footer"><button type="button" class="button secondary" data-no-loading @click="closeDetailPopup">Tutup</button></footer></section></div></Transition>
    <Transition name="modal"><div v-if="modal" class="modal-backdrop" @click.self="modal = ''"><section class="modal-card" :class="`modal-${modal}`"><div class="modal-heading"><div><h2>{{ modal === 'batch' ? 'Buat Batch Rekonsiliasi' : modal === 'editBatch' ? 'Edit Batch'  : modal === 'account' ? 'Tambah Sumber Data' : modal === 'accountEdit' ? 'Edit Rekening' : modal === 'accountView' ? 'Detail Rekening' : modal === 'proposal' ? 'Ajukan Manual Match' : modal === 'approval' ? 'Keputusan Approval' : modal === 'profile' ? 'Profil Saya' : modal === 'transaction' ? 'Detail Transaksi' : modal === 'auditDetail' ? 'Detail Audit Log' : modal === 'userActions' ? 'Akses Pengguna' : 'Tambah Pengguna' }}</h2><p>{{ modal === 'batch' || modal === 'editBatch' ? 'Tentukan rekening dan periode rekonsiliasi.' : modal === 'accountView' ? 'Informasi rekening sumber.' : modal === 'accountEdit' ? 'Perbarui rekening sumber. Rekening dengan batch aktif tidak dapat dinonaktifkan.' : modal === 'proposal' ? 'Permintaan ini akan ditinjau oleh Checker.' : modal === 'approval' ? approvalDetail?.reason : modal === 'profile' ? 'Informasi akun yang sedang digunakan.' : modal === 'transaction' ? 'Informasi transaksi dan hasil rekonsiliasi.' : modal === 'auditDetail' ? 'Ringkasan aktivitas dan perubahan yang tercatat.' : 'Lengkapi informasi untuk melanjutkan.' }}</p></div><button class="close-button" data-no-loading aria-label="Tutup" @click="modal = ''">×</button></div>
      <div class="modal-body">
      <template v-if="modal === 'transaction'"><div class="friendly-detail"><div class="friendly-highlight"><span>Nomor referensi</span><strong>{{ selectedTransaction?.reference_id || '—' }}</strong><small>{{ humanLabel(selectedTransaction?.status) }}</small></div><div class="friendly-grid"><div><span>Tanggal transaksi</span><strong>{{ formatDate(selectedTransaction?.date) }}</strong></div><div><span>Jenis transaksi</span><strong>{{ humanLabel(selectedTransaction?.direction) }}</strong></div><div><span>Nominal</span><strong>{{ money(selectedTransaction?.amount_rupiah) }}</strong></div><div><span>Sumber data</span><strong>{{ humanLabel(selectedTransaction?.source_type) }}</strong></div><div><span>Batch rekonsiliasi</span><strong>{{ transactionBatchLabel || 'Tidak tersedia' }}</strong></div><div><span>Mata uang</span><strong>{{ selectedTransaction?.currency || '—' }}</strong></div></div></div></template><template v-else-if="modal === 'auditDetail'"><div class="friendly-detail"><div class="friendly-highlight"><span>Aktivitas</span><strong>{{ humanLabel(auditDetail?.action) }}</strong><small>{{ humanLabel(auditDetail?.entity) }}</small></div><div class="friendly-grid"><div><span>Waktu</span><strong>{{ formatDateTime(auditDetail?.created_at) }}</strong></div><div><span>Pengguna</span><strong>{{ auditDetail?.actor || 'Sistem' }}</strong></div></div><div class="audit-change-grid"><section><h3>Sebelum</h3><div v-if="auditBefore.length" class="audit-change-list"><div v-for="field in auditBefore" :key="field.label"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div></div><p v-else>Tidak ada data sebelumnya.</p></section><section><h3>Sesudah</h3><div v-if="auditAfter.length" class="audit-change-list"><div v-for="field in auditAfter" :key="field.label"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div></div><p v-else>Tidak ada perubahan yang ditampilkan.</p></section></div></div></template><template v-else-if="modal === 'profile'"><label class="field-label">Nama lengkap<input class="form-control" :value="currentUser?.full_name" readonly /></label><label class="field-label">Email<input class="form-control" :value="currentUser?.email" readonly /></label><label class="field-label">Role<input class="form-control" :value="currentUser?.role" readonly /></label><div class="modal-actions"><button class="button secondary" data-no-loading @click="modal = ''">Tutup</button><button class="button primary" @click="logout">Keluar dari akun</button></div></template><template v-else-if="modal === 'batch' || modal === 'editBatch'"><label class="field-label">Rekening Sumber<GlobalDropdown v-model="batchForm.account_id" class="form-control" required><option value="" disabled>Pilih rekening</option><option v-for="account in rawAccounts.filter(item => item.active)" :key="account.id" :value="account.id">{{ account.account_code }} · {{ account.bank_display_name }}</option></GlobalDropdown></label><label class="field-label">Periode Batch<div class="date-pair"><input v-model="batchForm.period_from" class="form-control" type="date" /><input v-model="batchForm.period_to" class="form-control" type="date" /></div></label><label class="field-label">Catatan<input v-model="batchForm.remarks" class="form-control" placeholder="Opsional" /></label></template>
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
