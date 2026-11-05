// design-sync entry: the synced surface (primitives + shell + names + states)
// plus the app's own root providers. Re-exports only - no component code here.
import "./process-shim";
import React, { useLayoutEffect, useRef, useState } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { PathnameContext } from "next/dist/shared/lib/hooks-client-context.shared-runtime";

import { ErrorBoundary } from "@/components/ErrorBoundary";
import { NameResolverProvider } from "@/lib/queries/useNameResolver";
import { getQueryClient, makeQueryClient } from "@/lib/query-client";
import { ThemeProvider } from "@/lib/theme";
import { PortalTarget } from "@/components/ui/Modal";

export * from "@/components/ui/Acknowledge";
export * from "@/components/ui/ActionOutcome";
export * from "@/components/ui/Anchored";
export * from "@/lib/motion";
export * from "@/components/ui/ArrowLink";
export * from "@/components/ui/Avatar";
export * from "@/components/ui/Badge";
export * from "@/components/ui/Button";
export * from "@/components/ui/CancelQueuedStep";
export * from "@/components/ui/Card";
export * from "@/components/ui/CheckTrail";
export * from "@/components/ui/Choice";
export * from "@/components/ui/ChoiceRow";
export * from "@/components/ui/CommandBlock";
export * from "@/components/ui/CopiedSeat";
export * from "@/components/ui/CopyableValue";
export * from "@/components/ui/FilterBar";
export * from "@/components/ui/FilterChip";
export * from "@/components/ui/FilterPanel";
export * from "@/components/ui/FormPart";
export * from "@/components/ui/GoTo";
export * from "@/components/ui/Input";
export * from "@/components/ui/Kbd";
export * from "@/components/ui/KeysSheet";
export * from "@/components/ui/Modal";
export * from "@/components/ui/NextStep";
export * from "@/components/ui/Notice";
export * from "@/components/ui/NotOpenYet";
export * from "@/components/ui/PageHeader";
export * from "@/components/ui/PaginationRow";
export * from "@/components/ui/PlanReview";
export * from "@/components/ui/ReadFreshness";
export * from "@/components/ui/RehearsalDialog";
export * from "@/components/ui/RoleWordsTable";
export * from "@/components/ui/SectionNav";
export * from "@/components/ui/Select";
export * from "@/components/ui/SelectionBar";
export * from "@/components/ui/SignInRow";
export * from "@/components/ui/Skeleton";
export * from "@/components/ui/SortHead";
export * from "@/components/ui/StatCard";
export * from "@/components/ui/StatusSeat";
export * from "@/components/ui/Tabs";
export * from "@/components/ui/Term";
export * from "@/components/ui/Time";
export * from "@/components/ui/TokenBlock";
export * from "@/components/ui/WaitsMark";
export * from "@/components/ui/Withheld";

export * from "@/components/shell/AccountSheet";
export * from "@/components/shell/AppShell";
export { default as Sidebar } from "@/components/shell/Sidebar";
export * from "@/components/shell/SyndraMark";
export * from "@/components/shell/ThemeToggle";
export * from "@/components/shell/TopBar";
export * from "@/components/shell/TouchNav";
export * from "@/components/shell/ViewSwitch";

export * from "@/components/names/BundleName";
export * from "@/components/names/ProjectName";
export * from "@/components/names/RoleRef";
export * from "@/components/names/Unresolved";
export * from "@/components/names/UserAvatar";
export * from "@/components/names/UserName";

export * from "@/components/states/DegradedBanner";
export * from "@/components/states/OfflineBanner";
export * from "@/components/states/ReadGate";
export * from "@/components/states";

export { ThemeProvider };
export { useTheme } from "@/lib/theme";
export { UiViewProvider, useUiView } from "@/lib/ui-view";
export { BASIC_NAV, ADVANCED_NAV } from "@/lib/nav";
export { ApiError, WouldCancelQueued } from "@/lib/api-client";

/**
 * The app root outside Next.js: the same stack as `components/providers.tsx`
 * (query client, name resolver, theme, error boundary - minus the dev-only
 * query devtools) plus the current route, which the shell's `usePathname()`
 * reads to mark the active nav item. Inside the Next app the router supplies
 * the route; everywhere else, pass it here.
 *
 * `data` answers queries without a backend: each entry primes one query key
 * (e.g. `["name-catalog"]`, `["system", "mode"]`, `["governance", "indicators"]`)
 * on a client of its own, so seeded roots never share state.
 */
export function SyndraRoot({
  pathname = "/",
  data,
  children,
}: {
  pathname?: string;
  data?: Array<{ key: readonly unknown[]; value: unknown }>;
  children: React.ReactNode;
}) {
  const [client] = useState(() => {
    if (!data) return getQueryClient();
    const seeded = makeQueryClient();
    for (const { key, value } of data) seeded.setQueryData(key, value);
    return seeded;
  });
  // On the design canvas a frame marks itself [data-syndra-portal]; overlays
  // portal into it instead of the board's <body>. Resolved before paint (a
  // layout effect re-renders synchronously), so a dialog drawn open never
  // flashes on the board first. Outside a frame, behaviour is the app's.
  const probe = useRef<HTMLSpanElement>(null);
  const [frame, setFrame] = useState<HTMLElement | null | undefined>(undefined);
  useLayoutEffect(() => {
    setFrame(probe.current?.closest<HTMLElement>("[data-syndra-portal]") ?? null);
  }, []);
  if (frame === undefined) return <span ref={probe} hidden />;
  return (
    <PortalTarget.Provider value={frame}>
      <PathnameContext.Provider value={pathname}>
        <QueryClientProvider client={client}>
          <NameResolverProvider>
            <ThemeProvider>
              <ErrorBoundary>{children}</ErrorBoundary>
            </ThemeProvider>
          </NameResolverProvider>
        </QueryClientProvider>
      </PathnameContext.Provider>
    </PortalTarget.Provider>
  );
}
