import { websocketUrl } from './client'
import type { RealtimeStatus, WsClientAction, WsClientFrame, WsServerFrame } from './types'

type RealtimeCallbacks = {
  onStatusChange: (status: RealtimeStatus) => void
  onFrame: (frame: WsServerFrame) => void
  onError: (message: string) => void
}

const RECONNECT_DELAYS = [1000, 2000, 5000, 10000, 15000]
const HEARTBEAT_INTERVAL = 30000
const ACK_TIMEOUT = 15000

type PendingAck = {
  event: string
  resolve: (frame: WsServerFrame) => void
  reject: (error: Error) => void
  timer: number
  matchesFrame?: (frame: WsServerFrame) => boolean
}

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
  private pendingAcks = new Map<string, PendingAck>()

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
    this.rejectPendingAcks('实时连接已断开')
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

  sendAndWait<TPayload>(
    action: WsClientAction,
    payload: TPayload,
    event: string,
    matchesFrame?: (frame: WsServerFrame) => boolean,
  ) {
    const frameRequestId = requestId()
    const promise = new Promise<WsServerFrame>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        this.pendingAcks.delete(frameRequestId)
        reject(new Error('实时消息确认超时'))
      }, ACK_TIMEOUT)
      this.pendingAcks.set(frameRequestId, { event, resolve, reject, timer, matchesFrame })
    })

    try {
      this.send(action, payload, frameRequestId)
    } catch (caught) {
      this.clearPendingAck(frameRequestId)
      throw caught
    }
    return promise
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
        const frame = JSON.parse(event.data as string) as WsServerFrame
        this.resolvePendingAck(frame)
        this.callbacks.onFrame(frame)
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
      this.rejectPendingAcks('实时连接已断开')
      if (this.closedByUser) {
        this.callbacks.onStatusChange('disconnected')
        return
      }
      this.scheduleReconnect()
    })
  }

  private resolvePendingAck(frame: WsServerFrame) {
    if (frame.request_id) {
      const pending = this.pendingAcks.get(frame.request_id)
      if (pending && (frame.event === pending.event || frame.event === 'error')) {
        this.settlePendingAck(frame.request_id, pending, frame)
        return
      }
    }

    for (const [frameRequestId, pending] of this.pendingAcks) {
      if (pending.matchesFrame?.(frame)) {
        this.settlePendingAck(frameRequestId, pending, frame)
        return
      }
    }
  }

  private settlePendingAck(frameRequestId: string, pending: PendingAck, frame: WsServerFrame) {
    window.clearTimeout(pending.timer)
    this.pendingAcks.delete(frameRequestId)
    if (frame.event === 'error') {
      const data = frame.data as { msg?: string }
      pending.reject(new Error(data.msg || '实时消息发送失败'))
      return
    }
    pending.resolve(frame)
  }

  private clearPendingAck(frameRequestId: string) {
    const pending = this.pendingAcks.get(frameRequestId)
    if (!pending) {
      return
    }
    window.clearTimeout(pending.timer)
    this.pendingAcks.delete(frameRequestId)
  }

  private rejectPendingAcks(message: string) {
    for (const [frameRequestId, pending] of this.pendingAcks) {
      window.clearTimeout(pending.timer)
      pending.reject(new Error(message))
      this.pendingAcks.delete(frameRequestId)
    }
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
