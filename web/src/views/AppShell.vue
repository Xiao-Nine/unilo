<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import ChatMarkdown from '../components/ChatMarkdown.vue'
import { uploadWorkspaceFile } from '../api/workspace'
import { useChannelsStore } from '../stores/channels'
import { useSessionStore } from '../stores/session'
import { useThemeStore } from '../stores/theme'
import type { Message, MessageAttachment, WorkspaceFile } from '../api/types'

const router = useRouter()
const session = useSessionStore()
const theme = useThemeStore()
const channelsStore = useChannelsStore()
const {
  channels,
  selectedChannel,
  selectedMessages,
  loadingChannels,
  loadingMessages,
  sending,
  error,
  selectedHasMore,
  selectedLoadingOlder,
  selectedTypingUserIds,
  realtimeStatusLabel,
  realtimeError,
  presenceByUser,
  unreadByChannel,
} = storeToRefs(channelsStore)

type PastedImage = {
  id: string
  file: File
  localUrl: string
  name: string
  size: number
  mimeType: string
}

const newChannelName = ref('')
const editingChannelName = ref('')
const isRenamingChannel = ref(false)
const renameInput = ref<HTMLInputElement | null>(null)
const channelMenu = ref<{ channelId: string; x: number; y: number } | null>(null)
const composer = ref('')
const pastedImages = ref<PastedImage[]>([])
const replyingTo = ref<Message | null>(null)
const activeModule = ref('channels')
const messagesViewport = ref<HTMLElement | null>(null)
const shouldStickToBottom = ref(true)
const suppressHistoryLoad = ref(false)
const unseenBottomMessages = ref(0)
const previewImage = ref<{ src: string; alt: string } | null>(null)
const previewScale = ref(1)
const previewOverlay = ref<HTMLElement | null>(null)
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
  if (names.length === 0) {
    return ''
  }
  if (names.length === 1) {
    return `${names[0]} 正在输入...`
  }
  return `${names.slice(0, 2).join('、')} 等正在输入...`
})

const selectedMembers = computed(() => {
  const members = new Map<string, { id: string; nickname: string; avatar_url: string }>()
  for (const message of selectedMessages.value) {
    members.set(message.sender.id, message.sender)
  }
  return [...members.values()]
})

const messagesById = computed(() => {
  const messages = new Map<number, Message>()
  for (const message of selectedMessages.value) {
    messages.set(message.id, message)
  }
  return messages
})

const composerPlaceholder = '输入 Markdown 消息，Enter 发送，Shift+Enter 换行；可直接粘贴图片'

const modules = [
  { id: 'channels', label: '频道', icon: '#' },
  { id: 'drops', label: '动态', icon: 'D' },
  { id: 'workspace', label: '文件', icon: 'F' },
  { id: 'agent', label: 'AI', icon: 'A' },
  { id: 'search', label: '搜索', icon: 'S' },
]

function formatTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
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
  if (!event.ctrlKey) {
    return
  }
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
  if (message.msg_type === 'image' || message.metadata.attachments?.length) {
    return '[图片]'
  }
  if (message.msg_type === 'code') {
    return content.split('\n')[0] || '[代码]'
  }
  return content
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '[图片]')
    .replace(/[#*_`>\-[\]()]/g, '')
    .trim()
    .split('\n')[0] || '[消息]'
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

async function createChannel() {
  await channelsStore.createChannel(newChannelName.value)
  newChannelName.value = ''
}

async function startRenameChannel() {
  if (!selectedChannel.value) {
    return
  }
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
  if (!isRenamingChannel.value || !selectedChannel.value) {
    return
  }

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
  channelMenu.value = { channelId, x: event.clientX, y: event.clientY }
}

function closeChannelMenu() {
  channelMenu.value = null
}

async function deleteChannelFromMenu() {
  const channelId = channelMenu.value?.channelId
  const channel = channels.value.find((item) => item.id === channelId)
  closeChannelMenu()
  if (!channelId || !channel) {
    return
  }

  const confirmed = window.confirm(`确定删除频道 #${channel.name} 吗？`)
  if (!confirmed) {
    return
  }
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
  if (!selectedChannel.value) {
    return
  }

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
  if (event.key !== 'Enter' || event.shiftKey) {
    return
  }
  event.preventDefault()
  void sendMessage()
}

function revokePastedImage(image: PastedImage) {
  URL.revokeObjectURL(image.localUrl)
}

function clearPastedImages() {
  for (const image of pastedImages.value) {
    revokePastedImage(image)
  }
  pastedImages.value = []
}

function removePastedImage(imageId: string) {
  const image = pastedImages.value.find((item) => item.id === imageId)
  if (image) {
    revokePastedImage(image)
  }
  pastedImages.value = pastedImages.value.filter((item) => item.id !== imageId)
}

function handleComposerPaste(event: ClipboardEvent) {
  const items = [...(event.clipboardData?.items ?? [])]
  const images = items
    .filter((item) => item.kind === 'file')
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file && file.type.startsWith('image/')))

  if (images.length === 0) {
    return
  }

  event.preventDefault()
  pastedImages.value = [
    ...pastedImages.value,
    ...images.map((file) => ({
      id: crypto.randomUUID(),
      file,
      localUrl: URL.createObjectURL(file),
      name: file.name || `粘贴图片-${Date.now()}.png`,
      size: file.size,
      mimeType: file.type,
    })),
  ]
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
    parts.push(`![${attachment.name}](${attachment.preview_url})`)
  }
  return parts.filter(Boolean).join('\n\n')
}

