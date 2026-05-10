import { request } from './client'
import type { Drop, DropComment, DropLikeResponse, PaginatedResponse } from './types'

export function listDrops(page = 1, size = 20) {
  const params = new URLSearchParams({ page: String(page), size: String(size) })
  return request<PaginatedResponse<Drop>>(`/drops?${params.toString()}`)
}

export function createDrop(content: string) {
  return request<Drop>('/drops', {
    method: 'POST',
    body: { content },
  })
}

export function deleteDrop(dropId: string) {
  return request<unknown>(`/drops/${dropId}`, {
    method: 'DELETE',
  })
}

export function toggleDropLike(dropId: string) {
  return request<DropLikeResponse>(`/drops/${dropId}/like`, {
    method: 'POST',
  })
}

export function createDropComment(dropId: string, content: string, parentId: string | null = null, replyToUserId: string | null = null) {
  return request<DropComment>(`/drops/${dropId}/comments`, {
    method: 'POST',
    body: {
      content,
      parent_id: parentId,
      reply_to_user_id: replyToUserId,
    },
  })
}
