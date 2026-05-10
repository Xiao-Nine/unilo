import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import * as workspaceApi from '../api/workspace'
import type { WorkspaceFile } from '../api/types'
import { previewKind, type WorkspacePreviewKind } from '../utils/fileTypes'

type WorkspaceTabStatus = 'loading' | 'ready' | 'saving' | 'error'

type WorkspaceTab = {
  id: string
  file: WorkspaceFile
  kind: WorkspacePreviewKind
  status: WorkspaceTabStatus
  error: string
  previewUrl: string
  previewMimeType: string
  editorContent: string
  originalEditorContent: string
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const childrenByParentId = ref<Record<string, WorkspaceFile[]>>({})
  const expandedFolderIds = ref<Record<string, boolean>>({ root: true })
  const selectedDirectoryId = ref<string | null>(null)
  const loadingFolders = ref<Record<string, boolean>>({})
  const uploading = ref(false)
  const error = ref('')
  const openTabs = ref<WorkspaceTab[]>([])
  const activeTabId = ref('')

  const rootFiles = computed(() => childrenByParentId.value.root ?? [])
  const allFiles = computed(() => Object.values(childrenByParentId.value).flat())
  const activeTab = computed(() => openTabs.value.find((tab) => tab.id === activeTabId.value) ?? null)
  const selectedItemId = computed(() => activeTabId.value)
  const selectedItem = computed(() => activeTab.value?.file ?? null)
  const selectedKind = computed(() => activeTab.value?.kind ?? null)
  const previewUrl = computed(() => activeTab.value?.previewUrl ?? '')
  const previewMimeType = computed(() => activeTab.value?.previewMimeType ?? '')
  const editorContent = computed({
    get: () => activeTab.value?.editorContent ?? '',
    set: (value: string) => {
      const tab = activeTab.value
      if (tab) {
        tab.editorContent = value
      }
    },
  })
  const originalEditorContent = computed(() => activeTab.value?.originalEditorContent ?? '')
  const loadingPreview = computed(() => activeTab.value?.status === 'loading')
  const saving = computed(() => activeTab.value?.status === 'saving')
  const isDirty = computed(() => Boolean(activeTab.value && isTabDirty(activeTab.value)))
  const hasDirtyTabs = computed(() => openTabs.value.some(isTabDirty))

  function parentKey(parentId: string | null) {
    return parentId ?? 'root'
  }

  function isTabDirty(tab: WorkspaceTab) {
    return (tab.kind === 'text' || tab.kind === 'markdown') && tab.editorContent !== tab.originalEditorContent
  }

  function sortFiles(files: WorkspaceFile[]) {
    return [...files].sort((a, b) => {
      if (a.is_folder !== b.is_folder) {
        return a.is_folder ? -1 : 1
      }
      return a.name.localeCompare(b.name)
    })
  }

  function syncOpenTabFile(file: WorkspaceFile) {
    const tab = openTabs.value.find((item) => item.id === file.id)
    if (tab) {
      tab.file = file
    }
  }

  function upsertFile(file: WorkspaceFile) {
    const key = parentKey(file.parent_id)
    const current = childrenByParentId.value[key] ?? []
    const index = current.findIndex((item) => item.id === file.id)
    const next = index >= 0 ? current.map((item) => (item.id === file.id ? file : item)) : [...current, file]
    childrenByParentId.value = { ...childrenByParentId.value, [key]: sortFiles(next) }
    syncOpenTabFile(file)
  }

  function revokeTabPreviewUrl(tab: WorkspaceTab) {
    if (tab.previewUrl) {
      URL.revokeObjectURL(tab.previewUrl)
      tab.previewUrl = ''
    }
    tab.previewMimeType = ''
  }

  function removeTabWithoutSaving(tabId: string) {
    const index = openTabs.value.findIndex((tab) => tab.id === tabId)
    if (index < 0) {
      return
    }

    revokeTabPreviewUrl(openTabs.value[index])
    const nextTabs = openTabs.value.filter((tab) => tab.id !== tabId)
    openTabs.value = nextTabs
    if (activeTabId.value === tabId) {
      activeTabId.value = (nextTabs[index] ?? nextTabs[index - 1])?.id ?? ''
    }
  }

  function removeFile(fileId: string) {
    const next: Record<string, WorkspaceFile[]> = {}
    for (const [key, files] of Object.entries(childrenByParentId.value)) {
      next[key] = files.filter((file) => file.id !== fileId)
    }
    childrenByParentId.value = next
    removeTabWithoutSaving(fileId)
  }

  function createTab(file: WorkspaceFile): WorkspaceTab {
    return {
      id: file.id,
      file,
      kind: previewKind(file),
      status: 'loading',
      error: '',
      previewUrl: '',
      previewMimeType: '',
      editorContent: '',
      originalEditorContent: '',
    }
  }

  async function loadFolder(parentId: string | null = null) {
    const key = parentKey(parentId)
    loadingFolders.value = { ...loadingFolders.value, [key]: true }
    error.value = ''

    try {
      const result = await workspaceApi.listWorkspaceFiles(parentId)
      childrenByParentId.value = { ...childrenByParentId.value, [key]: sortFiles(result.files) }
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Workspace 加载失败'
    } finally {
      const next = { ...loadingFolders.value }
      delete next[key]
      loadingFolders.value = next
    }
  }

  async function toggleFolder(folder: WorkspaceFile) {
    if (!folder.is_folder) {
      return
    }
    const expanded = !expandedFolderIds.value[folder.id]
    expandedFolderIds.value = { ...expandedFolderIds.value, [folder.id]: expanded }
    if (expanded && !childrenByParentId.value[folder.id]) {
      await loadFolder(folder.id)
    }
  }

  async function loadTabPreview(tabId: string) {
    const tab = openTabs.value.find((item) => item.id === tabId)
    if (!tab) {
      return
    }

    revokeTabPreviewUrl(tab)
    tab.error = ''
    tab.status = 'loading'
    error.value = ''

    try {
      if (tab.kind === 'unsupported') {
        tab.status = 'ready'
        return
      }
      const blob = await workspaceApi.fetchWorkspacePreview(tab.file)
      tab.previewMimeType = blob.type || tab.file.mime_type
      if (tab.kind === 'text' || tab.kind === 'markdown') {
        const text = await blob.text()
        tab.editorContent = text
        tab.originalEditorContent = text
      } else {
        tab.previewUrl = URL.createObjectURL(blob)
      }
      tab.status = 'ready'
    } catch (caught) {
      tab.error = caught instanceof Error ? caught.message : '文件预览加载失败'
      error.value = tab.error
      tab.status = 'error'
    }
  }

  async function saveTabContent(tabId: string) {
    const tab = openTabs.value.find((item) => item.id === tabId)
    if (!tab || !isTabDirty(tab)) {
      return
    }

    tab.status = 'saving'
    tab.error = ''
    error.value = ''
    try {
      const saved = await workspaceApi.saveWorkspaceContent(tab.file.id, tab.editorContent)
      tab.originalEditorContent = tab.editorContent
      tab.file = saved
      upsertFile(saved)
      tab.status = 'ready'
    } catch (caught) {
      tab.error = caught instanceof Error ? caught.message : '保存失败'
      error.value = tab.error
      tab.status = 'error'
    }
  }

  async function saveSelectedContent() {
    if (activeTabId.value) {
      await saveTabContent(activeTabId.value)
    }
  }

  async function saveAllDirtyTabs() {
    for (const tab of [...openTabs.value]) {
      await saveTabContent(tab.id)
    }
  }

  async function activateTab(tabId: string) {
    if (activeTabId.value === tabId) {
      return
    }
    if (activeTabId.value) {
      await saveTabContent(activeTabId.value)
    }
    activeTabId.value = tabId
  }

  async function closeTab(tabId: string) {
    await saveTabContent(tabId)
    removeTabWithoutSaving(tabId)
  }

  function closeAllTabs() {
    for (const tab of openTabs.value) {
      revokeTabPreviewUrl(tab)
    }
    openTabs.value = []
    activeTabId.value = ''
  }

  async function openFile(file: WorkspaceFile, options: { single?: boolean } = {}) {
    if (file.is_folder) {
      return
    }

    if (options.single) {
      await saveAllDirtyTabs()
      closeAllTabs()
    }

    const existing = openTabs.value.find((tab) => tab.id === file.id)
    if (existing) {
      existing.file = file
      await activateTab(existing.id)
      return
    }

    const tab = createTab(file)
    openTabs.value = [...openTabs.value, tab]
    await activateTab(tab.id)
    await loadTabPreview(tab.id)
  }

  async function selectItem(file: WorkspaceFile) {
    await openFile(file)
  }

  async function createFolder(name: string, parentId: string | null = selectedDirectoryId.value) {
    error.value = ''
    try {
      const folder = await workspaceApi.createWorkspaceFolder(name, parentId)
      upsertFile(folder)
      return folder
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '创建文件夹失败'
      return null
    }
  }

  async function uploadFiles(files: File[], parentId: string | null = selectedDirectoryId.value) {
    if (files.length === 0) {
      return
    }
    uploading.value = true
    error.value = ''
    try {
      const uploaded = await Promise.all(files.map((file) => workspaceApi.uploadWorkspaceFile(file, parentId)))
      for (const file of uploaded) {
        upsertFile(file)
      }
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '上传失败'
    } finally {
      uploading.value = false
    }
  }

  async function renameItem(file: WorkspaceFile, name: string) {
    const nextName = name.trim()
    if (!nextName || nextName === file.name) {
      return
    }
    error.value = ''
    try {
      const renamed = await workspaceApi.renameWorkspaceFile(file.id, nextName)
      upsertFile(renamed)
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '重命名失败'
    }
  }

  async function moveItem(file: WorkspaceFile, targetParentId: string | null) {
    error.value = ''
    try {
      const moved = await workspaceApi.moveWorkspaceFile(file.id, targetParentId)
      removeFile(file.id)
      upsertFile(moved)
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '移动失败'
    }
  }

  async function deleteItem(file: WorkspaceFile) {
    error.value = ''
    try {
      await workspaceApi.deleteWorkspaceFile(file.id)
      removeFile(file.id)
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '删除失败'
    }
  }

  async function downloadItem(file: WorkspaceFile) {
    error.value = ''
    try {
      const blob = await workspaceApi.fetchWorkspaceDownload(file)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = file.name
      link.click()
      URL.revokeObjectURL(url)
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '下载失败'
    }
  }

  return {
    childrenByParentId,
    expandedFolderIds,
    selectedDirectoryId,
    loadingFolders,
    loadingPreview,
    uploading,
    saving,
    error,
    previewUrl,
    previewMimeType,
    editorContent,
    originalEditorContent,
    openTabs,
    activeTabId,
    activeTab,
    rootFiles,
    allFiles,
    selectedItemId,
    selectedItem,
    selectedKind,
    isDirty,
    hasDirtyTabs,
    parentKey,
    loadFolder,
    toggleFolder,
    openFile,
    selectItem,
    activateTab,
    closeTab,
    closeAllTabs,
    createFolder,
    uploadFiles,
    renameItem,
    moveItem,
    deleteItem,
    saveTabContent,
    saveSelectedContent,
    saveAllDirtyTabs,
    downloadItem,
  }
})
