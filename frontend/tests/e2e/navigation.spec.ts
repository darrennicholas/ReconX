import { expect, test, type Page } from '@playwright/test'

const menus: Record<string, { path: string; title: string }[]> = {
 admin: [
  { path: 'dashboard', title: 'Dashboard' }, { path: 'batches', title: 'Reconciliation Batches' },
  { path: 'transactions', title: 'Transactions' }, { path: 'matching', title: 'Matching Results' },
  { path: 'exceptions', title: 'Exceptions' }, { path: 'reports', title: 'Reports' },
  { path: 'accounts', title: 'Bank Accounts' }, { path: 'users', title: 'User Management' },
  { path: 'audit', title: 'Audit Logs' }, { path: 'settings', title: 'Settings' },
 ],
 operator: [{ path: 'imports', title: 'Import Data' }],
 checker: [{ path: 'approvals', title: 'Approval Queue' }],
 auditor: [{ path: 'approvals', title: 'Approval Queue' }, { path: 'audit', title: 'Audit Logs' }],
}

async function login(page: Page, role: string) {
 await page.goto('/login')
 await page.locator('input[type="email"]').fill(`${role}@reconx.local`)
 await page.locator('input[type="password"]').fill('developer')
 await page.getByRole('button', { name: 'Masuk', exact: true }).click()
 await expect(page.locator('h1')).toHaveText('Dashboard')
}

for (const [role, pages] of Object.entries(menus)) {
 test(`${role} can open permitted menus without horizontal page overflow`, async ({ page }) => {
  await login(page, role)
  for (const item of pages) {
   await page.goto(`/${item.path}`)
   await expect(page.locator('h1')).toHaveText(item.title)
   await expect(page.locator('.error-state')).toHaveCount(0)
   const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, viewport: document.documentElement.clientWidth }))
   expect(dimensions.scroll, item.path).toBeLessThanOrEqual(dimensions.viewport + 1)
  }
 })
}

test('operator cannot open admin user management', async ({ page }) => {
 await login(page, 'operator')
 await page.goto('/users')
 await expect(page).toHaveURL(/\/dashboard$/)
})

test('settings categories display their own content', async ({ page }) => {
 await login(page, 'admin')
 await page.goto('/settings')
 const categories = page.getByRole('navigation', { name: 'Kategori pengaturan' })
 await categories.getByRole('button', { name: /Parameter Matching/ }).click()
 await expect(page.getByText('Auto Exact Match')).toBeVisible()
 await categories.getByRole('button', { name: /Notifikasi/ }).click()
 await expect(page.getByText('Notifikasi dalam aplikasi')).toBeVisible()
 await expect(page.getByText('Auto Exact Match')).toBeHidden()
})

test('admin pages fit a narrow viewport', async ({ page }) => {
 await page.setViewportSize({ width: 390, height: 844 })
 await login(page, 'admin')
 for (const item of menus.admin!) {
  await page.goto(`/${item.path}`)
  await expect(page.locator('h1')).toHaveText(item.title)
  const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, viewport: document.documentElement.clientWidth }))
  expect(dimensions.scroll, item.path).toBeLessThanOrEqual(dimensions.viewport + 1)
 }
})
