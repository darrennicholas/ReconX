<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import DataTable from './components/DataTable.vue'
import { ApiError, apiRequest, downloadFile } from './api'

type Page = 'dashboard' | 'batches' | 'batchDetail' | 'imports' | 'transactions' | 'matching' | 'exceptions' | 'exceptionDetail' | 'approvals' | 'reports' | 'accounts' | 'users' | 'audit' | 'settings'
type Row = Record<string, string>

const page = ref<Page>('dashboard')
const apiStatus = ref<'checking' | 'online' | 'offline'>('checking')
const search = ref('')
const statusFilter = ref('All statuses')
const sourceFilter = ref('All sources')
const activeTab = ref('All')
const modal = ref('')
const toast = ref('')
const showNotifications = ref(false)
const selectedFile = ref('')
const fileToUpload = ref<File | null>(null)
const currentUser = ref<{ id: string; full_name: string; email: string; role: string } | null>(null)
const loginEmail = ref('')
const loginPassword = ref('')
const showLoginPassword = ref(false)
const loginError = ref('')
const busy = ref(false)
const selectedBatchID = ref('')
const selectedExceptionID = ref('')
const selectedApprovalID = ref('')
const selectedUserID = ref('')
const selectedAccountID = ref('')
const accountActiveForm = ref(true)
const selectedTransaction = ref<Record<string, any> | null>(null)
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
const dashboardFrom = ref('')
const dashboardTo = ref('')
const dashboardAccountID = ref('')

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
  if (page.value === 'matching') return [`Matched (${matchingRows.value.length})`, 'Semua']
  if (page.value === 'approvals') return ['Manual Matches', 'Batch Approvals', `Menunggu (${rawApprovals.value.filter(item => item.state === 'PENDING').length})`, 'Semua', 'Riwayat']
  if (page.value === 'exceptions') return [`Semua (${exceptionRows.value.length})`, 'Open', 'Investigating', 'Resolved']
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
const pageSize = 25
const currentPage = ref(1)
const paginatedRows = computed(() => filteredRows.value.slice((currentPage.value - 1) * pageSize, currentPage.value * pageSize))
const pageCount = computed(() => Math.max(1, Math.ceil(filteredRows.value.length / pageSize)))
const firstVisibleRow = computed(() => filteredRows.value.length ? (currentPage.value - 1) * pageSize + 1 : 0)
const lastVisibleRow = computed(() => Math.min(currentPage.value * pageSize, filteredRows.value.length))
watch([page, search, statusFilter, sourceFilter, activeTab], () => { currentPage.value = 1 })
const filteredRows = computed(() => {
  const rows = currentTable.value?.rows ?? []
  const q = search.value.trim().toLowerCase()
  return rows.filter(row => {
    const matchesSearch = !q || Object.values(row).some(value => value.toLowerCase().includes(q))
    const matchesStatus = statusFilter.value === 'All statuses' || row.status === statusFilter.value
    const matchesSource = sourceFilter.value === 'All sources' || row.source === sourceFilter.value
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
  if (page.value === 'imports' && role === 'OPERATOR') return 'Unggah File'
  if (page.value === 'matching' && role === 'OPERATOR') return 'Jalankan Matching'
  if (page.value === 'exceptions' && role === 'OPERATOR') return 'Investigasi'
  if (page.value === 'accounts' && role === 'ADMIN') return 'Tambah Sumber'
  if (page.value === 'users' && role === 'ADMIN') return 'Tambah Pengguna'
  if (page.value === 'reports') return 'Unduh Laporan'
  return ''
})

onMounted(async () => {
  try {
    const response = await fetch('/api/v1/health')
    apiStatus.value = response.ok ? 'online' : 'offline'
    if (response.ok) {
      try {
        const { data } = await apiRequest<typeof currentUser.value>('/api/v1/auth/me')
        currentUser.value = data
        await loadWorkspace()
      } catch { /* Login is required, or database setup has not been completed. */ }
    }
  } catch {
    apiStatus.value = 'offline'
  }
})

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
  const batches = value<Record<string, any>[]>(1); if (batches) {
    rawBatches.value = batches
    batchRows.value.splice(0, batchRows.value.length, ...batches.map((item: any) => ({ id: item.id, batch: item.batch_no, name: item.remarks || item.bank_display_name, source: item.account_code, period: `${formatDate(item.period_from)} – ${formatDate(item.period_to)}`, total: `${item.internal_count} + ${item.bank_count}`, rate: `${(item.internal_count ? item.matched_pairs * 100 / item.internal_count : 0).toFixed(1)}%`, status: item.state, updated: formatDate(item.updated_at) })))
    if (!batches.some(item => item.id === selectedBatchID.value)) selectedBatchID.value = batches[0]?.id ?? ''
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
}

async function loadDashboard() {
  if (dashboardFrom.value && dashboardTo.value && dashboardFrom.value > dashboardTo.value) {
    showToast('Tanggal mulai tidak boleh melewati tanggal akhir.')
    return
  }
  const params = new URLSearchParams()
  if (dashboardFrom.value) params.set('from', dashboardFrom.value)
  if (dashboardTo.value) params.set('to', dashboardTo.value)
  if (dashboardAccountID.value) params.set('account_id', dashboardAccountID.value)
  try { dashboardSummary.value = (await apiRequest<Record<string, any>>(`/api/v1/dashboard/summary${params.size ? `?${params}` : ''}`)).data }
  catch (error) { showToast(error instanceof Error ? error.message : 'Ringkasan dashboard gagal dimuat.') }
}

async function loadMatches() {
  if (!selectedBatchID.value) { matchingRows.value.splice(0); return }
  try {
    const { data } = await apiRequest<Record<string, any>[]>(`/api/v1/batches/${selectedBatchID.value}/matches`)
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
  if (matchesResult.status === 'fulfilled') {
    const data = matchesResult.value.data
    rawMatches.value = data
    matchingRows.value.splice(0, matchingRows.value.length, ...data.map((item: any) => ({
      id: item.id, ref: item.reference_id, date: formatDate(item.internal_date),
      internal: `${item.internal_direction} · ${money(item.internal_amount_rupiah)}`,
      bank: `${item.bank_direction} · ${money(item.bank_amount_rupiah)}`,
      amount: money(item.internal_amount_rupiah), rule: item.rule_id, status: item.method,
    })))
  } else matchingRows.value = []
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
async function login() {
  busy.value = true; loginError.value = ''
  try {
    const { data } = await apiRequest<typeof currentUser.value>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ email: loginEmail.value, password: loginPassword.value }) })
    currentUser.value = data; loginPassword.value = ''; await loadWorkspace()
  } catch (error) { loginError.value = error instanceof Error ? error.message : 'Login gagal.' }
  finally { busy.value = false }
}

