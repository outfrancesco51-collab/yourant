/**
 * Player.ts - Seanime-Inspired HTML5 Video Player Component
 * Part of Yourant Anime Tracking & Streaming Application
 *
 * Implements:
 * - HTML5 video element with crossOrigin="anonymous"
 * - Web Audio API 1000% volume booster integration & dual-zone slider
 * - Cinema Mode integration with 'C' & 'Escape' hotkeys
 * - Auto-hiding HUD controls (2.5s idle timer)
 * - Custom scrubber with buffered ranges and time hover tooltip
 * - Watch Party synchronization hooks (play, pause, seek, playbackRate)
 */

import { AudioBoosterManager, VolumeState } from '../audio/AudioBooster';
import { CinemaModeManager } from '../cinema/CinemaMode';
import { partySync } from '../party/WatchPartySync';
import { WatchPartyMessage } from '../api/ws';

export interface PlayerOptions {
  container: HTMLElement;
  src?: string;
  title?: string;
  poster?: string;
  cinemaManager?: CinemaModeManager;
  onPlay?: () => void;
  onPause?: () => void;
  onSeek?: (timestamp: number) => void;
  onTimeUpdate?: (currentTime: number, duration: number) => void;
  onEnded?: () => void;
}

export class Player {
  private container: HTMLElement;
  private video!: HTMLVideoElement;
  private booster!: AudioBoosterManager;
  private cinemaManager: CinemaModeManager;
  private options: PlayerOptions;

  // DOM Elements
  private playPauseBtn!: HTMLButtonElement;
  private bigPlayOverlay!: HTMLElement;
  private bufferSpinner!: HTMLElement;
  private scrubberContainer!: HTMLElement;
  private scrubberProgress!: HTMLElement;
  private scrubberBuffered!: HTMLElement;
  private scrubberThumb!: HTMLElement;
  private scrubberTooltip!: HTMLElement;
  private timeDisplay!: HTMLElement;
  private volumeBtn!: HTMLButtonElement;
  private volumeSlider!: HTMLInputElement;
  private volumeTrackFill!: HTMLElement;
  private volumeBadge!: HTMLElement;
  private playbackRateBtn!: HTMLButtonElement;
  private cinemaBtn!: HTMLButtonElement;
  private fullscreenBtn!: HTMLButtonElement;
  private floatingChatContainer!: HTMLElement;
  private chatListener: EventListener | null = null;

  // State
  private idleTimeoutId: number | null = null;
  private isScrubbing: boolean = false;
  private isRemoteAction: boolean = false;
  private playbackRates: number[] = [0.5, 0.75, 1.0, 1.25, 1.5, 2.0];
  private currentRateIndex: number = 2; // 1.0x

  constructor(options: PlayerOptions) {
    this.options = options;
    this.container = options.container;
    this.cinemaManager = options.cinemaManager || new CinemaModeManager();

    this.render();
    this.initAudioBooster();
    this.bindEvents();
    this.resetIdleTimer();

    if (options.src) {
      this.loadSource(options.src);
    }
  }

