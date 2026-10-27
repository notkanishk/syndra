"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useRef, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ServiceAccountTag, useAccountHref } from "@/components/system/ServiceAccountTag";
import { ArriveList } from "@/components/ui/ArriveList";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Mono, StatusDot } from "@/components/ui/Badge";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { FilterField, FilterPanel } from "@/components/ui/FilterPanel";
import { Input } from "@/components/ui/Input";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { ReadLine, SectionPill } from "@/components/ui/SectionNav";
import { Select } from "@/components/ui/Select";
import { LeavingRow, useLeavingRows, type LeaveProps } from "@/components/ui/LeavingRows";
import { RunLabel, SortHead, orderBy, runStarts, useSort, type Sort } from "@/components/ui/SortHead";
import { Relative } from "@/components/ui/Time";
import { Unfold } from "@/components/ui/Unfold";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import { request } from "@/lib/api-client";
import { shortId } from "@/lib/audit-vocabulary";
import { formatShortDate, humanizeKey } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useNameResolver } from "@/lib/queries/useNameResolver";

/**
 * Review › Unfinished revocations (9.9, 9.10).
 *
 * Beside Unexplained access, never inside it, because they are different
 * questions with different answers. Unexplained access appeared and cannot be
 * explained; this is access somebody revoked that is still there.
 *
 * The two populations render differently on purpose:
 *
 *   queued — still being sent, and the only content of the signal is how long
 *            it has been. Nothing is wrong yet.
 *   spent  — given up. Nothing will send it again, so waiting produces nothing
 *            and somebody has to act.
 *
 * Merged into one count, a healthy queue of five-minute-old rows hides a
 * revocation that failed permanently three days ago.
 */
interface UnconfirmedRevocation {
  id: string;
  op_type: string;
  user_id: string;
  project_id: string;
  role_keys: string[];
  status: string;
  attempts: number;
  last_error?: string;
  created_at: string;
  target: string;
  age_seconds: number;
  spent: boolean;
  /** Why an add-on's revocation was queued, where its project and roles say nothing. */
  reason?: string;
  /** The join key back to the change that caused this. */
  cascade_id?: string;
}

interface Listing {
  revocations: UnconfirmedRevocation[];
  summary: { queued: number; spent: number; oldest_age_seconds: number };
  escalated: boolean;
}

type SortKey = "who" | "what" | "where" | "when";
type SectionKey = "spent" | "queued";

/**
 * Both sections always, in this order, whatever either holds. They used to render only when
 * non-empty, so a revocation being given up on INSERTED a red card above the queue somebody was
 * reading: the list moved under them because the data changed, which is the one thing the structure
 * is not allowed to do. A section at zero is also the answer to a real question, "is anything
 * stuck?", and reads better as a hollow zero than as a card that is not there.
 */
const SECTIONS: Record<SectionKey, { id: string; label: string; hint: string; tone?: "danger"; empty: string }> = {
  spent: {
    id: "given-up",
    label: "Given up",
    hint: "They still have it until somebody ends it",
    tone: "danger",
    empty: "Nothing has been given up on. Every revocation still has a way through.",
  },
  queued: {
    id: "on-the-way",
    label: "Still on the way",
    hint: "Syndra keeps trying by itself",
    empty: "Nothing is on the way. Every revocation has gone through or been given up on.",
  },
};

/** Past this a revocation still waiting turns amber (route-revocations). */
const AMBER_AFTER_SECONDS = 30 * 60;

