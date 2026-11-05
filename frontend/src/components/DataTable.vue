<script setup lang="ts">
import { computed, ref } from 'vue'

interface Column {
  key: string
  label: string
}

type Row = Record<string, string>

const props = defineProps<{
  columns: Column[]
  rows: Row[]
  linkKey?: string
  hideActions?: boolean
  actionIcon?: 'view' | 'edit' | 'review' | 'history'
  actionLabel?: string
}>()

const emit = defineEmits<{ open: [row: Row] }>()
const sortKey = ref('')
const descending = ref(false)
const sortedRows = computed(() => {
  if (!sortKey.value) return props.rows
  return [...props.rows].sort((left, right) => {
    const comparison = (left[sortKey.value] ?? '').localeCompare(right[sortKey.value] ?? '', 'id', { numeric: true, sensitivity: 'base' })
    return descending.value ? -comparison : comparison
  })
})
function toggleSort(key: string) {
  if (sortKey.value === key) descending.value = !descending.value
  else { sortKey.value = key; descending.value = false }
}

function badgeFor(value: string) {
  const status = value.toLowerCase()
  if (status.includes('completed') || (status.includes('matched') && !status.includes('unmatched')) || status.includes('aktif') || status.includes('success') || status.includes('disetujui')) return 'badge success'
  if (status.includes('progress') || status.includes('review') || status.includes('menunggu') || status.includes('open')) return 'badge warning'
  if (status.includes('unmatched') || status.includes('only') || status.includes('mismatch') || status.includes('ditolak') || status.includes('failed')) return 'badge danger'
  return 'badge neutral'
}
</script>

<template>
  <div class="table-scroll">
    <table class="data-table">
      <thead>
        <tr>
          <th v-for="column in columns" :key="column.key" @click="toggleSort(column.key)">
            {{ column.label }}
          </th>
          <th v-if="!hideActions" class="action-head">Aksi</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, index) in sortedRows" :key="index">
          <td v-for="column in columns" :key="column.key">
            <button v-if="linkKey && column.key === linkKey" class="table-link" data-loading @click="emit('open', row)">{{ row[column.key] }}</button>
            <span v-else-if="column.key === 'status'" :class="badgeFor(row[column.key] ?? '')">{{ row[column.key] }}</span>
            <span v-else>{{ row[column.key] }}</span>
          </td>
          <td v-if="!hideActions" class="row-action"><slot name="actions" :row="row"><button type="button" data-loading :aria-label="actionLabel || 'Lihat detail'" :title="actionLabel || 'Lihat detail'" @click="emit('open', row)"><svg v-if="actionIcon === 'edit'" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4l11-11a2.8 2.8 0 0 0-4-4L4 16v4ZM13.5 6.5l4 4" /></svg><svg v-else-if="actionIcon === 'review'" viewBox="0 0 24 24" aria-hidden="true"><rect x="5" y="4" width="14" height="16" rx="2"/><path d="m8.5 12 2 2 5-5"/></svg><svg v-else-if="actionIcon === 'history'" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12a8 8 0 1 0 2-5.3L4 9m0-5v5h5m3-2v5l3 2"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z"/><circle cx="12" cy="12" r="2.5"/></svg></button></slot></td>
        </tr>
        <tr v-if="!rows.length">
          <td :colspan="columns.length + (hideActions ? 0 : 1)" class="empty-row">Tidak ada data yang sesuai dengan filter.</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
