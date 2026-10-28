"use client";

import Link from "next/link";
import { useMemo, useState } from "react";

import { ONBOARDING_TITLE, zitadelTitle } from "@/app/operations/event-words";
import { PlaceRole } from "@/components/audit/EventLine";
import { BundleName, ProjectName, UserName } from "@/components/names";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { StatusDot } from "@/components/ui/Badge";
import { Card, CardColumns } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { FilterChip } from "@/components/ui/FilterChip";
import { FilterField, FilterPanel } from "@/components/ui/FilterPanel";
import { Input } from "@/components/ui/Input";
import { Notice } from "@/components/ui/Notice";
import { Unfold } from "@/components/ui/Unfold";
import { PageHeader } from "@/components/ui/PageHeader";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { FilterPills } from "@/components/ui/Select";
import { RunLabel, SortHead, runStarts, useSort } from "@/components/ui/SortHead";
import { Term } from "@/components/ui/Term";
import { LogTime } from "@/components/ui/Time";
import { outcomeLabel, outcomeOf, type EventOutcome } from "@/lib/event-outcome";
import { formatShortDate, formatWhen } from "@/lib/format";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import {
  LOG_LIMIT,
  useOnboardingTriggers,
  useWebhookEvents,
  type OnboardingTriggerRow,
  type WebhookEventRow,
} from "@/lib/queries/useOperations";

type Source = "all" | "provider" | "onboarding";
type SortKey = "time" | "event" | "outcome";