  /**
   * Builds the DOM structure with metallic blue and blood red styling.
   */
  private render() {
    this.container.classList.add('player-container');
    this.container.tabIndex = 0;

    this.container.innerHTML = `
      <video class="player-video" playsinline preload="metadata" crossorigin="anonymous"></video>
      
      <!-- Big Center Play Button Overlay -->
      <div class="player-big-play-overlay">
        <button class="btn-big-play" aria-label="Play">
          <svg viewBox="0 0 24 24" width="36" height="36" fill="currentColor">
            <path d="M8 5v14l11-7z"/>
          </svg>
        </button>
      </div>

      <!-- Loading / Buffering Spinner -->
      <div class="player-spinner hidden">
        <div class="spinner-ring"></div>
      </div>

      <!-- Auto-Hiding Controls HUD -->
      <div class="player-controls">
        <!-- Interactive Scrubber Bar -->
        <div class="scrubber-bar" tabindex="0">
          <div class="scrubber-track">
            <div class="scrubber-buffered"></div>
            <div class="scrubber-progress"></div>
            <div class="scrubber-thumb"></div>
          </div>
          <div class="scrubber-tooltip">00:00</div>
        </div>

        <!-- Controls Bottom Toolbar -->
        <div class="controls-toolbar">
          <div class="controls-left">
            <!-- Play/Pause -->
            <button class="ctrl-btn btn-play-pause" title="Play (Space)">
              <svg class="icon-play" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d="M8 5v14l11-7z"/>
              </svg>
              <svg class="icon-pause hidden" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/>
              </svg>
            </button>

            <!-- Skip Back 10s -->
            <button class="ctrl-btn btn-skip-back" title="Rewind 10s (Left Arrow)">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M11 17l-5-5 5-5M18 17l-5-5 5-5"/>
              </svg>
              <span>10s</span>
            </button>

            <!-- Skip Fwd 10s -->
            <button class="ctrl-btn btn-skip-fwd" title="Forward 10s (Right Arrow)">
              <span>10s</span>
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M13 17l5-5-5-5M6 17l5-5-5-5"/>
              </svg>
            </button>

            <!-- Dual-Zone Volume Booster Cluster -->
            <div class="volume-cluster">
              <button class="ctrl-btn btn-volume" title="Mute (M)">
                <svg class="icon-vol-high" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                  <path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
                </svg>
                <svg class="icon-vol-mute hidden" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                  <path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
                </svg>
              </button>
              
              <div class="volume-slider-wrapper">
                <input type="range" class="volume-slider" min="0" max="1000" step="5" value="100" aria-label="Volume Booster" />
                <div class="volume-fill-bar"></div>
              </div>

              <!-- Dynamic Volume / Boost Badge -->
              <span class="volume-badge" title="Double click to reset to 100%">100%</span>
            </div>

            <!-- Time Display -->
            <div class="time-display">
              <span class="time-current">00:00</span>
              <span class="time-divider">/</span>
              <span class="time-duration">00:00</span>
            </div>
          </div>

          <div class="controls-right">
            <!-- Playback Rate Selector -->
            <button class="ctrl-btn btn-rate" title="Playback Speed">1.0x</button>

            <!-- Cinema Mode Toggle -->
            <button class="ctrl-btn btn-cinema" title="Cinema Mode (C)">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="2" y="4" width="20" height="16" rx="2"/>
                <path d="M7 4v16M17 4v16M2 12h20"/>
              </svg>
            </button>

            <!-- Native Fullscreen Toggle -->
            <button class="ctrl-btn btn-fullscreen" title="Fullscreen (F)">
              <svg class="icon-fs-enter" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"/>
              </svg>
              <svg class="icon-fs-exit hidden" viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d="M5 16h3v3h2v-5H5v2zm3-8H5v2h5V5H8v3zm6 11h2v-3h3v-2h-5v5zm2-14v3h3v2h-5V5h2z"/>
              </svg>
            </button>
          </div>
        </div>
      </div>

      <!-- Floating Translucent Watch Party Chat Stream -->
      <div class="player-floating-chat"></div>
    `;

    // Query elements
    this.video = this.container.querySelector('.player-video')!;
    this.playPauseBtn = this.container.querySelector('.btn-play-pause')!;
    this.bigPlayOverlay = this.container.querySelector('.player-big-play-overlay')!;
    this.bufferSpinner = this.container.querySelector('.player-spinner')!;
    this.scrubberContainer = this.container.querySelector('.scrubber-bar')!;
    this.scrubberProgress = this.container.querySelector('.scrubber-progress')!;
    this.scrubberBuffered = this.container.querySelector('.scrubber-buffered')!;
    this.scrubberThumb = this.container.querySelector('.scrubber-thumb')!;
    this.scrubberTooltip = this.container.querySelector('.scrubber-tooltip')!;
    this.timeDisplay = this.container.querySelector('.time-display')!;
    this.volumeBtn = this.container.querySelector('.btn-volume')!;
    this.volumeSlider = this.container.querySelector('.volume-slider')!;
    this.volumeTrackFill = this.container.querySelector('.volume-fill-bar')!;
    this.volumeBadge = this.container.querySelector('.volume-badge')!;
    this.playbackRateBtn = this.container.querySelector('.btn-rate')!;
    this.cinemaBtn = this.container.querySelector('.btn-cinema')!;
    this.fullscreenBtn = this.container.querySelector('.btn-fullscreen')!;
    this.floatingChatContainer = this.container.querySelector('.player-floating-chat')!;

    if (this.options.poster) {
      this.video.poster = this.options.poster;
    }
  }

  /**
   * Initializes the Web Audio 1000% booster graph singleton.
   */
  private initAudioBooster() {
    this.booster = new AudioBoosterManager(this.video);

    // Sync volume slider with booster state updates
    this.booster.onVolumeChange((state) => {
      this.updateVolumeUI(state);
    });
  }

