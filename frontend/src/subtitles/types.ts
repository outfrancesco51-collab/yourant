/**
 * types.ts - Subtitle subsystem type definitions
 * Parity with Seanime ASS/JASSUB & Blu-ray PGS subtitle architecture.
 */

export interface PgsEvent {
  startTime: number;    // In seconds
  duration: number;     // In seconds
  imageData: string;    // Base64 encoded PNG or image data URL
  width: number;        // Intrinsic width
  height: number;       // Intrinsic height
  x?: number;           // Target X position on canvas (default: center)
  y?: number;           // Target Y position on canvas (default: bottom - 20)
  canvasWidth?: number; // Video master canvas width
  canvasHeight?: number;// Video master canvas height
  cropX?: number;
  cropY?: number;
  cropWidth?: number;
  cropHeight?: number;
}

export type SubtitleTrackType = 'ass' | 'ssa' | 'pgs' | 'vtt' | 'srt';

export interface SubtitleTrack {
  id: string | number;
  label: string;
  language?: string;
  type: SubtitleTrackType;
  src?: string;          // External subtitle file URL
  content?: string;      // In-memory script (ASS/SSA or VTT/SRT text)
  events?: PgsEvent[];   // Bitmap PGS events
  default?: boolean;
  forced?: boolean;
}

export interface SubtitleStyle {
  Name?: string;
  FontName?: string;
  FontSize?: number;
  PrimaryColour?: number;
  SecondaryColour?: number;
  OutlineColour?: number;
  BackColour?: number;
  Bold?: number;
  Italic?: number;
  Underline?: number;
  StrikeOut?: number;
  ScaleX?: number;
  ScaleY?: number;
  Spacing?: number;
  Angle?: number;
  BorderStyle?: number;
  Outline?: number;
  Shadow?: number;
  Alignment?: number;
  MarginL?: number;
  MarginR?: number;
  MarginV?: number;
  Encoding?: number;
}

export interface VideoContentSize {
  displayedWidth: number;
  displayedHeight: number;
  offsetX: number;
  offsetY: number;
}
