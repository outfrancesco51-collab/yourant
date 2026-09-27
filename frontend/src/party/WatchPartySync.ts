/**
 * WatchPartySync.ts - Real-Time Playback Synchronization Engine
 * Part of Yourant Anime Tracking & Streaming Platform
 *
 * Implements the 3-Tier Drift Compensation Algorithm:
 * - Tier 1: |drift| < 0.300s  -> IN_SYNC      (rate: 1.00x, pill: green, label: "SYNCED")
 * - Tier 2: 0.3s <= |drift| <= 1.5s -> SPEED_ADJUST (rate: 1.05x behind, 0.95x ahead, pill: amber, label: "ADJUSTING SPEED")
 * - Tier 3: |drift| > 1.500s  -> HARD_SEEK    (seek target, rate: 1.00x, pill: red, label: "HARD RESYNC")
 */

import { wsClient, WatchPartyMessage } from '../api/ws';

export type SyncAction = 'IN_SYNC' | 'SPEED_ADJUST' | 'HARD_SEEK';
export type SyncPillColor = 'green' | 'amber' | 'red';

export interface SyncStatus {
  action: SyncAction;
  pillColor: SyncPillColor;
  label: string;
  driftSec: number;
  targetTime: number;
  playbackRate: number;
  isHost: boolean;
  isPlaying: boolean;
}

/**
 * Pure 3-tier drift compensation calculator conforming to system invariants.
 */
export function computeDriftCompensation(
  clientCurrentTime: number,
  hostTimestamp: number,
  isPlaying: boolean,
  transitDelaySec: number = 0,
  playbackRate: number = 1.0
): { targetTime: number; rate: number; action: SyncAction; pillColor: SyncPillColor; label: string; drift: number } {
  let targetTime = hostTimestamp;
  if (isPlaying) {
    targetTime = hostTimestamp + (transitDelaySec * playbackRate);
  }

  const drift = clientCurrentTime - targetTime;
  const absDrift = Math.abs(drift);

  if (absDrift < 0.300) {
    return {
      targetTime,
      rate: 1.0,
      action: 'IN_SYNC',
      pillColor: 'green',
      label: 'SYNCED',
      drift,
    };
  } else if (absDrift <= 1.500) {
    // Client is ahead -> slow down to 0.95x; client is behind -> speed up to 1.05x
    const rate = drift > 0 ? 0.95 : 1.05;
    return {
      targetTime,
      rate,
      action: 'SPEED_ADJUST',
      pillColor: 'amber',
      label: 'ADJUSTING SPEED',
      drift,
    };
  } else {
    // Large deviation -> hard seek resync
    return {
      targetTime,
      rate: 1.0,
      action: 'HARD_SEEK',
      pillColor: 'red',
      label: 'HARD RESYNC',
      drift,
    };
  }
}

export class WatchPartySync {
  private video: HTMLVideoElement | null = null;
  private isHost: boolean = false;
  private heartbeatInterval: number | null = null;
  private lastStatusListeners: Set<(status: SyncStatus) => void> = new Set();
  private isApplyingRemoteAction: boolean = false;

  private currentStatus: SyncStatus = {
    action: 'IN_SYNC',
    pillColor: 'green',
    label: 'SYNCED',
    driftSec: 0,
    targetTime: 0,
    playbackRate: 1.0,
    isHost: false,
    isPlaying: false,
  };

  private unsubscribers: Array<() => void> = [];

  constructor() {
    this.registerWsHandlers();
  }

  /**
   * Binds the synchronizer to an active HTML5 video element.
   */
  public attachVideo(video: HTMLVideoElement, isHost: boolean) {
    this.detachVideo();
    this.video = video;
    this.isHost = isHost;
    this.currentStatus.isHost = isHost;

    if (this.isHost) {
      this.setupHostListeners();
    }
  }

  public detachVideo() {
    this.stopHostHeartbeat();
    this.video = null;
  }

  public setIsHost(isHost: boolean) {
    this.isHost = isHost;
    this.currentStatus.isHost = isHost;
    if (this.video) {
      if (isHost) {
        this.setupHostListeners();
      } else {
        this.stopHostHeartbeat();
      }
    }
    this.notifyStatusListeners();
  }

  public getIsHost(): boolean {
    return this.isHost;
  }

  public getStatus(): SyncStatus {
    return this.currentStatus;
  }

  public onSyncStatus(cb: (status: SyncStatus) => void): () => void {
    this.lastStatusListeners.add(cb);
    cb(this.currentStatus);
    return () => this.lastStatusListeners.delete(cb);
  }

  // =========================================================================
  // HOST DISPATCH LOGIC
  // =========================================================================

  private setupHostListeners() {
    if (!this.video) return;

    this.video.addEventListener('play', this.handleHostPlay);
    this.video.addEventListener('pause', this.handleHostPause);
    this.video.addEventListener('seeked', this.handleHostSeek);

    if (!this.video.paused) {
      this.startHostHeartbeat();
    }
  }

  private handleHostPlay = () => {
    if (!this.isHost || !this.video || this.isApplyingRemoteAction) return;

    wsClient.send({
      type: 'host:play',
      payload: {
        isPlaying: true,
        timestamp: this.video.currentTime,
        playbackRate: this.video.playbackRate,
      },
    });
    this.startHostHeartbeat();
  };

  private handleHostPause = () => {
    if (!this.isHost || !this.video || this.isApplyingRemoteAction) return;

    this.stopHostHeartbeat();
    wsClient.send({
      type: 'host:pause',
      payload: {
        isPlaying: false,
        timestamp: this.video.currentTime,
        playbackRate: this.video.playbackRate,
      },
    });
  };

