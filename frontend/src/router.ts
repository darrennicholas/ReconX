import { createRouter, createWebHistory } from 'vue-router'
import { useSessionStore } from './stores/session'

const roles = {
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

const routes = [
  { path: '/login', name: 'login', component: () => import('./Workspace.vue'), meta: { public: true } },
  ...Object.entries(roles).map(([name, allowedRoles]) => ({
    path: name === 'batchDetail' ? '/batch/:id?' : name === 'exceptionDetail' ? '/exception/:id?' : `/${name === 'dashboard' ? 'dashboard' : name}`,
    name,
    component: () => import('./Workspace.vue'),
    meta: { roles: allowedRoles },
  })),
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async to => {
  const session = useSessionStore()
  await session.load()
  if (to.name !== 'login' && !session.currentUser) return { name: 'login' }
  if (to.name === 'login' && session.currentUser) return { name: 'dashboard' }
  const allowedRoles = to.meta.roles as readonly string[] | undefined
  if (allowedRoles && session.currentUser && !allowedRoles.includes(session.currentUser.role)) return { name: 'dashboard' }
  return true
})
