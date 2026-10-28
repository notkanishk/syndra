"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useRef, useState, type ReactNode } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { useDegraded } from "@/components/states/DegradedBanner";
import { useAccountHref } from "@/components/system/ServiceAccountTag";
import { isDeparted } from "@/lib/people-filters";
import { serviceAccountHref } from "@/lib/queries/useServiceAccounts";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Badge, Mono, StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader } from "@/components/ui/Card";
import { FilterBar } from "@/components/ui/FilterBar";
import { Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { PageHeader } from "@/components/ui/PageHeader";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { ReadFreshness, useRefreshPress } from "@/components/ui/ReadFreshness";
import { FilterPills, Select } from "@/components/ui/Select";
import { ReadLine, SectionPill } from "@/components/ui/SectionNav";
import { LeavingRow, useLeavingRows, type LeaveProps } from "@/components/ui/LeavingRows";
import { RunLabel, SortHead, runStarts, useSort } from "@/components/ui/SortHead";
import { Unfold } from "@/components/ui/Unfold";
import { BULK_MAX_USERS } from "@/lib/queries/useBulkGrants";
import { useGovernanceSummary } from "@/lib/queries/useGovernance";
import { useReconciliationDiff } from "@/lib/queries/useGrants";
import { useProjects } from "@/lib/queries/useProjects";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import {
  useAttributeDrift,
  useBulkAttributeDrift,
  useBulkMarkExternalDrift,
  useDriftItems,
  useDriftOrigin,
  useMarkExternalDrift,
  useReconcileNow,
  useRehearseAdoptDrift,
  useRehearseMarkExternalDrift,
  useRevokeDrift,
  type DriftTriageItem,
} from "@/lib/queries/useDrift";
import {
  RowCheckbox,
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection, type RowSelection } from "@/lib/useRowSelection";
import { Relative } from "@/components/ui/Time";
import { formatShortDate, formatRelative, formatWhen, humanizeKey, whyNotChecked } from "@/lib/format";
import {
  AGE_LABELS,
  EMPTY_DRIFT_FILTERS,
  ORIGIN_LABELS,
  REASON_LABELS,
  applyDriftFilters,
  driftReason,
  driftRequest,
  hasAnyDriftFilter,
  matchesDriftSearch,
  parseDriftFilters,
  serializeDriftFilters,
  sortDrift,
  type DriftFilters,
  type DriftSortKey,
} from "@/lib/drift-filters";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { FilterPanel, FilterField } from "@/components/ui/FilterPanel";
import { targetLabel } from "@/lib/nav";
import {
  outcomeFromError,
  succeeded,
  type ActionOutcome as Outcome,
} from "@/lib/outcome";
import dynamic from "next/dynamic";

// Opened on demand, so it stays out of the page's first load.
const RehearsalDialog = dynamic(() => import("@/components/ui/RehearsalDialog").then((m) => m.RehearsalDialog));

type Resolution = "attribute" | "revoke" | "external";

/**
 * The two directions drift runs in. A grouped list folds (RULES): each is a
 * card of its own, open by default, with its own count and paging.
 */
const SECTIONS = [
  {
    kind: "target_only",
    id: "sec-z",
    title: "In Zitadel, not given by Syndra",
    label: "Not given by Syndra",
    hint: "They keep it until you decide",
    tone: "danger",
  },
  {
    kind: "syndra_only",
    id: "sec-s",
    title: "Given by Syndra, gone from Zitadel",
    label: "Gone from Zitadel",
    hint: "They no longer have it",
    tone: "warn",
  },
] as const satisfies ReadonlyArray<{
  kind: string;
  id: string;
  title: string;
  label: string;
  hint: string;
  tone: "danger" | "warn";
}>;
type SectionKind = (typeof SECTIONS)[number]["kind"];

/** A quiet text control that sits among a row's buttons. */
const QUIET_LINK =
  "inline-flex min-h-[44px] items-center text-[12.5px] font-semibold text-muted link-tint desktop:min-h-6";

const isSafety = (item: DriftTriageItem) => (item.role_group ?? "").toLowerCase().includes("safety");

/**
 * S6 · Review › Drift — the highest-stakes screen in the product.
 *
 * Every row has to answer two questions in about two seconds: what is this, and
 * what happens if I revoke it. So the evidence sits ON the row rather than
 * behind a click, and the three resolutions are always in the same order —
 * Adopt, Revoke, Owned elsewhere — so a scanning eye learns one sequence.
 *
 * The list is gathered by direction and starts ordered by when it was found
 * (most recent on top); every heading sorts. The row LAYOUT never changes with
 * risk; only a left border does, on every safety-gated row. A queue whose rows
 * rearrange their own shape is a queue nobody can scan.
 */
export function UnexplainedAccess() {
  const params = useSearchParams();
  const router = useRouter();

  /**
   * Six filters, in the URL, so a narrowed queue is a link somebody can send.
   *
   * Three are request parameters (project, person, how it was found); three are
   * applied here over what came back, because they ask about fields the
   * endpoint does not filter on. `drift-filters.ts` says which is which.
   *
   * The person and role choices are built from the rows actually in the queue
   * rather than from the whole directory: there is no use offering three
   * hundred names when nine of them have drift, and a filter that can return
   * nothing is a filter that wastes a click.
   */
  const filters = useMemo(
    () => parseDriftFilters(new URLSearchParams(params.toString())),
    [params],
  );
  const setFilters = useCallback(
    (next: Partial<DriftFilters>) => {
      router.replace(`/governance/drift${serializeDriftFilters({ ...filters, ...next })}`, {
        scroll: false,
      });
    },
    [filters, router],
  );
  const drift = useDriftItems(driftRequest(filters));
  const summary = useGovernanceSummary();
  // Zitadel not answering: what is on screen is an old read, and a decision
  // made on it can be wrong. UI only: the buttons wait, the requests are
  // unchanged, and the one Notice above the page says why.
  const paused = useDegraded();
  const projects = useProjects();
  // Search is not a filter in the URL: it finds a row, it does not make a view.
  const [query, setQuery] = useState("");
  const [sort, flipSort, view] = useSort<DriftSortKey>("found");
  const [open, setOpen] = useState<Record<SectionKind, boolean>>({
    target_only: true,
    syndra_only: true,
  });
  // A folded section opens on the way to it (the chips and the pill call this).
  const openSection = useCallback(
    (section: { kind: SectionKind }) => setOpen((cur) => ({ ...cur, [section.kind]: true })),
    [],
  );
  const filtered = hasAnyDriftFilter(filters) || query.trim() !== "";
  // On the button, so "why am I seeing so few rows" never needs the panel
  // opened to answer it — that question is the one a hidden filter creates.
  const activeFilterCount = Object.values(filters).filter(Boolean).length;
  const reconcile = useReconcileNow();
  const [scanOutcome, setScanOutcome] = useState<Outcome | null>(null);

  const [pending, setPending] = useState<{ item: DriftTriageItem; resolution: Resolution } | null>(
    null,
  );
  const [expanded, setExpanded] = useState<string | null>(null);
  const [bulkOp, setBulkOp] = useState<"adopt" | "external" | null>(null);

  // What the request returned, before this page's own filters — the person and
  // role choices are drawn from here, so every option offered matches at least
  // one row.
  const returned = useMemo(() => drift.data ?? [], [drift.data]);
  const resolver = useNameResolver();
  const names = useMemo(() => {
    const who = (item: DriftTriageItem) => resolver.resolveUser(item.user_id).value?.display_name ?? "";
    const what = (item: DriftTriageItem) =>
      `${resolver.resolveProject(item.project_id).value?.name ?? ""} · ${humanizeKey(item.role_keys[0] ?? "")}`;
    return { who, what };
  }, [resolver]);
  const items = useMemo(
    () =>
      applyDriftFilters(returned, filters).filter((item) =>
        matchesDriftSearch(item, query, { who: names.who(item), what: names.what(item) }),
      ),
    [returned, filters, query, names],
  );
  // Options drawn from the queue itself, so nothing on offer returns nothing —
  // and each list is built with its OWN filter lifted, or choosing a name would
  // leave the name list holding only that name.
  const people = useMemo(() => {
    const byId = new Map<string, string>();
    for (const item of applyDriftFilters(returned, filters, new Date(), "user")) {
      if (byId.has(item.user_id)) continue;
      byId.set(item.user_id, resolver.resolveUser(item.user_id).value?.display_name ?? item.user_id);
    }
    return Array.from(byId, ([id, name]) => ({ id, name })).sort((a, b) =>
      a.name.localeCompare(b.name),
    );
  }, [returned, filters, resolver]);
  const roleKeys = useMemo(
    () =>
      Array.from(
        new Set(
          applyDriftFilters(returned, filters, new Date(), "role").flatMap((item) => item.role_keys),
        ),
      ).sort(),
    [returned, filters],
  );

  /**
   * What the comparison shows that no finding carries.
   *
   * The findings are what the scheduled check raised; the comparison is a read
   * of both sides now. A role Zitadel holds that Syndra does not explain, or
   * one Syndra gave that Zitadel lacks, can be in the comparison and not yet a
   * finding — and a person and project with roles different on both sides puts
   * each role on the side that holds it. Those rows have nothing to resolve (no
   * finding to adopt or revoke), so they are read-only, and they are dropped
   * where a finding already says the same thing. The origin, age and source
   * filters ask about fields a comparison row does not have, so they hide
   * these rather than claim a match.
   */
  const diff = useReconciliationDiff();
  const readRows = useMemo(() => {
    const read = diff.data;
    if (!read || filters.origin || filters.age || filters.source) return [];
    const said = new Set(
      returned
        .filter((item) => item.target === "zitadel")
        .flatMap((item) =>
          item.role_keys.map((r) => `${item.drift_type}|${item.user_id}|${item.project_id}|${r}`),
        ),
    );
    const rows: ReadRow[] = [];
    const add = (kind: SectionKind, userId: string, projectId: string, roles: string[], both: boolean) => {
      for (const role of roles) {
        const key = `${kind}|${userId}|${projectId}|${role}`;
        if (said.has(key)) continue;
        said.add(key);
        if (filters.user && userId !== filters.user) continue;
        if (filters.project && projectId !== filters.project) continue;
        if (filters.role && role !== filters.role) continue;
        const row = { key, kind, userId, projectId, role, both };
        const text = {
          who: resolver.resolveUser(userId).value?.display_name ?? "",
          what: `${resolver.resolveProject(projectId).value?.name ?? ""} · ${humanizeKey(role)}`,
        };
        if (matchesDriftSearch({ role_keys: [role] }, query, text)) rows.push(row);
      }
    };
    for (const g of read.only_in_zitadel) add("target_only", g.user_id, g.project_id, g.role_keys, false);
    for (const g of read.only_in_syndra) add("syndra_only", g.user_id, g.project_id, g.role_keys, false);
    for (const g of read.drift) {
      add("target_only", g.user_id, g.project_id, g.only_in_zitadel, true);
      add("syndra_only", g.user_id, g.project_id, g.only_in_syndra, true);
    }
    return rows;
  }, [diff.data, returned, filters, query, resolver]);

  const sorted = useMemo(
    () => sortDrift(items, view.sort.key, view.sort.dir, names),
    [items, view.sort, names],
  );
  // S5: a finding that goes on a later read (somebody adopted it elsewhere, Zitadel changed)
  // fades where it was and closes its gap after, and not under the pointer. Counts and
  // selection stay on the live findings; only what is drawn keeps the ones that are going.
  const leaving = useLeavingRows(sorted, (item) => item.id, !drift.isLoading && !drift.error, {
    narrowedBy: `${query}|${JSON.stringify(filters)}`,
  });
  const sections = useMemo(() => {
    return SECTIONS.map((section) => {
      const rows = leaving.rows.filter((item) => item.drift_type === section.kind);
      const read = readRows.filter((row) => row.kind === section.kind);
      const live = sorted.filter((item) => item.drift_type === section.kind).length;
      return { ...section, items: rows, read, count: live + read.length };
    }).filter((section) => section.items.length + section.read.length > 0);
  }, [leaving.rows, sorted, readRows]);
  // The order the rows are drawn in, so shift-click ranges mean what they look
  // like. Selection spans the whole queue, not what a section has rendered: a
  // triage backlog is exactly the case where paging four times before you can
  // act is the tedium worth removing.
  const selection = useRowSelection(
    useMemo(() => {
      const live = new Set(sorted.map((item) => item.id));
      return sections.flatMap((section) =>
        section.items.filter((item) => live.has(item.id)).map((item) => item.id),
      );
    }, [sections, sorted]),
  );
  // A triage row is read before it is resolved, so the seats arrive behind a
  // named control rather than in front of every row by default.
  const [selecting, setSelecting] = useState(false);
  const oldest = items.reduce<string | null>(
    (acc, item) => (!acc || item.detected_at < acc ? item.detected_at : acc),
    null,
  );
  const readAt = useMemo(
    () => (drift.dataUpdatedAt ? new Date(drift.dataUpdatedAt).toISOString() : null),
    [drift.dataUpdatedAt],
  );
  const refreshed = useRefreshPress(readAt);
  const unread = summary.data?.unreconciled_targets ?? [];

  const selectedIds = useMemo(() => Array.from(selection.selected), [selection.selected]);

  /**
   * Drift arrives in clusters — one misconfigured rule, one person onboarded by
   * hand, one project nobody told Syndra about. Selecting the cluster is the
   * actual shape of the work, and no amount of shift-clicking finds it as
   * reliably as asking for it.
   */
  const selectSimilar = useCallback(
    (item: DriftTriageItem) => {
      const kin = items.filter(
        (candidate) =>
          candidate.user_id === item.user_id || candidate.project_id === item.project_id,
      );
      selection.selectOnly(kin.map((candidate) => candidate.id));
    },
    [items, selection],
  );

  /**
   * What the selection is made of, not just how much of it there is.
   *
   * Safety-gated access is what an operator needs to know before resolving
   * twelve rows at once — a batch of twelve wiki roles and a batch containing
   * three laser-cutter roles are not the same decision.
   */
  const composition = useMemo(() => {
    if (selection.count === 0) return "";
    const chosen = items.filter((item) => selection.selected.has(item.id));
    const safety = chosen.filter(isSafety).length;
    const people = new Set(chosen.map((item) => item.user_id)).size;
    const parts = [`${people} ${people === 1 ? "person" : "people"}`];
    if (safety > 0) parts.unshift(`${safety} safety-gated`);
    return parts.join(" · ");
  }, [items, selection]);

  const head = (
    <CardColumns>
      {selecting && <span className="w-11 shrink-0 desktop:w-[26px]" />}
      <span className="w-[180px] shrink-0">
        <SortHead label="Who" name="who" sort={sort} onSort={flipSort} />
      </span>
      <span className="w-[200px] shrink-0">
        <SortHead label="What they can get into" name="what" by="what" sort={sort} onSort={flipSort} />
      </span>
      <span className="flex-1">
        <SortHead label="Why Syndra can’t explain it" name="why" by="why" sort={sort} onSort={flipSort} />
      </span>
      <span className="w-[110px] shrink-0 text-right">
        <SortHead label="Found" name="found" sort={sort} onSort={flipSort} />
      </span>
    </CardColumns>
  );

  // "Everything is explained" is a claim about every read, not only the findings: it waits for
  // the comparison and the system list to settle, and says what was not read instead of
  // vouching for it.
  const notRead = [
    ...unread.map((target) => `${targetLabel(target.target)} wasn’t read`),
    ...(summary.error ? ["Which systems were read isn’t known"] : []),
    ...(diff.error ? ["The comparison didn’t load"] : []),
    ...(diff.data?.truncated ? ["The comparison stopped early"] : []),
  ];

  const list = (
    <ListStates
      // Rows from the last filters are not this read: nothing here is "explained" until it lands.
      isLoading={
        drift.isLoading ||
        (sections.length === 0 && (drift.isPlaceholderData || !!(diff.isLoading || summary.isLoading)))
      }
      error={drift.error}
      isEmpty={sections.length === 0}
      onRetry={() => drift.refetch()}
      errorTitle="Couldn't load drift."
      skeleton={
        <Card>
          <RowSkeleton rows={4} label="Loading drift" />
        </Card>
      }
      empty={
        <Card>
          {/* "Nothing here" and "nothing here that matches" are different
              answers, and on this queue the difference is whether there is
              drift somewhere else that nobody is looking at. */}
          {filtered ? (
            <EmptyState
              title="Nothing unexplained matches those filters."
              guidance="There may still be drift under another project, or found the other way."
              action={{
                label: "Clear filters",
                onClick: () => {
                  setQuery("");
                  setFilters(EMPTY_DRIFT_FILTERS);
                },
              }}
            />
          ) : notRead.length > 0 ? (
            <EmptyState
              title="Nothing is waiting in what was read."
              guidance={`${notRead.join(". ")}. Anything that drifted there is not in this list.`}
            />
          ) : (
            <EmptyState
              title="Everything is explained."
              guidance="Every piece of access in Zitadel traces back to something Syndra did, and everything Syndra gave is there."
              resolved
            />
          )}
        </Card>
      }
    >
      <div className="flex flex-col gap-4">
        <SectionPill sections={sections} label="Sections of drift" onOpen={openSection} />
        {selecting && (
          <Card>
            <SelectAllRow
              inScope={items.length}
              noun={["item", "items"]}
              allSelected={selection.allSelected}
              {...selection.headerCheckboxProps}
            />
          </Card>
        )}
        <div data-selection-scope {...selection.containerProps} {...leaving.listProps} className="flex flex-col gap-4">
          {sections.map((section) => (
            <DriftSection
              key={section.kind}
              section={section}
              comparedAt={diff.data?.generated_at ?? ""}
              open={open[section.kind]}
              onToggle={() => setOpen((cur) => ({ ...cur, [section.kind]: !cur[section.kind] }))}
              head={head}
              leave={leaving.props}
              fade={view.className}
              runOf={
                view.sort.key === "what"
                  ? names.what
                  : view.sort.key === "why"
                    ? (item) => REASON_LABELS[driftReason(item)]
                    : null
              }
              renderRow={(item) => (
                <TriageRow
                  key={item.id}
                  selecting={selecting}
                  item={item}
                  selection={selection}
                  // Selecting is what this does, so it turns the mode on rather
                  // than filling a selection nothing on screen is showing.
                  onSelectSimilar={() => {
                    setSelecting(true);
                    selectSimilar(item);
                  }}
                  expanded={expanded === item.id}
                  onExpand={() => setExpanded((cur) => (cur === item.id ? null : item.id))}
                  onResolve={(resolution) => setPending({ item, resolution })}
                  paused={paused}
                />
              )}
            />
          ))}
        </div>
        {/* Agreement is rendered, not filtered out ("nothing wrong with the
            rest" is the answer somebody came for) — but only from a comparison
            that finished. A cut-off or failed one cannot say it. */}
        {diff.data && !diff.data.truncated && (
          <span className="inline-flex items-center gap-2.5 px-1 text-[13.5px] text-muted">
            <StatusDot tone="healthy" />
            Everything else in Zitadel agrees with Syndra.
          </span>
        )}
      </div>
    </ListStates>
  );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Drift"
        lede="Access in Zitadel that nothing in Syndra gave, and access Syndra gave that Zitadel no longer holds. Adopt keeps it and lets Syndra look after it; Revoke ends it; Owned elsewhere means another system manages it and Syndra stops asking."
        meta={
          items.length > 0
            ? `${items.length} ${filtered ? "matching these filters" : "to decide"}${
                oldest ? ` · oldest found ${formatRelative(oldest)}` : ""
              }`
            : undefined
        }
        actions={
          <Button
            isPending={reconcile.isPending || refreshed.pressing}
            disabled={paused}
            // The same press the freshness line below answers (S17): busy, the old age
            // stays until the read lands, then "just now" crossfades in and the dot blooms.
            onClick={() =>
              refreshed.press(async () => {
                setScanOutcome(null);
                try {
                  await reconcile.mutateAsync();
                  // The comparison is a second read of the same two sides.
                  void diff.refetch();
                  // The invalidation has already started the new read: join it, never restart it.
                  await drift.refetch({ cancelRefetch: false });
                  // A scan is a read, so it applies nothing — saying "applied"
                  // about a comparison would be the queue claiming it had
                  // acted on what it found.
                  setScanOutcome({
                    kind: "no_change",
                    message: "Checked again",
                    detail: "The list below is what the check found just now.",
                  });
                } catch (error) {
                  setScanOutcome(outcomeFromError(error));
                }
              })
            }
          >
            Check Zitadel again
          </Button>
        }
      />

      {/* Under the control that ran it, above the queue it describes. */}
      {scanOutcome && <ActionOutcome outcome={scanOutcome} />}

      {/* The board's sentence for the paused controls, worded as A-01 (14 Oct)
          has it: Syndra's own stops say they wait, never that they are paused.
          The seat is Unfold's, so the list below does not jump when it comes. */}
      <Unfold open={paused}>
        <p className="text-[13.5px] text-muted" role="status">
          Adopting, revoking and marking as owned elsewhere wait until Zitadel answers.
        </p>
      </Unfold>

      <FilterBar
        search={
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search person or role…"
            aria-label="Search drift by person or role"
          />
        }
        filters={
          <FilterPanel
            resultCount={items.length}
            noun={["finding", "findings"]}
            activeCount={activeFilterCount}
            onClear={() => setFilters(EMPTY_DRIFT_FILTERS)}
          >
            <FilterField label="Project">
              <Select
                value={filters.project}
                onChange={(event) => setFilters({ project: event.target.value })}
                aria-label="Filter by project"
              >
                <option value="">All projects</option>
                {(projects.data ?? []).map((entry) => (
                  <option key={entry.project.id} value={entry.project.id}>
                    {entry.project.name}
                  </option>
                ))}
              </Select>
            </FilterField>
            <FilterField label="Person">
              <Select
                value={filters.user}
                onChange={(event) => setFilters({ user: event.target.value })}
                aria-label="Filter by person"
              >
                <option value="">Anyone</option>
                {people.map((person) => (
                  <option key={person.id} value={person.id}>
                    {person.name}
                  </option>
                ))}
              </Select>
            </FilterField>
            <FilterField label="Role">
              <Select
                value={filters.role}
                onChange={(event) => setFilters({ role: event.target.value })}
                aria-label="Filter by role"
              >
                <option value="">Any role</option>
                {roleKeys.map((key) => (
                  <option key={key} value={key}>
                    {humanizeKey(key)}
                  </option>
                ))}
              </Select>
            </FilterField>
            <FilterField label="What is known about it">
              <FilterPills
                label="Filter by what is known about where it came from"
                value={filters.origin}
                onChange={(value) => setFilters({ origin: value as DriftFilters["origin"] })}
                options={[
                  { value: "", label: "Any" },
                  ...Object.entries(ORIGIN_LABELS).map(([value, label]) => ({ value, label })),
                ]}
              />
            </FilterField>
            <FilterField label="How long it has waited">
              <FilterPills
                label="Filter by how long it has been waiting"
                value={filters.age}
                onChange={(value) => setFilters({ age: value as DriftFilters["age"] })}
                options={[
                  { value: "", label: "Any" },
                  ...Object.entries(AGE_LABELS).map(([value, label]) => ({ value, label })),
                ]}
              />
            </FilterField>
            <FilterField label="How it was found">
              <FilterPills
                label="Filter by how it was found"
                value={filters.source}
                onChange={(value) => setFilters({ source: value })}
                options={[
                  { value: "", label: "Any" },
                  { value: "webhook", label: "Caught as it happened" },
                  { value: "reconciliation_sweep", label: "By the scheduled check" },
                ]}
              />
            </FilterField>
          </FilterPanel>
        }
        trailing={
          items.length > 0 ? (
            <SelectModeToggle active={selecting} onToggle={() => setSelecting((on) => !on)} />
          ) : undefined
        }
      />

      {/* The read line: how current the list is on the left, a way to each
          section on the right. Only once there is a list to be current. */}
      {!drift.isLoading && !drift.error && (
        <ReadLine sections={sections} label="Sections of drift" onOpen={openSection} className="-mt-1">
          <ReadFreshness
            state={{ readAt, current: true }}
            subject="This list"
            onRefresh={() => drift.refetch()}
            refresh={refreshed}
          />
        </ReadLine>
      )}

      {/* A system nobody could read says what is missing from the list and why;
          the rows below are still current for the ones that were read. */}
      {unread.map((target) => (
        <Notice key={target.target} tone="warn" live title={`${targetLabel(target.target)} hasn’t been read since ${formatWhen(target.since)}.`}>
          Anything that drifted on it is not in this list.{" "}
          {target.reason ? `${whyNotChecked(target.reason)} ` : ""}The rows below are current for
          the systems that were read.
        </Notice>
      ))}

      <Unfold open={Boolean(diff.data?.truncated)} gap={16}>
        <Notice tone="warn" title="The comparison stopped early.">
          The list was too long, so what the comparison adds to the findings is incomplete. Treat a
          missing row as “nothing found so far”, not as “nothing exists”.
        </Notice>
      </Unfold>
      <Unfold open={Boolean(diff.error)} gap={16}>
        <Notice tone="warn" title="The comparison didn’t load.">
          The findings below are still current. Differences only the comparison shows are missing.
        </Notice>
      </Unfold>

      {list}

      <SelectionBar
        count={selecting ? selection.count : 0}
        noun={["item", "items"]}
        composition={composition}
        // `boundBulkIDs` refuses past this on both drift bulk routes. Same
        // server constant the grant and request endpoints use.
        ceiling={BULK_MAX_USERS}
        onTakeCeiling={() =>
          selection.selectOnly(
            sections.flatMap((section) => section.items).slice(0, BULK_MAX_USERS).map((item) => item.id),
          )
        }
        onClear={selection.clear}
      >
        {/* Revoke is missing from this bar on purpose, and its absence is
            stated where somebody would go looking for it. An operator who
            selects nine rows to revoke and finds two buttons that are not
            Revoke concludes the bar is broken, or hunts. */}
        <span className="text-[13px] text-faint">Revoking is one row at a time.</span>
        {/* Both open a plan; neither resolves anything on tap. */}
        <SelectionAction disabled={paused} onClick={() => setBulkOp("adopt")}>
          Preview adopting these
        </SelectionAction>
        <SelectionAction disabled={paused} onClick={() => setBulkOp("external")}>
          Preview marking these owned elsewhere
        </SelectionAction>
      </SelectionBar>

      <ResolutionDialog pending={pending} onClose={() => setPending(null)} />

      {bulkOp && (
        <BulkResolutionDialog
          op={bulkOp}
          ids={selectedIds}
          composition={composition}
          onClose={() => setBulkOp(null)}
          onApplied={selection.clear}
        />
      )}
    </div>
  );
}

