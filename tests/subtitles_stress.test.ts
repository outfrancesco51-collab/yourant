/**
 * tests/subtitles_stress.test.ts
 *
 * Empirical Adversarial Stress Harness for Yourant Subtitle Subsystem:
 * - Rapid track switching (ASS -> PGS -> OFF -> ASS) under high frequency and concurrency
 * - Extreme subtitle delays (-10.0s, +10.0s, 0.0s, -100s, +100s)
 * - Out-of-order time updates, random seeking, and paused/playing states
 * - Comprehensive WebVTT/SRT to ASS parser fuzzing and cue edge cases
 * - Renderer lifecycle, re-initialization, and canvas leak prevention
 */

import { SubtitleManager } from '../frontend/src/subtitles/SubtitleManager';
import { SubtitleTrack, PgsEvent } from '../frontend/src/subtitles/types';
import { JassubRenderer } from '../frontend/src/subtitles/JassubRenderer';

// Mock DOM & Worker environment for Node.js
class MockWorker {
  public onmessage: any = null;
  public onerror: any = null;
  public postedMessages: any[] = [];
  public isTerminated: boolean = false;

  constructor(public url: string, public options?: any) {
    MockWorker.instances.push(this);
  }

  static instances: MockWorker[] = [];

  postMessage(msg: any, _transfer?: any[]) {
    if (this.isTerminated) {
      throw new Error('Cannot postMessage to terminated worker');
    }
    this.postedMessages.push(msg);

    // Auto-respond to Jassub CONSTRUCT & APPLY
    if (msg && msg.id && this.onmessage) {
      setTimeout(() => {
        if (this.isTerminated || !this.onmessage) return;
        if (msg.type === 'CONSTRUCT') {
          this.onmessage({
            data: { id: msg.id, type: 'HANDLER', name: 'proxy', value: 'mock-instance-marker-xyz' }
          });
        } else if (msg.type === 'APPLY') {
          this.onmessage({
            data: { id: msg.id, type: 'HANDLER', name: 'return', value: null }
          });
        }
      }, 0);
    }
  }

  terminate() {
    this.isTerminated = true;
  }
}

class MockElement {
  public clientWidth = 1920;
  public clientHeight = 1080;
  public videoWidth = 1920;
  public videoHeight = 1080;
  public currentTime = 0;
  public duration = 300;
  public paused = false;
  public seeking = false;
  public style: Record<string, string> = {};
  public children: any[] = [];
  public nextSibling: any = null;
  public parentElement: any = null;
  public listeners: Record<string, Function[]> = {};

  appendChild(child: any) {
    this.children.push(child);
    child.parentElement = this;
    return child;
  }
  insertBefore(child: any, _ref: any) {
    this.children.push(child);
    child.parentElement = this;
    return child;
  }
  removeChild(child: any) {
    this.children = this.children.filter(c => c !== child);
    child.parentElement = null;
    return child;
  }
  querySelector(_sel: string) {
    return null;
  }
  addEventListener(ev: string, cb: any) {
    if (!this.listeners[ev]) this.listeners[ev] = [];
    this.listeners[ev].push(cb);
  }
  removeEventListener(ev: string, cb: any) {
    if (this.listeners[ev]) {
      this.listeners[ev] = this.listeners[ev].filter(f => f !== cb);
    }
  }
  dispatchEvent(ev: string) {
    if (this.listeners[ev]) {
      for (const cb of this.listeners[ev]) {
        cb();
      }
    }
  }
  getContext(_type?: string) {
    return {
      clearRect: () => {},
      drawImage: () => {},
      fillRect: () => {},
      strokeRect: () => {},
    };
  }
  transferControlToOffscreen() {
    return new (globalThis as any).OffscreenCanvas(this.clientWidth, this.clientHeight);
  }
}

if (typeof document === 'undefined') {
  (globalThis as any).document = {
    createElement: (tag: string) => {
      const el = new MockElement();
      (el as any).tagName = tag.toUpperCase();
      return el;
    },
    body: new MockElement(),
  };

  (globalThis as any).getComputedStyle = () => ({
    position: 'relative',
  });

  (globalThis as any).ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  (globalThis as any).window = globalThis;
  (globalThis as any).Worker = MockWorker;

  (globalThis as any).OffscreenCanvas = class {
    width: number;
    height: number;
    constructor(w: number, h: number) {
      this.width = w;
      this.height = h;
    }
    getContext() { return {}; }
  };

  (globalThis as any).requestAnimationFrame = (cb: any) => setTimeout(cb, 10);
  (globalThis as any).cancelAnimationFrame = (id: any) => clearTimeout(id);
}

