<script setup lang="ts">
interface Column {
  key: string
  label: string
}

type Row = Record<string, string>

defineProps<{
  columns: Column[]
  rows: Row[]
  linkKey?: string
}>()

const emit = defineEmits<{ open: [] }>()

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
          <th v-for="column in columns" :key="column.key">
            {{ column.label }} <span class="sort-mark">↕</span>
          </th>
          <th class="action-head">Aksi</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, index) in rows" :key="index">
          <td v-for="column in columns" :key="column.key">
            <button v-if="linkKey && column.key === linkKey" class="table-link" @click="emit('open')">{{ row[column.key] }}</button>
            <span v-else-if="column.key === 'status'" :class="badgeFor(row[column.key] ?? '')">{{ row[column.key] }}</span>
            <span v-else>{{ row[column.key] }}</span>
          </td>
          <td class="row-action"><button aria-label="Lihat detail" @click="emit('open')">↗</button></td>
        </tr>
        <tr v-if="!rows.length">
          <td :colspan="columns.length + 1" class="empty-row">Tidak ada data yang sesuai dengan filter.</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
