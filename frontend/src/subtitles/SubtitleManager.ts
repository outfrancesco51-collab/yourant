/**
 * SubtitleManager.ts - Unified Subtitle Controller for Yourant Anime Player
 * Orchestrates JassubRenderer (ASS/SSA, VTT, SRT) and PgsRenderer (Blu-ray PGS),
 * managing active track selection, subtitle delay, and video synchronization.
 * Parity with Seanime VideoCoreSubtitleManager.
 */

import { PgsRenderer } from './PgsRenderer';
import { JassubRenderer } from './JassubRenderer';
import { SubtitleTrack, PgsEvent } from './types';

export interface SubtitleManagerEvents {
  trackchange: CustomEvent<{ track: SubtitleTrack | null }>;
  trackschange: CustomEvent<{ tracks: SubtitleTrack[] }>;
  delaychange: CustomEvent<{ delay: number }>;
}

export class SubtitleManager extends EventTarget {
  private video: HTMLVideoElement;
  private container: HTMLElement;
  private overlayContainer: HTMLElement;

  private pgsRenderer: PgsRenderer | null = null;
  private jassubRenderer: JassubRenderer | null = null;

  private tracks: SubtitleTrack[] = [];
  private currentTrack: SubtitleTrack | null = null;
  private delay: number = 0; // seconds

  // AI Subtitles Translation System
  public aiTranslationEnabled: boolean = false;
  public targetLanguage: string = 'en';

  constructor(video: HTMLVideoElement, container: HTMLElement) {
    super();
    this.video = video;
    this.container = container;

    // Create or locate the subtitle overlay container
    let existingOverlay = this.container.querySelector('.player-subtitle-overlay') as HTMLElement;
    if (!existingOverlay) {
      existingOverlay = document.createElement('div');
      existingOverlay.className = 'player-subtitle-overlay';
      // Insert right after the video element
      if (this.video.nextSibling) {
        this.container.insertBefore(existingOverlay, this.video.nextSibling);
      } else {
        this.container.appendChild(existingOverlay);
      }
    }
    this.overlayContainer = existingOverlay;
  }

  /**
   * Sets the full list of subtitle tracks.
   * Auto-selects the default track if specified and no track is active.
   */
  public async setTracks(tracks: SubtitleTrack[]): Promise<void> {
    this.tracks = [...tracks];

    this.dispatchEvent(new CustomEvent('trackschange', {
      detail: { tracks: this.tracks }
    }));

    if (!this.currentTrack) {
      const defaultTrack = this.tracks.find(t => t.default) || (this.tracks.length > 0 ? this.tracks[0] : null);
      if (defaultTrack) {
        await this.selectTrack(defaultTrack.id);
      }
    } else {
      // Re-validate current track exists in new list
      const stillExists = this.tracks.some(t => t.id === this.currentTrack!.id);
      if (!stillExists) {
        await this.selectTrack(null);
      }
    }
  }

  /**
   * Adds a single subtitle track dynamically.
   */
  public async addTrack(track: SubtitleTrack, select: boolean = false): Promise<void> {
    // Remove if already exists with same id
    this.tracks = this.tracks.filter(t => t.id !== track.id);
    this.tracks.push(track);

    this.dispatchEvent(new CustomEvent('trackschange', {
      detail: { tracks: this.tracks }
    }));

    if (select || (!this.currentTrack && track.default)) {
      await this.selectTrack(track.id);
    }
  }

