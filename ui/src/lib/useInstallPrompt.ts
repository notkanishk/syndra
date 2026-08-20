"use client";

import { useEffect, useState } from "react";

/**
 * The browser's install offer, held until somebody goes looking for it.
 *
 * Chrome fires `beforeinstallprompt` when it decides the page is
 * installable, and the event can be deferred and replayed later from a real
 * user gesture. Deferring is the whole point here: a banner on first load
 * interrupts somebody who came to read one thing, and gets dismissed by
 * reflex before it is read. It lives in the Account sheet instead, where a
 * member goes when they are thinking about this app rather than about the
 * machine they wanted to book.
 *
 * Returns null when there is nothing to offer — already installed, an
 * unsupported browser, or the offer already taken — and the caller renders
 * nothing rather than a control that explains why it cannot work.
 */
interface InstallEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

export function useInstallPrompt(): { install: () => Promise<void> } | null {
  const [event, setEvent] = useState<InstallEvent | null>(null);

  useEffect(() => {
    const onPrompt = (e: Event) => {
      // Chrome shows its own mini-infobar unless this is prevented, which is
      // the interruption this hook exists to move.
      e.preventDefault();
      setEvent(e as InstallEvent);
    };
    // Once installed the offer is meaningless and the control goes.
    const onInstalled = () => setEvent(null);

    window.addEventListener("beforeinstallprompt", onPrompt);
    window.addEventListener("appinstalled", onInstalled);
    return () => {
      window.removeEventListener("beforeinstallprompt", onPrompt);
      window.removeEventListener("appinstalled", onInstalled);
    };
  }, []);

  if (!event) return null;

  return {
    install: async () => {
      await event.prompt();
      await event.userChoice;
      // Spent either way: the event cannot be replayed, and a control that
      // silently does nothing on a second tap is worse than one that left.
      setEvent(null);
    },
  };
}
