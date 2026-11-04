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

const batchRows: Row[] = [
  { batch: 'REC-20261103-0001', name: 'Contoh Rekonsiliasi FSD', source: 'INTERNAL + BANK', period: '03 Nov 2026', total: '3 + 3', rate: '33.33%', status: 'Needs Review', updated: '03 Nov 2026' },
]
const transactionRows: Row[] = [
  { ref: 'TRX001', date: '03 Nov 2026', source: 'INTERNAL', description: 'Catatan transaksi internal', debit: '—', credit: 'Rp 500.000', status: 'Matched', batch: 'REC-20261103-0001' },
  { ref: 'TRX001', date: '03 Nov 2026', source: 'BANK', description: 'Mutasi rekening bank', debit: '—', credit: 'Rp 500.000', status: 'Matched', batch: 'REC-20261103-0001' },
  { ref: 'TRX002', date: '03 Nov 2026', source: 'INTERNAL', description: 'Catatan transaksi internal', debit: '—', credit: 'Rp 750.000', status: 'Amount Mismatch', batch: 'REC-20261103-0001' },
  { ref: 'TRX002', date: '03 Nov 2026', source: 'BANK', description: 'Mutasi rekening bank', debit: '—', credit: 'Rp 700.000', status: 'Amount Mismatch', batch: 'REC-20261103-0001' },
  { ref: 'TRX003', date: '03 Nov 2026', source: 'INTERNAL', description: 'Catatan transaksi internal', debit: '—', credit: 'Rp 250.000', status: 'Internal Only', batch: 'REC-20261103-0001' },
  { ref: 'TRX004', date: '03 Nov 2026', source: 'BANK', description: 'Mutasi rekening bank', debit: '—', credit: 'Rp 300.000', status: 'Bank Only', batch: 'REC-20261103-0001' },
]
const exceptionRows: Row[] = [
  { ref: 'TRX002', type: 'Amount mismatch', batch: 'REC-20261103-0001', nominalA: 'Rp 750.000', nominalB: 'Rp 700.000', difference: 'Rp 50.000', owner: 'Belum ditugaskan', status: 'Open' },
  { ref: 'TRX003', type: 'Internal only', batch: 'REC-20261103-0001', nominalA: 'Rp 250.000', nominalB: '—', difference: 'Rp 250.000', owner: 'Belum ditugaskan', status: 'Open' },
  { ref: 'TRX004', type: 'Bank only', batch: 'REC-20261103-0001', nominalA: '—', nominalB: 'Rp 300.000', difference: 'Rp 300.000', owner: 'Belum ditugaskan', status: 'Open' },
]
const approvalRows: Row[] = []
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
  { name: 'Dewi Lestari', email: 'dewi@reconx.co.id', role: 'Auditor', lastActive: '1 jam lalu', status: 'Aktif' },
]
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
const auditRows: Row[] = [
  { time: '03 Nov 2026 10:05', actor: 'Sari Wulandari', activity: 'Import file', module: 'Import', detail: 'internal.csv', status: 'Success' },
  { time: '03 Nov 2026 10:06', actor: 'Sari Wulandari', activity: 'Import file', module: 'Import', detail: 'bank.csv', status: 'Success' },
  { time: '03 Nov 2026 10:07', actor: 'Sari Wulandari', activity: 'Matching run', module: 'Reconciliation', detail: 'REC-20261103-0001', status: 'Success' },
  { time: '03 Nov 2026 10:07', actor: 'System', activity: 'Exceptions created', module: 'Exceptions', detail: 'TRX002 · TRX003 · TRX004', status: 'Success' },
]