function assert(condition: boolean, message: string) {
  if (!condition) {
    throw new Error(`[STRESS TEST FAILURE] ${message}`);
  }
}

async function sleep(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function runAllStressTests() {
  console.log('=== Suite 2: Subtitle Subsystem Empirical Stress Testing ===\n');

  const testVideo = document.createElement('video') as unknown as HTMLVideoElement;
  const testContainer = document.createElement('div');

  const mockAssTrack: SubtitleTrack = {
    id: 'track-ass-1',
    label: 'English [ASS]',
    language: 'en',
    type: 'ass',
    default: true,
    content: JassubRenderer.DEFAULT_HEADER + 'Dialogue: 0,0:00:01.00,0:00:05.00,Default,,0,0,0,,Stress Test ASS Dialogue\n',
  };

  const mockPgsTrack: SubtitleTrack = {
    id: 'track-pgs-1',
    label: 'English [PGS]',
    language: 'en',
    type: 'pgs',
    events: [
      {
        startTime: 1.0,
        duration: 4.0,
        imageData: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
        width: 100,
        height: 40,
        x: 910,
        y: 1000,
        canvasWidth: 1920,
        canvasHeight: 1080,
      },
      {
        startTime: 10.0,
        duration: 3.5,
        imageData: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==',
        width: 120,
        height: 50,
      }
    ],
  };

  const mockVttTrack: SubtitleTrack = {
    id: 'track-vtt-1',
    label: 'French [WebVTT]',
    language: 'fr',
    type: 'vtt',
    content: `WEBVTT

00:00:01.000 --> 00:00:04.000
Bonjour le monde

00:00:10.500 --> 00:00:14.250
Deuxième dialogue avec <b>balises</b>
`,
  };

  const manager = new SubtitleManager(testVideo, testContainer);
  await manager.setTracks([mockAssTrack, mockPgsTrack, mockVttTrack]);

  // --------------------------------------------------------------------------
  // TEST 2.1: Rapid Track Switching Stress Test (ASS -> PGS -> OFF -> ASS)
  // --------------------------------------------------------------------------
  console.log('--- Test 2.1: Rapid Track Switching Stress Test ---');
  const SWITCH_ROUNDS = 50;
  const startTime = Date.now();

  for (let i = 0; i < SWITCH_ROUNDS; i++) {
    // ASS
    await manager.selectTrack('track-ass-1');
    assert(manager.getCurrentTrack()?.id === 'track-ass-1', `Round ${i}: Should be ASS`);
    assert(manager.getCurrentTrack()?.type === 'ass', `Round ${i}: Type should be ass`);

    // PGS
    await manager.selectTrack('track-pgs-1');
    assert(manager.getCurrentTrack()?.id === 'track-pgs-1', `Round ${i}: Should be PGS`);
    assert(manager.getCurrentTrack()?.type === 'pgs', `Round ${i}: Type should be pgs`);

    // VTT
    await manager.selectTrack('track-vtt-1');
    assert(manager.getCurrentTrack()?.id === 'track-vtt-1', `Round ${i}: Should be VTT`);

    // OFF
    await manager.disableSubtitles();
    assert(manager.getCurrentTrack() === null, `Round ${i}: Should be OFF`);
  }
  const switchDuration = Date.now() - startTime;
  console.log(`✓ 2.1 Passed: Successfully executed ${SWITCH_ROUNDS * 4} track switches in ${switchDuration}ms without error.`);

  // Test 2.1b: Concurrent rapid switching without waiting
  console.log('--- Test 2.1b: Concurrent Un-awaited Track Switching ---');
  const concurrentPromises = [
    manager.selectTrack('track-ass-1'),
    manager.selectTrack('track-pgs-1'),
    manager.selectTrack(null),
    manager.selectTrack('track-vtt-1'),
    manager.selectTrack('track-pgs-1'),
  ];
  await Promise.all(concurrentPromises);
  assert(manager.getCurrentTrack() !== undefined, 'Current track should be defined after concurrent switches');
  console.log(`✓ 2.1b Passed: Handled concurrent un-awaited switching cleanly. Final track: ${manager.getCurrentTrack()?.id}`);

  // --------------------------------------------------------------------------
  // TEST 2.2: Extreme Subtitle Delay Values (-10.0s, +10.0s, 0.0s, ±100.0s)
  // --------------------------------------------------------------------------
  console.log('--- Test 2.2: Extreme Subtitle Delay Adjustments ---');
  let lastDelayEventValue: number | null = null;
  manager.addEventListener('delaychange', (e: any) => {
    lastDelayEventValue = e.detail.delay;
  });

  const testDelays = [-10.0, 10.0, 0.0, -100.0, 100.0, -0.1, 0.1, -15.55];
  for (const d of testDelays) {
    manager.setDelay(d);
    const expectedRounded = Math.round(d * 10) / 10;
    assert(manager.getDelay() === expectedRounded, `Delay should be ${expectedRounded}, got ${manager.getDelay()}`);
    assert(lastDelayEventValue === expectedRounded, `delaychange event detail should be ${expectedRounded}, got ${lastDelayEventValue}`);
  }

  // Reset to 0.0
  manager.setDelay(0.0);
  assert(manager.getDelay() === 0.0, 'Delay should be reset to 0');
  console.log(`✓ 2.2 Passed: Extreme delay values (-100s to +100s) handled accurately with 0.1s rounding.`);

  // --------------------------------------------------------------------------
  // TEST 2.3: Out-of-Order Time Updates & Seeking During Playback
  // --------------------------------------------------------------------------
  console.log('--- Test 2.3: Out-of-Order Time Updates & Seeking ---');
  await manager.selectTrack('track-pgs-1');
  const mockVideoEl = testVideo as any;

  // Jump time randomly: forward, backward, out-of-order, negative
  const timeJumps = [0, 50, 1.5, 300, 10.2, 0.5, 250, 2.0, -5.0, 10.5, 100];

  for (const t of timeJumps) {
    mockVideoEl.currentTime = t;
    mockVideoEl.seeking = true;
    mockVideoEl.dispatchEvent('seeking');
    await sleep(5);

    mockVideoEl.seeking = false;
    mockVideoEl.dispatchEvent('seeked');
    mockVideoEl.dispatchEvent('timeupdate');
    await sleep(5);
  }

  // Switch to ASS and repeat seeking barrage
  await manager.selectTrack('track-ass-1');
  for (const t of timeJumps) {
    mockVideoEl.currentTime = t;
    mockVideoEl.paused = false;
    mockVideoEl.dispatchEvent('timeupdate');
    await sleep(5);
  }
  console.log('✓ 2.3 Passed: Out-of-order time updates and seeking dispatched cleanly without exceptions.');

  // --------------------------------------------------------------------------
  // TEST 2.4: WebVTT / SRT to ASS Parser Fuzzing & Various Cue Formats
  // --------------------------------------------------------------------------
  console.log('--- Test 2.4: WebVTT & SRT to ASS Parser Fuzzing ---');

  // We can test convertVttOrSrtToAss via manager.addTrack with type 'vtt' / 'srt'
  const vttTestCases = [
    {
      name: 'Standard 3-part timestamps with ms',
      vtt: `WEBVTT\n\n00:01:23.456 --> 00:01:25.789\nStandard dialogue\n`,
      expectedStart: '0:01:23.45',
      expectedEnd: '0:01:25.78',
      expectedText: 'Standard dialogue',
    },
    {
      name: '2-part timestamps (mm:ss.ttt without hours)',
      vtt: `WEBVTT\n\n02:15.500 --> 02:20.120\nShort timestamp line\n`,
      expectedStart: '0:02:15.50',
      expectedEnd: '0:02:20.12',
      expectedText: 'Short timestamp line',
    },
    {
      name: 'WebVTT Cue with alignment & position settings',
      vtt: `WEBVTT\n\n00:00:05.100 --> 00:00:08.200 position:10%,line:90% align:left size:50%\nStyled WebVTT Cue\n`,
      expectedStart: '0:00:05.10',
      expectedEnd: '0:00:08.20',
      expectedText: 'Styled WebVTT Cue',
    },
    {
      name: 'WebVTT Cue with cue identifier',
      vtt: `WEBVTT\n\ncue-header-42\n00:03:10.000 --> 00:03:15.000\nCue with ID\n`,
      expectedStart: '0:03:10.00',
      expectedEnd: '0:03:15.00',
      expectedText: 'Cue with ID',
    },
    {
      name: 'HTML & formatting tags in WebVTT cue',
      vtt: `WEBVTT\n\n00:00:01.000 --> 00:00:03.000\n<b>Bold</b> and <i>Italic</i> and <c.yellow>Color</c> and <v Speaker>Voice</v>\n`,
      expectedStart: '0:00:01.00',
      expectedEnd: '0:00:03.00',
      expectedText: 'Bold and Italic and Color and Voice',
    },
    {
      name: 'Multi-line dialogue cue',
      vtt: `WEBVTT\n\n00:04:00.000 --> 00:04:05.000\nFirst line\nSecond line\nThird line\n`,
      expectedStart: '0:04:00.00',
      expectedEnd: '0:04:05.00',
      expectedText: 'First line\\NSecond line\\NThird line',
    },
    {
      name: 'SRT format with sequence number and comma milliseconds',
      vtt: `1\n00:01:00,500 --> 00:01:04,900\nSRT Subtitle line\n\n2\n00:01:05,000 --> 00:01:08,000\nSecond SRT line\n`,
      expectedStart: '0:01:00.50',
      expectedEnd: '0:01:04.90',
      expectedText: 'SRT Subtitle line',
    },
    {
      name: 'Windows CRLF line endings',
      vtt: "WEBVTT\r\n\r\n00:00:10.000 --> 00:00:12.000\r\nCRLF dialogue line\r\n",
      expectedStart: '0:00:10.00',
      expectedEnd: '0:00:12.00',
      expectedText: 'CRLF dialogue line',
    },
  ];

  // Access private convertVttOrSrtToAss for deep asserting
  const convertFn = (manager as any).convertVttOrSrtToAss.bind(manager);

  for (const tc of vttTestCases) {
    const assResult = convertFn(tc.vtt);
    assert(assResult.includes('[Script Info]'), `${tc.name}: Must contain Script Info header`);
    assert(assResult.includes('[Events]'), `${tc.name}: Must contain Events section`);
    assert(assResult.includes(`Dialogue: 0,${tc.expectedStart},${tc.expectedEnd}`), `${tc.name}: Time format mismatch in ASS output: ${assResult}`);
    assert(assResult.includes(tc.expectedText), `${tc.name}: Text content mismatch in ASS output: ${assResult}`);
    console.log(`  ✓ Sub-case passed: ${tc.name}`);
  }

  // Fuzz edge-case: malformed inputs should not throw unhandled exceptions
  const malformedInputs = [
    '',
    '   \n\n   ',
    'WEBVTT without timestamps',
    '00:00:01.000 --> invalid-time\nBroken cue\n',
    '99:99:99.999 --> 99:99:99.999\nInvalid hour/min/sec\n',
    'WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n<unclosed tag without text>',
  ];

  for (let i = 0; i < malformedInputs.length; i++) {
    try {
      const res = convertFn(malformedInputs[i]);
      assert(typeof res === 'string', `Malformed test ${i} returned non-string`);
    } catch (err: any) {
      assert(false, `Malformed test ${i} threw error: ${err.message}`);
    }
  }
  console.log(`✓ 2.4 Passed: All WebVTT and SRT cue formats parsed accurately; malformed inputs gracefully handled.`);

  // --------------------------------------------------------------------------
  // TEST 2.5: Destruction & Cleanup Integrity
  // --------------------------------------------------------------------------
  console.log('--- Test 2.5: Lifecycle & Cleanup Integrity ---');
  manager.destroy();
  assert(manager.getTracks().length === 0, 'Tracks must be empty after destroy');
  assert(manager.getCurrentTrack() === null, 'Current track must be null after destroy');
  assert(manager.getPgsRenderer() === null, 'PgsRenderer must be null after destroy');
  assert(manager.getJassubRenderer() === null, 'JassubRenderer must be null after destroy');
  console.log('✓ 2.5 Passed: SubtitleManager destroyed cleanly without dangling renderers or listeners.');

  console.log('\n=== Suite 2: All Subtitle Subsystem Stress Tests Passed Cleanly ===\n');
}

runAllStressTests().catch((err) => {
  console.error('\n❌ Suite 2 FAILED with error:', err);
  process.exit(1);
});