/** A role the comparison shows on one side only, with no finding behind it. */
type ReadRow = {
  key: string;
  kind: SectionKind;
  userId: string;
  projectId: string;
  role: string;
  /** The person holds other roles on this project on both sides. */
  both: boolean;
};

type SectionView = {
  kind: SectionKind;
  id: string;
  title: string;
  label: string;
  hint: string;
  tone: "danger" | "warn";
  /** Findings: what can be decided. */
  items: DriftTriageItem[];
  /** The comparison's own rows, read-only. */
  read: ReadRow[];
  count: number;
};

/**
 * One direction of drift: a card whose heading folds it, ten rows to start,
 * and under a sort that gathers equal values a quiet label for each run.
 */
function DriftSection({
  section,
  open,
  onToggle,
  head,
  leave,
  fade,
  runOf,
  renderRow,
  comparedAt,
}: {
  section: SectionView;
  open: boolean;
  onToggle: () => void;
  comparedAt: string;
  head: ReactNode;
  leave: (key: string) => LeaveProps;
  /** The re-sort's fade, on the rows and not the headings. */
  fade: string;
  runOf: ((item: DriftTriageItem) => string) | null;
  renderRow: (item: DriftTriageItem) => ReactNode;
}) {
  const paging = usePaging();
  const anchor = useRef<HTMLDivElement>(null);
  const visible = section.items.slice(0, paging.limit);
  // The comparison's rows follow the findings and share their ten.
  const visibleRead = section.read.slice(0, Math.max(0, paging.limit - visible.length));
  const runAt = runStarts(section.items, runOf);
  return (
    <section id={section.id} aria-labelledby={`${section.id}-title`} className="scroll-mt-4">
      <Card>
        <div ref={anchor}>
          <CardHeader
            tone={section.tone}
            count={section.count}
            note={section.hint}
            title={
              <button
                type="button"
                aria-expanded={open}
                aria-controls={`${section.id}-body`}
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
                <span id={`${section.id}-title`}>{section.title}</span>
              </button>
            }
          />
        </div>
        <Unfold open={open}>
          <div id={`${section.id}-body`}>
            {head}
            <div className={fade}>
              {visible.map((item, index) => {
                const run = runAt(index);
                return (
                  <LeavingRow key={item.id} {...leave(item.id)}>
                    {run && <RunLabel {...run} />}
                    {renderRow(item)}
                  </LeavingRow>
                );
              })}
            </div>
            {visibleRead.map((row) => (
              <ComparedRow key={row.key} row={row} comparedAt={comparedAt} />
            ))}
            <PaginationRow
              shown={visible.length + visibleRead.length}
              total={section.count}
              onMore={paging.more}
              onAll={() => paging.all(section.count)}
              onBack={paging.back}
              anchor={anchor}
            />
          </div>
        </Unfold>
      </Card>
    </section>
  );
}

