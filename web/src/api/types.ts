export type ApiResponse<T> = {
  code: number
  msg: string
  data: T
}

export type User = {
  id: string
  username: string
  nickname: string
  avatar_url: string
  created_at?: string
}

export type VerifyResponse = {
  is_valid: boolean
  server_name: string
  server_version: string
  api_base_url: string
}

export type LoginResponse = {
  access_token: string
  refresh_token: string
  access_token_expires_at: string
  refresh_token_expires_at: string
  user: User
}

export type RefreshResponse = {
  access_token: string
  refresh_token: string
  access_token_expires_at: string
  refresh_token_expires_at: string
}

export type Channel = {
  id: string
  name: string
  created_by: string
  created_at: string
  updated_at: string
  last_message_id: number
  last_read_message_id: number
  unread_count: number
}

export type MessageType = 'text' | 'code' | 'image' | 'file' | 'agent' | string

export type MessageAttachment = {
  file_id: string
  name: string
  mime_type: string
  size_bytes: number
  preview_url: string
  download_url: string
}

export type MessageMetadata = Record<string, unknown> & {
  attachments?: MessageAttachment[]
}

export type Message = {
  id: number
  channel_id: string
  sender_id: string
  sender: {
    id: string
    nickname: string
    avatar_url: string
  }
  reply_to_id: number | null
  msg_type: MessageType
  content: string
  metadata: MessageMetadata
  created_at: string
}

export type ListMessagesResponse = {
  messages: Message[]
  next_cursor: number
  has_more: boolean
}

export type WsClientAction = 'ping' | 'send_message' | 'typing' | 'invoke_agent'

export type WsServerEvent =
  | 'new_message'
  | 'channel_created'
  | 'channel_updated'
  | 'channel_deleted'
  | 'channel_read_updated'
  | 'typing_status'
  | 'presence_updated'
  | 'pong'
  | 'error'
  | 'agent_message'
  | 'agent_delta'

export type WsClientFrame<TPayload = unknown> = {
  action: WsClientAction
  request_id?: string
  payload: TPayload
}

export type WsServerFrame<TData = unknown> = {
  event: WsServerEvent
  request_id?: string | null
  data: TData
}

export type SendMessagePayload = {
  channel_id: string
  reply_to_id: number | null
  msg_type: MessageType
  content: string
  metadata: MessageMetadata
}

export type ChannelReadResponse = {
  channel_id: string
  last_read_message_id: number
  unread_count: number
}

export type WorkspaceFile = {
  id: string
  parent_id: string | null
  is_folder: boolean
  name: string
  uploader_id: string
  size_bytes: number
  mime_type: string
  file_hash: string
  metadata: Record<string, unknown>
  preview_url: string
  download_url: string
  created_at: string
  updated_at: string
}

export type TypingStatusPayload = {
  channel_id: string
  user_id: string
  is_typing: boolean
}

export type PresenceUpdatedPayload = {
  user_id: string
  status: string
}

export type ChannelDeletedPayload = {
  id: string
}

export type PongPayload = {
  ok: boolean
}

export type WsErrorPayload = {
  code: number
  msg: string
}

export type RealtimeStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'disconnected' | 'error'

export type ApiErrorPayload = {
  code: number
  msg: string
  status: number
}