  private handleHostSeek = () => {
    if (!this.isHost || !this.video || this.isApplyingRemoteAction) return;

    wsClient.send({
      type: 'host:seek',
      payload: {
        isPlaying: !this.video.paused,
        timestamp: this.video.currentTime,
        playbackRate: this.video.playbackRate,
      },
    });
  };

  private startHostHeartbeat() {
    this.stopHostHeartbeat();
    this.heartbeatInterval = window.setInterval(() => {
      if (this.isHost && this.video && !this.video.paused) {
        wsClient.send({
          type: 'host:heartbeat',
          payload: {
            isPlaying: true,
            timestamp: this.video.currentTime,
            playbackRate: this.video.playbackRate,
          },
        });
      }
    }, 1500);
  }

  private stopHostHeartbeat() {
    if (this.heartbeatInterval !== null) {
      window.clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }

  // =========================================================================
  // GUEST INCOMING DRIFT COMPENSATION LOGIC
  // =========================================================================

  private registerWsHandlers() {
    this.unsubscribers.push(
      wsClient.onMessage('host:play', (msg) => this.handleRemotePlay(msg)),
      wsClient.onMessage('host:pause', (msg) => this.handleRemotePause(msg)),
      wsClient.onMessage('host:seek', (msg) => this.handleRemoteSeek(msg)),
      wsClient.onMessage('host:heartbeat', (msg) => this.handleRemoteHeartbeat(msg)),
      wsClient.onMessage('room:joined', (msg) => this.handleRoomJoined(msg)),
      wsClient.onMessage('sync:state', (msg) => this.handleRemoteHeartbeat(msg))
    );
  }

  private handleRoomJoined(msg: WatchPartyMessage) {
    if (msg.payload) {
      const isHost = !!msg.payload.isHost;
      this.setIsHost(isHost);
      if (msg.payload.timestamp !== undefined) {
        this.handleRemoteHeartbeat(msg);
      }
    }
  }

  private handleRemotePlay(msg: WatchPartyMessage) {
    if (this.isHost || !this.video) return;

    const timestamp = msg.payload?.timestamp ?? 0;
    this.isApplyingRemoteAction = true;
    this.video.currentTime = timestamp;
    this.video.play().catch(console.warn).finally(() => {
      this.isApplyingRemoteAction = false;
    });

    this.updateStatus(0, timestamp, 1.0, 'IN_SYNC', 'green', 'SYNCED', true);
  }

  private handleRemotePause(msg: WatchPartyMessage) {
    if (this.isHost || !this.video) return;

    const timestamp = msg.payload?.timestamp ?? this.video.currentTime;
    this.isApplyingRemoteAction = true;
    this.video.pause();
    this.video.currentTime = timestamp;
    this.isApplyingRemoteAction = false;

    this.updateStatus(0, timestamp, 1.0, 'IN_SYNC', 'green', 'SYNCED', false);
  }

  private handleRemoteSeek(msg: WatchPartyMessage) {
    if (this.isHost || !this.video) return;

    const timestamp = msg.payload?.timestamp ?? 0;
    const isPlaying = !!msg.payload?.isPlaying;

    this.isApplyingRemoteAction = true;
    this.video.currentTime = timestamp;
    if (isPlaying && this.video.paused) {
      this.video.play().catch(console.warn);
    } else if (!isPlaying && !this.video.paused) {
      this.video.pause();
    }
    this.isApplyingRemoteAction = false;

    this.updateStatus(0, timestamp, 1.0, 'IN_SYNC', 'green', 'SYNCED', isPlaying);
  }

  private handleRemoteHeartbeat(msg: WatchPartyMessage) {
    if (this.isHost || !this.video) return;

    const hostTimestamp = msg.payload?.timestamp ?? 0;
    const isPlaying = msg.payload?.isPlaying !== undefined ? !!msg.payload.isPlaying : true;
    const hostRate = msg.payload?.playbackRate ?? 1.0;

    // Estimate transit delay from measured RTT latency
    const transitDelaySec = (wsClient.getLatency() / 1000.0);
    const clientTime = this.video.currentTime;

    const comp = computeDriftCompensation(clientTime, hostTimestamp, isPlaying, transitDelaySec, hostRate);

    // Apply 3-Tier compensation actions to HTML5 video element
    if (comp.action === 'HARD_SEEK') {
      this.isApplyingRemoteAction = true;
      this.video.currentTime = comp.targetTime;
      this.video.playbackRate = 1.0;
      this.isApplyingRemoteAction = false;
    } else {
      // Tier 1 (1.0x) or Tier 2 (1.05x / 0.95x)
      this.video.playbackRate = comp.rate;
    }

    if (isPlaying && this.video.paused) {
      this.video.play().catch(console.warn);
    } else if (!isPlaying && !this.video.paused) {
      this.video.pause();
    }

    this.updateStatus(comp.drift, comp.targetTime, comp.rate, comp.action, comp.pillColor, comp.label, isPlaying);
  }

  private updateStatus(
    driftSec: number,
    targetTime: number,
    rate: number,
    action: SyncAction,
    pillColor: SyncPillColor,
    label: string,
    isPlaying: boolean
  ) {
    this.currentStatus = {
      action,
      pillColor,
      label,
      driftSec,
      targetTime,
      playbackRate: rate,
      isHost: this.isHost,
      isPlaying,
    };
    this.notifyStatusListeners();
  }

  private notifyStatusListeners() {
    this.lastStatusListeners.forEach((listener) => listener(this.currentStatus));
  }

  public destroy() {
    this.detachVideo();
    this.unsubscribers.forEach((unsub) => unsub());
    this.unsubscribers = [];
    this.lastStatusListeners.clear();
  }
}

// Global Singleton Synchronizer
export const partySync = new WatchPartySync();