/**
 * A role the comparison shows on one side only, in the same seats as a finding
 * but with no answers: nothing has been raised for it, so there is nothing to
 * adopt, revoke or mark. Said in words so the missing buttons read as a fact
 * about the row rather than a fault.
 */
function ComparedRow({ row, comparedAt }: { row: ReadRow; comparedAt: string }) {
  const accountHref = useAccountHref();
  const inZitadel = row.kind === "target_only";
  const text = inZitadel
    ? row.both
      ? "Zitadel gives this person this role on this project and Syndra does not, while they hold other roles on it on both sides."
      : "Zitadel has this and nothing in Syndra explains it."
    : row.both
      ? "Syndra gave this role and Zitadel does not have it, while they hold other roles on this project on both sides."
      : "Syndra gave this and Zitadel does not have it, so the change never went through.";
  return (
    <div data-compared-row className="border-l-[3px] border-transparent">
      <div className="row-divider flex flex-wrap items-start gap-x-[18px] gap-y-3 px-5 py-4">
        <div className="flex w-full min-w-0 items-center gap-3 tablet:w-[180px] tablet:shrink-0">
          <UserAvatar id={row.userId} size="row" />
          <Link
            href={accountHref(row.userId)}
            className="text-[14.5px] font-semibold link-hover"
          >
            <UserName id={row.userId} />
          </Link>
        </div>
        <div className="flex w-full flex-col items-start gap-[3px] tablet:w-[200px] tablet:shrink-0">
          <Link
            href={`/projects/${row.projectId}`}
            className="text-[14px] leading-[1.45] link-hover"
          >
            <ProjectName id={row.projectId} /> · {humanizeKey(row.role)}
          </Link>
          <Mono className="text-[12.5px] text-faint">{row.role}</Mono>
        </div>
        <div className="order-last flex w-full min-w-0 flex-col gap-1 desktop:order-none desktop:w-auto desktop:flex-1">
          <p className="text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">{text}</p>
          <p className="text-[12.5px] leading-[1.5] text-faint">
            It is in the comparison and has not been raised as a finding, so there is nothing to
            resolve here yet.{" "}
            {!inZitadel && (
              <Link href="/governance/pending" className="font-semibold text-muted link-hover">
                Check Pending changes
              </Link>
            )}
          </p>
        </div>
        <div className="text-[13px] text-faint tablet:ml-auto tablet:w-[110px] tablet:shrink-0 tablet:pt-0.5 tablet:text-right">
          <span className="tablet:hidden">Compared </span>
          <Relative iso={comparedAt} />
        </div>
      </div>
    </div>
  );
}

