"use client";

import { useMemo, useState } from "react";

import { toast } from "sonner";

import { AccessSource } from "@/components/access/AccessSource";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { BulkDialog } from "@/components/people/BulkDialog";
import { Card, CardColumns } from "@/components/ui/Card";
import {
  RowCheckbox,
  SelectAllCheckbox,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection, type RowSelection } from "@/lib/useRowSelection";
import { PageHeader } from "@/components/ui/PageHeader";
import { ProjectName, UserAvatar, UserName } from "@/components/names";
import { useExpiringGrants, type ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";
import { useCreateGrant } from "@/lib/queries/useUsers";
import { daysUntil, formatShortDate } from "@/lib/format";

/** How long an extension buys. Stated in the button's toast, never implied. */
const EXTEND_DAYS = 90;

/**
 * S7 · Review › Expiring access.
 *
 * Its own route, not an audit tab: audit is a record you consult, this is
 * time-boxed work you act on before a deadline, and a sidebar badge should
 * point at a destination rather than a tab.
 *
 * There is exactly ONE action here: extend. Doing nothing is not an action and
 * is not drawn as one — no "let it lapse", no dismiss, no secondary control
 * that looks like it submits something. The row states the outcome of inaction
 * and the sweep does the rest.
 */
export default function ExpiringAccessPage() {
  const grants = useExpiringGrants(30);
  const rows = useMemo(() => [...(grants.data ?? [])].sort(sortBySoonest), [grants.data]);
  // Extending is the only action on this queue, and it is the one that most
  // obviously wants doing to a dozen rows at once.
  const selection = useRowSelection(useMemo(() => rows.map((grant) => grant.id), [rows]));
  const [extending, setExtending] = useState(false);

  // The bulk endpoint extends by user, and it only ever touches grants that
  // actually expire — which is every row on this screen.
  const selectedUsers = useMemo(
    () =>
      Array.from(
        new Set(
          rows.filter((grant) => selection.isSelected(grant.id)).map((grant) => grant.user_id),
        ),
      ),
    [rows, selection],
  );

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Expiring access"
        meta="Direct grants inside the next 30 days, soonest first. The sweep removes each one on its date whether or not you visit this page."
      />

      <Card>
        <CardColumns>
          <span className="w-[26px]">
            <SelectAllCheckbox
              label={
                selection.allSelected
                  ? "Clear the selection"
                  : `Select all ${rows.length} expiring grants`
              }
              {...selection.headerCheckboxProps}
            />
          </span>
          <span className="w-[210px]">Who</span>
          <span className="w-[260px]">What</span>
          <span className="w-[180px]">Granted</span>
          <span className="flex-1">State</span>
          <span className="w-[110px] text-right">Remaining</span>
          <span className="w-[110px] text-right">Action</span>
        </CardColumns>

        <div data-selection-scope {...selection.containerProps}>
        <ListStates
          isLoading={grants.isLoading}
          error={grants.error}
          isEmpty={rows.length === 0}
          onRetry={() => grants.refetch()}
          errorTitle="Couldn't load expiring access."
          skeleton={<RowSkeleton rows={4} label="Loading expiring access" />}
          empty={
            <EmptyState
              title="Nothing expires in the next 30 days."
              guidance="Direct grants appear here a month before their expiry date."
            />
          }
        >
          {rows.map((grant, index) => (
            // Only the soonest row is emphasised. Amber is a deadline signal,
            // not a decoration for the whole table — paint every row and the
            // one that actually needs attention stops standing out.
            <ExpiringRow key={grant.id} grant={grant} soonest={index === 0} selection={selection} />
          ))}
        </ListStates>
        </div>
      </Card>

      <SelectionBar
        count={selection.count}
        noun={["grant", "grants"]}
        composition={
          selectedUsers.length > 0
            ? `${selectedUsers.length} ${selectedUsers.length === 1 ? "person" : "people"}`
            : ""
        }
        onClear={selection.clear}
      >
        <SelectionAction onClick={() => setExtending(true)}>Extend</SelectionAction>
      </SelectionBar>

      {extending && (
        <BulkDialog
          op="extend"
          userIds={selectedUsers}
          scope="with access expiring"
          onClose={() => {
            setExtending(false);
            selection.clear();
          }}
        />
      )}

      <div className="flex flex-wrap gap-[18px]">
        <div className="card min-w-[320px] flex-1 px-5 py-4">
          <h2 className="type-card-title mb-2">Why there&rsquo;s no second button</h2>
          <p className="max-w-[60ch] text-[14px] leading-[1.55] text-muted">
            A &ldquo;Let it lapse&rdquo; or &ldquo;Dismiss&rdquo; control would submit nothing and
            change nothing — but it would make an operator believe they had recorded a decision.
            The row&rsquo;s resting state already says what happens.
          </p>
        </div>
        <div className="min-w-[320px] flex-1 rounded-panel border border-dashed border-line-strong px-5 py-4">
          <h2 className="type-card-title mb-2">Flagged, not assumed</h2>
          <p className="max-w-[60ch] text-[14px] leading-[1.55] text-muted">
            If operators later need to record &ldquo;I&rsquo;ve seen this and I&rsquo;m
            deliberately letting it go&rdquo;, that is a real new capability: an acknowledged state
            with its own column and endpoint. It must not be faked by hiding rows — a shared queue
            that diverges per operator is worse than a noisy one.
          </p>
        </div>
      </div>
    </div>
  );
}