  /**
   * Updates Volume UI based on booster state (cyan vs red boost zone).
   */
  private updateVolumeUI(state: VolumeState) {
    this.volumeSlider.value = state.percentage.toString();
    const percentWidth = (state.percentage / 1000.0) * 100;
    this.volumeTrackFill.style.width = `${percentWidth}%`;

    // Normal zone (0-100%) vs Boost Zone (101-1000%)
    if (state.percentage > 100) {
      this.volumeTrackFill.classList.add('boost-zone');
      this.volumeBadge.classList.add('badge-boost');
      this.volumeBadge.textContent = state.warningBadge 
        ? `⚠️ ${state.percentage}% (+${state.db}dB)` 
        : `⚡ ${state.percentage}%`;
    } else {
      this.volumeTrackFill.classList.remove('boost-zone');
      this.volumeBadge.classList.remove('badge-boost');
      this.volumeBadge.textContent = `${state.percentage}%`;
    }

    // Toggle mute icon
    const iconHigh = this.volumeBtn.querySelector('.icon-vol-high')!;
    const iconMute = this.volumeBtn.querySelector('.icon-vol-mute')!;
    if (state.isMuted) {
      iconHigh.classList.add('hidden');
      iconMute.classList.remove('hidden');
    } else {
      iconHigh.classList.remove('hidden');
      iconMute.classList.add('hidden');
    }
  }

  /**
   * Binds user interactions and media events.
   */
  private bindEvents() {
    // 1. Play / Pause
    const togglePlay = async () => {
      await this.booster.unlock();
      if (this.video.paused || this.video.ended) {
        this.video.play().catch(console.warn);
      } else {
        this.video.pause();
      }
    };

    this.playPauseBtn.addEventListener('click', togglePlay);
    this.bigPlayOverlay.addEventListener('click', togglePlay);
    this.video.addEventListener('click', togglePlay);

    this.video.addEventListener('play', () => {
      this.playPauseBtn.querySelector('.icon-play')!.classList.add('hidden');
      this.playPauseBtn.querySelector('.icon-pause')!.classList.remove('hidden');
      this.bigPlayOverlay.classList.add('hidden');
      this.resetIdleTimer();
      if (!this.isRemoteAction && this.options.onPlay) {
        this.options.onPlay();
      }
    });

    this.video.addEventListener('pause', () => {
      this.playPauseBtn.querySelector('.icon-play')!.classList.remove('hidden');
      this.playPauseBtn.querySelector('.icon-pause')!.classList.add('hidden');
      this.container.classList.remove('controls-idle');
      if (!this.isRemoteAction && this.options.onPause) {
        this.options.onPause();
      }
    });

    // 2. Skip buttons (10s)
    this.container.querySelector('.btn-skip-back')?.addEventListener('click', () => {
      this.seek(this.video.currentTime - 10);
    });
    this.container.querySelector('.btn-skip-fwd')?.addEventListener('click', () => {
      this.seek(this.video.currentTime + 10);
    });

    // 3. Volume slider & Mute button
    this.volumeSlider.addEventListener('input', () => {
      const val = parseFloat(this.volumeSlider.value);
      this.booster.setVolume(val);
    });

    this.volumeBtn.addEventListener('click', () => {
      this.booster.toggleMute();
    });

    // Double-click badge resets volume to 100%
    this.volumeBadge.addEventListener('dblclick', () => {
      this.booster.setVolume(100);
    });

    // 4. Playback Rate cycling
    this.playbackRateBtn.addEventListener('click', () => {
      this.currentRateIndex = (this.currentRateIndex + 1) % this.playbackRates.length;
      const rate = this.playbackRates[this.currentRateIndex];
      this.setPlaybackRate(rate);
    });

    // 5. Cinema Mode button
    this.cinemaBtn.addEventListener('click', () => {
      this.cinemaManager.toggle();
    });

    // 6. Fullscreen button
    this.fullscreenBtn.addEventListener('click', () => {
      this.toggleFullscreen();
    });

    document.addEventListener('fullscreenchange', () => {
      const isFs = !!document.fullscreenElement;
      this.fullscreenBtn.querySelector('.icon-fs-enter')!.classList.toggle('hidden', isFs);
      this.fullscreenBtn.querySelector('.icon-fs-exit')!.classList.toggle('hidden', !isFs);
    });

    // 7. Time & Buffer updates
    this.video.addEventListener('timeupdate', () => {
      if (!this.isScrubbing) {
        this.updateTimeDisplay();
      }
      this.updateBufferedBar();
      if (this.options.onTimeUpdate) {
        this.options.onTimeUpdate(this.video.currentTime, this.video.duration || 0);
      }
    });

    this.video.addEventListener('waiting', () => {
      this.bufferSpinner.classList.remove('hidden');
    });

    this.video.addEventListener('playing', () => {
      this.bufferSpinner.classList.add('hidden');
    });

    this.video.addEventListener('ended', () => {
      if (this.options.onEnded) this.options.onEnded();
    });

    // 8. Scrubber physics
    this.setupScrubberEvents();

    // 9. Idle HUD Timer
    const notifyActivity = () => {
      this.resetIdleTimer();
    };

    this.container.addEventListener('mousemove', notifyActivity);
    this.container.addEventListener('pointerdown', notifyActivity);
    this.container.addEventListener('keydown', notifyActivity);

    // 10. Hotkeys
    this.setupHotkeys();

    // 11. Watch Party Video Synchronization & Floating Chat Stream
    partySync.attachVideo(this.video, partySync.getIsHost());
    this.setupFloatingChat();
  }