function TriageRow({
  item,
  selecting,
  selection,
  onSelectSimilar,
  expanded,
  onExpand,
  onResolve,
  paused,
}: {
  item: DriftTriageItem;
  selecting: boolean;
  selection: RowSelection;
  onSelectSimilar: () => void;
  expanded: boolean;
  onExpand: () => void;
  onResolve: (resolution: Resolution) => void;
  /** Zitadel is not answering: the decisions wait. */
  paused: boolean;
}) {
  const role = item.role_keys[0] ?? "";
  // A machine account is not a person: an integration that provisions itself
  // on every deploy will re-create this tomorrow whatever Syndra records, so
  // adopting is the wrong verb and is neutralised rather than hidden.
  // Adopting records access Zitadel has. A grant Syndra records that Zitadel
  // doesn't (syndra_only) has nothing to adopt, and adopting it wrote a second,
  // permanent record of access nobody holds.
  const nothingToAdopt = item.drift_type === "syndra_only";
  const adoptPointless = item.user_is_service_account || nothingToAdopt;
  const why = explainDrift(item);

  return (
    <div
      {...(isSafety(item) ? { "data-danger-edge": "" } : {})}
      className={isSafety(item) ? "border-l-[3px] border-danger" : "border-l-[3px] border-transparent"}
    >
      <div
        className={`row-divider flex flex-wrap items-start gap-x-[18px] gap-y-3 px-5 py-4 ${
          selection.isSelected(item.id) ? "bg-accent-soft" : ""
        }`}
        {...selection.rowProps(item.id)}
      >
        {selecting && (
          <span className="order-1">
            <RowCheckbox
              label={`Select the ${role} row`}
              {...selection.checkboxProps(item.id)}
            />
          </span>
        )}

        {/* Who links to the person; the evidence has its own control. */}
        <div className="order-1 flex w-full min-w-0 items-center gap-3 tablet:w-[180px] tablet:shrink-0">
          <UserAvatar id={item.user_id} size="row" />
          <span className="min-w-0">
            <Link
              href={item.user_is_service_account ? serviceAccountHref(item.user_id) : `/users/${item.user_id}`}
              className="block text-[14.5px] font-semibold link-hover"
            >
              <UserName id={item.user_id} />
            </Link>
            <span className="block text-[12.5px] text-faint">{describeHolder(item)}</span>
          </span>
        </div>

        <div className="order-2 flex w-full flex-col items-start gap-[3px] tablet:w-[200px] tablet:shrink-0">
          <Link
            href={`/projects/${item.project_id}`}
            className="text-[14px] leading-[1.45] link-hover"
          >
            <ProjectName id={item.project_id} /> · {humanizeKey(role)}
          </Link>
          <Mono className="text-[12.5px] text-faint">{role}</Mono>
          {item.target !== "zitadel" && (
            <span className="text-[12.5px] text-muted">On {targetLabel(item.target)}</span>
          )}
          <RiskPill item={item} />
        </div>

        <div className="order-3 text-[13px] text-faint tablet:ml-auto tablet:w-[110px] tablet:shrink-0 tablet:pt-0.5 tablet:text-right desktop:order-4">
          <span className="tablet:hidden">Found </span>
          <Relative iso={item.detected_at} />
        </div>

        {/* The reason gets the width, and the answers sit beneath it in the
            same order on every row: Adopt · Revoke · Owned elsewhere. Revoke is
            a red OUTLINE here — the solid fill exists only on the confirming
            button inside its dialog. Below the desktop breakpoint this block
            drops under the line and takes the full width. */}
        <div className="order-4 flex w-full min-w-0 flex-col gap-2.5 desktop:order-3 desktop:w-auto desktop:flex-1">
          <div className="flex flex-col gap-1">
            <p className="text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">{why.text}</p>
            {/* Two separate facts, said separately (owner's requirement). */}
            {why.note && (
              <p className="text-[13.5px] leading-[1.55] text-warn-text [text-wrap:pretty]">{why.note}</p>
            )}
            <WhoMadeIt item={item} />
          </div>
          <div className="flex flex-wrap items-center gap-3 desktop:gap-2">
            <Button
              variant={adoptPointless ? "ghost" : "accent"}
              size="sm"
              disabled={adoptPointless || paused}
              reason={nothingToAdopt ? "Zitadel doesn't have this, so there's nothing to adopt." : undefined}
              onClick={() => onResolve("attribute")}
            >
              Adopt
            </Button>
            <Button variant="danger" size="sm" disabled={paused} onClick={() => onResolve("revoke")}>
              Revoke
            </Button>
            <Button variant="outline" size="sm" disabled={paused} onClick={() => onResolve("external")}>
              Owned elsewhere
            </Button>
            <button
              type="button"
              onClick={onExpand}
              aria-expanded={expanded}
              data-testid="drift-row-toggle"
              className={QUIET_LINK}
            >
              {expanded ? "Hide evidence" : "Show evidence"}
            </button>
            {/* Drift arrives in clusters, so the fastest way to select the ones
                that belong together is to say so, not to hunt for them. */}
            <button type="button" onClick={onSelectSimilar} className={QUIET_LINK}>
              Select similar rows
            </button>
          </div>
          {item.user_is_service_account && (
            <p className="text-[12.5px] leading-[1.5] text-faint">
              Adopt is off: this account re-creates its own access on every deploy, whatever Syndra
              records.
            </p>
          )}
        </div>
      </div>

      <Unfold open={expanded}>
        <ExpandedEvidence item={item} />
      </Unfold>
    </div>
  );
}

