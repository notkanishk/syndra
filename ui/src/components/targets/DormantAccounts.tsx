"use client";

import { useSystemNames } from "@/lib/queries/useFieldSchema";

import { NOTHING_TO_DO, outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { formatBytes } from "@/lib/format";
import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { EmptyState, ListStates } from "@/components/states";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
import { RowCheckbox } from "@/components/ui/SelectionBar";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { ReadAgainError, ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import {
  useDormantAccounts,
  useSweepDormant,
  type DormantAccount,
  type SweepResult,
} from "@/lib/queries/useDormant";
import { cooldownOf, pageLoad, useRetryOnce } from "@/lib/queries/fresh";
import { retryingLine, STOP_WAITING } from "@/components/results/copy";
import { sortByName } from "@/lib/sort";
import { oneShot } from "@/lib/secret";

/**
 * Accounts Syndra created whose reason for existing has gone (9.11/9.12;
 * design §29).
 *
 * **This is the only bulk action in the product, and the exception is
 * principled.** Nothing else offers one because every revoke removes real
 * access from a real person, and a bulk revoke would be guessing at forty of
 * them. No active role grants any of these accounts, so removing them takes
 * access from nobody: this is tidying forty things that already grant nothing.
 * It is not permission to add bulk actions elsewhere.
 *
 * Grouped by cause, because each cause has a different remedy. A former
 * member's account is housekeeping; *still a member, role deleted* is somebody
 * who may be quietly locked out — same dormancy, opposite action — so those rows
 * are not selectable at all and say why.
 */
/** `stale`: the system isn't answering, so this is its last read and the card recedes (A-13). */
export function DormantAccounts({ target, stale = false }: { target: string; stale?: boolean }) {
  const nameOf = useSystemNames();
  const dormant = useDormantAccounts(target);
  const remove = useSweepDormant(target);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [acknowledged, setAcknowledged] = useState(false);
  // Held in component state for the length of one submission and never put in
  // the query cache. The add-on holds no delete-capable credential of its own,
  // so this is the only place in the deployment one exists.
  const [elevatedKey, setElevatedKey] = useState("");
  // One result slot, cleared by the press before it writes (A-22 G23).
  const [result, setResult] = useState<{ outcome: Outcome; word: string } | null>(null);

  const accounts = dormant.data?.accounts ?? [];
  // The list always reads the target itself, so a load inside the cooldown is refused (429, nothing was read).
  // That is said once, by the freshness line and its waiting button: no other block, and no "nothing was changed".
  // Amber only when the reader's own press ran into it, not a page load inside someone else's wait.
  const [pressedAt, setPressedAt] = useState(0);
  const cooldown = cooldownOf(dormant.error, dormant.errorUpdatedAt);
  const refused = pressedAt > 0 && dormant.errorUpdatedAt >= pressedAt ? cooldown : pageLoad(cooldown);
  const name = nameOf(target);
  const byName = (list: DormantAccount[]) => sortByName(list, (a) => a.account, (a) => a.account);
  const safe = byName(accounts.filter((a) => !a.subject_still_member));
  const lockouts = byName(accounts.filter((a) => a.subject_still_member));
  const chosen = safe.filter((a) => selected.has(a.account));
  // A removal refused inside the add-on's cooldown waits out its retry_after and is sent once more
  // by itself (RFC 9110 §10.2.3), so one press removes on a live read (owner Q1). The key rides in
  // that wait's closure, no longer than it already sits in the field, and a fresh box is made for
  // the second send. A second refusal is shown as it is.
  const retry = useRetryOnce();
  // The refusal a wait is holding back: Stop waiting shows it, so there is always a next step.
  const [held, setHeld] = useState<unknown>(null);
  function stopWaiting() {
    retry.cancel();
    setResult({ outcome: { ...outcomeFromError(held), retryAfter: retry.left }, word: "Not removed" });
  }
  function removeNow(accounts: string[], key: string) {
    setResult(null);
    remove.mutate(
      { accounts, elevatedKey: oneShot(key) },
      {
        onSuccess: (outcome) => {
          setResult(sweepResult(outcome, name));
          setSelected(new Set());
          setAcknowledged(false);
          setElevatedKey("");
        },
        onError: (error) => {
          const cooled = cooldownOf(error);
          if (cooled && retry.refused(cooled.retry_after, () => removeNow(accounts, key))) {
            setHeld(error);
            return;
          }
          setResult({ outcome: outcomeFromError(error), word: "Not removed" });
        },
      },
    );
  }

  function toggle(account: string) {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(account)) next.delete(account);
      else next.add(account);
      return next;
    });
    setAcknowledged(false);
  }

  return (
    <Card stale={stale}>
      <CardHeader
        title="No longer needed"
        count={accounts.length}
        note="Accounts Syndra created that no active role needs any more"
      />

      {(dormant.data || refused) && (
        <div className="px-5 pb-2">
          <ReadFreshness
            subject="This list"
            state={{
              readAt: dormant.data?.state_read_at,
              current: dormant.data ? true : undefined,
              truncated: dormant.data?.truncated,
              stale: refused,
            }}
            onRefresh={() => {
              setPressedAt(Date.now());
              return dormant.refetch();
            }}
            refreshing={dormant.isFetching}
          />
        </div>
      )}

      {/* A press that failed for any other reason says so beside the press and keeps the list on hand. */}
      {dormant.data && <ReadAgainError error={dormant.error} />}

      {!dormant.data && refused ? null : (
      <ListStates
        isLoading={dormant.isLoading}
        // A refusal is the line's to say, never a reason to take the list away while it is on hand.
        error={refused || dormant.data ? null : dormant.error}
        isEmpty={accounts.length === 0}
        onRetry={() => dormant.refetch()}
        errorTitle="This list could not be read"
        empty={
          <EmptyState
            title="Nothing to tidy."
            guidance={`Every account Syndra created on ${name} is still backed by a role.`}
          />
        }
      >
        <>
          {safe.length > 0 && (
            <>
              <GroupHeading
                title="Their membership ended"
                blurb="Housekeeping. Removing these takes access from nobody, because nobody is left to take it from."
              />
              {safe.map((account) => (
                <CardRow key={account.account} className="flex-wrap">
                  {/* The same 24px glyph in a 44px box every other selectable
                      list uses. This one had its own 16px input, which is the
                      smallest target in the product in front of the most
                      destructive action in it. */}
                  <RowCheckbox
                    label={`Select ${account.account}`}
                    checked={selected.has(account.account)}
                    onChange={() => toggle(account.account)}
                  />
                  <AccountLine account={account} />
                </CardRow>
              ))}
            </>
          )}

          {lockouts.length > 0 && (
            <>
              <GroupHeading
                title="Still a member, but no role gives them access here"
                blurb="Not housekeeping. Removing one of these locks the person out rather than tidying up. Give them a role that is mapped here, or revoke their access deliberately from the list above."
              />
              {lockouts.map((account) => (
                // Unselectable, and the heading above says why. The bulk count
                // never includes these, so the one bulk action in the product
                // can only ever touch accounts that grant nobody anything.
                <CardRow key={account.account}>
                  <AccountLine account={account} />
                </CardRow>
              ))}
            </>
          )}
        </>
      </ListStates>
      )}

      {result && (
        <div className="border-t border-line px-5 py-4">
          <ActionOutcome outcome={result.outcome} word={result.word} />
        </div>
      )}

      {chosen.length > 0 && (
        <div className="grid gap-3 border-t border-line px-5 py-4">
          {/* The acknowledgement names the DATA, not the people. Removing the
              row is not what cannot be undone — the files are. */}
          <AcknowledgeCount
            checked={acknowledged}
            onChange={setAcknowledged}
            count={chosen.length}
            noun="accounts"
            verb="removes"
            consequence={filesSentence(chosen, name)}
          />
          <div>
            <FieldLabel htmlFor="sweep-credential">
              A {name} API key that is allowed to delete accounts
            </FieldLabel>
            <Input
              id="sweep-credential"
              type="password"
              autoComplete="off"
              value={elevatedKey}
              onChange={(e) => setElevatedKey(e.target.value)}
            />
            <FieldHint>
              {/* Why the operator is being asked at all, said once. It is the
                  reason a compromise of the add-on cannot destroy anybody's
                  files, and without the explanation it reads as friction. */}
              Syndra&rsquo;s own {name} API key can create and edit accounts but, on purpose,
              not delete them. Paste a key that can. It is used for this removal only and never
              stored.
            </FieldHint>
          </div>
          <div className="flex gap-2">
            <Button
              variant="danger"
              size="sm"
              disabled={!acknowledged || !elevatedKey || retry.waiting}
              isPending={remove.isPending}
              reason={
                retry.waiting
                  ? retryingLine(name, retry.left)
                  : !remove.isPending
                  ? !acknowledged
                    ? "Check the box above to confirm you understand what this removes."
                    : !elevatedKey
                      ? `Paste a ${name} API key that is allowed to delete accounts.`
                      : undefined
                  : undefined
              }
              onClick={() => retry.press(() => removeNow(chosen.map((a) => a.account), elevatedKey))}
            >
              {`Remove ${chosen.length} ${chosen.length === 1 ? "account" : "accounts"}`}
            </Button>
            {retry.waiting && (
              <Button variant="outline" size="sm" onClick={stopWaiting}>
                {STOP_WAITING}
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                // Nothing chosen is nothing to send: a wait for one stops with it.
                retry.cancel();
                setSelected(new Set());
                setAcknowledged(false);
              }}
            >
              Clear selection
            </Button>
          </div>
          <p className="text-[13px] text-faint">
            {/* Not queued, and said plainly, because everything else on these
                screens is. A purge is a one-shot operation: it cannot be
                rehearsed against a queue an operator could still inspect. */}
            This happens at once, not from Pending changes. Each account is checked again as
            it goes; one that has gained a role since you loaded this list is skipped, not
            removed.
          </p>
        </div>
      )}
    </Card>
  );
}

