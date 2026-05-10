<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import ChatMarkdown from '../components/ChatMarkdown.vue'
import WorkspaceTreeNode from '../components/workspace/WorkspaceTreeNode.vue'
import { uploadWorkspaceFile } from '../api/workspace'
import { useChannelsStore } from '../stores/channels'
import { useDropsStore } from '../stores/drops'
import { useSessionStore } from '../stores/session'
import { useThemeStore } from '../stores/theme'
import { useWorkspaceStore } from '../stores/workspace'
import type { Drop, Message, MessageAttachment, WorkspaceFile } from '../api/types'

const router = useRouter()
const session = useSessionStore()
const theme = useThemeStore()
const channelsStore = useChannelsStore()
const dropsStore = useDropsStore()
const workspaceStore = useWorkspaceStore()

const {
  channels,
  selectedChannel,
  selectedMessages,
  loadingChannels,
  loadingMessages,
  sending,
  error: channelError,
  selectedHasMore,
  selectedLoadingOlder,
  selectedTypingUserIds,
  realtimeStatusLabel,
  realtimeError,
  presenceByUser,
  unreadByChannel,
} = storeToRefs(channelsStore)

const {
  activeTag,
  loading: dropsLoading,
  publishing: dropsPublishing,
  liking: dropsLiking,
  commenting: dropsCommenting,
  deleting: dropsDeleting,
  error: dropsError,
  backendUnavailable: dropsBackendUnavailable,
  tags: dropTags,
  filteredPosts,
} = storeToRefs(dropsStore)

const {
  childrenByParentId,
  expandedFolderIds,
  selectedItemId,
  loadingFolders,
  loadingPreview,
  uploading: workspaceUploading,
  saving: workspaceSaving,
  error: workspaceError,
  previewUrl: workspacePreviewUrl,
  editorContent,
  openTabs: workspaceTabs,
  activeTabId: activeWorkspaceTabId,
  rootFiles,
  allFiles,
  selectedItem: selectedWorkspaceItem,
  selectedKind: workspacePreviewKind,
  isDirty: workspaceIsDirty,
  hasDirtyTabs: workspaceHasDirtyTabs,
} = storeToRefs(workspaceStore)

type ActiveFeature = 'home' | 'channels' | 'drops' | 'workspace'

type ComposerFile = {
  id: string
  file: File
  localUrl: string
  name: string
  size: number
  mimeType: string
}

type MenuState<T> = {
  target: T
  x: number
  y: number
}

const features: { id: ActiveFeature; label: string; icon: string }[] = [
  { id: 'channels', label: 'Channels', icon: '#' },
  { id: 'drops', label: 'Drops', icon: 'D' },
  { id: 'workspace', label: 'Workspace', icon: 'W' },
]

const newChannelName = ref('')
const editingChannelName = ref('')
const isRenamingChannel = ref(false)
const renameInput = ref<HTMLInputElement | null>(null)
const channelMenu = ref<MenuState<string> | null>(null)
const composer = ref('')
const composerFiles = ref<ComposerFile[]>([])
const composerFileInput = ref<HTMLInputElement | null>(null)
const replyingTo = ref<Message | null>(null)
const activeFeature = ref<ActiveFeature>('channels')
const newDropContent = ref('')
const dropCommentDrafts = ref<Record<string, string>>({})
const mobileMainOpen = ref(false)
const messagesViewport = ref<HTMLElement | null>(null)
const shouldStickToBottom = ref(true)
const suppressHistoryLoad = ref(false)
const unseenBottomMessages = ref(0)
const previewImage = ref<{ src: string; alt: string } | null>(null)
const previewScale = ref(1)
const previewOverlay = ref<HTMLElement | null>(null)
const messageSearch = ref('')
const workspaceMenu = ref<MenuState<WorkspaceFile> | null>(null)
const workspaceFileInput = ref<HTMLInputElement | null>(null)
const workspaceUploadTarget = ref<WorkspaceFile | null>(null)
const isMobileWorkspace = ref(false)
let mobileMediaQuery: MediaQueryList | undefined
let typingThrottleTimer: number | undefined
let typingIdleTimer: number | undefined
let typingChannelId = ''

const sendersById = computed(() => {
  const senders = new Map<string, string>()
  for (const message of selectedMessages.value) {
    senders.set(message.sender.id, message.sender.nickname)
  }
  return senders
})

const typingLabel = computed(() => {
  const names = selectedTypingUserIds.value.map((userId) => sendersById.value.get(userId) ?? '有人')
  if (names.length === 0) return ''
  if (names.length === 1) return `${names[0]} 正在输入...`
  return `${names.slice(0, 2).join('、')} 等正在输入...`
})

const selectedMembers = computed(() => {
  const members = new Map<string, { id: string; nickname: string; avatar_url: string }>()
  for (const message of selectedMessages.value) {
    members.set(message.sender.id, message.sender)
  }
  if (session.user) {
    members.set(session.user.id, session.user)
  }
  return [...members.values()]
})

const onlineMembers = computed(() =>
  selectedMembers.value.filter((member) => member.id === session.user?.id || presenceByUser.value[member.id] === 'online'),
)

const messagesById = computed(() => {
  const messages = new Map<number, Message>()
  for (const message of selectedMessages.value) {
    messages.set(message.id, message)
  }
  return messages
})

const visibleMessages = computed(() => {
  const keyword = messageSearch.value.trim().toLowerCase()
  if (!keyword) {
    return selectedMessages.value
  }
  return selectedMessages.value.filter(
    (message) =>
      message.content.toLowerCase().includes(keyword) || message.sender.nickname.toLowerCase().includes(keyword),
  )
})

