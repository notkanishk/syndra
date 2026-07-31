"use client";

import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { FilterPills } from "@/components/ui/Select";
import { PageHeader } from "@/components/ui/PageHeader";
import { BundleName, ProjectName, UserName } from "@/components/names";
import {
  useOnboardingTriggers,
  useWebhookEvents,
  type OnboardingTriggerRow,
  type WebhookEventRow,
} from "@/lib/queries/useOperations";
import { ClockTime, LogTime } from "@/components/ui/Time";

type Source = "all" | "provider" | "onboarding";

/**
 * S11 · System › Event activity.
 *
 * A raw timeline, not a dashboard. Its job is forensic — "what did the identity
 * provider tell us at 09:38, and what did we do about it". No tiles, no counts,
 * no roll-ups: Today already owns actionable summary, and a second dashboard
 * here would make it ambiguous which one is authoritative.
 *
 * Webhook events and onboarding triggers are ONE time-ordered stream. They are
 * two tables in the database and one sequence of things that happened; a
 * reader following a single event across both should not have to interleave two
 * lists by eye.
 */
export default function EventActivityPage() {
  const [source, setSource] = useState<Source>("all");
  const events = useWebhookEvents();
  const triggers = useOnboardingTriggers();
  const [openPayload, setOpenPayload] = useState<string | null>(null);

  const stream = useMemo(() => {
    const rows: StreamRow[] = [];
    if (source !== "onboarding") {
      for (const event of events.data ?? []) rows.push(fromWebhook(event));
    }
    if (source !== "provider") {
      for (const trigger of triggers.data ?? []) rows.push(fromTrigger(trigger));
    }
    return rows.sort((a, b) => (a.at < b.at ? 1 : -1));
  }, [events.data, triggers.data, source]);

  const isLoading = events.isLoading || triggers.isLoading;
  const error = events.error ?? triggers.error;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Event activity"
        meta="What the identity provider told us, in the order it told us."
        actions={
          <FilterPills<Source>
            label="Filter by source"
            value={source}
            onChange={setSource}
            options={[
              { value: "all", label: "All sources" },
              { value: "provider", label: "Identity provider" },
              { value: "onboarding", label: "Onboarding" },
            ]}
          />
        }
      />

      <Card>
        <CardHeader
          title="Timeline"
          count={stream.length}
          note={
            <>
              newest first · loaded <ClockTime />
            </>
          }
        />
        <ListStates
          isLoading={isLoading}
          error={error}
          isEmpty={stream.length === 0}
          onRetry={() => {
            events.refetch();
            triggers.refetch();
          }}
          errorTitle="Couldn't load event activity."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading events" />}
          empty={
            <EmptyState
              title="Nothing has happened yet."
              guidance="Events arrive when somebody changes access in the identity provider, or when a new person is onboarded."
            />
          }
        >
          {stream.map((row) => (
            <div key={row.id}>
              <div
                // Only the error row is tinted, and only because it is the row
                // somebody is scrolling to find. A log where half the lines are
                // coloured is a log nobody reads.
                className={`row-divider flex flex-wrap items-start gap-4 px-5 py-3 ${
                  row.failed ? "bg-danger-soft" : ""
                }`}
              >
                <Mono className="w-[64px] shrink-0 text-faint">
                  <LogTime iso={row.at} />
                </Mono>
                <span className="w-[160px] shrink-0 truncate text-[12.5px] font-semibold">
                  {row.type}
                </span>
                <span
                  className={`min-w-[240px] flex-1 text-[13.5px] leading-[1.5] ${
                    row.failed ? "text-danger-text" : "text-muted"
                  }`}
                >
                  {row.sentence}
                </span>
                <button
                  type="button"
                  onClick={() => setOpenPayload((cur) => (cur === row.id ? null : row.id))}
                  aria-expanded={openPayload === row.id}
                  className="shrink-0 text-[13px] text-faint transition-colors hover:text-ink"
                >
                  payload {openPayload === row.id ? "⌃" : "⌄"}
                </button>
              </div>

              {openPayload === row.id && (
                <pre className="row-divider overflow-x-auto bg-surface-0 px-5 py-3.5 font-mono text-[12.5px] leading-[1.7] text-muted">
                  {JSON.stringify(row.payload, null, 2)}
                </pre>
              )}
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}

interface StreamRow {
  id: string;
  at: string;
  type: string;
  sentence: React.ReactNode;
  failed: boolean;
  payload: unknown;
}

function fromWebhook(event: WebhookEventRow): StreamRow {
  const failed = event.status === "failed";
  return {
    id: `w:${event.id}`,
    at: event.created_at,
    type: `provider.${event.event_type}`,
    failed,
    payload: event,
    sentence: failed ? (
      <>
        {event.error_message || "Processing failed."} — <UserName id={event.user_id} />,{" "}
        <ProjectName id={event.source_project} />
      </>
    ) : (
      <>
        <UserName id={event.user_id} /> · <ProjectName id={event.source_project} />
        {event.role_key ? (
          <>
            {" / "}
            <Mono>{event.role_key}</Mono>
          </>
        ) : null}
        {event.status.startsWith("dropped") ? (
          <span className="text-faint"> — received and deliberately not acted on</span>
        ) : (
          <span className="text-faint"> — processed</span>
        )}
      </>
    ),
  };
}

function fromTrigger(trigger: OnboardingTriggerRow): StreamRow {
  const failed = trigger.status === "failed";
  return {
    id: `t:${trigger.id}`,
    at: trigger.created_at,
    type: "onboarding.trigger",
    failed,
    payload: trigger,
    sentence: failed ? (
      <>
        {trigger.error_message || "Onboarding failed."} — <UserName id={trigger.user_id} />
      </>
    ) : (
      <>
        <UserName id={trigger.user_id} /> arrived via {trigger.source}
        {trigger.bundle_id ? (
          <>
            {" — "}
            <BundleName id={trigger.bundle_id} /> assigned automatically
          </>
        ) : (
          <span className="text-faint"> — no default bundle configured, nothing assigned</span>
        )}
      </>
    ),
  };
}