  /**
   * Listens for real-time Watch Party chat events and renders floating translucent messages.
   */
  private setupFloatingChat() {
    this.chatListener = ((e: CustomEvent<WatchPartyMessage>) => {
      const msg = e.detail;
      if (!this.floatingChatContainer || !msg.payload?.chatText) return;

      const item = document.createElement('div');
      item.className = `player-floating-msg ${msg.isHost ? 'is-host' : ''}`;
      const author = msg.senderName || 'Anonymous';
      const text = msg.payload.chatText;
      item.innerHTML = `
        <span class="player-floating-author ${msg.isHost ? 'is-host' : ''}">${this.escapeHtml(author)}:</span>
        <span class="player-floating-text">${this.escapeHtml(text)}</span>
      `;
      this.floatingChatContainer.prepend(item);

      setTimeout(() => {
        if (item.parentNode) {
          item.remove();
        }
      }, 4000);
    }) as EventListener;

    window.addEventListener('yourant:wp-chat', this.chatListener);
  }

  private escapeHtml(str: string): string {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  /**
   * Configures scrubber bar click, drag, and hover preview.
   */
  private setupScrubberEvents() {
    const calculateTimeFromEvent = (e: MouseEvent | PointerEvent): number => {
      const rect = this.scrubberContainer.getBoundingClientRect();
      const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      return pos * (this.video.duration || 0);
    };

    // Hover tooltip
    this.scrubberContainer.addEventListener('mousemove', (e) => {
      const rect = this.scrubberContainer.getBoundingClientRect();
      const pos = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      const hoverTime = pos * (this.video.duration || 0);
      this.scrubberTooltip.textContent = this.formatTime(hoverTime);
      this.scrubberTooltip.style.left = `${pos * 100}%`;
    });

    // Drag / Scrubbing
    this.scrubberContainer.addEventListener('pointerdown', (e) => {
      this.isScrubbing = true;
      this.scrubberContainer.setPointerCapture(e.pointerId);
      const targetTime = calculateTimeFromEvent(e);
      this.updateScrubberUI(targetTime);

      const onPointerMove = (moveEvt: PointerEvent) => {
        if (!this.isScrubbing) return;
        const time = calculateTimeFromEvent(moveEvt);
        this.updateScrubberUI(time);
      };

      const onPointerUp = (upEvt: PointerEvent) => {
        if (!this.isScrubbing) return;
        this.isScrubbing = false;
        const finalTime = calculateTimeFromEvent(upEvt);
        this.seek(finalTime);
        this.scrubberContainer.removeEventListener('pointermove', onPointerMove);
        this.scrubberContainer.removeEventListener('pointerup', onPointerUp);
      };

      this.scrubberContainer.addEventListener('pointermove', onPointerMove);
      this.scrubberContainer.addEventListener('pointerup', onPointerUp);
    });
  }

  /**
   * Keyboard shortcuts handling.
   */
  private setupHotkeys() {
    this.container.addEventListener('keydown', (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) {
        return;
      }

      switch (e.key) {
        case ' ':
        case 'k':
        case 'K':
          e.preventDefault();
          this.playPauseBtn.click();
          break;
        case 'ArrowLeft':
          e.preventDefault();
          this.seek(this.video.currentTime - 5);
          break;
        case 'ArrowRight':
          e.preventDefault();
          this.seek(this.video.currentTime + 5);
          break;
        case 'ArrowUp':
          e.preventDefault();
          this.booster.setVolume(this.booster.getState().percentage + 10);
          break;
        case 'ArrowDown':
          e.preventDefault();
          this.booster.setVolume(this.booster.getState().percentage - 10);
          break;
        case 'm':
        case 'M':
          e.preventDefault();
          this.booster.toggleMute();
          break;
        case 'f':
        case 'F':
          e.preventDefault();
          this.toggleFullscreen();
          break;
        case '0': case '1': case '2': case '3': case '4':
        case '5': case '6': case '7': case '8': case '9':
          e.preventDefault();
          const fraction = parseInt(e.key, 10) / 10.0;
          this.seek((this.video.duration || 0) * fraction);
          break;
      }
    });
  }

