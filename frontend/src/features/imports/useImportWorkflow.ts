import type { Ref } from 'vue'
import { ApiError } from '../../api'
import { uploadTransactionFile } from './importApi'

interface ImportRowError { line: number; field: string; message: string }
interface ImportContext {
  selectedBatchId: Ref<string>
  source: Ref<string>
  selectedFile: Ref<string>
  file: Ref<File | null>
  stage: Ref<number>
  busy: Ref<boolean>
  error: Ref<string>
  errors: Ref<ImportRowError[]>
  showError: Ref<boolean>
  loadImports: () => Promise<void>
  loadWorkspace: () => Promise<void>
  showSuccess: (message: string) => void
}

export function useImportWorkflow(context: ImportContext) {
  function onFile(event: Event) {
    const file = (event.target as HTMLInputElement).files?.[0]
    if (file) {
      context.error.value = ''; context.errors.value = []
      context.file.value = file; context.selectedFile.value = file.name; context.stage.value = 3
    }
  }
  function clearImportFile() {
    context.error.value = ''; context.errors.value = []; context.file.value = null; context.selectedFile.value = ''
    context.stage.value = context.selectedBatchId.value ? 2 : 1
    const input = document.getElementById('file-input') as HTMLInputElement | null
    if (input) input.value = ''
  }
  function changeImportBatch() {
    context.stage.value = context.selectedBatchId.value ? (context.file.value ? 3 : 2) : 1
    void context.loadImports()
  }
  async function uploadImport() {
    const file = context.file.value
    if (!file || !context.selectedBatchId.value) { context.error.value = 'Pilih batch dan file CSV terlebih dahulu.'; context.showError.value = true; return }
    context.error.value = ''; context.errors.value = []
    if (!file.name.toLowerCase().endsWith('.csv')) { context.error.value = 'Format file tidak didukung. Simpan file Excel sebagai CSV UTF-8 terlebih dahulu.'; context.showError.value = true; return }
    if (file.size > 10 * 1024 * 1024) { context.error.value = 'Ukuran file melebihi batas 10 MB.'; context.showError.value = true; return }
    context.stage.value = 3; context.busy.value = true
    try {
      await uploadTransactionFile(context.selectedBatchId.value, context.source.value, file)
      clearImportFile(); context.stage.value = 4
      await context.loadWorkspace(); await context.loadImports()
      context.showSuccess('CSV tervalidasi dan diimpor.')
    } catch (error) {
      const data = error instanceof ApiError ? error.data as { errors?: ImportRowError[] } | undefined : undefined
      context.errors.value = data?.errors ?? []
      context.error.value = context.errors.value.length ? 'Data dalam file belum lolos validasi. Periksa rincian berikut.' : error instanceof Error ? error.message : 'Import gagal. Periksa format file dan coba lagi.'
      context.stage.value = 2
      await context.loadImports()
      context.showError.value = true
    } finally { context.busy.value = false }
  }
  return { onFile, clearImportFile, changeImportBatch, uploadImport }
}

export function importValidationMessage(message: string) {
  const translations: Record<string, string> = {
    'Reference cannot be empty': 'Reference tidak boleh kosong.',
    'Date must use YYYY-MM-DD': 'Tanggal harus menggunakan format YYYY-MM-DD.',
    'Date falls outside the batch period': 'Tanggal berada di luar periode batch.',
    'Direction must be CREDIT or DEBIT': 'Arah transaksi harus CREDIT atau DEBIT.',
    'Amount must be a positive integer in rupiah': 'Nominal harus berupa bilangan bulat positif dalam rupiah.',
    'Currency must be IDR': 'Mata uang harus IDR.',
    'Column count does not match header': 'Jumlah kolom tidak sesuai dengan header.',
    'CSV row format is invalid': 'Format baris CSV tidak valid.',
    'CSV must contain at least one transaction': 'File harus berisi minimal satu transaksi.',
  }
  return translations[message] ?? message
}
