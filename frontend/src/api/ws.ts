/**
 * Watch Party WebSocket Client (frontend/src/api/ws.ts)
 * 
 * Implements robust real-time communication for Watch Parties:
 * - Automatic connection management & exponential backoff reconnection
 * - Typed protocol message dispatching
 * - Continuous ping/pong Round Trip Time (RTT) latency measurement
 */

export interface MemberInfo {
  id: string;
  name: string;
  isHost: boolean;
  joinedAt: number;
}

export interface WatchPartyMessage<T = Record<string, any>> {
  type: string;
  roomId: string;
  senderId: string;
  senderName: string;
  isHost: boolean;
  clientTime?: number;
  serverTime?: number;
  payload?: T;
}

export type MessageHandler = (msg: WatchPartyMessage) => void;
export type LatencyHandler = (latencyMs: number) => void;
export type ConnectionHandler = (connected: boolean) => void;

export interface WSClientOptions {
  roomId?: string;
  userId: string;
  name: string;
  isHost?: boolean;
}

export class WatchPartyWSClient {
  private ws: WebSocket | null = null;
  private options: WSClientOptions | null = null;
  private handlers: Map<string, Set<MessageHandler>> = new Map();
  private connectionHandlers: Set<ConnectionHandler> = new Set();
  private latencyHandlers: Set<LatencyHandler> = new Set();

  private isManualClose: boolean = false;
  private reconnectAttempts: number = 0;
  private reconnectTimer: number | null = null;
  private pingTimer: number | null = null;
  private currentLatencyMs: number = 0;
  private isConnecting: boolean = false;

  constructor() {
    // Internal handler for pong latency measurement
    this.onMessage('pong', (msg) => {
      if (msg.clientTime) {
        const rtt = Date.now() - msg.clientTime;
        this.currentLatencyMs = Math.max(1, Math.round(rtt / 2));
        this.notifyLatencyChange();
      }
    });
  }

  /**
   * Connect to the backend Watch Party WebSocket endpoint.
   */
  public connect(options: WSClientOptions): Promise<void> {
    this.options = { ...options };
    this.isManualClose = false;
    this.isConnecting = true;

    return new Promise((resolve, reject) => {
      try {
        if (this.ws) {
          this.ws.close();
          this.ws = null;
        }

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const host = window.location.host;
        const q = new URLSearchParams();
        if (options.roomId) q.set('roomId', options.roomId);
        if (options.userId) q.set('userId', options.userId);
        if (options.name) q.set('name', options.name);
        if (options.isHost !== undefined) q.set('isHost', options.isHost ? 'true' : 'false');

        const wsUrl = `${protocol}//${host}/api/watchparty/ws?${q.toString()}`;
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
          this.isConnecting = false;
          this.reconnectAttempts = 0;
          this.startHeartbeat();
          this.notifyConnectionChange(true);
          resolve();
        };

        this.ws.onmessage = (event) => {
          this.handleRawMessage(event.data);
        };

        this.ws.onclose = () => {
          this.isConnecting = false;
          this.stopHeartbeat();
          this.notifyConnectionChange(false);

          if (!this.isManualClose) {
            this.scheduleReconnect();
          }
        };

        this.ws.onerror = (err) => {
          this.isConnecting = false;
          console.warn('[WP-WS] WebSocket error:', err);
          if (this.isConnecting) {
            reject(err);
          }
        };
      } catch (err) {
        this.isConnecting = false;
        reject(err);
      }
    });
  }

  /**
   * Disconnects the current WebSocket session intentionally.
   */
  public disconnect() {
    this.isManualClose = true;
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.stopHeartbeat();
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.notifyConnectionChange(false);
  }

  /**
   * Sends a typed JSON protocol message.
   */
  public send(msg: Partial<WatchPartyMessage>) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('[WP-WS] Cannot send message, socket is not open');
      return;
    }

    const fullMsg: WatchPartyMessage = {
      type: msg.type || 'unknown',
      roomId: msg.roomId || this.options?.roomId || '',
      senderId: msg.senderId || this.options?.userId || '',
      senderName: msg.senderName || this.options?.name || '',
      isHost: msg.isHost !== undefined ? msg.isHost : (this.options?.isHost || false),
      clientTime: msg.clientTime || Date.now(),
      payload: msg.payload || {},
    };

    this.ws.send(JSON.stringify(fullMsg));
  }

  /**
   * Registers a message listener for a specific protocol type.
   */
  public onMessage(type: string, handler: MessageHandler): () => void {
    if (!this.handlers.has(type)) {
      this.handlers.set(type, new Set());
    }
    this.handlers.get(type)!.add(handler);

    return () => {
      this.handlers.get(type)?.delete(handler);
    };
  }

  public onConnectionChange(handler: ConnectionHandler): () => void {
    this.connectionHandlers.add(handler);
    return () => this.connectionHandlers.delete(handler);
  }

  public onLatencyChange(handler: LatencyHandler): () => void {
    this.latencyHandlers.add(handler);
    return () => this.latencyHandlers.delete(handler);
  }

  public isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  public getLatency(): number {
    return this.currentLatencyMs;
  }

  public getOptions(): WSClientOptions | null {
    return this.options;
  }

  public updateRoomId(roomId: string) {
    if (this.options) {
      this.options.roomId = roomId;
    }
  }

  public setIsHost(isHost: boolean) {
    if (this.options) {
      this.options.isHost = isHost;
    }
  }

  private handleRawMessage(data: string) {
    try {
      const msg = JSON.parse(data) as WatchPartyMessage;
      
      // Update room ID if provided in message
      if (msg.roomId && this.options && (!this.options.roomId || this.options.roomId !== msg.roomId)) {
        this.options.roomId = msg.roomId;
      }

      // Check for room:joined to sync host status
      if (msg.type === 'room:joined' && msg.payload) {
        if (msg.payload.hostId && this.options) {
          this.options.isHost = (msg.payload.hostId === this.options.userId);
        }
      }

      // Dispatch to specific handlers
      const typeHandlers = this.handlers.get(msg.type);
      if (typeHandlers) {
        typeHandlers.forEach((h) => h(msg));
      }

      // Dispatch to wildcard handlers
      const allHandlers = this.handlers.get('*');
      if (allHandlers) {
        allHandlers.forEach((h) => h(msg));
      }
    } catch (err) {
      console.error('[WP-WS] Failed to parse message JSON:', err);
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat();
    // Immediate ping on connect
    this.sendPing();
    this.pingTimer = window.setInterval(() => {
      this.sendPing();
    }, 5000);
  }

  private stopHeartbeat() {
    if (this.pingTimer !== null) {
      window.clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }

  private sendPing() {
    if (this.isConnected()) {
      this.send({
        type: 'ping',
        clientTime: Date.now(),
      });
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer !== null || !this.options) return;

    this.reconnectAttempts++;
    const delay = Math.min(10000, 1000 * Math.pow(1.5, this.reconnectAttempts));

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      if (!this.isManualClose && this.options) {
        this.connect(this.options).catch((err) => {
          console.warn('[WP-WS] Reconnect attempt failed:', err);
        });
      }
    }, delay);
  }

  private notifyConnectionChange(connected: boolean) {
    this.connectionHandlers.forEach((h) => h(connected));
  }

  private notifyLatencyChange() {
    this.latencyHandlers.forEach((h) => h(this.currentLatencyMs));
  }
}

// Singleton WS Client
export const wsClient = new WatchPartyWSClient();
