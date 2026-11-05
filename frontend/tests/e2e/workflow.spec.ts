import { expect, test, type Page } from '@playwright/test'

async function login(page: Page, role: 'operator' | 'checker') {
  await page.goto('/login')
  await page.locator('input[type="email"]').fill(`${role}@reconx.local`)
  await page.locator('input[type="password"]').fill('developer')
  await page.getByRole('button', { name: 'Masuk', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Dashboard', exact: true })).toBeVisible()
}

async function capture(page: Page, name: string) {
  if (!process.env.RECONX_CAPTURE_DOCS) return
  await expect(page.getByRole('status', { name: 'Sedang memproses' })).toBeHidden()
  await page.screenshot({ path: `../docs/images/${name}.png`, animations: 'disabled' })
}

test('operator reviews ambiguous transactions in the exception modal', async ({ page }) => {
  await login(page, 'operator')
  await capture(page, 'dashboard')
  await page.getByRole('navigation', { name: 'Navigasi utama' }).getByRole('button', { name: 'Exceptions', exact: true }).click()
  const row = page.locator('tbody tr').filter({ hasText: 'AMBIGUOUS_MATCH' }).first()
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: 'Lihat detail' }).click()
  const dialog = page.getByRole('dialog', { name: 'Detail exception' })
  await expect(dialog.getByText('Transaksi dengan reference yang sama')).toBeVisible()
  await expect(dialog.locator('.candidate-table tbody tr')).toHaveCount(3)
  await capture(page, 'exception')
  await dialog.getByRole('button', { name: 'Tugaskan', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Berhasil' })).toBeVisible()
  await page.getByRole('button', { name: 'OK', exact: true }).click()
  await dialog.getByRole('button', { name: 'Lihat batch' }).click()
  const batchDialog = page.getByRole('dialog', { name: 'Detail batch' })
  await expect(batchDialog.getByText('Transaksi Internal')).toBeVisible()
  await batchDialog.getByRole('button', { name: 'Tutup', exact: true }).click()
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: 'Tutup', exact: true }).click()
  await expect(page).toHaveURL(/\/exceptions$/)
  await page.getByRole('navigation', { name: 'Navigasi utama' }).getByRole('button', { name: 'Reports', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Status Batch' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Unduh CSV' })).toBeVisible()
})

test('checker reviews and approves a manual match', async ({ page }) => {
  await login(page, 'checker')
  await page.getByRole('navigation', { name: 'Navigasi utama' }).getByRole('button', { name: 'Approval Queue', exact: true }).click()
  const row = page.locator('tbody tr').filter({ hasText: 'MANUAL_MATCH' }).filter({ hasText: 'PENDING' }).first()
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: 'Tinjau approval' }).click()
  await expect(page.getByRole('heading', { name: 'Keputusan Approval' })).toBeVisible()
  await expect(page.getByText('Sumber internal:')).toBeVisible()
  await capture(page, 'approval')
  await page.getByRole('button', { name: 'Setujui', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Berhasil' })).toBeVisible()
  await page.getByRole('button', { name: 'OK', exact: true }).click()
})

test('operator receives a clear warning when a CSV import fails validation', async ({ page }) => {
  await login(page, 'operator')
  await page.getByRole('navigation', { name: 'Navigasi utama' }).getByRole('button', { name: 'Import Data', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Format file untuk import' })).toBeVisible()
  await page.locator('.import-layout [role="combobox"]').nth(1).click()
  await page.locator('[role="option"]:not([disabled])').first().click()
  await page.locator('#file-input').setInputFiles({ name: 'invalid.csv', mimeType: 'text/csv', buffer: Buffer.from('reference_id,date,direction,amount,currency\nINVALID,bukan-tanggal,DEBIT,100,IDR\n') })
  await page.getByRole('button', { name: /Validasi & Import/ }).click()
  const alert = page.getByRole('alertdialog')
  await expect(alert.getByText('Import belum berhasil')).toBeVisible()
  await expect(alert.getByText(/tanggal/i).first()).toBeVisible()
  await alert.getByRole('button', { name: 'Mengerti' }).click()
})

test('operator can run matching for a ready batch', async ({ page }) => {
  await login(page, 'operator')
  await page.getByRole('navigation', { name: 'Navigasi utama' }).getByRole('button', { name: 'Matching', exact: true }).click()
  const row = page.locator('.matching-batch-panel tbody tr').filter({ hasText: 'RX-SEED-READY' })
  await expect(row).toBeVisible()
  await row.getByRole('button', { name: /Jalankan matching batch/ }).click()
  await expect(page.getByRole('heading', { name: 'Berhasil' })).toBeVisible({ timeout: 20_000 })
  await expect(page.getByText(/Matching selesai:/)).toBeVisible()
  await page.getByRole('button', { name: 'OK', exact: true }).click()
})