const SOURCE_WORD = { provider: "Zitadel", onboarding: "New people" } as const;

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
  const [query, setQuery] = useState("");
  const [sort, flipOrSet, view] = useSort<SortKey>("time");
  const events = useWebhookEvents();
  const triggers = useOnboardingTriggers();
  const resolver = useNameResolver();
  const paging = usePaging();
  const [openDetails, setOpenDetails] = useState<string | null>(null);

  const all = useMemo(() => {
    const rows: StreamRow[] = [];
    for (const event of events.data ?? []) rows.push(fromWebhook(event));
    for (const trigger of triggers.data ?? []) rows.push(fromTrigger(trigger));
    return rows;
  }, [events.data, triggers.data]);

  // Every filter is applied here rather than in the query. The status filter
  // the backend offers covers webhook events only and takes one exact status,
  // so it can express neither "either table" nor a bucket that spans two words
  // for the same thing — and this list polls every five seconds, so a
  // server-side filter would drop the page into a loading state on every pill.
  // Search reads the same names the rows show, so what is typed is what is seen.
  const needle = query.trim().toLowerCase();
  const stream = useMemo(() => {
    const names = (row: StreamRow) =>
      [
        resolver.resolveUser(row.userId).value?.display_name,
        row.projectId && resolver.resolveProject(row.projectId).value?.name,
        row.bundleId && resolver.resolveBundle(row.bundleId).value?.name,
      ];
    return all
      .filter(
        (row) =>
          (source === "all" || row.source === source) &&
          (outcome === "all" || row.outcome === outcome) &&
          (!needle || names(row).some((name) => name && name.toLowerCase().includes(needle))),
      )
      .sort((a, b) => {
        let order = view.sort.key === "time" ? b.ts - a.ts : runOf(a, view.sort.key).localeCompare(runOf(b, view.sort.key));
        if (view.sort.dir < 0) order = -order;
        return order || b.ts - a.ts;
      });
  }, [all, source, outcome, needle, view.sort, resolver]);
  const filtered = source !== "all" || outcome !== "all" || needle !== "";

  // Runs: under Time, one per calendar day; under Event or Outcome, equal values
  // gathered under a quiet label (RULES: sort lives on the headings). The count is
  // the run's whole size, not how much of it is on screen yet.
  const runAt = useMemo(() => runStarts(stream, (row) => runOf(row, view.sort.key)), [stream, view.sort.key]);
  const visible = stream.slice(0, paging.limit);

  const isLoading = events.isLoading || triggers.isLoading;
  // One stream failing no longer blanks both: a stream with nothing to show is named in a
  // Notice and the other's rows stay. Only when neither can be read is it the list's error.
  const eventsDown = Boolean(events.error) && !events.data;
  const triggersDown = Boolean(triggers.error) && !triggers.data;
  const bothDown = eventsDown && triggersDown;
  // A read that failed AFTER one had landed: the rows are what had arrived by then.
  const lastReadMs = Math.max(events.dataUpdatedAt || 0, triggers.dataUpdatedAt || 0);
  const readAt = lastReadMs ? new Date(lastReadMs).toISOString() : undefined;
  const stale = !bothDown && Boolean(events.error ?? triggers.error) && !eventsDown && !triggersDown;
  const retry = () => {
    events.refetch();
    triggers.refetch();
  };
  const clearAll = () => {
    setSource("all");
    setOutcome("all");
    setQuery("");
  };

  return (
    <div className="flex flex-col gap-[18px]">
      {/* A notice that can come and go while the page is open leaves by fading,
          then folding (A-08 §5): the Unfold opens it, and closes it. */}
      <Unfold open={Boolean(stale)} gap={18}>
        <Notice tone="warn" live title={`Events haven't been read since ${formatWhen(readAt)}.`}>
          Syndra&apos;s server stopped answering this page. The list below is what had arrived by
          then; anything newer will appear when it answers again.
        </Notice>
      </Unfold>

      <PageHeader
        title="Incoming events"
        lede={
          <>
            Things that happened outside Syndra and reached it: access changed in{" "}
            <Term name="zitadel">Zitadel</Term>, and people joining. Nothing here needs a decision.
          </>
        }
      />

      {/* The bar waits for the list: no search, read line or count until there is
          something to search. */}
      {!isLoading && !bothDown && (
        <>
          <FilterBar
            search={
              <Input
                type="search"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search person, project or bundle…"
                aria-label="Search events by person, project or bundle"
              />
            }
            filters={
              <FilterPanel
                resultCount={stream.length}
                noun={["event", "events"]}
                activeCount={[source !== "all", outcome !== "all"].filter(Boolean).length}
                onClear={() => {
                  setSource("all");
                  setOutcome("all");
                }}
              >
                <FilterField label="Source">
                  <FilterPills<Source>
                    label="Filter by source"
                    value={source}
                    onChange={setSource}
                    options={[
                      { value: "all", label: "All sources" },
                      { value: "provider", label: SOURCE_WORD.provider },
                      { value: "onboarding", label: SOURCE_WORD.onboarding },
                    ]}
                  />
                </FilterField>
                <FilterField label="Outcome">
                  <FilterPills<EventOutcome>
                    label="Filter by outcome"
                    value={outcome}
                    onChange={setOutcome}
                    options={[
                      { value: "all", label: "Any outcome" },
                      ...(["done", "waiting", "failed", "dropped"] as const).map((value) => ({
                        value,
                        label: outcomeLabel(value).word,
                      })),
                    ]}
                  />
                </FilterField>
              </FilterPanel>
            }
          />
          <div className="-mt-1 flex flex-wrap items-center gap-x-3 gap-y-2">
            <ReadFreshness state={{ readAt, current: !stale }} subject="The timeline" />
            {((events.data?.length ?? 0) >= LOG_LIMIT || (triggers.data?.length ?? 0) >= LOG_LIMIT) && (
              <span className="text-[13px] text-faint">The newest {LOG_LIMIT} of each log</span>
            )}
            {source !== "all" && (
              <FilterChip onClear={() => setSource("all")} clearLabel="Show every source">
                {SOURCE_WORD[source]}
              </FilterChip>
            )}
            {outcome !== "all" && (
              <FilterChip onClear={() => setOutcome("all")} clearLabel="Show every outcome">
                {outcomeLabel(outcome).word}
              </FilterChip>
            )}
          </div>
        </>
      )}

      <Unfold open={eventsDown !== triggersDown} gap={18}>
        <Notice
          tone="danger"
          blocking
          title={eventsDown ? "Events from Zitadel could not be read." : "People joining could not be read."}
          action={{ label: "Try again", onClick: retry }}
        >
          The list below has only what {eventsDown ? "new people" : "Zitadel"} sent, and that part
          is current. Nothing was changed.
        </Notice>
      </Unfold>

      <Card>
        <ListStates
          isLoading={isLoading}
          error={bothDown ? (events.error ?? triggers.error) : null}
          isEmpty={stream.length === 0}
          onRetry={retry}
          errorTitle="Couldn't load the events."
          skeleton={<RowSkeleton rows={6} avatar={false} label="Loading events" />}
          empty={
            // An emptied filter and an empty log are different facts, and only
            // one of them means nothing has happened.
            filtered ? (
              <EmptyState
                title="No events match those filters."
                guidance={`${all.length} ${all.length === 1 ? "event is" : "events are"} in the timeline.`}
                action={{ label: "Clear filters", onClick: clearAll }}
              />
            ) : (
              <EmptyState
                title="Nothing has happened yet."
                guidance="An event lands here when somebody changes access in Zitadel, or when a new person joins."
              />
            )
          }
        >
          {/* Headings only over rows: an empty list is a bare card (route-events, empty). */}
          <CardColumns>
            <span className="w-[200px] shrink-0">
              <SortHead label="Event" name="event" sort={sort} onSort={flipOrSet} />
            </span>
            <span className="min-w-0 flex-1">What happened</span>
            <span className="hidden w-[150px] shrink-0 desktop:block">
              <SortHead label="Outcome" name="outcome" sort={sort} onSort={flipOrSet} />
            </span>
            <span className="w-[110px] shrink-0 text-right">
              <SortHead label="Time" name="time" sort={sort} onSort={flipOrSet} />
            </span>
          </CardColumns>
          {visible.map((row, index) => {
            const run = runAt(index);
            return (
              <div key={row.id} data-event-row className={view.rowClassName}>
                {run && <RunLabel {...run} />}
                <EventRow
                  row={row}
                  sortKey={view.sort.key}
                  open={openDetails === row.id}
                  onToggle={() => setOpenDetails((cur) => (cur === row.id ? null : row.id))}
                />
              </div>
            );
          })}
          <PaginationRow
            shown={visible.length}
            total={stream.length}
            onMore={paging.more}
            onAll={() => paging.all(stream.length)}
            onBack={paging.back}
          />
        </ListStates>
      </Card>
    </div>
  );
}

