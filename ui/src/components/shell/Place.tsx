"use client";

import { useEffect, useLayoutEffect, useState, type MouseEvent, type RefObject } from "react";

import { GlideIndicator, SETTLE_MS, stillMotion, useGlideIndicator, type Box } from "@/lib/motion";

/**
 * Your place in the rail and the tab bar (A-25 moment 4; owner, 20 Nov 2026: "do glide with light").
 * The pill, the bold violet word and the dot carry the place; the pill glides to a new one with the
 * glide indicator's own movement, shaped as the row is (one box, its own corners), and a faint light
 * travels under it. It travels only when the place was reached by the pointer, a press on one of the
 * group's own links just before the path changed (a plain primary click); a key (Go to, Enter on a
 * row), Back, a first paint and a later re-measure land it at once, and so does reduced motion. A
 * place inside a fold that is moving or shut has no box: no pill and no light, and the row's own fill
 * marks it while it is there.
 */
export function usePlace<T extends HTMLElement = HTMLDivElement>(pathname: string, view?: unknown) {
  // Measured again when the path changes or the view does: a view's fold moves the rows.
  const { ref, box } = useGlideIndicator<T>(`${pathname} ${String(view)}`);
  const [pressed, setPressed] = useState<string | null>(null);
  const [place, setPlace] = useState({ path: pathname, travel: false });
  // Decided once, when the path changes: travel only if this group's own link was clicked for it.
  if (place.path !== pathname) {
    setPlace({ path: pathname, travel: pressed === pathname });
    setPressed(null);
  }
  // One glide, then still: a later re-measure (a resize, a fold) lands it at once.
  useEffect(() => {
    if (!place.travel) return;
    const settled = setTimeout(() => setPlace((now) => (now.path === place.path ? { ...now, travel: false } : now)), SETTLE_MS);
    return () => clearTimeout(settled);
  }, [place]);
  // A plain primary click by a pointer on one of the group's own links: not a key (detail 0), not a
  // new tab or window (a modifier, another button), which leaves this page where it is.
  const onClick = (event: MouseEvent<HTMLElement>) => {
    if (event.detail === 0 || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    const link = (event.target as Element).closest<HTMLAnchorElement>("a[href]");
    if (link && event.currentTarget.contains(link)) setPressed(new URL(link.href).pathname);
  };
  return { ref, box, travel: place.travel, onClick };
}

/** The pill and its light, inside the group whose rows they mark. */
export function PlaceMarks({
  group,
  box,
  travel,
  axis,
}: {
  group: RefObject<HTMLElement | null>;
  box: Box | null;
  travel: boolean;
  /** The rail spends its light top to bottom; the tab bar, side to side. */
  axis: "rail" | "bar";
}) {
  return (
    <>
      <PlaceLight group={group} box={box} travel={travel} axis={axis} />
      <GlideIndicator box={box} kind="fill" travel={travel} className="place-pill bg-accent-soft" />
    </>
  );
}

/**
 * One pool under the pill, the room's own layer (A-25 moment 4): as wide as the rail and up to 176px
 * tall in the rail, as tall as the bar and up to 160px wide in the tab bar, and never larger than the
 * room it has, so its closest-side falloff is spent inside every edge (rule 1). It moves with the pill;
 * its size, where it has to change, changes at once.
 */
function PlaceLight({ group, box, travel, axis }: { group: RefObject<HTMLElement | null>; box: Box | null; travel: boolean; axis: "rail" | "bar" }) {
  const [pool, setPool] = useState<{ left: number; top: number; width: number; height: number } | null>(null);
  useLayoutEffect(() => {
    const el = group.current;
    const room = el?.closest<HTMLElement>("[data-place-room]");
    if (!el || !room || !box) return setPool(null);
    // The group's place in its room, and the room's whole extent (a rail scrolls: its content's).
    const at = el.getBoundingClientRect();
    const of = room.getBoundingClientRect();
    const dx = at.left - of.left - room.clientLeft;
    const dy = at.top - of.top - room.clientTop + room.scrollTop;
    if (axis === "rail") {
      const centre = dy + box.top + box.height / 2;
      const height = Math.min(176, 2 * Math.min(centre, room.scrollHeight - centre));
      setPool({ left: -dx, top: box.top + box.height / 2 - height / 2, width: room.clientWidth, height });
    } else {
      const centre = dx + box.left + box.width / 2;
      const width = Math.min(160, 2 * Math.min(centre, room.clientWidth - centre));
      setPool({ left: box.left + box.width / 2 - width / 2, top: -dy, width, height: room.clientHeight });
    }
  }, [group, box, axis]);
  if (!pool) return null;
  const glide = travel && !stillMotion() ? "place-light-glide" : "";
  return (
    <span
      aria-hidden
      data-place-light={axis}
      className={`place-light pointer-events-none absolute left-0 top-0 ${glide}`}
      style={{ transform: `translate(${pool.left}px, ${pool.top}px)`, width: pool.width, height: pool.height }}
    />
  );
}
