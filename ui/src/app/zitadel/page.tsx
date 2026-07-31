"use client";

import { EmptyState, ErrorState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useZitadelHealth } from "@/lib/queries/useZitadel";
import { useProjects } from "@/lib/queries/useProjects";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api-client";

interface RotationStatus {
  status?: string;
  rotated_at?: string;
  message?: string;
  days_since_rotation?: number;
}

/**
 * S9 · System › Identity provider.
 *
 * Read-mostly. Zitadel owns authorization state; this screen exists to answer
 * "is it reachable, does it agree with us, and when was the signing key last
 * rotated" — not to become a second console for editing it.
 */
export default function IdentityProviderPage() {
  const health = useZitadelHealth();
  const projects = useProjects();
  const rotation = useQuery({
    queryKey: ["zitadel", "rotation"],
    queryFn: () => request<RotationStatus>("/zitadel/action-rotation-status"),
    retry: false,
  });

  const live = health.data?.status === "ok";

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Identity provider"
        meta={health.data?.domain ? <Mono>{health.data.domain}</Mono> : "Zitadel"}
      />

      <div className="flex flex-wrap gap-[18px]">
        <Card className="min-w-[300px] flex-1">
          <CardHeader title="Reachability" />
          <div className="row-divider flex flex-col gap-2 px-5 py-4">
            {health.isLoading ? (
              <RowSkeleton rows={2} avatar={false} label="Checking the identity provider" />
            ) : health.error ? (
              <ErrorState
                title="Couldn't reach the health endpoint."
                error={health.error}
                onRetry={() => health.refetch()}
              />
            ) : (
              <>
                <div className="flex items-center gap-2.5">
                  <span
                    aria-hidden
                    className={`h-2.5 w-2.5 rounded-pill ${live ? "bg-accent" : "bg-danger"}`}
                  />
                  <span className="text-[15px] font-semibold">
                    {live ? "Reachable" : health.data?.status === "disabled" ? "Not configured" : "Unreachable"}
                  </span>
                  {health.data?.latency_ms !== undefined && (
                    <span className="text-[13.5px] text-faint">{health.data.latency_ms}ms</span>
                  )}
                </div>
                <p className="text-[13.5px] leading-[1.55] text-muted">
                  {live
                    ? `Running in ${health.data?.mode} mode with ${health.data?.projects_total ?? 0} projects.`
                    : health.data?.error ||
                      "MkAuth keeps queueing writes while it is unreachable — nothing is lost, and nothing reaches the provider until it returns."}
                </p>
              </>
            )}
          </div>
        </Card>

        <Card className="min-w-[300px] flex-1">
          <CardHeader title="Action signing key" />
          <div className="row-divider flex flex-col gap-2 px-5 py-4">
            {rotation.isLoading ? (
              <RowSkeleton rows={2} avatar={false} label="Checking the signing key" />
            ) : rotation.error ? (
              <p className="text-[13.5px] text-muted">
                Rotation status is unavailable. The claim-injection target keeps working; only this
                report is missing.
              </p>
            ) : (
              <>
                <div className="text-[15px] font-semibold">
                  {rotation.data?.status ?? "unknown"}
                </div>
                <p className="text-[13.5px] leading-[1.55] text-muted">
                  {rotation.data?.message ??
                    "The key signs every claim-injection call. Rotating it invalidates the old one immediately."}
                </p>
              </>
            )}
          </div>
        </Card>
      </div>

      <Card>
        <CardHeader
          title="Projects it knows about"
          count={(projects.data ?? []).length}
          note="What MkAuth reads from the provider, not what MkAuth manages."
        />
        <ListStates
          isLoading={projects.isLoading}
          error={projects.error}
          isEmpty={(projects.data ?? []).length === 0}
          onRetry={() => projects.refetch()}
          errorTitle="Couldn't list projects."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Loading projects" />}
          empty={
            <EmptyState
              title="No projects visible."
              guidance="Either none exist, or the service account can't see them."
            />
          }
        >
          {(projects.data ?? []).map((entry) => (
            <div key={entry.project.id} className="row-divider flex items-center gap-4 px-5 py-3">
              <span className="min-w-0 flex-1 truncate text-[15px] font-semibold">
                {entry.project.name}
              </span>
              <Mono className="w-[240px] shrink-0 truncate text-faint">{entry.project.id}</Mono>
              <Badge>{entry.active_role_keys.length} roles</Badge>
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
