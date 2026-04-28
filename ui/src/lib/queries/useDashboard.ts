"use client";

import { useAuditEntries } from "@/lib/queries/useAudit";
import { useBundles } from "@/lib/queries/useBundles";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useProjects } from "@/lib/queries/useProjects";

/**
 * Composition hook for the admin dashboard. Aggregates the four queries the
 * hero needs into a single call-site so the page doesn't have to wire and
 * track each one individually. React Query dedupes per-key so other surfaces
 * that read the same hooks (e.g. the audit page) share the cache.
 */
export function useDashboardSummary() {
  const governance = useGovernanceSummary();
  const audit = useAuditEntries({ limit: 20 });
  const projects = useProjects();
  const bundles = useBundles();

  return {
    governance,
    audit,
    projects,
    bundles,
    isLoading:
      governance.isLoading || audit.isLoading || projects.isLoading || bundles.isLoading,
    isError: Boolean(governance.error || audit.error || projects.error || bundles.error),
  };
}
