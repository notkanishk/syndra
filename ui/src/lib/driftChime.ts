"use client";

const STORAGE_KEY = "syndra-drift-chime";
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
 * One AudioContext for the page, so its `state` can be read: a browser lets
 * sound start only after a click on the page, and until then the context sits
 * `suspended`. ChimeToggle says so rather than letting "On" promise a sound.
 * Null where WebAudio does not exist.
 */
export function getChimeContext(): AudioContext | null {
  if (typeof window === "undefined") return null;
  if (shared && shared.state !== "closed") return shared;
  const Ctx = window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
  shared = Ctx ? new Ctx() : null;
  return shared;
}
let shared: AudioContext | null = null;

/**
 * Plays a short synthesized beep (~120ms) to cue a new drift item, gated on
 * the chime toggle and `prefers-reduced-motion`. Caller is responsible for
 * only invoking this when the drift count has just increased; "Play it once"
 * calls it directly, which is the same path minus that check.
 *
 * ponytail: a WebAudio oscillator avoids shipping/inlining an audio asset for
 * one short beep — swap for a real chime file only if this reads as too
 * utilitarian.
 */
export function playDriftChime() {
  if (typeof window === "undefined") return;
  if (!isChimeEnabled()) return;
  if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;

  const ctx = getChimeContext();
  if (!ctx) return;
  void ctx.resume?.();
  const osc = ctx.createOscillator();
  const gain = ctx.createGain();
  osc.type = "sine";
  osc.frequency.value = 880;
  gain.gain.setValueAtTime(0.15, ctx.currentTime);
  gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.12);
  osc.connect(gain).connect(ctx.destination);
  osc.start();
  osc.stop(ctx.currentTime + 0.12);
}
