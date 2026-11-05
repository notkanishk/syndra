"use client";

import { useEffect, useLayoutEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";

import { returnedToThisPage } from "@/lib/motion";

/**
 * Two kinds of memory, kept apart on purpose (A-19 P5 and P8).
 *
 * Place is this session only: where you stood on a page, handed back when you
 * return to it by Back and never otherwise. It lives in sessionStorage, keyed
 * by the route (path and query) and the name of the thing remembered. Every
 * mount writes its value, so a fresh visit overwrites whatever an earlier
 * visit left and nothing crosses visits.
 *
 * Views are a preference: a list's sort and window pill, kept in localStorage
 * under `syndra:view:<list>` so they last across visits. Search text and
 * filter chips never come here. The pill or arrow that holds the value is what
 * makes it visible.
 *
 * Every read and write is guarded: a blocked or full store is a page that
 * simply forgets.
 */
const PLACE = "syndra:place:";
const VIEW = "syndra:view:";

const here = () => window.location.pathname + window.location.search;

function store(kind: "session" | "local"): Storage | null {
  try {
    return kind === "session" ? window.sessionStorage : window.localStorage;
  } catch {
    return null;
  }
}

function read(kind: "session" | "local", key: string): unknown {
  try {
    const raw = store(kind)?.getItem(key);
    return raw == null ? undefined : JSON.parse(raw);
  } catch {
    return undefined;
  }
}

function write(kind: "session" | "local", key: string, value: unknown) {
  try {
    store(kind)?.setItem(key, JSON.stringify(value));
  } catch {
    // Forgetting is the whole failure mode.
  }
}

/** What this route kept under `name`, only when the reader came back to it. */
export function recallPlace(name: string): unknown {
  if (typeof window === "undefined" || !returnedToThisPage()) return undefined;
  return read("session", `${PLACE}${here()}:${name}`);
}

export function keepPlace(name: string, value: unknown) {
  if (typeof window !== "undefined") write("session", `${PLACE}${here()}:${name}`, value);
}

/** `useState` that a Back returns to its last value. `ok` rejects a stored value of the wrong shape. */
export function usePlace<T>(
  name: string,
  initial: T,
  ok: (value: unknown) => value is T = (value): value is T => typeof value === typeof initial,
): [T, Dispatch<SetStateAction<T>>] {
  const [value, set] = useState<T>(() => {
    const kept = recallPlace(name);
    return ok(kept) ? kept : initial;
  });
  useEffect(() => keepPlace(name, value), [name, value]);
  return [value, set];
}

/** A list's remembered view, or undefined. */
export function recallView<T>(list: string, name: string, ok: (value: unknown) => value is T): T | undefined {
  if (typeof window === "undefined") return undefined;
  const all = read("local", `${VIEW}${list}`) as Record<string, unknown> | undefined;
  const kept = all?.[name];
  return ok(kept) ? kept : undefined;
}

export function keepView(list: string, name: string, value: unknown) {
  if (typeof window === "undefined") return;
  const all = (read("local", `${VIEW}${list}`) as Record<string, unknown> | undefined) ?? {};
  write("local", `${VIEW}${list}`, { ...all, [name]: value });
}

/**
 * A pill that remembers: where the reader left it last time, across visits.
 * A Back returns the same value, because a change is written the moment it is made.
 */
export function usePill<T extends string>(list: string, initial: T, values: readonly T[]) {
  const isValue = (value: unknown): value is T => values.includes(value as T);
  const [value, set] = useState<T>(() => recallView(list, "pill", isValue) ?? initial);
  return [
    value,
    (next: T) => {
      set(next);
      keepView(list, "pill", next);
    },
  ] as const;
}

/**
 * Scroll for the app's one scroller: saved as it moves, handed back after a Back
 * once the page is tall enough to hold it. Read at the moment the route
 * changes, before the reset to the top can overwrite it. A touch, wheel or key
 * from the reader ends the restoring: it never fights a hand.
 */
export function usePlaceScroll(route: string) {
  const lastPath = useRef<string | null>(null);
  useLayoutEffect(() => {
    const box = document.getElementById("app-scroll");
    if (!box) return;
    const key = `${PLACE}${here()}:scroll`;
    const kept = recallPlace("scroll");
    const target = typeof kept === "number" && kept > 0 ? kept : 0;
    // A fresh visit starts at the top and says so, so an older visit's place is gone.
    // A query-only change is the same page: it keeps where the reader is (A-20 P19).
    const path = route.split("?")[0];
    const samePage = lastPath.current === path;
    lastPath.current = path;
    write("session", key, target === 0 && samePage ? box.scrollTop : target);

    let live = true;
    let frame = 0;
    let tries = 0;
    const stop = () => {
      live = false;
      cancelAnimationFrame(frame);
    };
    const restore = () => {
      if (!live) return;
      // About a second: a slow read gives up rather than waiting for ever.
      if (box.scrollHeight - box.clientHeight >= target || tries++ > 60) {
        box.scrollTop = target;
        return;
      }
      frame = requestAnimationFrame(restore);
    };
    if (target > 0) restore();

    let saving = 0;
    const save = () => {
      cancelAnimationFrame(saving);
      saving = requestAnimationFrame(() => write("session", key, box.scrollTop));
    };
    const ends = ["wheel", "touchstart", "keydown"];
    box.addEventListener("scroll", save, { passive: true });
    for (const type of ends) box.addEventListener(type, stop, { passive: true });
    return () => {
      stop();
      cancelAnimationFrame(saving);
      box.removeEventListener("scroll", save);
      for (const type of ends) box.removeEventListener(type, stop);
    };
    // Re-armed by path and query: a query-only change is a place of its own, so a Back to it returns its scroll (A-20 P19).
  }, [route]);
}