function EventRow({
  row,
  sortKey,
  open,
  onToggle,
}: {
  row: StreamRow;
  sortKey: SortKey;
  open: boolean;
  onToggle: () => void;
}) {
  const { word, tone } = outcomeLabel(row.outcome);
  // Under Time the day heading carries the date, so the clock alone is enough; under any
  // other order the headings have gone and the time has to carry its own date.
  const when =
    sortKey === "time" ? <LogTime iso={row.at} /> : <>{formatWhen(row.at)}</>;
  const outcomeWord = (
    <span
      className={`inline-flex items-center gap-2 font-semibold ${
        row.failed ? "text-danger-text" : row.outcome === "done" ? "text-ink" : "text-muted"
      }`}
    >
      <StatusDot tone={tone} />
      {word}
    </span>
  );
  return (
    <>
      <div
        // Only the failed row is tinted, and only because it is the row somebody is
        // scrolling to find. A log where half the lines are coloured is a log nobody reads.
        className={`row-divider flex flex-col items-start gap-2 px-5 py-4 tablet:flex-row tablet:gap-[18px] ${
          row.failed ? "bg-danger-soft" : ""
        }`}
      >
        <span className="flex min-w-0 flex-col gap-[3px] tablet:w-[200px] tablet:shrink-0">
          <span className="text-[14px] font-semibold leading-[1.4] [text-wrap:pretty]">{row.title}</span>
          <span className="type-mono text-[12.5px] text-faint">{row.type}</span>
        </span>
        <span className="flex w-full min-w-0 flex-1 flex-col items-start gap-1.5">
          <span
            className={`text-[13.5px] leading-[1.55] [text-wrap:pretty] ${
              row.failed ? "text-danger-text" : "text-muted"
            }`}
          >
            {row.sentence}
          </span>
          {/* Outcome has its own column from desktop up; below it, it sits under the sentence. */}
          <span className="inline-flex items-center gap-2 text-[13px] desktop:hidden">
            {outcomeWord}
            <span className="font-normal text-faint tablet:hidden">· {formatWhen(row.at)}</span>
          </span>
          <button
            type="button"
            onClick={onToggle}
            aria-expanded={open}
            className="inline-flex min-h-[44px] items-center text-[12.5px] font-semibold text-muted link-tint desktop:min-h-6"
          >
            {open ? "Hide details" : "Show details"}
          </button>
        </span>
        <span className="hidden w-[150px] shrink-0 pt-px text-[13.5px] desktop:block">{outcomeWord}</span>
        <span className="type-mono hidden w-[110px] shrink-0 pt-0.5 text-right text-[12.5px] text-muted tablet:block">
          {when}
        </span>
      </div>
      {open && (
        <pre className="row-divider overflow-x-auto bg-surface-0 px-5 py-3.5 font-mono text-[12.5px] leading-[1.7] text-muted">
          {JSON.stringify(row.details, null, 2)}
        </pre>
      )}
    </>
  );
}

/** The label a run of rows sits under, for the order the list is in. */
function runOf(row: StreamRow, key: SortKey): string {
  return key === "time" ? dayLabel(row.at) : key === "event" ? row.title : row.word;
}

/**
 * "Today", "Yesterday", else the short date — keyed by the SAME `formatShortDate` the rest of
 * the product reads dates in. A UTC-derived key here against a local-time clock on the row is
 * exactly the mismatch that once put an entry under the wrong date.
 */
function dayLabel(iso: string, now: Date = new Date()): string {
  const date = formatShortDate(iso, now);
  if (date === formatShortDate(now.toISOString(), now)) return "Today";
  const yesterday = new Date(now.getTime() - 864e5);
  return date === formatShortDate(yesterday.toISOString(), now) ? "Yesterday" : date;
}

