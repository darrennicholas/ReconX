<script setup lang="ts">
import { computed } from 'vue'
import DataTable from '../../components/DataTable.vue'
import GlobalDropdown from '../../components/GlobalDropdown.vue'
type Row = Record<string, string>
const props = defineProps<{ accounts: Record<string, any>[]; statusOptions: string[]; summary: Record<string, any>; loading: boolean; forbidden: boolean; error: string; rows: Row[]; columns: { key: string; label: string }[]; currentPage: number; pageCount: number; firstVisibleRow: number; lastVisibleRow: number; paginationTotal: number }>()
const emit = defineEmits<{ download: []; reload: []; open: [row: Row]; page: [page: number] }>()
const from = defineModel<string>('from', { required: true })
const to = defineModel<string>('to', { required: true })
const accountId = defineModel<string>('accountId', { required: true })
const query = defineModel<string>('query', { required: true })
const status = defineModel<string>('status', { required: true })
const exportType = defineModel<string>('exportType', { required: true })
const reportStatusChart = computed(() => {
  const palette = ['#0d9488', '#488ec1', '#e5a23d', '#8e79c8', '#70b8a5', '#e5906a', '#97aab8', '#b96f78']
  const items = ((props.summary.batch_statuses ?? []) as {name: string; count: number}[]).filter(item => item.count > 0)
  const total = items.reduce((sum, item) => sum + item.count, 0)
  let offset = 0
  const segments = items.map((item, index) => { const start = offset; offset += item.count / total * 100; return {...item, color: palette[index % palette.length], start, end: offset} })
  return {total, segments, background: `conic-gradient(${segments.map(item => `${item.color} ${item.start}% ${item.end}%`).join(', ')})`}
})
const reportExceptionChart = computed(() => {
  const items = (props.summary.exception_reasons ?? []) as {name: string; count: number}[]
  const max = Math.max(1, ...items.map(item => item.count))
  return items.map(item => ({...item, width: `${item.count / max * 100}%`}))
})
</script>

<template>
        <section class="reports-page">
          <article class="panel report-filters"><div class="filter-block"><label>Mulai</label><input v-model="from" class="form-control" type="date" /></div><div class="filter-block"><label>Sampai</label><input v-model="to" class="form-control" type="date" /></div><div class="filter-block"><label>Rekening</label><GlobalDropdown v-model="accountId" class="form-control"><option value="">Semua rekening</option><option v-for="account in accounts" :key="account.id" :value="account.id">{{ account.account_code }} · {{ account.bank_display_name }}</option></GlobalDropdown></div><div class="filter-block"><label>Cari batch</label><input v-model="query" class="form-control" placeholder="No. batch / rekening" /></div><div class="filter-block"><label>Status batch</label><GlobalDropdown v-model="status" class="form-control"><option value="">Semua status</option><option v-for="status in statusOptions" :key="status" :value="status">{{ status }}</option></GlobalDropdown></div><div class="report-export"><label>Jenis laporan</label><div><GlobalDropdown v-model="exportType" aria-label="Jenis laporan"><option value="batches">Ringkasan batch</option><option value="transactions">Transaksi</option><option value="exceptions">Riwayat exception</option></GlobalDropdown><button class="button secondary" data-loading @click="emit('download')">Unduh CSV</button></div></div></article>
          <div class="report-summary-grid"><article v-for="card in [{label: 'Transaksi Internal', value: summary.internal_count ?? 0, icon: '↗', tone: 'teal'}, {label: 'Transaksi Bank', value: summary.bank_count ?? 0, icon: '↙', tone: 'blue'}, {label: 'Matched Pairs', value: summary.matched_pairs ?? 0, icon: '✓', tone: 'teal'}, {label: 'Belum Cocok', value: summary.unmatched_internal ?? 0, icon: '≠', tone: 'amber'}, {label: 'Exception Aktif', value: summary.active_exceptions ?? 0, icon: '!', tone: 'orange'}, {label: 'Match Rate', value: `${Number(summary.match_rate ?? 0).toFixed(1)}%`, icon: '%', tone: 'violet'}, {label: 'Rata-rata Matching', value: `${Number(summary.average_match_seconds ?? 0).toFixed(1)} dtk`, icon: '◷', tone: 'slate'}]" :key="card.label" class="report-stat" :class="`tone-${card.tone}`"><div><span>{{ card.label }}</span><strong>{{ card.value }}</strong></div><span class="report-stat-icon">{{ card.icon }}</span></article></div>
          <div class="report-chart-grid"><article class="panel report-chart"><div class="panel-heading"><div><h2>Status Batch</h2><p>Distribusi batch sesuai filter</p></div></div><div v-if="reportStatusChart.total" class="report-donut-layout"><div class="report-donut" :style="{ background: reportStatusChart.background }"><div><strong>{{ reportStatusChart.total }}</strong><span>Batch</span></div></div><div class="report-legend"><div v-for="item in reportStatusChart.segments" :key="item.name"><span><i :style="{background:item.color}"></i>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div></div></div><p v-else class="report-empty">Belum ada batch pada filter ini.</p></article><article class="panel report-chart"><div class="panel-heading"><div><h2>Exception Aktif</h2><p>Jenis selisih pada batch terpilih</p></div></div><div v-if="reportExceptionChart.length" class="report-bars"><div v-for="item in reportExceptionChart" :key="item.name"><div><span>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div><div class="report-bar-track"><i :style="{width:item.width}"></i></div></div></div><p v-else class="report-empty">Tidak ada exception aktif.</p></article></div>
          <article class="panel table-panel"><div class="panel-heading"><div><h2>Batch dalam laporan</h2><p>{{ summary.total_batches ?? 0 }} batch sesuai filter; ekspor memakai filter yang sama.</p></div></div><p v-if="loading" class="table-state">Memuat laporan…</p><p v-else-if="forbidden" class="table-state error-state">Akses laporan tidak diizinkan.</p><p v-else-if="error" class="table-state error-state">{{ error }} <button class="button secondary" @click="emit('reload')">Coba lagi</button></p><DataTable v-else :columns="columns" :rows="rows" @open="emit('open', $event)" /><div class="table-footer"><span>Menampilkan {{ firstVisibleRow }}–{{ lastVisibleRow }} dari {{ paginationTotal }} batch</span><div><button :disabled="currentPage <= 1" @click="emit('page', currentPage - 1)">‹</button><button class="current-page">{{ currentPage }} / {{ pageCount }}</button><button :disabled="currentPage >= pageCount" @click="emit('page', currentPage + 1)">›</button></div></div></article>
        </section>

</template>