/**
 * What actually goes with the accounts.
 *
 * Says the size when the target reported one and says so plainly when it did
 * not. A sentence claiming "0 bytes" from a read that never happened would be
 * the one part of this ceremony that is not true — and it is the part the whole
 * ceremony is about.
 */
function filesSentence(chosen: DormantAccount[], name: string): string {
  const known = chosen.filter((a) => typeof a.bytes_held === "number");
  if (known.length === chosen.length && known.length > 0) {
    const total = known.reduce((sum, a) => sum + (a.bytes_held ?? 0), 0);
    return `${formatBytes(total)} of their files goes with the accounts.`;
  }
  return `Their home directories and everything in them go with the accounts. Syndra cannot see how much that is on ${name}.`;
}

function GroupHeading({ title, blurb }: { title: string; blurb: string }) {
  return (
    <div className="row-divider px-5 py-2.5">
      <p className="text-[13px] font-semibold text-ink">{title}</p>
      <p className="text-[13px] text-muted">{blurb}</p>
    </div>
  );
}

function AccountLine({ account }: { account: DormantAccount }) {
  return (
    <>
      <span className="font-mono text-[13.5px]">{account.account}</span>
      {account.subject_id && (
        <span className="text-[13.5px] text-muted">
          <UserName id={account.subject_id} fallback={account.display_name || undefined} />
        </span>
      )}
      <span className="flex-1" />
      <span className="text-[13px] text-faint">
        last signed in <Relative iso={account.last_seen_at} />
      </span>
    </>
  );
}

