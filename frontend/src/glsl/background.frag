precision highp float;

uniform vec2 u_resolution;
uniform float u_time;
uniform vec2 u_mouse;
uniform float u_dim;

varying vec2 v_uv;

// Pseudo-random 2D hash generator
float hash(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

// 2D Smooth Bilinear Noise
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

// Fractal Brownian Motion (FBM) with 4 octaves and octave decay
float fbm(vec2 p) {
    float v = 0.0;
    float a = 0.5;
    vec2 shift = vec2(100.0);
    // 2D Rotation matrix to break axial grid artifacts
    mat2 rot = mat2(cos(0.5), sin(0.5), -sin(0.5), cos(0.5));
    for (int i = 0; i < 4; ++i) {
        v += a * noise(p);
        p = rot * p * 2.0 + shift;
        a *= 0.5;
    }
    return v;
}

void main() {
    // Viewport resolution with division-by-zero protection
    vec2 res = max(u_resolution, vec2(1.0, 1.0));
    vec2 st = (gl_FragCoord.xy - 0.5 * res) / res.y;

    // Clamped mouse parallax with subtle dampening
    vec2 clampedMouse = clamp(u_mouse, vec2(0.0), res);
    vec2 mouseOffset = (clampedMouse / res - 0.5) * 0.35;
    st += mouseOffset;

    // Clamped elapsed time
    float t = max(u_time, 0.0) * 0.12;

    // Multi-pass Domain Warping for fluid organic energy ribbons
    vec2 q = vec2(
        fbm(st + vec2(0.0, t)),
        fbm(st + vec2(5.2, 1.3))
    );

    vec2 r = vec2(
        fbm(st + 4.0 * q + vec2(1.7 - t * 0.5, 9.2)),
        fbm(st + 4.0 * q + vec2(8.3, 2.8 + t * 0.3))
    );

    float f = fbm(st + 3.5 * r);

    // Color Palette Tokens
    vec3 c_deep_void    = vec3(0.027, 0.039, 0.075); // Deep Metallic Void #070a13
    vec3 c_steel_blue   = vec3(0.059, 0.090, 0.165); // Steel Slate Blue #0f172a
    vec3 c_cyan_neon    = vec3(0.220, 0.741, 0.973); // Metallic Cyan #38bdf8
    vec3 c_blood_deep   = vec3(0.298, 0.020, 0.098); // Deep Crimson #4c0519
    vec3 c_blood_bright = vec3(0.882, 0.114, 0.282); // Blood Red #e11d48
    vec3 c_ruby_flare   = vec3(0.984, 0.443, 0.522); // Ruby Highlight #fb7185

    // Layer 1: Base metallic obsidian-to-steel depth
    vec3 color = mix(c_deep_void, c_steel_blue, clamp(length(q), 0.0, 1.0));

    // Layer 2: Metallic cyan ribbon currents
    color = mix(color, c_cyan_neon, clamp(length(r.x), 0.0, 1.0) * 0.65);

    // Layer 3: Blood red organic wave ribbons
    float ribbon = smoothstep(0.3, 0.7, f * f * 2.5);
    color = mix(color, c_blood_bright, ribbon * 0.85);

    // Layer 4: Intense ruby flares at wave crests
    float flare = pow(clamp(f * r.y * 1.8, 0.0, 1.0), 3.0);
    color += c_ruby_flare * flare * 0.9;

    // Layer 5: Ambient floating embers
    vec2 ember_uv = st * 8.0;
    ember_uv.y -= t * 3.0;
    float ember = smoothstep(0.985, 1.0, hash(floor(ember_uv)));
    color += c_ruby_flare * ember * 0.6;

    // Cinema Mode visual isolation dimming (clamped 0.0 - 1.0)
    float dim = clamp(u_dim, 0.0, 1.0);
    color *= dim;

    gl_FragColor = vec4(color, 1.0);
}
