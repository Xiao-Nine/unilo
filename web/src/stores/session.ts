import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { apiBaseUrl, configureApi, normalizeServerUrl, setAccessToken } from '../api/client'
import * as authApi from '../api/auth'
import type { User } from '../api/types'

const STORAGE_KEY = 'unilo.session'
export const DEFAULT_DEV_SERVER_URL = 'http://47.98.210.172:8000'

type PersistedSession = {
  serverUrl: string
  apiBaseUrl: string
  serverName: string
  accessToken: string
  refreshToken: string
  accessTokenExpiresAt: string
  refreshTokenExpiresAt: string
  user: User | null
}

function loadPersistedSession(): PersistedSession {
  const fallback: PersistedSession = {
    serverUrl: '',
    apiBaseUrl: '',
    serverName: '',
    accessToken: '',
    refreshToken: '',
    accessTokenExpiresAt: '',
    refreshTokenExpiresAt: '',
    user: null,
  }

  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) {
    return fallback
  }

  try {
    return { ...fallback, ...(JSON.parse(raw) as Partial<PersistedSession>) }
  } catch {
    localStorage.removeItem(STORAGE_KEY)
    return fallback
  }
}

export const useSessionStore = defineStore('session', () => {
  const persisted = loadPersistedSession()

  const serverUrl = ref(persisted.serverUrl)
  const apiBase = ref(persisted.apiBaseUrl)
  const serverName = ref(persisted.serverName)
  const accessToken = ref(persisted.accessToken)
  const refreshToken = ref(persisted.refreshToken)
  const accessTokenExpiresAt = ref(persisted.accessTokenExpiresAt)
  const refreshTokenExpiresAt = ref(persisted.refreshTokenExpiresAt)
  const user = ref<User | null>(persisted.user)
  const loading = ref(false)
  const error = ref('')

  const hasVerifiedServer = computed(() => Boolean(serverUrl.value && apiBase.value))
  const isAuthenticated = computed(() => Boolean(accessToken.value && user.value))

  function persist() {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        serverUrl: serverUrl.value,
        apiBaseUrl: apiBase.value,
        serverName: serverName.value,
        accessToken: accessToken.value,
        refreshToken: refreshToken.value,
        accessTokenExpiresAt: accessTokenExpiresAt.value,
        refreshTokenExpiresAt: refreshTokenExpiresAt.value,
        user: user.value,
      } satisfies PersistedSession),
    )
  }

  function clearAuth() {
    accessToken.value = ''
    refreshToken.value = ''
    accessTokenExpiresAt.value = ''
    refreshTokenExpiresAt.value = ''
    user.value = null
    setAccessToken('')
    persist()
  }

  function syncApi() {
    if (serverUrl.value) {
      configureApi(serverUrl.value, accessToken.value)
    }
  }

  async function verify(inputServerUrl: string, secretKey: string) {
    loading.value = true
    error.value = ''

    try {
      const normalizedUrl = normalizeServerUrl(inputServerUrl)
      const result = await authApi.verifyService(normalizedUrl, secretKey)
      if (!result.is_valid) {
        throw new Error('服务密钥无效')
      }

      serverUrl.value = normalizedUrl
      apiBase.value = result.api_base_url || apiBaseUrl(normalizedUrl)
      serverName.value = result.server_name || 'Unilo'
      configureApi(serverUrl.value, accessToken.value)
      persist()
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '服务验证失败'
      throw caught
    } finally {
      loading.value = false
    }
  }

  async function login(payload: authApi.LoginPayload) {
    loading.value = true
    error.value = ''
    syncApi()

    try {
      const result = await authApi.login(payload)
      accessToken.value = result.access_token
      refreshToken.value = result.refresh_token
      accessTokenExpiresAt.value = result.access_token_expires_at
      refreshTokenExpiresAt.value = result.refresh_token_expires_at
      user.value = result.user
      setAccessToken(accessToken.value)
      persist()
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '登录失败'
      throw caught
    } finally {
      loading.value = false
    }
  }

  async function register(payload: authApi.RegisterPayload) {
    loading.value = true
    error.value = ''
    syncApi()

    try {
      await authApi.register(payload)
      await login({ username: payload.username, password: payload.password })
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '注册失败'
      throw caught
    } finally {
      loading.value = false
    }
  }

  async function loadMe() {
    if (!accessToken.value) {
      return
    }

    syncApi()
    try {
      user.value = await authApi.me()
      persist()
    } catch {
      clearAuth()
    }
  }

  async function logout() {
    const token = refreshToken.value
    syncApi()
    clearAuth()

    if (token) {
      try {
        await authApi.logout(token)
      } catch {
        return
      }
    }
  }

  syncApi()

  return {
    serverUrl,
    apiBase,
    serverName,
    accessToken,
    refreshToken,
    user,
    loading,
    error,
    hasVerifiedServer,
    isAuthenticated,
    verify,
    login,
    register,
    loadMe,
    logout,
    syncApi,
  }
})
