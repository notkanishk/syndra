// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChimeToggle } from "@/components/settings/ChimeToggle";
import { isChimeEnabled, setChimeEnabled } from "@/lib/driftChime";
import { setMediaQuery } from "@/test-utils/media";

const REDUCED = "(prefers-reduced-motion: reduce)";

const SILENT = /Silent in this browser/;
const BLOCKED = /Can't play yet/;
const PLAYS = /Plays here/;

/**
 * A WebAudio stub whose `state` the test chooses, counting what plays. The
 * chime keeps one context for the page, so this is one object too, re-armed
 * per test rather than replaced.
 */
const start = vi.fn();
const resume = vi.fn();
const ctx = {
  state: "running" as string,
  currentTime: 0,
  destination: {},
  resume,
  addEventListener: () => {},
  removeEventListener: () => {},
  createOscillator: () => ({
    type: "",
    frequency: { value: 0 },
    connect: (node: unknown) => node,
    start,
    stop: () => {},
  }),
  createGain: () => ({
    gain: { setValueAtTime: () => {}, exponentialRampToValueAtTime: () => {} },
    connect: (node: unknown) => node,
  }),
};
(window as unknown as { AudioContext: unknown }).AudioContext = vi.fn(function () {
  return ctx;
});

function stubAudio(state: "suspended" | "running") {
  ctx.state = state;
  start.mockClear();
  resume.mockClear();
  return { start, resume };
}

/**
 * Off · On, and one line under it saying whether On will be heard. A toggle
 * that claims to be on while nothing can be heard is a lie the page is in a
 * position to catch, so it catches it.
 */
describe("the chime toggle", () => {
  beforeEach(() => {
    setMediaQuery(REDUCED, false);
    stubAudio("running");
  });

  it("is Off · On and keeps its stored preference", async () => {
    setChimeEnabled(false);
    render(<ChimeToggle />);

    const on = await screen.findByRole("radio", { name: "On" });
    expect(screen.getByRole("radio", { name: "Off" }).getAttribute("aria-checked")).toBe("true");
    fireEvent.click(on);
    expect(isChimeEnabled()).toBe(true);
    expect(localStorage.getItem("syndra-drift-chime")).toBe("on");
  });

  it("says nothing while off", async () => {
    setChimeEnabled(false);
    setMediaQuery(REDUCED, true);
    render(<ChimeToggle />);

    await screen.findByRole("radio", { name: "On" });
    expect(screen.queryByText(SILENT)).toBeNull();
    expect(screen.queryByText(PLAYS)).toBeNull();
    expect(screen.queryByRole("button", { name: "Play it once" })).toBeNull();
  });

  it("admits it is silent under reduced motion, with no button", async () => {
    setChimeEnabled(true);
    setMediaQuery(REDUCED, true);
    render(<ChimeToggle />);

    expect(await screen.findByText(SILENT)).toBeTruthy();
    expect(screen.getByText(/Nothing will play while that is set/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Play it once" })).toBeNull();
  });

  it("says it cannot play yet while the browser holds the sound back", async () => {
    setChimeEnabled(true);
    stubAudio("suspended");
    render(<ChimeToggle />);

    expect(await screen.findByText(BLOCKED)).toBeTruthy();
    expect(screen.getByRole("button", { name: "Play it once" })).toBeTruthy();
  });

  it("says it plays here otherwise, and plays the tone on request", async () => {
    setChimeEnabled(true);
    const audio = stubAudio("running");
    render(<ChimeToggle />);

    expect(await screen.findByText(PLAYS)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Play it once" }));
    await waitFor(() => expect(audio.start).toHaveBeenCalledTimes(1));
    expect(audio.resume).toHaveBeenCalled();
    expect(screen.queryByText("That was this.")).toBeNull();
  });
});
