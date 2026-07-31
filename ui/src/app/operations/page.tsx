"use client";

import { useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { FilterPills } from "@/components/ui/Select";
import { PageHeader } from "@/components/ui/PageHeader";
import { useOnboardingTriggers, useWebhookEvents } from "@/lib/queries/useOperations";
import { Relative } from "@/components/ui/Time";

type Status = "all" | "processed" | "failed" | "dropped_enrichment_incomplete";

/**
 * S11 · System › Event activity.
 *
 * A raw timeline, not a dashboard. Its job is forensic — "what did the
 * identity provider tell us at 14:12, and what did we do about it". No summary
 * counts, no tiles: Today owns actionable summary, and a second dashboard here
 * would make it ambiguous which one is authoritative.
 */
export default function EventActivityPage() {
  const [status, setStatus] = useState<Status>("all");
  const events = useWebhookEvents(status === "all" ? {} : { status });
  const triggers = useOnboardingTriggers();

  const rows = events.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Event activity"
        meta="What the identity provider told us, in the order it told us."
        actions={
          <FilterPills<Status>
            label="Filter by outcome"
            value={status}
            onChange={setStatus}
            options={[
              { value: "all", label: "All" },
              { value: "processed", label: "Processed" },
              { value: "failed", label: "Failed" },
              { value: "dropped_enrichment_incomplete", label: "Dropped" },
            ]}
          />
        }
      />

      <Card>
        <CardHeader title="Inbound events" count={rows.length} />
        <ListStates
          isLoading={events.isLoading}
          error={events.error}
          isEmpty={rows.length === 0}
          onRetry={() => events.refetch()}
          errorTitle="Couldn't load event activity."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading events" />}
          empty={
            <EmptyState
              title={status === "all" ? "No inbound events yet." : "Nothing with that outcome."}
              guidance="Events arrive when somebody changes access in the identity provider directly."
            />
          }
        >
          {rows.map((event) => (
            <div key={event.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
              <Mono className="w-[260px] shrink-0 truncate">{event.event_type}</Mono>
              <div className="min-w-0 flex-1 truncate text-[13.5px] text-muted">
                {event.user_id}
                {event.source_project ? ` · ${event.source_project}` : ""}
              </div>
              <Badge
                tone={
                  event.status === "failed"
                    ? "danger"
                    : event.status.startsWith("dropped")
                      ? "warn"
                      : "neutral"
                }
              >
                {event.status}
              </Badge>
              <div className="w-[110px] shrink-0 text-right text-[13px] text-faint">
                <Relative iso={event.created_at} />
              </div>
              {event.error_message && (
                <div className="w-full text-[13px] text-danger-text">{event.error_message}</div>
              )}
            </div>
          ))}
        </ListStates>
      </Card>

      <Card>
        <CardHeader title="Onboarding triggers" count={(triggers.data ?? []).length} />
        <ListStates
          isLoading={triggers.isLoading}
          error={triggers.error}
          isEmpty={(triggers.data ?? []).length === 0}
          onRetry={() => triggers.refetch()}
          errorTitle="Couldn't load onboarding triggers."
          skeleton={<RowSkeleton rows={3} avatar={false} label="Loading triggers" />}
          empty={
            <EmptyState
              title="Nobody has been onboarded recently."
              guidance="A trigger is written when a new person arrives and the default bundle is applied."
            />
          }
        >
          {(triggers.data ?? []).map((trigger) => (
            <div key={trigger.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
              <Mono className="w-[260px] shrink-0 truncate">{trigger.user_id}</Mono>
              <Badge tone={trigger.status === "failed" ? "danger" : "neutral"}>
                {trigger.status}
              </Badge>
              <span className="flex-1" />
              <div className="text-[13px] text-faint"><Relative iso={trigger.created_at} /></div>
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
