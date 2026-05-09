import { websocketUrl } from './client'
import type { RealtimeStatus, WsClientAction, WsClientFrame, WsServerFrame } from './types'

type RealtimeCallbacks = {
  onStatusChange: (status: RealtimeStatus) => void
  onFrame: (frame: WsServerFrame) => void
  onError: (message: string) => void
}

const RECONNECT_DELAYS = [1000, 2000, 5000, 10000, 15000]
const HEARTBEAT_INTERVAL = 30000

function requestId() {
  if ('crypto' in window && typeof window.crypto.randomUUID === 'function') {
    return window.crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export class RealtimeClient {
  private readonly serverUrl: string
  private readonly accessToken: string
  private readonly callbacks: RealtimeCallbacks
  private socket: WebSocket | null = null
  private heartbeatTimer: number | undefined
  private reconnectTimer: number | undefined
  private reconnectAttempt = 0
  private closedByUser = false

  constructor(serverUrl: string, accessToken: string, callbacks: RealtimeCallbacks) {
    this.serverUrl = serverUrl
    this.accessToken = accessToken
    this.callbacks = callbacks
  }

  connect() {
    this.closedByUser = false
    this.open(this.reconnectAttempt > 0 ? 'reconnecting' : 'connecting')
  }

  disconnect() {
    this.closedByUser = true
    this.clearTimers()
    this.socket?.close()
    this.socket = null
    this.callbacks.onStatusChange('disconnected')
  }

  send<TPayload>(action: WsClientAction, payload: TPayload, frameRequestId = requestId()) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      throw new Error('实时连接未建立')
    }

    const frame: WsClientFrame<TPayload> = {
      action,
      request_id: frameRequestId,
      payload,
    }
    this.socket.send(JSON.stringify(frame))
    return frameRequestId
  }

  private open(status: RealtimeStatus) {
    this.callbacks.onStatusChange(status)
    this.socket?.close()
    this.socket = new WebSocket(websocketUrl(this.serverUrl, this.accessToken))

    this.socket.addEventListener('open', () => {
      this.reconnectAttempt = 0
      this.callbacks.onStatusChange('connected')
      this.startHeartbeat()
    })

    this.socket.addEventListener('message', (event) => {
      try {
        this.callbacks.onFrame(JSON.parse(event.data as string) as WsServerFrame)
      } catch {
        this.callbacks.onError('实时消息解析失败')
      }
    })

    this.socket.addEventListener('error', () => {
      this.callbacks.onStatusChange('error')
      this.callbacks.onError('实时连接发生错误')
    })

    this.socket.addEventListener('close', () => {
      this.stopHeartbeat()
      if (this.closedByUser) {
        this.callbacks.onStatusChange('disconnected')
        return
      }
      this.scheduleReconnect()
    })
  }

  private startHeartbeat() {
    this.stopHeartbeat()
    this.heartbeatTimer = window.setInterval(() => {
      try {
        this.send('ping', {})
      } catch {
        this.socket?.close()
      }
    }, HEARTBEAT_INTERVAL)
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      window.clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = undefined
    }
  }

  private scheduleReconnect() {
    this.callbacks.onStatusChange('reconnecting')
    const delay = RECONNECT_DELAYS[Math.min(this.reconnectAttempt, RECONNECT_DELAYS.length - 1)]
    this.reconnectAttempt += 1
    this.reconnectTimer = window.setTimeout(() => this.open('reconnecting'), delay)
  }

  private clearTimers() {
    this.stopHeartbeat()
    if (this.reconnectTimer) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = undefined
    }
  }
}
