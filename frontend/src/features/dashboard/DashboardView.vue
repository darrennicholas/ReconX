<script setup lang="ts">
import { computed } from 'vue'
import DataTable from '../../components/DataTable.vue'
type Row = Record<string, string>
const props = defineProps<{ summary: Record<string, any>; loading: boolean; forbidden: boolean; error: string; rows: Row[]; columns: {key: string; label: string}[] }>()
const emit = defineEmits<{ reload: []; viewBatches: []; open: [row: Row] }>()
const dashboardTrendPoints = computed(() => {
  const points = (props.summary.daily_trend ?? []) as { date: string; matched_count: number; unmatched_count: number }[]
  const visible = points.slice(-14)
  const maximum = Math.max(1, ...visible.map(point => point.matched_count + point.unmatched_count))
  return visible.map(point => ({
    ...point,
    label: new Intl.DateTimeFormat('id-ID', { day: '2-digit', month: 'short', timeZone: 'Asia/Jakarta' }).format(new Date(point.date)),
    matchedHeight: `${point.matched_count / maximum * 100}%`,
    unmatchedHeight: `${point.unmatched_count / maximum * 100}%`,
  }))
})
const dashboardStatusPoints = computed(() => props.summary.batch_statuses ?? [])
const dashboardExceptionPoints = computed(() => props.summary.exception_reasons ?? [])
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
</script>

<template>
        <section class="dashboard-page"><p v-if="loading" class="table-state">Memuat ringkasan…</p><p v-else-if="forbidden" class="table-state error-state">Akses dashboard tidak diizinkan.</p><p v-else-if="error" class="table-state error-state">{{ error }} <button class="button secondary" @click="emit('reload')">Coba lagi</button></p>
          <div class="kpi-grid">
            <article class="kpi-card"><div class="kpi-top"><span>Total Batch</span><span class="kpi-icon teal">▦</span></div><strong>{{ summary.total_batches ?? 0 }}</strong><small class="trend neutral-text">{{ summary.active_batches ?? 0 }} masih aktif</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Matched Pairs</span><span class="kpi-icon green">✓</span></div><strong>{{ summary.matched_pairs ?? 0 }}</strong><small class="trend positive">{{ Number(summary.match_rate ?? 0).toFixed(1) }}% dari transaksi internal</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Exceptions Aktif</span><span class="kpi-icon amber">⚑</span></div><strong>{{ summary.open_exceptions ?? 0 }}</strong><small class="trend warning-text">{{ summary.exceptions_older_24h ?? 0 }} lebih dari 24 jam</small></article>
            <article class="kpi-card"><div class="kpi-top"><span>Approval Menunggu</span><span class="kpi-icon blue">◷</span></div><strong>{{ summary.pending_approvals ?? 0 }}</strong><small class="trend neutral-text">Perlu tindakan Checker</small></article>
          </div>
          <div class="dashboard-insights">
            <article class="panel dashboard-chart-panel"><div class="panel-heading"><div><h2>Tren Rekonsiliasi</h2><p>Transaksi internal per tanggal pada filter terpilih</p></div></div><div v-if="trendChart.points.length" class="trend-line-chart"><div class="trend-chart-scale"><span>{{ trendChart.maximum }}</span><span>0</span></div><div class="trend-chart-main"><svg viewBox="0 0 1000 180" preserveAspectRatio="none" role="img" aria-label="Tren transaksi matched dan unmatched"><line v-for="y in [20, 90, 160]" :key="y" x1="30" x2="970" :y1="y" :y2="y" class="trend-grid-line"/><polyline v-if="trendChart.points.length > 1" :points="trendChart.matched" class="trend-line matched"/><polyline v-if="trendChart.points.length > 1" :points="trendChart.unmatched" class="trend-line unmatched"/><g v-for="point in trendChart.points" :key="point.date"><circle :cx="point.x" :cy="point.matchedY" r="5" class="trend-dot matched"><title>{{ point.label }}: {{ point.matched_count }} matched</title></circle><circle :cx="point.x" :cy="point.unmatchedY" r="5" class="trend-dot unmatched"><title>{{ point.label }}: {{ point.unmatched_count }} unmatched</title></circle></g></svg><div class="trend-axis"><span v-for="point in trendChart.ticks" :key="point!.date">{{ point!.label }}</span></div></div></div><p v-else class="chart-empty">Belum ada transaksi pada periode ini.</p><div class="chart-legend"><span><i class="trend-matched"></i>Matched internal</span><span><i class="trend-unmatched"></i>Unmatched internal</span></div></article>
            <article class="panel dashboard-distribution"><div class="panel-heading"><div><h2>Status Batch</h2><p>Jumlah batch sesuai filter</p></div></div><div v-if="statusChart.total" class="status-chart"><div class="status-donut" :style="{ background: statusChart.background }" role="img" :aria-label="`Total ${statusChart.total} batch berdasarkan status`"><div><strong>{{ statusChart.total }}</strong><span>Batch</span></div></div><div class="status-legend"><div v-for="item in statusChart.segments" :key="item.name" class="status-legend-row"><i :style="{ background: item.color }"></i><span>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div></div></div><p v-else class="chart-empty">Belum ada batch.</p></article>
            <article class="panel dashboard-distribution"><div class="panel-heading"><div><h2>Top Selisih</h2><p>Exception yang masih aktif</p></div></div><div v-if="exceptionChart.length" class="exception-chart"><div v-for="item in exceptionChart" :key="item.name" class="exception-chart-row"><div><span>{{ item.name.split('_').join(' ') }}</span><b>{{ item.count }}</b></div><div class="exception-chart-track"><i :style="{ width: item.width }"></i></div></div></div><p v-else class="chart-empty">Belum ada exception aktif.</p></article>
          </div><article class="panel table-panel recent-panel"><div class="panel-heading"><div><h2>Batch Rekonsiliasi</h2><p>Status dan ringkasan dari database workspace</p></div><button class="link-button" @click="emit('viewBatches')">Lihat semua batch <span>→</span></button></div><DataTable :columns="columns" :rows="rows.slice(0, 6)" @open="emit('open', $event)" /></article>
          <div class="bottom-grid"><article class="panel process-panel"><div class="panel-heading"><div><h2>Alur Proses</h2><p>Batch berjalan melalui state machine yang tervalidasi di server</p></div></div><div class="process-flow"><div v-for="(step, i) in [{icon:'▱',title:'Buat Batch',text:'Pilih rekening & periode'}, {icon:'⇧',title:'Import Data',text:'Unggah dua CSV'}, {icon:'⇄',title:'Matching',text:'Pencocokan exact'}, {icon:'⌕',title:'Investigasi',text:'Tinjau exception'}, {icon:'✓',title:'Approval',text:'Pemeriksaan Checker'}, {icon:'▥',title:'Tutup Batch',text:'Kunci hasil disetujui'}]" :key="step.title" class="process-step"><span class="process-icon">{{ step.icon }}</span><div><b>{{ i + 1 }}. {{ step.title }}</b><small>{{ step.text }}</small></div><span v-if="i < 5" class="process-arrow">→</span></div></div></article></div>
        </section>

</template>
