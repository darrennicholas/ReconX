<script setup lang="ts">
import { computed } from 'vue'
const props = defineProps<{ kind: 'transaction' | 'audit'; selectedTransaction: Record<string, any> | null; auditDetail: Record<string, any> | null; transactionBatchLabel: string }>()
function formatDate(value?: string) { return value ? new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeZone: 'Asia/Jakarta' }).format(new Date(value)) : '—' }
function money(value?: number | null) { return value == null ? '—' : new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(value) }
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
const auditBefore = computed(() => auditFields(props.auditDetail?.before))
const auditAfter = computed(() => auditFields(props.auditDetail?.after))

</script>

<template>
<template v-if="kind === 'transaction'"><div class="friendly-detail"><div class="friendly-highlight"><span>Nomor referensi</span><strong>{{ selectedTransaction?.reference_id || '—' }}</strong><small>{{ humanLabel(selectedTransaction?.status) }}</small></div><div class="friendly-grid"><div><span>Tanggal transaksi</span><strong>{{ formatDate(selectedTransaction?.date) }}</strong></div><div><span>Jenis transaksi</span><strong>{{ humanLabel(selectedTransaction?.direction) }}</strong></div><div><span>Nominal</span><strong>{{ money(selectedTransaction?.amount_rupiah) }}</strong></div><div><span>Sumber data</span><strong>{{ humanLabel(selectedTransaction?.source_type) }}</strong></div><div><span>Batch rekonsiliasi</span><strong>{{ transactionBatchLabel || 'Tidak tersedia' }}</strong></div><div><span>Mata uang</span><strong>{{ selectedTransaction?.currency || '—' }}</strong></div></div></div></template><template v-else><div class="friendly-detail"><div class="friendly-highlight"><span>Aktivitas</span><strong>{{ humanLabel(auditDetail?.action) }}</strong><small>{{ humanLabel(auditDetail?.entity) }}</small></div><div class="friendly-grid"><div><span>Waktu</span><strong>{{ formatDateTime(auditDetail?.created_at) }}</strong></div><div><span>Pengguna</span><strong>{{ auditDetail?.actor || 'Sistem' }}</strong></div></div><div class="audit-change-grid"><section><h3>Sebelum</h3><div v-if="auditBefore.length" class="audit-change-list"><div v-for="field in auditBefore" :key="field.label"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div></div><p v-else>Tidak ada data sebelumnya.</p></section><section><h3>Sesudah</h3><div v-if="auditAfter.length" class="audit-change-list"><div v-for="field in auditAfter" :key="field.label"><span>{{ field.label }}</span><strong>{{ field.value }}</strong></div></div><p v-else>Tidak ada perubahan yang ditampilkan.</p></section></div></div></template>
</template>