const composerPlaceholder = '输入 Markdown 消息，Enter 发送，Shift+Enter 换行；可粘贴或添加文件'
const workspaceRootLoading = computed(() => Boolean(loadingFolders.value.root))
const workspaceFolders = computed(() => allFiles.value.filter((file) => file.is_folder))

function formatTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function canDeleteDrop(post: Drop) {
  return post.author_id === session.user?.id
}

async function publishDrop() {
  const created = await dropsStore.publishDrop(newDropContent.value)
  if (created) {
    newDropContent.value = ''
  }
}

async function deleteDrop(post: Drop) {
  const confirmed = window.confirm('确定删除这条 Drop 吗？')
  if (confirmed) {
    await dropsStore.deleteDrop(post.id)
  }
}

async function submitDropComment(post: Drop) {
  const comment = await dropsStore.createComment(post.id, dropCommentDrafts.value[post.id] ?? '')
  if (comment) {
    dropCommentDrafts.value = { ...dropCommentDrafts.value, [post.id]: '' }
  }
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${Math.ceil(value / 1024)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

async function openImagePreview(src: string, alt = '') {
  previewImage.value = { src, alt }
  previewScale.value = 1
  await nextTick()
  previewOverlay.value?.focus()
}

function closeImagePreview() {
  previewImage.value = null
  previewScale.value = 1
}

function zoomImagePreview(delta: number) {
  previewScale.value = Math.min(5, Math.max(0.2, Number((previewScale.value + delta).toFixed(2))))
}

function handlePreviewWheel(event: WheelEvent) {
  if (!event.ctrlKey) return
  event.preventDefault()
  zoomImagePreview(event.deltaY > 0 ? -0.12 : 0.12)
}

function handlePreviewKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    closeImagePreview()
    return
  }
  if (event.key === '+' || event.key === '=') {
    zoomImagePreview(0.15)
    return
  }
  if (event.key === '-') {
    zoomImagePreview(-0.15)
    return
  }
  if (event.key === '0') {
    previewScale.value = 1
  }
}

function messagePreview(message: Message) {
  const content = message.content.trim()
  if (message.msg_type === 'image' || message.metadata.attachments?.length) return '[附件]'
  if (message.msg_type === 'code') return content.split('\n')[0] || '[代码]'
  return (
    content
      .replace(/!\[[^\]]*\]\([^)]*\)/g, '[图片]')
      .replace(/[#*_`>\-[\]()]/g, '')
      .trim()
      .split('\n')[0] || '[消息]'
  )
}

function resolveReply(message: Message) {
  return message.reply_to_id ? messagesById.value.get(message.reply_to_id) : null
}

function startReply(message: Message) {
  replyingTo.value = message
}

function cancelReply() {
  replyingTo.value = null
}

async function activateFeature(feature: ActiveFeature) {
  if (activeFeature.value === 'workspace' && feature !== 'workspace') {
    await workspaceStore.saveAllDirtyTabs()
  }
  activeFeature.value = feature
  mobileMainOpen.value = feature === 'channels' && Boolean(selectedChannel.value)
  closeChannelMenu()
  closeWorkspaceMenu()
}

async function createChannel() {
  await channelsStore.createChannel(newChannelName.value)
  newChannelName.value = ''
  mobileMainOpen.value = true
}

async function selectChannelForMobile(channelId: string) {
  mobileMainOpen.value = true
  await channelsStore.selectChannel(channelId)
}

function closeMobileMain() {
  mobileMainOpen.value = false
  stopTyping()
  replyingTo.value = null
  cancelRenameChannel()
}

async function startRenameChannel() {
  if (!selectedChannel.value) return
  editingChannelName.value = selectedChannel.value.name
  isRenamingChannel.value = true
  await nextTick()
  renameInput.value?.focus()
  renameInput.value?.select()
}

function cancelRenameChannel() {
  isRenamingChannel.value = false
  editingChannelName.value = ''
}

async function submitRenameChannel() {
  if (!isRenamingChannel.value || !selectedChannel.value) return

  const channelId = selectedChannel.value.id
  const nextName = editingChannelName.value.trim()
  const previousName = selectedChannel.value.name
  cancelRenameChannel()
  if (nextName && nextName !== previousName) {
    await channelsStore.renameChannel(channelId, nextName)
  }
}

function finishRenameChannel() {
  renameInput.value?.blur()
}

function openChannelMenu(channelId: string, event: MouseEvent) {
  event.preventDefault()
  event.stopPropagation()
  channelMenu.value = { target: channelId, x: event.clientX, y: event.clientY }
}

function closeChannelMenu() {
  channelMenu.value = null
}

async function renameChannelFromMenu() {
  const channelId = channelMenu.value?.target
  const channel = channels.value.find((item) => item.id === channelId)
  closeChannelMenu()
  if (!channel) return
  const nextName = window.prompt('重命名频道', channel.name)?.trim()
  if (nextName && nextName !== channel.name) {
    await channelsStore.renameChannel(channel.id, nextName)
  }
}

async function deleteChannelFromMenu() {
  const channelId = channelMenu.value?.target
  const channel = channels.value.find((item) => item.id === channelId)
  closeChannelMenu()
  if (!channelId || !channel) return

  const confirmed = window.confirm(`确定删除频道 #${channel.name} 吗？`)
  if (!confirmed) return
  if (selectedChannel.value?.id === channelId) {
    cancelRenameChannel()
  }
  await channelsStore.deleteChannel(channelId)
}

function clearTypingTimers() {
  if (typingThrottleTimer !== undefined) {
    window.clearTimeout(typingThrottleTimer)
    typingThrottleTimer = undefined
  }
  if (typingIdleTimer !== undefined) {
    window.clearTimeout(typingIdleTimer)
    typingIdleTimer = undefined
  }
}

function stopTyping() {
  if (typingChannelId) {
    channelsStore.sendTyping(false, typingChannelId)
    typingChannelId = ''
  }
  clearTypingTimers()
}