/**
 * What the sweep did, as one result (A-22): the act's word and its "Not", one
 * sentence per account that was not removed, and the step left.
 *
 * Four outcomes and only one of them is "removed". An unconfirmed purge is the
 * one place in the product where retrying is not free (the account may be
 * gone), so it is named, and its step is to check first.
 */
export function sweepResult(result: SweepResult, name: string): { outcome: Outcome; word: string } {
  const unresolved = result.outcomes.filter((o) => o.outcome !== "removed");
  const fallback: Record<string, string> = {
    refused: "refused. It gained a role since you loaded this list, so it was left alone",
    unreached: `${name} did not answer, so it was not removed`,
    indeterminate: `not confirmed. Check ${name} before trying again`,
  };
  const removed = `${result.removed} ${result.removed === 1 ? "account" : "accounts"} removed.`;
  if (unresolved.length === 0) {
    return { outcome: { kind: "applied", message: removed, next: NOTHING_TO_DO }, word: "Removed" };
  }
  const why = unresolved.map((o) => `${o.account}: ${o.detail ?? fallback[o.outcome] ?? "recorded"}.`.replace(/\.\.$/, "."));
  const next = unresolved.some((o) => o.outcome === "indeterminate")
    ? `Check ${name} before you try again.`
    : "Refresh this list to see what is left.";
  return {
    outcome: { kind: "failed", recorded: true, message: [removed, ...why].join(" "), next },
    word: result.removed > 0 ? `Removed ${result.removed} of ${result.outcomes.length}` : "Not removed",
  };
}