const tableConfig: Partial<Record<Page, { columns: { key: string; label: string }[]; rows: Row[] }>> = {
  batches: { columns: [{ key: 'batch', label: 'No. Batch' }, { key: 'name', label: 'Nama Batch' }, { key: 'source', label: 'Sumber' }, { key: 'period', label: 'Periode' }, { key: 'total', label: 'Total Tx' }, { key: 'rate', label: 'Match Rate' }, { key: 'status', label: 'Status' }, { key: 'updated', label: 'Diperbarui' }], rows: batchRows },
  transactions: { columns: [{ key: 'ref', label: 'Ref. No.' }, { key: 'date', label: 'Tanggal' }, { key: 'source', label: 'Sumber' }, { key: 'description', label: 'Deskripsi' }, { key: 'debit', label: 'Debit' }, { key: 'credit', label: 'Kredit' }, { key: 'status', label: 'Status' }, { key: 'batch', label: 'Batch' }], rows: transactionRows },
  matching: { columns: [{ key: 'ref', label: 'Ref. No.' }, { key: 'date', label: 'Tanggal' }, { key: 'internal', label: 'Data Internal' }, { key: 'bank', label: 'Data Bank' }, { key: 'amount', label: 'Nominal' }, { key: 'rule', label: 'Rule' }, { key: 'status', label: 'Status' }], rows: [
    { ref: 'TRX001', date: '03 Nov 2026', internal: 'CREDIT · Rp 500.000', bank: 'CREDIT · Rp 500.000', amount: 'Rp 500.000', rule: 'M01 exact', status: 'Matched' },
    { ref: 'TRX002', date: '03 Nov 2026', internal: 'CREDIT · Rp 750.000', bank: 'CREDIT · Rp 700.000', amount: 'Rp 50.000 diff.', rule: 'AMOUNT_MISMATCH', status: 'Amount Mismatch' },
    { ref: 'TRX003', date: '03 Nov 2026', internal: 'CREDIT · Rp 250.000', bank: '—', amount: 'Rp 250.000', rule: 'INTERNAL_ONLY', status: 'Internal Only' },
    { ref: 'TRX004', date: '03 Nov 2026', internal: '—', bank: 'CREDIT · Rp 300.000', amount: 'Rp 300.000', rule: 'BANK_ONLY', status: 'Bank Only' },
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
const pageAction = computed(() => ({ batches: 'Buat Batch', imports: 'Unggah File', accounts: 'Tambah Sumber', users: 'Tambah Pengguna', reports: 'Unduh Laporan', exceptions: 'Investigasi', dashboard: 'Buat Batch' } as Partial<Record<Page, string>>)[page.value] ?? '')

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
  activeTab.value = id === 'batchDetail' ? 'Ringkasan' : id === 'settings' ? 'Umum' : id === 'approvals' ? 'Menunggu (0)' : 'All'
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
  const csv = 'Batch,Source,Period,Internal transactions,Bank transactions,Matched references,Exceptions,Internal match rate,Status\nREC-20261103-0001,INTERNAL + BANK,2026-11-03,3,3,1,3,33.33%,Needs Review\n'
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
  if (v.includes('completed') || (v.includes('matched') && !v.includes('unmatched')) || v.includes('aktif') || v.includes('success') || v.includes('disetujui')) return 'badge success'
  if (v.includes('progress') || v.includes('review') || v.includes('menunggu') || v.includes('open')) return 'badge warning'
  if (v.includes('unmatched') || v.includes('only') || v.includes('mismatch') || v.includes('ditolak') || v.includes('failed')) return 'badge danger'
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
        <div class="page-header"><div><div class="breadcrumb-mobile">Workspace <span>/</span> {{ currentTitle }}</div><h1>{{ currentTitle }}</h1><p>{{ subtitles[page] }}</p></div><div class="header-actions"><span v-if="page === 'dashboard'" class="range-select">03 Nov 2026 · Dataset FSD <span>▦</span></span><button v-if="pageAction" class="button primary" @click="runAction"><span class="button-plus">＋</span>{{ pageAction }}</button></div></div>

        <div class="demo-banner"><span class="demo-banner-icon">i</span><span><strong>Demo sintetis.</strong> Bukan data nasabah atau bank nyata. Sistem tidak terhubung ke bank, memindahkan dana, atau memposting jurnal.</span><span class="api-state"><i :class="apiStatus"></i> API {{ apiStatus }}</span></div>

        <section v-if="page === 'dashboard'" class="dashboard-page">
          <div class="kpi-grid">
            <article class="kpi-card"><div class="kpi-top"><span>Batch Contoh</span><span class="kpi-icon teal">▦</span></div><strong>1</strong><small class="trend neutral-text">03 Nov 2026 <span>dataset FSD</span></small><div class="mini-bars"><i v-for="h in [30,44,37,61,55,75]" :key="h" :style="{ height: h + '%' }"></i></div></article>
            <article class="kpi-card"><div class="kpi-top"><span>Matched Pairs</span><span class="kpi-icon green">✓</span></div><strong>1</strong><small class="trend positive">33,33% <span>dari 3 internal</span></small><div class="mini-bars green-bars"><i v-for="h in [38,50,43,65,58,76]" :key="h" :style="{ height: h + '%' }"></i></div></article>
            <article class="kpi-card"><div class="kpi-top"><span>Exceptions Aktif</span><span class="kpi-icon amber">⚑</span></div><strong>3</strong><small class="trend warning-text">1 amount mismatch · 2 one-sided</small><div class="mini-bars amber-bars"><i v-for="h in [72,62,79,58,70,48]" :key="h" :style="{ height: h + '%' }"></i></div></article>
            <article class="kpi-card"><div class="kpi-top"><span>Status Batch</span><span class="kpi-icon blue">◷</span></div><strong class="small-metric">Review</strong><small class="trend warning-text">Tidak dapat ditutup</small><div class="progress-track"><i style="width:33%"></i></div><small class="progress-caption">3 exception masih aktif</small></article>
          </div>

          <div class="dashboard-grid">
            <article class="panel trend-panel"><div class="panel-heading"><div><h2>Contoh Hasil End-to-End</h2><p>Rasio match = transaksi internal matched ÷ total internal (1 ÷ 3)</p></div><span class="badge warning">Needs Review</span></div><div class="scenario-flow"><div><small>INTERNAL</small><b>3 transaksi</b><span>Rp 1.500.000</span></div><i>⇄</i><div><small>BANK</small><b>3 transaksi</b><span>Rp 1.500.000</span></div><i>→</i><div class="scenario-result"><small>HASIL</small><b>1 match · 3 exceptions</b><span>Batch belum boleh ditutup</span></div></div><div class="scenario-note"><b>TRX001</b> matched · <b>TRX002</b> selisih Rp 50.000 · <b>TRX003</b> internal only · <b>TRX004</b> bank only</div></article>
            <article class="panel result-panel"><div class="panel-heading"><div><h2>Hasil Rekonsiliasi</h2><p>1 referensi matched dari 4 referensi unik</p></div><button class="more-button">···</button></div><div class="donut-area"><div class="donut"><div><strong>33,33%</strong><span>rate internal</span></div></div><div class="donut-legend"><p><i class="dot green-dot"></i><span>Matched pair</span><b>1</b></p><p><i class="dot amber-dot"></i><span>Exception</span><b>3</b></p><p class="legend-total"><span>Internal · Bank</span><b>3 · 3</b></p></div></div></article>
          </div>

          <article class="panel table-panel recent-panel"><div class="panel-heading"><div><h2>Batch Rekonsiliasi Terbaru</h2><p>Aktivitas terbaru pada workspace</p></div><button class="link-button" @click="navigate('batches')">Lihat semua batch <span>→</span></button></div><DataTable :columns="tableConfig.batches!.columns" :rows="batchRows.slice(0, 4)" @open="navigate('batchDetail')" /></article>

          <div class="bottom-grid"><article class="panel process-panel"><div class="panel-heading"><div><h2>Alur Proses End-to-End</h2><p>Ringkasan proses rekonsiliasi</p></div><span class="tiny-caption">6 tahap</span></div><div class="process-flow"><div v-for="(step, i) in [{icon:'▱',title:'Buat Batch',text:'Pilih rekening & periode'}, {icon:'⇧',title:'Import Data',text:'Unggah CSV internal dan bank'}, {icon:'⇄',title:'Matching',text:'Pencocokan otomatis'}, {icon:'⌕',title:'Investigasi',text:'Tinjau transaksi selisih'}, {icon:'✓',title:'Approval',text:'Pemeriksaan Checker'}, {icon:'▥',title:'Tutup Batch',text:'Kunci hasil yang disetujui'}]" :key="step.title" class="process-step"><span class="process-icon">{{ step.icon }}</span><div><b>{{ i + 1 }}. {{ step.title }}</b><small>{{ step.text }}</small></div><span v-if="i < 5" class="process-arrow">→</span></div></div></article><article class="panel system-panel"><div class="panel-heading"><div><h2>Ringkasan Sumber Data</h2><p>Status sumber untuk workspace demo</p></div><button class="link-button" @click="navigate('accounts')">Kelola sumber →</button></div><div class="source-row"><span class="source-logo cb">CB</span><span><b>Core Banking</b><small>System · aktif</small></span><span class="badge success">Aktif</span></div><div class="source-row"><span class="source-logo qr">QR</span><span><b>QRIS</b><small>Payment · aktif</small></span><span class="badge success">Aktif</span></div><div class="source-row"><span class="source-logo va">VA</span><span><b>Virtual Account</b><small>System · aktif</small></span><span class="badge success">Aktif</span></div></article></div>
        </section>

        <section v-else-if="page === 'imports'" class="page-grid import-layout">
          <article class="panel import-card"><div class="panel-heading"><div><h2>Unggah File Transaksi</h2><p>Format contoh CSV tersedia untuk kedua sumber.</p></div><span class="step-count">01 <i></i> 04</span></div><div class="stepper"><div class="step active"><b>1</b><span>Unggah File</span></div><i></i><div class="step"><b>2</b><span>Mapping Kolom</span></div><i></i><div class="step"><b>3</b><span>Preview Data</span></div><i></i><div class="step"><b>4</b><span>Selesai</span></div></div><div class="upload-grid"><button class="drop-zone" @click="openFilePicker"><span class="upload-cloud">⇧</span><b>{{ selectedFile || 'Tarik & letakkan file di sini' }}</b><small>atau pilih file dari komputer</small><span class="button primary small-button">Pilih File</span><small class="format-note">Format CSV (maks. 10 MB)</small></button><div class="upload-side"><div class="field-label">Sumber Data</div><select class="form-control"><option>INTERNAL · Catatan perusahaan</option><option>BANK · Mutasi bank</option></select><div class="field-label spaced">Batch Rekonsiliasi</div><select class="form-control"><option>REC-20261103-0001 · 03 Nov 2026</option><option>Buat batch baru</option></select><div class="check-note"><span>✓</span><div><b>Dataset sintetis</b><small>Contoh data tidak memuat informasi nasabah nyata atau kredensial bank.</small></div></div></div></div><input id="file-input" class="hidden-input" type="file" accept=".csv" @change="onFile" /><div class="import-actions"><button class="button secondary" @click="selectedFile = ''">Batal</button><button class="button primary" :disabled="!selectedFile" @click="toast = 'File demo siap dipetakan.'">Lanjutkan <span>→</span></button></div></article>
          <article class="panel history-card"><div class="panel-heading"><div><h2>Contoh CSV FSD</h2><p>Fixture sintetis di backend/examples</p></div><span class="badge neutral">2 file</span></div><div v-for="file in [{name:'internal.csv',meta:'03 Nov 2026 · 3 baris',size:'INTERNAL'}, {name:'bank.csv',meta:'03 Nov 2026 · 3 baris',size:'BANK'}]" :key="file.name" class="history-item"><span class="file-icon">CSV</span><span class="file-info"><b>{{ file.name }}</b><small>{{ file.meta }} · {{ file.size }}</small></span><span class="check-round">✓</span></div></article>
        </section>

        <section v-else-if="page === 'batchDetail'" class="detail-page">
          <button class="back-link" @click="navigate('batches')">← Kembali ke daftar batch</button><div class="detail-title"><div><div class="title-line"><h2>REC-20261103-0001</h2><span class="badge warning">REVIEW</span></div><p>Contoh Rekonsiliasi FSD <span>·</span> Satu account <span>·</span> 03 Nov 2026</p></div><button class="button secondary" @click="navigate('matching')">Lihat Hasil</button><button class="button primary" disabled title="Batch belum dapat ditutup selama 3 exception aktif">Tutup Batch</button></div><div class="kpi-grid detail-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>3</strong><small class="trend neutral-text">Total Rp 1.500.000</small></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>3</strong><small class="trend neutral-text">Total Rp 1.500.000</small></article><article class="kpi-card"><div class="kpi-top">Matched</div><strong>1 pasangan</strong><small class="trend positive">TRX001 · 33,33% internal</small></article><article class="kpi-card"><div class="kpi-top">Exceptions</div><strong>3</strong><small class="trend warning-text">Batch belum boleh ditutup</small></article></div><article class="panel detail-tabs"><div class="tabs"><button v-for="tab in ['Ringkasan','Detail Transaksi','Exceptions (3)','Riwayat Aktivitas']" :key="tab" :class="{ selected: activeTab === tab }" @click="activeTab = tab">{{ tab }}</button></div><div class="detail-summary"><div><h3>{{ activeTab === 'Ringkasan' ? 'Ringkasan batch' : activeTab }}</h3><p>Data sintetis dari contoh end-to-end FSD.</p><div class="match-progress"><span>Matched</span><div><i></i></div><b>1 pasangan</b></div><div class="match-progress unmatched-progress"><span>Exceptions</span><div><i></i></div><b>3 aktif</b></div></div><div class="batch-state"><span class="state-check">!</span><b>Belum dapat ditutup</b><small>Selesaikan selisih melalui koreksi sumber dan jalankan ulang rekonsiliasi.</small></div></div></article></section>

        <section v-else-if="page === 'exceptionDetail'" class="detail-page"><button class="back-link" @click="navigate('exceptions')">← Kembali ke Exceptions</button><div class="detail-title"><div><div class="title-line"><h2>TRX002</h2><span class="badge warning">AMOUNT_MISMATCH</span></div><p>REC-20261103-0001 <span>·</span> Data sintetis <span>·</span> 03 Nov 2026</p></div><button class="button secondary">Assign ke saya</button></div><div class="comparison-grid"><article class="panel"><div class="panel-heading"><div><h2>Data Internal</h2><p>internal.csv · sumber pembukuan perusahaan</p></div><span class="source-logo cb">IN</span></div><div class="data-pair"><span>Reference ID</span><b>TRX002</b></div><div class="data-pair"><span>Tanggal</span><b>03 Nov 2026</b></div><div class="data-pair"><span>Arah</span><b>CREDIT</b></div><div class="data-pair"><span>Nominal</span><b>Rp 750.000</b></div></article><article class="panel"><div class="panel-heading"><div><h2>Data Bank</h2><p>bank.csv · mutasi rekening sintetis</p></div><span class="source-logo qr">BK</span></div><div class="data-pair"><span>Reference ID</span><b>TRX002</b></div><div class="data-pair"><span>Tanggal</span><b>03 Nov 2026</b></div><div class="data-pair"><span>Arah</span><b>CREDIT</b></div><div class="data-pair mismatch"><span>Nominal</span><b>Rp 700.000 <small>Selisih Rp 50.000</small></b></div></article></div><article class="panel timeline-panel"><div class="panel-heading"><div><h2>Hasil Pemeriksaan</h2><p>Contoh ini tidak dapat diselesaikan dengan catatan manual.</p></div></div><div class="timeline"><div><i></i><span><b>AMOUNT_MISMATCH</b><small>Amount berbeda; batch tidak dapat ditutup sampai sumber diperbaiki dan diimpor ulang.</small></span></div></div></article></section>

        <section v-else-if="page === 'reports'" class="reports-page"><article class="panel report-filters"><div class="filter-block"><label>Periode</label><button class="range-select">03 Nov 2026 <span>▦</span></button></div><div class="filter-block"><label>Sumber</label><select class="form-control"><option>Semua Sumber</option><option>INTERNAL</option><option>BANK</option></select></div><button class="button primary" @click="downloadReport">⇩ Unduh Laporan</button></article><div class="kpi-grid report-kpis"><article class="kpi-card"><div class="kpi-top">Baris Sumber</div><strong>6</strong><small class="trend neutral-text">3 internal + 3 bank</small></article><article class="kpi-card"><div class="kpi-top">Matched Pairs</div><strong>1</strong><small class="trend positive">33,33% dari internal</small></article><article class="kpi-card"><div class="kpi-top">Exceptions</div><strong>3</strong><small class="trend warning-text">Aktif pada batch contoh</small></article></div><div class="dashboard-grid"><article class="panel trend-panel"><div class="panel-heading"><div><h2>Distribusi Exception</h2><p>Snapshot tiga exception pada batch FSD</p></div><button class="select-button">Dataset FSD ⌄</button></div><div class="bar-chart"><div v-for="(bar, i) in [45,45,45]" :key="i" class="bar-column"><div class="bar"><i :style="{ height: bar + '%' }"></i></div><small>{{ ['Mismatch','Internal','Bank'][i] }}</small></div></div></article><article class="panel"><div class="panel-heading"><div><h2>Jenis Selisih</h2><p>Satu kasus per kategori pada contoh</p></div></div><div v-for="item in [{name:'Amount mismatch',n:'1',p:'33,3%'},{name:'Internal only',n:'1',p:'33,3%'},{name:'Bank only',n:'1',p:'33,3%'}]" :key="item.name" class="rank-row"><span>{{ item.name }}</span><div><i :style="{ width: item.p }"></i></div><b>{{ item.n }}</b><small>{{ item.p }}</small></div></article></div></section>

        <section v-else-if="page === 'settings'" class="settings-page"><div class="settings-nav panel"><button v-for="s in ['Umum','Parameter Matching','Integrasi','Notifikasi','Manajemen Role']" :key="s" :class="{ selected: activeTab === s || (activeTab === 'All' && s === 'Umum') }" @click="activeTab = s">{{ s }} <span>›</span></button></div><article class="panel settings-card"><div class="panel-heading"><div><h2>{{ activeTab === 'All' ? 'Pengaturan Sistem' : activeTab }}</h2><p>Konfigurasi umum, parameter rekonsiliasi, dan integrasi.</p></div></div><section class="settings-section"><h3>Umum</h3><div class="settings-row"><div><b>Zona Waktu Default</b><small>Digunakan untuk tanggal batch dan audit log.</small></div><select class="form-control compact"><option>Asia/Jakarta (WIB, UTC+7)</option></select></div><div class="settings-row"><div><b>Maksimum Ukuran Import</b><small>Batas ukuran file transaksi yang diterima.</small></div><select class="form-control compact"><option>10 MB</option><option>20 MB</option></select></div><div class="settings-row"><div><b>Format CSV</b><small>Delimiter yang digunakan pada file transaksi.</small></div><select class="form-control compact"><option>Koma (,)</option><option>Titik koma (;)</option></select></div></section><section class="settings-section"><h3>Parameter Matching</h3><div class="matching-rule"><span class="rule-code">M01</span><div><b>Auto Exact Match</b><small>Reference, tanggal, arah, mata uang, dan nominal harus sama.</small></div><span class="readonly-chip">Aktif · Read-only</span></div><div class="settings-row"><div><b>Toleransi Selisih Tanggal</b><small>Hanya dapat diajukan untuk manual match dan perlu checker approval.</small></div><select class="form-control compact"><option>3 hari kalender</option></select></div></section><button class="button primary" @click="toast = 'Pengaturan demo tersimpan.'">Simpan Perubahan</button></article></section>

        <section v-else class="list-page"><div v-if="page === 'matching'" class="kpi-grid matching-kpis"><article class="kpi-card"><div class="kpi-top">Transaksi Internal</div><strong>3</strong><small class="trend neutral-text">Total Rp 1.500.000</small></article><article class="kpi-card"><div class="kpi-top">Transaksi Bank</div><strong>3</strong><small class="trend neutral-text">Total Rp 1.500.000</small></article><article class="kpi-card"><div class="kpi-top">Matched Pairs</div><strong>1</strong><small class="trend positive">33,33% dari internal</small></article><article class="kpi-card"><div class="kpi-top">Exceptions</div><strong>3</strong><small class="trend warning-text">Batch REC-20261103-0001</small></article></div><div v-if="page === 'exceptions'" class="kpi-grid exception-kpis"><article class="kpi-card"><div class="kpi-top">Amount mismatch</div><strong>1</strong><small class="trend warning-text">TRX002 · Rp 50.000</small></article><article class="kpi-card"><div class="kpi-top">Internal only</div><strong>1</strong><small class="trend neutral-text">TRX003 · Rp 250.000</small></article><article class="kpi-card"><div class="kpi-top">Bank only</div><strong>1</strong><small class="trend neutral-text">TRX004 · Rp 300.000</small></article><article class="kpi-card"><div class="kpi-top">Batch status</div><strong class="small-metric">Needs Review</strong><small class="trend warning-text">Tidak dapat ditutup</small></article></div><article class="panel table-panel"><div class="table-toolbar"><div class="tabs"><button v-for="tab in (page === 'matching' ? ['Semua (4)','Matched (1)','Exceptions (3)'] : page === 'approvals' ? ['Menunggu (0)','Riwayat','Semua'] : page === 'exceptions' ? ['Semua (3)','Open (3)','In Review (0)'] : ['Semua','Aktif','Arsip'])" :key="tab" :class="{ selected: activeTab === tab || (activeTab === 'All' && tab.startsWith('Semua')) }" @click="activeTab = tab">{{ tab }}</button></div><div class="filters"><label class="search-field"><svg viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m16 16 4 4"/></svg><input v-model="search" :placeholder="page === 'exceptions' ? 'Cari reference / batch...' : 'Cari batch / reference...'" /></label><select v-if="['batches','exceptions','transactions'].includes(page)" v-model="sourceFilter" class="filter-select"><option>All sources</option><option>INTERNAL</option><option>BANK</option></select><select v-model="statusFilter" class="filter-select"><option>All statuses</option><option>Needs Review</option><option>Matched</option><option>Amount Mismatch</option><option>Internal Only</option><option>Bank Only</option><option>Open</option></select><button class="filter-icon" aria-label="Filter">☷</button></div></div><DataTable v-if="currentTable" :columns="currentTable.columns" :rows="filteredRows" :link-key="page === 'batches' ? 'batch' : page === 'exceptions' ? 'ref' : ''" @open="openTableRow" /><div class="table-footer"><span>Menampilkan 1–{{ filteredRows.length }} dari {{ currentTable?.rows.length }} data</span><div><button disabled>‹</button><button class="current-page">1</button></div></div></article></section>

        <section v-if="page === 'users'" class="panel role-matrix-panel"><div class="panel-heading"><div><h2>Role &amp; Access Control</h2><p>Role tetap MVP · matrix baca-saja · bukan role builder dinamis.</p></div><span class="badge neutral">RBAC</span></div><div class="role-cards"><article v-for="role in roleCards" :key="role.name" class="role-card"><span :class="['role-symbol', role.tone]">{{ role.name.slice(0, 1) }}</span><div><b>{{ role.name }}</b><small>{{ role.description }}</small></div></article></div><div class="role-table-scroll"><table class="role-table"><thead><tr><th>Fitur</th><th>Admin</th><th>Operator</th><th>Checker</th><th>Auditor</th></tr></thead><tbody><tr v-for="row in roleMatrixRows" :key="row.feature"><td>{{ row.feature }}</td><td>{{ row.admin }}</td><td>{{ row.operator }}</td><td>{{ row.checker }}</td><td>{{ row.auditor }}</td></tr></tbody></table></div><div class="sod-note"><b>Segregation of duties</b><span>Checker tidak boleh menyetujui permintaan sendiri. Admin tidak mendapat bypass approval. Akun nonaktif kehilangan akses sesi.</span></div></section>

        <footer class="app-footer">ReconX <span>·</span> Bank Transaction Reconciliation &amp; Exception Management <span class="footer-version">v0.1 · Synthetic demo</span></footer>
      </div>
    </main>

    <div v-if="modal" class="modal-backdrop" @click.self="modal = ''"><section class="modal-card"><div class="modal-heading"><div><h2>{{ modal === 'batch' ? 'Buat Batch Rekonsiliasi' : modal === 'account' ? 'Tambah Sumber Data' : modal === 'proposal' ? 'Ajukan Manual Match' : 'Tambah Pengguna' }}</h2><p>{{ modal === 'batch' ? 'Tentukan rekening dan periode rekonsiliasi.' : modal === 'proposal' ? 'Permintaan ini akan ditinjau oleh Checker.' : 'Lengkapi informasi untuk melanjutkan.' }}</p></div><button class="close-button" @click="modal = ''">×</button></div><label v-if="modal === 'batch' || modal === 'account'" class="field-label">{{ modal === 'batch' ? 'Rekening Sumber' : 'Nama Sumber' }}<select class="form-control"><option>Core Banking</option><option>QRIS</option><option>Virtual Account</option><option>EDC</option></select></label><label v-if="modal === 'batch'" class="field-label">Periode Batch<div class="date-pair"><input class="form-control" type="date" value="2026-11-03" /><input class="form-control" type="date" value="2026-11-03" /></div></label><label v-if="modal === 'user'" class="field-label">Nama Lengkap<input class="form-control" placeholder="Nama pengguna" /></label><label v-if="modal === 'user'" class="field-label">Email<input class="form-control" type="email" placeholder="nama@perusahaan.co.id" /></label><label v-if="modal === 'user'" class="field-label">Role<select class="form-control"><option>Operator</option><option>Checker</option><option>Admin</option><option>Auditor</option></select></label><label v-if="modal === 'proposal'" class="field-label">Alasan Pengajuan<textarea class="form-control" rows="4" placeholder="Jelaskan alasan manual match (minimum 20 karakter)."></textarea></label><div class="modal-actions"><button class="button secondary" @click="modal = ''">Batal</button><button class="button primary" @click="submitModal">{{ modal === 'proposal' ? 'Kirim untuk Approval' : 'Simpan' }}</button></div></section></div>
    <div v-if="toast" class="toast-message"><span>✓</span>{{ toast }}</div>
  </div>
</template>