interface StreamRow {
  id: string;
  at: string;
  ts: number;
  /** Which table it came from — what the source pills filter on. */
  source: Exclude<Source, "all">;
  /** The event in words. */
  title: string;
  /** The raw type, kept in mono beneath the title for whoever matches it against Zitadel. */
  type: string;
  sentence: React.ReactNode;
  /** See `outcomeOf`. */
  outcome: string;
  /** The outcome in words; what an Outcome sort gathers on. */
  word: string;
  failed: boolean;
  details: unknown;
  /** What search reads. */
  userId: string;
  projectId?: string;
  bundleId?: string;
}

function Person({ id }: { id: string }) {
  return (
    <Link href={`/users/${id}`} className="text-ink link-hover">
      <UserName id={id} />
    </Link>
  );
}

function Project({ id }: { id: string }) {
  return (
    <Link href={`/projects/${id}`} className="text-ink link-hover">
      <ProjectName id={id} />
    </Link>
  );
}

/** What a webhook event was about, as a sentence. The sentence says only what Syndra knows. */
function webhookSentence(event: WebhookEventRow): React.ReactNode {
  const who = <Person id={event.user_id} />;
  // The enrichment-incomplete drop is exactly the row with no project: it is written empty.
  const place = event.source_project ? (
    event.role_key ? (
      <PlaceRole projectId={event.source_project} roleKey={event.role_key} />
    ) : (
      <Project id={event.source_project} />
    )
  ) : (
    "a role"
  );
  switch (event.event_type) {
    case "grant_added":
      return <>{who} was given {place}.</>;
    case "grant_removed":
      return <>{who} no longer has {place}.</>;
    case "grant_changed":
      return event.source_project ? (
        <>{who}&apos;s access on <Project id={event.source_project} /> changed.</>
      ) : (
        <>{who}&apos;s access changed.</>
      );
    case "user_created":
      return <>{who} was added.</>;
    case "user_deactivated":
      return <>{who} was deactivated.</>;
    case "user_locked":
      return <>{who} was locked.</>;
    default:
      return <>Zitadel sent an event about {who}.</>;
  }
}

function fromWebhook(event: WebhookEventRow): StreamRow {
  const outcome = outcomeOf(event.status);
  const failed = outcome === "failed";
  const base = webhookSentence(event);
  // What Syndra did about it, in the same breath as what happened.
  const tail =
    outcome === "failed" ? (
      <> Syndra could not record it{event.error_message ? `: ${event.error_message}` : "."}</>
    ) : outcome === "dropped" ? (
      event.status === "dropped_enrichment_incomplete" ? (
        <> Zitadel did not say which project or role this was about, so Syndra could not act on it.</>
      ) : (
        <> Syndra chose not to act.</>
      )
    ) : outcome === "waiting" ? (
      <> Syndra is still recording it.</>
    ) : outcome === "done" ? (
      <> Syndra recorded it.</>
    ) : null;
  return {
    id: `w:${event.id}`,
    at: event.created_at,
    ts: Date.parse(event.created_at),
    source: "provider",
    title: zitadelTitle(event.event_type),
    type: `zitadel.${event.event_type}`,
    sentence: (
      <>
        {base}
        {tail}
      </>
    ),
    outcome,
    word: outcomeLabel(outcome).word,
    failed,
    details: event,
    userId: event.user_id,
    projectId: event.source_project || undefined,
  };
}

function fromTrigger(trigger: OnboardingTriggerRow): StreamRow {
  const outcome = outcomeOf(trigger.status);
  const failed = outcome === "failed";
  const who = <Person id={trigger.user_id} />;
  // Bundles are deletable, and this log outlives them: the trigger row keeps the id of
  // whatever was handed out at the time. The default em dash would render a retired bundle as
  // nothing, which reads as nothing having been given.
  const bundle = trigger.bundle_id ? (
    <>
      <BundleName id={trigger.bundle_id} fallback="a bundle since retired" />, the default{" "}
      <Term name="bundle">bundle</Term> for new members
    </>
  ) : (
    <>the default <Term name="bundle">bundle</Term> for new members</>
  );
  const tail =
    outcome === "failed" ? (
      <> Syndra could not welcome them{trigger.error_message ? `: ${trigger.error_message}` : "."}</>
    ) : outcome === "dropped" ? (
      <> Nothing was given: no default bundle for new members is set.</>
    ) : outcome === "waiting" ? (
      <> Syndra is giving them {bundle}.</>
    ) : outcome === "done" && trigger.bundle_id ? (
      <> Syndra gave them {bundle}.</>
    ) : null;
  return {
    id: `t:${trigger.id}`,
    at: trigger.created_at,
    ts: Date.parse(trigger.created_at),
    source: "onboarding",
    title: ONBOARDING_TITLE,
    type: "onboarding.trigger",
    sentence: (
      <>
        {who} joined.{tail}
      </>
    ),
    outcome,
    word: outcomeLabel(outcome).word,
    failed,
    details: trigger,
    userId: trigger.user_id,
    bundleId: trigger.bundle_id,
  };
}
