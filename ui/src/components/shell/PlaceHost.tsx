"use client";

import { usePathname } from "next/navigation";

import { usePlaceScroll } from "@/lib/place";

/** Back keeps your place (A-19 P5): the app's scroll, for this session only. */
export function PlaceHost() {
  usePlaceScroll(usePathname());
  return null;
}
