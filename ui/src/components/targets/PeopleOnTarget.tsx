"use client";

import { useEffect, useRef, useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { HoldDialog } from "@/components/review/HoldDialog";
import { ServiceAccountTag, useAccountHref } from "@/components/system/ServiceAccountTag";
import { RevokeDialog } from "@/components/targets/RevokeDialog";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns, CardHeader, CardRow, NUMBER_COL, type Columns, type SeatWidth } from "@/components/ui/Card";
import { LeavingRow, useLeavingRows } from "@/lib/motion";
import { PaginationRow, usePaging } from "@/components/ui/PaginationRow";
import { UNKNOWN_ACCOUNT, UserAvatar, UserName } from "@/components/names";
import { StandingLine } from "@/components/ui/StandingLine";
import { rowStep } from "@/lib/account-standing";
import { accountField } from "@/lib/fields";
import { schemaReason } from "@/components/states/SchemaState";
import { useFieldSchemaState, useMappingFields, useSystemNames } from "@/lib/queries/useFieldSchema";
import { formatShortDate } from "@/lib/format";
import { glideTo } from "@/lib/motion";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useTargetHomes, useTargetInventory, type AccountHome, type BoundAccount } from "@/lib/queries/useTargets";
import { HomeLine, LimitDialog, limitVerb } from "@/components/kinds/storage/HomeLimit";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { ofRead, readOfQuery, notRead } from "@/lib/queries/read";
import { sortByName } from "@/lib/sort";
import { useShownNames } from "@/components/names/shownNames";

/**
 * Whose accounts are on this target (design §21's second question).
 *
 * The inventory beside this one answers which accounts Syndra does NOT manage.
 * This answers the half an operator actually acts on, and it had no surface at
 * all: the bindings were read on every inventory pass, counted, and thrown away.
 *
 * It is also where the two answers to one question live side by side — **pause
 * it or end it.** A hold withholds what a role grants and can be lifted; taking
 * access away composes a hold with a credential rotation and is dressed as the
 * revocation it is. Putting them on the same row is what makes the choice
 * visible; putting them on different screens would make the heavier one the
 * only one an operator remembers.
 */
/**
 * The table's columns, its headings' and every row's: the person, their account and its status
 * sharing the row 1 to 1 to 1.3 (D-1's board, part 2 frame 10, draws the last two 1 to 1.3), and
 * since when. Shares held to a minimum, never a fixed width or a basis: with the person at 220 and
 * bases of 140 and 200 the line came to 1034px, so Put on hold / Revoke dropped under every row up
 * to a 1400px window although the rows fitted at 1280 (staging, 19 Nov). Now 140 + 120 + 140 + 110,
 * the wide seat (a system with a home folder limit has three verbs) and the gaps is 874: one line
 * from a 1220px window, and from 1120 with the default seat.
 */
const COLS: Columns = [
  "min-w-[min(140px,100%)] flex-1",
  "min-w-[min(120px,100%)] flex-1",
  "min-w-[min(140px,100%)] flex-[1.3_1_0%]",
  NUMBER_COL,
];

