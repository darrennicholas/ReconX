<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import DataTable from './components/DataTable.vue'

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

const navGroups: { title: string; items: { id: Page; label: string; icon: string; count?: string }[] }[] = [
  { title: 'OVERVIEW', items: [{ id: 'dashboard', label: 'Dashboard', icon: 'grid' }] },
  { title: 'OPERATIONS', items: [
    { id: 'batches', label: 'Reconciliation Batches', icon: 'layers', count: '2' },
    { id: 'imports', label: 'Import Data', icon: 'upload' },
    { id: 'transactions', label: 'Transactions', icon: 'rows' },
    { id: 'matching', label: 'Matching', icon: 'shuffle' },
    { id: 'exceptions', label: 'Exceptions', icon: 'alert', count: '12' },
  ] },
  { title: 'GOVERNANCE', items: [{ id: 'approvals', label: 'Approval Queue', icon: 'check', count: '3' }] },
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

const batchRows: Row[] = [
  { batch: 'RB-2025-00048', name: 'Rekon BCA Jan-2', source: 'Core Banking', period: '30 Jan 2025', total: '12,480', rate: '98.72%', status: 'Completed', updated: '30 Jan 2025, 15:42' },
  { batch: 'RB-2025-00047', name: 'Rekon VA Jan-2', source: 'Virtual Account', period: '30 Jan 2025', total: '8,721', rate: '99.14%', status: 'Completed', updated: '30 Jan 2025, 14:18' },
  { batch: 'RB-2025-00046', name: 'Rekon QRIS Jan-2', source: 'QRIS', period: '30 Jan 2025', total: '5,230', rate: '96.08%', status: 'In Progress', updated: '30 Jan 2025, 13:06' },
  { batch: 'RB-2025-00045', name: 'Rekon BCA Jan-1', source: 'Core Banking', period: '29 Jan 2025', total: '12,510', rate: '99.02%', status: 'Completed', updated: '29 Jan 2025, 17:25' },
  { batch: 'RB-2025-00044', name: 'Rekon EDC Jan-1', source: 'EDC', period: '29 Jan 2025', total: '7,090', rate: '97.44%', status: 'Completed', updated: '29 Jan 2025, 16:02' },
]
const transactionRows: Row[] = [
  { ref: 'QRIS000008323', date: '30 Jan 2025 14:20', source: 'QRIS', description: 'Pembayaran invoice 008323', debit: '—', credit: 'Rp 1.250.000', status: 'Matched', batch: 'RB-2025-00048' },
  { ref: 'QRIS000008337', date: '30 Jan 2025 14:18', source: 'QRIS', description: 'Pembayaran invoice 008337', debit: '—', credit: 'Rp 750.000', status: 'Matched', batch: 'RB-2025-00048' },
  { ref: 'VA0000018402', date: '30 Jan 2025 13:51', source: 'Virtual Account', description: 'Top up merchant 18402', debit: 'Rp 85.000', credit: '—', status: 'Unmatched', batch: 'RB-2025-00047' },
  { ref: 'EDC000004821', date: '30 Jan 2025 12:09', source: 'EDC', description: 'Settlement terminal 04', debit: '—', credit: 'Rp 3.450.000', status: 'Matched', batch: 'RB-2025-00044' },
]
const exceptionRows: Row[] = [
  { ref: 'QRIS000008323', type: 'Amount mismatch', batch: 'RB-2025-00048', nominalA: 'Rp 75.000', nominalB: 'Rp 70.000', difference: 'Rp 5.000', owner: 'Sari Wulandari', status: 'Open' },
  { ref: 'QRIS000008337', type: 'Date variance', batch: 'RB-2025-00047', nominalA: 'Rp 100.000', nominalB: 'Rp 100.000', difference: '1 day', owner: 'Budi Ananta', status: 'In Review' },
  { ref: 'QRIS000008341', type: 'Internal only', batch: 'RB-2025-00046', nominalA: 'Rp 50.000', nominalB: '—', difference: 'Rp 50.000', owner: 'Unassigned', status: 'Open' },
  { ref: 'QRIS000008355', type: 'Date variance', batch: 'RB-2025-00045', nominalA: 'Rp 200.000', nominalB: 'Rp 200.000', difference: '2 days', owner: 'Rizky Pratama', status: 'In Review' },
]
const approvalRows: Row[] = [
  { request: 'APPR-2025-0012', type: 'Adjustment', batch: 'RB-2025-00048', amount: 'Rp 5.000.000', maker: 'Budi Ananta', submitted: '30 Jan 2025 14:30', status: 'Menunggu' },
  { request: 'APPR-2025-0011', type: 'Close Batch', batch: 'RB-2025-00046', amount: '—', maker: 'Sari Wulandari', submitted: '30 Jan 2025 13:12', status: 'Menunggu' },
  { request: 'APPR-2025-0010', type: 'Adjustment', batch: 'RB-2025-00045', amount: 'Rp 2.500.000', maker: 'Rizky Pratama', submitted: '29 Jan 2025 17:20', status: 'Disetujui' },
  { request: 'APPR-2025-0009', type: 'Close Batch', batch: 'RB-2025-00044', amount: '—', maker: 'Budi Ananta', submitted: '29 Jan 2025 16:35', status: 'Ditolak' },
]
const accountRows: Row[] = [
  { code: 'CB', name: 'Core Banking', type: 'System', account: '•••• 2048', currency: 'IDR', status: 'Aktif' },
  { code: 'QRIS', name: 'QRIS', type: 'Payment', account: 'Merchant •••• 8321', currency: 'IDR', status: 'Aktif' },
  { code: 'EDC', name: 'EDC', type: 'Payment', account: 'Terminal •••• 2410', currency: 'IDR', status: 'Aktif' },
  { code: 'VA', name: 'Virtual Account', type: 'System', account: 'VA •••• 5420', currency: 'IDR', status: 'Aktif' },
  { code: 'ECH', name: 'E-Channel (Mobile/Web)', type: 'Channel', account: 'Digital •••• 0198', currency: 'IDR', status: 'Aktif' },
]
const userRows: Row[] = [
  { name: 'Budi Ananta', email: 'budi@reconx.co.id', role: 'Admin', lastActive: 'Baru saja', status: 'Aktif' },
  { name: 'Sari Wulandari', email: 'sari@reconx.co.id', role: 'Operator', lastActive: '5 menit lalu', status: 'Aktif' },
  { name: 'Rizky Pratama', email: 'rizky@reconx.co.id', role: 'Checker', lastActive: '25 menit lalu', status: 'Aktif' },
  { name: 'Dewi Lestari', email: 'dewi@reconx.co.id', role: 'Approver', lastActive: '1 jam lalu', status: 'Aktif' },
  { name: 'Acif Santoso', email: 'acif@reconx.co.id', role: 'Auditor', lastActive: '2 jam lalu', status: 'Aktif' },
]
const auditRows: Row[] = [
  { time: '30 Jan 2025 15:42', actor: 'Sari Wulandari', activity: 'Import file', module: 'Import', detail: 'qris_20250130.csv', status: 'Success' },
  { time: '30 Jan 2025 15:18', actor: 'Budi Ananta', activity: 'Batch created', module: 'Reconciliation', detail: 'RB-2025-00048', status: 'Success' },
  { time: '30 Jan 2025 15:06', actor: 'Rizky Pratama', activity: 'Exception assigned', module: 'Exceptions', detail: 'QRIS000008323', status: 'Success' },
  { time: '30 Jan 2025 14:50', actor: 'Budi Ananta', activity: 'Approval requested', module: 'Approval', detail: 'APPR-2025-0012', status: 'Success' },
]

const tableConfig: Partial<Record<Page, { columns: { key: string; label: string }[]; rows: Row[] }>> = {
  batches: { columns: [{ key: 'batch', label: 'No. Batch' }, { key: 'name', label: 'Nama Batch' }, { key: 'source', label: 'Sumber' }, { key: 'period', label: 'Periode' }, { key: 'total', label: 'Total Tx' }, { key: 'rate', label: 'Match Rate' }, { key: 'status', label: 'Status' }, { key: 'updated', label: 'Diperbarui' }], rows: batchRows },
  transactions: { columns: [{ key: 'ref', label: 'Ref. No.' }, { key: 'date', label: 'Tanggal' }, { key: 'source', label: 'Sumber' }, { key: 'description', label: 'Deskripsi' }, { key: 'debit', label: 'Debit' }, { key: 'credit', label: 'Kredit' }, { key: 'status', label: 'Status' }, { key: 'batch', label: 'Batch' }], rows: transactionRows },
  matching: { columns: [{ key: 'ref', label: 'Ref. No.' }, { key: 'date', label: 'Tanggal' }, { key: 'internal', label: 'Data Internal' }, { key: 'bank', label: 'Data Bank' }, { key: 'amount', label: 'Nominal' }, { key: 'rule', label: 'Rule' }, { key: 'status', label: 'Status' }], rows: [
    { ref: 'QRIS000008323', date: '30 Jan 2025 14:20', internal: '30 Jan · Credit', bank: '30 Jan · Credit', amount: 'Rp 75.000', rule: 'M01 Exact', status: 'Matched' },
    { ref: 'QRIS000008337', date: '30 Jan 2025 14:18', internal: '30 Jan · Credit', bank: '31 Jan · Credit', amount: 'Rp 100.000', rule: 'Date variance', status: 'Unmatched' },
    { ref: 'QRIS000008341', date: '30 Jan 2025 14:03', internal: '30 Jan · Debit', bank: '—', amount: 'Rp 50.000', rule: 'Internal only', status: 'Unmatched' },
    { ref: 'QRIS000008355', date: '30 Jan 2025 13:58', internal: '30 Jan · Credit', bank: '30 Jan · Credit', amount: 'Rp 200.000', rule: 'M01 Exact', status: 'Matched' },
  ] },
  exceptions: { columns: [{ key: 'ref', label: 'No. Ref.' }, { key: 'type', label: 'Tipe Selisih' }, { key: 'batch', label: 'Batch' }, { key: 'nominalA', label: 'Nominal A' }, { key: 'nominalB', label: 'Nominal B' }, { key: 'difference', label: 'Selisih' }, { key: 'owner', label: 'PIC' }, { key: 'status', label: 'Status' }], rows: exceptionRows },
  approvals: { columns: [{ key: 'request', label: 'No. Pengajuan' }, { key: 'type', label: 'Jenis' }, { key: 'batch', label: 'Batch' }, { key: 'amount', label: 'Nominal' }, { key: 'maker', label: 'Maker' }, { key: 'submitted', label: 'Diajukan' }, { key: 'status', label: 'Status' }], rows: approvalRows },
  accounts: { columns: [{ key: 'code', label: 'Kode' }, { key: 'name', label: 'Nama Sumber' }, { key: 'type', label: 'Tipe' }, { key: 'account', label: 'Rekening' }, { key: 'currency', label: 'Mata Uang' }, { key: 'status', label: 'Status' }], rows: accountRows },
  users: { columns: [{ key: 'name', label: 'Nama' }, { key: 'email', label: 'Email' }, { key: 'role', label: 'Role' }, { key: 'lastActive', label: 'Aktif Terakhir' }, { key: 'status', label: 'Status' }], rows: userRows },
  audit: { columns: [{ key: 'time', label: 'Waktu' }, { key: 'actor', label: 'Pengguna' }, { key: 'activity', label: 'Aktivitas' }, { key: 'module', label: 'Modul' }, { key: 'detail', label: 'Detail' }, { key: 'status', label: 'Hasil' }], rows: auditRows },
}

const currentTitle = computed(() => titles[page.value])
const currentNavGroup = computed(() => navGroups.find(group => group.items.some(item => item.id === page.value))?.title ?? 'OPERATIONS')
const currentTable = computed(() => tableConfig[page.value])
const filteredRows = computed(() => {
  const rows = currentTable.value?.rows ?? []
  const q = search.value.trim().toLowerCase()
  return rows.filter(row => {
    const matchesSearch = !q || Object.values(row).some(value => value.toLowerCase().includes(q))
    const matchesStatus = statusFilter.value === 'All statuses' || row.status === statusFilter.value
    const matchesSource = sourceFilter.value === 'All sources' || row.source === sourceFilter.value
    return matchesSearch && matchesStatus && matchesSource
  })
})
const pageAction = computed(() => ({ batches: 'Buat Batch', imports: 'Unggah File', accounts: 'Tambah Sumber', users: 'Tambah Pengguna', reports: 'Unduh Laporan', exceptions: 'Investigasi', approvals: 'Lihat Detail', dashboard: 'Buat Batch' } as Partial<Record<Page, string>>)[page.value] ?? '')

onMounted(async () => {
  try {
    const response = await fetch('/api/v1/health')
    apiStatus.value = response.ok ? 'online' : 'offline'
  } catch {
    apiStatus.value = 'offline'
  }
})

function navigate(id: string) {
  page.value = id as Page
  search.value = ''
  statusFilter.value = 'All statuses'
  activeTab.value = id === 'batchDetail' ? 'Ringkasan' : id === 'settings' ? 'Umum' : 'All'
}

function openFilePicker() {
  document.querySelector<HTMLInputElement>('#file-input')?.click()
}

function openTableRow() {
  if (page.value === 'batches') navigate('batchDetail')
  else if (page.value === 'exceptions') navigate('exceptionDetail')
}

function runAction() {
  if (page.value === 'imports') {
    openFilePicker()
    return
  }
  if (page.value === 'reports') {
    downloadReport()
    return
  }
  if (page.value === 'exceptions') {
    navigate('exceptionDetail')
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
  if (file) selectedFile.value = file.name
}

function downloadReport() {
  const csv = 'Batch,Source,Period,Transactions,Match Rate,Status\nRB-2025-00048,Core Banking,30 Jan 2025,12480,98.72%,Completed\nRB-2025-00047,Virtual Account,30 Jan 2025,8721,99.14%,Completed\n'
  const link = document.createElement('a')
  link.href = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  link.download = 'reconx-report-demo.csv'
  link.click()
  URL.revokeObjectURL(link.href)
}

function submitModal() {
  const label = modal.value === 'user' ? 'Pengguna' : modal.value === 'account' ? 'Sumber' : 'Batch'
  modal.value = ''
  toast.value = `${label} contoh berhasil ditambahkan ke pratinjau.`
  window.setTimeout(() => (toast.value = ''), 2800)
}

function badgeClass(value: string) {
  const v = value.toLowerCase()
  if (v.includes('completed') || v.includes('matched') || v.includes('aktif') || v.includes('success') || v.includes('disetujui')) return 'badge success'
  if (v.includes('progress') || v.includes('review') || v.includes('menunggu') || v.includes('open')) return 'badge warning'
  if (v.includes('unmatched') || v.includes('ditolak') || v.includes('failed')) return 'badge danger'
  return 'badge neutral'
}
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <button class="brand" aria-label="ReconX dashboard" @click="navigate('dashboard')">
        <span class="brand-symbol"><i></i><b></b></span><span>Recon<span class="brand-x">X</span></span>
      </button>
      <div class="workspace-switch"><span class="workspace-avatar">R</span><span class="workspace-name">ReconX Workspace<small>Development</small></span><span class="chevron">⌄</span></div>
      <nav class="navigation" aria-label="Navigasi utama">
        <section v-for="group in navGroups" :key="group.title" class="nav-group">
          <p class="nav-heading">{{ group.title }}</p>
          <button v-for="item in group.items" :key="item.id" class="nav-item" :class="{ active: page === item.id }" @click="navigate(item.id)">
            <svg viewBox="0 0 24 24" aria-hidden="true"><template v-if="item.icon === 'grid'"><rect x="3.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="3.5" width="7" height="7" rx="1.5"/><rect x="3.5" y="13.5" width="7" height="7" rx="1.5"/><rect x="13.5" y="13.5" width="7" height="7" rx="1.5"/></template><template v-else-if="item.icon === 'layers'"><path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5M3 16l9 5 9-5"/></template><template v-else-if="item.icon === 'upload'"><path d="M12 16V4m0 0L7 9m5-5 5 5"/><path d="M4 15v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4"/></template><template v-else-if="item.icon === 'rows'"><path d="M4 5h16M4 10h16M4 15h16M4 20h16"/><path d="M2 5h.01M2 10h.01M2 15h.01M2 20h.01"/></template><template v-else-if="item.icon === 'shuffle'"><path d="m16 3 4 4-4 4M4 7h3c5 0 5 10 10 10h3M16 13l4 4-4 4M4 17h3c1.5 0 2.5-.9 3.5-2"/></template><template v-else-if="item.icon === 'alert'"><path d="M10.3 3.9 2.8 17a2 2 0 0 0 1.7 3h15a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4m0 4h.01"/></template><template v-else-if="item.icon === 'check'"><path d="M9 12.5 11.5 15 16 9.5"/><circle cx="12" cy="12" r="9"/></template><template v-else-if="item.icon === 'chart'"><path d="M4 19V5m0 14h17"/><path d="m7 15 4-4 3 2 6-7"/></template><template v-else-if="item.icon === 'bank'"><path d="m3 9 9-6 9 6"/><path d="M4 10h16M5 10v9m4-9v9m6-9v9m4-9v9M3 21h18"/></template><template v-else-if="item.icon === 'users'"><circle cx="9" cy="8" r="3"/><path d="M3 20v-1a6 6 0 0 1 12 0v1M16 5.5a3 3 0 0 1 0 5.8M18 14a5 5 0 0 1 3 4.5V20"/></template><template v-else-if="item.icon === 'history'"><path d="M3 12a9 9 0 1 0 2.6-6.4L3 8"/><path d="M3 3v5h5m4-1v5l3 2"/></template><template v-else><circle cx="12" cy="12" r="3"/><path d="m19.4 15 .1.1 1.1.9-1 1.7-1.4-.5a8 8 0 0 1-1.5.9l-.3 1.5h-2l-.4-1.5a8 8 0 0 1-1.6-.9l-1.4.5-1-1.7 1.2-1a7 7 0 0 1 0-1.8l-1.2-1 1-1.7 1.4.5a8 8 0 0 1 1.6-.9l.4-1.5h2l.3 1.5a8 8 0 0 1 1.5.9l1.4-.5 1 1.7-1.1.9a7 7 0 0 1 0 1.9Z" transform="translate(-2 -2)"/></template></svg>
            <span>{{ item.label }}</span><span v-if="item.count" class="nav-count">{{ item.count }}</span>
          </button>
        </section>
      </nav>
      <div class="sidebar-bottom"><div class="user-avatar">BA</div><div class="user-info"><strong>Budi Ananta</strong><small>Administrator</small></div><button class="more-button" aria-label="Menu akun">···</button></div>
    </aside>

    <main class="main-area">
      <header class="topbar">
        <div class="crumb"><span>{{ currentNavGroup }}</span><span class="crumb-slash">/</span><strong>{{ currentTitle }}</strong></div>
        <div class="topbar-actions">
          <span class="demo-chip"><i></i> DEMO DATA</span>
          <div class="top-search"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" aria-label="Cari" placeholder="Cari apa saja..." /></div>
          <div class="notification-wrap"><button class="top-icon" aria-label="Notifikasi" @click="showNotifications = !showNotifications"><svg viewBox="0 0 24 24"><path d="M18 9a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4"/></svg><i></i></button><div v-if="showNotifications" class="notification-popover"><strong>Notifikasi</strong><p><span class="notice-dot"></span> 2 batch menunggu semakan</p><p><span class="notice-dot amber-dot"></span> 12 exception belum diselesaikan</p></div></div>
          <span class="top-divider"></span><button class="profile-button"><span class="user-avatar small-avatar">BA</span><span class="profile-name">Budi Ananta<small>Administrator</small></span><span class="chevron">⌄</span></button>
        </div>
      </header>

      <div class="content-area">
        <div class="page-header"><div><div class="breadcrumb-mobile">Workspace <span>/</span> {{ currentTitle }}</div><h1>{{ currentTitle }}</h1><p>{{ subtitles[page] }}</p></div><div class="header-actions"><span v-if="page === 'dashboard'" class="range-select">1 Jan 2025 – 31 Jan 2025 <span>▦</span></span><button v-if="pageAction" class="button primary" @click="runAction"><span class="button-plus">＋</span>{{ pageAction }}</button></div></div>

        <div class="demo-banner"><span class="demo-banner-icon">i</span><span>Anda sedang melihat <strong>data demo</strong>. Informasi pada layar ini hanya contoh desain dan belum tersambung ke data operasional.</span><span class="api-state"><i :class="apiStatus"></i> API {{ apiStatus }}</span></div>

        <section v-if="page === 'dashboard'" class="dashboard-page">
          <div class="kpi-grid">
            <article class="kpi-card"><div class="kpi-top"><span>Total Batch</span><span class="kpi-icon teal">▦</span></div><strong>48</strong><small class="trend positive">↗ 12% <span>vs bulan lalu</span></small><div class="mini-bars"><i v-for="h in [25,35,29,44,40,52,46,64,57,72,65,80]" :key="h" :style="{ height: h + '%' }"></i></div></article>
            <article class="kpi-card"><div class="kpi-top"><span>Matched</span><span class="kpi-icon green">✓</span></div><strong>46.320</strong><small class="trend positive">(98,72%) <span>tingkat kecocokan</span></small><div class="mini-bars green-bars"><i v-for="h in [35,40,38,48,56,54,68,70,64,78,84,92]" :key="h" :style="{ height: h + '%' }"></i></div></article>
            <article class="kpi-card"><div class="kpi-top"><span>Unmatched</span><span class="kpi-icon amber">⚑</span></div><strong>602</strong><small class="trend warning-text">(1,28%) <span>perlu ditinjau</span></small><div class="mini-bars amber-bars"><i v-for="h in [70,62,78,66,58,55,50,48,44,40,36,32]" :key="h" :style="{ height: h + '%' }"></i></div></article>
            <article class="kpi-card"><div class="kpi-top"><span>In Progress</span><span class="kpi-icon blue">◷</span></div><strong>2</strong><small class="trend neutral-text">1 batch <span>siap disemak</span></small><div class="progress-track"><i style="width:66%"></i></div><small class="progress-caption">1 batch sedang berjalan</small></article>
          </div>

          <div class="dashboard-grid">
            <article class="panel trend-panel"><div class="panel-heading"><div><h2>Tren Rekonsiliasi</h2><p>Performa matching selama periode terpilih</p></div><button class="select-button">% Match <span>⌄</span></button></div><div class="chart-wrap"><div class="y-labels"><span>100%</span><span>99,0%</span><span>98,0%</span><span>97,0%</span><span>96,0%</span></div><svg class="line-chart" viewBox="0 0 620 190" preserveAspectRatio="none" aria-label="Grafik tren matching"><defs><linearGradient id="area" x1="0" x2="0" y1="0" y2="1"><stop offset="0%" stop-color="#11b8a4" stop-opacity=".22"/><stop offset="100%" stop-color="#11b8a4" stop-opacity=".01"/></linearGradient></defs><path class="grid-lines" d="M0 15H620M0 55H620M0 95H620M0 135H620M0 175H620"/><path class="chart-area" d="M0 143 C35 130 47 124 78 118 S125 123 155 105 S204 94 232 99 S282 73 310 78 S357 73 388 69 S430 64 465 69 S510 72 542 65 S585 70 620 61 V190 H0Z"/><path class="chart-line" d="M0 143 C35 130 47 124 78 118 S125 123 155 105 S204 94 232 99 S282 73 310 78 S357 73 388 69 S430 64 465 69 S510 72 542 65 S585 70 620 61"/><circle v-for="(point, i) in [[0,143],[78,118],[155,105],[232,99],[310,78],[388,69],[465,69],[542,65],[620,61]]" :key="i" :cx="point[0]" :cy="point[1]" r="3.5" class="chart-point"/></svg><div class="x-labels"><span>1 Jan</span><span>5 Jan</span><span>10 Jan</span><span>15 Jan</span><span>20 Jan</span><span>25 Jan</span><span>31 Jan</span></div></div></article>
            <article class="panel result-panel"><div class="panel-heading"><div><h2>Hasil Rekonsiliasi</h2><p>Ringkasan transaksi demo</p></div><button class="more-button">···</button></div><div class="donut-area"><div class="donut"><div><strong>98,72%</strong><span>Matched</span></div></div><div class="donut-legend"><p><i class="dot green-dot"></i><span>Matched</span><b>46.320</b></p><p><i class="dot amber-dot"></i><span>Unmatched</span><b>602</b></p><p class="legend-total"><span>Total</span><b>46.922</b></p></div></div></article>
          </div>

          <article class="panel table-panel recent-panel"><div class="panel-heading"><div><h2>Batch Rekonsiliasi Terbaru</h2><p>Aktivitas terbaru pada workspace</p></div><button class="link-button" @click="navigate('batches')">Lihat semua batch <span>→</span></button></div><DataTable :columns="tableConfig.batches!.columns" :rows="batchRows.slice(0, 4)" @open="navigate('batchDetail')" /></article>

          <div class="bottom-grid"><article class="panel process-panel"><div class="panel-heading"><div><h2>Alur Proses End-to-End</h2><p>Ringkasan proses rekonsiliasi</p></div><span class="tiny-caption">6 tahap</span></div><div class="process-flow"><div v-for="(step, i) in [{icon:'⇧',title:'Import Data',text:'Unggah file transaksi'}, {icon:'▱',title:'Buat Batch',text:'Pilih rekening & periode'}, {icon:'⇄',title:'Matching',text:'Pencocokan otomatis'}, {icon:'⌕',title:'Investigasi',text:'Tinjau transaksi selisih'}, {icon:'✓',title:'Approval',text:'Persetujuan hasil'}, {icon:'▥',title:'Laporan',text:'Monitoring & export'}]" :key="step.title" class="process-step"><span class="process-icon">{{ step.icon }}</span><div><b>{{ i + 1 }}. {{ step.title }}</b><small>{{ step.text }}</small></div><span v-if="i < 5" class="process-arrow">→</span></div></div></article><article class="panel system-panel"><div class="panel-heading"><div><h2>Ringkasan Sumber Data</h2><p>Status sumber untuk workspace demo</p></div><button class="link-button" @click="navigate('accounts')">Kelola sumber →</button></div><div class="source-row"><span class="source-logo cb">CB</span><span><b>Core Banking</b><small>System · aktif</small></span><span class="badge success">Aktif</span></div><div class="source-row"><span class="source-logo qr">QR</span><span><b>QRIS</b><small>Payment · aktif</small></span><span class="badge success">Aktif</span></div><div class="source-row"><span class="source-logo va">VA</span><span><b>Virtual Account</b><small>System · aktif</small></span><span class="badge success">Aktif</span></div></article></div>
        </section>

        <section v-else-if="page === 'imports'" class="page-grid import-layout">
          <article class="panel import-card"><div class="panel-heading"><div><h2>Unggah File Transaksi</h2><p>Impor file internal dan bank ke dalam batch rekonsiliasi.</p></div><span class="step-count">01 <i></i> 04</span></div><div class="stepper"><div class="step active"><b>1</b><span>Unggah File</span></div><i></i><div class="step"><b>2</b><span>Mapping Kolom</span></div><i></i><div class="step"><b>3</b><span>Preview Data</span></div><i></i><div class="step"><b>4</b><span>Selesai</span></div></div><div class="upload-grid"><button class="drop-zone" @click="openFilePicker"><span class="upload-cloud">⇧</span><b>{{ selectedFile || 'Tarik & letakkan file di sini' }}</b><small>atau pilih file dari komputer</small><span class="button primary small-button">Pilih File</span><small class="format-note">Format CSV, XLSX (maks. 10 MB)</small></button><div class="upload-side"><div class="field-label">Sumber Data</div><select class="form-control"><option>QRIS</option><option>Core Banking</option><option>Virtual Account</option></select><div class="field-label spaced">Batch Rekonsiliasi</div><select class="form-control"><option>RB-2025-00048 · Jan 2025</option><option>Buat batch baru</option></select><div class="check-note"><span>✓</span><div><b>Validasi otomatis</b><small>Format kolom dan duplikasi diperiksa saat file diunggah.</small></div></div></div></div><input id="file-input" class="hidden-input" type="file" accept=".csv,.xlsx" @change="onFile" /><div class="import-actions"><button class="button secondary" @click="selectedFile = ''">Batal</button><button class="button primary" :disabled="!selectedFile" @click="toast = 'File demo siap dipetakan.'">Lanjutkan <span>→</span></button></div></article>
          <article class="panel history-card"><div class="panel-heading"><div><h2>Riwayat Import Terakhir</h2><p>File yang pernah diunggah</p></div><button class="more-button">···</button></div><div v-for="file in [{name:'qris_20250130.csv',meta:'30 Jan 2025 · 12.480 data',size:'1,8 MB'}, {name:'transfer_20250130.csv',meta:'30 Jan 2025 · 8.721 data',size:'2,1 MB'}, {name:'edc_20250129.csv',meta:'29 Jan 2025 · 7.090 data',size:'980 KB'}]" :key="file.name" class="history-item"><span class="file-icon">CSV</span><span class="file-info"><b>{{ file.name }}</b><small>{{ file.meta }} · {{ file.size }}</small></span><span class="check-round">✓</span></div></article>
        </section>

        <section v-else-if="page === 'batchDetail'" class="detail-page">
          <button class="back-link" @click="navigate('batches')">← Kembali ke daftar batch</button><div class="detail-title"><div><div class="title-line"><h2>RB-2025-00048</h2><span class="badge success">Completed</span></div><p>Rekon BCA Jan-2 <span>·</span> Core Banking <span>·</span> 1 Jan 2025 – 31 Jan 2025</p></div><button class="button secondary">Unduh Hasil</button><button class="button primary" @click="navigate('approvals')">Tutup Batch</button></div><div class="kpi-grid detail-kpis"><article class="kpi-card"><div class="kpi-top">Total Transaksi</div><strong>12.480</strong><small class="trend neutral-text">Internal + Bank</small></article><article class="kpi-card"><div class="kpi-top">Matched</div><strong>12.318</strong><small class="trend positive">98,70% cocok</small></article><article class="kpi-card"><div class="kpi-top">Unmatched</div><strong>162</strong><small class="trend warning-text">1,30% perlu ditinjau</small></article><article class="kpi-card"><div class="kpi-top">Dibuat Oleh</div><strong class="small-metric">Sari Wulandari</strong><small class="trend neutral-text">30 Jan 2025 · 10:22</small></article></div><article class="panel detail-tabs"><div class="tabs"><button v-for="tab in ['Ringkasan','Detail Transaksi','Exceptions (12)','Riwayat Aktivitas']" :key="tab" :class="{ selected: activeTab === tab }" @click="activeTab = tab">{{ tab }}</button></div><div class="detail-summary"><div><h3>{{ activeTab === 'Ringkasan' ? 'Ringkasan batch' : activeTab }}</h3><p>Hasil terbaru dari run rekonsiliasi.</p><div class="match-progress"><span>Matched</span><div><i></i></div><b>12.318 (98,70%)</b></div><div class="match-progress unmatched-progress"><span>Unmatched</span><div><i></i></div><b>162 (1,30%)</b></div></div><div class="batch-state"><span class="state-check">✓</span><b>Batch selesai</b><small>Semua hasil tersimpan pada versi sumber 3.</small></div></div></article></section>

        <section v-else-if="page === 'exceptionDetail'" class="detail-page"><button class="back-link" @click="navigate('exceptions')">← Kembali ke Exceptions</button><div class="detail-title"><div><div class="title-line"><h2>QRIS000008323</h2><span class="badge warning">Open</span></div><p>Amount mismatch <span>·</span> RB-2025-00048 <span>·</span> Dibuat 30 Jan 2025, 15:06</p></div><button class="button secondary">Assign ke saya</button><button class="button primary" @click="modal = 'proposal'">Ajukan Manual Match</button></div><div class="comparison-grid"><article class="panel"><div class="panel-heading"><div><h2>Data Internal</h2><p>Core Banking · transaksi asal</p></div><span class="source-logo cb">CB</span></div><div class="data-pair"><span>Reference ID</span><b>QRIS000008323</b></div><div class="data-pair"><span>Tanggal</span><b>30 Jan 2025</b></div><div class="data-pair"><span>Arah</span><b>Kredit</b></div><div class="data-pair"><span>Nominal</span><b>Rp 75.000</b></div></article><article class="panel"><div class="panel-heading"><div><h2>Data Bank</h2><p>QRIS · transaksi sumber</p></div><span class="source-logo qr">QR</span></div><div class="data-pair"><span>Reference ID</span><b>QRIS000008323</b></div><div class="data-pair"><span>Tanggal</span><b>30 Jan 2025</b></div><div class="data-pair"><span>Arah</span><b>Kredit</b></div><div class="data-pair mismatch"><span>Nominal</span><b>Rp 70.000 <small>Selisih Rp 5.000</small></b></div></article></div><article class="panel timeline-panel"><div class="panel-heading"><div><h2>Riwayat Investigasi</h2><p>Jejak tindakan exception ini</p></div><button class="button secondary small-button">＋ Tambah Catatan</button></div><div class="timeline"><div><i></i><span><b>Exception dibuat oleh sistem</b><small>Rule M01 menemukan perbedaan nominal · 30 Jan 2025, 15:06</small></span></div><div><i></i><span><b>Ditugaskan ke Sari Wulandari</b><small>30 Jan 2025, 15:10</small></span></div></div></article></section>

        <section v-else-if="page === 'reports'" class="reports-page"><article class="panel report-filters"><div class="filter-block"><label>Periode</label><button class="range-select">1 Jan 2025 – 31 Jan 2025 <span>▦</span></button></div><div class="filter-block"><label>Sumber</label><select class="form-control"><option>Semua Sumber</option><option>Core Banking</option><option>QRIS</option></select></div><button class="button primary" @click="downloadReport">⇩ Unduh Laporan</button></article><div class="kpi-grid report-kpis"><article class="kpi-card"><div class="kpi-top">Total Transaksi</div><strong>1.286.420</strong><small class="trend positive">↗ 8,42% vs bulan lalu</small></article><article class="kpi-card"><div class="kpi-top">Matched</div><strong>1.269.732</strong><small class="trend positive">98,70% match rate</small></article><article class="kpi-card"><div class="kpi-top">Unmatched</div><strong>16.688</strong><small class="trend warning-text">1,30% perlu ditinjau</small></article></div><div class="dashboard-grid"><article class="panel trend-panel"><div class="panel-heading"><div><h2>Tren Unmatched</h2><p>Jumlah transaksi belum cocok per bulan</p></div><button class="select-button">Tahun 2025 ⌄</button></div><div class="bar-chart"><div v-for="(bar, i) in [48,62,54,78,68,58,45,73,61,88,69,56]" :key="i" class="bar-column"><div class="bar"><i :style="{ height: bar + '%' }"></i></div><small>{{ ['Agu','Sep','Okt','Nov','Des','Jan','Feb','Mar','Apr','Mei','Jun','Jul'][i] }}</small></div></div></article><article class="panel"><div class="panel-heading"><div><h2>Top Sumber Selisih</h2><p>Distribusi exception berdasarkan sumber</p></div></div><div v-for="item in [{name:'QRIS',n:'6.340',p:'37,4%'},{name:'Transfer',n:'4.780',p:'28,2%'},{name:'EDC',n:'2.910',p:'17,1%'},{name:'Virtual Account',n:'1.240',p:'7,3%'}]" :key="item.name" class="rank-row"><span>{{ item.name }}</span><div><i :style="{ width: item.p }"></i></div><b>{{ item.n }}</b><small>{{ item.p }}</small></div></article></div></section>

        <section v-else-if="page === 'settings'" class="settings-page"><div class="settings-nav panel"><button v-for="s in ['Umum','Parameter Matching','Integrasi','Notifikasi','Manajemen Role']" :key="s" :class="{ selected: activeTab === s || (activeTab === 'All' && s === 'Umum') }" @click="activeTab = s">{{ s }} <span>›</span></button></div><article class="panel settings-card"><div class="panel-heading"><div><h2>{{ activeTab === 'All' ? 'Pengaturan Sistem' : activeTab }}</h2><p>Konfigurasi umum, parameter rekonsiliasi, dan integrasi.</p></div></div><section class="settings-section"><h3>Umum</h3><div class="settings-row"><div><b>Zona Waktu Default</b><small>Digunakan untuk tanggal batch dan audit log.</small></div><select class="form-control compact"><option>Asia/Jakarta (WIB, UTC+7)</option></select></div><div class="settings-row"><div><b>Maksimum Ukuran Import</b><small>Batas ukuran file transaksi yang diterima.</small></div><select class="form-control compact"><option>10 MB</option><option>20 MB</option></select></div><div class="settings-row"><div><b>Format CSV</b><small>Delimiter yang digunakan pada file transaksi.</small></div><select class="form-control compact"><option>Koma (,)</option><option>Titik koma (;)</option></select></div></section><section class="settings-section"><h3>Parameter Matching</h3><div class="matching-rule"><span class="rule-code">M01</span><div><b>Auto Exact Match</b><small>Reference, tanggal, arah, mata uang, dan nominal harus sama.</small></div><span class="readonly-chip">Aktif · Read-only</span></div><div class="settings-row"><div><b>Toleransi Selisih Tanggal</b><small>Hanya dapat diajukan untuk manual match dan perlu checker approval.</small></div><select class="form-control compact"><option>3 hari kalender</option></select></div></section><button class="button primary" @click="toast = 'Pengaturan demo tersimpan.'">Simpan Perubahan</button></article></section>

        <section v-else class="list-page"><div v-if="page === 'matching'" class="kpi-grid matching-kpis"><article class="kpi-card"><div class="kpi-top">Total Transaksi</div><strong>12.480</strong><small class="trend neutral-text">Detail hasil batch</small></article><article class="kpi-card"><div class="kpi-top">Matched</div><strong>12.318</strong><small class="trend positive">98,70%</small></article><article class="kpi-card"><div class="kpi-top">Unmatched</div><strong>162</strong><small class="trend warning-text">1,30%</small></article><article class="kpi-card"><div class="kpi-top">Batch Aktif</div><select class="form-control compact"><option>RB-2025-00048 · Jan 2025</option></select></article></div><div v-if="page === 'exceptions'" class="kpi-grid exception-kpis"><article class="kpi-card"><div class="kpi-top">Open Exceptions</div><strong>8</strong><small class="trend warning-text">Perlu investigasi</small></article><article class="kpi-card"><div class="kpi-top">In Review</div><strong>4</strong><small class="trend neutral-text">Sedang ditangani</small></article><article class="kpi-card"><div class="kpi-top">Rata-rata Aging</div><strong>1,8 <small>hari</small></strong><small class="trend positive">↓ 0,4 hari</small></article><article class="kpi-card"><div class="kpi-top">Menunggu Approval</div><strong>3</strong><small class="trend neutral-text">Antrean checker</small></article></div><article class="panel table-panel"><div class="table-toolbar"><div class="tabs"><button v-for="tab in (page === 'matching' ? ['Semua (12.480)','Matched (12.318)','Unmatched (162)'] : page === 'approvals' ? ['Menunggu (3)','Riwayat','Semua'] : page === 'exceptions' ? ['Semua (12)','Open (8)','In Review (4)'] : ['Semua','Aktif','Arsip'])" :key="tab" :class="{ selected: activeTab === tab || (activeTab === 'All' && tab.startsWith('Semua')) }" @click="activeTab = tab">{{ tab }}</button></div><div class="filters"><label class="search-field"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" :placeholder="page === 'exceptions' ? 'Cari Ref. No. / Merchant...' : 'Cari batch / nama...'" /></label><select v-if="['batches','exceptions','transactions'].includes(page)" v-model="sourceFilter" class="filter-select"><option>All sources</option><option>Core Banking</option><option>QRIS</option><option>Virtual Account</option><option>EDC</option></select><select v-model="statusFilter" class="filter-select"><option>All statuses</option><option>Completed</option><option>In Progress</option><option>Matched</option><option>Unmatched</option><option>Open</option><option>In Review</option><option>Menunggu</option></select><button class="filter-icon" aria-label="Filter">☷</button></div></div><DataTable v-if="currentTable" :columns="currentTable.columns" :rows="filteredRows" :link-key="page === 'batches' ? 'batch' : page === 'exceptions' ? 'ref' : ''" @open="openTableRow" /><div class="table-footer"><span>Menampilkan 1–{{ filteredRows.length }} dari {{ currentTable?.rows.length }} data</span><div><button disabled>‹</button><button class="current-page">1</button><button>2</button><button>3</button><button>…</button><button>10</button><button>›</button></div></div></article></section>

        <footer class="app-footer">ReconX <span>·</span> Bank Transaction Reconciliation &amp; Exception Management <span class="footer-version">v0.1 · Demo workspace</span></footer>
      </div>
    </main>

    <div v-if="modal" class="modal-backdrop" @click.self="modal = ''"><section class="modal-card"><div class="modal-heading"><div><h2>{{ modal === 'batch' ? 'Buat Batch Rekonsiliasi' : modal === 'account' ? 'Tambah Sumber Data' : modal === 'proposal' ? 'Ajukan Manual Match' : 'Tambah Pengguna' }}</h2><p>{{ modal === 'batch' ? 'Tentukan rekening dan periode rekonsiliasi.' : modal === 'proposal' ? 'Permintaan ini akan ditinjau oleh Checker.' : 'Lengkapi informasi untuk melanjutkan.' }}</p></div><button class="close-button" @click="modal = ''">×</button></div><label v-if="modal === 'batch' || modal === 'account'" class="field-label">{{ modal === 'batch' ? 'Rekening Sumber' : 'Nama Sumber' }}<select class="form-control"><option>Core Banking</option><option>QRIS</option><option>Virtual Account</option><option>EDC</option></select></label><label v-if="modal === 'batch'" class="field-label">Periode Batch<div class="date-pair"><input class="form-control" type="date" value="2025-01-01" /><input class="form-control" type="date" value="2025-01-31" /></div></label><label v-if="modal === 'user'" class="field-label">Nama Lengkap<input class="form-control" placeholder="Nama pengguna" /></label><label v-if="modal === 'user'" class="field-label">Email<input class="form-control" type="email" placeholder="nama@perusahaan.co.id" /></label><label v-if="modal === 'user'" class="field-label">Role<select class="form-control"><option>Operator</option><option>Checker</option><option>Admin</option><option>Auditor</option></select></label><label v-if="modal === 'proposal'" class="field-label">Alasan Pengajuan<textarea class="form-control" rows="4" placeholder="Jelaskan alasan manual match (minimum 20 karakter)."></textarea></label><div class="modal-actions"><button class="button secondary" @click="modal = ''">Batal</button><button class="button primary" @click="submitModal">{{ modal === 'proposal' ? 'Kirim untuk Approval' : 'Simpan' }}</button></div></section></div>
    <div v-if="toast" class="toast-message"><span>✓</span>{{ toast }}</div>
  </div>
</template>
