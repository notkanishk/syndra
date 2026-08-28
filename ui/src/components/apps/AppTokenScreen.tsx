"use client";

import { Term } from "@/components/ui/Term";
import { useMemo, useState } from "react";

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
  // The editor holds edits the preview has not seen. Lifted here because they
  // are siblings, and the preview's whole claim is that it shows what this app
  // would receive right now.
  const [editing, setEditing] = useState(false);

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
        <RowSkeleton rows={4} avatar={false} label="Loading app" />
      </Card>
    );
  }
  if (apps.error) {
    return (
      <ErrorState
        title="Couldn't load this app."
        error={apps.error}
        onRetry={() => apps.refetch()}
      />
    );
  }
  if (!app) {
    return (
      <ErrorState
        title="That app doesn't exist."
        error={new Error("It may have been removed in Zitadel.")}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[22px]">
      <PageHeader
        title={app.application.name}
        lede={
          <>
            When someone signs in to {app.application.name}, Zitadel hands it a{" "}
            <Term name="token">token</Term> — a short list of facts about that person, including
            their roles. Token format sets what goes in it; Preview token shows exactly what{" "}
            {app.application.name} would receive for a given person.
          </>
        }
        meta={
          <span className="flex flex-wrap items-center gap-2 text-[14px] text-faint">
            uses roles from <ProjectName id={app.application.project_id} /> · id{" "}
            <Mono>{app.application.id}</Mono>
          </span>
        }
      />

      <div className="flex flex-col items-stretch gap-5 tablet:flex-row tablet:flex-wrap">
        <div className="w-full tablet:min-w-[420px] tablet:flex-1">
          <TokenFormatEditor
            projectId={app.application.project_id}
            applicationId={applicationId}
            applicationName={app.application.name}
            shape={shape}
            siblingCount={siblings.length}
            onDirtyChange={setEditing}
          />
        </div>
        <div className="w-full tablet:min-w-[420px] tablet:flex-1">
          {/* The preview reads the SAVED shape. While the editor holds
              unsaved edits it is a preview of a shape nobody is looking at,
              and side by side on a desktop that is misleading; stacked on a
              phone, where the editor is a scroll away, it is worse. It says so
              rather than being quietly wrong. */}
          <TokenPreview
            applicationId={applicationId}
            applicationName={app.application.name}
            projectId={app.application.project_id}
            projectName={shape.data?.project_name}
            behindEdits={editing}
          />
        </div>
      </div>
    </div>
  );
}
