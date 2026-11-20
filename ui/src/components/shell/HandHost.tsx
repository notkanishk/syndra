"use client";

import { useEffect } from "react";

/**
 * Where the light under your hand is on first (A-25 moment 2, build order: "Home and People first").
 * The shell marks its `main` with `data-hand` on these routes; the full rollout is this one line.
 */
export const HAND_FIRST = /^\/($|users(\/|$))/;

/** Every wearer of the edge's light around `start`, innermost first (a button, then its card). */
function wearers(start: EventTarget | null): HTMLElement[] {
  const found: HTMLElement[] = [];
  if (!(start instanceof Element) || !start.closest("[data-hand]") || start.closest("[data-no-hand]")) return found;
  for (let el = start.closest<HTMLElement>(".rim"); el; el = el.parentElement?.closest<HTMLElement>(".rim") ?? null) {
    found.push(el);
  }
  return found;
}

/**
 * The one listener for the light under your hand (A-25 moment 2), for cards and buttons alike: one
 * passive `pointermove` on the window, never one per element. The event only remembers where the
 * pointer is; once a frame the wearers under it are measured, all of them first, then each is told
 * where the pointer is in its own box (`--px`, `--py`), which moves a gradient and lays nothing out.
 * A touch has no pointer to follow. A keyboard focus tells the card beside it where its row is
 * (`--fx`, `--fy`).
 */
export function HandHost() {
  useEffect(() => {
    let frame = 0;
    let last: PointerEvent | null = null;
    const draw = () => {
      frame = 0;
      if (!last) return;
      const { target, clientX, clientY } = last;
      const under = wearers(target);
      const boxes = under.map((el) => el.getBoundingClientRect());
      under.forEach((el, i) => {
        el.style.setProperty("--px", `${clientX - boxes[i].left}px`);
        el.style.setProperty("--py", `${clientY - boxes[i].top}px`);
      });
    };
    const move = (event: PointerEvent) => {
      if (event.pointerType === "touch") return;
      last = event;
      // not-motion: one write a frame of where the pointer is; the light is colour only, nothing moves.
      if (!frame) frame = requestAnimationFrame(draw);
    };
    const focus = (event: FocusEvent) => {
      const target = event.target;
      if (!(target instanceof Element) || !target.matches(":focus-visible")) return;
      const row = target.getBoundingClientRect();
      for (const card of wearers(target).filter((el) => el.classList.contains("rim-card"))) {
        const box = card.getBoundingClientRect();
        card.style.setProperty("--fx", `${row.left + row.width / 2 - box.left}px`);
        card.style.setProperty("--fy", `${row.top + row.height / 2 - box.top}px`);
      }
    };
    window.addEventListener("pointermove", move, { passive: true });
    window.addEventListener("focusin", focus);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("pointermove", move);
      window.removeEventListener("focusin", focus);
    };
  }, []);
  return null;
}
