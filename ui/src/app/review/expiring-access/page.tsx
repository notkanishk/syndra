"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useMemo, useRef, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { KEPT, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { BulkDialog } from "@/components/people/BulkDialog";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { FilterField, FilterPanel } from "@/components/ui/FilterPanel";
import { StatusDot } from "@/components/ui/Badge";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { ReadLine, SectionPill } from "@/components/ui/SectionNav";
import { FilterPills, Select } from "@/components/ui/Select";
import { RunLabel, SortHead, runStarts, useSort, type Sort } from "@/components/ui/SortHead";
import { Unfold } from "@/components/ui/Unfold";
import {
  RowCheckbox,
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { BULK_MAX_USERS } from "@/lib/queries/useBulkGrants";
import { useRowSelection, type RowSelection } from "@/lib/useRowSelection";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { RoleRef, UserAvatar, UserName } from "@/components/names";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import {
  useAcknowledgeExpiry,
  useClearExpiryAcknowledgement,
  useExpiringGrants,
  type ExpiringGrantRow,
} from "@/lib/queries/useExpiringAccess";
import { useCreateGrant } from "@/lib/queries/useUsers";
import { daysUntil, formatShortDate, humanizeKey } from "@/lib/format";
import {
  outcomeFromError,
  statesNothingChanged,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";
import { ActionOutcome } from "@/components/ui/ActionOutcome";

/** How long an extension buys. Stated in the row's own result, never implied. */
const EXTEND_DAYS = 90;

/**
 * S7 · Review › Expiring access.
 *
 * Its own route, not an audit tab: audit is a record you consult, this is
 * time-boxed work you act on before a deadline, and a sidebar badge should
 * point at a destination rather than a tab.
 *
 * Two actions, and they are not symmetrical. **Extend** changes the access.
 * **Let it lapse** changes nothing about the access — it records that somebody
 * looked and decided, which is the only thing this queue could not previously
 * say. The sweep removes the grant on its date either way.
 *
 * That second action was deliberately absent for a long time, on the grounds
 * that a control which submits nothing is worse than no control. It submits
 * something now: a stored, audited, shared decision, which stops asking the
 * whole team a question one of them has already answered.
 *
 * It is per-row and never bulk. Acknowledging is exactly the act that must not
 * be doable to twelve rows in one gesture — the value of the record is that a
 * person read the row.
 */
/**
 * The one expiring window, from `?within=`: 14 or 30 days, 14 first (A-04 §4,
 * A-06 §7). Home's tile and the rail open on 14, so the number you tapped is the
 * number you land on, and Insights' Ending soon uses the same two. Anything else
 * in the link, 7 or 90 included, is 14.
 */
const WINDOWS = [14, 30] as const;
type Window = (typeof WINDOWS)[number];

function windowDays(raw: string | null): Window {
  return WINDOWS.find((d) => String(d) === raw) ?? WINDOWS[0];
}

type SortKey = "who" | "what" | "given" | "ends";

/** The two cards. `ends` is the clock: soonest first unless a heading says otherwise. */
const SECTIONS = {
  undecided: {
    id: "sec-undecided",
    label: "Undecided",
    hint: "Each ends on its date unless you extend it",
    tone: "warn",
  },
  lapse: {
    id: "sec-lapse",
    label: "Let lapse",
    hint: "Somebody looked and decided. Extend still works",
    tone: undefined,
  },
} as const;
type SectionKey = keyof typeof SECTIONS;

export default function ExpiringAccessPage() {
  const router = useRouter();
  const within = windowDays(useSearchParams().get("within"));
  const grants = useExpiringGrants(within);
  const rows = useMemo(() => [...(grants.data ?? [])].sort(sortBySoonest), [grants.data]);

  // Search finds a row and the project narrows the window; neither is in the link, which
  // carries only how far ahead (`?within=`). Names are the ones the rows show, so what is typed
  // is what is seen.
  const resolver = useNameResolver();
  const [query, setQuery] = useState("");
  const [project, setProject] = useState("");
  const [sort, flipSort] = useSort<SortKey>("ends");
  const [open, setOpen] = useState<Record<SectionKey, boolean>>({ undecided: true, lapse: true });
  const text = useMemo(
    () => ({
      who: (grant: ExpiringGrantRow) => resolver.resolveUser(grant.user_id).value?.display_name ?? "",
      what: (grant: ExpiringGrantRow) =>
        `${resolver.resolveProject(grant.project_id).value?.name ?? ""} · ${humanizeKey(grant.role_key)}`,
      given: (grant: ExpiringGrantRow) =>
        resolver.resolveUser(grant.granted_by).value?.display_name ?? "",
    }),
    [resolver],
  );
  const needle = query.trim().toLowerCase();
  const filtered = needle !== "" || project !== "";
  // Still soonest first: the amber row and the "soonest ends" line are about the clock, not
  // about the order a heading has put the rows in.
  const matched = useMemo(
    () =>
      rows.filter(
        (grant) =>
          (!project || grant.project_id === project) &&
          (!needle ||
            [text.who(grant), text.what(grant), grant.role_key].some((part) =>
              part.toLowerCase().includes(needle),
            )),
      ),
    [rows, project, needle, text],
  );
  // Only projects with a row in the window, so nothing on offer returns nothing.
  const projects = useMemo(() => {
    const byId = new Map<string, string>();
    for (const grant of rows) {
      if (!byId.has(grant.project_id))
        byId.set(grant.project_id, resolver.resolveProject(grant.project_id).value?.name ?? "");
    }
    return Array.from(byId, ([id, name]) => ({ id, name })).sort((a, b) => a.name.localeCompare(b.name));
  }, [rows, resolver]);
  // A project from the other window that this one has no row for would filter to nothing under
  // a heading that says so. Reset it once the new window has been read.
  if (project && !grants.isLoading && !grants.error && !projects.some((entry) => entry.id === project)) {
    setProject("");
  }
  const ordered = useMemo(() => sortExpiring(matched, sort, text), [matched, sort, text]);
  const soonestId = matched.find((grant) => !grant.acknowledged)?.id;

  // Two groups, not a filter. The cost this screen imposes is rescanning rows somebody has
  // already judged, and a heading is where the eye stops; a filter would hide the same rows
  // behind a control an operator has to remember to check.
  const undecided = useMemo(() => ordered.filter((grant) => !grant.acknowledged), [ordered]);
  const acknowledged = useMemo(() => ordered.filter((grant) => grant.acknowledged), [ordered]);
  // Across the whole window: "every one decided" is a fact about the queue, not about a search.
  const anyUndecided = rows.some((grant) => !grant.acknowledged);

  const sections = useMemo(
    () =>
      (["undecided", "lapse"] as const)
        .map((key) => ({
          key,
          ...SECTIONS[key],
          count: (key === "undecided" ? undecided : acknowledged).length,
        }))
        .filter((section) => section.count > 0),
    [undecided, acknowledged],
  );
  const openSection = (section: { key: SectionKey }) =>
    setOpen((cur) => ({ ...cur, [section.key]: true }));
  const readAt = useMemo(
    () => (grants.dataUpdatedAt ? new Date(grants.dataUpdatedAt).toISOString() : null),
    [grants.dataUpdatedAt],
  );

  // Selection spans the undecided rows only. Extending is the one thing worth doing to a dozen at
  // once, and a row already dealt with is not part of the working set.
  const selection = useRowSelection(useMemo(() => undecided.map((grant) => grant.id), [undecided]));
  // Selection is announced by a named control rather than by a permanent
  // column of checkboxes. On this screen the row's own actions are the common
  // case — most visits acknowledge one grant — and a checkbox in front of
  // every row makes the rare bulk errand look like the point of the page.
  const [selecting, setSelecting] = useState(false);
  const [extending, setExtending] = useState(false);
  const [lapsing, setLapsing] = useState<ExpiringGrantRow | null>(null);

  // The bulk endpoint extends by user, and it only ever touches grants that
  // actually expire — which is every row on this screen.
  // The ticked rows themselves. This screen's rows ARE grants, and both halves have to travel:
  // the ids say what to extend, the user ids say whose plan to rehearse. Sending only the people
  // would extend every expiring grant they hold, including ones beyond this screen's 30 days —
  // rows the operator never saw, let alone chose.
  const selectedGrants = useMemo(
    () => undecided.filter((grant) => selection.isSelected(grant.id)),
    [undecided, selection],
  );
  const selectedGrantIds = useMemo(() => selectedGrants.map((grant) => grant.id), [selectedGrants]);
  const selectedUsers = useMemo(
    () => Array.from(new Set(selectedGrants.map((grant) => grant.user_id))),
    [selectedGrants],
  );

  const people = new Set(matched.map((grant) => grant.user_id)).size;
  const soonest = matched.find((grant) => !grant.acknowledged)?.expires_at;
  const showRows = !grants.isLoading && !grants.error && rows.length > 0;
  const section = (key: SectionKey, list: ExpiringGrantRow[], first: boolean) => (
    <ExpiringSection
      {...SECTIONS[key]}
      rows={list}
      open={open[key]}
      onToggle={() => setOpen((cur) => ({ ...cur, [key]: !cur[key] }))}
      sort={sort}
      onSort={flipSort}
      // Only the soonest undecided row is amber: a deadline signal, not a decoration.
      soonestId={first ? soonestId : undefined}
      selecting={selecting}
      selection={selection}
      onLetLapse={setLapsing}
      selectAll={
        first && selecting ? (
          <SelectAllRow
            inScope={undecided.length}
            total={matched.length}
            noun={["role", "roles"]}
            allSelected={selection.allSelected}
            {...selection.headerCheckboxProps}
          />
        ) : null
      }
    />
  );

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Expiring access"
        meta={
          showRows ? (
            <MetaRow>
              {[
                `${matched.length} ${matched.length === 1 ? "role" : "roles"}${filtered ? " matching" : ""} · ${people} ${people === 1 ? "person" : "people"}`,
                // "Every one decided" is true of the whole window or it is not said: a search or
                // a project that matches nothing has decided nothing.
                soonest
                  ? `soonest ends ${formatShortDate(soonest)}`
                  : !anyUndecided
                    ? "every one decided"
                    : "",
              ]}
            </MetaRow>
          ) : undefined
        }
        lede={`Direct access that ends in the next ${within} days. Syndra ends each on its date whether or not you visit this page. Extend gives ${EXTEND_DAYS} more days. Let it lapse changes nothing: it records that you looked, and the row asks again if the date changes.`}
      />

      {/* A filter, not an action, so FilterPills and not a Segmented. Select is an action on the
          list, so it trails. The default window keeps the link clean. */}
      <FilterBar
        leading={
          <FilterPills
            label="How far ahead"
            value={String(within)}
            options={WINDOWS.map((d) => ({ value: String(d), label: `${d} days` }))}
            onChange={(next) =>
              router.replace(
                next === String(WINDOWS[0])
                  ? "/review/expiring-access"
                  : `/review/expiring-access?within=${next}`,
                { scroll: false },
              )
            }
          />
        }
        search={
          showRows ? (
            <Input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search person or role…"
              aria-label="Search expiring access by person or role"
            />
          ) : undefined
        }
        filters={
          showRows ? (
            <FilterPanel
              resultCount={matched.length}
              noun={["role", "roles"]}
              activeCount={project ? 1 : 0}
              onClear={() => setProject("")}
            >
              <FilterField label="Project">
                <Select
                  value={project}
                  onChange={(event) => setProject(event.target.value)}
                  aria-label="Filter by project"
                >
                  <option value="">All projects</option>
                  {projects.map((entry) => (
                    <option key={entry.id} value={entry.id}>
                      {entry.name}
                    </option>
                  ))}
                </Select>
              </FilterField>
            </FilterPanel>
          ) : undefined
        }
        trailing={
          undecided.length > 0 ? (
            <SelectModeToggle active={selecting} onToggle={() => setSelecting((on) => !on)} />
          ) : undefined
        }
      />

      {/* The read line: how current the list is on the left (Syndra's own, until the sweep
          records its runs), a chip per card at the right end. Not while the read is out. */}
      {!grants.isLoading && !grants.error && (
        <ReadLine sections={sections} label="Sections of expiring access" onOpen={openSection} className="-mt-1">
          <ReadFreshness
            state={{ readAt, current: true }}
            subject="Syndra’s own list"
            onRefresh={() => grants.refetch()}
          />
        </ReadLine>
      )}

      {showRows && matched.length === 0 ? (
        <Card>
          <EmptyState
            title="Nothing that ends matches those filters."
            guidance={`${rows.length} ${rows.length === 1 ? "role ends" : "roles end"} in the next ${within} days.`}
            action={{
              label: "Clear filters",
              onClick: () => {
                setQuery("");
                setProject("");
              },
            }}
          />
        </Card>
      ) : showRows ? (
        <div data-selection-scope {...selection.containerProps} className="flex flex-col gap-4">
          {/* Everything in the window has been judged. Said outright, because a queue whose only
              rows are acknowledged ones looks identical to a queue nobody has touched. */}
          <SectionPill sections={sections} label="Sections of expiring access" onOpen={openSection} />
          {!anyUndecided && (
            <span className="inline-flex items-center gap-2.5 px-1 text-[13.5px] text-muted">
              <StatusDot tone="healthy" />
              Nothing here is waiting on a decision. Every role that ends in the next {within} days has
              been looked at.
            </span>
          )}
          {undecided.length > 0 && section("undecided", undecided, true)}
          {acknowledged.length > 0 && section("lapse", acknowledged, false)}
        </div>
      ) : (
        <Card>
          <ListStates
            isLoading={grants.isLoading}
            error={grants.error}
            isEmpty
            onRetry={() => grants.refetch()}
            errorTitle="Couldn't load access that ends."
            skeleton={<RowSkeleton rows={4} label="Loading access that ends" />}
            empty={
              <EmptyState
                title={`Nothing ends in the next ${within} days.`}
                guidance="Direct access appears here a month before it ends."
                resolved
              />
            }
          >
            {null}
          </ListStates>
        </Card>
      )}

      <SelectionBar
        count={selecting ? selection.count : 0}
        noun={["role", "roles"]}
        composition={
          selectedUsers.length > 0
            ? `${selectedUsers.length} ${selectedUsers.length === 1 ? "person" : "people"}`
            : ""
        }
        // The bulk endpoint caps distinct PEOPLE at 500 and this screen selects
        // GRANTS, so the number the bar counts is not the number the server
        // refuses on. 600 grants held by 300 people is legal; 500 grants held
        // by 500 people is already at the limit.
        ceiling={BULK_MAX_USERS}
        ceilingCount={selectedUsers.length}
        ceilingNoun={["person", "people"]}
        onTakeCeiling={() => {
          // Whole people, in the order shown. Once the cohort is full the
          // later grants of somebody already in it still come along — dropping
          // them would extend part of a person's access and leave the rest to
          // lapse, which is a worse thing to do than refuse.
          const cohort = new Set<string>();
          const keep: string[] = [];
          for (const grant of undecided) {
            if (!cohort.has(grant.user_id)) {
              if (cohort.size === BULK_MAX_USERS) continue;
              cohort.add(grant.user_id);
            }
            keep.push(grant.id);
          }
          selection.selectOnly(keep);
        }}
        onClear={selection.clear}
      >
        {/* Tapping this opens a plan, it does not extend anything. */}
        <SelectionAction onClick={() => setExtending(true)}>
          Preview an extension
        </SelectionAction>
      </SelectionBar>

      {extending && (
        <BulkDialog
          op="extend"
          userIds={selectedUsers}
          grantIds={selectedGrantIds}
          scope="with access that ends"
          onClose={() => {
            setExtending(false);
            selection.clear();
          }}
        />
      )}

      {lapsing && <LetItLapseDialog grant={lapsing} onClose={() => setLapsing(null)} />}
    </div>
  );
}

/**
 * One card of the two: a counted heading, the column headings, then rows ten at a time
 * (route-expiring.dc.html). Paging is per section, so a long Undecided never pushes the decided
 * rows out of reach.
 */
function ExpiringSection({
  id,
  label,
  hint,
  tone,
  rows,
  open,
  onToggle,
  sort,
  onSort,
  soonestId,
  selecting,
  selection,
  onLetLapse,
  selectAll,
}: {
  id: string;
  label: string;
  hint: string;
  tone?: "warn";
  rows: ExpiringGrantRow[];
  open: boolean;
  onToggle: () => void;
  sort: Sort<SortKey>;
  onSort: (key: SortKey) => void;
  soonestId?: string;
  selecting: boolean;
  selection: RowSelection;
  onLetLapse: (grant: ExpiringGrantRow) => void;
  selectAll: React.ReactNode;
}) {
  const paging = usePaging();
  const resolver = useNameResolver();
  const anchor = useRef<HTMLDivElement>(null);
  const shown = rows.slice(0, paging.limit);
  // What and Given by gather equal values under a counted label; Who and Ends in do not.
  const runAt = runStarts(
    rows,
    sort.key === "what"
      ? (grant) => `${resolver.resolveProject(grant.project_id).value?.name ?? ""} · ${humanizeKey(grant.role_key)}`
      : sort.key === "given"
        ? (grant) => resolver.resolveUser(grant.granted_by).value?.display_name ?? "somebody no longer listed"
        : null,
  );
  return (
    <section id={id} aria-labelledby={`${id}-title`} className="scroll-mt-4">
      <Card>
        <div ref={anchor}>
          <CardHeader
            tone={tone}
            count={rows.length}
            note={hint}
            title={
              <button
                type="button"
                aria-expanded={open}
                aria-controls={`${id}-body`}
                onClick={onToggle}
                className="inline-flex items-center gap-3 text-left"
              >
                <span
                  aria-hidden
                  className={`flex size-7 flex-none items-center justify-center rounded-pill border border-line text-[13px] text-muted motion-press ${
                    open ? "rotate-90" : ""
                  }`}
                >
                  ›
                </span>
                <span id={`${id}-title`}>{label}</span>
              </button>
            }
          />
        </div>
        <Unfold open={open}>
          <div id={`${id}-body`}>
            <CardColumns>
              {selecting && <span className="w-11 shrink-0 desktop:w-[26px]" />}
              <span className="w-[190px]">
                <SortHead label="Who" name="who" sort={sort} onSort={onSort} />
              </span>
              <span className="flex-1">
                <SortHead label="What they can get into" name="what" sort={sort} onSort={onSort} />
              </span>
              <span className="hidden w-[160px] desktop:block">
                <SortHead label="Given by" name="given" sort={sort} onSort={onSort} />
              </span>
              <span className="w-[110px]">
                <SortHead label="Ends in" name="ends" sort={sort} onSort={onSort} />
              </span>
              <span className="min-w-[300px]" />
            </CardColumns>
            {selectAll}
            {shown.map((grant, index) => {
              const run = runAt(index);
              return (
                <div key={grant.id}>
                  {run && <RunLabel {...run} />}
                  <ExpiringRow
                    grant={grant}
                    soonest={grant.id === soonestId}
                    selecting={selecting}
                    selection={selection}
                    onLetLapse={() => onLetLapse(grant)}
                  />
                </div>
              );
            })}
            <PaginationRow
              shown={shown.length}
              total={rows.length}
              onMore={paging.more}
              onAll={() => paging.all(rows.length)}
              onBack={paging.back}
              anchor={anchor}
            />
          </div>
        </Unfold>
      </Card>
    </section>
  );
}

function ExpiringRow({
  grant,
  soonest,
  selecting,
  selection,
  onLetLapse,
}: {
  grant: ExpiringGrantRow;
  soonest: boolean;
  selecting: boolean;
  selection: RowSelection;
  onLetLapse: () => void;
}) {
  const ack = grant.acknowledged ?? null;
  // Extending re-submits the grant with a later date: POST upserts on
  // (user, project, role) and overwrites expires_at, so this renews in place
  // rather than creating a duplicate.
  const extend = useCreateGrant(grant.user_id);
  // No sheet of its own: a small one holds the step if extending would cancel
  // a change still waiting for you to send (A-04 §14).
  const step = useCancelStep({ host: "Extend this access", goAhead: "extend", standalone: true });
  const clearAck = useClearExpiryAcknowledgement();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const remaining = daysUntil(grant.expires_at);
  // The result takes the seat the buttons held, until the next read. A refusal does not: the
  // buttons stay beside it, so the attempt can be made again.
  const reported = outcome !== null && !statesNothingChanged(outcome.kind);

  return (
    <div
      className={`row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-[18px] ${
        soonest
          ? "border-l-[3px] border-warn bg-warn-soft"
          : selecting && ack
            ? // A row selection cannot reach keeps a dashed edge while the mode
              // is on, so an empty checkbox cell reads as "not eligible" rather
              // than as a checkbox that failed to draw. Outside the mode there
              // is nothing to be ineligible for, and the edge would be noise.
              "border-l-[3px] border-dashed border-line-strong"
            : "border-l-[3px] border-transparent"
      } ${selection.isSelected(grant.id) ? "bg-accent-soft" : ""}`}
      {...(ack ? {} : selection.rowProps(grant.id))}
    >
      {selecting && (
        <span className="w-11 shrink-0 desktop:w-[26px]">
          {/* No checkbox on a row somebody has dealt with — it is not part of the working set, and
              offering it back into a bulk extend would undo a decision by accident. The reason is
              already on the row: who acknowledged it, and when. */}
          {!ack && (
            <RowCheckbox
              label={`Select ${grant.role_key}, ending ${formatShortDate(grant.expires_at)}`}
              {...selection.checkboxProps(grant.id)}
            />
          )}
        </span>
      )}
      <span className="flex w-full min-w-0 items-center gap-3 tablet:w-[190px] tablet:shrink-0">
        <UserAvatar id={grant.user_id} />
        <Link
          href={`/users/${grant.user_id}`}
          className="min-w-0 text-[15px] font-semibold hover:text-accent-text hover:underline hover:underline-offset-[3px]"
        >
          <UserName id={grant.user_id} />
        </Link>
      </span>

      <div className="flex w-full min-w-0 flex-col gap-[3px] tablet:flex-1">
        <span className="text-[14.5px] leading-[1.45]">
          <RoleRef projectId={grant.project_id} roleKey={grant.role_key} />
        </span>
        <span className="type-mono text-[12.5px] text-faint">{grant.role_key}</span>
        {/* Laptop width only: the Given by column needs a desktop, and a phone has none (#54). */}
        <span className="hidden text-[12.5px] text-faint tablet:block desktop:hidden">
          Given by <UserName id={grant.granted_by} fallback="somebody no longer listed" />,{" "}
          {formatShortDate(grant.created_at)}
        </span>
        {ack && (
          // Who and when, always. The point of a shared acknowledgement is that the next person
          // can see whose judgement they would be overriding.
          <span className="mt-[3px] text-[13.5px] leading-[1.5] text-muted">
            <span className="font-semibold text-ink">
              <UserName id={ack.by} fallback="somebody no longer listed" />
            </span>{" "}
            let it lapse on {formatShortDate(ack.at)}
            {ack.note ? ` — ${ack.note}` : ""}
          </span>
        )}
      </div>

      <div className="hidden w-[160px] shrink-0 text-[13.5px] leading-[1.45] text-muted desktop:block">
        <UserName id={grant.granted_by} fallback="somebody no longer listed" />
        <span className="block text-[12.5px] text-faint">{formatShortDate(grant.created_at)}</span>
      </div>

      <div className="flex flex-col gap-0.5 tablet:w-[110px] tablet:shrink-0">
        <span className={`text-[13.5px] font-semibold ${soonest ? "text-warn-text" : "text-muted"}`}>
          {remaining === null
            ? "—"
            : remaining <= 0
              ? "Ends today"
              : `${remaining} ${remaining === 1 ? "day" : "days"} left`}
        </span>
        <span className="text-[12.5px] text-faint">{formatShortDate(grant.expires_at)}</span>
      </div>

      <div className="flex w-full min-w-0 flex-col items-start gap-2 desktop:w-auto desktop:min-w-[300px] desktop:items-end">
        {reported ? (
          <ActionOutcome placement="seat" word={outcome.message} outcome={outcome} />
        ) : (
          <div className="flex w-full flex-wrap items-center gap-2 tablet:w-auto tablet:justify-end">
            {/* Extend stays on an acknowledged row. Changing your mind toward KEEPING somebody's
                access is the reversal that must never be harder than the one that lets it go. */}
            <Button
              variant={soonest ? "accent" : "outline"}
              size="sm"
              isPending={extend.isPending}
              onClick={async () => {
                try {
                  const result = await step.run(() =>
                    extend.mutateAsync({
                      project_id: grant.project_id,
                      role_key: grant.role_key,
                      reason: "Extended from Expiring access",
                      duration_days: EXTEND_DAYS,
                      extend: true,
                    }),
                  );
                  if (result === KEPT) return;
                  // This endpoint enqueues to the outbox in manual mode ("pending")
                  // rather than applying inline — see CreateGrantResult. Reporting
                  // "applied" unconditionally would say Zitadel has it when only
                  // Syndra's own ledger does.
                  const waiting = result?.status === "pending";
                  setOutcome({
                    kind: waiting ? "queued" : "applied",
                    message: waiting
                      ? `Will extend by ${EXTEND_DAYS} days once sent`
                      : `Extended by ${EXTEND_DAYS} days`,
                    detail: waiting ? undefined : "It leaves this list on the next read.",
                  });
                } catch (error) {
                  setOutcome(outcomeFromError(error));
                }
              }}
            >
              Extend {EXTEND_DAYS} days
            </Button>
            {ack ? (
              <Button
                variant="ghost"
                size="sm"
                isPending={clearAck.isPending}
                onClick={async () => {
                  try {
                    await clearAck.mutateAsync(grant.id);
                    setOutcome({
                      kind: "applied",
                      message: "Back in the undecided list",
                      detail: "The list asks about it again.",
                    });
                  } catch (error) {
                    setOutcome(outcomeFromError(error));
                  }
                }}
              >
                Undo letting it lapse
              </Button>
            ) : (
              <Button variant="ghost" size="sm" onClick={onLetLapse}>
                Let it lapse
              </Button>
            )}
          </div>
        )}
        {step.element}
        {outcome && !reported && <ActionOutcome placement="seat" outcome={outcome} />}
      </div>
    </div>
  );
}

/**
 * The acknowledgement dialog. A dialog rather than a click for two reasons: the note is the part
 * that makes the record useful to somebody else, and this is the one place to state plainly that
 * recording a decision is not the same as changing anything.
 */
/**
 * Whether the action ran. A FAILED attempt keeps its button — retiring the
 * control on any outcome at all would leave a refusal on screen with no way to
 * try again.
 */
function succeeded(outcome: Outcome | null): boolean {
  return outcome !== null && !statesNothingChanged(outcome.kind);
}

function LetItLapseDialog({
  grant,
  onClose,
}: {
  grant: ExpiringGrantRow;
  onClose: () => void;
}) {
  const acknowledge = useAcknowledgeExpiry();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [note, setNote] = useState("");

  const expiresAt = grant.expires_at;

  return (
    <Modal open onClose={onClose} busy={acknowledge.isPending} size="md" labelledBy="lapse-title">
      <ModalHeader
        title="Let this lapse?"
        titleId="lapse-title"
        lede={
          <>
            <RoleRef projectId={grant.project_id} roleKey={grant.role_key} /> for{" "}
            <UserName id={grant.user_id} />, ending {formatShortDate(expiresAt)}.
          </>
        }
      />

      <div className="px-6">
        <div className="accent-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1">What this does</div>
          <ul className="flex flex-col gap-1 text-muted">
            <li>
              Nothing to the access. It ends on {formatShortDate(expiresAt)}, as it would have; this
              records that you looked.
            </li>
            <li>The list stops asking, and shows your name against the decision.</li>
            <li>
              If somebody extends it or gives it again, the row comes back and asks again: you agreed to
              this date, not a later one.
            </li>
          </ul>
        </div>

        <div className="mt-4">
          <FieldLabel htmlFor="lapse-note">Why (optional)</FieldLabel>
          <Input
            id="lapse-note"
            value={note}
            onChange={(event) => setNote(event.target.value)}
            placeholder="Cohort ends this term"
          />
          <FieldHint>
            The next person reading this queue is the audience — they see this instead of asking
            you.
          </FieldHint>
        </div>
      </div>

      {/* The dialog states what it did, and this is the one screen where that
          sentence matters most: recording a decision changes nothing about the
          access, and the dialog exists to say so. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter note={succeeded(outcome) ? undefined : "Extend instead if the access should continue."}>
        {/* Nothing closed this on success, so the same "Record the decision"
            button sat there re-submittable — a second click recorded the same
            acknowledgment again, against a grant that had not moved. */}
        {!succeeded(outcome) && (
          <Button
            variant="accent"
            isPending={acknowledge.isPending}
            onClick={async () => {
              if (!expiresAt) return;
              try {
                await acknowledge.mutateAsync({
                  grantId: grant.id,
                  expiresAt,
                  note: note.trim(),
                });
                setOutcome({
                  kind: "applied",
                  message: "Recorded — it will end on its date",
                  detail:
                    "Nothing about the access changed — it still ends on its date. What changed is that the queue stops asking your colleagues a question you have answered.",
                });
              } catch (error) {
                // A 409 here is the reopen rule arriving early: the grant was
                // extended while this dialog was open, so the date on screen is
                // not the date the grant has. The server's message says to
                // reload, and a refusal is what it should read as.
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            Record the decision
          </Button>
        )}
        <Button
          variant={succeeded(outcome) ? "outline" : "ghost"}
          onClick={onClose}
          disabled={acknowledge.isPending}
        >
          {succeeded(outcome) ? "Close" : "Cancel"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

function sortBySoonest(a: ExpiringGrantRow, b: ExpiringGrantRow): number {
  if (!a.expires_at) return 1;
  if (!b.expires_at) return -1;
  return a.expires_at < b.expires_at ? -1 : 1;
}

/**
 * The order a heading puts a card's rows in. Ends in is the clock itself, soonest first;
 * the others compare their words, and equal words fall back to the clock.
 */
function sortExpiring(
  rows: ExpiringGrantRow[],
  sort: Sort<SortKey>,
  text: Record<"who" | "what" | "given", (grant: ExpiringGrantRow) => string>,
): ExpiringGrantRow[] {
  return [...rows].sort((a, b) => {
    if (sort.key === "ends") return sortBySoonest(a, b) * sort.dir;
    const order = text[sort.key](a).localeCompare(text[sort.key](b)) * sort.dir;
    return order || sortBySoonest(a, b);
  });
}
