"use client";

import { useEffect, useState } from "react";

/**
 * Whether the browser has a network at all.
 *
 * Distinct from `degraded`, and the distinction is the whole reason this
 * exists. Degraded means the API answered and answered badly — the directory
 * fell back, so every name on screen is fiction. Offline means nothing
 * answered: what is on screen is whatever already arrived, and it was true
 * when it arrived.
 *
 * They must never share a banner. An operator who reads "Syndra can't reach
 * the provider" while standing in a workshop with no wifi goes looking for a
 * broken server, and the thing that is broken is the room they are in.
 *
 * Optimistic on the first render — `navigator.onLine` is unavailable during
 * SSR, and a page that flashes "no network" before hydration would be lying
 * more often than it was right.
 */
export function useOnline(): boolean {
  const [online, setOnline] = useState(true);

  useEffect(() => {
    if (typeof navigator === "undefined") return;
    setOnline(navigator.onLine);

    const goOnline = () => setOnline(true);
    const goOffline = () => setOnline(false);
    window.addEventListener("online", goOnline);
    window.addEventListener("offline", goOffline);
    return () => {
      window.removeEventListener("online", goOnline);
      window.removeEventListener("offline", goOffline);
    };
  }, []);

  return online;
}
