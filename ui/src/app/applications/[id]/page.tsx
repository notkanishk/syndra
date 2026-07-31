"use client";

import { use } from "react";

import { AppTokenScreen } from "@/components/apps/AppTokenScreen";

/**
 * E6 · App token — format and preview. One screen, two equal halves.
 *
 * This is the most technical thing in Basic, and it earns its place by being
 * the fastest way to end "my app isn't seeing the roles it expects".
 */
export default function AppPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <AppTokenScreen applicationId={id} />;
}
