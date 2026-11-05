"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { Suspense } from "react";

import { usePlaceScroll } from "@/lib/place";

function Scroll() {
  const query = useSearchParams().toString();
  // The route is the path and the query: a query-only change is a place of its own (A-20 P19).
  usePlaceScroll(usePathname() + (query ? `?${query}` : ""));
  return null;
}

/** Back keeps your place (A-19 P5): the app's scroll, for this session only. */
export function PlaceHost() {
  return (
    <Suspense fallback={null}>
      <Scroll />
    </Suspense>
  );
}
