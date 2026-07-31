"use client";

import { useMemo } from "react";

import { TokenFormatEditor } from "@/components/apps/TokenFormatEditor";
import { TokenPreview } from "@/components/apps/TokenPreview";
import { ErrorState, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { ProjectName } from "@/components/names";
import { Card } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { useApplications } from "@/lib/queries/useApplications";
import { useClaimShape } from "@/lib/queries/useClaimShape";
import { useCrumb } from "@/lib/page-crumb";

/**
 * E6 · One screen, two equal halves: what the token is shaped like on the
 * left, what it actually contains on the right.
 *
 * Both halves read from the same claim profiles the data plane applies, so the
 * preview is the token. Before this, the simulator invented an envelope the
 * Actions v2 path never emitted — a debugging screen that showed a token which
 * did not exist.
 */
export function AppTokenScreen({ applicationId }: { applicationId: string }) {
  const apps = useApplications();
  const app = (apps.data ?? []).find((entry) => entry.application.id === applicationId);
  const shape = useClaimShape(app?.application.project_id);

  useCrumb(app?.application.name);

  const siblings = useMemo(
    () =>
      (apps.data ?? []).filter(
        (entry) =>
          entry.application.project_id === app?.application.project_id &&
          entry.application.id !== applicationId,
      ),
    [apps.data, app?.application.project_id, applicationId],
  );

  if (apps.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={4} avatar={false} label="Loading application" />
      </Card>
    );
  }
  if (apps.error) {
    return (
      <ErrorState
        title="Couldn't load this application."
        error={apps.error}
        onRetry={() => apps.refetch()}
      />
    );
  }
  if (!app) {
    return (
      <ErrorState
        title="That application doesn't exist."
        error={new Error("It may have been removed from the identity provider.")}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[22px]">
      <PageHeader
        title={app.application.name}
        meta={
          <span className="flex flex-wrap items-center gap-2 text-[14px] text-faint">
            {app.application.consumer} · reads <ProjectName id={app.application.project_id} /> ·{" "}
            <Mono>{app.application.id}</Mono>
          </span>
        }
      />

      <div className="flex flex-wrap items-stretch gap-5">
        <div className="min-w-[420px] flex-1">
          <TokenFormatEditor
            projectId={app.application.project_id}
            applicationId={applicationId}
            applicationName={app.application.name}
            shape={shape}
            siblingCount={siblings.length}
          />
        </div>
        <div className="min-w-[420px] flex-1">
          <TokenPreview
            applicationId={applicationId}
            applicationName={app.application.name}
            projectId={app.application.project_id}
          />
        </div>
      </div>
    </div>
  );
}