async function logout() {
  try { await apiRequest('/api/v1/auth/logout', { method: 'POST' }) } catch { /* Session may already be expired. */ }
  currentUser.value = null; selectedBatchID.value = ''; selectedExceptionID.value = ''; exceptionDetail.value = null; modal.value = ''
}

function navigate(id: string) {
  page.value = id as Page
  search.value = ''
  statusFilter.value = 'All statuses'
  activeTab.value = id === 'batchDetail' ? 'Ringkasan' : id === 'settings' ? 'Umum' : id === 'approvals' ? 'Menunggu' : 'All'
  if (id === 'imports' && !selectedBatchID.value && rawBatches.value.length) selectedBatchID.value = rawBatches.value[0].id
  if (id === 'imports') void loadImports()
}

function openFilePicker() {
  document.querySelector<HTMLInputElement>('#file-input')?.click()
}

async function openTableRow(row?: Row) {
  if (page.value === 'batches' || page.value === 'dashboard' || page.value === 'reports') { selectedBatchID.value = row?.id || selectedBatchID.value; navigate('batchDetail'); await Promise.all([loadMatches(), loadBatchDetails()]) }
  else if (page.value === 'transactions' && row?.id) await openTransaction(row)
  else if (page.value === 'exceptions') { selectedExceptionID.value = row?.id || selectedExceptionID.value; navigate('exceptionDetail'); await loadExceptionDetail() }
  else if (page.value === 'approvals') { selectedApprovalID.value = row?.id || ''; modal.value = 'approval'; decisionComment.value = '' }
  else if (page.value === 'users' && row?.id) {
    selectedUserID.value = row.id; resetPasswordValue.value = ''
    const user = rawUsers.value.find(item => item.id === row.id)
    if (user) userEditForm.value = { full_name: user.full_name, email: user.email, role: user.role }
    modal.value = 'userActions'
  } else if (page.value === 'accounts' && currentUser.value?.role === 'ADMIN' && row?.id) {
    const account = rawAccounts.value.find(item => item.id === row.id)
    if (!account) return
    selectedAccountID.value = account.id
    accountForm.value = { account_code: account.account_code, bank_display_name: account.bank_display_name, masked_account_no: account.masked_account_no, account_holder_name: account.account_holder_name }
    accountActiveForm.value = account.active
    modal.value = 'accountEdit'
  }
}
async function openTransaction(row: Row) {
  if (!row.id) return
  try {
    selectedTransaction.value = (await apiRequest<Record<string, any>>(`/api/v1/transactions/${row.id}`)).data
    modal.value = 'transaction'
  } catch (error) { showToast(error instanceof Error ? error.message : 'Detail transaksi tidak dapat dibuka.') }
}
function formatJSON(value: unknown) { return JSON.stringify(value ?? {}, null, 2) }

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
    selectedExceptionID.value = rawExceptions.value[0]?.id || ''
    navigate('exceptionDetail')
    await loadExceptionDetail()
    return
  }
  if (page.value === 'matching') { await runMatching(); return }
  if (page.value === 'approvals') {
    toast.value = 'Pilih permintaan dalam senarai untuk semakan.'
    window.setTimeout(() => (toast.value = ''), 2800)
    return
  }
  modal.value = page.value === 'accounts' ? 'account' : page.value === 'users' ? 'user' : 'batch'
}

function onFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (file) { fileToUpload.value = file; selectedFile.value = file.name }
}

async function downloadReport() {
  const params = new URLSearchParams()
  if (reportFrom.value) params.set('from', reportFrom.value)
  if (reportTo.value) params.set('to', reportTo.value)
  if (reportAccountID.value) params.set('account_id', reportAccountID.value)
  try { await downloadFile(`/api/v1/reports/reconciliations/export${params.size ? `?${params}` : ''}`, 'reconx-reconciliation.csv') }
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
    showToast('Perubahan berhasil disimpan.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Permintaan gagal.') }
  finally { busy.value = false }
}

