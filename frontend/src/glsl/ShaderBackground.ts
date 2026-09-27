/**
 * Embedded GLSL Shader Fallbacks
 * Ensures ShaderBackground functions out-of-the-box in unit tests, mock environments,
 * and bundlers without requiring specialized loader plugins.
 */
const DEFAULT_VERT_SHADER = `
attribute vec2 a_position;
varying vec2 v_uv;

void main() {
    v_uv = (a_position + 1.0) * 0.5;
    gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

const DEFAULT_FRAG_SHADER = `
precision highp float;

uniform vec2 u_resolution;
uniform float u_time;
uniform vec2 u_mouse;
uniform float u_dim;

varying vec2 v_uv;

float hash(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(
        mix(hash(i + vec2(0.0, 0.0)), hash(i + vec2(1.0, 0.0)), u.x),
        mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), u.x),
        u.y
    );
}

float fbm(vec2 p) {
    float v = 0.0;
    float a = 0.5;
    vec2 shift = vec2(100.0);
    mat2 rot = mat2(cos(0.5), sin(0.5), -sin(0.5), cos(0.5));
    for (int i = 0; i < 4; ++i) {
        v += a * noise(p);
        p = rot * p * 2.0 + shift;
        a *= 0.5;
    }
    return v;
}

void main() {
    vec2 res = max(u_resolution, vec2(1.0, 1.0));
    vec2 st = (gl_FragCoord.xy - 0.5 * res) / res.y;

    vec2 clampedMouse = clamp(u_mouse, vec2(0.0), res);
    vec2 mouseOffset = (clampedMouse / res - 0.5) * 0.35;
    st += mouseOffset;

    float t = max(u_time, 0.0) * 0.12;

    vec2 q = vec2(fbm(st + vec2(0.0, t)), fbm(st + vec2(5.2, 1.3)));
    vec2 r = vec2(fbm(st + 4.0 * q + vec2(1.7 - t * 0.5, 9.2)), fbm(st + 4.0 * q + vec2(8.3, 2.8 + t * 0.3)));
    float f = fbm(st + 3.5 * r);

    vec3 c_deep_void    = vec3(0.027, 0.039, 0.075);
    vec3 c_steel_blue   = vec3(0.059, 0.090, 0.165);
    vec3 c_cyan_neon    = vec3(0.220, 0.741, 0.973);
    vec3 c_blood_deep   = vec3(0.298, 0.020, 0.098);
    vec3 c_blood_bright = vec3(0.882, 0.114, 0.282);
    vec3 c_ruby_flare   = vec3(0.984, 0.443, 0.522);

    vec3 color = mix(c_deep_void, c_steel_blue, clamp(length(q), 0.0, 1.0));
    color = mix(color, c_cyan_neon, clamp(length(r.x), 0.0, 1.0) * 0.65);

    float ribbon = smoothstep(0.3, 0.7, f * f * 2.5);
    color = mix(color, c_blood_bright, ribbon * 0.85);

    float flare = pow(clamp(f * r.y * 1.8, 0.0, 1.0), 3.0);
    color += c_ruby_flare * flare * 0.9;

    vec2 ember_uv = st * 8.0;
    ember_uv.y -= t * 3.0;
    float ember = smoothstep(0.985, 1.0, hash(floor(ember_uv)));
    color += c_ruby_flare * ember * 0.6;

    float dim = clamp(u_dim, 0.0, 1.0);
    color *= dim;

    gl_FragColor = vec4(color, 1.0);
}
`;

export interface ShaderBackgroundOptions {
  vertSource?: string;
  fragSource?: string;
  maxDpr?: number;
}

export class ShaderBackground {
  private canvas: HTMLCanvasElement;
  private gl: WebGLRenderingContext | null = null;
  private program: WebGLProgram | null = null;
  private vertexBuffer: WebGLBuffer | null = null;

  // Uniform locations
  private uResolutionLoc: WebGLUniformLocation | null = null;
  private uTimeLoc: WebGLUniformLocation | null = null;
  private uMouseLoc: WebGLUniformLocation | null = null;
  private uDimLoc: WebGLUniformLocation | null = null;

  // Animation & timing
  private animId: number = 0;
  private startTime: number = performance.now();
  private isDestroyed: boolean = false;
  private isContextLost: boolean = false;
  private maxDpr: number;

  // Mouse parallax tracking
  private mouseX: number = 0;
  private mouseY: number = 0;
  private targetMouseX: number = 0;
  private targetMouseY: number = 0;

  // Cinema Mode dimming factor (1.0 = normal, 0.15 = cinema)
  private dimFactor: number = 1.0;
  private targetDimFactor: number = 1.0;

  // Shader sources
  private vertSource: string;
  private fragSource: string;

  constructor(canvas: HTMLCanvasElement, options?: ShaderBackgroundOptions) {
    this.canvas = canvas;
    this.maxDpr = options?.maxDpr ?? 1.5;
    this.vertSource = options?.vertSource ?? DEFAULT_VERT_SHADER;
    this.fragSource = options?.fragSource ?? DEFAULT_FRAG_SHADER;

    this.initContext();
    if (this.gl) {
      this.initProgram();
      this.initBuffers();
      this.setupEventListeners();
      this.resize();
      this.startLoop();
    }
  }

  /**
   * Initializes WebGL 1.0 context with low-power preference
   */
  private initContext(): void {
    const glOptions: WebGLContextAttributes = {
      alpha: false,
      antialias: false,
      depth: false,
      stencil: false,
      preserveDrawingBuffer: false,
      powerPreference: 'low-power'
    };

    this.gl = (this.canvas.getContext('webgl', glOptions) ||
      this.canvas.getContext('experimental-webgl', glOptions)) as WebGLRenderingContext | null;

    if (!this.gl) {
      console.warn('[ShaderBackground] WebGL not supported on this device. Fallback to CSS backdrop.');
    }
  }

  /**
   * Compiles vertex and fragment shaders and links program
   */
  private initProgram(): void {
    if (!this.gl) return;
    const gl = this.gl;

    const vertShader = this.compileShader(gl.VERTEX_SHADER, this.vertSource);
    const fragShader = this.compileShader(gl.FRAGMENT_SHADER, this.fragSource);

    if (!vertShader || !fragShader) {
      console.error('[ShaderBackground] Failed to compile shaders.');
      return;
    }

    const program = gl.createProgram();
    if (!program) return;

    gl.attachShader(program, vertShader);
    gl.attachShader(program, fragShader);
    gl.linkProgram(program);

    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      console.error('[ShaderBackground] Program link error:', gl.getProgramInfoLog(program));
      gl.deleteProgram(program);
      return;
    }

    this.program = program;
    gl.useProgram(this.program);

    // Cache uniform locations
    this.uResolutionLoc = gl.getUniformLocation(this.program, 'u_resolution');
    this.uTimeLoc = gl.getUniformLocation(this.program, 'u_time');
    this.uMouseLoc = gl.getUniformLocation(this.program, 'u_mouse');
    this.uDimLoc = gl.getUniformLocation(this.program, 'u_dim');
  }

  private compileShader(type: number, source: string): WebGLShader | null {
    if (!this.gl) return null;
    const gl = this.gl;

    const shader = gl.createShader(type);
    if (!shader) return null;

    gl.shaderSource(shader, source.trim());
    gl.compileShader(shader);

    if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
      console.error(
        `[ShaderBackground] ${type === gl.VERTEX_SHADER ? 'Vertex' : 'Fragment'} compile error:`,
        gl.getShaderInfoLog(shader)
      );
      gl.deleteShader(shader);
      return null;
    }

    return shader;
  }

  /**
   * Sets up full-screen quad VBO
   */
  private initBuffers(): void {
    if (!this.gl || !this.program) return;
    const gl = this.gl;

    // Triangle strip quad covering [-1, 1] NDC
    const quadVertices = new Float32Array([
      -1.0, -1.0,
       1.0, -1.0,
      -1.0,  1.0,
       1.0,  1.0,
    ]);

    this.vertexBuffer = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, this.vertexBuffer);
    gl.bufferData(gl.ARRAY_BUFFER, quadVertices, gl.STATIC_DRAW);

    const aPosLoc = gl.getAttribLocation(this.program, 'a_position');
    if (aPosLoc !== -1) {
      gl.enableVertexAttribArray(aPosLoc);
      gl.vertexAttribPointer(aPosLoc, 2, gl.FLOAT, false, 0, 0);
    }
  }

  /**
   * Resizes canvas buffer clamped to max DPR (default 1.5)
   */
  public resize = (): void => {
    if (!this.canvas || !this.gl) return;

    const dpr = Math.min(window.devicePixelRatio || 1, this.maxDpr);
    const displayWidth = window.innerWidth || document.documentElement.clientWidth || 1920;
    const displayHeight = window.innerHeight || document.documentElement.clientHeight || 1080;

    const bufferWidth = Math.max(1, Math.floor(displayWidth * dpr));
    const bufferHeight = Math.max(1, Math.floor(displayHeight * dpr));

    if (this.canvas.width !== bufferWidth || this.canvas.height !== bufferHeight) {
      this.canvas.width = bufferWidth;
      this.canvas.height = bufferHeight;
    }

    this.gl.viewport(0, 0, this.canvas.width, this.canvas.height);
  };

  /**
   * Updates Cinema Mode state (dimming GLSL background to 0.15)
   * @param active boolean - true for cinema mode, false for normal mode
   */
  public setCinemaMode(active: boolean): void {
    this.targetDimFactor = active ? 0.15 : 1.0;
  }

  /**
   * Gets current dim factor for verification
   */
  public getDimFactor(): number {
    return this.dimFactor;
  }

  /**
   * Core requestAnimationFrame render loop
   */
  private startLoop(): void {
    if (this.animId) cancelAnimationFrame(this.animId);

    const render = () => {
      if (this.isDestroyed) return;

      // Smooth dimming transition via exponential lerp
      this.dimFactor += (this.targetDimFactor - this.dimFactor) * 0.08;
      // Clamp dimFactor to boundary [0.0, 1.0]
      this.dimFactor = Math.max(0.0, Math.min(1.0, this.dimFactor));

      // Smooth mouse parallax lerp
      this.mouseX += (this.targetMouseX - this.mouseX) * 0.05;
      this.mouseY += (this.targetMouseY - this.mouseY) * 0.05;

      if (!document.hidden && !this.isContextLost && this.gl && this.program) {
        const gl = this.gl;
        const elapsedSec = Math.max(0.0, (performance.now() - this.startTime) / 1000.0);

        gl.viewport(0, 0, this.canvas.width, this.canvas.height);

        if (this.uResolutionLoc) {
          gl.uniform2f(this.uResolutionLoc, this.canvas.width, this.canvas.height);
        }
        if (this.uTimeLoc) {
          gl.uniform1f(this.uTimeLoc, elapsedSec);
        }
        if (this.uMouseLoc) {
          gl.uniform2f(this.uMouseLoc, this.mouseX, this.mouseY);
        }
        if (this.uDimLoc) {
          gl.uniform1f(this.uDimLoc, this.dimFactor);
        }

        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      }

      this.animId = requestAnimationFrame(render);
    };

    this.animId = requestAnimationFrame(render);
  }

  /**
   * Event listeners: resize, mouse parallax, tab visibility, context loss
   */
  private setupEventListeners(): void {
    window.addEventListener('resize', this.resize);
    window.addEventListener('mousemove', this.handleMouseMove);
    document.addEventListener('visibilitychange', this.handleVisibilityChange);

    this.canvas.addEventListener('webglcontextlost', this.handleContextLost);
    this.canvas.addEventListener('webglcontextrestored', this.handleContextRestored);
  }

  private handleMouseMove = (e: MouseEvent): void => {
    const dpr = Math.min(window.devicePixelRatio || 1, this.maxDpr);
    // Clamp mouse coordinates to viewport boundary
    this.targetMouseX = Math.max(0, Math.min(window.innerWidth * dpr, e.clientX * dpr));
    this.targetMouseY = Math.max(0, Math.min(window.innerHeight * dpr, e.clientY * dpr));
  };

  private handleVisibilityChange = (): void => {
    if (document.hidden) {
      if (this.animId) {
        cancelAnimationFrame(this.animId);
        this.animId = 0;
      }
    } else {
      if (!this.animId && !this.isDestroyed) {
        this.startLoop();
      }
    }
  };

  private handleContextLost = (e: Event): void => {
    e.preventDefault();
    this.isContextLost = true;
    if (this.animId) {
      cancelAnimationFrame(this.animId);
      this.animId = 0;
    }
    console.warn('[ShaderBackground] WebGL context lost.');
  };

  private handleContextRestored = (): void => {
    console.log('[ShaderBackground] WebGL context restored. Re-initializing...');
    this.isContextLost = false;
    this.initContext();
    if (this.gl) {
      this.initProgram();
      this.initBuffers();
      this.resize();
      this.startLoop();
    }
  };

  /**
   * Destroys the manager, freeing WebGL resources and event listeners
   */
  public destroy(): void {
    this.isDestroyed = true;
    if (this.animId) {
      cancelAnimationFrame(this.animId);
      this.animId = 0;
    }

    window.removeEventListener('resize', this.resize);
    window.removeEventListener('mousemove', this.handleMouseMove);
    document.removeEventListener('visibilitychange', this.handleVisibilityChange);

    this.canvas.removeEventListener('webglcontextlost', this.handleContextLost);
    this.canvas.removeEventListener('webglcontextrestored', this.handleContextRestored);

    if (this.gl) {
      if (this.vertexBuffer) {
        this.gl.deleteBuffer(this.vertexBuffer);
        this.vertexBuffer = null;
      }
      if (this.program) {
        this.gl.deleteProgram(this.program);
        this.program = null;
      }
    }
  }
}
