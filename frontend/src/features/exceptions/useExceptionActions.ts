import type { Ref } from 'vue'
import { apiRequest, downloadFile } from '../../api'
import type { SessionUser } from '../../stores/session'

interface ExceptionContext {
  id: Ref<string>; assignmentTarget: Ref<string>; currentUser: Ref<SessionUser | null>; note: Ref<string>
  evidenceFile: Ref<File | null>; detailPopup: Ref<'batch' | 'exception' | ''>
  loadWorkspace: () => Promise<void>; loadDetail: () => Promise<void>
  showSuccess: (message: string) => void; showToast: (message: string) => void
}

export function useExceptionActions(context: ExceptionContext) {
  async function assignException() {
    if (!context.id.value) return
    const assigneeId = context.assignmentTarget.value || context.currentUser.value?.id || ''
    if (!assigneeId) { context.showToast('Pilih Operator terlebih dahulu.'); return }
    try {
      await apiRequest(`/api/v1/exceptions/${encodeURIComponent(context.id.value)}/actions`, { method: 'POST', body: JSON.stringify({ type: 'ASSIGN', assignee_id: assigneeId, note: '' }) })
      await context.loadWorkspace(); await context.loadDetail()
      context.showSuccess(assigneeId === context.currentUser.value?.id ? 'Exception ditugaskan ke Anda.' : 'Exception berhasil ditugaskan.')
    } catch (error) { context.showToast(error instanceof Error ? error.message : 'Aksi gagal.') }
  }
  async function addExceptionNote() {
    if (!context.id.value || !context.note.value.trim()) return
    try {
      await apiRequest(`/api/v1/exceptions/${encodeURIComponent(context.id.value)}/actions`, { method: 'POST', body: JSON.stringify({ type: 'NOTE', note: context.note.value.trim() }) })
      context.note.value = ''; await context.loadWorkspace(); await context.loadDetail(); context.showSuccess('Catatan investigasi disimpan.')
    } catch (error) { context.showToast(error instanceof Error ? error.message : 'Catatan gagal disimpan.') }
  }
  function onEvidenceFile(event: Event) { context.evidenceFile.value = (event.target as HTMLInputElement).files?.[0] ?? null }
  function chooseEvidenceFile() {
    const scope = context.detailPopup.value === 'exception' ? document.querySelector('.detail-popup') : document
    scope?.querySelector<HTMLInputElement>('#exception-evidence-input')?.click()
  }
  async function uploadExceptionEvidence() {
    const file = context.evidenceFile.value
    if (!context.id.value || !file) { context.showToast('Pilih file evidence terlebih dahulu.'); return }
    if (file.size > 5 * 1024 * 1024) { context.showToast('Ukuran evidence maksimal 5 MB.'); return }
    const form = new FormData(); form.set('file', file)
    try {
      await apiRequest(`/api/v1/exceptions/${encodeURIComponent(context.id.value)}/evidence`, { method: 'POST', idempotent: true, body: form })
      context.evidenceFile.value = null; await context.loadWorkspace(); await context.loadDetail()
      context.showSuccess('Evidence tersimpan di penyimpanan private.')
    } catch (error) { context.showToast(error instanceof Error ? error.message : 'Evidence gagal diunggah.') }
  }
  async function downloadExceptionEvidence(action: Record<string, any>) {
    try { await downloadFile(`/api/v1/exceptions/${encodeURIComponent(context.id.value)}/evidence/${encodeURIComponent(action.evidence_id)}`, action.file_name || 'evidence') }
    catch (error) { context.showToast(error instanceof Error ? error.message : 'Evidence tidak dapat diunduh.') }
  }
  return { assignException, addExceptionNote, onEvidenceFile, chooseEvidenceFile, uploadExceptionEvidence, downloadExceptionEvidence }
}
