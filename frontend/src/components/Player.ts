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
import { SubtitleManager } from '../subtitles/SubtitleManager';
import { SubtitleTrack } from '../subtitles/types';
import '../subtitles/subtitles.css';

export interface PlayerOptions {
  container: HTMLElement;
  src?: string;
  title?: string;
  poster?: string;
  cinemaManager?: CinemaModeManager;
  subtitleTracks?: SubtitleTrack[];
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

  // Subtitle Subsystem
  private subtitleManager!: SubtitleManager;
  private subtitleBtn!: HTMLButtonElement;
  private subtitleBadge!: HTMLElement;
  private subtitleMenu!: HTMLElement;
  private subtitleTracksContainer!: HTMLElement;
  private subtitleDelayDisplay!: HTMLElement;
  private isSubtitleMenuOpen: boolean = false;

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
    this.initSubtitleManager();
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

            <!-- Subtitle Toggle / Picker Button -->
            <button class="ctrl-btn btn-subtitles" title="Subtitles (S)" aria-label="Subtitles">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d="M19 4H5c-1.11 0-2 .9-2 2v12c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm-8 7H9.5v-.5h-2v3h2V13H11v1c0 .55-.45 1-1 1H7c-.55 0-1-.45-1-1v-4c0-.55.45-1 1-1h3c.55 0 1 .45 1 1v1zm7 0h-1.5v-.5h-2v3h2V13H18v1c0 .55-.45 1-1 1h-3c-.55 0-1-.45-1-1v-4c0-.55.45-1 1-1h3c.55 0 1 .45 1 1v1z"/>
              </svg>
              <span class="sub-track-badge">OFF</span>
            </button>

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

