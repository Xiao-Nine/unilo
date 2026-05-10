import type { WorkspaceFile } from '../api/types'

const editableExtensions = new Set([
  'txt',
  'md',
  'markdown',
  'json',
  'js',
  'ts',
  'vue',
  'css',
  'html',
  'sh',
  'bash',
  'zsh',
  'yaml',
  'yml',
  'toml',
  'xml',
  'csv',
])

const markdownExtensions = new Set(['md', 'markdown'])

export type WorkspacePreviewKind = 'folder' | 'markdown' | 'text' | 'image' | 'video' | 'unsupported'

export function fileExtension(name: string) {
  const parts = name.toLowerCase().split('.')
  return parts.length > 1 ? parts.at(-1) ?? '' : ''
}

export function isMarkdownFile(file: WorkspaceFile) {
  return markdownExtensions.has(fileExtension(file.name))
}

export function isEditableTextFile(file: WorkspaceFile) {
  if (file.is_folder) {
    return false
  }
  return file.mime_type.startsWith('text/') || editableExtensions.has(fileExtension(file.name))
}

export function isImageFile(file: WorkspaceFile) {
  return file.mime_type.startsWith('image/')
}

export function isVideoFile(file: WorkspaceFile) {
  return file.mime_type.startsWith('video/')
}

export function previewKind(file: WorkspaceFile): WorkspacePreviewKind {
  if (file.is_folder) {
    return 'folder'
  }
  if (isMarkdownFile(file)) {
    return 'markdown'
  }
  if (isEditableTextFile(file)) {
    return 'text'
  }
  if (isImageFile(file)) {
    return 'image'
  }
  if (isVideoFile(file)) {
    return 'video'
  }
  return 'unsupported'
}
