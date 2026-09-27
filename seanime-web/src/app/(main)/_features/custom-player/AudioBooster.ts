/**
 * AudioBooster.ts - Web Audio API 1000% Volume Booster Engine
 * Part of Yourant Anime Tracking & Streaming Application
 *
 * Implements:
 * - Audio graph: AudioContext -> MediaElementSourceNode -> GainNode -> DynamicsCompressorNode -> destination
 * - Gain range: 0.0 to 10.0 (0% to 1000% volume)
 * - WeakMap singleton pattern to prevent InvalidStateError: HTMLMediaElement already connected
 * - Autoplay suspension unlock on first user gesture
 * - DynamicsCompressorNode brickwall peak limiter (-2.0 dBFS threshold, 16:1 ratio, 3ms attack)
 */

export interface BoosterGraph {
  context: AudioContext;
  source: MediaElementAudioSourceNode;
  gainNode: GainNode;
  compressor: DynamicsCompressorNode;
}

export interface VolumeState {
  percentage: number;      // 0 to 1000
  gain: number;            // 0.0 to 10.0
  db: number;              // Decibel boost: 20 * log10(gain)
  isBoostZone: boolean;    // true if percentage > 100
  warningBadge: boolean;   // true if percentage > 200
  isMuted: boolean;        // true if gain === 0 or muted
}

// Module-level WeakMap: strictly prevents InvalidStateError on DOM re-renders or multiple attaches
const nodeCache = new WeakMap<HTMLMediaElement, BoosterGraph>();

export class AudioBoosterManager {
  private mediaElement: HTMLMediaElement;
  private graph: BoosterGraph;
  private currentPercentage: number = 100;
  private preMutePercentage: number = 100;
  private isMuted: boolean = false;
  private listeners: Array<(state: VolumeState) => void> = [];

  constructor(mediaElement: HTMLMediaElement) {
    this.mediaElement = mediaElement;
    this.graph = this.getOrCreateGraph(mediaElement);
    this.setupGestureUnlock();
  }

  /**
   * Retrieves existing singleton graph or constructs a new Web Audio graph.
   */
  private getOrCreateGraph(element: HTMLMediaElement): BoosterGraph {
    const existing = nodeCache.get(element);
    if (existing) {
      if (existing.context.state === 'suspended') {
        this.unlockContext(existing.context);
      }
      return existing;
    }

    // Initialize cross-browser AudioContext
    const AudioCtxClass = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    const context = new AudioCtxClass();

    // 1. Create MediaElementSource (1:1 with media element)
    const source = context.createMediaElementSource(element);

    // 2. Create GainNode supporting up to 10.0x gain (1000%)
    const gainNode = context.createGain();
    gainNode.gain.setValueAtTime(1.0, context.currentTime);

    // 3. Create DynamicsCompressorNode configured as Brickwall Safety Peak Limiter
    // Clamps peaks strictly below 0 dBFS to prevent speaker blowouts & digital clipping
    const compressor = context.createDynamicsCompressor();
    compressor.threshold.setValueAtTime(-2.0, context.currentTime); // Clamps signals above -2.0 dBFS
    compressor.knee.setValueAtTime(12.0, context.currentTime);       // Soft musical knee
    compressor.ratio.setValueAtTime(16.0, context.currentTime);      // 16:1 aggressive limiting
    compressor.attack.setValueAtTime(0.003, context.currentTime);    // 3ms ultra-fast transient capture
    compressor.release.setValueAtTime(0.20, context.currentTime);    // 200ms natural decompression

    // 4. Connect Audio Pipeline: Source -> Gain -> Limiter Compressor -> Hardware Destination
    source.connect(gainNode);
    gainNode.connect(compressor);
    compressor.connect(context.destination);

    const graph: BoosterGraph = { context, source, gainNode, compressor };
    nodeCache.set(element, graph);

    return graph;
  }

  /**
   * Listens for user gestures to safely resume AudioContext if suspended by browser autoplay policy.
   */
  private setupGestureUnlock() {
    this.unlockContext(this.graph.context);
  }

