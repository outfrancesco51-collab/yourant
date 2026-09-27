/**
 * JassubRenderer.ts - libass WebAssembly ASS/SSA Subtitle Renderer
 * Bridges to /jassub/jassub-worker.js using the Comlink/abslink RPC protocol.
 * Parity with Seanime ASSRenderer & JASSUB integration.
 */

import { SubtitleStyle, VideoContentSize } from './types';

export interface JassubRendererOptions {
  videoElement: HTMLVideoElement;
  containerElement?: HTMLElement;
  workerUrl?: string;
  wasmUrl?: string;
  modernWasmUrl?: string;
  defaultFont?: string;
  defaultFontUrl?: string;
  debug?: boolean;
}

export class JassubRenderer {
  private videoElement: HTMLVideoElement;
  private containerElement: HTMLElement;
  private workerUrl: string;
  private wasmUrl: string;
  private modernWasmUrl: string;
  private defaultFont: string;
  private defaultFontUrl: string;
  private debug: boolean;

  private canvas: HTMLCanvasElement | null = null;
  private worker: Worker | null = null;
  private readyPromise: Promise<void> | null = null;
  private isReady: boolean = false;
  private isDestroyed: boolean = false;
  private animationFrameId: number | null = null;
  private resizeObserver: ResizeObserver | null = null;

  // RPC State
  private instanceMarkerID: string | null = null;
  private pendingRequests: Map<string, { resolve: (val: any) => void; reject: (err: any) => void }> = new Map();
  private timeOffset: number = 0;
  private canvasWidth: number = 0;
  private canvasHeight: number = 0;
  private currentTrackContent: string | null = null;

  // Bound event handlers for clean removal
  private boundOnPlay: () => void;
  private boundOnPause: () => void;
  private boundOnSeeking: () => void;
  private boundOnSeeked: () => void;
  private boundOnTimeUpdate: () => void;

  public static readonly DEFAULT_HEADER = `[Script Info]
Title: Yourant Subtitles
ScriptType: v4.00+
WrapStyle: 0
PlayResX: 1920
PlayResY: 1080
ScaledBorderAndShadow: yes

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Roboto Medium,55,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2.5,1,2,30,30,30,0

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
`;

  constructor(options: JassubRendererOptions) {
    this.videoElement = options.videoElement;
    this.containerElement = options.containerElement || this.videoElement.parentElement || document.body;
    this.workerUrl = options.workerUrl || '/jassub/jassub-worker.js';
    this.wasmUrl = options.wasmUrl || '/jassub/jassub-worker.wasm';
    this.modernWasmUrl = options.modernWasmUrl || '/jassub/jassub-worker-modern.wasm';
    this.defaultFont = options.defaultFont || 'roboto medium';
    this.defaultFontUrl = options.defaultFontUrl || '/fonts/Roboto-Medium.ttf';
    this.debug = options.debug ?? false;

    this.boundOnPlay = () => this.drawFrame(false);
    this.boundOnPause = () => this.drawFrame(true);
    this.boundOnSeeking = () => this.drawFrame(true);
    this.boundOnSeeked = () => this.drawFrame(true);
    this.boundOnTimeUpdate = () => this.drawFrame(false);

    this.setupCanvas();
    this.readyPromise = this.initWorker();
    this.setupVideoEvents();
    this.setupResizeObserver();
    this.startRenderLoop();
  }

  public get ready(): Promise<void> {
    return this.readyPromise || Promise.resolve();
  }

  public getTrackContent(): string | null {
    return this.currentTrackContent;
  }

  public getDimensions(): { width: number; height: number } {
    return { width: this.canvasWidth, height: this.canvasHeight };
  }

  public async setTrack(content: string): Promise<void> {
    await this.ready;
    this.currentTrackContent = content;
    if (!this.instanceMarkerID) return;

    await this.invoke('setTrack', [content]);
    await this.resize();
    this.drawFrame(true);
  }

  public async setTrackByUrl(url: string): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;

