/**
 * PgsRenderer.ts - Blu-ray PGS (Presentation Graphic Stream) Subtitle Renderer
 * Communicates with /pgs-renderer.worker.js via OffscreenCanvas 2D context.
 * Parity with Seanime VideoCorePgsRenderer architecture.
 */

import { PgsEvent, VideoContentSize } from './types';

export interface PgsRendererOptions {
  videoElement: HTMLVideoElement;
  containerElement?: HTMLElement;
  workerUrl?: string;
  debug?: boolean;
}

export class PgsRenderer {
  private videoElement: HTMLVideoElement;
  private containerElement: HTMLElement;
  private workerUrl: string;
  private debug: boolean;

  private canvas: HTMLCanvasElement | null = null;
  private worker: Worker | null = null;
  private animationFrameId: number | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private isDestroyed: boolean = false;
  private canvasWidth: number = 0;
  private canvasHeight: number = 0;
  private timeOffset: number = 0;

  // Fallback for environments without OffscreenCanvas transfer support
  private fallbackCtx: CanvasRenderingContext2D | null = null;
  private fallbackEvents: Map<string, PgsEvent> = new Map();
  private fallbackImageCache: Map<string, HTMLImageElement> = new Map();
  private fallbackCurrentEvent: PgsEvent | null = null;

  constructor(options: PgsRendererOptions) {
    this.videoElement = options.videoElement;
    this.containerElement = options.containerElement || this.videoElement.parentElement || document.body;
    this.workerUrl = options.workerUrl || '/pgs-renderer.worker.js';
    this.debug = options.debug ?? false;

    this.setupCanvas();
    this.setupWorker();
    this.setupResizeObserver();
    this.startRenderLoop();
  }

  public addEvent(event: PgsEvent) {
    if (this.worker) {
      this.worker.postMessage({
        type: 'addEvent',
        payload: event,
      });
    } else if (this.fallbackCtx) {
      this.addFallbackEvent(event);
    }
  }

  public addEvents(events: PgsEvent[]) {
    if (events.length === 0) return;

    if (this.worker) {
      this.worker.postMessage({
        type: 'addEvents',
        payload: events,
      });
    } else if (this.fallbackCtx) {
      for (const ev of events) {
        this.addFallbackEvent(ev);
      }
    }
  }

  public resize() {
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

    if (this.worker) {
      this.worker.postMessage({
        type: 'resize',
        payload: {
          width: displayedWidth,
          height: displayedHeight,
        },
      });
    } else if (this.canvas && this.fallbackCtx) {
      this.canvas.width = displayedWidth;
      this.canvas.height = displayedHeight;
    }

    if (this.debug) {
      console.log('[PGS RENDERER] Resized canvas:', {
        width: displayedWidth,
        height: displayedHeight,
        left: offsetX,
        top: offsetY,
      });
    }
  }

  public setTimeOffset(offset: number) {
    this.timeOffset = offset;
    if (this.worker) {
      this.worker.postMessage({
        type: 'setTimeOffset',
        payload: { offset },
      });
    }
  }

  public clear() {
    if (this.worker) {
      this.worker.postMessage({ type: 'clear' });
    }
    if (this.fallbackCtx && this.canvas) {
      this.fallbackEvents.clear();
      this.fallbackImageCache.clear();
      this.fallbackCurrentEvent = null;
      this.fallbackCtx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    }
  }

  public stop() {
    // No-op for parity
  }

  public destroy() {
    this.isDestroyed = true;

    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }

    if (this.worker) {
      this.worker.terminate();
      this.worker = null;
    }

    if (this.resizeObserver) {
      this.resizeObserver.disconnect();
      this.resizeObserver = null;
    }

    if (this.canvas && this.canvas.parentElement) {
      this.canvas.parentElement.removeChild(this.canvas);
    }

