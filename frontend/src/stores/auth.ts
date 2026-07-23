// Store de autenticación. El access_token vive solo aquí (memoria).
// El refresh_token vive en sessionStorage (ver services/api.ts).

import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  api,
  clearTokens,
  getRefreshToken,
  setAccessToken,
  setRefreshToken,
} from '@/services/api'
import { defaultLocale, i18n, setLocale } from '@/i18n'
import type { LocaleCode, PublicUsuario, Role } from '@/services/types'

// Matriz de permisos espejo del backend (autoridad final = backend).
type Action =
  | 'usuarios.manage'
  | 'catalogos.manage'
  | 'alertas.manage'
  | 'equipos.write'
  | 'equipos.read'
  | 'nodos.read'
  | 'nodos.write'
  | 'modelos3d.read'
  | 'modelos3d.write'
  | 'modelos3d.delete'

const PERMS: Record<Role, ReadonlyArray<Action>> = {
  ADMINISTRADOR: [
    'usuarios.manage',
    'catalogos.manage',
    'alertas.manage',
    'equipos.write',
    'equipos.read',
    'nodos.read',
    'nodos.write',
    'modelos3d.read',
    'modelos3d.write',
    'modelos3d.delete',
  ],
  OPERADOR: ['equipos.write', 'equipos.read', 'nodos.read', 'modelos3d.read', 'modelos3d.write'],
  CONSULTA: ['equipos.read', 'nodos.read', 'modelos3d.read'],
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<PublicUsuario | null>(null)
  const loading = ref(false)
  const lastError = ref<string | null>(null)

  const isAuthenticated = computed(() => user.value !== null)
  const role = computed<Role | null>(() => user.value?.rol ?? null)

  function tr(key: string): string {
    return i18n.global.t(key)
  }

  function applyUser(next: PublicUsuario | null): void {
    user.value = next
    setLocale(next?.idioma_preferido ?? defaultLocale)
  }

  function can(action: Action): boolean {
    const r = role.value
    if (!r) return false
    return PERMS[r].includes(action)
  }

  function hasRole(...roles: Role[]): boolean {
    return role.value !== null && roles.includes(role.value)
  }

  async function login(correo: string, password: string): Promise<void> {
    loading.value = true
    lastError.value = null
    try {
      const r = await api.auth.login(correo, password)
      setAccessToken(r.access_token)
      setRefreshToken(r.refresh_token)
      applyUser(r.user)
    } catch (e) {
      lastError.value = (e as { message?: string }).message ?? tr('auth.login.error')
      throw e
    } finally {
      loading.value = false
    }
  }

  async function bootstrap(): Promise<void> {
    // Si tenemos refresh en sessionStorage, intentamos restaurar la sesión
    // pidiendo /me. Si el access falta, el interceptor hará refresh.
    if (!getRefreshToken()) return
    try {
      applyUser(await api.auth.me())
    } catch {
      clearTokens()
      applyUser(null)
    }
  }

  async function logout(): Promise<void> {
    try {
      await api.auth.logout()
    } catch {
      // Aunque el backend falle, limpiamos local.
    }
    clearTokens()
    applyUser(null)
  }

  function forceLogout(): void {
    clearTokens()
    applyUser(null)
  }

  async function refreshMe(): Promise<void> {
    applyUser(await api.auth.me())
  }

  async function setLanguage(idioma: LocaleCode): Promise<void> {
    applyUser(await api.auth.updatePreferences(idioma))
  }

  return {
    user,
    loading,
    lastError,
    isAuthenticated,
    role,
    can,
    hasRole,
    login,
    logout,
    bootstrap,
    forceLogout,
    refreshMe,
    setLanguage,
  }
})
