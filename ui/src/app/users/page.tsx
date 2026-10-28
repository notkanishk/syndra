"use client";

import { useBundles } from "@/lib/queries/useBundles";
import { useUiView } from "@/lib/ui-view";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { RoleRef } from "@/components/names";
import { BulkDialog } from "@/components/people/BulkDialog";
import { ListStates, EmptyState, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { FilterChip } from "@/components/ui/FilterChip";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { ReadFreshness, type ReadState } from "@/components/ui/ReadFreshness";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { SortHead, orderBy, useSort } from "@/components/ui/SortHead";
import { FilterBar } from "@/components/ui/FilterBar";
import { FilterPanel, FilterField } from "@/components/ui/FilterPanel";
import { Select } from "@/components/ui/Select";
import {
  RowCheckbox,
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection, type RowSelection } from "@/lib/useRowSelection";
import {
  ATTENTION_LABELS,
  ATTENTION_VALUES,
  applyFilters,
  describeFilters,
  hasAnyFilter,
  isDeparted,
  isPerson,
  NO_BUNDLE,
  EMPTY_FILTERS,
  parseFilters,
  serializeFilters,
  type PeopleFilters,
  observedHolding,
} from "@/lib/people-filters";
import { BULK_MAX_USERS, type BulkOp } from "@/lib/queries/useBulkGrants";
import { useProjects } from "@/lib/queries/useProjects";
import { useRoleMembers } from "@/lib/queries/useRoleMembers";
import { useUsers, type UserListEntry } from "@/lib/queries/useUsers";
import { useDebounce } from "@/lib/useDebounce";
import { useIsTouch } from "@/lib/useViewport";
import { daysUntil } from "@/lib/format";

/** Explicit pagination, never infinite scroll: a queue you can reach the end of. */

/**
 * People — the index.
 *
 * The last column is the reason this list exists rather than being a plain
 * directory: it carries the one thing about this person that might need you.
 * Everything else on the row is context for recognising the right human
 * quickly. Take that column away and this is a phone book.
 *
 * Two things layer on top of that, both off by default:
 *
 *   - **Filters live in the URL.** Every count elsewhere in the product links
 *     here already narrowed, and the resulting view is shareable.
 *   - **Bulk mode is a toggle.** Checkboxes, the selection bar, and every bulk
 *     verb exist only while it is on. The default reading experience is
 *     unchanged — a list you scan, not a form you operate.
 */
export default function PeoplePage() {
  const router = useRouter();
  const params = useSearchParams();

  const filters = useMemo(() => parseFilters(new URLSearchParams(params.toString())), [params]);
  const bulkMode = params.get("bulk") === "1";
  // What a bulk pick is FOR, apart from who is listed: "Add people to this
  // role" sent the role as a filter, which listed only the people who already
  // held it. It travels as `give_*` instead, and prefills the dialog.
  const give = {
    projectId: params.get("give_project") ?? "",
    roleKey: params.get("give_role") ?? "",
    bundleId: params.get("give_bundle") ?? "",
  };

  // The search box is typed into, so it holds local state and syncs to the URL
  // on a debounce — writing a history entry per keystroke would make the back
  // button useless.
  const [queryDraft, setQueryDraft] = useState(filters.q);
  const debounced = useDebounce(queryDraft, 250);
  const paging = usePaging();
  const limit = paging.limit;
  const [wholeFilter, setWholeFilter] = useState(false);
  const [bulkOp, setBulkOp] = useState<BulkOp | null>(null);

  const users = useUsers(filters.q);
  const bundles = useBundles();
  const projects = useProjects();
  // A role filter is only meaningful inside a project, and membership comes
  // from the role-members endpoint so "who holds this" cannot mean two
  // different things on two screens.
  const roleMembers = useRoleMembers(filters.role ? filters.project : "", filters.role);

  const setParams = useCallback(
    (next: Partial<PeopleFilters>, extra: Record<string, string> = {}) => {
      const merged = { ...filters, ...next };
      const giving: Record<string, string> = give.roleKey
        ? { give_project: give.projectId, give_role: give.roleKey }
        : give.bundleId
          ? { give_bundle: give.bundleId }
          : {};
      const carried = bulkMode ? { bulk: "1", ...giving, ...extra } : extra;
      router.replace(`/users${serializeFilters(merged, carried)}`, { scroll: false });
    },
    [filters, bulkMode, router, give.projectId, give.roleKey, give.bundleId],
  );

  // The last search this page wrote to the URL, so that what comes back can be
  // told from a change made elsewhere (Back/Forward, a nav click).
  const wroteQ = useRef(filters.q);
  useEffect(() => {
    if (debounced !== filters.q) {
      wroteQ.current = debounced;
      setParams({ q: debounced });
    }
    // Only the debounced draft should drive this; re-running on every filter
    // change would fight the URL it just wrote.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debounced]);

  // The URL is the truth: a search changed from outside replaces the draft, or
  // the box would keep showing, and soon re-write, the old one.
  useEffect(() => {
    if (filters.q === wroteQ.current) return;
    wroteQ.current = filters.q;
    setQueryDraft(filters.q);
  }, [filters.q]);

  const filterKey = `${filters.q}|${filters.project}|${filters.role}|${filters.bundle}|${filters.version}|${filters.attention}`;

  // Changing what the filter matches while a selection is live would silently
  // re-aim the pending action at a different set of people. Drop it, and reset
  // the page window with it.
  useEffect(() => {
    paging.back();
    setWholeFilter(false);
  }, [filterKey, paging.back]); // eslint-disable-line react-hooks/exhaustive-deps

  // Leaving the mode keeps the selection — an operator who taps Done to read
  // one person's row properly has not changed their mind about the other
  // nine — but it drops `wholeFilter`, which is a claim about how the
  // selection was made and stops being true the moment the mode is left.
  useEffect(() => {
    if (!bulkMode) setWholeFilter(false);
  }, [bulkMode]);

  // People never includes a service account — not listed, not counted (A-07 §30).
  // A search that lands on one says so below, and points at where it lives.
  const all = useMemo(() => (users.data ?? []).filter(isPerson), [users.data]);
  const matchedServiceAccounts = useMemo(() => {
    const query = filters.q.trim().toLowerCase();
    if (!query) return [];
    return (users.data ?? []).filter(
      (entry) => !isPerson(entry) && entry.user.name.toLowerCase().includes(query),
    );
  }, [users.data, filters.q]);
  const holders = useMemo(() => {
    if (!filters.role || !roleMembers.data) return null;
    return new Set(roleMembers.data.members.map((member) => member.user.id));
  }, [filters.role, roleMembers.data]);

  // The board's default: Person, A to Z. Sorting is local to the view; the
  // filters in the URL decide who is listed, the headings only their order.
  const [sort, setSort, view] = useSort<PeopleSort>(
    // Who just arrived, from Home's "and N more" (A-07 §33).
    filters.bundle === NO_BUNDLE ? "joined" : "person",
  );
  const rows = useMemo(
    () => orderBy(applyFilters(all, filters, holders), view.sort, PEOPLE_SORT),
    [all, filters, holders, view.sort],
  );
  // S5: somebody who leaves the list on a later read (marked as a service account, no longer
  // matching) fades where they were and closes the gap after, and not under the pointer.
  const leaving = useLeavingRows(rows, (entry) => entry.user.id, !users.isLoading && !users.error, {
    narrowedBy: JSON.stringify(filters),
  });
  const visible = rows.slice(0, limit);
  const drawn = leaving.window(limit);
  // Selection spans the whole filter, not the rendered page.
  const selection = useRowSelection(useMemo(() => rows.map((entry) => entry.user.id), [rows]));
  const { selfId } = useUiView();
  const expiringSoon = all.filter((entry) => entry.expiring_count > 0).length;
  // One basis for the whole list: every row carries an identical copy of when
  // Zitadel was read, so the line says it once, under the search.
  const basis = all.find((entry) => entry.observation)?.observation;
  const readState: ReadState = {
    readAt: basis?.read_at,
    current: basis ? basis.current : undefined,
    truncated: basis?.truncated,
  };

  const projectName = useMemo(
    () => projects.data?.find((row) => row.project.id === filters.project)?.project.name ?? "",
    [projects.data, filters.project],
  );
  const scope = describeFilters(filters, projectName);
  // What the panel is hiding. Search is not counted — it is on the row, in
  // plain sight, and a badge reading "1" beside text somebody can already see
  // is a badge that means nothing.
  const narrowingCount = [
    filters.project,
    filters.role,
    filters.bundle,
    filters.version,
    filters.attention,
  ].filter(Boolean).length;

  const bulkOpen = bulkOp !== null;
  const exitBulk = useCallback(() => {
    router.replace(`/users${serializeFilters(filters)}`, { scroll: false });
  }, [filters, router]);

  useEffect(() => {
    if (!bulkMode) return;
    function onKey(event: KeyboardEvent) {
      // An Esc a dialog (or anything else) already took is not ours: it closes
      // the dialog and must not also drop the selection behind it.
      if (event.key === "Escape" && !event.defaultPrevented && !bulkOpen) exitBulk();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [bulkMode, bulkOpen, exitBulk]);

  // Select-all covers every row matching the filter rather than the rendered
  // page — otherwise selecting 214 people means paging four times first, which
  // is the tedium this mode exists to remove. `wholeFilter` only remembers that
  // it happened, so the bar can offer to narrow back to what is on screen.
  function toggleAll() {
    // While a new search is read, the rows on screen still answer the last
    // one: "every match" would select the wrong people.
    if (users.isPlaceholderData) return;
    const wasAll = selection.allSelected;
    selection.toggleAll();
    setWholeFilter(!wasAll && rows.length > visible.length);
  }

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="People"
        lede="Everyone Syndra knows, and what each can use. Open a person to see and change their access; select several to change them together."
        meta={
          all.length > 0
            ? `${all.length} ${all.length === 1 ? "person" : "people"}${
                expiringSoon > 0 ? ` · ${expiringSoon} with access ending inside 30 days` : ""
              }`
            : undefined
        }
      />

      {/* The filters live on their own row, not in the page header: three
          controls in the header stretched to the width of the actions area and
          stacked into three tall rows beside the title. Search stays visible —
          it is the fastest way to one person, and a search behind a disclosure
          is a search nobody uses. */}
      <FilterBar
        search={
          <Input
            value={queryDraft}
            onChange={(event) => setQueryDraft(event.target.value)}
            placeholder="Search name, email or role key…"
            aria-label="Search people by name, email or role key"
          />
        }
        filters={
          <FilterPanel
            resultCount={rows.length}
            noun={["person", "people"]}
            activeCount={narrowingCount}
            onClear={() => setParams(EMPTY_FILTERS)}
          >
            <FilterField label="Project">
              <Select
                value={filters.project}
                onChange={(event) => setParams({ project: event.target.value, role: "" })}
                aria-label="Filter by project"
              >
                <option value="">Any project</option>
                {(projects.data ?? []).map((row) => (
                  <option key={row.project.id} value={row.project.id}>
                    {row.project.name}
                  </option>
                ))}
              </Select>
            </FilterField>
            <FilterField label="Needs attention">
              <Select
                value={filters.attention}
                onChange={(event) =>
                  setParams({ attention: event.target.value as PeopleFilters["attention"] })
                }
                aria-label="Filter by what needs attention"
              >
                <option value="">Anything</option>
                {ATTENTION_VALUES.map((value) => (
                  <option key={value} value={value}>
                    {ATTENTION_LABELS[value]}
                  </option>
                ))}
              </Select>
            </FilterField>
          </FilterPanel>
        }
        trailing={
          <SelectModeToggle
            active={bulkMode}
            onToggle={() => (bulkMode ? exitBulk() : setParams({}, { bulk: "1" }))}
          />
        }
      />

      {all.length > 0 && (
        <ReadFreshness state={readState} subject="The people list" className="-mt-1.5 px-[2px]" />
      )}

      {matchedServiceAccounts.map((entry) => (
        <ArrowLink key={entry.user.id} href={`/system/service-accounts/${entry.user.id}`}>
          {`${entry.user.name} is a service account`}
        </ArrowLink>
      ))}

      {bulkMode && give.roleKey && (
        <p className="text-[14px] text-muted">
          Pick who should get <RoleRef projectId={give.projectId} roleKey={give.roleKey} />, then choose
          Give a role. Everyone is listed, including people who already hold it.
        </p>
      )}
      {bulkMode && give.bundleId && (
        <p className="text-[14px] text-muted">
          Pick who should get {bundles.data?.find((b) => b.id === give.bundleId)?.name ?? "the bundle"}, then
          choose Give a bundle.
        </p>
      )}

      {(filters.role || filters.bundle || filters.version) && (
        <div className="flex flex-wrap items-center gap-2">
          {filters.role && (
            <FilterChip
              onClear={() => setParams({ role: "" })}
              clearLabel={`Clear filter: holding ${filters.role}`}
            >
              {`Holding ${filters.role}${projectName ? ` in ${projectName}` : ""}`}
            </FilterChip>
          )}
          {filters.bundle === NO_BUNDLE && (
            <FilterChip onClear={() => setParams({ bundle: "" })} clearLabel="Clear filter: no bundle">
              No bundle
            </FilterChip>
          )}
          {filters.bundle && filters.bundle !== NO_BUNDLE && (
            // Two chips, not one, because they clear independently: dropping
            // the version to see the whole bundle is the move an operator makes
            // constantly once they have found the stragglers.
            <FilterChip
              onClear={() => setParams({ bundle: "", version: "" })}
              clearLabel={`Clear filter: in the ${filters.bundle} bundle`}
            >
              {`In the ${filters.bundle} bundle`}
            </FilterChip>
          )}
          {filters.version && (
            <FilterChip
              onClear={() => setParams({ version: "" })}
              clearLabel={`Clear filter: on v${filters.version}`}
            >
              {`on v${filters.version}`}
            </FilterChip>
          )}
        </div>
      )}

      <Card>
        <CardColumns>
          {bulkMode && <span className="w-11 shrink-0 desktop:w-[26px]" />}
          <span className="w-[250px] min-w-[180px]">
            <SortHead label="Person" name="person" sort={sort} onSort={setSort} />
          </span>
          <span className="hidden w-[180px] shrink-0 desktop:block">
            <SortHead label="Bundles" name="bundles" sort={sort} onSort={setSort} />
          </span>
          <span className="min-w-[170px] flex-1">Access</span>
          <span className="w-[110px] shrink-0 text-right">
            <SortHead label="Attention" name="attention" sort={sort} onSort={setSort} />
          </span>
        </CardColumns>

        {bulkMode && rows.length > 0 && (
          <SelectAllRow
            inScope={rows.length}
            total={all.length}
            noun={["person", "people"]}
            allSelected={selection.allSelected}
            {...selection.headerCheckboxProps}
            onChange={toggleAll}
          />
        )}

        <div data-selection-scope {...selection.containerProps} {...leaving.listProps}>
        <div className={view.className}>
        <ListStates
          // The last search's empty answer is not this one's: "Nobody matches" waits for it.
          isLoading={users.isLoading || (users.isPlaceholderData && leaving.rows.length === 0)}
          error={users.error}
          // Not `rows`: the last rows of a search fade where they were before "Nobody matches that" takes the place.
          isEmpty={leaving.rows.length === 0}
          onRetry={() => users.refetch()}
          errorTitle="Couldn't load people."
          skeleton={<RowSkeleton rows={6} label="Loading people" />}
          empty={
            <EmptyState
              title={hasAnyFilter(filters) ? "Nobody matches that." : "There's nobody here yet."}
              guidance={
                hasAnyFilter(filters)
                  ? "Try a shorter search, part of an email address, or a role key."
                  : "People appear here once they exist in Zitadel."
              }
              action={
                hasAnyFilter(filters)
                  ? {
                      label: "Clear the filters",
                      onClick: () => {
                        setQueryDraft("");
                        router.replace(`/users${bulkMode ? "?bulk=1" : ""}`, { scroll: false });
                      },
                    }
                  : undefined
              }
            />
          }
        >
          {drawn.map((entry) => (
            <LeavingRow key={entry.user.id} {...leaving.props(entry.user.id)}>
              <PersonRow entry={entry} bulkMode={bulkMode} selection={selection} />
            </LeavingRow>
          ))}

          <PaginationRow
            shown={visible.length}
            total={rows.length}
            onMore={paging.more}
            onAll={() => paging.all(rows.length)}
            onBack={paging.back}
          />
        </ListStates>
        </div>
        </div>
      </Card>

      {bulkMode && (
        <SelectionBar
          count={selection.count}
          noun={["person", "people"]}
          scope={scope}
          // #32: never take access from yourself in bulk, and say so before.
          composition={
            selfId && selection.isSelected(selfId)
              ? "Includes you · you are left out of anything that revokes access"
              : undefined
          }
          wholeScope={wholeFilter}
          visibleCount={visible.length}
          onSelectVisibleOnly={() => {
            selection.selectOnly(visible.map((entry) => entry.user.id));
            setWholeFilter(false);
          }}
          // The bulk endpoint takes 500 at a time. A selection over that is not
          // silently trimmed to fit and the bar is not disabled either — both
          // hand the operator a number they did not choose. It offers the one
          // narrowing move, in the order already on screen, so the 500 is a
          // cohort somebody picked rather than wherever the list happened to
          // stop.
          ceiling={BULK_MAX_USERS}
          onTakeCeiling={() => {
            selection.selectOnly(rows.slice(0, BULK_MAX_USERS).map((entry) => entry.user.id));
            setWholeFilter(false);
          }}
          onClear={() => {
            selection.clear();
            setWholeFilter(false);
          }}
        >
          {/* Every one of these opens a rehearsal, so every one of them says
              so. "Remove role" on a bar above nine selected people names an
              outcome that tapping it does not produce — the plan comes first,
              and the label that hides that is the label an operator taps
              expecting to be asked. */}
          <SelectionAction onClick={() => setBulkOp("assign_role")}>
            Give a role
          </SelectionAction>
          <SelectionAction onClick={() => setBulkOp("assign_bundle")}>
            Give a bundle
          </SelectionAction>
          <SelectionAction onClick={() => setBulkOp("extend")}>
            Extend what ends
          </SelectionAction>
          <SelectionAction tone="danger" onClick={() => setBulkOp("remove_bundle")}>
            Remove from a bundle
          </SelectionAction>
          <SelectionAction tone="danger" onClick={() => setBulkOp("remove_role")}>
            Revoke a role
          </SelectionAction>
        </SelectionBar>
      )}

      {bulkOp && (
        <BulkDialog
          op={bulkOp}
          userIds={Array.from(selection.selected)}
          scope={scope}
          initial={
            give.roleKey || give.bundleId ? give : { projectId: filters.project, roleKey: filters.role }
          }
          onClose={() => setBulkOp(null)}
        />
      )}
    </div>
  );
}

function PersonRow({
  entry,
  bulkMode,
  selection,
}: {
  entry: UserListEntry;
  bulkMode: boolean;
  selection: RowSelection;
}) {
  const selected = selection.isSelected(entry.user.id);
  const touch = useIsTouch();
  // A departed account still belongs in the list — it is often exactly who you
  // came looking for — but it reads at reduced contrast so a live person is
  // never mistaken for one who left.
  const departed = isDeparted(entry.user.status);
  // Who they are, in the words they hold: title, then team. The email is only
  // the fallback for someone with no title, never a second line of its own.
  const lead = entry.user.title || entry.user.email;
  const subtitle = entry.user.team ? `${lead} · ${entry.user.team}` : lead;
  const bundleLine = (entry.bundle_names ?? [])
    .map((name) => {
      const version = entry.bundle_versions?.[name];
      return version ? `${name} v${version}` : name;
    })
    .join(" · ");

  const body = (
    <>
      <span className="flex w-full min-w-0 items-center gap-3 tablet:w-[250px]">
        <Avatar name={entry.user.name} />
        <span className="min-w-0">
          {/* In bulk mode the row is a control, so the name becomes the only
              link out — clicking anywhere else selects rather than navigates.
              With a mouse that is a precise escape hatch. With a thumb it is a
              trap: the name is the largest thing in the row and the obvious
              place to press, and pressing it leaves the mode and drops the
              operator on somebody else's page mid-selection. Below the desktop
              breakpoint the whole row selects, which is what A4 asked for. */}
          {bulkMode && !touch ? (
            <Link
              href={`/users/${entry.user.id}`}
              onClick={(event) => event.stopPropagation()}
              className="block truncate text-[15px] font-semibold link-hover"
            >
              {entry.user.name}
            </Link>
          ) : (
            <span className="block truncate text-[15px] font-semibold">{entry.user.name}</span>
          )}
          <span className="block truncate text-[12.5px] text-faint" title={subtitle}>
            {subtitle}
          </span>
          {/* Below desktop the bundles fold into a line under the person, as the
              board draws them, so Access keeps its width. */}
          {bundleLine && (
            <span className="block truncate text-[12.5px] text-faint desktop:hidden">{bundleLine}</span>
          )}
        </span>
      </span>

      <span className="hidden w-[180px] shrink-0 flex-wrap gap-1.5 desktop:flex">
        {entry.bundle_names?.length ? (
          entry.bundle_names.map((name) => {
            // The version rides on the chip. "Lab Tech" and "Lab Tech v2" are
            // different facts about a person, and the row is where the
            // difference between two people in the same bundle is visible.
            const version = entry.bundle_versions?.[name];
            return (
              <Badge key={name}>
                {name}
                {version ? <span className="text-faint"> v{version}</span> : null}
              </Badge>
            );
          })
        ) : (
          <span className="text-[13px] text-faint">none</span>
        )}
      </span>

      {/* Wraps. What somebody holds is what the row is read for. */}
      <span className="min-w-0 text-[13px] text-muted [text-wrap:pretty] tablet:min-w-[170px] tablet:flex-1 tablet:text-[14px]">
        {describeAccess(entry)}
      </span>

      <span className="tablet:w-[110px] tablet:shrink-0 tablet:text-right">
        <NeedsAttention entry={entry} />
      </span>
    </>
  );

  // Stacked on a phone, columns above the tablet breakpoint. The cells that
  // survive are the two an operator scans for — who this is, and whether
  // anything about them needs doing — plus the sentence explaining why they
  // are in the list. Team and bundles fold away: both are one tap deeper, on
  // the person's own page, and neither is what this screen is scanned for.
  const shared = `row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3.5 hover-tint tablet:flex-row tablet:items-center tablet:gap-[18px] ${
    departed ? "opacity-60" : ""
  }`;

  if (!bulkMode) {
    return (
      <Link href={`/users/${entry.user.id}`} className={shared}>
        {body}
      </Link>
    );
  }

  return (
    // A label rather than a div: the whole row becomes the checkbox's hit area
    // for free, keyboard and screen reader included.
    <label
      className={`${shared} cursor-pointer select-none ${selected ? "bg-accent-soft" : ""}`}
      {...selection.rowProps(entry.user.id)}
    >
      <span className="w-11 desktop:w-[26px]">
        <RowCheckbox
          label={`Select ${entry.user.name}`}
          {...selection.checkboxProps(entry.user.id)}
        />
      </span>
      {body}
    </label>
  );
}

/**
 * One line, in the semantic colour it belongs to. Deliberately singular: if
 * three things need you the row names the most urgent, because a cell holding
 * three coloured phrases is a cell nobody reads.
 */
function NeedsAttention({ entry }: { entry: UserListEntry }) {
  const attention = attentionOf(entry);
  if (!attention) {
    // Nothing to report renders nothing — not a lone dash under the access
    // text at phone width, where this is the only thing in the column. The
    // header keeps the column's width on desktop regardless.
    return null;
  }
  return (
    <span className={`text-[13.5px] font-semibold [text-wrap:pretty] ${attention.tone}`}>
      {attention.text}
    </span>
  );
}

/**
 * The most urgent thing about a person, in a few words and the colour it
 * belongs to. `rank` orders the Attention heading: urgent first when it sorts
 * ascending, so the first press puts the people who need you on top.
 */
function attentionOf(entry: UserListEntry): { text: string; tone: string; rank: number } | null {
  if (entry.unexplained_count > 0) {
    const n = entry.unexplained_count;
    return {
      text: n === 1 ? "Unexplained role" : `${n} unexplained roles`,
      tone: "text-danger-text",
      rank: -3_000 - n,
    };
  }
  if (entry.expiring_count > 0) {
    const days = daysUntil(entry.soonest_expiry);
    const n = entry.expiring_count;
    const when = days === null ? "soon" : days <= 0 ? "today" : `in ${days} day${days === 1 ? "" : "s"}`;
    return {
      text: n === 1 ? `Ends ${when}` : `${n} end, the first ${when}`,
      tone: "text-warn-text",
      rank: -2_000 - n,
    };
  }
  if (entry.open_request_count > 0) {
    const n = entry.open_request_count;
    return {
      text: n === 1 ? "Open request" : `${n} open requests`,
      tone: "text-accent-text",
      rank: -1_000 - n,
    };
  }
  return null;
}

type PeopleSort = "person" | "bundles" | "attention" | "joined";

const PEOPLE_SORT: Record<PeopleSort, (entry: UserListEntry) => string | number> = {
  person: (entry) => entry.user.name,
  // Nobody-in-a-bundle sorts after the last bundle name, not before the first.
  bundles: (entry) => entry.bundle_names?.[0] ?? "\uffff",
  attention: (entry) => attentionOf(entry)?.rank ?? 0,
  // Newest first is its natural order (A-07 §18), so it ascends on the negated
  // time. Nobody with no date goes last, not first. No heading of its own: it
  // is only ever the default for a view that is about who just arrived.
  joined: (entry) => {
    const at = Date.parse(entry.user.joined ?? "");
    return Number.isNaN(at) ? Number.MAX_SAFE_INTEGER : -at;
  },
};

/**
 * What Zitadel shows this person holding, not what Syndra decided to give
 * them — "holding is observed; explanation is recorded". Falls back to the
 * recorded count only in the one case where there is nothing else to say:
 * before the org has ever been read.
 */
function describeAccess(entry: UserListEntry): string {
  const count = observedHolding(entry);
  if (count === undefined) {
    // Told apart, because they ask different things of an operator: nobody has
    // looked yet, versus the look did not finish.
    return entry.observation?.read_at ? "Couldn’t check" : "Not checked yet";
  }
  if (count === 0) {
    return entry.effective_role_count > 0 ? "Not in Zitadel yet" : "Nothing yet";
  }
  const roles = `${count} ${count === 1 ? "role" : "roles"}`;
  const projects = entry.observed_project_count ?? 0;
  if (!projects) return roles;
  return `${roles} across ${projects} ${projects === 1 ? "project" : "projects"}`;
}