    try {
      const resp = await fetch(url);
      const text = await resp.text();
      await this.setTrack(text);
    } catch (err) {
      if (this.debug) console.warn('[JASSUB] Falling back to worker setTrackByUrl', err);
      await this.invoke('setTrackByUrl', [url]);
      await this.resize();
      this.drawFrame(true);
    }
  }

  public async createEvent(event: any): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;
    await this.invoke('createEvent', [event]);
    this.drawFrame(false);
  }

  public async styleOverride(style: SubtitleStyle): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;
    await this.invoke('styleOverride', [style]);
    this.drawFrame(true);
  }

  public async disableStyleOverride(): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;
    await this.invoke('disableStyleOverride', []);
    this.drawFrame(true);
  }

  public async setDefaultFont(fontName: string): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;
    await this.invoke('setDefaultFont', [fontName]);
  }

  public async addFonts(fonts: string[]): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;
    await this.invoke('addFonts', [fonts]);
  }

  public setTimeOffset(offset: number) {
    this.timeOffset = offset;
    this.drawFrame(true);
  }

  public async clear(): Promise<void> {
    await this.ready;
    if (!this.instanceMarkerID) return;
    this.currentTrackContent = null;
    await this.invoke('setTrack', [JassubRenderer.DEFAULT_HEADER]);
    this.drawFrame(true);
  }

  public async resize(): Promise<void> {
    if (!this.canvas) return;

    const size = this.getRenderedVideoContentSize(
      this.videoElement.clientWidth,
      this.videoElement.clientHeight
    );
    if (!size) return;

    const { displayedWidth, displayedHeight, offsetX, offsetY } = size;
    this.canvasWidth = displayedWidth;
    this.canvasHeight = displayedHeight;

    this.canvas.style.width = `${displayedWidth}px`;
    this.canvas.style.height = `${displayedHeight}px`;
    this.canvas.style.left = `${offsetX}px`;
    this.canvas.style.top = `${offsetY}px`;

    if (this.instanceMarkerID) {
      const vw = this.videoElement.videoWidth || displayedWidth;
      const vh = this.videoElement.videoHeight || displayedHeight;
      await this.invoke('_resizeCanvas', [displayedWidth, displayedHeight, vw, vh]);
      this.drawFrame(true);
    }
  }

  public destroy() {
    this.isDestroyed = true;

    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }

    this.removeVideoEvents();

    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }

    if (this.worker) {
      if (this.instanceMarkerID) {
        try {
          this.invoke('freeTrack', []).catch(() => {});
        } catch {
          // ignore
        }
      }
      this.worker.terminate();
      this.worker = null;
    }

    if (this.canvas && this.canvas.parentElement) {
      this.canvas.parentElement.removeChild(this.canvas);
    }

    this.canvas = null;
    this.pendingRequests.clear();
  }

  private setupCanvas() {
    this.canvas = document.createElement('canvas');
    this.canvas.className = 'vc-jassub-canvas';
    this.canvas.style.position = 'absolute';
    this.canvas.style.pointerEvents = 'none';
    this.canvas.style.zIndex = '10';
    this.canvas.style.objectFit = 'contain';
    this.canvas.style.objectPosition = 'center';

    if (getComputedStyle(this.containerElement).position === 'static') {
      this.containerElement.style.position = 'relative';
    }
    this.containerElement.appendChild(this.canvas);

    const size = this.getRenderedVideoContentSize(
      this.videoElement.clientWidth,
      this.videoElement.clientHeight
    );
    if (size) {
      const { displayedWidth, displayedHeight, offsetX, offsetY } = size;
      this.canvas.width = displayedWidth;
      this.canvas.height = displayedHeight;
      this.canvasWidth = displayedWidth;
      this.canvasHeight = displayedHeight;
      this.canvas.style.width = `${displayedWidth}px`;
      this.canvas.style.height = `${displayedHeight}px`;
      this.canvas.style.left = `${offsetX}px`;
      this.canvas.style.top = `${offsetY}px`;
    }
  }

  private async initWorker(): Promise<void> {
    if (!this.canvas) return;

    return new Promise<void>((resolve, reject) => {
      try {
        // Name must match 'jassub-worker' for jassub-worker.js to expose ASSRenderer via abslink
        const WorkerClass = typeof window !== 'undefined' ? window.Worker : (typeof Worker !== 'undefined' ? Worker : null);
        if (!WorkerClass) throw new Error('Worker not supported');
        this.worker = new WorkerClass(this.workerUrl, { name: 'jassub-worker' });

        this.worker.onmessage = (e: MessageEvent) => {
          this.handleWorkerMessage(e.data);
        };

        this.worker.onerror = (err) => {
          console.error('[JASSUB WORKER ERROR]', err);
          if (!this.isReady) {
            reject(err);
          }
        };

        const size = this.getRenderedVideoContentSize(
          this.videoElement.clientWidth,
          this.videoElement.clientHeight
        );
        const initialWidth = size?.displayedWidth || 640;
        const initialHeight = size?.displayedHeight || 360;

        let offscreenCanvas: OffscreenCanvas;
        if (this.canvas && typeof this.canvas.transferControlToOffscreen === 'function') {
          offscreenCanvas = this.canvas.transferControlToOffscreen();
        } else {
          // OffscreenCanvas fallback
          offscreenCanvas = new OffscreenCanvas(initialWidth, initialHeight);
        }

        const constructReqId = this.randomId();
        const initData = {
          wasmUrl: this.wasmUrl,
          modernWasmUrl: this.modernWasmUrl,
          defaultFont: this.defaultFont,
          availableFonts: {
            [this.defaultFont]: this.defaultFontUrl,
          },
          fonts: [this.defaultFontUrl],
          subContent: JassubRenderer.DEFAULT_HEADER,
          width: initialWidth,
          height: initialHeight,
          debug: this.debug,
          queryFonts: false,
        };

        this.pendingRequests.set(constructReqId, {
          resolve: (result: any) => {
            if (this.debug) console.log('[JASSUB] Constructed ASSRenderer proxy:', result);
            // Result is { type: "HANDLER", name: "proxy", value: markerID }
            if (result && typeof result === 'object' && result.value) {
              this.instanceMarkerID = result.value;
            } else if (typeof result === 'string') {
              this.instanceMarkerID = result;
            }
            this.isReady = true;
            resolve();
          },
          reject: (err: any) => {
            console.error('[JASSUB] Failed to construct ASSRenderer:', err);
            reject(err);
          },
        });

        // Send CONSTRUCT message to jassub-worker.js via abslink RPC protocol
        this.worker.postMessage(
          {
            id: constructReqId,
            type: 'CONSTRUCT',
            path: [],
            argumentList: [
              { type: 'RAW', value: initData },
              { type: 'RAW', value: null }, // getFont callback
              { type: 'RAW', value: offscreenCanvas },
            ],
          },
          [offscreenCanvas]
        );
      } catch (err) {
        console.error('[JASSUB] Init failed:', err);
        reject(err);
      }
    });
  }

  private handleWorkerMessage(data: any) {
    if (!data || !data.id) return;

    const pending = this.pendingRequests.get(data.id);
    if (pending) {
      this.pendingRequests.delete(data.id);
      if (data.type === 'HANDLER' && data.name === 'throw') {
        pending.reject(data.value);
      } else {
        pending.resolve(data);
      }
    }
  }

  private invoke(method: string, args: any[] = []): Promise<any> {
    if (!this.worker || !this.instanceMarkerID) {
      return Promise.resolve(null);
    }

    return new Promise((resolve, reject) => {
      const reqId = this.randomId();
      this.pendingRequests.set(reqId, { resolve, reject });

      const argumentList = args.map((arg) => ({
        type: 'RAW',
        value: arg,
      }));

      this.worker!.postMessage({
        id: reqId,
        type: 'APPLY',
        markerID: this.instanceMarkerID,
        path: [method],
        argumentList,
      });
    });
  }

  private drawFrame(repaint: boolean = false) {
    if (!this.isReady || !this.instanceMarkerID) return;

    const time = this.videoElement.currentTime + this.timeOffset;
    this.invoke('_draw', [time, repaint ? 1 : 0]).catch(() => {});
  }

  private setupVideoEvents() {
    this.videoElement.addEventListener('play', this.boundOnPlay);
    this.videoElement.addEventListener('pause', this.boundOnPause);
    this.videoElement.addEventListener('seeking', this.boundOnSeeking);
    this.videoElement.addEventListener('seeked', this.boundOnSeeked);
    this.videoElement.addEventListener('timeupdate', this.boundOnTimeUpdate);
  }

  private removeVideoEvents() {
    this.videoElement.removeEventListener('play', this.boundOnPlay);
    this.videoElement.removeEventListener('pause', this.boundOnPause);
    this.videoElement.removeEventListener('seeking', this.boundOnSeeking);
    this.videoElement.removeEventListener('seeked', this.boundOnSeeked);
    this.videoElement.removeEventListener('timeupdate', this.boundOnTimeUpdate);
  }

  private setupResizeObserver() {
    if (!this.videoElement || !this.canvas) return;

    let resizeRafId: number | null = null;
    this.resizeObserver = new ResizeObserver((entries) => {
      if (!this.canvas || !entries[0]) return;

      if (resizeRafId !== null) return;
      const { width, height } = entries[0].contentRect;

      resizeRafId = requestAnimationFrame(() => {
        resizeRafId = null;
        if (!this.canvas) return;

        const size = this.getRenderedVideoContentSize(width, height);
        if (!size) return;

        const { displayedWidth, displayedHeight, offsetX, offsetY } = size;
        this.canvasWidth = displayedWidth;
        this.canvasHeight = displayedHeight;

        this.canvas.style.width = `${displayedWidth}px`;
        this.canvas.style.height = `${displayedHeight}px`;
        this.canvas.style.left = `${offsetX}px`;
        this.canvas.style.top = `${offsetY}px`;

        if (this.instanceMarkerID) {
          const vw = this.videoElement.videoWidth || displayedWidth;
          const vh = this.videoElement.videoHeight || displayedHeight;
          this.invoke('_resizeCanvas', [displayedWidth, displayedHeight, vw, vh]).catch(() => {});
          this.drawFrame(true);
        }
      });
    });

    this.resizeObserver.observe(this.videoElement);
  }

  private startRenderLoop() {
    const render = () => {
      if (this.isDestroyed) return;

      if (!this.videoElement.paused || this.videoElement.seeking) {
        this.drawFrame(false);
      }

      this.animationFrameId = requestAnimationFrame(render);
    };

    render();
  }

  private getRenderedVideoContentSize(containerWidth: number, containerHeight: number): VideoContentSize | null {
    const videoWidth = this.videoElement.videoWidth;
    const videoHeight = this.videoElement.videoHeight;

    if (!videoWidth || !videoHeight || containerWidth === 0 || containerHeight === 0) {
      return {
        displayedWidth: containerWidth || 640,
        displayedHeight: containerHeight || 360,
        offsetX: 0,
        offsetY: 0,
      };
    }

    const containerRatio = containerWidth / containerHeight;
    const videoRatio = videoWidth / videoHeight;

    let displayedWidth: number;
    let displayedHeight: number;
    let offsetX = 0;
    let offsetY = 0;

    if (videoRatio > containerRatio) {
      displayedWidth = containerWidth;
      displayedHeight = containerWidth / videoRatio;
      offsetY = (containerHeight - displayedHeight) / 2;
    } else {
      displayedHeight = containerHeight;
      displayedWidth = containerHeight * videoRatio;
      offsetX = (containerWidth - displayedWidth) / 2;
    }

    return {
      displayedWidth: Math.round(displayedWidth),
      displayedHeight: Math.round(displayedHeight),
      offsetX: Math.round(offsetX),
      offsetY: Math.round(offsetY),
    };
  }

  private randomId(): string {
    return Math.random().toString(36).substring(2, 15) + Math.random().toString(36).substring(2, 15);
  }
}
