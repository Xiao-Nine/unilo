import { request } from './client'
import type { Channel, ChannelReadResponse, ListMessagesResponse, Message, SendMessagePayload } from './types'

export function listChannels() {
  return request<Channel[]>('/channels')
}

export function createChannel(name: string) {
  return request<Channel>('/channels', {
    method: 'POST',
    body: { name },
  })
}

export function updateChannel(channelId: string, name: string) {
  return request<Channel>(`/channels/${channelId}`, {
    method: 'PATCH',
    body: { name },
  })
}

export function deleteChannel(channelId: string) {
  return request<{ deleted: boolean }>(`/channels/${channelId}`, {
    method: 'DELETE',
  })
}

export function markChannelRead(channelId: string, lastReadMessageId: number) {
  return request<ChannelReadResponse>(`/channels/${channelId}/read`, {
    method: 'POST',
    body: { last_read_message_id: lastReadMessageId },
  })
}

export function listMessages(channelId: string, cursor = 0, limit = 50) {
  const params = new URLSearchParams({ limit: String(limit) })
  if (cursor > 0) {
    params.set('cursor', String(cursor))
  }

  return request<ListMessagesResponse>(`/channels/${channelId}/messages?${params.toString()}`)
}

export function createMessage(channelId: string, payload: Omit<SendMessagePayload, 'channel_id'>) {
  return request<Message>(`/channels/${channelId}/messages`, {
    method: 'POST',
    body: payload,
  })
}
