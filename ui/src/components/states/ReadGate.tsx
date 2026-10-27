"use client";

import { ListStates } from "@/components/states";

/**
 * A card or a screen that is one read (Motion v2 S7): its skeleton appears
 * only if the read is still going after 200ms, fades when the read lands, and
 * then the content arrives. It never crossfades.
 *
 * This is `ListStates` for what is not a list. The content owns its own empty
 * and failed states, so nothing here is checked for either; the gate is only
 * the hand-over, built once so a loading card cannot hard-swap.
 */
export function ReadGate({
  loading,
  skeleton,
  children,
}: {
  loading: boolean;
  skeleton: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <ListStates
      isLoading={loading}
      error={null}
      isEmpty={false}
      errorTitle=""
      empty={null}
      skeleton={skeleton}
    >
      {children}
    </ListStates>
  );
}
