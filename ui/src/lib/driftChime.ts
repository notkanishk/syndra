"use client";

const STORAGE_KEY = "syndra-drift-chime";
const SEEN_KEY = "syndra-drift-chime-seen";
/** Fired on `window` the first time the chime actually plays, so ChimeToggle
 * can surface a one-time explanatory tooltip. ponytail: a CustomEvent is the
 * whole "pub/sub" needed for one listener — no context/store required. */
export const CHIME_FIRST_PLAY_EVENT = "syndra-drift-chime-first-play";

/** Whether the drift chime is enabled. Default on; mirrors theme.tsx's localStorage idiom. */
export function isChimeEnabled(): boolean {
  if (typeof window === "undefined") return true;
  return localStorage.getItem(STORAGE_KEY) !== "off";
}

export function setChimeEnabled(on: boolean) {
  if (typeof window !== "undefined") {
    localStorage.setItem(STORAGE_KEY, on ? "on" : "off");
  }
}

/**
 * Plays a short synthesized beep (~120ms) to cue a new drift item, gated on
 * the chime toggle and `prefers-reduced-motion`. Caller is responsible for
 * only invoking this when the drift count has just increased.
 *
 * ponytail: a WebAudio oscillator avoids shipping/inlining an audio asset for
 * one short beep — swap for a real chime file only if this reads as too
 * utilitarian.
 */
export function playDriftChime() {
  if (typeof window === "undefined") return;
  if (!isChimeEnabled()) return;
  if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;

  const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  if (!Ctx) return;
  const ctx = new Ctx();
  const osc = ctx.createOscillator();
  const gain = ctx.createGain();
  osc.type = "sine";
  osc.frequency.value = 880;
  gain.gain.setValueAtTime(0.15, ctx.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.12);
  osc.connect(gain).connect(ctx.destination);
  osc.start();
  osc.stop(ctx.currentTime + 0.12);
  osc.onended = () => ctx.close();

  if (localStorage.getItem(SEEN_KEY) !== "1") {
    localStorage.setItem(SEEN_KEY, "1");
    window.dispatchEvent(new CustomEvent(CHIME_FIRST_PLAY_EVENT));
  }
}