  private unlockContext(ctx: AudioContext) {
    if (ctx.state === 'running') return;

    const resumeHandler = () => {
      if (ctx.state === 'suspended') {
        ctx.resume().catch(() => {});
      }
      window.removeEventListener('click', resumeHandler);
      window.removeEventListener('keydown', resumeHandler);
      window.removeEventListener('pointerdown', resumeHandler);
      window.removeEventListener('touchstart', resumeHandler);
    };

    window.addEventListener('click', resumeHandler, { passive: true });
    window.addEventListener('keydown', resumeHandler, { passive: true });
    window.addEventListener('pointerdown', resumeHandler, { passive: true });
    window.addEventListener('touchstart', resumeHandler, { passive: true });
  }

  /**
   * Explicitly unlocks the AudioContext (e.g. called from Play button click).
   */
  public async unlock(): Promise<void> {
    if (this.graph.context.state === 'suspended') {
      try {
        await this.graph.context.resume();
      } catch (err) {
        console.warn('AudioContext resume failed:', err);
      }
    }
  }

  /**
   * Sets volume percentage between 0% and 1000%.
   * 0% = 0.0 gain (mute)
   * 100% = 1.0 gain (standard unity gain)
   * 1000% = 10.0 gain (+20 dB boost)
   */
  public setVolume(percentage: number): VolumeState {
    const clamped = Math.max(0.0, Math.min(1000.0, percentage));
    this.currentPercentage = clamped;
    this.isMuted = (clamped === 0);

    const targetGain = clamped / 100.0;

    // Ensure AudioContext is active
    if (this.graph.context.state === 'suspended') {
      this.graph.context.resume().catch(() => {});
    }

    // Smooth exponential ramp avoids zipper noise and audible clicks
    const currentTime = this.graph.context.currentTime;
    this.graph.gainNode.gain.cancelScheduledValues(currentTime);
    this.graph.gainNode.gain.setTargetAtTime(targetGain, currentTime, 0.015);

    const state = this.getState();
    this.notifyListeners(state);
    return state;
  }

  /**
   * Toggles mute state. Restores previous volume on unmute.
   */
  public toggleMute(): VolumeState {
    if (this.isMuted || this.currentPercentage === 0) {
      const restore = this.preMutePercentage > 0 ? this.preMutePercentage : 100;
      return this.setVolume(restore);
    } else {
      this.preMutePercentage = this.currentPercentage;
      return this.setVolume(0);
    }
  }

  /**
   * Returns current VolumeState snapshot.
   */
  public getState(): VolumeState {
    const gain = this.currentPercentage / 100.0;
    const db = gain > 0 ? 20.0 * Math.log10(gain) : -Infinity;
    return {
      percentage: this.currentPercentage,
      gain,
      db: Math.round(db * 10) / 10,
      isBoostZone: this.currentPercentage > 100.0,
      warningBadge: this.currentPercentage > 200.0,
      isMuted: this.currentPercentage === 0 || this.isMuted,
    };
  }

  /**
   * Subscribes to volume state updates.
   */
  public onVolumeChange(listener: (state: VolumeState) => void): () => void {
    this.listeners.push(listener);
    listener(this.getState());
    return () => {
      this.listeners = this.listeners.filter(l => l !== listener);
    };
  }

  private notifyListeners(state: VolumeState) {
    for (const listener of this.listeners) {
      try {
        listener(state);
      } catch (err) {
        console.error('Error in volume change listener:', err);
      }
    }
  }

  /**
   * Access to raw AudioContext for analysis or testing.
   */
  public getContext(): AudioContext {
    return this.graph.context;
  }

  /**
   * Access to GainNode for testing.
   */
  public getGainNode(): GainNode {
    return this.graph.gainNode;
  }

  /**
   * Access to Limiter Compressor for testing.
   */
  public getCompressor(): DynamicsCompressorNode {
    return this.graph.compressor;
  }

  public getMediaElement(): HTMLMediaElement {
    return this.mediaElement;
  }
}
