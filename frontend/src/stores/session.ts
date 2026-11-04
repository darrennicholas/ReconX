import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiRequest } from '../api'

export interface SessionUser {
  id: string
  full_name: string
  email: string
  role: 'ADMIN' | 'OPERATOR' | 'CHECKER' | 'AUDITOR'
}

export const useSessionStore = defineStore('session', () => {
  const currentUser = ref<SessionUser | null>(null)
  const initialized = ref(false)
  let loading: Promise<void> | null = null

  async function load() {
    if (initialized.value) return
    if (!loading) {
      loading = (async () => {
        try {
          currentUser.value = (await apiRequest<SessionUser>('/api/v1/auth/me')).data
        } catch {
          currentUser.value = null
        } finally {
          initialized.value = true
          loading = null
        }
      })()
    }
    await loading
  }

  function setUser(user: SessionUser) {
    currentUser.value = user
    initialized.value = true
  }

  function clearUser() {
    currentUser.value = null
    initialized.value = true
  }

  return { currentUser, initialized, load, setUser, clearUser }
})
