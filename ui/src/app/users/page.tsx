"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";

import { BulkDialog } from "@/components/people/BulkDialog";
import { ListStates, EmptyState, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Card, CardColumns } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PageHeader } from "@/components/ui/PageHeader";
import { Select } from "@/components/ui/Select";
import {
  RowCheckbox,
  SelectAllCheckbox,
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
  parseFilters,
  serializeFilters,
  type PeopleFilters,
} from "@/lib/people-filters";
import type { BulkOp } from "@/lib/queries/useBulkGrants";
import { useProjects } from "@/lib/queries/useProjects";
import { useRoleMembers } from "@/lib/queries/useRoleMembers";
import { useUsers, type UserListEntry } from "@/lib/queries/useUsers";
import { useDebounce } from "@/lib/useDebounce";
import { daysUntil } from "@/lib/format";

/** Explicit pagination, never infinite scroll: a queue you can reach the end of. */
const PAGE = 50;

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

  // The search box is typed into, so it holds local state and syncs to the URL
  // on a debounce — writing a history entry per keystroke would make the back
  // button useless.
  const [queryDraft, setQueryDraft] = useState(filters.q);
  const debounced = useDebounce(queryDraft, 250);
  const [limit, setLimit] = useState(PAGE);
  const [wholeFilter, setWholeFilter] = useState(false);
  const [bulkOp, setBulkOp] = useState<BulkOp | null>(null);

  const users = useUsers(filters.q);
  const projects = useProjects();
  // A role filter is only meaningful inside a project, and membership comes
  // from the role-members endpoint so "who holds this" cannot mean two
  // different things on two screens.
  const roleMembers = useRoleMembers(filters.role ? filters.project : "", filters.role);

  const setParams = useCallback(
    (next: Partial<PeopleFilters>, extra: Record<string, string> = {}) => {
      const merged = { ...filters, ...next };
      const carried = bulkMode ? { bulk: "1", ...extra } : extra;
      router.replace(`/users${serializeFilters(merged, carried)}`, { scroll: false });
    },
    [filters, bulkMode, router],
  );

  useEffect(() => {
    if (debounced !== filters.q) setParams({ q: debounced });
    // Only the debounced draft should drive this; re-running on every filter
    // change would fight the URL it just wrote.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debounced]);

  const filterKey = `${filters.q}|${filters.project}|${filters.role}|${filters.bundle}|${filters.version}|${filters.attention}`;

  // Changing what the filter matches while a selection is live would silently
  // re-aim the pending action at a different set of people. Drop it, and reset
  // the page window with it.
  useEffect(() => {
    setLimit(PAGE);
    setWholeFilter(false);
  }, [filterKey]);

  // Leaving bulk mode clears the selection, so re-entering never resumes a
  // selection the operator has forgotten making.
  useEffect(() => {
    if (!bulkMode) setWholeFilter(false);
  }, [bulkMode]);

  const all = useMemo(() => users.data ?? [], [users.data]);
  const holders = useMemo(() => {
    if (!filters.role || !roleMembers.data) return null;
    return new Set(roleMembers.data.members.map((member) => member.user.id));
  }, [filters.role, roleMembers.data]);

  const rows = useMemo(() => applyFilters(all, filters, holders), [all, filters, holders]);
  const visible = rows.slice(0, limit);
  // Selection spans the whole filter, not the rendered page.
  const selection = useRowSelection(useMemo(() => rows.map((entry) => entry.user.id), [rows]));
  const expiringSoon = all.filter((entry) => entry.expiring_count > 0).length;

  const projectName = useMemo(
    () => projects.data?.find((row) => row.project.id === filters.project)?.project.name ?? "",
    [projects.data, filters.project],
  );
  const scope = describeFilters(filters, projectName);

  const exitBulk = useCallback(() => {
    router.replace(`/users${serializeFilters(filters)}`, { scroll: false });
  }, [filters, router]);

  useEffect(() => {
    if (!bulkMode) return;
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") exitBulk();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [bulkMode, exitBulk]);

  // Select-all covers every row matching the filter rather than the rendered
  // page — otherwise selecting 214 people means paging four times first, which
  // is the tedium this mode exists to remove. `wholeFilter` only remembers that
  // it happened, so the bar can offer to narrow back to what is on screen.
  function toggleAll() {
    const wasAll = selection.allSelected;
    selection.toggleAll();
    setWholeFilter(!wasAll && rows.length > visible.length);
  }

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="People"
        meta={
          all.length > 0
            ? `${all.length} ${all.length === 1 ? "account" : "accounts"}${
                expiringSoon > 0 ? ` · ${expiringSoon} with access expiring inside 30 days` : ""
              }`
            : undefined
        }
        actions={
          <>
            <Input
              value={queryDraft}
              onChange={(event) => setQueryDraft(event.target.value)}
              // Role keys are searchable here because "who has `trained` in the
              // laser lab" gets typed on this page before anyone thinks to go
              // to Roles. The placeholder has to say so or nobody tries it.
              placeholder="Search name, email or role key…"
              aria-label="Search people by name, email or role key"
              className="min-w-[240px]"
            />
            <Select
              value={filters.project}
              onChange={(event) => setParams({ project: event.target.value, role: "" })}
              aria-label="Filter by project"
              className="w-[170px]"
            >
              <option value="">Any project</option>
              {(projects.data ?? []).map((row) => (
                <option key={row.project.id} value={row.project.id}>
                  {row.project.name}
                </option>
              ))}
            </Select>
            <Select
              value={filters.attention}
              onChange={(event) =>
                setParams({ attention: event.target.value as PeopleFilters["attention"] })
              }
              aria-label="Filter by what needs attention"
              className="w-[190px]"
            >
              <option value="">Anything</option>
              {ATTENTION_VALUES.map((value) => (
                <option key={value} value={value}>
                  {ATTENTION_LABELS[value]}
                </option>
              ))}
            </Select>
            <button
              type="button"
              onClick={() => (bulkMode ? exitBulk() : setParams({}, { bulk: "1" }))}
              aria-pressed={bulkMode}
              className={`min-h-[44px] rounded-pill border px-4 text-[13.5px] font-semibold motion-tint desktop:min-h-0 desktop:py-[7px] ${
                bulkMode
                  ? "border-accent-line bg-accent-soft text-accent-text"
                  : "border-line-strong hover:bg-[var(--hover)]"
              }`}
            >
              {bulkMode ? "Done selecting" : "Select"}
            </button>
          </>
        }
      />

      {(filters.role || filters.bundle || filters.version) && (
        <div className="flex flex-wrap items-center gap-2">
          {filters.role && (
            <FilterChip
              label={`Holding ${filters.role}${projectName ? ` in ${projectName}` : ""}`}
              onClear={() => setParams({ role: "" })}
            />
          )}
          {filters.bundle && (
            // Two chips, not one, because they clear independently: dropping
            // the version to see the whole bundle is the move an operator makes
            // constantly once they have found the stragglers.
            <FilterChip
              label={`In the ${filters.bundle} bundle`}
              onClear={() => setParams({ bundle: "", version: "" })}
            />
          )}
          {filters.version && (
            <FilterChip
              label={`on v${filters.version}`}
              onClear={() => setParams({ version: "" })}
            />
          )}
        </div>
      )}

      <Card>
        <CardColumns>
          {bulkMode && (
            <span className="w-[26px]">
              <SelectAllCheckbox
                {...selection.headerCheckboxProps}
                onChange={toggleAll}
                label={
                  selection.allSelected
                    ? "Clear the selection"
                    : `Select all ${rows.length} people matching this filter`
                }
              />
            </span>
          )}
          <span className="w-[250px]">Person</span>
          <span className="w-[150px]">Team</span>
          <span className="w-[220px]">Bundles</span>
          <span className="flex-1">Access</span>
          <span className="w-[150px] text-right">Needs attention</span>
        </CardColumns>

        <div data-selection-scope {...selection.containerProps}>
        <ListStates
          isLoading={users.isLoading}
          error={users.error}
          isEmpty={rows.length === 0}
          onRetry={() => users.refetch()}
          errorTitle="Couldn't load people."
          skeleton={<RowSkeleton rows={6} label="Loading people" />}
          empty={
            <EmptyState
              title={hasAnyFilter(filters) ? "Nobody matches that." : "There's nobody here yet."}
              guidance={
                hasAnyFilter(filters)
                  ? "Try a shorter search, part of an email address, or a role key."
                  : "People appear here once they exist in the identity provider."
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
          {visible.map((entry) => (
            <PersonRow
              key={entry.user.id}
              entry={entry}
              bulkMode={bulkMode}
              selection={selection}
            />
          ))}

          {rows.length > visible.length && (
            <div className="row-divider flex items-center gap-4 px-5 py-3.5">
              <span className="text-[13.5px] text-faint">{rows.length - visible.length} more</span>
              <button
                type="button"
                onClick={() => setLimit((current) => current + PAGE)}
                className="min-h-[44px] rounded-pill border border-line-strong px-4 text-[13.5px] font-semibold motion-tint hover:bg-[var(--hover)] desktop:min-h-0 desktop:py-1.5"
              >
                Load next {Math.min(PAGE, rows.length - visible.length)}
              </button>
            </div>
          )}
        </ListStates>
        </div>
      </Card>

      {bulkMode && (
        <SelectionBar
          count={selection.count}
          noun={["person", "people"]}
          scope={scope}
          wholeScope={wholeFilter}
          visibleCount={visible.length}
          onSelectVisibleOnly={() => {
            selection.selectOnly(visible.map((entry) => entry.user.id));
            setWholeFilter(false);
          }}
          onClear={() => {
            selection.clear();
            setWholeFilter(false);
          }}
        >
          <SelectionAction onClick={() => setBulkOp("assign_role")}>Grant role</SelectionAction>
          <SelectionAction onClick={() => setBulkOp("assign_bundle")}>Add to bundle</SelectionAction>
          <SelectionAction onClick={() => setBulkOp("extend")}>Extend expiring</SelectionAction>
          <SelectionAction tone="danger" onClick={() => setBulkOp("remove_bundle")}>
            Remove bundle
          </SelectionAction>
          <SelectionAction tone="danger" onClick={() => setBulkOp("remove_role")}>
            Remove role
          </SelectionAction>
        </SelectionBar>
      )}

      {bulkOp && (
        <BulkDialog
          op={bulkOp}
          userIds={Array.from(selection.selected)}
          scope={scope}
          initial={{ projectId: filters.project, roleKey: filters.role }}
          onClose={() => setBulkOp(null)}
        />
      )}
    </div>
  );
}

/** A filter that has no dropdown of its own — arrived by link, cleared by hand. */
function FilterChip({ label, onClear }: { label: string; onClear: () => void }) {
  return (
    <div className="flex items-center gap-2.5 self-start rounded-pill bg-tint-2 py-1.5 pl-4 pr-2.5 text-[13.5px]">
      {label}
      <button
        type="button"
        onClick={onClear}
        aria-label={`Clear filter: ${label}`}
        className="rounded-pill px-2 py-0.5 font-semibold text-muted motion-tint hover:text-ink"
      >
        ✕
      </button>
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
  // A departed account still belongs in the list — it is often exactly who you
  // came looking for — but it reads at reduced contrast so a live person is
  // never mistaken for one who left.
  const departed = isDeparted(entry.user.status);

  const body = (
    <>
      <span className="flex w-full min-w-0 items-center gap-3 tablet:w-[250px]">
        <Avatar name={entry.user.name} />
        <span className="min-w-0">
          {/* In bulk mode the row is a control, so the name becomes the only
              link out — clicking anywhere else selects rather than navigates. */}
          {bulkMode ? (
            <Link
              href={`/users/${entry.user.id}`}
              onClick={(event) => event.stopPropagation()}
              className="block truncate text-[15px] font-semibold hover:text-accent-text"
            >
              {entry.user.name}
            </Link>
          ) : (
            <span className="block truncate text-[15px] font-semibold">{entry.user.name}</span>
          )}
          <span className="block truncate text-[12.5px] text-faint">
            {entry.user.title || entry.user.email}
          </span>
        </span>
      </span>

      <span className="hidden w-[150px] truncate text-[14px] text-muted tablet:block">{entry.user.team || "—"}</span>

      <span className="hidden w-[220px] flex-wrap gap-1.5 tablet:flex">
        {entry.bundle_names?.length ? (
          entry.bundle_names.map((name) => {
            // The version rides on the chip. "Lab Tech" and "Lab Tech v2" are
            // different facts about a person, and the row is where the
            // difference between two people in the same bundle is visible.
            const version = entry.bundle_versions?.[name];
            return (
              <span key={name} className="rounded-pill bg-tint-2 px-2.5 py-1 text-[12.5px]">
                {name}
                {version ? <span className="text-faint"> v{version}</span> : null}
              </span>
            );
          })
        ) : (
          <span className="text-[13px] text-faint">none</span>
        )}
      </span>

      <span className="min-w-0 text-[13px] text-muted tablet:flex-1 tablet:truncate tablet:text-[14px]">{describeAccess(entry)}</span>

      <span className="tablet:w-[150px] tablet:text-right">
        <NeedsAttention entry={entry} />
      </span>
    </>
  );

  // Stacked on a phone, columns above the tablet breakpoint. The cells that
  // survive are the two an operator scans for — who this is, and whether
  // anything about them needs doing — plus the sentence explaining why they
  // are in the list. Team and bundles fold away: both are one tap deeper, on
  // the person's own page, and neither is what this screen is scanned for.
  const shared = `row-divider flex min-h-[60px] flex-col items-start gap-1.5 px-5 py-3.5 motion-tint hover:bg-[var(--hover)] tablet:flex-row tablet:items-center tablet:gap-[18px] ${
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
      className={`${shared} cursor-pointer select-none ${selected ? "bg-accent-soft/40" : ""}`}
      {...selection.rowProps(entry.user.id)}
    >
      <span className="w-[26px]">
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
  if (entry.unexplained_count > 0) {
    return (
      <span className="text-[13.5px] font-semibold text-danger-text">
        {entry.unexplained_count} unexplained
      </span>
    );
  }
  if (entry.expiring_count > 0) {
    const days = daysUntil(entry.soonest_expiry);
    return (
      <span className="text-[13.5px] font-semibold text-warn-text">
        {entry.expiring_count} expires{" "}
        {days === null ? "soon" : days <= 0 ? "today" : `in ${days} day${days === 1 ? "" : "s"}`}
      </span>
    );
  }
  if (entry.open_request_count > 0) {
    return (
      <span className="text-[13.5px] font-semibold text-accent-text">
        {entry.open_request_count} open request{entry.open_request_count === 1 ? "" : "s"}
      </span>
    );
  }
  return <span className="text-[13.5px] text-faint">—</span>;
}

function describeAccess(entry: UserListEntry): string {
  if (entry.effective_role_count === 0) return "No roles yet";
  const roles = `${entry.effective_role_count} ${entry.effective_role_count === 1 ? "role" : "roles"}`;
  if (!entry.project_count) return roles;
  return `${roles} across ${entry.project_count} ${entry.project_count === 1 ? "project" : "projects"}`;
}
