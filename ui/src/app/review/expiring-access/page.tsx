"use client";

import { useMemo, useState } from "react";

import { toast } from "sonner";

import { AccessSource } from "@/components/access/AccessSource";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
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
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { RoleRef, UserAvatar, UserName } from "@/components/names";
import {
  useAcknowledgeExpiry,
  useClearExpiryAcknowledgement,
  useExpiringGrants,
  type ExpiringGrantRow,
} from "@/lib/queries/useExpiringAccess";
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
export default function ExpiringAccessPage() {
  const grants = useExpiringGrants(30);
  const rows = useMemo(() => [...(grants.data ?? [])].sort(sortBySoonest), [grants.data]);

  // Two groups, not a filter. The cost this screen imposes is rescanning rows somebody has
  // already judged, and a heading is where the eye stops; a filter would hide the same rows
  // behind a control an operator has to remember to check.
  const undecided = useMemo(() => rows.filter((grant) => !grant.acknowledged), [rows]);
  const acknowledged = useMemo(() => rows.filter((grant) => grant.acknowledged), [rows]);

  // Selection spans the undecided rows only. Extending is the one thing worth doing to a dozen at
  // once, and a row already dealt with is not part of the working set.
  const selection = useRowSelection(useMemo(() => undecided.map((grant) => grant.id), [undecided]));
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
                  : `Select all ${undecided.length} undecided expiring grants`
              }
              {...selection.headerCheckboxProps}
            />
          </span>
          <span className="w-[210px]">Who</span>
          <span className="w-[260px]">What</span>
          <span className="w-[180px]">Granted</span>
          <span className="flex-1">State</span>
          <span className="w-[110px] text-right">Remaining</span>
          <span className="w-[200px] text-right">Action</span>
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
              resolved
            />
          }
        >
          {undecided.map((grant, index) => (
            // Only the soonest row is emphasised. Amber is a deadline signal,
            // not a decoration for the whole table — paint every row and the
            // one that actually needs attention stops standing out.
            <ExpiringRow
              key={grant.id}
              grant={grant}
              soonest={index === 0}
              selection={selection}
              onLetLapse={() => setLapsing(grant)}
            />
          ))}

          {/* Everything in the window has been judged. Said outright, because a queue whose only
              rows are acknowledged ones looks identical to a queue nobody has touched. */}
          {undecided.length === 0 && acknowledged.length > 0 && (
            <div className="row-divider px-5 py-3.5 text-[14px] text-muted">
              Nothing here is waiting on a decision. Every grant expiring in the next 30 days has
              been looked at.
            </div>
          )}

          {acknowledged.length > 0 && (
            <>
              <div className="row-divider bg-tint-1 px-5 py-2.5">
                <span className="type-label">
                  Acknowledged · {acknowledged.length}{" "}
                  {acknowledged.length === 1 ? "grant" : "grants"} that will lapse
                </span>
              </div>
              {acknowledged.map((grant) => (
                <ExpiringRow
                  key={grant.id}
                  grant={grant}
                  soonest={false}
                  selection={selection}
                  onLetLapse={() => setLapsing(grant)}
                />
              ))}
            </>
          )}
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
          grantIds={selectedGrantIds}
          scope="with access expiring"
          onClose={() => {
            setExtending(false);
            selection.clear();
          }}
        />
      )}

      {lapsing && <LetItLapseDialog grant={lapsing} onClose={() => setLapsing(null)} />}

      <div className="flex flex-wrap gap-[18px]">
        <div className="card min-w-[320px] flex-1 px-5 py-4">
          <h2 className="type-card-title mb-2">
            What &ldquo;Let it lapse&rdquo; does, and what it doesn&rsquo;t
          </h2>
          <p className="max-w-[60ch] text-[14px] leading-[1.55] text-muted">
            It records that you read the row and decided. It changes nothing about the access — the
            grant already lapses on its date, and it still will. What changes is that the queue
            stops asking your colleagues a question you have answered, with your name on the
            answer.
          </p>
        </div>
        <div className="card min-w-[320px] flex-1 px-5 py-4">
          <h2 className="type-card-title mb-2">When it comes back</h2>
          <p className="max-w-[60ch] text-[14px] leading-[1.55] text-muted">
            An acknowledgement is about one date. If somebody extends or re-grants the access, the
            date you agreed to no longer exists, so the row returns as undecided and asks again —
            you have not signed off on the new date. Nothing else clears it, and there is no timer.
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
  onLetLapse,
}: {
  grant: ExpiringGrantRow;
  soonest: boolean;
  selection: RowSelection;
  onLetLapse: () => void;
}) {
  const ack = grant.acknowledged ?? null;
  // Extending re-submits the grant with a later date: POST upserts on
  // (user, project, role) and overwrites expires_at, so this renews in place
  // rather than creating a duplicate.
  const extend = useCreateGrant(grant.user_id);
  const clearAck = useClearExpiryAcknowledgement();
  const remaining = daysUntil(grant.expires_at);

  return (
    <div
      className={`row-divider flex flex-wrap items-center gap-[18px] px-5 py-3.5 ${
        soonest ? "border-l-[3px] border-warn bg-warn-soft" : "border-l-[3px] border-transparent"
      } ${ack ? "opacity-[.72]" : ""} ${selection.isSelected(grant.id) ? "bg-accent-soft/30" : ""}`}
      {...(ack ? {} : selection.rowProps(grant.id))}
    >
      <span className="w-[26px]">
        {/* No checkbox on a row somebody has dealt with — it is not part of the working set, and
            offering it back into a bulk extend would undo a decision by accident. */}
        {!ack && (
          <RowCheckbox label="Select this expiring grant" {...selection.checkboxProps(grant.id)} />
        )}
      </span>
      <span className="flex w-[210px] min-w-0 items-center gap-3">
        <UserAvatar id={grant.user_id} />
        <span className="truncate text-[15px] font-semibold">
          <UserName id={grant.user_id} />
        </span>
      </span>

      <div className="w-[260px] shrink-0 truncate text-[14.5px] text-ink/80">
        <RoleRef projectId={grant.project_id} roleKey={grant.role_key} />
      </div>

      <div className="w-[180px] shrink-0 truncate text-[13.5px] text-faint">
        by <UserName id={grant.granted_by} fallback="somebody no longer listed" />,{" "}
        {formatShortDate(grant.created_at)}
      </div>

      <div className="flex min-w-[220px] flex-1 items-center gap-3">
        <AccessSource kind="direct" />
        {ack ? (
          // Who and when, always. The point of a shared acknowledgement is that the next person
          // can see whose judgement they would be overriding.
          <span className="min-w-0 truncate text-[14px] text-muted">
            <span className="font-semibold text-ink/80">
              <UserName id={ack.by} fallback={ack.by} />
            </span>{" "}
            let this lapse on {formatShortDate(ack.at)}
            {ack.note ? ` — ${ack.note}` : ""}
          </span>
        ) : (
          <span className="truncate text-[14px] text-muted">
            No action — expires {formatShortDate(grant.expires_at)}
          </span>
        )}
      </div>

      <div
        className={`w-[110px] shrink-0 text-right text-[13.5px] font-semibold ${
          soonest ? "text-warn-text" : "text-muted"
        }`}
      >
        {remaining === null ? "—" : remaining <= 0 ? "today" : `${remaining} days`}
      </div>

      <div className="flex w-[200px] shrink-0 items-center justify-end gap-2">
        {/* Extend stays on an acknowledged row. Changing your mind toward KEEPING somebody's
            access is the reversal that must never be harder than the one that lets it go. */}
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

        {ack ? (
          <Button
            variant="ghost"
            size="sm"
            isPending={clearAck.isPending}
            onClick={async () => {
              try {
                await clearAck.mutateAsync(grant.id);
                toast.success("Back in the queue.");
              } catch (error) {
                toast.error(error instanceof Error ? error.message : "That didn't go through.");
              }
            }}
          >
            Undo
          </Button>
        ) : (
          <Button variant="ghost" size="sm" onClick={onLetLapse}>
            Let it lapse
          </Button>
        )}
      </div>
    </div>
  );
}

