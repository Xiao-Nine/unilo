import { request, requestWithServer } from './client'
import type { LoginResponse, RefreshResponse, User, VerifyResponse } from './types'

export type RegisterPayload = {
  username: string
  password: string
  nickname: string
}

export type LoginPayload = {
  username: string
  password: string
}

export function verifyService(serverUrl: string, secretKey: string) {
  return requestWithServer<VerifyResponse>(serverUrl, '/auth/verify', {
    method: 'POST',
    body: { secret_key: secretKey },
  })
}

export function register(payload: RegisterPayload) {
  return request<{ user: User }>('/auth/register', {
    method: 'POST',
    body: payload,
  })
}

export function login(payload: LoginPayload) {
  return request<LoginResponse>('/auth/login', {
    method: 'POST',
    body: payload,
  })
}

export function refresh(refreshToken: string) {
  return request<RefreshResponse>('/auth/refresh', {
    method: 'POST',
    body: { refresh_token: refreshToken },
  })
}

export function logout(refreshToken: string) {
  return request<{ logged_out: boolean }>('/auth/logout', {
    method: 'POST',
    body: { refresh_token: refreshToken },
  })
}

export function me() {
  return request<User>('/auth/me')
}