async function uploadImport() {
  if (!fileToUpload.value || !selectedBatchID.value) { showToast('Pilih file dan batch terlebih dahulu.'); return }
  const form = new FormData(); form.set('file', fileToUpload.value); form.set('source_type', importSource.value)
  busy.value = true
  try {
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/imports`, { method: 'POST', idempotent: true, body: form })
    fileToUpload.value = null; selectedFile.value = ''; await loadWorkspace(); await loadImports(); showToast('CSV tervalidasi dan diimpor.')
  } catch (error) {
    const data = error instanceof ApiError ? error.data as { errors?: { line: number; field: string; message: string }[] } | undefined : undefined
    const first = data?.errors?.[0]
    await loadImports()
    showToast(first ? `Validasi CSV baris ${first.line} (${first.field}): ${first.message}` : error instanceof Error ? error.message : 'Import gagal.')
  } finally { busy.value = false }
}

async function runMatching() {
  if (!selectedBatchID.value) { showToast('Buat batch terlebih dahulu.'); return }
  busy.value = true
  try {
    const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${selectedBatchID.value}`)
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/match-runs`, { method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}' })
    await loadWorkspace(); showToast('Matching selesai; batch masuk tahap review.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Matching gagal.') }
  finally { busy.value = false }
}

const exceptionDetail = ref<Record<string, any> | null>(null)
const approvalDetail = computed(() => rawApprovals.value.find(item => item.id === selectedApprovalID.value) ?? null)
const selectedBatch = computed(() => rawBatches.value.find(item => item.id === selectedBatchID.value) ?? null)
const selectedUser = computed(() => rawUsers.value.find(item => item.id === selectedUserID.value) ?? null)
const exceptionTypeCounts = computed(() => rawExceptions.value.reduce((counts: Record<string, number>, item) => { counts[item.type] = (counts[item.type] ?? 0) + 1; return counts }, {}))
async function loadExceptionDetail() {
  if (!selectedExceptionID.value) { exceptionDetail.value = null; return }
  try { exceptionDetail.value = (await apiRequest<Record<string, any>>(`/api/v1/exceptions/${selectedExceptionID.value}`)).data }
  catch (error) { showToast(error instanceof Error ? error.message : 'Exception tidak dapat dibuka.') }
}
async function openBatchException(row: Row) {
  selectedExceptionID.value = row.id || ''
  navigate('exceptionDetail')
  await loadExceptionDetail()
}
function openBatchApproval(row: Row) {
  selectedApprovalID.value = row.id || ''
  decisionComment.value = ''
  modal.value = 'approval'
}
async function assignException() {
  if (!selectedExceptionID.value) return
  try { await apiRequest(`/api/v1/exceptions/${selectedExceptionID.value}/actions`, { method: 'POST', body: JSON.stringify({ type: 'ASSIGN', note: '' }) }); await loadWorkspace(); await loadExceptionDetail(); showToast('Exception ditugaskan ke Anda.') }
  catch (error) { showToast(error instanceof Error ? error.message : 'Aksi gagal.') }
}
async function addExceptionNote() {
  if (!selectedExceptionID.value || !investigationNote.value.trim()) return
  try {
    await apiRequest(`/api/v1/exceptions/${selectedExceptionID.value}/actions`, { method: 'POST', body: JSON.stringify({ type: 'NOTE', note: investigationNote.value.trim() }) })
    investigationNote.value = ''; await loadWorkspace(); await loadExceptionDetail(); showToast('Catatan investigation disimpan.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Catatan gagal disimpan.') }
}
function onEvidenceFile(event: Event) {
  evidenceFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
}
function chooseEvidenceFile() {
  document.querySelector<HTMLInputElement>('#exception-evidence-input')?.click()
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
    showToast('Evidence tersimpan di penyimpanan private.')
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
    await loadWorkspace(); showToast('Batch dikirim ke antrean approval.')
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
    await loadWorkspace(); showToast('Batch berhasil ditutup dan dikunci.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Batch belum dapat ditutup.') }
  finally { busy.value = false }
}
async function transitionBatch(action: 'cancel' | 'reopen' | 'retry') {
  if (!selectedBatchID.value) return
  busy.value = true
  try {
    const { data: batch } = await apiRequest<Record<string, any>>(`/api/v1/batches/${selectedBatchID.value}`)
    await apiRequest(`/api/v1/batches/${selectedBatchID.value}/${action}`, { method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}' })
    await loadWorkspace(); showToast(action === 'cancel' ? 'Batch dibatalkan.' : action === 'reopen' ? 'Batch dibuka untuk koreksi dan import ulang.' : 'Batch siap dicoba kembali.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Perubahan state batch gagal.') }
  finally { busy.value = false }
}
function editBatch() {
  const batch = selectedBatch.value
  if (!batch) return
  batchForm.value = { account_id: batch.account_id, period_from: String(batch.period_from).slice(0, 10), period_to: String(batch.period_to).slice(0, 10), remarks: batch.remarks || '' }
  modal.value = 'editBatch'
}
async function saveSettings() {
  try { await apiRequest('/api/v1/settings', { method: 'PATCH', body: JSON.stringify({ timezone: timezoneSetting.value, max_import_bytes: Number(maxImportSetting.value), csv_delimiter: csvDelimiterSetting.value }) }); showToast('Pengaturan berhasil disimpan.') }
  catch (error) { showToast(error instanceof Error ? error.message : 'Pengaturan gagal disimpan.') }
}
async function toggleUserActive() {
  const user = selectedUser.value
  if (!user) return
  try {
    await apiRequest(`/api/v1/users/${user.id}`, { method: 'PATCH', body: JSON.stringify({ full_name: user.full_name, email: user.email, role: user.role, active: !user.active }) })
    modal.value = ''; await loadWorkspace(); showToast(`${user.full_name} ${user.active ? 'dinonaktifkan' : 'diaktifkan'}.`)
  } catch (error) { showToast(error instanceof Error ? error.message : 'Status user gagal diubah.') }
}
async function saveUserChanges() {
  const user = selectedUser.value
  if (!user) return
  try {
    await apiRequest(`/api/v1/users/${user.id}`, { method: 'PATCH', body: JSON.stringify({ ...userEditForm.value, active: user.active }) })
    modal.value = ''; await loadWorkspace(); showToast('Role dan data pengguna diperbarui.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Perubahan pengguna gagal disimpan.') }
}
async function resetUserPassword() {
  if (!selectedUserID.value || resetPasswordValue.value.length < 12) { showToast('Password minimal 12 karakter.'); return }
  try {
    await apiRequest(`/api/v1/users/${selectedUserID.value}/reset-password`, { method: 'POST', body: JSON.stringify({ password: resetPasswordValue.value }) })
    modal.value = ''; resetPasswordValue.value = ''; showToast('Password direset dan sesi user lama dicabut.')
  } catch (error) { showToast(error instanceof Error ? error.message : 'Password gagal direset.') }
}
async function decideApproval(decision: 'approve' | 'reject') {
  const comment = decisionComment.value.trim()
  if (decision === 'reject' && !comment) { showToast('Komentar wajib diisi untuk penolakan.'); return }
  busy.value = true
  try {
    await apiRequest(`/api/v1/approvals/${selectedApprovalID.value}/${decision}`, { method: 'POST', idempotent: true, body: JSON.stringify({ comment }) })
    modal.value = ''; decisionComment.value = ''; await loadWorkspace(); showToast(decision === 'approve' ? 'Approval disetujui.' : 'Approval ditolak.')
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
    selectedBatchID.value = item.target_entity_id
    navigate('batchDetail')
    await Promise.all([loadMatches(), loadBatchDetails()])
  } else if (item.target_entity === 'exception') {
    selectedExceptionID.value = item.target_entity_id
    navigate('exceptionDetail')
    await loadExceptionDetail()
  } else if (item.target_entity === 'approval') {
    selectedApprovalID.value = item.target_entity_id
    decisionComment.value = ''
    modal.value = 'approval'
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
  <div class="app-shell">
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
      <div class="sidebar-bottom"><div class="user-avatar">{{ currentUser.full_name.slice(0, 2).toUpperCase() }}</div><div class="user-info"><strong>{{ currentUser.full_name }}</strong><small>{{ currentUser.role }}</small></div><button class="more-button" aria-label="Keluar" title="Keluar" @click="logout">↪</button></div>
    </aside>

    <main class="main-area">
      <header class="topbar">
        <div class="crumb"><span>{{ currentNavGroup }}</span><span class="crumb-slash">/</span><strong>{{ currentTitle }}</strong></div>
        <div class="topbar-actions">
          <div class="top-search"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" aria-label="Cari" placeholder="Cari apa saja..." /></div>
          <div class="notification-wrap"><button class="top-icon" aria-label="Notifikasi" @click="showNotifications = !showNotifications"><svg viewBox="0 0 24 24"><path d="M18 9a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></svg><i v-if="notifications.some(item => !item.read_at)"></i></button><div v-if="showNotifications" class="notification-popover"><strong>Notifikasi</strong><button v-for="item in notifications" :key="item.id" class="notification-item" :class="{ unread: !item.read_at }" @click="openNotification(item)"><span class="notice-dot" :class="{ 'amber-dot': item.type.includes('EXCEPTION') }"></span> {{ item.title }} · {{ item.body }}</button><p v-if="!notifications.length">Belum ada notifikasi.</p></div></div>
          <span class="top-divider"></span><button class="profile-button" @click="modal = 'profile'"><span class="user-avatar small-avatar">{{ currentUser.full_name.slice(0, 2).toUpperCase() }}</span><span class="profile-name">{{ currentUser.full_name }}<small>{{ currentUser.role }}</small></span><span class="chevron">⌄</span></button>
        </div>
      </header>

      <div class="content-area">
        <div class="page-header"><div><div class="breadcrumb-mobile">Workspace <span>/</span> {{ currentTitle }}</div><h1>{{ currentTitle }}</h1><p>{{ subtitles[page] }}</p></div><div class="header-actions"><div v-if="page === 'dashboard'" class="dashboard-filter-controls"><input v-model="dashboardFrom" aria-label="Dari tanggal" type="date" /><input v-model="dashboardTo" aria-label="Sampai tanggal" type="date" /><select v-model="dashboardAccountID" aria-label="Rekening"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }}</option></select><button class="button secondary" @click="loadDashboard">Terapkan</button></div><button v-if="pageAction" class="button primary" @click="runAction"><span class="button-plus">＋</span>{{ pageAction }}</button></div></div>

        <section v-if="page === 'dashboard'" class="dashboard-page">
          <div class="kpi-grid">
            <article class="kpi-card"><div class="kpi-top"><span>Total Batch</span><span class="kpi-icon teal">▦</span></div><strong>{{ dashboardSummary.total_batches ?? 0 }}</strong><small class="trend neutral-text">{{ dashboardSummary.active_batches ?? 0 }} masih aktif</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Matched Pairs</span><span class="kpi-icon green">✓</span></div><strong>{{ dashboardSummary.matched_pairs ?? 0 }}</strong><small class="trend positive">{{ Number(dashboardSummary.match_rate ?? 0).toFixed(1) }}% dari transaksi internal</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Exceptions Aktif</span><span class="kpi-icon amber">⚑</span></div><strong>{{ dashboardSummary.open_exceptions ?? 0 }}</strong><small class="trend warning-text">{{ dashboardSummary.exceptions_older_24h ?? 0 }} lebih dari 24 jam</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Approval Menunggu</span><span class="kpi-icon blue">◷</span></div><strong>{{ dashboardSummary.pending_approvals ?? 0 }}</strong><small class="trend neutral-text">Perlu tindakan Checker</small></article>
          </div>
          <div class="dashboard-insights"><article class="panel dashboard-chart-panel"><div class="panel-heading"><div><h2>Tren Rekonsiliasi</h2><p>Transaksi internal per tanggal pada filter terpilih</p></div></div><div v-if="dashboardTrendPoints.length" class="trend-bars"><div v-for="point in dashboardTrendPoints" :key="point.date" class="trend-day"><div class="trend-bar-area"><i class="trend-matched" :style="{ height: point.matchedHeight }" :title="`${point.matched_count} matched`"></i><i class="trend-unmatched" :style="{ height: point.unmatchedHeight }" :title="`${point.unmatched_count} unmatched`"></i></div><small>{{ point.label }}</small></div></div><p v-else class="chart-empty">Belum ada transaksi pada periode ini.</p><div class="chart-legend"><span><i class="trend-matched"></i>Matched internal</span><span><i class="trend-unmatched"></i>Unmatched internal</span></div></article><article class="panel dashboard-distribution"><div class="panel-heading"><div><h2>Status Batch</h2><p>Jumlah batch sesuai filter</p></div></div><div v-for="item in dashboardStatusPoints" :key="item.name" class="distribution-row"><span>{{ item.name }}</span><b>{{ item.count }}</b></div><p v-if="!dashboardStatusPoints.length" class="chart-empty">Belum ada batch.</p></article><article class="panel dashboard-distribution"><div class="panel-heading"><div><h2>Top Selisih</h2><p>Exception yang masih aktif</p></div></div><div v-for="item in dashboardExceptionPoints" :key="item.name" class="distribution-row"><span>{{ item.name }}</span><b>{{ item.count }}</b></div><p v-if="!dashboardExceptionPoints.length" class="chart-empty">Belum ada exception aktif.</p></article></div><article class="panel table-panel recent-panel"><div class="panel-heading"><div><h2>Batch Rekonsiliasi</h2><p>Status dan ringkasan dari database workspace</p></div><button class="link-button" @click="navigate('batches')">Lihat semua batch <span>→</span></button></div><DataTable :columns="tableConfig.batches!.columns" :rows="batchRows.slice(0, 6)" @open="openTableRow" /></article>
          <div class="bottom-grid"><article class="panel process-panel"><div class="panel-heading"><div><h2>Alur Proses</h2><p>Batch berjalan melalui state machine yang tervalidasi di server</p></div></div><div class="process-flow"><div v-for="(step, i) in [{icon:'▱',title:'Buat Batch',text:'Pilih rekening & periode'}, {icon:'⇧',title:'Import Data',text:'Unggah dua CSV'}, {icon:'⇄',title:'Matching',text:'Pencocokan exact'}, {icon:'⌕',title:'Investigasi',text:'Tinjau exception'}, {icon:'✓',title:'Approval',text:'Pemeriksaan Checker'}, {icon:'▥',title:'Tutup Batch',text:'Kunci hasil disetujui'}]" :key="step.title" class="process-step"><span class="process-icon">{{ step.icon }}</span><div><b>{{ i + 1 }}. {{ step.title }}</b><small>{{ step.text }}</small></div><span v-if="i < 5" class="process-arrow">→</span></div></div></article></div>
        </section>

        <section v-else-if="page === 'imports'" class="page-grid import-layout">
          <article class="panel import-card"><div class="panel-heading"><div><h2>Unggah File Transaksi</h2><p>CSV divalidasi seluruhnya sebelum transaksi disimpan.</p></div><span class="step-count">CSV · 10 MB maks.</span></div>
            <div class="upload-grid"><button class="drop-zone" @click="openFilePicker"><span class="upload-cloud">⇧</span><b>{{ selectedFile || 'Pilih file CSV' }}</b><small>Kolom wajib: reference_id, date, direction, amount, currency</small><span class="button primary small-button">Pilih File</span><small class="format-note">Data ditolak seluruhnya bila ada baris tidak valid.</small></button>
              <div class="upload-side"><div class="field-label">Sumber Data</div><select v-model="importSource" class="form-control"><option value="INTERNAL">INTERNAL · Catatan perusahaan</option><option value="BANK">BANK · Mutasi bank</option></select><div class="field-label spaced">Batch Rekonsiliasi</div><select v-model="selectedBatchID" @change="loadImports" class="form-control"><option value="" disabled>Pilih batch draft</option><option v-for="batch in rawBatches.filter(item => item.state === 'DRAFT')" :key="batch.id" :value="batch.id">{{ batch.batch_no }} · {{ formatDate(batch.period_from) }}</option></select><div class="check-note"><span>✓</span><div><b>Import atomik</b><small>File tersimpan secara private dan transaksi aktif hanya dibuat setelah seluruh baris lolos validasi.</small></div></div></div></div>
            <input id="file-input" class="hidden-input" type="file" accept=".csv,text/csv" @change="onFile" /><div class="import-actions"><button class="button secondary" @click="selectedFile = ''; fileToUpload = null">Batal</button><button class="button primary" :disabled="!fileToUpload || busy" @click="uploadImport">{{ busy ? 'Memproses…' : 'Validasi & Import' }} <span>→</span></button></div>
          </article>
          <article class="panel history-card"><div class="panel-heading"><div><h2>Riwayat Import</h2><p>Satu file valid untuk setiap sumber per versi batch.</p></div></div><div v-for="item in rawImports" :key="item.id" class="history-item"><span class="file-icon">CSV</span><span class="file-info"><b>{{ item.original_name }}</b><small>{{ item.source_type }} · {{ item.state }} · {{ item.rows_count }} baris</small></span><button class="link-button" @click="downloadOriginalImport(item)">File</button><button v-if="item.has_error_report" class="link-button" @click="downloadImportErrors(item)">Unduh Error</button></div><p v-if="!rawImports.length" class="login-footnote">Belum ada file pada batch terpilih.</p></article>
        </section>

        <section v-else-if="page === 'batchDetail'" class="detail-page">
          <button class="back-link" @click="navigate('batches')">← Kembali ke daftar batch</button>
          <div class="detail-title"><div><div class="title-line"><h2>{{ selectedBatch?.batch_no || 'Batch tidak dipilih' }}</h2><span class="badge" :class="badgeClass(selectedBatch?.state || '')">{{ selectedBatch?.state || '—' }}</span></div><p>{{ selectedBatch?.remarks || selectedBatch?.bank_display_name || 'Tanpa catatan' }} <span>·</span> {{ formatDate(selectedBatch?.period_from) }} – {{ formatDate(selectedBatch?.period_to) }} · {{ selectedBatch?.account_code }}</p></div><button class="button secondary" @click="navigate('matching')">Hasil Matching</button><button v-if="selectedBatch?.state === 'DRAFT' && currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="busy" @click="editBatch">Edit Batch</button><button v-if="selectedBatch?.state === 'DRAFT' && currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="busy" @click="transitionBatch('cancel')">Batalkan</button><button v-if="selectedBatch?.state === 'REVIEW' && currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="busy" @click="transitionBatch('reopen')">Koreksi Sumber</button><button v-if="selectedBatch?.state === 'REVIEW' && currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="busy" @click="submitBatch">Ajukan Approval</button><button v-if="selectedBatch?.state === 'FAILED' && currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="busy" @click="transitionBatch('retry')">Coba Matching Lagi</button><button v-if="selectedBatch?.state === 'APPROVED' && currentUser?.role === 'CHECKER'" class="button primary" :disabled="busy" @click="closeBatch">Tutup Batch</button></div>
          <div class="kpi-grid detail-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>{{ selectedBatch?.internal_count ?? 0 }}</strong></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>{{ selectedBatch?.bank_count ?? 0 }}</strong></article><article class="kpi-card"><div class="kpi-top">Matched</div><strong>{{ selectedBatch?.matched_pairs ?? 0 }}</strong></article><article class="kpi-card"><div class="kpi-top">Exceptions Aktif</div><strong>{{ selectedBatch?.active_exceptions ?? 0 }}</strong></article></div>
          <article class="panel detail-tabs"><div class="tabs"><button v-for="tab in batchTabs" :key="tab" :class="{ selected: activeTab === tab }" @click="activeTab = tab">{{ tab }}</button></div><div v-if="activeTab === 'Ringkasan'" class="detail-summary"><div><h3>Ringkasan</h3><p>Snapshot sumber: versi {{ selectedBatch?.source_version ?? '—' }} · optimistic row version {{ selectedBatch?.row_version ?? '—' }}</p><div class="match-progress"><span>Matched</span><div><i :style="{ width: `${selectedBatch?.internal_count ? selectedBatch.matched_pairs * 100 / selectedBatch.internal_count : 0}%` }"></i></div><b>{{ selectedBatch?.matched_pairs ?? 0 }} pasangan</b></div><div class="match-progress unmatched-progress"><span>Exceptions</span><div><i :style="{ width: `${selectedBatch?.internal_count ? selectedBatch.active_exceptions * 100 / selectedBatch.internal_count : 0}%` }"></i></div><b>{{ selectedBatch?.active_exceptions ?? 0 }} aktif</b></div></div><div class="batch-state"><span class="state-check">!</span><b>{{ selectedBatch?.state === 'CLOSED' ? 'Batch ditutup' : selectedBatch?.active_exceptions ? 'Belum dapat ditutup' : 'Periksa state batch' }}</b><small>Batch hanya bisa diajukan saat seluruh data cocok dan tidak ada exception atau proposal approval yang tertunda.</small></div></div><div v-else-if="activeTab === 'Imports'"><div v-for="item in rawImports" :key="item.id" class="history-item"><span class="file-icon">CSV</span><span class="file-info"><b>{{ item.original_name }}</b><small>{{ item.source_type }} · {{ item.state }} · {{ item.rows_count }} baris · {{ formatDate(item.created_at) }}</small></span><button class="link-button" @click="downloadOriginalImport(item)">File</button><button v-if="item.has_error_report" class="link-button" @click="downloadImportErrors(item)">Unduh Error</button></div><p v-if="!rawImports.length" class="login-footnote">Belum ada file import untuk batch ini.</p></div><div v-else-if="activeTab === 'Detail Transaksi'"><DataTable :columns="[{ key: 'ref', label: 'Reference' }, { key: 'date', label: 'Tanggal' }, { key: 'source', label: 'Sumber' }, { key: 'description', label: 'Arah' }, { key: 'debit', label: 'Debit' }, { key: 'credit', label: 'Kredit' }, { key: 'status', label: 'Status' }]" :rows="batchTransactionRows" @open="openTransaction" /></div><div v-else-if="activeTab === 'Matching Results'"><DataTable :columns="tableConfig.matching!.columns" :rows="matchingRows" /></div><div v-else-if="activeTab === 'Exceptions'"><DataTable :columns="tableConfig.exceptions!.columns" :rows="batchExceptionRows" link-key="ref" @open="openBatchException" /></div><div v-else-if="activeTab === 'Approvals'"><DataTable :columns="tableConfig.approvals!.columns" :rows="batchApprovalRows" @open="openBatchApproval" /></div><div v-else><DataTable :columns="tableConfig.audit!.columns" :rows="batchAuditRows" /></div></article>
        </section>

        <section v-else-if="page === 'exceptionDetail'" class="detail-page"><button class="back-link" @click="navigate('exceptions')">← Kembali ke Exceptions</button>
          <div class="detail-title"><div><div class="title-line"><h2>{{ exceptionDetail?.reference_id || 'Exception' }}</h2><span class="badge warning">{{ exceptionDetail?.type || '—' }}</span></div><p>{{ exceptionDetail?.batch_no }} <span>·</span> {{ exceptionDetail?.state }}</p></div><button v-if="currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="!selectedExceptionID" @click="assignException">Assign ke saya</button><button v-if="exceptionDetail?.type === 'DATE_VARIANCE' && currentUser?.role === 'OPERATOR'" class="button primary" @click="modal = 'proposal'">Ajukan Manual Match</button></div>
          <div class="comparison-grid"><article class="panel"><div class="panel-heading"><div><h2>Data Internal</h2><p>{{ exceptionDetail?.internal?.source_type || 'INTERNAL' }}</p></div></div><div class="data-pair"><span>Reference ID</span><b>{{ exceptionDetail?.internal?.reference_id || '—' }}</b></div><div class="data-pair"><span>Tanggal</span><b>{{ formatDate(exceptionDetail?.internal?.date) }}</b></div><div class="data-pair"><span>Arah</span><b>{{ exceptionDetail?.internal?.direction || '—' }}</b></div><div class="data-pair"><span>Nominal</span><b>{{ money(exceptionDetail?.internal?.amount_rupiah) }}</b></div></article><article class="panel"><div class="panel-heading"><div><h2>Data Bank</h2><p>{{ exceptionDetail?.bank?.source_type || 'BANK' }}</p></div></div><div class="data-pair"><span>Reference ID</span><b>{{ exceptionDetail?.bank?.reference_id || '—' }}</b></div><div class="data-pair"><span>Tanggal</span><b>{{ formatDate(exceptionDetail?.bank?.date) }}</b></div><div class="data-pair"><span>Arah</span><b>{{ exceptionDetail?.bank?.direction || '—' }}</b></div><div class="data-pair mismatch"><span>Nominal</span><b>{{ money(exceptionDetail?.bank?.amount_rupiah) }}</b></div></article></div>
          <article class="panel timeline-panel"><div class="panel-heading"><div><h2>Riwayat Pemeriksaan</h2><p>Catatan dan assignment exception</p></div></div><div class="timeline"><div v-for="action in exceptionDetail?.actions || []" :key="action.id"><i></i><span><b>{{ action.action_type }} · {{ action.actor_name }}</b><small>{{ action.note || 'Tidak ada catatan' }} · {{ formatDate(action.created_at) }}</small><button v-if="action.has_evidence" class="link-button" @click="downloadExceptionEvidence(action)">Unduh {{ action.file_name || 'evidence' }}</button></span></div><p v-if="!exceptionDetail?.actions?.length">Belum ada tindakan.</p></div><label v-if="currentUser?.role === 'OPERATOR'" class="field-label">Catatan investigasi<textarea v-model="investigationNote" class="form-control" rows="2" placeholder="Catatan untuk audit trail"></textarea></label><button v-if="currentUser?.role === 'OPERATOR'" class="button secondary" :disabled="!investigationNote.trim()" @click="addExceptionNote">Simpan Catatan</button><div v-if="currentUser?.role === 'OPERATOR'" class="evidence-upload"><input id="exception-evidence-input" class="hidden-input" type="file" accept=".pdf,.png,.jpg,.jpeg,.txt,application/pdf,image/png,image/jpeg,text/plain" @change="onEvidenceFile" /><span>{{ evidenceFile?.name || 'PDF, PNG, JPEG, TXT · maks. 5 MB' }}</span><button class="button secondary" @click="chooseEvidenceFile">Pilih Evidence</button><button class="button primary" :disabled="!evidenceFile" @click="uploadExceptionEvidence">Unggah</button></div></article>
        </section>

        <section v-else-if="page === 'reports'" class="reports-page">
          <article class="panel report-filters"><div class="filter-block"><label>Mulai</label><input v-model="reportFrom" class="form-control" type="date" /></div><div class="filter-block"><label>Sampai</label><input v-model="reportTo" class="form-control" type="date" /></div><div class="filter-block"><label>Rekening</label><select v-model="reportAccountID" class="form-control"><option value="">Semua rekening</option><option v-for="account in rawAccounts" :key="account.id" :value="account.id">{{ account.account_code }} · {{ account.bank_display_name }}</option></select></div><button class="button primary" @click="downloadReport">⇩ Unduh CSV</button></article>
          <div class="kpi-grid report-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>{{ dashboardSummary.internal_count ?? 0 }}</strong></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>{{ dashboardSummary.bank_count ?? 0 }}</strong></article><article class="kpi-card"><div class="kpi-top">Matched Pairs</div><strong>{{ dashboardSummary.matched_pairs ?? 0 }}</strong></article><article class="kpi-card"><div class="kpi-top">Exceptions Aktif</div><strong>{{ dashboardSummary.open_exceptions ?? 0 }}</strong></article></div>
          <article class="panel table-panel"><div class="panel-heading"><div><h2>Snapshot batch</h2><p>Unduhan CSV berisi batch, transaksi, dan exception yang dapat Anda akses.</p></div></div><DataTable :columns="tableConfig.batches!.columns" :rows="batchRows" @open="openTableRow" /></article>
        </section>

        <section v-else-if="page === 'settings'" class="settings-page"><div class="settings-nav panel"><button v-for="s in ['Umum','Parameter Matching','Integrasi','Notifikasi','Manajemen Role']" :key="s" :class="{ selected: activeTab === s || (activeTab === 'All' && s === 'Umum') }" @click="activeTab = s">{{ s }} <span>›</span></button></div><article class="panel settings-card"><div class="panel-heading"><div><h2>{{ activeTab === 'All' ? 'Pengaturan Sistem' : activeTab }}</h2><p>Konfigurasi umum, parameter rekonsiliasi, dan integrasi.</p></div></div><section class="settings-section"><h3>Umum</h3><div class="settings-row"><div><b>Zona Waktu Default</b><small>Digunakan untuk tanggal batch dan audit log.</small></div><select v-model="timezoneSetting" :disabled="currentUser?.role !== 'ADMIN'" class="form-control compact"><option value="Asia/Jakarta">Asia/Jakarta (WIB, UTC+7)</option></select></div><div class="settings-row"><div><b>Maksimum Ukuran Import</b><small>Batas ukuran file transaksi yang diterima.</small></div><select v-model.number="maxImportSetting" :disabled="currentUser?.role !== 'ADMIN'" class="form-control compact"><option :value="5242880">5 MB</option><option :value="10485760">10 MB</option></select></div><div class="settings-row"><div><b>Format CSV</b><small>Delimiter yang digunakan pada file transaksi.</small></div><select v-model="csvDelimiterSetting" :disabled="currentUser?.role !== 'ADMIN'" class="form-control compact"><option value=",">Koma (,)</option></select></div></section><section class="settings-section"><h3>Parameter Matching</h3><div class="matching-rule"><span class="rule-code">M01</span><div><b>Auto Exact Match</b><small>Reference, tanggal, arah, mata uang, dan nominal harus sama.</small></div><span class="readonly-chip">Aktif · Read-only</span></div><div class="settings-row"><div><b>Toleransi Selisih Tanggal</b><small>Hanya dapat diajukan untuk manual match dan perlu checker approval.</small></div><select class="form-control compact"><option>3 hari kalender</option></select></div></section><button v-if="currentUser?.role === 'ADMIN'" class="button primary" @click="saveSettings">Simpan Perubahan</button></article></section>

        <section v-else class="list-page"><div v-if="page === 'matching'" class="kpi-grid matching-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>{{ selectedBatch?.internal_count ?? 0 }}</strong><small class="trend neutral-text">Batch terpilih</small></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>{{ selectedBatch?.bank_count ?? 0 }}</strong><small class="trend neutral-text">Batch terpilih</small></article><article class="kpi-card"><div class="kpi-top">Matched Pairs</div><strong>{{ matchingRows.length }}</strong><small class="trend positive">Rule M01</small></article><article class="kpi-card"><div class="kpi-top">Exceptions</div><strong>{{ selectedBatch?.active_exceptions ?? 0 }}</strong><small class="trend warning-text">{{ selectedBatch?.batch_no ?? 'Pilih batch' }}</small></article></div><div v-if="page === 'exceptions'" class="kpi-grid exception-kpis"><article v-for="kind in ['AMOUNT_MISMATCH','DIRECTION_MISMATCH','DATE_VARIANCE','INTERNAL_ONLY','BANK_ONLY']" :key="kind" class="kpi-card"><div class="kpi-top">{{ kind }}</div><strong>{{ exceptionTypeCounts[kind] ?? 0 }}</strong><small class="trend neutral-text">Exception aktif di workspace</small></article></div><article class="panel table-panel"><div class="table-toolbar"><div class="tabs"><button v-for="tab in currentTableTabs" :key="tab" :class="{ selected: activeTab === tab || (activeTab === 'All' && tab.startsWith('Semua')) || (activeTab === 'Menunggu' && tab.startsWith('Menunggu')) }" @click="activeTab = tab">{{ tab }}</button></div><div class="filters"><label class="search-field"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" :placeholder="page === 'exceptions' ? 'Cari reference / batch...' : 'Cari batch / reference...'" /></label><select v-if="['batches','exceptions','transactions'].includes(page)" v-model="sourceFilter" class="filter-select"><option>All sources</option><option>INTERNAL</option><option>BANK</option></select><select v-model="statusFilter" class="filter-select"><option>All statuses</option><option v-for="option in currentStatusOptions" :key="option">{{ option }}</option></select><button class="filter-icon" aria-label="Filter">☷</button></div></div><DataTable v-if="currentTable" :columns="currentTable.columns" :rows="paginatedRows" :link-key="page === 'batches' ? 'batch' : page === 'exceptions' ? 'ref' : ''" @open="openTableRow" /><div class="table-footer"><span>Menampilkan {{ firstVisibleRow }}–{{ lastVisibleRow }} dari {{ filteredRows.length }} baris ({{ currentTable?.rows.length }} dimuat)</span><div><button :disabled="currentPage <= 1" @click="currentPage--">‹</button><button class="current-page">{{ currentPage }} / {{ pageCount }}</button><button :disabled="currentPage >= pageCount" @click="currentPage++">›</button></div></div></article></section>

        <section v-if="page === 'users'" class="panel role-matrix-panel"><div class="panel-heading"><div><h2>Role &amp; Access Control</h2><p>Role tetap MVP · matrix baca-saja · bukan role builder dinamis.</p></div><span class="badge neutral">RBAC</span></div><div class="role-cards"><article v-for="role in roleCards" :key="role.name" class="role-card"><span :class="['role-symbol', role.tone]">{{ role.name.slice(0, 1) }}</span><div><b>{{ role.name }}</b><small>{{ role.description }}</small></div></article></div><div class="role-table-scroll"><table class="role-table"><thead><tr><th>Fitur</th><th>Admin</th><th>Operator</th><th>Checker</th><th>Auditor</th></tr></thead><tbody><tr v-for="row in roleMatrixRows" :key="row.feature"><td>{{ row.feature }}</td><td>{{ row.admin }}</td><td>{{ row.operator }}</td><td>{{ row.checker }}</td><td>{{ row.auditor }}</td></tr></tbody></table></div><div class="sod-note"><b>Segregation of duties</b><span>Checker tidak boleh menyetujui permintaan sendiri. Admin tidak mendapat bypass approval. Akun nonaktif kehilangan akses sesi.</span></div></section>

        <footer class="app-footer">ReconX <span>·</span> Bank Transaction Reconciliation &amp; Exception Management <span class="footer-version">v0.2 · Modular monolith</span></footer>
      </div>
    </main>
    </template>

    <Transition name="modal"><div v-if="modal" class="modal-backdrop" @click.self="modal = ''"><section class="modal-card"><div class="modal-heading"><div><h2>{{ modal === 'batch' ? 'Buat Batch Rekonsiliasi' : modal === 'editBatch' ? 'Edit Batch'  : modal === 'account' ? 'Tambah Sumber Data' : modal === 'accountEdit' ? 'Edit Rekening' : modal === 'proposal' ? 'Ajukan Manual Match' : modal === 'approval' ? 'Keputusan Approval' : modal === 'profile' ? 'Profil Saya' : modal === 'transaction' ? 'Detail Transaksi' : modal === 'userActions' ? 'Akses Pengguna' : 'Tambah Pengguna' }}</h2><p>{{ modal === 'batch' || modal === 'editBatch' ? 'Tentukan rekening dan periode rekonsiliasi.' : modal === 'accountEdit' ? 'Perbarui rekening sumber. Rekening dengan batch aktif tidak dapat dinonaktifkan.' : modal === 'proposal' ? 'Permintaan ini akan ditinjau oleh Checker.' : modal === 'approval' ? approvalDetail?.reason : modal === 'profile' ? 'Informasi akun yang sedang digunakan.' : modal === 'transaction' ? 'Data sumber dan status rekonsiliasi (read-only).' : 'Lengkapi informasi untuk melanjutkan.' }}</p></div><button class="close-button" @click="modal = ''">×</button></div>
      <template v-if="modal === 'transaction'"><div class="data-pair"><span>Reference</span><b>{{ selectedTransaction?.reference_id }}</b></div><div class="data-pair"><span>Tanggal · Arah</span><b>{{ formatDate(selectedTransaction?.date) }} · {{ selectedTransaction?.direction }}</b></div><div class="data-pair"><span>Nominal · Mata uang</span><b>{{ money(selectedTransaction?.amount_rupiah) }} · {{ selectedTransaction?.currency }}</b></div><div class="data-pair"><span>Sumber · Status</span><b>{{ selectedTransaction?.source_type }} · {{ selectedTransaction?.status }}</b></div><div class="data-pair"><span>Batch · Source version</span><b>{{ selectedTransaction?.batch_id }} · {{ selectedTransaction?.source_version }}</b></div><label class="field-label">Raw row (read-only)<pre class="raw-row-preview">{{ formatJSON(selectedTransaction?.raw_row) }}</pre></label></template><template v-else-if="modal === 'profile'"><label class="field-label">Nama lengkap<input class="form-control" :value="currentUser?.full_name" readonly /></label><label class="field-label">Email<input class="form-control" :value="currentUser?.email" readonly /></label><label class="field-label">Role<input class="form-control" :value="currentUser?.role" readonly /></label><div class="modal-actions"><button class="button secondary" @click="modal = ''">Tutup</button><button class="button primary" @click="logout">Keluar dari akun</button></div></template><template v-else-if="modal === 'batch' || modal === 'editBatch'"><label class="field-label">Rekening Sumber<select v-model="batchForm.account_id" class="form-control" required><option value="" disabled>Pilih rekening</option><option v-for="account in rawAccounts.filter(item => item.active)" :key="account.id" :value="account.id">{{ account.account_code }} · {{ account.bank_display_name }}</option></select></label><label class="field-label">Periode Batch<div class="date-pair"><input v-model="batchForm.period_from" class="form-control" type="date" /><input v-model="batchForm.period_to" class="form-control" type="date" /></div></label><label class="field-label">Catatan<input v-model="batchForm.remarks" class="form-control" placeholder="Opsional" /></label></template>
      <template v-else-if="modal === 'account' || modal === 'accountEdit'"><label class="field-label">Kode sumber<input v-model="accountForm.account_code" class="form-control" placeholder="QRIS" /></label><label class="field-label">Nama sumber<input v-model="accountForm.bank_display_name" class="form-control" placeholder="QRIS Utama" /></label><label class="field-label">Nomor rekening tersamarkan<input v-model="accountForm.masked_account_no" class="form-control" /></label><label class="field-label">Pemilik rekening<input v-model="accountForm.account_holder_name" class="form-control" /></label><label v-if="modal === 'accountEdit'" class="field-label">Status<select v-model="accountActiveForm" class="form-control"><option :value="true">Aktif</option><option :value="false">Nonaktif</option></select></label></template>
      <template v-else-if="modal === 'user'"><label class="field-label">Nama Lengkap<input v-model="userForm.full_name" class="form-control" placeholder="Nama pengguna" /></label><label class="field-label">Email<input v-model="userForm.email" class="form-control" type="email" placeholder="nama@perusahaan.co.id" /></label><label class="field-label">Password awal<input v-model="userForm.password" class="form-control" type="password" minlength="12" /></label><label class="field-label">Role<select v-model="userForm.role" class="form-control"><option value="OPERATOR">Operator</option><option value="CHECKER">Checker</option><option value="ADMIN">Admin</option><option value="AUDITOR">Auditor</option></select></label></template>
      <label v-else-if="modal === 'proposal'" class="field-label">Alasan Pengajuan<textarea v-model="proposalReason" class="form-control" rows="4" placeholder="Minimum 20 karakter; hanya DATE_VARIANCE yang dapat diusulkan."></textarea></label>
      <template v-else-if="modal === 'approval'"><p class="approval-summary">{{ approvalDetail?.kind }} · {{ approvalDetail?.batch_no }} · Maker: {{ approvalDetail?.maker_name }}</p><div v-if="approvalDetail?.kind === 'MANUAL_MATCH' && approvalDetail.internal && approvalDetail.bank" class="comparison-grid approval-comparison"><article class="panel"><h3>Internal</h3><p>{{ approvalDetail.internal.reference_id }} · {{ formatDate(approvalDetail.internal.date) }}</p><p>{{ approvalDetail.internal.direction }} · {{ money(approvalDetail.internal.amount_rupiah) }}</p></article><article class="panel"><h3>Bank</h3><p>{{ approvalDetail.bank.reference_id }} · {{ formatDate(approvalDetail.bank.date) }}</p><p>{{ approvalDetail.bank.direction }} · {{ money(approvalDetail.bank.amount_rupiah) }}</p></article></div><p v-if="approvalDetail?.reason" class="approval-summary">Alasan maker: {{ approvalDetail.reason }}</p><label class="field-label">Komentar keputusan<textarea v-model="decisionComment" class="form-control" rows="3" placeholder="Wajib diisi jika menolak"></textarea></label><div class="modal-actions"><template v-if="currentUser?.role === 'CHECKER'"><button class="button secondary" :disabled="busy" @click="decideApproval('reject')">Tolak</button><button class="button primary" :disabled="busy" @click="decideApproval('approve')">Setujui</button></template><span v-else class="login-footnote">Mode auditor: hanya baca.</span></div></template>
      <template v-else-if="modal === 'userActions'"><label class="field-label">Nama<input v-model="userEditForm.full_name" class="form-control" /></label><label class="field-label">Email<input v-model="userEditForm.email" class="form-control" type="email" /></label><label class="field-label">Role<select v-model="userEditForm.role" class="form-control"><option value="OPERATOR">Operator</option><option value="CHECKER">Checker</option><option value="ADMIN">Admin</option><option value="AUDITOR">Auditor</option></select></label><div class="modal-actions"><button class="button secondary" :disabled="busy" @click="toggleUserActive">{{ selectedUser?.active ? 'Nonaktifkan Akun' : 'Aktifkan Akun' }}</button><button class="button secondary" :disabled="busy" @click="saveUserChanges">Simpan Role</button></div><label class="field-label">Password baru<input v-model="resetPasswordValue" class="form-control" type="password" minlength="12" placeholder="Minimum 12 karakter" /></label><div class="modal-actions"><button class="button primary" :disabled="busy" @click="resetUserPassword">Reset Password</button></div></template>
      <div v-if="modal !== 'approval' && modal !== 'userActions' && modal !== 'profile' && modal !== 'transaction'" class="modal-actions"><button class="button secondary" @click="modal = ''">Batal</button><button class="button primary" :disabled="busy" @click="submitModal">{{ busy ? 'Menyimpan…' : modal === 'proposal' ? 'Kirim untuk Approval' : modal === 'editBatch' ? 'Simpan Perubahan' : 'Simpan' }}</button></div>
    </section></div></Transition>
    <div v-if="toast" class="toast-message"><span>✓</span>{{ toast }}</div>
  </div>
</template>