/**
 * The risk pill says what it means in words as well as colour: amber and red
 * are load-bearing here, and colour is never allowed to be the only signal.
 */
function RiskPill({ item }: { item: DriftTriageItem }) {
  // Only a target whose access Syndra catalogues can have a role missing from
  // it. On one that has none, "not in catalogue" would be true of every row and
  // informative about none.
  if (item.role_catalogue_applies && !item.role_in_catalogue) {
    return <Badge className="mt-1">Role unknown to Syndra</Badge>;
  }
  if ((item.role_group ?? "").toLowerCase().includes("safety")) {
    return (
      <Badge tone="dangerSoft" className="mt-1">
        {item.role_group}
      </Badge>
    );
  }
  if (item.role_group) {
    // The same pill as the safety one in every respect but its tone. It used to
    // be a separate inline copy that had lost its `font-semibold`, so a role
    // group rendered bold or not depending on whether it was a safety group —
    // a weight difference that read as meaning and carried none.
    return <Badge className="mt-1">{item.role_group}</Badge>;
  }
  return null;
}

/** Three columns: what a revoke costs, what an adopt records, and the evidence. */
function ExpandedEvidence({ item }: { item: DriftTriageItem }) {
  return (
    <div className="row-divider grid gap-5 bg-surface-0 px-5 py-4 tablet:grid-cols-3">
      <div>
        <div className="type-label mb-1.5">If you revoke</div>
        <p className="text-[13.5px] leading-[1.55] text-muted">
          This queues the removal — revocations send on their own, every few minutes. An app they are already signed in to keeps what it was told until they sign in again.
        </p>
      </div>
      <div>
        <div className="type-label mb-1.5">If you adopt</div>
        <p className="text-[13.5px] leading-[1.55] text-muted">
          Syndra records this as access you gave, and the row stops appearing here. The
          person&rsquo;s access does not change.
        </p>
      </div>
      <div>
        <div className="type-label mb-1.5">Evidence</div>
        <dl className="type-mono grid grid-cols-[84px_1fr] gap-x-3 gap-y-1 text-[12.5px] text-muted">
          <dt className="text-faint">Zitadel id</dt>
          <dd className="truncate">{item.zitadel_grant_id || "—"}</dd>
          <dt className="text-faint">Created</dt>
          <dd>{item.upstream_created_at ? formatShortDate(item.upstream_created_at) : "unknown"}</dd>
          <dt className="text-faint">Made by</dt>
          <dd className="truncate">
            {item.upstream_actor ? <UserName id={item.upstream_actor} /> : "unknown"}
          </dd>
          <dt className="text-faint">Last seen</dt>
          <dd>{item.last_seen_at ? formatShortDate(item.last_seen_at) : "—"}</dd>
        </dl>
      </div>
    </div>
  );
}

