import type { ApiErrorPayload, ApiResponse } from './types'

export class ApiError extends Error {
  code: number
  status: number

  constructor(payload: ApiErrorPayload) {
    super(payload.msg)
    this.name = 'ApiError'
    this.code = payload.code
    this.status = payload.status
  }
}

export type RequestOptions = {
  method?: string
  body?: unknown
  token?: string
}

let activeServerUrl = ''
let activeAccessToken = ''
let unauthorizedHandler: (() => void) | null = null

export function normalizeServerUrl(serverUrl: string) {
  return serverUrl.trim().replace(/\/+$/, '')
}

export function apiBaseUrl(serverUrl: string) {
  return `${normalizeServerUrl(serverUrl)}/api/v1`
}

export function websocketUrl(serverUrl: string, accessToken: string) {
  const url = new URL(`${normalizeServerUrl(serverUrl)}/api/v1/ws`)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.searchParams.set('token', accessToken)
  return url.toString()
}

export function resolveApiUrl(pathOrUrl: string) {
  if (!activeServerUrl) {
    throw new ApiError({ code: 400, msg: '请先完成服务验证', status: 400 })
  }
  if (/^https?:\/\//.test(pathOrUrl)) {
    return pathOrUrl
  }
  if (pathOrUrl.startsWith('/api/v1/')) {
    return `${activeServerUrl}${pathOrUrl}`
  }
  const path = pathOrUrl.startsWith('/') ? pathOrUrl : `/${pathOrUrl}`
  return `${apiBaseUrl(activeServerUrl)}${path}`
}

export function configureApi(serverUrl: string, accessToken = '') {
  activeServerUrl = normalizeServerUrl(serverUrl)
  activeAccessToken = accessToken
}

export function setAccessToken(accessToken: string) {
  activeAccessToken = accessToken
}

export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler
}

async function parseApiResponse<T>(response: Response) {
  let payload: ApiResponse<T> | null = null
  try {
    payload = (await response.json()) as ApiResponse<T>
  } catch {
    throw new ApiError({ code: response.status, msg: '服务响应格式错误', status: response.status })
  }

  if (!response.ok || payload.code !== 200) {
    const error = new ApiError({ code: payload.code, msg: payload.msg || '请求失败', status: response.status })
    if (response.status === 401) {
      unauthorizedHandler?.()
    }
    throw error
  }

  return payload.data
}

function authHeaders(token = activeAccessToken) {
  const headers = new Headers()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  return headers
}

export async function request<T>(path: string, options: RequestOptions = {}) {
  const headers = authHeaders(options.token ?? activeAccessToken)
  headers.set('Content-Type', 'application/json')

  const response = await fetch(resolveApiUrl(path), {
    method: options.method ?? 'GET',
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  })

  return parseApiResponse<T>(response)
}

export async function requestFormData<T>(path: string, formData: FormData, options: RequestOptions = {}) {
  const response = await fetch(resolveApiUrl(path), {
    method: options.method ?? 'POST',
    headers: authHeaders(options.token ?? activeAccessToken),
    body: formData,
  })

  return parseApiResponse<T>(response)
}

export async function fetchAuthorizedBlob(pathOrUrl: string, token = activeAccessToken) {
  const response = await fetch(resolveApiUrl(pathOrUrl), {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    if (response.status === 401) {
      unauthorizedHandler?.()
    }
    throw new ApiError({ code: response.status, msg: '文件预览加载失败', status: response.status })
  }

  return response.blob()
}

export async function requestWithServer<T>(serverUrl: string, path: string, options: RequestOptions = {}) {
  const previousServerUrl = activeServerUrl
  const previousAccessToken = activeAccessToken
  configureApi(serverUrl, options.token ?? '')

  try {
    return await request<T>(path, options)
  } finally {
    activeServerUrl = previousServerUrl
    activeAccessToken = previousAccessToken
  }
}