function handleComposerInput() {
  if (!selectedChannel.value) return

  if (typingChannelId && typingChannelId !== selectedChannel.value.id) {
    channelsStore.sendTyping(false, typingChannelId)
  }
  typingChannelId = selectedChannel.value.id

  if (typingThrottleTimer === undefined) {
    channelsStore.sendTyping(true, typingChannelId)
    typingThrottleTimer = window.setTimeout(() => {
      typingThrottleTimer = undefined
    }, 2000)
  }

  if (typingIdleTimer !== undefined) {
    window.clearTimeout(typingIdleTimer)
  }
  typingIdleTimer = window.setTimeout(stopTyping, 2500)
}

function handleComposerKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey) return
  event.preventDefault()
  void sendMessage()
}

function revokeComposerFile(file: ComposerFile) {
  if (file.localUrl) {
    URL.revokeObjectURL(file.localUrl)
  }
}

function clearComposerFiles() {
  for (const file of composerFiles.value) {
    revokeComposerFile(file)
  }
  composerFiles.value = []
}

function removeComposerFile(fileId: string) {
  const file = composerFiles.value.find((item) => item.id === fileId)
  if (file) {
    revokeComposerFile(file)
  }
  composerFiles.value = composerFiles.value.filter((item) => item.id !== fileId)
}

function appendComposerFiles(files: File[]) {
  composerFiles.value = [
    ...composerFiles.value,
    ...files.map((file) => ({
      id: crypto.randomUUID(),
      file,
      localUrl: file.type.startsWith('image/') ? URL.createObjectURL(file) : '',
      name: file.name || `附件-${Date.now()}`,
      size: file.size,
      mimeType: file.type || 'application/octet-stream',
    })),
  ]
}

function handleComposerPaste(event: ClipboardEvent) {
  const images = [...(event.clipboardData?.items ?? [])]
    .filter((item) => item.kind === 'file')
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file && file.type.startsWith('image/')))

  if (images.length === 0) return
  event.preventDefault()
  appendComposerFiles(images)
}

function openComposerFilePicker() {
  composerFileInput.value?.click()
}

function handleComposerFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  appendComposerFiles([...(input.files ?? [])])
  input.value = ''
}

function attachmentFromFile(file: WorkspaceFile): MessageAttachment {
  return {
    file_id: file.id,
    name: file.name,
    mime_type: file.mime_type,
    size_bytes: file.size_bytes,
    preview_url: file.preview_url,
    download_url: file.download_url,
  }
}

function buildMarkdownContent(attachments: MessageAttachment[]) {
  const parts = [composer.value.trim()]
  for (const attachment of attachments) {
    if (attachment.mime_type.startsWith('image/')) {
      parts.push(`![${attachment.name}](${attachment.preview_url})`)
    } else {
      parts.push(`[${attachment.name}](${attachment.download_url})`)
    }
  }
  return parts.filter(Boolean).join('\n\n')
}

async function sendMessage() {
  if (!composer.value.trim() && composerFiles.value.length === 0) return

  const uploadedFiles = await Promise.all(composerFiles.value.map((item) => uploadWorkspaceFile(item.file)))
  const attachments = uploadedFiles.map(attachmentFromFile)
  const sent = await channelsStore.sendMessage(buildMarkdownContent(attachments), {
    replyToId: replyingTo.value?.id ?? null,
    msgType: 'text',
    metadata: { attachments },
  })
  if (sent) {
    stopTyping()
    composer.value = ''
    replyingTo.value = null
    clearComposerFiles()
    shouldStickToBottom.value = true
    await scrollMessagesToBottom()
  }
}

function isNearBottom() {
  const viewport = messagesViewport.value
  if (!viewport) return true
  return viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight < 120
}

function markCurrentChannelRead() {
  if (!selectedChannel.value || !shouldStickToBottom.value) return
  const latestMessage = selectedMessages.value[selectedMessages.value.length - 1]
  if (latestMessage && latestMessage.id > selectedChannel.value.last_read_message_id) {
    void channelsStore.markChannelRead(selectedChannel.value.id, latestMessage.id)
  }
}

function handleMessagesScroll() {
  const viewport = messagesViewport.value
  shouldStickToBottom.value = isNearBottom()
  if (shouldStickToBottom.value) {
    unseenBottomMessages.value = 0
    markCurrentChannelRead()
  }

  if (
    viewport &&
    !suppressHistoryLoad.value &&
    !shouldStickToBottom.value &&
    viewport.scrollTop < 80 &&
    selectedHasMore.value &&
    !selectedLoadingOlder.value &&
    !loadingMessages.value
  ) {
    void loadOlderMessages()
  }
}

async function scrollMessagesToBottom() {
  await nextTick()
  if (messagesViewport.value) {
    messagesViewport.value.scrollTop = messagesViewport.value.scrollHeight
  }
  shouldStickToBottom.value = true
  unseenBottomMessages.value = 0
  markCurrentChannelRead()
}

async function loadOlderMessages() {
  const viewport = messagesViewport.value
  const previousScrollHeight = viewport?.scrollHeight ?? 0
  const previousScrollTop = viewport?.scrollTop ?? 0

  await channelsStore.loadOlderMessages()
  await nextTick()

  if (viewport) {
    viewport.scrollTop = viewport.scrollHeight - previousScrollHeight + previousScrollTop
  }
}

async function refreshCurrentChannel() {
  if (!selectedChannel.value) return

  suppressHistoryLoad.value = true
  shouldStickToBottom.value = true
  await channelsStore.loadMessages()
  await scrollMessagesToBottom()
  window.setTimeout(() => {
    suppressHistoryLoad.value = false
  }, 100)
}

async function logout() {
  channelsStore.reset()
  await session.logout()
  await router.push('/login')
}