export function WithdrawnAccess() {
  const query = useQuery({
    queryKey: ["governance", "unconfirmed-revocations"],
    queryFn: () => request<Listing>("/governance/unconfirmed-revocations"),
    refetchInterval: 60_000,
  });
  const resolver = useNameResolver();

  const rows = useMemo(() => query.data?.revocations ?? [], [query.data]);
  const [search, setSearch] = useState("");
  const [where, setWhere] = useState("");
  // Most recent first, as the board draws it: `when` is how long ago, so ascending is newest.
  const [sort, flipSort, view] = useSort<SortKey>("when");
  const [open, setOpen] = useState<Record<SectionKey, boolean>>({ spent: true, queued: true });

  const text = useMemo(
    () => ({
      who: (row: UnconfirmedRevocation) => resolver.resolveUser(row.user_id).value?.display_name ?? "",
      what: (row: UnconfirmedRevocation) => whatText(row, resolver.resolveProject(row.project_id).value?.name ?? ""),
      where: (row: UnconfirmedRevocation) => targetLabel(row.target),
      when: (row: UnconfirmedRevocation) => row.age_seconds,
    }),
    [resolver],
  );
  const needle = search.trim().toLowerCase();
  const filtered = needle !== "" || where !== "";
  const matched = useMemo(
    () =>
      orderBy(rows, view.sort, text).filter(
        (row) =>
          (!where || row.target === where) &&
          (!needle || [text.who(row), text.what(row), text.where(row)].some((part) => part.toLowerCase().includes(needle))),
      ),
    [rows, view.sort, text, where, needle],
  );
  // Only systems with a row, so nothing on offer returns nothing.
  const systems = useMemo(() => Array.from(new Set(rows.map((row) => row.target))).sort(), [rows]);

  const spent = matched.filter((row) => row.spent);
  const queued = matched.filter((row) => !row.spent);
  // S5: a revocation that clears on a later read (it reached the system, or somebody ended it)
  // fades where it was and closes its gap after, and not under the pointer.
  const leaving = useLeavingRows(matched, (row) => row.id, !query.isLoading && !query.error, {
    narrowedBy: `${where}|${needle}`,
  });
  // The meta line counts everything outstanding, from the endpoint's own totals:
  // the list is a 200-row page, and a search narrows what is shown, not what is outstanding.
  const givenUp = query.data?.summary.spent ?? 0;
  const onTheWay = query.data?.summary.queued ?? 0;
  // Only a whole list can say how many people: a capped page would undercount them.
  const people = rows.length >= givenUp + onTheWay ? new Set(rows.map((row) => row.user_id)).size : null;
  const sections = (["spent", "queued"] as const).map((key) => ({
    key,
    ...SECTIONS[key],
    count: (key === "spent" ? spent : queued).length,
  }));
  const openSection = (section: { key: SectionKey }) => setOpen((cur) => ({ ...cur, [section.key]: true }));
  const showRows = !query.isLoading && !query.error && rows.length > 0;
  const readAt = useMemo(
    () => (query.dataUpdatedAt ? new Date(query.dataUpdatedAt).toISOString() : null),
    [query.dataUpdatedAt],
  );

  return (
    <div className="flex flex-col gap-4">
      {/* The intro keeps 12px of its own below it, so the first card never reads as attached to it. */}
      <div className="pb-3">
        <PageHeader
          title="Unfinished revocations"
          meta={
            showRows ? (
              <MetaRow>
                {[
                  `${givenUp} given up`,
                  `${onTheWay} on the way`,
                  ...(people === null ? [] : [`${people} ${people === 1 ? "person" : "people"}`]),
                ]}
              </MetaRow>
            ) : undefined
          }
          lede="Access somebody revoked that has not gone away yet. Revocations still on the way clear themselves. The ones Syndra has given up on do not, and those people still have the access until somebody ends it."
        />
      </div>

      <div className="flex flex-col gap-3.5">
        {showRows && (
          <FilterBar
            search={
              <Input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                placeholder="Search person or role…"
                aria-label="Search unfinished revocations by person or role"
              />
            }
            filters={
              <FilterPanel
                resultCount={matched.length}
                noun={["revocation", "revocations"]}
                activeCount={where ? 1 : 0}
                onClear={() => setWhere("")}
              >
                <FilterField label="Where">
                  <Select value={where} onChange={(event) => setWhere(event.target.value)} aria-label="Filter by system">
                    <option value="">Every system</option>
                    {systems.map((target) => (
                      <option key={target} value={target}>
                        {targetLabel(target)}
                      </option>
                    ))}
                  </Select>
                </FilterField>
              </FilterPanel>
            }
          />
        )}

        {/* How current the list is on the left, a chip per section at the right end. Not while the
            read is out. */}
        {showRows && (
          <ReadLine sections={sections} label="Sections of unfinished revocations" onOpen={openSection} className="-mt-1">
            <ReadFreshness
              state={{ readAt, current: true }}
              subject="Syndra’s send queue"
              onRefresh={() => query.refetch()}
            />
          </ReadLine>
        )}

        {showRows ? (
          <div className="flex flex-col gap-4" {...leaving.listProps}>
            <SectionPill sections={sections} label="Sections of unfinished revocations" onOpen={openSection} />
            {/* The two sections are the page's cards: they arrive on first paint (S11). */}
            <ArriveList>
              {(["spent", "queued"] as const).map((key) => (
                <RevocationSection
                  key={key}
                  {...SECTIONS[key]}
                  rows={leaving.rows.filter((row) => row.spent === (key === "spent"))}
                  count={(key === "spent" ? spent : queued).length}
                  leave={leaving.props}
                  view={view}
                  // A search that finds nothing here does not say nothing has been given up on.
                  empty={filtered ? "Nothing here matches." : SECTIONS[key].empty}
                  open={open[key]}
                  onToggle={() => setOpen((cur) => ({ ...cur, [key]: !cur[key] }))}
                  sort={sort}
                  onSort={flipSort}
                />
              ))}
            </ArriveList>
          </div>
        ) : (
          <Card>
            <ListStates
              isLoading={query.isLoading}
              error={query.error}
              isEmpty
              onRetry={() => query.refetch()}
              errorTitle="Couldn't load unfinished revocations."
              skeleton={<RowSkeleton rows={4} label="Loading unfinished revocations" />}
              empty={
                <EmptyState
                  title="Nothing outstanding"
                  guidance="Every revocation has reached the system it was sent to."
                  resolved
                />
              }
            >
              {null}
            </ListStates>
          </Card>
        )}
      </div>
    </div>
  );
}

