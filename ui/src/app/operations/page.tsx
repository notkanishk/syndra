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
import { outcomeOf, type EventOutcome } from "@/lib/event-outcome";

type Source = "all" | "provider" | "onboarding";

/**
 * S11 · System › Incoming events.
 *
 * A raw timeline, not a dashboard. Its job is forensic — "what did Zitadel
 * tell us at 09:38, and what did we do about it". No tiles, no counts, no
 * roll-ups: Home already owns actionable summary, and a second dashboard here
 * would make it ambiguous which one is authoritative.
 *
 * Webhook events and onboarding triggers are ONE time-ordered stream. They are
 * two tables in the database and one sequence of things that happened; a
 * reader following a single event across both should not have to interleave two
 * lists by eye.
 */
export default function EventActivityPage() {
  const [source, setSource] = useState<Source>("all");
  const [outcome, setOutcome] = useState<EventOutcome>("all");
  const events = useWebhookEvents();
  const triggers = useOnboardingTriggers();
  const [openDetails, setOpenDetails] = useState<string | null>(null);

  const all = useMemo(() => {
    const rows: StreamRow[] = [];
    for (const event of events.data ?? []) rows.push(fromWebhook(event));
    for (const trigger of triggers.data ?? []) rows.push(fromTrigger(trigger));
    return rows.sort((a, b) => (a.at < b.at ? 1 : -1));
  }, [events.data, triggers.data]);

  // Both filters are applied here rather than in the query. The status filter
  // the backend offers covers webhook events only and takes one exact status,
  // so it can express neither "either table" nor a bucket that spans two words
  // for the same thing — and this list polls every five seconds, so a
  // server-side filter would drop the page into a loading state on every pill.
  const stream = useMemo(
    () =>
      all.filter(
        (row) =>
          (source === "all" || row.source === source) &&
          (outcome === "all" || row.outcome === outcome),
      ),
    [all, source, outcome],
  );
  const filtered = source !== "all" || outcome !== "all";

  const isLoading = events.isLoading || triggers.isLoading;
  const error = events.error ?? triggers.error;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Incoming events"
        lede="Things that happened outside Syndra and reached it: access changed in Zitadel (the service everyone signs in through), and people joining. Newest first. Nothing here needs a decision."
        actions={
          <>
            <FilterPills<Source>
              label="Filter by source"
              value={source}
              onChange={setSource}
              options={[
                { value: "all", label: "All sources" },
                { value: "provider", label: "Zitadel" },
                { value: "onboarding", label: "New people" },
              ]}
            />
            <FilterPills<EventOutcome>
              label="Filter by outcome"
              value={outcome}
              onChange={setOutcome}
              options={[
                { value: "all", label: "Any outcome" },
                { value: "done", label: "Done" },
                { value: "waiting", label: "Waiting" },
                { value: "failed", label: "Failed" },
                // The reason this filter exists. `dropped_enrichment_incomplete`
                // was invented so a deliberate non-action stops being silent;
                // until now there was no way to ask the screen for one.
                { value: "dropped", label: "Not acted on" },
              ]}
            />
          </>
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
          errorTitle="Couldn't load the events."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading events" />}
          empty={
            // An emptied filter and an empty log are different facts, and only
            // one of them means nothing has happened.
            filtered ? (
              <EmptyState
                title="No events match those filters."
                guidance={`${all.length} ${all.length === 1 ? "event is" : "events are"} in the timeline.`}
                action={{
                  label: "Clear filters",
                  onClick: () => {
                    setSource("all");
                    setOutcome("all");
                  },
                }}
              />
            ) : (
              <EmptyState
                title="Nothing has happened yet."
                guidance="An event lands here when somebody changes access in Zitadel, or when a new person joins."
              />
            )
          }
        >
          {stream.map((row) => (
            <div key={row.id}>
              <div
                // Only the error row is tinted, and only because it is the row
                // somebody is scrolling to find. A log where half the lines are
                // coloured is a log nobody reads.
                className={`row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3 tablet:flex-row tablet:flex-wrap tablet:gap-4 ${
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
                  onClick={() => setOpenDetails((cur) => (cur === row.id ? null : row.id))}
                  aria-expanded={openDetails === row.id}
                  className="shrink-0 text-[13px] text-faint motion-tint hover:text-ink"
                >
                  {openDetails === row.id ? "Hide details" : "Show details"}
                </button>
              </div>

              {openDetails === row.id && (
                <pre className="row-divider overflow-x-auto bg-surface-0 px-5 py-3.5 font-mono text-[12.5px] leading-[1.7] text-muted">
                  {JSON.stringify(row.details, null, 2)}
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
  /** Which table it came from — what the source pills filter on. */
  source: Exclude<Source, "all">;
  /** See `outcomeOf`. */
  outcome: string;
  failed: boolean;
  details: unknown;
}

function fromWebhook(event: WebhookEventRow): StreamRow {
  const outcome = outcomeOf(event.status);
  const failed = outcome === "failed";
  return {
    id: `w:${event.id}`,
    at: event.created_at,
    type: `zitadel.${event.event_type}`,
    source: "provider",
    outcome,
    failed,
    details: event,
    sentence: failed ? (
      <>
        Failed: {event.error_message || "Syndra could not process it."} —{" "}
        <UserName id={event.user_id} />, <ProjectName id={event.source_project} />
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
        {outcome === "dropped" ? (
          <span className="text-faint"> — received and deliberately not acted on</span>
        ) : (
          <span className="text-faint"> — recorded</span>
        )}
      </>
    ),
  };
}

function fromTrigger(trigger: OnboardingTriggerRow): StreamRow {
  const outcome = outcomeOf(trigger.status);
  const failed = outcome === "failed";
  return {
    id: `t:${trigger.id}`,
    at: trigger.created_at,
    type: "onboarding.trigger",
    source: "onboarding",
    outcome,
    failed,
    details: trigger,
    sentence: failed ? (
      <>
        Failed: {trigger.error_message || "Syndra could not welcome them."} —{" "}
        <UserName id={trigger.user_id} />
      </>
    ) : (
      <>
        <UserName id={trigger.user_id} /> joined
        {trigger.bundle_id ? (
          <>
            {" — "}
            {/*
              Bundles are deletable, and this log outlives them: the trigger row keeps the id
              of whatever was handed out at the time. The default em dash would render that as
              "— given automatically", which reads as nothing having been given.
            */}
            <BundleName id={trigger.bundle_id} fallback="a bundle since retired" /> given
            automatically as the default bundle (set of roles) for new members
          </>
        ) : (
          <span className="text-faint">
            {" "}
            — no default bundle (set of roles) for new members is set, so nothing was given
          </span>
        )}
      </>
    ),
  };
}