  /**
   * Selects an active track by ID, or null to disable subtitles.
   */
  public async selectTrack(id: string | number | null): Promise<void> {
    if (id === null || id === undefined || id === -1) {
      await this.disableSubtitles();
      return;
    }

    const track = this.tracks.find(t => t.id === id);
    if (!track) {
      console.warn('[SUBTITLE MANAGER] Track not found for id:', id);
      await this.disableSubtitles();
      return;
    }

    this.currentTrack = track;

    if (track.type === 'pgs') {
      // Deactivate ASS/Jassub
      if (this.jassubRenderer) {
        await this.jassubRenderer.clear();
      }

      // Initialize PGS renderer if needed
      if (!this.pgsRenderer) {
        this.pgsRenderer = new PgsRenderer({
          videoElement: this.video,
          containerElement: this.overlayContainer,
          workerUrl: '/pgs-renderer.worker.js',
        });
      }

      this.pgsRenderer.clear();
      this.pgsRenderer.setTimeOffset(-this.delay);

      if (track.events && track.events.length > 0) {
        this.pgsRenderer.addEvents(track.events);
      } else if (track.src) {
        // If external PGS track URL given, fetch and parse if JSON
        try {
          const resp = await fetch(track.src);
          const data = await resp.json();
          if (Array.isArray(data)) {
            this.pgsRenderer.addEvents(data as PgsEvent[]);
          }
        } catch (err) {
          console.warn('[SUBTITLE MANAGER] Failed to fetch external PGS events:', err);
        }
      }

      this.pgsRenderer.resize();
    } else {
      // ASS, SSA, VTT, or SRT
      if (this.pgsRenderer) {
        this.pgsRenderer.clear();
      }

      if (!this.jassubRenderer) {
        this.jassubRenderer = new JassubRenderer({
          videoElement: this.video,
          containerElement: this.overlayContainer,
          workerUrl: '/jassub/jassub-worker.js',
          wasmUrl: '/jassub/jassub-worker.wasm',
          modernWasmUrl: '/jassub/jassub-worker-modern.wasm',
          defaultFont: 'roboto medium',
          defaultFontUrl: '/fonts/Roboto-Medium.ttf',
        });
      }

      this.jassubRenderer.setTimeOffset(-this.delay);

      let content = track.content || '';

      if (!content && track.src) {
        try {
          const resp = await fetch(track.src);
          content = await resp.text();
        } catch (err) {
          console.error('[SUBTITLE MANAGER] Failed to fetch subtitle file:', track.src, err);
        }
      }

      if (track.type === 'vtt' || track.type === 'srt') {
        content = this.convertVttOrSrtToAss(content);
      }

      if (content) {
        if (this.aiTranslationEnabled) {
          content = await this.translateSubtitles(content, this.targetLanguage);
        }
        await this.jassubRenderer.setTrack(content);
      } else {
        await this.jassubRenderer.clear();
      }

      await this.jassubRenderer.resize();
    }

    this.dispatchEvent(new CustomEvent('trackchange', {
      detail: { track: this.currentTrack }
    }));
  }

  /**
   * Disables active subtitle rendering.
   */
  public async disableSubtitles(): Promise<void> {
    this.currentTrack = null;
    if (this.pgsRenderer) {
      this.pgsRenderer.clear();
    }
    if (this.jassubRenderer) {
      await this.jassubRenderer.clear();
    }

    this.dispatchEvent(new CustomEvent('trackchange', {
      detail: { track: null }
    }));
  }

  /**
   * Cycles to the next available subtitle track, or OFF.
   */
  public async cycleTrack(): Promise<SubtitleTrack | null> {
    if (this.tracks.length === 0) {
      await this.disableSubtitles();
      return null;
    }

    if (!this.currentTrack) {
      // Currently OFF -> pick first track
      await this.selectTrack(this.tracks[0].id);
      return this.currentTrack;
    }

    const currentIndex = this.tracks.findIndex(t => t.id === this.currentTrack!.id);
    if (currentIndex >= 0 && currentIndex < this.tracks.length - 1) {
      // Pick next track
      await this.selectTrack(this.tracks[currentIndex + 1].id);
      return this.currentTrack;
    } else {
      // Was on last track -> turn OFF
      await this.disableSubtitles();
      return null;
    }
  }

  /**
   * Adjusts subtitle timing delay in seconds (+ delay displays earlier, - delay displays later).
   */
  public setDelay(seconds: number): void {
    this.delay = Math.round(seconds * 10) / 10;
    if (this.pgsRenderer) {
      this.pgsRenderer.setTimeOffset(-this.delay);
    }
    if (this.jassubRenderer) {
      this.jassubRenderer.setTimeOffset(-this.delay);
    }

    this.dispatchEvent(new CustomEvent('delaychange', {
      detail: { delay: this.delay }
    }));
  }

  public getDelay(): number {
    return this.delay;
  }

  public getTracks(): SubtitleTrack[] {
    return [...this.tracks];
  }

  public getCurrentTrack(): SubtitleTrack | null {
    return this.currentTrack;
  }

  public getPgsRenderer(): PgsRenderer | null {
    return this.pgsRenderer;
  }

