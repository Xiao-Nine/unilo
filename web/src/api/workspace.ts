import { fetchAuthorizedBlob, request, requestFormData } from './client'
import type {
  DeleteWorkspaceFileResponse,
  WorkspaceFile,
  WorkspaceHashCheckResponse,
  WorkspaceListResponse,
} from './types'

function hex(buffer: ArrayBuffer) {
  return [...new Uint8Array(buffer)].map((value) => value.toString(16).padStart(2, '0')).join('')
}

export async function sha256File(file: File) {
  return hex(await crypto.subtle.digest('SHA-256', await file.arrayBuffer()))
}

export function listWorkspaceFiles(parentId: string | null = null) {
  const params = new URLSearchParams()
  if (parentId) {
    params.set('parent_id', parentId)
  }
  const query = params.toString()
  return request<WorkspaceListResponse>(`/workspace/files${query ? `?${query}` : ''}`)
}

export function createWorkspaceFolder(name: string, parentId: string | null = null) {
  return request<WorkspaceFile>('/workspace/folders', {
    method: 'POST',
    body: { name, parent_id: parentId },
  })
}

export function checkWorkspaceFile(fileHash: string, name: string, parentId: string | null = null) {
  return request<WorkspaceHashCheckResponse>('/workspace/files/check', {
    method: 'POST',
    body: { file_hash: fileHash, name, parent_id: parentId },
  })
}

export async function uploadWorkspaceFile(file: File, parentId: string | null = null) {
  const formData = new FormData()
  formData.set('file', file)
  formData.set('file_hash', await sha256File(file))
  if (parentId) {
    formData.set('parent_id', parentId)
  }

  return requestFormData<WorkspaceFile>('/workspace/files/upload', formData)
}

export function renameWorkspaceFile(fileId: string, name: string) {
  return request<WorkspaceFile>(`/workspace/files/${fileId}`, {
    method: 'PATCH',
    body: { name },
  })
}

export function moveWorkspaceFile(fileId: string, targetParentId: string | null) {
  return request<WorkspaceFile>(`/workspace/files/${fileId}/move`, {
    method: 'POST',
    body: { target_parent_id: targetParentId },
  })
}

export function deleteWorkspaceFile(fileId: string) {
  return request<DeleteWorkspaceFileResponse>(`/workspace/files/${fileId}`, {
    method: 'DELETE',
  })
}

export function fetchWorkspacePreview(file: WorkspaceFile) {
  return fetchAuthorizedBlob(file.preview_url || `/workspace/files/${file.id}/preview`)
}

export function fetchWorkspaceDownload(file: WorkspaceFile) {
  return fetchAuthorizedBlob(file.download_url || `/workspace/files/${file.id}/download`)
}

export async function saveWorkspaceContent(fileId: string, content: string) {
  const fileHash = hex(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(content)))
  return request<WorkspaceFile>(`/workspace/files/${fileId}/content`, {
    method: 'PUT',
    body: { content, file_hash: fileHash },
  })
}
