import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { RealtimeClient } from '../api/realtime'
import * as channelApi from '../api/channels'
import { useSessionStore } from './session'
import type {
  Channel,
  ChannelDeletedPayload,
  ChannelReadResponse,
  Message,
  MessageType,
  PongPayload,
  PresenceUpdatedPayload,
  RealtimeStatus,
  TypingStatusPayload,
  SendMessagePayload,
  WsErrorPayload,
  WsServerFrame,
} from '../api/types'

const TYPING_TTL_MS = 5000
const CLIENT_MESSAGE_ID_KEY = '_client_message_id'

type SendMessageOptions = {
  replyToId?: number | null
  msgType?: MessageType
  metadata?: Record<string, unknown>
}

export const useChannelsStore = defineStore('channels', () => {
  const channels = ref<Channel[]>([])
  const selectedChannelId = ref('')
  const messagesByChannel = ref<Record<string, Message[]>>({})
  const loadingChannels = ref(false)
  const loadingMessages = ref(false)
  const sending = ref(false)
  const error = ref('')
  const realtimeStatus = ref<RealtimeStatus>('idle')
  const realtimeError = ref('')
  const lastPongAt = ref('')
  const typingByChannel = ref<Record<string, Record<string, number>>>({})
  const presenceByUser = ref<Record<string, string>>({})
  const nextCursorByChannel = ref<Record<string, number>>({})
  const hasMoreByChannel = ref<Record<string, boolean>>({})
  const loadingOlderByChannel = ref<Record<string, boolean>>({})
  const unreadByChannel = ref<Record<string, number>>({})
  const typingExpiryTimers = new Map<string, number>()
  let realtimeClient: RealtimeClient | null = null

  const selectedChannel = computed(() =>
    channels.value.find((channel) => channel.id === selectedChannelId.value) ?? null,
  )
  const selectedMessages = computed(() => messagesByChannel.value[selectedChannelId.value] ?? [])
  const selectedHasMore = computed(() => hasMoreByChannel.value[selectedChannelId.value] ?? false)
  const selectedLoadingOlder = computed(() => loadingOlderByChannel.value[selectedChannelId.value] ?? false)
  const selectedTypingUserIds = computed(() => Object.keys(typingByChannel.value[selectedChannelId.value] ?? {}))
  const realtimeStatusLabel = computed(() => {
    if (realtimeStatus.value === 'connected') return '实时在线'
    if (realtimeStatus.value === 'connecting') return '连接中'
    if (realtimeStatus.value === 'reconnecting') return '重连中'
    if (realtimeStatus.value === 'error') return '连接异常'
    return '离线'
  })

  function sortChannels(nextChannels: Channel[]) {
    return [...nextChannels].sort((a, b) => a.created_at.localeCompare(b.created_at))
  }

  function createClientMessageId() {
    if ('crypto' in window && typeof window.crypto.randomUUID === 'function') {
      return window.crypto.randomUUID()
    }
    return `${Date.now()}-${Math.random().toString(16).slice(2)}`
  }

  function sortAndDedupeMessages(messages: Message[]) {
    const map = new Map<number, Message>()
    for (const message of messages) {
      map.set(message.id, message)
    }
    return [...map.values()].sort((a, b) => a.id - b.id)
  }

  function upsertChannel(channel: Channel) {
    const existingIndex = channels.value.findIndex((item) => item.id === channel.id)
    const nextChannels = [...channels.value]
    if (existingIndex >= 0) {
      const existing = nextChannels[existingIndex]
      nextChannels[existingIndex] = {
        ...channel,
        last_message_id: existing.last_message_id,
        last_read_message_id: existing.last_read_message_id,
        unread_count: existing.unread_count,
      }
    } else {
      nextChannels.push(channel)
    }
    channels.value = sortChannels(nextChannels)

    if (!selectedChannelId.value) {
      selectedChannelId.value = channel.id
      void loadMessages(channel.id)
    }
  }

  function removeChannel(channelId: string) {
    channels.value = channels.value.filter((channel) => channel.id !== channelId)
    const nextMessages = { ...messagesByChannel.value }
    delete nextMessages[channelId]
    messagesByChannel.value = nextMessages

    const nextTyping = { ...typingByChannel.value }
    delete nextTyping[channelId]
    typingByChannel.value = nextTyping

    const nextCursor = { ...nextCursorByChannel.value }
    delete nextCursor[channelId]
    nextCursorByChannel.value = nextCursor

    const nextHasMore = { ...hasMoreByChannel.value }
    delete nextHasMore[channelId]
    hasMoreByChannel.value = nextHasMore

    const nextLoadingOlder = { ...loadingOlderByChannel.value }
    delete nextLoadingOlder[channelId]
    loadingOlderByChannel.value = nextLoadingOlder

    const nextUnread = { ...unreadByChannel.value }
    delete nextUnread[channelId]
    unreadByChannel.value = nextUnread

    if (selectedChannelId.value === channelId) {
      selectedChannelId.value = channels.value[0]?.id ?? ''
      if (selectedChannelId.value) {
        void loadMessages(selectedChannelId.value)
      }
    }
  }

  function mergeMessages(channelId: string, incomingMessages: Message[], mode: 'replace' | 'append' | 'prepend' | 'merge') {
    const currentMessages = messagesByChannel.value[channelId] ?? []
    const nextMessages =
      mode === 'replace'
        ? incomingMessages
        : mode === 'prepend'
          ? [...incomingMessages, ...currentMessages]
          : [...currentMessages, ...incomingMessages]

    messagesByChannel.value = {
      ...messagesByChannel.value,
      [channelId]: sortAndDedupeMessages(nextMessages),
    }
  }

  function insertMessage(message: Message) {
    mergeMessages(message.channel_id, [message], 'append')
  }

  function patchChannel(channelId: string, patch: Partial<Channel>) {
    channels.value = channels.value.map((channel) => (channel.id === channelId ? { ...channel, ...patch } : channel))
  }

  function latestMessageId(channelId: string) {
    const messages = messagesByChannel.value[channelId] ?? []
    return messages.reduce((latest, message) => Math.max(latest, message.id), 0)
  }

  function incrementUnread(channelId: string) {
    const unreadCount = (unreadByChannel.value[channelId] ?? 0) + 1
    unreadByChannel.value = {
      ...unreadByChannel.value,
      [channelId]: unreadCount,
    }
    patchChannel(channelId, { unread_count: unreadCount })
  }

  function clearUnread(channelId: string) {
    if (!unreadByChannel.value[channelId]) {
      return
    }
    unreadByChannel.value = {
      ...unreadByChannel.value,
      [channelId]: 0,
    }
    patchChannel(channelId, { unread_count: 0 })
  }

  function setPagination(channelId: string, nextCursor: number, hasMore: boolean) {
    nextCursorByChannel.value = {
      ...nextCursorByChannel.value,
      [channelId]: nextCursor,
    }
    hasMoreByChannel.value = {
      ...hasMoreByChannel.value,
      [channelId]: hasMore,
    }
  }

  function setLoadingOlder(channelId: string, loading: boolean) {
    loadingOlderByChannel.value = {
      ...loadingOlderByChannel.value,
      [channelId]: loading,
    }
  }

  function typingTimerKey(channelId: string, userId: string) {
    return `${channelId}:${userId}`
  }

  function clearTypingTimer(channelId: string, userId: string) {
    const key = typingTimerKey(channelId, userId)
    const timer = typingExpiryTimers.get(key)
    if (timer !== undefined) {
      window.clearTimeout(timer)
      typingExpiryTimers.delete(key)
    }
  }

  function removeTypingUser(channelId: string, userId: string) {
    clearTypingTimer(channelId, userId)
    const currentTyping = typingByChannel.value[channelId]
    if (!currentTyping?.[userId]) {
      return
    }

    const nextChannelTyping = { ...currentTyping }
    delete nextChannelTyping[userId]
    typingByChannel.value = {
      ...typingByChannel.value,
      [channelId]: nextChannelTyping,
    }
  }

  function clearTypingTimers() {
    for (const timer of typingExpiryTimers.values()) {
      window.clearTimeout(timer)
    }
    typingExpiryTimers.clear()
  }

  function handleTypingStatus(payload: TypingStatusPayload) {
    const session = useSessionStore()
    if (payload.user_id === session.user?.id) {
      return
    }

    if (!payload.is_typing) {
      removeTypingUser(payload.channel_id, payload.user_id)
      return
    }

    clearTypingTimer(payload.channel_id, payload.user_id)
    typingByChannel.value = {
      ...typingByChannel.value,
      [payload.channel_id]: {
        ...(typingByChannel.value[payload.channel_id] ?? {}),
        [payload.user_id]: Date.now() + TYPING_TTL_MS,
      },
    }

    const key = typingTimerKey(payload.channel_id, payload.user_id)
    typingExpiryTimers.set(
      key,
      window.setTimeout(() => removeTypingUser(payload.channel_id, payload.user_id), TYPING_TTL_MS),
    )
  }

  function handlePresenceUpdated(payload: PresenceUpdatedPayload) {
    presenceByUser.value = {
      ...presenceByUser.value,
      [payload.user_id]: payload.status,
    }
  }

  function handleChannelReadUpdated(payload: ChannelReadResponse) {
    patchChannel(payload.channel_id, {
      last_read_message_id: payload.last_read_message_id,
      unread_count: payload.unread_count,
    })
    unreadByChannel.value = {
      ...unreadByChannel.value,
      [payload.channel_id]: payload.unread_count,
    }
  }

  function handleIncomingMessage(message: Message, countUnread: boolean) {
    const session = useSessionStore()
    insertMessage(message)
    patchChannel(message.channel_id, { last_message_id: message.id })
    if (message.sender_id === session.user?.id) {
      patchChannel(message.channel_id, { last_read_message_id: message.id, unread_count: 0 })
      clearUnread(message.channel_id)
    } else if (countUnread && message.channel_id !== selectedChannelId.value) {
      incrementUnread(message.channel_id)
    }
  }

  function handleRealtimeFrame(frame: WsServerFrame) {
    switch (frame.event) {
      case 'new_message':
        handleIncomingMessage(frame.data as Message, true)
        break
      case 'message_sent':
        handleIncomingMessage(frame.data as Message, false)
        break
      case 'channel_created':
      case 'channel_updated':
        upsertChannel(frame.data as Channel)
        break
      case 'channel_deleted':
        removeChannel((frame.data as ChannelDeletedPayload).id)
        break
      case 'channel_read_updated':
        handleChannelReadUpdated(frame.data as ChannelReadResponse)
        break
      case 'typing_status':
        handleTypingStatus(frame.data as TypingStatusPayload)
        break
      case 'presence_updated':
        handlePresenceUpdated(frame.data as PresenceUpdatedPayload)
        break
      case 'pong':
        if ((frame.data as PongPayload).ok) {
          lastPongAt.value = new Date().toISOString()
        }
        break
      case 'error':
        realtimeError.value = (frame.data as WsErrorPayload).msg
        break
      case 'agent_message':
      case 'agent_delta':
        break
    }
  }

  function syncUnreadFromChannels() {
    const nextUnread: Record<string, number> = {}
    for (const channel of channels.value) {
      nextUnread[channel.id] = channel.unread_count ?? 0
    }
    unreadByChannel.value = nextUnread
  }

  async function loadChannels() {
    loadingChannels.value = true
    error.value = ''

    try {
      channels.value = sortChannels(await channelApi.listChannels())
      syncUnreadFromChannels()
      if (!selectedChannelId.value && channels.value.length > 0) {
        await selectChannel(channels.value[0].id)
      }
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '加载频道失败'
      throw caught
    } finally {
      loadingChannels.value = false
    }
  }

  async function createChannel(name: string) {
    const trimmedName = name.trim()
    if (!trimmedName) {
      return
    }

    const channel = await channelApi.createChannel(trimmedName)
    upsertChannel(channel)
    await selectChannel(channel.id)
  }

  async function renameChannel(channelId: string, name: string) {
    const trimmedName = name.trim()
    if (!channelId || !trimmedName) {
      return false
    }

    error.value = ''
    try {
      upsertChannel(await channelApi.updateChannel(channelId, trimmedName))
      return true
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '重命名频道失败'
      throw caught
    }
  }

  async function deleteChannel(channelId: string) {
    if (!channelId) {
      return false
    }

    error.value = ''
    try {
      await channelApi.deleteChannel(channelId)
      removeChannel(channelId)
      return true
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '删除频道失败'
      throw caught
    }
  }

  async function selectChannel(channelId: string) {
    selectedChannelId.value = channelId
    clearUnread(channelId)
    await loadMessages(channelId)
    await markChannelRead(channelId)
  }

  async function loadMessages(channelId = selectedChannelId.value, showLoading = true) {
    if (!channelId) {
      return
    }

    if (showLoading) {
      loadingMessages.value = true
    }
    error.value = ''

    try {
      const result = await channelApi.listMessages(channelId, 0, 50)
      mergeMessages(channelId, result.messages, 'replace')
      patchChannel(channelId, { last_message_id: latestMessageId(channelId) })
      setPagination(channelId, result.next_cursor, result.has_more)
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '加载消息失败'
      throw caught
    } finally {
      if (showLoading) {
        loadingMessages.value = false
      }
    }
  }

  async function loadOlderMessages(channelId = selectedChannelId.value) {
    if (!channelId || !hasMoreByChannel.value[channelId] || loadingOlderByChannel.value[channelId]) {
      return
    }

    setLoadingOlder(channelId, true)
    error.value = ''

    try {
      const result = await channelApi.listMessages(channelId, nextCursorByChannel.value[channelId] ?? 0, 50)
      mergeMessages(channelId, result.messages, 'prepend')
      setPagination(channelId, result.next_cursor, result.has_more)
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '加载历史消息失败'
      throw caught
    } finally {
      setLoadingOlder(channelId, false)
    }
  }

  async function markChannelRead(channelId = selectedChannelId.value, lastReadMessageId = latestMessageId(channelId)) {
    if (!channelId || lastReadMessageId <= 0) {
      return false
    }

    try {
      const read = await channelApi.markChannelRead(channelId, lastReadMessageId)
      patchChannel(channelId, {
        last_read_message_id: read.last_read_message_id,
        unread_count: read.unread_count,
      })
      unreadByChannel.value = {
        ...unreadByChannel.value,
        [channelId]: read.unread_count,
      }
      return true
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '更新已读位置失败'
      return false
    }
  }

  function sendTyping(isTyping: boolean, channelId = selectedChannelId.value) {
    if (!channelId || !realtimeClient || realtimeStatus.value !== 'connected') {
      return false
    }

    try {
      realtimeClient.send('typing', {
        channel_id: channelId,
        is_typing: isTyping,
      })
      return true
    } catch (caught) {
      realtimeError.value = caught instanceof Error ? caught.message : '发送输入状态失败'
      return false
    }
  }

  async function sendMessage(content: string, options: SendMessageOptions = {}) {
    const trimmedContent = content.trim()
    if (!selectedChannelId.value || !trimmedContent) {
      return false
    }

    const payload: SendMessagePayload = {
      channel_id: selectedChannelId.value,
      reply_to_id: options.replyToId ?? null,
      msg_type: options.msgType ?? 'text',
      content: trimmedContent,
      metadata: options.metadata ?? {},
    }

    sending.value = true
    error.value = ''

    try {
      if (realtimeClient && realtimeStatus.value === 'connected') {
        const session = useSessionStore()
        const clientMessageId = createClientMessageId()
        const realtimePayload = {
          ...payload,
          metadata: {
            ...payload.metadata,
            [CLIENT_MESSAGE_ID_KEY]: clientMessageId,
          },
        }
        await realtimeClient.sendAndWait('send_message', realtimePayload, 'message_sent', (frame) => {
          if (frame.event !== 'new_message') {
            return false
          }
          const message = frame.data as Message
          return (
            message.channel_id === realtimePayload.channel_id &&
            message.sender_id === session.user?.id &&
            message.metadata?.[CLIENT_MESSAGE_ID_KEY] === clientMessageId
          )
        })
      } else {
        const message = await channelApi.createMessage(selectedChannelId.value, {
          reply_to_id: payload.reply_to_id,
          msg_type: payload.msg_type,
          content: payload.content,
          metadata: payload.metadata,
        })
        insertMessage(message)
      }
      return true
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : '发送消息失败'
      throw caught
    } finally {
      sending.value = false
    }
  }

  function connectRealtime(serverUrl: string, accessToken: string) {
    if (!serverUrl || !accessToken) {
      return
    }

    disconnectRealtime()
    realtimeError.value = ''
    realtimeClient = new RealtimeClient(serverUrl, accessToken, {
      onStatusChange: (status) => {
        realtimeStatus.value = status
      },
      onFrame: handleRealtimeFrame,
      onError: (message) => {
        realtimeError.value = message
      },
    })
    realtimeClient.connect()
  }

  function disconnectRealtime() {
    realtimeClient?.disconnect()
    realtimeClient = null
    realtimeStatus.value = 'disconnected'
    clearTypingTimers()
  }

  function reset() {
    disconnectRealtime()
    channels.value = []
    selectedChannelId.value = ''
    messagesByChannel.value = {}
    clearTypingTimers()
    typingByChannel.value = {}
    presenceByUser.value = {}
    nextCursorByChannel.value = {}
    hasMoreByChannel.value = {}
    loadingOlderByChannel.value = {}
    unreadByChannel.value = {}
    realtimeError.value = ''
    lastPongAt.value = ''
    error.value = ''
  }

  return {
    channels,
    selectedChannelId,
    selectedChannel,
    selectedMessages,
    loadingChannels,
    loadingMessages,
    sending,
    error,
    selectedHasMore,
    selectedLoadingOlder,
    selectedTypingUserIds,
    realtimeStatus,
    realtimeStatusLabel,
    realtimeError,
    lastPongAt,
    typingByChannel,
    presenceByUser,
    unreadByChannel,
    loadChannels,
    createChannel,
    renameChannel,
    deleteChannel,
    selectChannel,
    loadMessages,
    loadOlderMessages,
    markChannelRead,
    sendMessage,
    sendTyping,
    connectRealtime,
    disconnectRealtime,
    reset,
  }
})