  /**
   * Formats seconds into MM:SS or HH:MM:SS.
   */
  private formatTime(seconds: number): string {
    if (isNaN(seconds) || seconds < 0) return '00:00';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);

    const pad = (n: number) => n.toString().padStart(2, '0');
    if (h > 0) {
      return `${pad(h)}:${pad(m)}:${pad(s)}`;
    }
    return `${pad(m)}:${pad(s)}`;
  }

  private updateTimeDisplay() {
    const cur = this.video.currentTime || 0;
    const dur = this.video.duration || 0;
    const curEl = this.timeDisplay.querySelector('.time-current')!;
    const durEl = this.timeDisplay.querySelector('.time-duration')!;
    curEl.textContent = this.formatTime(cur);
    durEl.textContent = this.formatTime(dur);

    this.updateScrubberUI(cur);
  }

  private updateScrubberUI(currentTime: number) {
    const dur = this.video.duration || 1;
    const pct = Math.max(0, Math.min(100, (currentTime / dur) * 100));
    this.scrubberProgress.style.width = `${pct}%`;
    this.scrubberThumb.style.left = `${pct}%`;
  }

  private updateBufferedBar() {
    const dur = this.video.duration;
    if (!dur || !this.video.buffered.length) return;
    const end = this.video.buffered.end(this.video.buffered.length - 1);
    const pct = Math.min(100, (end / dur) * 100);
    this.scrubberBuffered.style.width = `${pct}%`;
  }

  private resetIdleTimer() {
    if (this.idleTimeoutId !== null) {
      window.clearTimeout(this.idleTimeoutId);
    }
    this.container.classList.remove('controls-idle');

    if (!this.video.paused) {
      this.idleTimeoutId = window.setTimeout(() => {
        this.container.classList.add('controls-idle');
      }, 2500);
    }
  }

  public seek(seconds: number) {
    const target = Math.max(0, Math.min(this.video.duration || 0, seconds));
    this.video.currentTime = target;
    this.updateScrubberUI(target);

    if (!this.isRemoteAction && this.options.onSeek) {
      this.options.onSeek(target);
    }
  }

  public setPlaybackRate(rate: number) {
    this.video.playbackRate = rate;
    this.playbackRateBtn.textContent = `${rate}x`;
  }

  public toggleFullscreen() {
    if (!document.fullscreenElement) {
      this.container.requestFullscreen().catch(console.warn);
    } else {
      document.exitFullscreen().catch(console.warn);
    }
  }

  public loadSource(url: string) {
    this.video.src = url;
    this.video.load();
  }

  // =========================================================================
  // Watch Party Remote Synchronizer APIs
  // =========================================================================
  public applyRemotePlay(timestamp: number) {
    this.isRemoteAction = true;
    this.video.currentTime = timestamp;
    this.video.play().finally(() => {
      this.isRemoteAction = false;
    });
  }

  public applyRemotePause(timestamp: number) {
    this.isRemoteAction = true;
    this.video.currentTime = timestamp;
    this.video.pause();
    this.isRemoteAction = false;
  }

  public applyRemoteSeek(timestamp: number) {
    this.isRemoteAction = true;
    this.video.currentTime = timestamp;
    this.isRemoteAction = false;
  }

  public applyRemoteRate(rate: number) {
    this.video.playbackRate = rate;
  }

  public getVideoElement(): HTMLVideoElement {
    return this.video;
  }

  public getBooster(): AudioBoosterManager {
    return this.booster;
  }

  public destroy() {
    if (this.idleTimeoutId !== null) {
      window.clearTimeout(this.idleTimeoutId);
    }
    if (this.chatListener) {
      window.removeEventListener('yourant:wp-chat', this.chatListener);
      this.chatListener = null;
    }
    partySync.detachVideo();
    this.video.pause();
    this.video.src = '';
    this.container.innerHTML = '';
  }
}
