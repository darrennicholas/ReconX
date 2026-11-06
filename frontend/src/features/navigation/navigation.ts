export type Page = 'dashboard' | 'batches' | 'batchDetail' | 'imports' | 'transactions' | 'matching' | 'exceptions' | 'exceptionDetail' | 'approvals' | 'reports' | 'accounts' | 'users' | 'audit' | 'settings'

export const navGroups: { title: string; items: { id: Page; label: string; icon: string; count?: string }[] }[] = [
  { title: 'OVERVIEW', items: [{ id: 'dashboard', label: 'Dashboard', icon: 'grid' }] },
  { title: 'OPERATIONS', items: [
    { id: 'batches', label: 'Reconciliation Batches', icon: 'layers', count: '1' },
    { id: 'imports', label: 'Import Data', icon: 'upload' },
    { id: 'transactions', label: 'Transactions', icon: 'rows' },
    { id: 'matching', label: 'Matching', icon: 'shuffle' },
    { id: 'exceptions', label: 'Exceptions', icon: 'alert', count: '3' },
  ] },
  { title: 'GOVERNANCE', items: [{ id: 'approvals', label: 'Approval Queue', icon: 'check', count: '0' }] },
  { title: 'ANALYTICS', items: [{ id: 'reports', label: 'Reports', icon: 'chart' }] },
  { title: 'ADMINISTRATION', items: [
    { id: 'accounts', label: 'Bank Accounts', icon: 'bank' },
    { id: 'users', label: 'Users & Access', icon: 'users' },
    { id: 'audit', label: 'Audit Logs', icon: 'history' },
    { id: 'settings', label: 'Settings', icon: 'settings' },
  ] },
]

export const titles: Record<Page, string> = {
  dashboard: 'Dashboard', batches: 'Reconciliation Batches', batchDetail: 'Batch Detail', imports: 'Import Data',
  transactions: 'Transactions', matching: 'Matching Results', exceptions: 'Exceptions', exceptionDetail: 'Exception Detail',
  approvals: 'Approval Queue', reports: 'Reports', accounts: 'Bank Accounts', users: 'User Management', audit: 'Audit Logs', settings: 'Settings',
}
export const subtitles: Record<Page, string> = {
  dashboard: 'Overview of your reconciliation activity and workspace health.',
  batches: 'Manage reconciliation batches across all connected sources.',
  batchDetail: 'Review the batch summary, source files, matching and approvals.',
  imports: 'Upload transaction files and review validation results.',
  transactions: 'Search and review imported transactions across batches.',
  matching: 'Review automatic matching results for a reconciliation batch.',
  exceptions: 'Investigate unmatched transactions and data discrepancies.',
  exceptionDetail: 'Compare source records and track the investigation history.',
  approvals: 'Review pending manual matches and batch close requests.',
  reports: 'Monitor reconciliation performance and export summaries.',
  accounts: 'Manage bank and source account reference data.',
  users: 'Manage workspace access and role assignments.',
  audit: 'Trace important actions performed in this workspace.',
  settings: 'Review system defaults and matching policies.',
}


export const routeRoles = {
  dashboard: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  batches: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  batchDetail: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  imports: ['OPERATOR'],
  transactions: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  matching: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  exceptions: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  exceptionDetail: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  approvals: ['CHECKER', 'AUDITOR'],
  reports: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  accounts: ['ADMIN', 'OPERATOR', 'CHECKER', 'AUDITOR'],
  users: ['ADMIN'],
  audit: ['ADMIN', 'CHECKER', 'AUDITOR'],
  settings: ['ADMIN', 'CHECKER', 'AUDITOR'],
} as const