function openWorkspaceMenu(item: WorkspaceFile, event: MouseEvent) {
  event.preventDefault()
  event.stopPropagation()
  workspaceMenu.value = { target: item, x: event.clientX, y: event.clientY }
}

function closeWorkspaceMenu() {
  workspaceMenu.value = null
}

async function selectWorkspaceItem(item: WorkspaceFile) {
  if (item.is_folder) {
    await workspaceStore.toggleFolder(item)
    return
  }
  await workspaceStore.openFile(item, { single: isMobileWorkspace.value })
  if (isMobileWorkspace.value) {
    mobileMainOpen.value = true
  }
}

async function toggleWorkspaceFolder(item: WorkspaceFile) {
  await workspaceStore.toggleFolder(item)
}

async function createWorkspaceFolder() {
  const name = window.prompt('新建文件夹名称')?.trim()
  if (name) {
    await workspaceStore.createFolder(name)
  }
}

async function renameWorkspaceItem() {
  const item = workspaceMenu.value?.target ?? selectedWorkspaceItem.value
  closeWorkspaceMenu()
  if (!item) return
  const name = window.prompt('重命名', item.name)?.trim()
  if (name) {
    await workspaceStore.renameItem(item, name)
  }
}

async function moveWorkspaceItem() {
  const item = workspaceMenu.value?.target ?? selectedWorkspaceItem.value
  closeWorkspaceMenu()
  if (!item) return
  const choices = workspaceFolders.value.filter((folder) => folder.id !== item.id)
  const message = ['输入目标文件夹名称，留空移动到根目录：', ...choices.map((folder) => `- ${folder.name}`)].join('\n')
  const folderName = window.prompt(message, '')?.trim()
  const target = folderName ? choices.find((folder) => folder.name === folderName) : null
  if (folderName && !target) {
    window.alert('未找到该目标文件夹')
    return
  }
  await workspaceStore.moveItem(item, target?.id ?? null)
}

async function deleteWorkspaceItem() {
  const item = workspaceMenu.value?.target ?? selectedWorkspaceItem.value
  closeWorkspaceMenu()
  if (!item) return
  const confirmed = window.confirm(`确定将 ${item.name} 移入回收站吗？`)
  if (confirmed) {
    await workspaceStore.deleteItem(item)
  }
}

async function downloadWorkspaceItem() {
  const item = workspaceMenu.value?.target ?? selectedWorkspaceItem.value
  closeWorkspaceMenu()
  if (item && !item.is_folder) {
    await workspaceStore.downloadItem(item)
  }
}

function openWorkspaceFilePicker(target: WorkspaceFile | null = null) {
  workspaceUploadTarget.value = target
  workspaceFileInput.value?.click()
}

function uploadToWorkspaceMenuTarget() {
  const item = workspaceMenu.value?.target
  closeWorkspaceMenu()
  if (item?.is_folder) {
    openWorkspaceFilePicker(item)
  }
}

async function handleWorkspaceFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  await workspaceStore.uploadFiles([...(input.files ?? [])], workspaceUploadTarget.value?.id ?? null)
  workspaceUploadTarget.value = null
  input.value = ''
}

async function handleWorkspaceDrop(event: DragEvent, targetFolder: WorkspaceFile | null = null) {
  event.preventDefault()
  event.stopPropagation()
  const files = [...(event.dataTransfer?.files ?? [])]
  await workspaceStore.uploadFiles(files, targetFolder?.id ?? null)
}

async function activateWorkspaceTab(tabId: string) {
  await workspaceStore.activateTab(tabId)
}

async function closeWorkspaceTab(tabId: string) {
  await workspaceStore.closeTab(tabId)
}

async function saveWorkspaceContent() {
  await workspaceStore.saveSelectedContent()
}

function updateMobileWorkspace() {
  isMobileWorkspace.value = mobileMediaQuery?.matches ?? window.matchMedia('(max-width: 820px)').matches
}

async function handleWorkspaceSaveShortcut(event: KeyboardEvent) {
  if (activeFeature.value !== 'workspace') {
    return
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 's') {
    event.preventDefault()
    await workspaceStore.saveSelectedContent()
  }
}

function handleBeforeUnload(event: BeforeUnloadEvent) {
  if (!workspaceHasDirtyTabs.value) {
    return
  }
  void workspaceStore.saveAllDirtyTabs()
  event.preventDefault()
  event.returnValue = ''
}

function handlePageHide() {
  void workspaceStore.saveAllDirtyTabs()
}

watch(
  () => selectedMessages.value.length,
  (nextLength, previousLength) => {
    if (shouldStickToBottom.value) {
      void scrollMessagesToBottom()
      markCurrentChannelRead()
    } else if (nextLength > previousLength && !selectedLoadingOlder.value) {
      unseenBottomMessages.value += nextLength - previousLength
    }
  },
)

watch(
  () => channelsStore.selectedChannelId,
  () => {
    stopTyping()
    replyingTo.value = null
    cancelRenameChannel()
    shouldStickToBottom.value = true
    messageSearch.value = ''
    void scrollMessagesToBottom()
  },
)

watch(activeFeature, (feature) => {
  if (feature === 'drops' && !dropsStore.posts.length && !dropsLoading.value) {
    void dropsStore.loadDrops()
  }
  if (feature === 'workspace' && !rootFiles.value.length && !workspaceRootLoading.value) {
    void workspaceStore.loadFolder()
  }
})

onMounted(async () => {
  session.syncApi()
  mobileMediaQuery = window.matchMedia('(max-width: 820px)')
  updateMobileWorkspace()
  mobileMediaQuery.addEventListener('change', updateMobileWorkspace)
  window.addEventListener('keydown', handleWorkspaceSaveShortcut)
  window.addEventListener('beforeunload', handleBeforeUnload)
  window.addEventListener('pagehide', handlePageHide)
  await channelsStore.loadChannels()
  channelsStore.connectRealtime(session.serverUrl, session.accessToken)
  await scrollMessagesToBottom()
})

