"use client";

import { useEffect, useState } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Segmented } from "@/components/ui/Select";

import { getChimeContext, isChimeEnabled, playDriftChime, setChimeEnabled } from "@/lib/driftChime";
import { useReducedMotion } from "@/lib/useViewport";

/**
 * The drift chime, as a per-browser preference: Off · On, and under it one
 * line that says whether On will actually be heard.
 *
 * Two things can silence it while it reads "On" — a browser set to reduce
 * motion, and a browser that allows sound only after a click on the page — and
 * a control that claims to be on while guaranteeing silence is a lie it is in
 * a position to catch.
 */
export function ChimeToggle() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [suspended, setSuspended] = useState(false);
  const quieted = useReducedMotion();

  useEffect(() => setEnabled(isChimeEnabled()), []);

  // Only worth a context while it is on and could play.
  useEffect(() => {
    if (!enabled || quieted) return;
    const ctx = getChimeContext();
    if (!ctx) return;
    const read = () => setSuspended(ctx.state === "suspended");
    read();
    ctx.addEventListener?.("statechange", read);
    return () => ctx.removeEventListener?.("statechange", read);
  }, [enabled, quieted]);

  // Render nothing until the stored preference is read: a toggle that flips
  // itself after hydration reads as the setting having changed.
  if (enabled === null) return <span aria-hidden className="block h-8" />;

  const line = quieted
    ? "Silent in this browser. It asks for less motion, and a sound that arrives on its own is movement in the room. Nothing will play while that is set."
    : suspended
      ? "Can't play yet. This browser allows sound only after a click on the page. Play it once to allow it for this visit."
      : "Plays here. This is the tone you'll hear.";

  return (
    <div className="flex flex-col items-start gap-2">
      <Segmented
        size="sm"
        label="Sound when drift is found"
        options={[
          { value: "off", label: "Off" },
          { value: "on", label: "On" },
        ]}
        value={enabled ? "on" : "off"}
        onChange={(next) => {
          setChimeEnabled(next === "on");
          setEnabled(next === "on");
        }}
      />
      {enabled && (
        <>
          <span className="inline-flex items-start gap-2 text-[13px] text-muted">
            <StatusDot
              tone={quieted ? "neutral" : suspended ? "warn" : "healthy"}
              className="mt-[7px]"
            />
            {line}
          </span>
          {!quieted && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                playDriftChime();
                const ctx = getChimeContext();
                if (ctx) setSuspended(ctx.state === "suspended");
              }}
            >
              Play it once
            </Button>
          )}
        </>
      )}
    </div>
  );
}