/**
 * The acknowledgement dialog. A dialog rather than a click for two reasons: the note is the part
 * that makes the record useful to somebody else, and this is the one place to state plainly that
 * recording a decision is not the same as changing anything.
 */
function LetItLapseDialog({
  grant,
  onClose,
}: {
  grant: ExpiringGrantRow;
  onClose: () => void;
}) {
  const acknowledge = useAcknowledgeExpiry();
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
            <UserName id={grant.user_id} />, expiring {formatShortDate(expiresAt)}.
          </>
        }
      />

      <div className="px-6">
        <div className="accent-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1">What this does</div>
          <ul className="flex flex-col gap-1 text-muted">
            <li>
              Nothing to the access. It already lapses on {formatShortDate(expiresAt)} and it still
              will — this records that you looked.
            </li>
            <li>The queue stops asking, and shows your name against the decision.</li>
            <li>
              If somebody extends or re-grants it, this stops applying and the row comes back — you
              have not signed off on a date that did not exist yet.
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

      <ModalFooter note="Extend instead if the access should continue.">
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
              toast.success("Recorded. It still lapses on its date.");
              onClose();
            } catch (error) {
              // A 409 here is the reopen rule arriving early: the grant was extended while this
              // dialog was open, so the date on screen is not the date the grant has. The message
              // is the server's, which says to reload.
              toast.error(
                error instanceof Error ? error.message : "That didn't go through.",
              );
            }
          }}
        >
          Record it
        </Button>
        <Button variant="ghost" onClick={onClose} disabled={acknowledge.isPending}>
          Cancel
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
