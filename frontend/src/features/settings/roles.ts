export const roleMatrixRows = [
  { feature: 'User & role', admin: 'CRUD', operator: '—', checker: '—', auditor: '—' },
  { feature: 'Bank account', admin: 'CRUD', operator: 'View', checker: 'View', auditor: 'View' },
  { feature: 'Dashboard', admin: 'View', operator: 'View', checker: 'View', auditor: 'View' },
  { feature: 'Buat/edit draft batch', admin: '—', operator: 'Own', checker: 'View', auditor: 'View' },
  { feature: 'Import dan run matching', admin: '—', operator: 'Own', checker: 'View', auditor: 'View' },
  { feature: 'Hasil & transaksi', admin: 'View', operator: 'Scoped', checker: 'View', auditor: 'View' },
  { feature: 'Investigasi exception', admin: '—', operator: 'Assigned/own', checker: 'View', auditor: 'View' },
  { feature: 'Propose manual match', admin: '—', operator: 'Yes', checker: '—', auditor: '—' },
  { feature: 'Approve/reject match', admin: '—', operator: '—', checker: 'Yes, not own', auditor: '—' },
  { feature: 'Submit batch approval', admin: '—', operator: 'Maker', checker: '—', auditor: '—' },
  { feature: 'Approve/reject/close batch', admin: '—', operator: '—', checker: 'Yes, not maker', auditor: '—' },
  { feature: 'Reports/export', admin: 'View', operator: 'View', checker: 'View', auditor: 'View' },
  { feature: 'Audit logs', admin: 'View', operator: '—', checker: 'View', auditor: 'View' },
  { feature: 'System settings', admin: 'CRUD', operator: '—', checker: 'View', auditor: 'View' },
]
export const roleCards = [
  { name: 'Admin', description: 'Kelola pengguna, rekening referensi, dan konfigurasi sistem.', tone: 'purple' },
  { name: 'Operator', description: 'Buat batch, impor data, jalankan matching, dan investigasi.', tone: 'teal' },
  { name: 'Checker', description: 'Periksa hasil dan setujui secara independen dari maker.', tone: 'blue' },
  { name: 'Auditor', description: 'Akses read-only ke batch, transaksi, laporan, approval, dan log.', tone: 'amber' },
]