/** `stale`: the system isn't answering, so this is its last read and the card recedes (A-13). */
export function PeopleOnTarget({ target, stale = false }: { target: string; stale?: boolean }) {
  const nameOf = useSystemNames();
  const inventory = useTargetInventory(target);
  // A read with nothing to give has no count, no list and no empty state (design §13.6): the
  // Accounts region says why, once, above the cards.
  const read = readOfQuery(inventory);
  const homes = readOfQuery(useTargetHomes(target));
  const names = useShownNames();
  const accounts = sortByName(ofRead(read, (d) => d.accounts) ?? [], (a) => names.user(a.subject_id), (a) => a.subject_id);
  const homeOf = new Map((ofRead(homes, (d) => d.accounts) ?? []).map((h) => [h.subject_id, h]));
  const name = nameOf(target);
  const paging = usePaging(undefined, undefined, "people");
  // S5: somebody whose access has been revoked leaves on the next read, fading where they were and
  // closing the gap after, and not under the pointer.
  const leaving = useLeavingRows(accounts, (a) => a.subject_id, !inventory.isLoading && !inventory.error);
  const drawn = leaving.window(paging.limit);
  // The seat is the table's, decided by the system's setup (does it have a home limit?) and never
  // by the rows read: three verbs where it has one, two where it has none (D-1 §5 #18). Wide until the
  // setup is known (or if it can't be read): a seat that narrowed or widened when a read landed would move.
  const setupKnown = Boolean(useFieldSchemaState(target).schema);
  const hasLimit = Boolean(useMappingFields(target).limit);
  const seat: SeatWidth = setupKnown && !hasLimit ? "default" : "wide";

  // `?person=<id>#accounts` from a person's page (A-10 §3): the list opens far
  // enough to hold their row, the page glides there and the row flashes once
  // (Motion 8d). Once per visit.
  const landed = useRef(false);
  const { all: showAll } = paging;
  useEffect(() => {
    const list = (inventory.data && ofRead(inventory.data, (d) => d.accounts)) ?? [];
    if (landed.current || inventory.isLoading || list.length === 0) return;
    const person = new URLSearchParams(window.location.search).get("person");
    landed.current = true;
    const index = person ? list.findIndex((a) => a.subject_id === person) : -1;
    if (index < 0) return;
    if (index >= paging.limit) showAll(list.length);
    requestAnimationFrame(() => {
      const row = document.getElementById(`person-${person}`);
      if (row) void glideTo(row);
    });
  }, [inventory.isLoading, inventory.data, paging.limit, showAll]);

  return (
    <Card stale={stale || read.state === "kept"} {...leaving.listProps}>
      <CardHeader
        title="Accounts Syndra manages"
        count={ofRead(read, (d) => d.bound)}
        note="Put what a role gives them on hold, or revoke it"
      />
      {hasLimit && !notRead(read) && notRead(homes) && (
        // A system with home folders whose accounts were read and whose homes were not: why, in
        // §13.1's words, never homes shown empty as though nobody had one.
        <CardRow>
          <ReadFreshness read={homes} system={name} />
        </CardRow>
      )}
      <ListStates
        read={read}
        error={inventory.error}
        isEmpty={leaving.rows.length === 0}
        onRetry={() => inventory.refetch()}
        errorTitle="The account list could not be read"
        empty={
          <EmptyState
            title="Nobody yet."
            guidance={`No role is mapped to ${name} yet, or the changes that would create accounts are still waiting for you to send from Pending changes.`}
          />
        }
      >
        <>
          {/* Name · description · numbers (8b): the person, the account they
              have there, and since when in the last seat. */}
          <CardColumns cols={COLS} seat={seat}>
            <span>Person</span>
            <span>Account on {name}</span>
            <span>Status</span>
            <span>Since</span>
          </CardColumns>
          {drawn.map((account) => (
            <LeavingRow key={account.subject_id} {...leaving.props(account.subject_id)}>
              <PersonRow
                target={target}
                account={account}
                home={homeOf.get(account.subject_id)}
                seat={seat}
              />
            </LeavingRow>
          ))}
          <PaginationRow
            shown={Math.min(paging.limit, accounts.length)}
            total={accounts.length}
            onMore={paging.more}
            onAll={() => paging.all(accounts.length)}
            onBack={paging.back}
          />
        </>
      </ListStates>
    </Card>
  );
}