/**
 * The three resolutions, each with its consequence stated. Only Revoke takes a
 * solid destructive fill, and only inside this dialog.
 */
function ResolutionDialog({
  pending,
  onClose,
}: {
  pending: { item: DriftTriageItem; resolution: Resolution } | null;
  onClose: () => void;
}) {
  const attribute = useAttributeDrift();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const revoke = useRevokeDrift();
  const external = useMarkExternalDrift();

  if (!pending) return null;
  const { item, resolution } = pending;
  const busy = attribute.isPending || revoke.isPending || external.isPending;
  const role = item.role_keys[0] ?? "";

  const copy = {
    attribute: {
      title: "Adopt this access in Syndra?",
      lede: "Syndra takes ownership of it. The access stays exactly as it is; from now on Syndra explains it and can extend, end or revoke it like any other access.",
      confirm: "Adopt in Syndra",
      variant: "accent" as const,
    },
    external: {
      title: "Is this owned by another system?",
      lede: "The access stays and stops being listed here. Use this when a known system legitimately manages it, not to quiet something you have not identified.",
      confirm: "Mark as owned elsewhere",
      variant: "accent" as const,
    },
    revoke: {
      title: "Revoke this access?",
      lede: "This queues removal of the access in Zitadel and records your decision in Syndra, so the next check will not list it again.",
      confirm: "Revoke access",
      variant: "dangerConfirm" as const,
    },
  }[resolution];

  return (
    <Modal open onClose={onClose} busy={busy} size="sm" labelledBy="triage-title">
      <ModalHeader title={copy.title} titleId="triage-title" lede={copy.lede} />
      <div className="flex flex-col gap-3 px-6">
        <div className="rounded-inner bg-tint-1 px-4 py-3.5 text-[14.5px]">
          <UserName id={item.user_id} /> · <ProjectName id={item.project_id} /> / <Mono>{role}</Mono>
        </div>

        {resolution === "revoke" && (
          <>
            <div className="danger-note px-4 py-3.5 text-[14.5px] leading-[1.5]">
              <strong className="font-semibold text-danger-text">
                They will lose this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                This queues the removal in Syndra; revocations send on their own, every few
                minutes. Until then they still have the access.
              </span>
            </div>
            <p className="text-[13.5px] leading-[1.55] text-muted">
              If another system adds this access back, it shows up here again after the next
              check.
            </p>
            {item.other_items_for_user > 0 && (
              <p className="text-[13.5px] leading-[1.55] text-muted">
                Before you decide: this person has{" "}
                <strong className="font-semibold text-ink">
                  {item.other_items_for_user} more{" "}
                  {item.other_items_for_user === 1 ? "item" : "items"}
                </strong>{" "}
                in this list. One stray role is a mistake; several is a leaver whose access was
                never ended.
              </p>
            )}
          </>
        )}
      </div>
      {/* The dialog states what it did, and for a revocation that is that
          nothing has reached the target yet — the access is still there until
          the drain runs. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        {/* Cancel is first in the DOM — and so is what the dialog's opening
            focus lands on — so that the destructive fill on Revoke's confirm
            button is never also the button the Modal focus trap hands the
            cursor to. `order-2` puts it back on the right, where the confirm
            action has always sat. */}
        <Button
          variant={succeeded(outcome) ? "outline" : "ghost"}
          className="order-2"
          onClick={onClose}
        >
          {succeeded(outcome) ? "Close" : "Cancel"}
        </Button>
        {/* This is the highest-stakes revoke in the product, and it stayed
            armed after it ran: a second click on the same "Revoke access"
            button revoked an already-revoked grant a second time, on a
            confirm dialog that never closed to report it. Once the mutation
            has taken, the only action left true is leaving. */}
        {!succeeded(outcome) && (
          <Button
            variant={copy.variant}
            className="order-1"
            isPending={busy}
            onClick={async () => {
              try {
                if (resolution === "attribute") {
                  await attribute.mutateAsync({ id: item.id, body: { source: "external_backfill" } });
                } else if (resolution === "revoke") {
                  await revoke.mutateAsync({ id: item.id });
                } else {
                  await external.mutateAsync({ id: item.id, body: {} });
                }
                setOutcome(
                  resolution === "revoke"
                    ? {
                        kind: "queued",
                        message: "Revocation recorded",
                        detail:
                          "Not sent to Zitadel yet; revocations send on their own, every few minutes. Until then the person still has the access.",
                      }
                    : {
                        kind: "applied",
                        // Not "will not be listed again" — that renders right
                        // above this item's own name and role while both are
                        // still on screen, which makes it false the instant it
                        // appears. It leaves once this dialog closes.
                        message: "Resolved. It drops off this list once you close this.",
                      },
                );
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {copy.confirm}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

/**
 * A full sentence naming the upstream actor and date where the detector knew
 * them. Where it did not — the reconciliation sweep compares grant sets and
 * genuinely cannot know who made a change — the row says so rather than
 * naming a plausible culprit.
 */
/**
 * Who made it, asked one row at a time.
 *
 * A sweep-detected row cannot name an actor: the sweep compares grant SETS, and
 * a set difference has no author. The add-on targets solve the same problem
 * with a recorded merge base — remember what you last saw, infer who moved.
 * Zitadel needs no inference; it is event-sourced, and the event that created
 * the grant carries its editor. Reading it is strictly better than deducing it,
 * which is why this side has no merge base and this button instead.
 *
 * Behind a click rather than on load. The queue routinely holds dozens of rows
 * and this is one API call each; asking for all of them to render a page would
 * be a burst against the identity provider to answer a question nobody asked.
 *
 * Rendered only where it can help: a row Syndra already has an actor for says
 * so through `explainDrift`, and a row naming no grant has no event to read.
 */
function WhoMadeIt({ item }: { item: DriftTriageItem }) {
  const [asked, setAsked] = useState(false);
  const origin = useDriftOrigin(item.id, asked);

  if (item.upstream_actor || !item.zitadel_grant_id) return null;

  if (!asked) {
    return (
      <button
        type="button"
        onClick={() => setAsked(true)}
        className={QUIET_LINK}
      >
        Who made it?
      </button>
    );
  }
  if (origin.isLoading) {
    return <p className="mt-1 text-[12.5px] text-faint">Asking Zitadel…</p>;
  }

  const data = origin.data;
  // Three different answers, and none of them may render as another: the
  // lookup failed, the log does not go back that far, or the event names
  // nobody. Collapsing any pair would put a claim on the row that nothing
  // supports.
  if (origin.isError || !data || !data.readable) {
    return (
      <p className="mt-1 text-[12.5px] text-warn-text">
        Zitadel could not be asked. Somebody may still have made this; it is just not known who.
        {data?.detail ? ` ${data.detail}` : ""}
      </p>
    );
  }
  if (!data.recorded) {
    return (
      <p className="mt-1 text-[12.5px] text-faint">
        Zitadel&rsquo;s event log does not go back to when this was made.
      </p>
    );
  }
  if (!data.attributed) {
    return (
      <p className="mt-1 text-[12.5px] text-faint">
        Zitadel recorded the change and not who made it.
      </p>
    );
  }
  return (
    <p className="mt-1 text-[12.5px] text-muted">
      Made by{" "}
      <span className="font-semibold text-ink">
        {data.actor_name || (data.actor_id ? <UserName id={data.actor_id} /> : data.service)}
      </span>
      {data.actor_name && data.service ? (
        <span className="text-faint"> via {data.service}</span>
      ) : null}
      {data.at ? (
        <>
          {" "}
          <Relative iso={data.at} />
        </>
      ) : null}
      .
    </p>
  );
}

function explainDrift(item: DriftTriageItem): { text: string; note?: string } {
  const text = explainBody(item);
  // Two separate facts, said separately (owner's requirement): the sentence is
  // what the evidence does not show; this one is a guess about WHY, and only
  // ventured when the event log reaches back far enough to support it
  // (event_possibly_missed already carries that check). It is the sweep-found
  // case alone — a row with a named maker has no missing event to wonder at.
  const sweepFound =
    item.drift_type !== "syndra_only" && !item.upstream_created_at && !item.upstream_actor;
  return {
    text,
    note:
      sweepFound && item.event_possibly_missed
        ? "Zitadel usually tells Syndra about a change like this on its own. This time nothing arrived, which is worth a look."
        : undefined,
  };
}

function explainBody(item: DriftTriageItem): string {
  if (item.drift_type === "syndra_only") {
    // The row used to say "usually a queued write that never landed" for every
    // one of these, which is the wrong half of the story whenever the target was
    // seen HOLDING it: that is not a write that never happened, it is one
    // somebody undid. Told with its history, the row is recognisably the same
    // entitlement Syndra applied rather than a stranger.
    const p = item.provenance;
    // The write landing is the stronger evidence and, for a grant applied and
    // removed between two sweeps, the only evidence: nothing ever read it, so
    // the observation below does not exist. Told first for that reason.
    if (p?.applied_at) {
      const who = p.granted_by ? ` by ${p.granted_by}` : "";
      const why = p.reason ? ` — ${p.reason}` : "";
      const held = p.last_observed_at
        ? ` Zitadel was still holding it on ${formatShortDate(p.last_observed_at)}.`
        : "";
      const removedBy = item.upstream_actor ? ` Removed by ${item.upstream_actor}.` : "";
      return `Granted${who}${why}. Syndra applied it on ${formatShortDate(
        p.applied_at,
      )} and Zitadel accepted it.${held} It is not there now, so somebody removed it.${removedBy}`;
    }
    if (p?.last_observed_at) {
      const who = p.granted_by ? ` by ${p.granted_by}` : "";
      const when = p.granted_at ? ` on ${formatShortDate(p.granted_at)}` : "";
      const why = p.reason ? ` — ${p.reason}` : "";
      const removedBy = item.upstream_actor ? ` Removed by ${item.upstream_actor}.` : "";
      return `Granted${who}${when}${why}. Zitadel was still holding it on ${formatShortDate(
        p.last_observed_at,
      )} and does not now, so somebody removed it there.${removedBy}`;
    }
    if (p?.granted_at) {
      return `Granted${p.granted_by ? ` by ${p.granted_by}` : ""} on ${formatShortDate(
        p.granted_at,
      )}. Zitadel has never been seen holding it, so the change most likely never went through. Check Pending changes.`;
    }
    return "Syndra expects this access but Zitadel does not have it, and there is no record of it ever being held there.";
  }
  const when = item.upstream_created_at ? ` on ${formatShortDate(item.upstream_created_at)}` : "";
  const who = item.upstream_actor ? ` by ${item.upstream_actor}` : "";
  if (when || who) {
    return `Created in Zitadel${when}${who}. Nothing in Syndra — no direct access, no bundle, no automatic rule — gives it.`;
  }
  return "Found by the scheduled check, which compares lists and cannot see who made the change. Nothing in Syndra — no direct access, no bundle, no automatic rule — gives it.";
}

// The third copy of "is this person gone", now the shared one. It was missing
// `deactivated`, `locked` and `deleted`, so an account the people index and the
// bulk guard both called departed read as "Member" here — on the row where
// whether somebody is a leaver is the entire basis of the triage decision.
function describeHolder(item: DriftTriageItem): string {
  if (item.user_is_service_account) return "Service account";
  if (isDeparted(item.user_status)) return "No longer active";
  return "Member";
}

/**
 * Bulk adopt / bulk mark-as-external, rehearsed.
 *
 * The same dialog the People page opens for a bulk grant, because it is the
 * same decision shape: see what would happen to every selected row, then apply
 * it. Adopting writes ledger rows and marking-external suppresses future
 * detection — neither is trivially undone from here, and a triage queue is
 * exactly where an operator is moving fast.
 */
function BulkResolutionDialog({
  op,
  ids,
  composition,
  onClose,
  onApplied,
}: {
  op: "adopt" | "external";
  ids: string[];
  composition: string;
  onClose: () => void;
  onApplied: () => void;
}) {
  const rehearseAdopt = useRehearseAdoptDrift();
  const applyAdopt = useBulkAttributeDrift();
  const rehearseExternal = useRehearseMarkExternalDrift();
  const applyExternal = useBulkMarkExternalDrift();

  const adoptBody = { ids, source: "external_backfill" as const };
  const externalBody = { ids, reason: "Marked as owned elsewhere in bulk from Unexplained access" };

  return (
    <RehearsalDialog
      title={op === "adopt" ? "Adopt in Syndra" : "Mark as owned elsewhere"}
      lede={composition}
      noun={["item", "items"]}
      onRehearse={(acknowledgeScope) =>
        op === "adopt"
          ? rehearseAdopt.mutateAsync({ ...adoptBody, acknowledge_scope: acknowledgeScope })
          : rehearseExternal.mutateAsync({ ...externalBody, acknowledge_scope: acknowledgeScope })
      }
      onApply={async (planId) => {
        const plan =
          op === "adopt"
            ? await applyAdopt.mutateAsync({ ...adoptBody, plan_id: planId })
            : await applyExternal.mutateAsync({ ...externalBody, plan_id: planId });
        onApplied();
        return plan;
      }}
      onClose={onClose}
    />
  );
}