function ExpiringRow({
  grant,
  soonest,
  selection,
}: {
  grant: ExpiringGrantRow;
  soonest: boolean;
  selection: RowSelection;
}) {
  // Extending re-submits the grant with a later date: POST upserts on
  // (user, project, role) and overwrites expires_at, so this renews in place
  // rather than creating a duplicate.
  const extend = useCreateGrant(grant.user_id);
  const remaining = daysUntil(grant.expires_at);

  return (
    <div
      className={`row-divider flex flex-wrap items-center gap-[18px] px-5 py-3.5 ${
        soonest ? "border-l-[3px] border-warn bg-warn-soft" : "border-l-[3px] border-transparent"
      } ${selection.isSelected(grant.id) ? "bg-accent-soft/30" : ""}`}
      {...selection.rowProps(grant.id)}
    >
      <span className="w-[26px]">
        <RowCheckbox label="Select this expiring grant" {...selection.checkboxProps(grant.id)} />
      </span>
      <span className="flex w-[210px] min-w-0 items-center gap-3">
        <UserAvatar id={grant.user_id} />
        <span className="truncate text-[15px] font-semibold">
          <UserName id={grant.user_id} />
        </span>
      </span>

      <div className="w-[260px] shrink-0 truncate text-[14.5px] text-ink/80">
        <ProjectName id={grant.project_id} /> / <Mono>{grant.role_key}</Mono>
      </div>

      <div className="w-[180px] shrink-0 truncate text-[13.5px] text-faint">
        by <UserName id={grant.granted_by} fallback="somebody no longer listed" />,{" "}
        {formatShortDate(grant.created_at)}
      </div>

      <div className="flex min-w-[220px] flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        <span className="truncate text-[14px] text-muted">
          No action — expires {formatShortDate(grant.expires_at)}
        </span>
      </div>

      <div
        className={`w-[110px] shrink-0 text-right text-[13.5px] font-semibold ${
          soonest ? "text-warn-text" : "text-muted"
        }`}
      >
        {remaining === null ? "—" : remaining <= 0 ? "today" : `${remaining} days`}
      </div>

      <div className="w-[110px] shrink-0 text-right">
        <Button
          variant={soonest ? "accent" : "outline"}
          size="sm"
          isPending={extend.isPending}
          onClick={async () => {
            try {
              await extend.mutateAsync({
                project_id: grant.project_id,
                role_key: grant.role_key,
                reason: "Extended from Expiring access",
                duration_days: EXTEND_DAYS,
              });
              toast.success(`Extended by ${EXTEND_DAYS} days.`);
            } catch (error) {
              toast.error(
                error instanceof Error ? error.message : "The extension didn't go through.",
              );
            }
          }}
        >
          Extend
        </Button>
      </div>
    </div>
  );
}

function sortBySoonest(a: ExpiringGrantRow, b: ExpiringGrantRow): number {
  if (!a.expires_at) return 1;
  if (!b.expires_at) return -1;
  return a.expires_at < b.expires_at ? -1 : 1;
}