onBeforeUnmount(() => {
  stopTyping()
  clearComposerFiles()
  void workspaceStore.saveAllDirtyTabs()
  workspaceStore.closeAllTabs()
  mobileMediaQuery?.removeEventListener('change', updateMobileWorkspace)
  window.removeEventListener('keydown', handleWorkspaceSaveShortcut)
  window.removeEventListener('beforeunload', handleBeforeUnload)
  window.removeEventListener('pagehide', handlePageHide)
  channelsStore.disconnectRealtime()
})
</script>

<template>
  <main :class="['app-shell', `feature-${activeFeature}`, { 'mobile-main-open': mobileMainOpen }]" @click="closeChannelMenu(); closeWorkspaceMenu()">
    <aside class="feature-rail" @click.stop>
      <button
        :class="['brand-mark', { active: activeFeature === 'home' }]"
        type="button"
        title="主页"
        @click="activateFeature('home')"
      >
        U
      </button>
      <div class="feature-buttons">
        <button
          v-for="feature in features"
          :key="feature.id"
          :class="['module-button', { active: activeFeature === feature.id }]"
          type="button"
          :title="feature.label"
          @click="activateFeature(feature.id)"
        >
          {{ feature.icon }}
        </button>
      </div>
      <div class="rail-bottom">
        <button class="module-button" type="button" :title="`切换主题：${theme.label}`" @click="theme.toggle">
          {{ theme.mode === 'dark' ? '亮' : '暗' }}
        </button>
        <button class="module-button" type="button" title="退出" @click="logout">↩</button>
      </div>
    </aside>

    <aside v-if="activeFeature === 'channels'" class="feature-sidebar channel-sidebar" @click.stop="closeChannelMenu">
      <header class="sidebar-header">
        <div>
          <p class="eyebrow">Space</p>
          <h2>{{ session.serverName || 'Unilo' }}</h2>
        </div>
        <span class="status-pill" :title="realtimeError || realtimeStatusLabel">{{ realtimeStatusLabel }}</span>
      </header>

      <form class="inline-form" @submit.prevent="createChannel">
        <input v-model="newChannelName" placeholder="新建频道" />
        <button type="submit">+</button>
      </form>

      <div class="channel-list">
        <div
          v-for="channel in channels"
          :key="channel.id"
          :class="['channel-row', { active: selectedChannel?.id === channel.id }]"
          @contextmenu="openChannelMenu(channel.id, $event)"
        >
          <button class="channel-item" type="button" @click="selectChannelForMobile(channel.id)">
            <span>#</span>
            <span class="channel-name">{{ channel.name }}</span>
            <span v-if="unreadByChannel[channel.id]" class="unread-badge">
              {{ unreadByChannel[channel.id] > 99 ? '99+' : unreadByChannel[channel.id] }}
            </span>
          </button>
          <button class="channel-menu-trigger" type="button" title="频道设置" @click="openChannelMenu(channel.id, $event)">
            ⚙
          </button>
        </div>
        <p v-if="loadingChannels" class="muted compact">正在加载频道...</p>
        <p v-else-if="channels.length === 0" class="muted compact">还没有频道，创建一个开始聊天。</p>
      </div>

      <div
        v-if="channelMenu"
        class="context-menu"
        :style="{ left: `${channelMenu.x}px`, top: `${channelMenu.y}px` }"
        @click.stop
      >
        <button type="button" @click="renameChannelFromMenu">重命名</button>
        <button class="danger" type="button" @click="deleteChannelFromMenu">删除频道</button>
      </div>

      <footer class="desktop-user-card" aria-label="当前用户">
        <div class="avatar user-card-avatar">{{ session.user?.nickname?.slice(0, 1) || 'U' }}</div>
        <div class="desktop-user-info">
          <strong>{{ session.user?.nickname || session.user?.username }}</strong>
          <p>Online</p>
        </div>
        <div class="desktop-user-actions">
          <button type="button" title="静音" disabled>⌁</button>
          <button type="button" title="耳机" disabled>⌕</button>
          <button type="button" title="设置" disabled>⚙</button>
        </div>
      </footer>
    </aside>

    <aside v-else-if="activeFeature === 'drops'" class="feature-sidebar drops-sidebar">
      <header class="sidebar-header">
        <div>
          <p class="eyebrow">Async Feed</p>
          <h2>Drops</h2>
        </div>
        <button class="icon-button" type="button" title="刷新 Drops" @click="dropsStore.loadDrops">↻</button>
      </header>
      <div class="drop-tag-list">
        <button :class="['drop-tag', { active: !activeTag }]" type="button" @click="dropsStore.setActiveTag('')">
          All
        </button>
        <button
          v-for="tag in dropTags"
          :key="tag"
          :class="['drop-tag', { active: activeTag === tag }]"
          type="button"
          @click="dropsStore.setActiveTag(tag)"
        >
          #{{ tag }}
        </button>
        <p v-if="!dropTags.length" class="muted compact">暂无标签。</p>
      </div>
      <footer class="desktop-user-card" aria-label="当前用户">
        <div class="avatar user-card-avatar">{{ session.user?.nickname?.slice(0, 1) || 'U' }}</div>
        <div class="desktop-user-info">
          <strong>{{ session.user?.nickname || session.user?.username }}</strong>
          <p>Online</p>
        </div>
        <div class="desktop-user-actions">
          <button type="button" title="静音" disabled>⌁</button>
          <button type="button" title="耳机" disabled>⌕</button>
          <button type="button" title="设置" disabled>⚙</button>
        </div>
      </footer>
    </aside>

    <aside v-else-if="activeFeature === 'workspace'" class="feature-sidebar workspace-sidebar" @dragover.prevent @drop="handleWorkspaceDrop">
      <header class="sidebar-header">
        <div>
          <p class="eyebrow">Files</p>
          <h2>Workspace</h2>
        </div>
        <button class="icon-button" type="button" title="刷新文件" @click="workspaceStore.loadFolder()">↻</button>
      </header>
      <div class="workspace-actions">
        <button class="ghost-button small" type="button" @click="createWorkspaceFolder">新建文件夹</button>
        <button class="ghost-button small" type="button" @click="() => openWorkspaceFilePicker()">上传</button>
        <input ref="workspaceFileInput" class="hidden-file-input" type="file" multiple @change="handleWorkspaceFileChange" />
      </div>
      <div class="workspace-tree">
        <p v-if="workspaceRootLoading" class="muted compact">正在加载文件...</p>
        <WorkspaceTreeNode
          v-for="item in rootFiles"
          :key="item.id"
          :item="item"
          :children-by-parent-id="childrenByParentId"
          :expanded-folder-ids="expandedFolderIds"
          :selected-item-id="selectedItemId"
          :loading-folders="loadingFolders"
          @select="selectWorkspaceItem"
          @toggle="toggleWorkspaceFolder"
          @menu="openWorkspaceMenu"
          @drop-files="(folder, files) => workspaceStore.uploadFiles(files, folder.id)"
        />
        <p v-if="!workspaceRootLoading && rootFiles.length === 0" class="muted compact">拖拽本地文件到这里上传。</p>
      </div>
      <p v-if="workspaceUploading" class="muted compact">正在上传...</p>
      <p v-if="workspaceError" class="error-text compact">{{ workspaceError }}</p>

      <footer class="desktop-user-card" aria-label="当前用户">
        <div class="avatar user-card-avatar">{{ session.user?.nickname?.slice(0, 1) || 'U' }}</div>
        <div class="desktop-user-info">
          <strong>{{ session.user?.nickname || session.user?.username }}</strong>
          <p>Online</p>
        </div>
        <div class="desktop-user-actions">
          <button type="button" title="静音" disabled>⌁</button>
          <button type="button" title="耳机" disabled>⌕</button>
          <button type="button" title="设置" disabled>⚙</button>
        </div>
      </footer>

      <div
        v-if="workspaceMenu"
        class="context-menu"
        :style="{ left: `${workspaceMenu.x}px`, top: `${workspaceMenu.y}px` }"
        @click.stop
      >
        <button type="button" @click="renameWorkspaceItem">重命名</button>
        <button type="button" @click="moveWorkspaceItem">移动</button>
        <button v-if="workspaceMenu.target.is_folder" type="button" @click="uploadToWorkspaceMenuTarget">上传到此目录</button>
        <button v-if="!workspaceMenu.target.is_folder" type="button" @click="downloadWorkspaceItem">下载</button>
        <button class="danger" type="button" @click="deleteWorkspaceItem">删除</button>
      </div>
    </aside>

    <section v-if="activeFeature === 'home'" class="main-panel home-panel"></section>

    <section v-else-if="activeFeature === 'channels'" class="main-panel chat-panel">
      <header class="chat-header">
        <button class="mobile-back-button" type="button" @click="closeMobileMain">‹</button>
        <div class="channel-title-block">
          <p class="eyebrow">Channel</p>
          <div class="channel-title-line">
            <input
              v-if="isRenamingChannel"
              ref="renameInput"
              v-model="editingChannelName"
              class="channel-title-input"
              maxlength="100"
              @blur="submitRenameChannel"
              @keydown.enter.prevent="finishRenameChannel"
              @keydown.esc.prevent="cancelRenameChannel"
            />
            <h1 v-else>{{ selectedChannel ? `# ${selectedChannel.name}` : '选择或创建频道' }}</h1>
            <button class="icon-button" type="button" :disabled="!selectedChannel" title="重命名频道" @click="startRenameChannel">
              ✎
            </button>
          </div>
        </div>
        <div class="chat-header-actions">
          <input v-model="messageSearch" class="chat-search" placeholder="搜索当前消息" />
          <div class="members-popover-wrap">
            <button class="icon-button" type="button" title="在线成员">👥</button>
            <div class="members-popover">
              <strong>在线成员</strong>
              <p v-if="onlineMembers.length === 0" class="muted compact">暂无在线成员。</p>
              <div v-for="member in onlineMembers" :key="member.id" class="member-item">
                <span class="presence-dot online"></span>
                {{ member.nickname }}
              </div>
            </div>
          </div>
          <button class="icon-button" type="button" :disabled="!selectedChannel" title="刷新" @click="refreshCurrentChannel">↻</button>
        </div>
      </header>

      <div ref="messagesViewport" class="message-list" @scroll="handleMessagesScroll">
        <button v-if="!shouldStickToBottom" class="jump-bottom-button" type="button" @click="scrollMessagesToBottom">
          {{ unseenBottomMessages > 0 ? `${unseenBottomMessages} 条新消息，跳到底部` : '跳到底部' }}
        </button>
        <p v-if="loadingMessages" class="muted">正在加载消息...</p>
        <p v-else-if="selectedLoadingOlder" class="muted compact history-loading">正在加载更早消息...</p>
        <p v-else-if="!selectedChannel" class="empty-state">选择左侧频道，或新建一个频道。</p>
        <p v-else-if="selectedMessages.length === 0" class="empty-state">这个频道还没有消息。</p>
        <p v-else-if="messageSearch && visibleMessages.length === 0" class="empty-state">没有匹配的消息。</p>

        <article v-for="message in visibleMessages" :key="message.id" class="message-card">
          <div :class="['avatar small-avatar', { online: presenceByUser[message.sender.id] === 'online' }]">
            {{ message.sender.nickname.slice(0, 1) || '?' }}
          </div>
          <div class="message-body">
            <div class="message-meta">
              <strong>{{ message.sender.nickname }}</strong>
              <span>{{ formatTime(message.created_at) }}</span>
              <button class="message-action" type="button" @click="startReply(message)">回复</button>
            </div>
            <div v-if="message.reply_to_id" class="reply-quote">
              <template v-if="resolveReply(message)">
                回复 {{ resolveReply(message)?.sender.nickname }}：{{ messagePreview(resolveReply(message)!) }}
              </template>
              <template v-else>回复了一条较早的消息</template>
            </div>
            <pre v-if="message.msg_type === 'code'" class="code-message"><code>{{ message.content }}</code></pre>
            <img
              v-else-if="message.msg_type === 'image'"
              class="image-message"
              :src="message.content"
              alt="图片消息"
              loading="lazy"
              @click="openImagePreview(message.content, '图片消息')"
            />
            <ChatMarkdown v-else :content="message.content" @preview-image="openImagePreview" />
          </div>
        </article>
      </div>

      <p v-if="typingLabel" class="typing-indicator">{{ typingLabel }}</p>
      <div v-if="replyingTo" class="reply-preview">
        <div>
          <strong>回复 {{ replyingTo.sender.nickname }}</strong>
          <p>{{ messagePreview(replyingTo) }}</p>
        </div>
        <button type="button" @click="cancelReply">取消</button>
      </div>
      <div v-if="composerFiles.length" class="composer-attachments">
        <div v-for="file in composerFiles" :key="file.id" class="composer-attachment">
          <img v-if="file.localUrl" :src="file.localUrl" :alt="file.name" />
          <div v-else class="file-chip">{{ file.name.slice(0, 1).toUpperCase() }}</div>
          <div>
            <strong>{{ file.name }}</strong>
            <p>{{ formatBytes(file.size) }}</p>
          </div>
          <button type="button" @click="removeComposerFile(file.id)">移除</button>
        </div>
      </div>
      <form class="discord-composer" @submit.prevent="sendMessage">
        <input ref="composerFileInput" class="hidden-file-input" type="file" multiple @change="handleComposerFileChange" />
        <button class="composer-icon-button" type="button" title="添加附件" @click="openComposerFilePicker">+</button>
        <textarea
          v-model="composer"
          :disabled="!selectedChannel || sending"
          :placeholder="composerPlaceholder"
          rows="2"
          @input="handleComposerInput"
          @keydown="handleComposerKeydown"
          @paste="handleComposerPaste"
        ></textarea>
        <button class="composer-icon-button" type="button" title="占位功能" disabled>◎</button>
        <button class="composer-icon-button" type="button" title="占位功能" disabled>⌘</button>
        <button
          class="primary-button send-button"
          type="submit"
          :disabled="!selectedChannel || sending || (!composer.trim() && composerFiles.length === 0)"
        >
          {{ sending ? '发送中' : '发送' }}
        </button>
      </form>
      <p v-if="channelError" class="error-text inline-error">{{ channelError }}</p>
    </section>

    <section v-else-if="activeFeature === 'drops'" class="main-panel drops-panel">
      <header class="main-header">
        <div>
          <p class="eyebrow">Drops</p>
          <h1>{{ activeTag ? `#${activeTag}` : '全部动态' }}</h1>
        </div>
      </header>
      <form class="drop-composer" @submit.prevent="publishDrop">
        <textarea
          v-model="newDropContent"
          :disabled="dropsPublishing"
          placeholder="发布一个 Drop，支持 Markdown 和 #标签"
          rows="4"
        ></textarea>
        <div class="drop-composer-footer">
          <span class="muted compact">Markdown 内容会在发布后渲染为动态。</span>
          <button class="primary-button" type="submit" :disabled="dropsPublishing || !newDropContent.trim()">
            {{ dropsPublishing ? '发布中' : '发布 Drop' }}
          </button>
        </div>
      </form>
      <div class="drops-feed">
        <p v-if="dropsLoading" class="muted">正在加载 Drops...</p>
        <div v-else-if="dropsBackendUnavailable" class="empty-state">
          Drops 后端暂不可用，页面结构已预留。
          <span v-if="dropsError">{{ dropsError }}</span>
        </div>
        <p v-else-if="filteredPosts.length === 0" class="empty-state">暂无动态。</p>
        <article v-for="post in filteredPosts" :key="post.id" class="drop-card">
          <div class="drop-meta">
            <div class="avatar small-avatar">{{ post.author.nickname.slice(0, 1) || '?' }}</div>
            <div>
              <strong>{{ post.author.nickname }}</strong>
              <p>{{ formatDate(post.created_at) }}</p>
            </div>
            <button
              v-if="canDeleteDrop(post)"
              class="drop-delete-button"
              type="button"
              :disabled="dropsDeleting[post.id]"
              @click="deleteDrop(post)"
            >
              {{ dropsDeleting[post.id] ? '删除中' : '删除' }}
            </button>
          </div>
          <ChatMarkdown :content="post.content" @preview-image="openImagePreview" />
          <div class="drop-actions-row">
            <button type="button" :disabled="dropsLiking[post.id]" @click="dropsStore.toggleLike(post.id)">
              {{ post.is_liked_by_me ? '已赞' : '点赞' }} · {{ post.like_count }}
            </button>
            <span>评论 · {{ post.comment_count }}</span>
          </div>
          <div class="drop-comments">
            <article v-for="comment in post.comments ?? []" :key="comment.id" class="drop-comment">
              <div class="avatar small-avatar">{{ comment.author.nickname.slice(0, 1) || '?' }}</div>
              <div>
                <div class="drop-comment-meta">
                  <strong>{{ comment.author.nickname }}</strong>
                  <span>{{ formatDate(comment.created_at) }}</span>
                </div>
                <ChatMarkdown :content="comment.content" @preview-image="openImagePreview" />
              </div>
            </article>
            <form class="drop-comment-form" @submit.prevent="submitDropComment(post)">
              <input
                v-model="dropCommentDrafts[post.id]"
                :disabled="dropsCommenting[post.id]"
                placeholder="写下评论..."
              />
              <button class="ghost-button small" type="submit" :disabled="dropsCommenting[post.id] || !dropCommentDrafts[post.id]?.trim()">
                {{ dropsCommenting[post.id] ? '发送中' : '评论' }}
              </button>
            </form>
          </div>
        </article>
      </div>
    </section>

    <section v-else-if="activeFeature === 'workspace'" class="main-panel workspace-main">
      <header class="main-header workspace-main-header">
        <button class="mobile-back-button" type="button" @click="closeMobileMain">‹</button>
        <div>
          <p class="eyebrow">Workspace</p>
          <h1>{{ selectedWorkspaceItem?.name || '选择文件' }}</h1>
        </div>
        <div v-if="selectedWorkspaceItem" class="workspace-main-actions">
          <button class="ghost-button small" type="button" @click="renameWorkspaceItem">重命名</button>
          <button class="ghost-button small" type="button" @click="moveWorkspaceItem">移动</button>
          <button v-if="!selectedWorkspaceItem.is_folder" class="ghost-button small" type="button" @click="downloadWorkspaceItem">
            下载
          </button>
          <button class="ghost-button small danger" type="button" @click="deleteWorkspaceItem">删除</button>
        </div>
      </header>

      <div v-if="workspaceTabs.length" class="workspace-tab-strip">
        <button
          v-for="tab in workspaceTabs"
          :key="tab.id"
          :class="['workspace-tab', { active: activeWorkspaceTabId === tab.id, dirty: tab.editorContent !== tab.originalEditorContent }]"
          type="button"
          @click="activateWorkspaceTab(tab.id)"
        >
          <span class="workspace-tab-name">{{ tab.file.name }}</span>
          <span v-if="tab.editorContent !== tab.originalEditorContent" class="workspace-tab-dirty-dot"></span>
          <span v-if="tab.status === 'saving'" class="workspace-tab-status">保存中</span>
          <span v-else-if="tab.status === 'error'" class="workspace-tab-status error">错误</span>
          <span class="workspace-tab-close" role="button" tabindex="0" @click.stop="closeWorkspaceTab(tab.id)">×</span>
        </button>
      </div>

      <div class="workspace-preview" @dragover.prevent @drop="handleWorkspaceDrop">
        <p v-if="loadingPreview" class="muted">正在加载预览...</p>
        <div v-else-if="!selectedWorkspaceItem" class="empty-state">从左侧选择文件，或拖拽本地文件到 Workspace 上传。</div>
        <div v-else-if="workspacePreviewKind === 'image'" class="media-preview">
          <img :src="workspacePreviewUrl" :alt="selectedWorkspaceItem.name" @click="openImagePreview(workspacePreviewUrl, selectedWorkspaceItem.name)" />
        </div>
        <div v-else-if="workspacePreviewKind === 'video'" class="media-preview">
          <video :src="workspacePreviewUrl" controls></video>
        </div>
        <div v-else-if="workspacePreviewKind === 'text' || workspacePreviewKind === 'markdown'" class="workspace-editor">
          <div class="workspace-editor-toolbar">
            <span>{{ selectedWorkspaceItem.mime_type || 'text/plain' }}</span>
            <button class="primary-button" type="button" :disabled="!workspaceIsDirty || workspaceSaving" @click="saveWorkspaceContent">
              {{ workspaceSaving ? '保存中' : workspaceIsDirty ? '保存' : '已保存' }}
            </button>
          </div>
          <div :class="['workspace-editor-grid', { markdown: workspacePreviewKind === 'markdown' }]">
            <textarea v-model="editorContent" spellcheck="false"></textarea>
            <div v-if="workspacePreviewKind === 'markdown'" class="workspace-markdown-preview">
              <ChatMarkdown :content="editorContent" @preview-image="openImagePreview" />
            </div>
          </div>
        </div>
        <div v-else class="unsupported-preview">
          <h2>暂无法编辑和预览</h2>
          <p>{{ selectedWorkspaceItem.name }}</p>
          <p>{{ selectedWorkspaceItem.mime_type || '未知类型' }} · {{ formatBytes(selectedWorkspaceItem.size_bytes) }}</p>
          <button class="primary-button" type="button" @click="downloadWorkspaceItem">下载文件</button>
        </div>
      </div>
    </section>

    <nav class="mobile-tab-bar" aria-label="主导航">
      <button
        :class="['mobile-tab-button', { active: activeFeature === 'home' }]"
        type="button"
        @click="activateFeature('home')"
      >
        <span>U</span>
        主页
      </button>
      <button
        v-for="feature in features"
        :key="feature.id"
        :class="['mobile-tab-button', { active: activeFeature === feature.id }]"
        type="button"
        @click="activateFeature(feature.id)"
      >
        <span>{{ feature.icon }}</span>
        {{ feature.label }}
      </button>
    </nav>

    <div
      v-if="previewImage"
      ref="previewOverlay"
      class="image-preview-overlay"
      tabindex="0"
      @click="closeImagePreview"
      @wheel="handlePreviewWheel"
      @keydown="handlePreviewKeydown"
    >
      <div class="image-preview-stage">
        <img
          :src="previewImage.src"
          :alt="previewImage.alt || '图片预览'"
          :style="{ transform: `scale(${previewScale})` }"
          @click.stop="closeImagePreview"
        />
      </div>
    </div>
  </main>
</template>
