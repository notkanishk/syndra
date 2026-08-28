"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { HoldDialog } from "@/components/review/HoldDialog";
import { TakeAwayDialog } from "@/components/targets/TakeAwayDialog";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { targetLabel } from "@/lib/nav";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useTargetInventory, type BoundAccount } from "@/lib/queries/useTargets";

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
export function PeopleOnTarget({ target }: { target: string }) {
  const inventory = useTargetInventory(target);
  const accounts = inventory.data?.accounts ?? [];
  const name = targetLabel(target);

  return (
    <Card>
      <CardHeader
        // "Bound", not "People with an account here". The region heading above says
        // whose accounts these are; repeating it in every card title spends the
        // width on a sentence the reader has already read.
        title="Have an account here"
        count={inventory.data?.bound}
        note="Put what a role gives them on hold, or revoke it (end their access)"
      />
      <ListStates
        isLoading={inventory.isLoading}
        error={inventory.error}
        isEmpty={accounts.length === 0}
        onRetry={() => inventory.refetch()}
        errorTitle="The account list could not be read"
        empty={
          <EmptyState
            title="Nobody yet"
            guidance={`No role is mapped to ${name} yet, or the changes that would create accounts are still waiting to be sent from Pending changes.`}
          />
        }
      >
        <>
          {accounts.map((account, i) => (
            <PersonRow key={account.subject_id} target={target} account={account} first={i === 0} />
          ))}
        </>
      </ListStates>
    </Card>
  );
}

function PersonRow({
  target,
  account,
  first,
}: {
  target: string;
  account: BoundAccount;
  first: boolean;
}) {
  const [dialog, setDialog] = useState<"hold" | "takeaway" | null>(null);
  // The name, because both dialogs speak to and about a person — and rung 3
  // asks the operator to type it. Through the same resolver every other surface
  // uses, so the name in the dialog is the name in the row: asking somebody to
  // type "Ada Rivera" beside a row that says "Ada" is a puzzle, not a check.
  const resolver = useNameResolver();
  const name = resolver.resolveUser(account.subject_id).value?.display_name ?? account.subject_id;

  return (
    <>
      <CardRow first={first} className="flex-wrap">
        <span className="text-[14px]">
          <UserName id={account.subject_id} />
        </span>
        <span className="font-mono text-[13.5px] text-muted">{account.username}</span>
        {account.account_uid !== undefined && (
          <span className="text-[13px] text-faint">id {account.account_uid}</span>
        )}
        <span className="text-[13px] text-faint">
          account since <Relative iso={account.bound_at} />
        </span>
        <span className="flex-1" />
        {/* Pause it, or end it. The two answers to one question, in that order:
            the reversible one first. */}
        <Button variant="outline" size="sm" onClick={() => setDialog("hold")}>
          Put on hold
        </Button>
        <Button variant="danger" size="sm" onClick={() => setDialog("takeaway")}>
          Revoke
        </Button>
      </CardRow>

      {dialog === "hold" && (
        <HoldDialog
          subjectId={account.subject_id}
          subjectName={name}
          target={target}
          // The lifecycle field, which is what a hold on a whole target holds.
          // A hold on one share is authored from the share's own row on the
          // person's page; this row is about the account.
          //
          // `value` is what the resolver reads — a lifecycle denial is always
          // `enabled = true`, the state being refused — and `label` is what the
          // operator reads. Sending the label would be a malformed term the
          // backend refuses; showing the value would be a dialog saying "Hold
          // true for Ada".
          field="enabled"
          value="true"
          label={`access to ${targetLabel(target)}`}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "takeaway" && (
        <TakeAwayDialog
          target={target}
          subjectId={account.subject_id}
          subjectName={name}
          onClose={() => setDialog(null)}
        />
      )}
    </>
  );
}