  public getJassubRenderer(): JassubRenderer | null {
    return this.jassubRenderer;
  }

  public resize(): void {
    if (this.pgsRenderer) {
      this.pgsRenderer.resize();
    }
    if (this.jassubRenderer) {
      this.jassubRenderer.resize().catch(() => {});
    }
  }

  public destroy(): void {
    if (this.pgsRenderer) {
      this.pgsRenderer.destroy();
      this.pgsRenderer = null;
    }
    if (this.jassubRenderer) {
      this.jassubRenderer.destroy();
      this.jassubRenderer = null;
    }
    if (this.overlayContainer && this.overlayContainer.parentElement) {
      this.overlayContainer.parentElement.removeChild(this.overlayContainer);
    }
    this.tracks = [];
    this.currentTrack = null;
  }

  /**
   * Enables real-time AI subtitle translation.
   */
  public enableAITranslation(lang: string = 'en'): void {
    this.aiTranslationEnabled = true;
    this.targetLanguage = lang;
    if (this.currentTrack) {
      this.selectTrack(this.currentTrack.id); // Reload track to apply translation
    }
  }

  public disableAITranslation(): void {
    this.aiTranslationEnabled = false;
    if (this.currentTrack) {
      this.selectTrack(this.currentTrack.id);
    }
  }

  /**
   * AI Translation wrapper that intercepts parsed ASS lines.
   */
  private async translateSubtitles(content: string, targetLanguage: string): Promise<string> {
    const lines = content.split('\n');
    const translatedLines = await Promise.all(lines.map(async (line) => {
      if (line.trim().startsWith('Dialogue:')) {
        const parts = line.split(',');
        if (parts.length >= 10) {
          const textIndex = 9;
          const text = parts.slice(textIndex).join(',');
          
          // Mock API call to translator
          // In a real implementation, this would batch lines and call an LLM/DeepL API
          const translatedText = `[AI ${targetLanguage.toUpperCase()}] ${text}`;
          
          return [...parts.slice(0, textIndex), translatedText].join(',');
        }
      }
      return line;
    }));
    return translatedLines.join('\n');
  }

  /**
   * Converts WebVTT or SubRip (.srt) subtitles to ASS script.
   */
  private convertVttOrSrtToAss(content: string): string {
    const lines = content.replace(/\r\n/g, '\n').replace(/\r/g, '\n').split('\n');
    const dialogues: string[] = [];

    const timeRegex = /(?:(\d{1,2}):)?(\d{2}):(\d{2})[.,](\d{2,3})\s*-->\s*(?:(\d{1,2}):)?(\d{2}):(\d{2})[.,](\d{2,3})/;

    let inCue = false;
    let startAss = '';
    let endAss = '';
    let textBuffer: string[] = [];

    const flushCue = () => {
      if (startAss && endAss && textBuffer.length > 0) {
        const text = textBuffer
          .join('\\N')
          .replace(/<[^>]+>/g, '') // strip HTML/WebVTT tags
          .trim();
        if (text) {
          dialogues.push(`Dialogue: 0,${startAss},${endAss},Default,,0,0,0,,${text}`);
        }
      }
      startAss = '';
      endAss = '';
      textBuffer = [];
      inCue = false;
    };

    const formatAssTime = (h: string | undefined, m: string, s: string, ms: string) => {
      const hours = parseInt(h || '0', 10);
      const centis = Math.floor(parseInt(ms.padEnd(3, '0').substring(0, 3), 10) / 10);
      return `${hours}:${m.padStart(2, '0')}:${s.padStart(2, '0')}.${String(centis).padStart(2, '0')}`;
    };

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i].trim();

      const match = line.match(timeRegex);
      if (match) {
        flushCue();
        startAss = formatAssTime(match[1], match[2], match[3], match[4]);
        endAss = formatAssTime(match[5], match[6], match[7], match[8]);
        inCue = true;
        continue;
      }

      if (inCue) {
        if (line === '') {
          flushCue();
        } else if (!/^\d+$/.test(line)) {
          // Exclude SRT sequence numbers
          textBuffer.push(line);
        }
      }
    }
    flushCue();

    return JassubRenderer.DEFAULT_HEADER + dialogues.join('\n') + '\n';
  }
}