function PersonRow({
  target,
  account,
  home,
  seat,
}: {
  target: string;
  account: BoundAccount;
  /** The actions seat's width, shared with the column heads. */
  seat: SeatWidth;
  /** Their home folder limit and where it comes from; absent until read. */
  home?: AccountHome;
}) {
  const nameOf = useSystemNames();
  const [dialog, setDialog] = useState<"hold" | "revoke" | "limit" | null>(null);
  const state = useFieldSchemaState(target);
  const accountName = accountField(state.schema)?.name;
  const why = schemaReason(state, nameOf(target)) ?? (accountName ? undefined : `${nameOf(target)}'s fields name no account, so Syndra cannot say what to hold.`);
  const verb = limitVerb(home);
  // The name, because both dialogs speak to and about a person — and rung 3
  // asks the operator to type it. Through the same resolver every other surface
  // uses, so the name in the dialog is the name in the row: asking somebody to
  // type "Molly Holly" beside a row that says "Molly" is a puzzle, not a check.
  const resolver = useNameResolver();
  const accountHref = useAccountHref();
  const name = resolver.resolveUser(account.subject_id).value?.display_name ?? UNKNOWN_ACCOUNT;
  const step = account.standing && rowStep(account.standing, nameOf(target), "operator");

  return (
    <>
      <CardRow
        id={`person-${account.subject_id}`}
        data-account={account.subject_id}
        className="scroll-mt-4"
        top
        cols={COLS}
        seat={seat}
        // The standing's next step and the home folder line are sentences: the row's detail line, full width
        // under it, never inside the 240px Status column (D-1 §5 #39).
        // A healthy row with nothing to do has no step, so no detail line (D-1 Q5).
        detail={
          (step || home) && (
            <>
              {step && (
                <span className="block" data-standing-step>
                  {step}
                </span>
              )}
              {home && <HomeLine home={home} />}
            </>
          )
        }
        action={
          <>
            {/* Its home folder limit first, the one write place for it (A-03 §3);
                then pause it, or end it, the reversible one first. */}
            {verb && (
              <Button variant="outline" size="sm" onClick={() => setDialog("limit")}>
                {verb}
              </Button>
            )}
            <Button variant="outline" size="sm" disabled={!accountName} reason={why} onClick={() => setDialog("hold")}>
              Put on hold
            </Button>
            <Button variant="danger" size="sm" onClick={() => setDialog("revoke")}>
              Revoke
            </Button>
          </>
        }
      >
        <span className="flex items-center gap-2.5">
          <UserAvatar id={account.subject_id} size="row" />
          <span className="min-w-0">
            <UserName id={account.subject_id} href={accountHref(account.subject_id)} className="block truncate text-body font-semibold text-ink" />
            <ServiceAccountTag id={account.subject_id} />
          </span>
        </span>
        <span className="block [overflow-wrap:anywhere] font-mono text-small text-ink">{account.username}</span>
        {account.standing ? (
          <StandingLine standing={account.standing} system={nameOf(target)} audience="operator" step={false} />
        ) : (
          <span />
        )}
        <span data-figure className="text-small text-muted">{formatShortDate(account.bound_at)}</span>
      </CardRow>
      {dialog === "limit" && home && (
        <LimitDialog target={target} name={name} home={home} onClose={() => setDialog(null)} />
      )}

      {dialog === "hold" && (
        <HoldDialog
          subjectId={account.subject_id}
          subjectName={name}
          target={target}
          // The lifecycle field, which is what a hold on a whole target holds.
          // No hold on one folder is authored anywhere today: the person's page
          // only reads holds (A-10 §5), and single-folder holds are left out
          // until needed (owner, 3 Nov). If they come, they come here, as a
          // choice in this dialog.
          //
          // `value` is what the resolver reads — a lifecycle denial is always
          // `enabled = true`, the state being refused — and `label` is what the
          // operator reads. Sending the label would be a malformed term the
          // backend refuses; showing the value would be a dialog saying "Hold
          // true for Charlotte".
          field={accountName ?? ""}
          value="true"
          label={`access to ${nameOf(target)}`}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "revoke" && (
        <RevokeDialog
          target={target}
          subjectId={account.subject_id}
          subjectName={name}
          onClose={() => setDialog(null)}
        />
      )}
    </>
  );
}
