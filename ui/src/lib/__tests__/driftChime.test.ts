// @vitest-environment jsdom
import { beforeEach, describe, expect, it, vi } from "vitest";

import { playDriftChime, setChimeEnabled } from "@/lib/driftChime";
import { setMediaQuery } from "@/test-utils/media";

/**
 * The chime is the product's only sound, and the only thing that silences it
 * besides the toggle is `prefers-reduced-motion`. Until jsdom had a matchMedia
 * stub, that guard could not be reached by a test at all: `window.matchMedia?.`
 * returned undefined, so every run took the "no preference" branch and the
 * reduced-motion path was never once executed.
 */

/** Minimal WebAudio surface — enough for the chime, and counted. */
function stubAudioContext() {
  const close = vi.fn();
  const start = vi.fn();
  const stop = vi.fn();
  const ctor = vi.fn(function AudioContextStub(this: Record<string, unknown>) {
    return {
      currentTime: 0,
      destination: {},
      close,
      createOscillator: () => ({
        type: "",
        frequency: { value: 0 },
        // WebAudio's connect returns the destination node so calls chain.
        connect: (node: unknown) => node,
        start,
        stop,
        onended: null,
      }),
      createGain: () => ({
        gain: { value: 0, setValueAtTime: () => {}, exponentialRampToValueAtTime: () => {} },
        // WebAudio's connect returns the destination node so calls chain.
        connect: (node: unknown) => node,
      }),
    };
  });
  (window as unknown as { AudioContext: unknown }).AudioContext = ctor;
  return ctor;
}

describe("the drift chime honours reduced motion", () => {
  beforeEach(() => {
    setChimeEnabled(true);
  });

  it("plays when nothing asks it not to", () => {
    const ctor = stubAudioContext();
    playDriftChime();
    expect(ctor).toHaveBeenCalledTimes(1);
  });

  it("stays silent under prefers-reduced-motion: reduce", () => {
    const ctor = stubAudioContext();
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    playDriftChime();
    expect(ctor, "a sound is movement for anyone who asked for less of it").not.toHaveBeenCalled();
  });

  it("stays silent when the operator turned it off, preference or not", () => {
    const ctor = stubAudioContext();
    setChimeEnabled(false);
    playDriftChime();
    expect(ctor).not.toHaveBeenCalled();
  });
});