/** What a row revokes as a person would search for it: project, role keys, or an add-on's reason. */
function whatText(row: UnconfirmedRevocation, project: string) {
  if (!row.project_id) return row.reason ?? `Access on ${targetLabel(row.target)}`;
  return `${project} ${row.role_keys.map(humanizeKey).join(", ")} ${row.role_keys.join(" ")}`.trim();
}

function RevocationSection({
  id,
  label,
  hint,
  tone,
  empty,
  rows,
  count,
  leave,
  view,
  open,
  onToggle,
  sort,
  onSort,
}: {
  id: string;
  label: string;
  hint: string;
  tone?: "danger";
  empty: string;
  rows: UnconfirmedRevocation[];
  /** The live rows: the ones going are still drawn, but not counted. */
  count: number;
  leave: (key: string) => LeaveProps;
  view: { sort: Sort<SortKey>; className: string };
  open: boolean;
  onToggle: () => void;
  sort: Sort<SortKey>;
  onSort: (key: SortKey) => void;
}) {
  const paging = usePaging();
  const resolver = useNameResolver();
  const anchor = useRef<HTMLDivElement>(null);
  const shown = rows.slice(0, paging.limit);
  // What and Where gather equal values under a counted label; Who and Revoked do not.
  const runAt = runStarts(
    rows,
    view.sort.key === "what"
      ? (row) => whatText(row, resolver.resolveProject(row.project_id).value?.name ?? "")
      : view.sort.key === "where"
        ? (row) => targetLabel(row.target)
        : null,
  );
  return (
    <section id={id} aria-labelledby={`${id}-title`} className="scroll-mt-4">
      <Card>
        <div ref={anchor}>
          <CardHeader
            tone={tone}
            count={count}
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
                  className={`flex size-7 flex-none items-center justify-center rounded-pill border border-line text-[13px] text-muted motion-turn ${
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
            {rows.length === 0 ? (
              <div className="row-divider flex items-center gap-2.5 px-5 py-4 text-[14px] text-muted">
                <StatusDot tone="healthy" />
                {empty}
              </div>
            ) : (
              <>
                <CardColumns>
                  <span className="w-[190px] flex-none">
                    <SortHead label="Who" name="who" sort={sort} onSort={onSort} />
                  </span>
                  <span className="min-w-0 flex-1">
                    <SortHead label="What was revoked" name="what" sort={sort} onSort={onSort} />
                  </span>
                  <span className="hidden w-[130px] flex-none desktop:block">
                    <SortHead label="Where" name="where" sort={sort} onSort={onSort} />
                  </span>
                  <span className="w-[110px] flex-none text-right">
                    <SortHead label="Revoked" name="when" sort={sort} onSort={onSort} />
                  </span>
                  <span className="hidden w-[240px] flex-none desktop:block">
                    <span className="sr-only">What happens next</span>
                  </span>
                </CardColumns>
                <div className={view.className}>
                  {shown.map((row, index) => {
                    const run = runAt(index);
                    return (
                      <LeavingRow key={row.id} {...leave(row.id)}>
                        {run && <RunLabel {...run} />}
                        <Row row={row} />
                      </LeavingRow>
                    );
                  })}
                </div>
                <PaginationRow
                  shown={shown.length}
                  total={rows.length}
                  onMore={paging.more}
                  onAll={() => paging.all(rows.length)}
                  onBack={paging.back}
                  anchor={anchor}
                />
              </>
            )}
          </div>
        </Unfold>
      </Card>
    </section>
  );
}

function Row({ row }: { row: UnconfirmedRevocation }) {
  const resolver = useNameResolver();
  const accountHref = useAccountHref();
  const first = resolver.resolveUser(row.user_id).value?.display_name?.split(" ")[0];
  const where = targetLabel(row.target);
  return (
    <div className="row-divider flex min-h-[60px] flex-col items-start gap-2 px-5 py-3.5 tablet:flex-row tablet:flex-wrap tablet:items-start tablet:gap-[18px]">
      <span className="flex w-full items-center gap-3 tablet:w-[190px] tablet:shrink-0">
        <UserAvatar id={row.user_id} />
        <span className="min-w-0">
          <Link href={accountHref(row.user_id)} className="text-[15px] font-semibold link-hover">
            <UserName id={row.user_id} />
          </Link>
          <ServiceAccountTag id={row.user_id} />
        </span>
      </span>

      <span className="flex w-full min-w-0 flex-col gap-[3px] pt-0.5 tablet:flex-1">
        <span className="text-[14.5px] leading-[1.45]">
          {row.project_id ? (
            <>
              <ProjectName id={row.project_id} />
              {row.role_keys.length > 0 && " / "}
              {row.role_keys.map(humanizeKey).join(", ")}
            </>
          ) : (
            (row.reason ?? `Access on ${where}`)
          )}
        </span>
        <span className="flex flex-wrap gap-x-3 gap-y-1 text-[12.5px] text-faint">
          {row.role_keys.map((key) => (
            <Mono key={key}>{key}</Mono>
          ))}
          {row.cascade_id && (
            // The join key, as a link, exactly as it appears in the audit log and on a person's
            // activity. A finding an operator cannot trace back to the change that caused it is a
            // mystery.
            <Link
              href={`/operations/cascades?cascade=${encodeURIComponent(row.cascade_id)}`}
              aria-label="See the edit that set off this revocation, in Change history"
              className="link-hover"
            >
              <Mono>{shortId(row.cascade_id, "c")}</Mono>
            </Link>
          )}
          <span className="desktop:hidden">· {where}</span>
        </span>
        {row.last_error && (
          // The reason, not a status. A given-up row an operator can see and not act on is the
          // whole difference between a finding and a mystery.
          <span className="mt-1 flex items-baseline gap-2 text-[13.5px] leading-[1.5] text-danger-text">
            <StatusDot tone="danger" />
            {row.last_error}
          </span>
        )}
      </span>

      <span className="hidden w-[130px] flex-none pt-0.5 text-[14px] text-muted desktop:block">{where}</span>

      <span className="flex flex-col gap-0.5 pt-0.5 tablet:w-[110px] tablet:shrink-0 tablet:text-right">
        <span
          className={`text-[13.5px] font-semibold ${
            row.spent ? "text-danger-text" : row.age_seconds >= AMBER_AFTER_SECONDS ? "text-warn-text" : "text-muted"
          }`}
        >
          <Relative iso={row.created_at} />
        </span>
        <span className="text-[12.5px] text-faint">{formatShortDate(row.created_at)}</span>
      </span>

      <span className="flex w-full flex-none justify-end pt-0.5 desktop:w-[240px]">
        {row.spent ? (
          // The way forward is the person's own page, where Syndra revokes it again.
          <ArrowLink href={`/users/${row.user_id}`}>
            Revoke again on {first ? `${first}’s` : "their"} page
          </ArrowLink>
        ) : (
          // The API carries how many tries so far, not the limit or the next time, so neither is claimed (A-07 §21).
          <span className="text-right text-[13px] text-muted">
            {row.attempts === 0
              ? "Not tried yet"
              : `Tried ${row.attempts} ${row.attempts === 1 ? "time" : "times"}`}
            {" · Tries again by itself"}
          </span>
        )}
      </span>
    </div>
  );
}
