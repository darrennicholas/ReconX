import type { Ref } from 'vue'
import { apiRequest } from '../../api'
import { fetchBatch } from '../batches/batchApi'

interface MatchingContext {
  selectedBatchId: Ref<string>
  busy: Ref<boolean>
  progress: Ref<string>
  loadWorkspace: () => Promise<void>
  loadMatchingBatchOptions: () => Promise<void>
  showToast: (message: string) => void
  showSuccess: (message: string) => void
}

export function useMatchingWorkflow(context: MatchingContext) {
  async function runMatching(batchId: string) {
    if (!batchId || context.busy.value) return
    context.selectedBatchId.value = batchId
    context.busy.value = true
    try {
      const { data: batch } = await fetchBatch(batchId)
      const { data: started } = await apiRequest<Record<string, any>>(`/api/v1/batches/${encodeURIComponent(batchId)}/match-runs`, {
        method: 'POST', idempotent: true, headers: { 'If-Match': `"${batch.row_version}"` }, body: '{}',
      })
      context.progress.value = 'PROCESSING'
      const deadline = Date.now() + 30 * 60 * 1000
      let completed: Record<string, any> | null = null
      while (Date.now() < deadline) {
        await new Promise(resolve => window.setTimeout(resolve, 750))
        try {
          const { data } = await apiRequest<Record<string, any>>(`/api/v1/batches/${encodeURIComponent(batchId)}/match-runs/latest`)
          if (data.id === started.id) {
            context.progress.value = data.state
            if (data.state === 'SUCCEEDED' || data.state === 'FAILED') { completed = data; break }
          }
        } catch { /* Keep polling through temporary request failures. */ }
      }
      await context.loadWorkspace(); await context.loadMatchingBatchOptions()
      context.selectedBatchId.value = batchId
      if (!completed) context.showToast('Matching masih berjalan. Periksa kembali hasil batch sebentar lagi.')
      else if (completed.state === 'SUCCEEDED') context.showSuccess(`Matching selesai: ${completed.matched_pairs} pasangan cocok, ${completed.exceptions} exception.`)
      else context.showToast('Matching gagal. Periksa status batch sebelum mencoba lagi.')
    } catch (error) { context.showToast(error instanceof Error ? error.message : 'Matching gagal.') }
    finally { context.progress.value = ''; context.busy.value = false }
  }
  return { runMatching }
}