    this.canvas = null;
    this.fallbackCtx = null;
    this.fallbackEvents.clear();
    this.fallbackImageCache.clear();
  }

  private setupCanvas() {
    this.canvas = document.createElement('canvas');
    this.canvas.className = 'vc-pgs-canvas';
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

  private setupWorker() {
    if (!this.canvas) return;

    const canTransfer = typeof this.canvas.transferControlToOffscreen === 'function';

    if (canTransfer) {
      try {
        const WorkerClass = typeof window !== 'undefined' ? window.Worker : (typeof Worker !== 'undefined' ? Worker : null);
        if (!WorkerClass) throw new Error('Worker not supported');
        this.worker = new WorkerClass(this.workerUrl);

        this.worker.onmessage = (e: MessageEvent) => {
          const { type, payload } = e.data;
          if (type === 'debug' && this.debug) {
            console.log('[PGS RENDERER DEBUG]', payload.message, payload.data);
          } else if (type === 'error') {
            console.error('[PGS RENDERER ERROR]', payload.message, payload.error);
          }
        };

        const offscreenCanvas = this.canvas.transferControlToOffscreen();
        this.worker.postMessage(
          {
            type: 'init',
            payload: {
              canvas: offscreenCanvas,
              debug: this.debug,
            },
          },
          [offscreenCanvas]
        );
        return;
      } catch (err) {
        console.warn('[PGS RENDERER] Failed to init OffscreenCanvas worker, using fallback:', err);
        if (this.worker) {
          this.worker.terminate();
          this.worker = null;
        }
      }
    }

    // Fallback if transferControlToOffscreen is unavailable or threw
    this.fallbackCtx = this.canvas.getContext('2d', { alpha: true });
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

        if (this.worker) {
          this.worker.postMessage({
            type: 'resize',
            payload: { width: displayedWidth, height: displayedHeight },
          });
        } else if (this.canvas && this.fallbackCtx) {
          this.canvas.width = displayedWidth;
          this.canvas.height = displayedHeight;
        }
      });
    });

    this.resizeObserver.observe(this.videoElement);
  }

  private startRenderLoop() {
    const render = () => {
      if (this.isDestroyed) return;

      if (!this.videoElement.paused || this.videoElement.seeking) {
        const currentTime = this.videoElement.currentTime;

        if (this.worker) {
          this.worker.postMessage({
            type: 'render',
            payload: {
              currentTime,
              canvasWidth: this.canvasWidth,
              canvasHeight: this.canvasHeight,
              isPlaying: !this.videoElement.paused,
            },
          });
        } else if (this.fallbackCtx) {
          this.renderFallbackFrame(currentTime + this.timeOffset);
        }
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

  private addFallbackEvent(event: PgsEvent) {
    const key = `${event.startTime}-${event.duration}-${event.imageData.substring(0, 50)}`;
    if (this.fallbackEvents.has(key)) return;

    this.fallbackEvents.set(key, event);
    if (!this.fallbackImageCache.has(event.imageData)) {
      const img = new Image();
      img.src = event.imageData;
      this.fallbackImageCache.set(event.imageData, img);
    }
  }

  private renderFallbackFrame(currentTime: number) {
    if (!this.fallbackCtx || !this.canvas) return;

    let eventToDisplay: PgsEvent | null = null;
    for (const ev of this.fallbackEvents.values()) {
      if (currentTime >= ev.startTime && currentTime <= ev.startTime + ev.duration) {
        eventToDisplay = ev;
        break;
      }
    }

    if (eventToDisplay !== this.fallbackCurrentEvent) {
      this.fallbackCtx.clearRect(0, 0, this.canvas.width, this.canvas.height);
      this.fallbackCurrentEvent = eventToDisplay;

      if (eventToDisplay) {
        const img = this.fallbackImageCache.get(eventToDisplay.imageData);
        if (img && img.complete) {
          const vw = eventToDisplay.canvasWidth || this.canvasWidth;
          const vh = eventToDisplay.canvasHeight || this.canvasHeight;
          const scaleX = this.canvasWidth / vw;
          const scaleY = this.canvasHeight / vh;
          const x = (eventToDisplay.x !== undefined ? eventToDisplay.x : (vw - eventToDisplay.width) / 2) * scaleX;
          const y = (eventToDisplay.y !== undefined ? eventToDisplay.y : vh - eventToDisplay.height - 20) * scaleY;
          const w = eventToDisplay.width * scaleX;
          const h = eventToDisplay.height * scaleY;
          this.fallbackCtx.drawImage(img, x, y, w, h);
        }
      }
    }
  }
}
