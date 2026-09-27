/**
 * CinemaMode.ts - Cinema Mode Controller & Visual Isolation Manager
 * Part of Yourant Anime Tracking & Streaming Application
 *
 * Implements:
 * - Body class toggling (.cinema-mode-active)
 * - Coordination with ShaderBackground (u_dim: 1.0 -> 0.15)
 * - Hotkeys: 'C' key toggles, 'Escape' exits
 * - Form input guards (ignoring hotkeys in input/textarea/contentEditable)
 * - Custom DOM event dispatch: 'cinemamodechange'
 */

export interface ShaderBackgroundLike {
  setCinemaMode: (active: boolean) => void;
}

export type CinemaModeCallback = (active: boolean) => void;

export class CinemaModeManager {
  private active: boolean = false;
  private shaderBg: ShaderBackgroundLike | null = null;
  private callbacks: CinemaModeCallback[] = [];
  private keydownHandler: ((e: KeyboardEvent) => void) | null = null;

  constructor(shaderBg?: ShaderBackgroundLike) {
    if (shaderBg) {
      this.shaderBg = shaderBg;
    }
    this.initHotkeys();
  }

  public setShaderBackground(shaderBg: ShaderBackgroundLike) {
    this.shaderBg = shaderBg;
    if (this.active) {
      this.shaderBg.setCinemaMode(true);
    }
  }

  public isActive(): boolean {
    return this.active;
  }

  /**
   * Toggles cinema mode state.
   */
  public toggle(): boolean {
    return this.setActive(!this.active);
  }

  /**
   * Enables cinema mode visual isolation.
   */
  public enable(): boolean {
    return this.setActive(true);
  }

  /**
   * Disables cinema mode and restores standard layout.
   */
  public disable(): boolean {
    return this.setActive(false);
  }

  /**
   * Internal state setter with DOM update and notifications.
   */
  public setActive(active: boolean): boolean {
    if (this.active === active) return this.active;
    this.active = active;

    // 1. Toggle DOM body class
    document.body.classList.toggle('cinema-mode-active', this.active);

    // 2. Notify Shader Background for smooth u_dim transition (1.0 <-> 0.15)
    if (this.shaderBg) {
      try {
        this.shaderBg.setCinemaMode(this.active);
      } catch (err) {
        console.warn('Failed to update ShaderBackground cinema mode:', err);
      }
    }

    // 3. Dispatch native DOM CustomEvent for decoupled listeners
    document.dispatchEvent(new CustomEvent('cinemamodechange', {
      detail: { active: this.active },
      bubbles: true,
    }));

    // 4. Notify registered callbacks
    for (const cb of this.callbacks) {
      try {
        cb(this.active);
      } catch (err) {
        console.error('Error in CinemaMode callback:', err);
      }
    }

    return this.active;
  }

  /**
   * Sets up hotkey event listeners on the window object.
   */
  private initHotkeys() {
    this.keydownHandler = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (!target) return;

      // Ignore hotkeys when user is typing into input fields, textareas, or chat
      const tagName = target.tagName;
      if (tagName === 'INPUT' || tagName === 'TEXTAREA' || target.isContentEditable) {
        return;
      }

      // 'C' or 'c': Toggle Cinema Mode
      if (e.key === 'c' || e.key === 'C') {
        e.preventDefault();
        this.toggle();
        return;
      }

      // 'Escape': Exit Cinema Mode if active
      if (e.key === 'Escape' && this.active) {
        e.preventDefault();
        this.disable();
        return;
      }
    };

    window.addEventListener('keydown', this.keydownHandler);
  }

  /**
   * Subscribe to cinema mode state changes.
   */
  public onToggle(callback: CinemaModeCallback): () => void {
    this.callbacks.push(callback);
    callback(this.active);
    return () => {
      this.callbacks = this.callbacks.filter(cb => cb !== callback);
    };
  }

  /**
   * Clean up event listeners.
   */
  public destroy() {
    if (this.keydownHandler) {
      window.removeEventListener('keydown', this.keydownHandler);
      this.keydownHandler = null;
    }
    if (this.active) {
      this.disable();
    }
    this.callbacks = [];
  }
}
