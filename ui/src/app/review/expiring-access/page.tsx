"use client";

import { useMemo, useState } from "react";


import { AccessSource } from "@/components/access/AccessSource";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { BulkDialog } from "@/components/people/BulkDialog";
import { Card, CardColumns } from "@/components/ui/Card";
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
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
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

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Expiring access"
        lede="Direct grants that end in the next 30 days, soonest first. Syndra ends each one on its date whether or not you visit this page."
        actions={
          undecided.length > 0 ? (
            <SelectModeToggle active={selecting} onToggle={() => setSelecting((on) => !on)} />
          ) : undefined
        }
      />

      <Card>
        <CardColumns>
          {selecting && <span className="w-11 shrink-0 desktop:w-[26px]" />}
          <span className="w-[210px]">Who</span>
          <span className="w-[260px]">What</span>
          <span className="w-[180px]">Granted</span>
          <span className="flex-1">State</span>
          <span className="w-[110px] text-right">Remaining</span>
          <span className="w-[200px] text-right">Action</span>
        </CardColumns>

        {/* Only the undecided rows can be selected, so the count here is the
            undecided count and never the number of rows on screen. */}
        {selecting && undecided.length > 0 && (
          <SelectAllRow
            inScope={undecided.length}
            total={rows.length}
            noun={["role", "roles"]}
            allSelected={selection.allSelected}
            {...selection.headerCheckboxProps}
          />
        )}

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
              guidance="Direct grants appear here a month before they end."
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
              selecting={selecting}
              selection={selection}
              onLetLapse={() => setLapsing(grant)}
            />
          ))}

          {/* Everything in the window has been judged. Said outright, because a queue whose only
              rows are acknowledged ones looks identical to a queue nobody has touched. */}
          {undecided.length === 0 && acknowledged.length > 0 && (
            <div className="row-divider px-5 py-3.5 text-[14px] text-muted">
              Nothing here is waiting on a decision. Every role that ends in the next 30 days has
              been looked at.
            </div>
          )}

          {acknowledged.length > 0 && (
            <>
              <div className="row-divider bg-tint-1 px-5 py-2.5">
                <span className="type-label">
                  Let lapse · {acknowledged.length}{" "}
                  {acknowledged.length === 1 ? "role" : "roles"}
                </span>
              </div>
              {acknowledged.map((grant) => (
                <ExpiringRow
                  key={grant.id}
                  grant={grant}
                  soonest={false}
                  selecting={selecting}
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
            access already ends on its date, and it still will. What changes is that the queue
            stops asking your colleagues a question you have answered, with your name on the
            answer.
          </p>
        </div>
        <div className="card min-w-[320px] flex-1 px-5 py-4">
          <h2 className="type-card-title mb-2">When a row comes back</h2>
          <p className="max-w-[60ch] text-[14px] leading-[1.55] text-muted">
            An acknowledgement is about one date. If somebody extends the access or gives it again, the
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
  const clearAck = useClearExpiryAcknowledgement();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const remaining = daysUntil(grant.expires_at);

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
      } ${ack ? "opacity-[.72]" : ""} ${selection.isSelected(grant.id) ? "bg-accent-soft/30" : ""}`}
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
      <span className="flex w-full min-w-0 items-center gap-3 tablet:w-[210px]">
        <UserAvatar id={grant.user_id} />
        <span className="truncate text-[15px] font-semibold">
          <UserName id={grant.user_id} />
        </span>
      </span>

      <div className="w-full text-[14.5px] text-ink/80 tablet:w-[260px] tablet:shrink-0 tablet:truncate">
        <RoleRef projectId={grant.project_id} roleKey={grant.role_key} />
      </div>

      <div className="hidden w-[180px] shrink-0 truncate text-[13.5px] text-faint tablet:block">
        by <UserName id={grant.granted_by} fallback="somebody no longer listed" />,{" "}
        {formatShortDate(grant.created_at)}
      </div>

      <div className="flex w-full flex-wrap items-center gap-3 tablet:min-w-[220px] tablet:flex-1">
        <AccessSource kind="direct" />
        {ack ? (
          // Who and when, always. The point of a shared acknowledgement is that the next person
          // can see whose judgement they would be overriding.
          <span className="min-w-0 text-[14px] text-muted tablet:truncate">
            <span className="font-semibold text-ink/80">
              <UserName id={ack.by} fallback={ack.by} />
            </span>{" "}
            let this lapse on {formatShortDate(ack.at)}
            {ack.note ? ` — ${ack.note}` : ""}
          </span>
        ) : (
          <span className="text-[14px] text-muted tablet:truncate">
            Undecided — ends {formatShortDate(grant.expires_at)}
          </span>
        )}
      </div>

      <div
        className={`text-[13.5px] font-semibold tablet:w-[110px] tablet:shrink-0 tablet:text-right ${
          soonest ? "text-warn-text" : "text-muted"
        }`}
      >
        {remaining === null ? "—" : remaining <= 0 ? "ends today" : `${remaining} days left`}
        <span className="tablet:hidden" />
      </div>

      <div className="flex w-full items-center gap-3 tablet:w-[200px] tablet:shrink-0 tablet:justify-end tablet:gap-2">
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
              setOutcome({
                kind: "applied",
                message: `Extended by ${EXTEND_DAYS} days`,
                detail: "It leaves this list when the page next refreshes.",
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

      {/* The row reports its own extension and keeps its seat: it leaves this
          queue on the next read, not under the thumb that extended it. */}
      {outcome && <ActionOutcome outcome={outcome} placement="inline" className="w-full" />}
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
              If somebody extends it or gives it again, this stops applying and the row comes back — you
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

      {/* The dialog states what it did, and this is the one screen where that
          sentence matters most: recording a decision changes nothing about the
          access, and the dialog exists to say so. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

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
              setOutcome({
                kind: "applied",
                message: "Recorded — it will lapse on its date",
                detail:
                  "Nothing about the access changed — it still lapses on its date. What changed is that the queue stops asking your colleagues a question you have answered.",
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