async function sendMessage() {
  if (!composer.value.trim() && pastedImages.value.length === 0) {
    return
  }

  const uploadedFiles = await Promise.all(pastedImages.value.map((image) => uploadWorkspaceFile(image.file)))
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
    clearPastedImages()
    shouldStickToBottom.value = true
    await scrollMessagesToBottom()
  }
}

function isNearBottom() {
  const viewport = messagesViewport.value
  if (!viewport) {
    return true
  }
  return viewport.scrollHeight - viewport.scrollTop - viewport.clientHeight < 120
}

function markCurrentChannelRead() {
  if (!selectedChannel.value || !shouldStickToBottom.value) {
    return
  }
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
  if (!selectedChannel.value) {
    return
  }

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
    void scrollMessagesToBottom()
  },
)

onMounted(async () => {
  session.syncApi()
  await channelsStore.loadChannels()
  channelsStore.connectRealtime(session.serverUrl, session.accessToken)
  await scrollMessagesToBottom()
})

onBeforeUnmount(() => {
  stopTyping()
  clearPastedImages()
  channelsStore.disconnectRealtime()
})
</script>

<template>
  <main class="app-shell">
    <aside class="module-rail">
      <div class="brand-mark">U</div>
      <button
        v-for="module in modules"
        :key="module.id"
        :class="['module-button', { active: activeModule === module.id }]"
        type="button"
        :title="module.label"
        @click="activeModule = module.id"
      >
        {{ module.icon }}
      </button>
      <button class="module-button" type="button" :title="`切换主题：${theme.label}`" @click="theme.toggle">
        {{ theme.mode === 'dark' ? '亮' : '暗' }}
      </button>
      <button class="module-button bottom" type="button" title="退出" @click="logout">↩</button>
    </aside>

    <aside class="channel-panel">
      <header class="panel-header">
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

      <div class="channel-list" @click="closeChannelMenu">
        <div
          v-for="channel in channels"
          :key="channel.id"
          :class="['channel-row', { active: selectedChannel?.id === channel.id }]"
          @contextmenu="openChannelMenu(channel.id, $event)"
        >
          <button class="channel-item" type="button" @click="channelsStore.selectChannel(channel.id)">
            <span>#</span>
            <span class="channel-name">{{ channel.name }}</span>
            <span v-if="unreadByChannel[channel.id]" class="unread-badge">
              {{ unreadByChannel[channel.id] > 99 ? '99+' : unreadByChannel[channel.id] }}
            </span>
          </button>
          <button class="channel-menu-trigger" type="button" title="频道选项" @click="openChannelMenu(channel.id, $event)">
            ⋯
          </button>
        </div>
        <p v-if="loadingChannels" class="muted compact">正在加载频道...</p>
        <p v-else-if="channels.length === 0" class="muted compact">还没有频道，创建一个开始聊天。</p>
      </div>
      <div
        v-if="channelMenu"
        class="channel-context-menu"
        :style="{ left: `${channelMenu.x}px`, top: `${channelMenu.y}px` }"
        @click.stop
      >
        <button type="button" @click="deleteChannelFromMenu">删除频道</button>
      </div>

      <footer class="user-card">
        <div class="avatar">{{ session.user?.nickname?.slice(0, 1) || 'U' }}</div>
        <div>
          <strong>{{ session.user?.nickname || session.user?.username }}</strong>
          <p>{{ session.user?.username }}</p>
        </div>
      </footer>
    </aside>

    <section class="chat-panel">
      <header class="chat-header">
        <div class="channel-title-block">
          <p class="eyebrow">Channel</p>
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
          <h1 v-else @dblclick="startRenameChannel">
            {{ selectedChannel ? `# ${selectedChannel.name}` : '选择或创建频道' }}
          </h1>
        </div>
        <button class="ghost-button small" type="button" :disabled="!selectedChannel" @click="refreshCurrentChannel">
          刷新
        </button>
      </header>

      <div ref="messagesViewport" class="message-list" @scroll="handleMessagesScroll">
        <button
          v-if="!shouldStickToBottom"
          class="jump-bottom-button"
          type="button"
          @click="scrollMessagesToBottom"
        >
          {{ unseenBottomMessages > 0 ? `${unseenBottomMessages} 条新消息，跳到底部` : '跳到底部' }}
        </button>
        <p v-if="loadingMessages" class="muted">正在加载消息...</p>
        <p v-else-if="selectedLoadingOlder" class="muted compact history-loading">正在加载更早消息...</p>
        <p v-else-if="!selectedChannel" class="empty-state">选择左侧频道，或新建一个频道。</p>
        <p v-else-if="selectedMessages.length === 0" class="empty-state">这个频道还没有消息。</p>

        <article v-for="message in selectedMessages" :key="message.id" class="message-card">
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
      <div v-if="pastedImages.length" class="composer-attachments">
        <div v-for="image in pastedImages" :key="image.id" class="composer-attachment">
          <img :src="image.localUrl" :alt="image.name" />
          <div>
            <strong>{{ image.name }}</strong>
            <p>{{ Math.ceil(image.size / 1024) }} KB</p>
          </div>
          <button type="button" @click="removePastedImage(image.id)">移除</button>
        </div>
      </div>
      <form class="composer" @submit.prevent="sendMessage">
        <textarea
          v-model="composer"
          :disabled="!selectedChannel || sending"
          :placeholder="composerPlaceholder"
          rows="2"
          @input="handleComposerInput"
          @keydown="handleComposerKeydown"
          @paste="handleComposerPaste"
        ></textarea>
        <button
          class="primary-button"
          type="submit"
          :disabled="!selectedChannel || sending || (!composer.trim() && pastedImages.length === 0)"
        >
          {{ sending ? '发送中' : '发送' }}
        </button>
      </form>
      <p v-if="error" class="error-text inline-error">{{ error }}</p>
    </section>

    <aside class="right-dock">
      <section class="dock-card accent-card">
        <p class="eyebrow">Backend</p>
        <h3>ali6 已连接</h3>
        <p>{{ session.serverUrl }}</p>
      </section>
      <section class="dock-card">
        <p class="eyebrow">Drops</p>
        <h3>异步动态</h3>
        <p>下一阶段接入 Markdown 动态、点赞和评论。</p>
      </section>
      <section class="dock-card">
        <p class="eyebrow">Workspace</p>
        <h3>共享文件</h3>
        <p>预留文件树、上传、预览和回收站入口。</p>
      </section>
      <section class="dock-card">
        <p class="eyebrow">Members</p>
        <h3>频道成员</h3>
        <div v-if="selectedMembers.length" class="member-list">
          <div v-for="member in selectedMembers" :key="member.id" class="member-item">
            <span :class="['presence-dot', { online: presenceByUser[member.id] === 'online' }]"></span>
            {{ member.nickname }}
          </div>
        </div>
        <p v-else>当前频道还没有可展示成员。</p>
      </section>
      <section class="dock-card terminal-card">
        <p class="eyebrow">AI Assistant</p>
        <pre>context: {{ selectedChannel?.name || 'none' }}
status: standby
stream: pending</pre>
      </section>
    </aside>

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
