import type { Ref } from 'vue'
import { apiRequest } from '../../api'
import { fetchBatch } from './batchApi'

interface BatchContext {
  selectedBatchId: Ref<string>
  busy: Ref<boolean>
  loadWorkspace: () => Promise<void>
  showToast: (message: string) => void
  showSuccess: (message: string) => void
}

export function useBatchWorkflow(context: BatchContext) {
  async function perform(action: 'submit' | 'close' | 'cancel' | 'reopen' | 'retry', success: string, failure: string) {
    const id = context.selectedBatchId.value
    if (!id) { context.showToast('Pilih batch terlebih dahulu.'); return }
    context.busy.value = true
    try {
      const { data: batch } = await fetchBatch(id)
      await apiRequest(`/api/v1/batches/${encodeURIComponent(id)}/${action}`, {
        method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}',
      })
      await context.loadWorkspace(); context.showSuccess(success)
    } catch (error) { context.showToast(error instanceof Error ? error.message : failure) }
    finally { context.busy.value = false }
  }
  function submitBatch() { return perform('submit', 'Batch dikirim ke antrean approval.', 'Batch belum dapat diajukan.') }
  function closeBatch() {
    if (!context.selectedBatchId.value || !window.confirm('Tutup batch yang sudah disetujui? Hasilnya akan dikunci dan tidak dapat diubah.')) return
    return perform('close', 'Batch berhasil ditutup dan dikunci.', 'Batch belum dapat ditutup.')
  }
  function transitionBatch(action: 'cancel' | 'reopen' | 'retry') {
    if (!context.selectedBatchId.value) return
    const message = action === 'cancel' ? 'Batch dibatalkan.' : action === 'reopen' ? 'Batch dibuka untuk koreksi dan import ulang.' : 'Batch siap dicoba kembali.'
    return perform(action, message, 'Perubahan state batch gagal.')
  }
  return { submitBatch, closeBatch, transitionBatch }
}