      <!-- Subtitle Menu Drawer / Popup -->
      <div class="player-subtitle-menu hidden">
        <div class="sub-menu-header">
          <div class="sub-menu-title">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
              <path d="M19 4H5c-1.11 0-2 .9-2 2v12c0 1.1.89 2 2 2h14c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm-8 7H9.5v-.5h-2v3h2V13H11v1c0 .55-.45 1-1 1H7c-.55 0-1-.45-1-1v-4c0-.55.45-1 1-1h3c.55 0 1 .45 1 1v1zm7 0h-1.5v-.5h-2v3h2V13H18v1c0 .55-.45 1-1 1h-3c-.55 0-1-.45-1-1v-4c0-.55.45-1 1-1h3c.55 0 1 .45 1 1v1z"/>
            </svg>
            <span>Subtitles</span>
          </div>
          <button class="btn-close-sub-menu" aria-label="Close subtitle menu">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
              <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
            </svg>
          </button>
        </div>
        <div class="sub-menu-tracks"></div>
        <div class="sub-menu-delay">
          <div class="sub-delay-label-row">
            <span>Subtitle Delay:</span>
            <span class="sub-delay-val">0.0s</span>
          </div>
          <div class="sub-delay-controls">
            <button class="btn-sub-delay btn-delay-minus">-0.1s</button>
            <button class="btn-sub-delay btn-delay-plus">+0.1s</button>
            <button class="btn-sub-delay-reset">Reset</button>
          </div>
        </div>
        <div class="sub-menu-upload">
          <label class="btn-sub-upload">
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
              <path d="M9 16h6v-6h4l-7-7-7 7h4zm-4 2h14v2H5z"/>
            </svg>
            <span>Load Subtitle File</span>
            <input type="file" class="sub-file-input" accept=".ass,.ssa,.srt,.vtt" />
          </label>
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
    this.subtitleBtn = this.container.querySelector('.btn-subtitles')!;
    this.subtitleBadge = this.container.querySelector('.sub-track-badge')!;
    this.subtitleMenu = this.container.querySelector('.player-subtitle-menu')!;
    this.subtitleTracksContainer = this.container.querySelector('.sub-menu-tracks')!;
    this.subtitleDelayDisplay = this.container.querySelector('.sub-delay-val')!;
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
      this.subtitleManager.resize();
    });

    // 6b. Subtitle Controls & Menu
    this.subtitleBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      this.toggleSubtitleMenu();
    });

    this.subtitleMenu.querySelector('.btn-close-sub-menu')?.addEventListener('click', (e) => {
      e.stopPropagation();
      this.closeSubtitleMenu();
    });

    this.subtitleMenu.querySelector('.btn-delay-minus')?.addEventListener('click', (e) => {
      e.stopPropagation();
      this.subtitleManager.setDelay(this.subtitleManager.getDelay() - 0.1);
    });

    this.subtitleMenu.querySelector('.btn-delay-plus')?.addEventListener('click', (e) => {
      e.stopPropagation();
      this.subtitleManager.setDelay(this.subtitleManager.getDelay() + 0.1);
    });

    this.subtitleMenu.querySelector('.btn-sub-delay-reset')?.addEventListener('click', (e) => {
      e.stopPropagation();
      this.subtitleManager.setDelay(0);
    });

    const fileInput = this.subtitleMenu.querySelector('.sub-file-input') as HTMLInputElement | null;
    fileInput?.addEventListener('change', () => {
      const file = fileInput.files?.[0];
      if (!file) return;
      const reader = new FileReader();
      reader.onload = async (ev) => {
        const text = ev.target?.result as string;
        if (!text) return;
        const ext = file.name.split('.').pop()?.toLowerCase() || 'ass';
        const type = (ext === 'srt' ? 'srt' : ext === 'vtt' ? 'vtt' : 'ass');
        const track: SubtitleTrack = {
          id: `custom-${Date.now()}`,
          label: file.name,
          language: 'custom',
          type,
          content: text,
        };
        await this.subtitleManager.addTrack(track, true);
        fileInput.value = '';
      };
      reader.readAsText(file);
    });

    document.addEventListener('click', (e) => {
      if (this.isSubtitleMenuOpen && !this.subtitleMenu.contains(e.target as Node) && !this.subtitleBtn.contains(e.target as Node)) {
        this.closeSubtitleMenu();
      }
    });

    window.addEventListener('resize', () => {
      this.subtitleManager.resize();
    });

    this.video.addEventListener('loadedmetadata', () => {
      this.subtitleManager.resize();
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
        case 's':
        case 'S':
          e.preventDefault();
          this.cycleSubtitles();
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

  public loadSource(url: string, subtitleTracks?: SubtitleTrack[]) {
    this.video.src = url;
    this.video.load();
    if (subtitleTracks && subtitleTracks.length > 0) {
      this.subtitleManager.setTracks(subtitleTracks);
    }
  }

  // =========================================================================
  // Subtitle Subsystem Helper APIs
  // =========================================================================
  private initSubtitleManager() {
    this.subtitleManager = new SubtitleManager(this.video, this.container);

    this.subtitleManager.addEventListener('trackchange', () => {
      this.updateSubtitleUI();
    });

    this.subtitleManager.addEventListener('trackschange', () => {
      this.renderSubtitleTracksList();
    });

    this.subtitleManager.addEventListener('delaychange', (e: Event) => {
      const customEv = e as CustomEvent<{ delay: number }>;
      if (this.subtitleDelayDisplay) {
        const d = customEv.detail.delay;
        this.subtitleDelayDisplay.textContent = `${d >= 0 ? '+' : ''}${d.toFixed(1)}s`;
      }
    });

    const tracks = this.options.subtitleTracks || this.getDefaultDemoTracks();
    this.subtitleManager.setTracks(tracks);
  }

  public getSubtitleManager(): SubtitleManager {
    return this.subtitleManager;
  }

  public setSubtitleTracks(tracks: SubtitleTrack[]) {
    this.subtitleManager.setTracks(tracks);
  }

  public async cycleSubtitles(): Promise<void> {
    const nextTrack = await this.subtitleManager.cycleTrack();
    this.updateSubtitleUI();
    this.showSubtitleHudNotification(nextTrack ? nextTrack.label : 'Subtitles Off');
  }

  public toggleSubtitleMenu(): void {
    if (this.isSubtitleMenuOpen) {
      this.closeSubtitleMenu();
    } else {
      this.openSubtitleMenu();
    }
  }

  public openSubtitleMenu(): void {
    this.isSubtitleMenuOpen = true;
    this.subtitleMenu.classList.remove('hidden');
    this.renderSubtitleTracksList();
  }

  public closeSubtitleMenu(): void {
    this.isSubtitleMenuOpen = false;
    this.subtitleMenu.classList.add('hidden');
  }

  private updateSubtitleUI(): void {
    const current = this.subtitleManager.getCurrentTrack();
    if (current) {
      this.subtitleBtn.classList.add('active');
      const tag = current.language ? current.language.toUpperCase() : (current.type === 'pgs' ? 'PGS' : 'SUB');
      this.subtitleBadge.textContent = tag.substring(0, 3);
    } else {
      this.subtitleBtn.classList.remove('active');
      this.subtitleBadge.textContent = 'OFF';
    }
    this.renderSubtitleTracksList();
  }

  private renderSubtitleTracksList(): void {
    if (!this.subtitleTracksContainer) return;
    this.subtitleTracksContainer.innerHTML = '';

    const current = this.subtitleManager.getCurrentTrack();
    const tracks = this.subtitleManager.getTracks();

    // Off option
    const offItem = document.createElement('button');
    offItem.className = `sub-track-item ${!current ? 'selected' : ''}`;
    offItem.innerHTML = `
      <div class="sub-track-item-left">
        <span class="sub-track-radio"></span>
        <span class="sub-track-name">Off</span>
      </div>
      <span class="sub-track-tag">None</span>
    `;
    offItem.addEventListener('click', async (e) => {
      e.stopPropagation();
      await this.subtitleManager.selectTrack(null);
    });
    this.subtitleTracksContainer.appendChild(offItem);

    // Track items
    tracks.forEach((track) => {
      const isSelected = current && current.id === track.id;
      const item = document.createElement('button');
      item.className = `sub-track-item ${isSelected ? 'selected' : ''}`;
      const typeLabel = track.type.toUpperCase();
      item.innerHTML = `
        <div class="sub-track-item-left">
          <span class="sub-track-radio"></span>
          <span class="sub-track-name">${this.escapeHtml(track.label)}</span>
        </div>
        <span class="sub-track-tag">${typeLabel}</span>
      `;
      item.addEventListener('click', async (e) => {
        e.stopPropagation();
        await this.subtitleManager.selectTrack(track.id);
      });
      this.subtitleTracksContainer.appendChild(item);
    });
  }

  private showSubtitleHudNotification(text: string): void {
    let hud = this.container.querySelector('.player-sub-hud-toast') as HTMLElement;
    if (!hud) {
      hud = document.createElement('div');
      hud.className = 'player-sub-hud-toast';
      hud.style.cssText = 'position: absolute; top: 20px; left: 50%; transform: translateX(-50%); background: rgba(15,23,42,0.85); backdrop-filter: blur(10px); color: #38bdf8; border: 1px solid rgba(56,189,248,0.3); border-radius: 6px; padding: 6px 16px; font-size: 13px; font-weight: 600; pointer-events: none; z-index: 30; transition: opacity 0.3s ease; box-shadow: 0 4px 16px rgba(0,0,0,0.6);';
      this.container.appendChild(hud);
    }
    hud.textContent = text;
    hud.style.opacity = '1';
    setTimeout(() => {
      if (hud) hud.style.opacity = '0';
    }, 1500);
  }

  private getDefaultDemoTracks(): SubtitleTrack[] {
    return [
      {
        id: 'demo-ass',
        label: 'English [ASS Fansub] (Worker)',
        language: 'en',
        type: 'ass',
        default: false,
        content: `[Script Info]
Title: Yourant Demo ASS
ScriptType: v4.00+
WrapStyle: 0
PlayResX: 1920
PlayResY: 1080
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Roboto Medium,52,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2.5,1,2,30,30,30,0
Style: BloodRed,Roboto Medium,56,&H001D1DE1,&H000000FF,&H00F8BD38,&H00000000,1,0,0,0,100,100,0,0,1,3.0,1,2,30,30,30,0

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:06.00,Default,,0,0,0,,{\\b1}Welcome to Yourant Streaming{\\b0}\\NPowered by \\c&H38BDF8&jassub-worker.js\\c&HFFFFFF& (WASM libass)
Dialogue: 0,0:00:06.50,0:00:13.00,BloodRed,,0,0,0,,Metallic Blue & Blood Red Theme Active
Dialogue: 0,0:00:13.50,0:00:20.00,Default,,0,0,0,,Volume Booster graph up to {\\b1}1000%{\\b0} available
Dialogue: 0,0:00:20.50,0:00:30.00,Default,,0,0,0,,Press {\\b1}S{\\b0} to cycle subtitle tracks
`
      },
      {
        id: 'demo-pgs',
        label: 'English [Blu-ray PGS] (Worker)',
        language: 'pgs',
        type: 'pgs',
        default: false,
        events: [
          {
            startTime: 1.0,
            duration: 6.0,
            imageData: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAJYAAAAwCAYAAAD7G8b8AAAAAXNSR0IArs4c6QAAAARnQU1BAACxjwv8YQUAAAAJcEhZcwAADsMAAA7DAcdvqGQAAAA8SURBVHhe7cExAQAAAMKg9U9tCy8gAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAOBqBiIAAZv17t4AAAAASUVORK5CYII=',
            width: 150,
            height: 48,
            x: 245,
            y: 300,
            canvasWidth: 640,
            canvasHeight: 360,
          }
        ]
      }
    ];
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
    this.subtitleManager.destroy();
    partySync.detachVideo();
    this.video.pause();
    this.video.src = '';
    this.container.innerHTML = '';
  }
}
