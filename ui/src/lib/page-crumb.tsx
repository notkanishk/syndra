"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

/**
 * The trailing breadcrumb for a detail page.
 *
 * Ids resolve asynchronously, so a detail page publishes its human name once
 * it has one. Until then the crumb is simply absent — the breadcrumb reads
 * "People" rather than flashing "People / u_2f81" and then correcting itself.
 * Never render a raw id where a name is coming.
 */

interface PageCrumbValue {
  crumb: string | null;
  setCrumb: (value: string | null) => void;
}

const PageCrumbContext = createContext<PageCrumbValue | null>(null);

export function PageCrumbProvider({ children }: { children: React.ReactNode }) {
  const [crumb, setCrumb] = useState<string | null>(null);
  const value = useMemo(() => ({ crumb, setCrumb }), [crumb]);
  return <PageCrumbContext.Provider value={value}>{children}</PageCrumbContext.Provider>;
}

export function usePageCrumb(): PageCrumbValue {
  return useContext(PageCrumbContext) ?? { crumb: null, setCrumb: () => {} };
}

/**
 * Publishes a detail page's name into the breadcrumb, and clears it on
 * unmount so the next route never inherits the previous page's title.
 */
export function useCrumb(label: string | null | undefined) {
  const { setCrumb } = usePageCrumb();
  const clear = useCallback(() => setCrumb(null), [setCrumb]);

  useEffect(() => {
    setCrumb(label ?? null);
    return clear;
  }, [label, setCrumb, clear]);
}
