import { requestFormData } from './client'
import type { WorkspaceFile } from './types'

function hex(buffer: ArrayBuffer) {
  return [...new Uint8Array(buffer)].map((value) => value.toString(16).padStart(2, '0')).join('')
}

export async function sha256File(file: File) {
  return hex(await crypto.subtle.digest('SHA-256', await file.arrayBuffer()))
}

export async function uploadWorkspaceFile(file: File) {
  const formData = new FormData()
  formData.set('file', file)
  formData.set('file_hash', await sha256File(file))

  return requestFormData<WorkspaceFile>('/workspace/files/upload', formData)
}
