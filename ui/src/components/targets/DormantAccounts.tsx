"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { AcknowledgeCount } from "@/components/ui/Acknowledge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { ReadFreshness } from "@/components/ui/ReadFreshness";
import { Relative } from "@/components/ui/Time";
import { UserName } from "@/components/names";
import { Input } from "@/components/ui/Input";
import {
  useDormantAccounts,
  useSweepDormant,
  type DormantAccount,
  type SweepResult,
} from "@/lib/queries/useDormant";
import { targetLabel } from "@/lib/nav";

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
export function DormantAccounts({ target }: { target: string }) {
  const dormant = useDormantAccounts(target);
  const remove = useSweepDormant(target);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [acknowledged, setAcknowledged] = useState(false);
  // Held in component state for the length of one submission and never put in
  // the query cache. The add-on holds no delete-capable credential of its own,
  // so this is the only place in the deployment one exists.
  const [elevatedKey, setElevatedKey] = useState("");
  const [result, setResult] = useState<SweepResult | null>(null);

  const accounts = dormant.data?.accounts ?? [];
  const safe = accounts.filter((a) => !a.subject_still_member);
  const lockouts = accounts.filter((a) => a.subject_still_member);
  const chosen = safe.filter((a) => selected.has(a.account));

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
    <Card>
      <CardHeader
        title="Accounts nothing explains any more"
        count={accounts.length}
        note="No active role grants any of these"
      />

      {dormant.data && (
        <div className="px-5 pb-2">
          <ReadFreshness
            subject="This list"
            state={{
              readAt: dormant.data.state_read_at,
              current: true,
              truncated: dormant.data.truncated,
            }}
            onRefresh={() => dormant.refetch()}
            refreshing={dormant.isFetching}
          />
        </div>
      )}

      <ListStates
        isLoading={dormant.isLoading}
        error={dormant.error}
        isEmpty={accounts.length === 0}
        onRetry={() => dormant.refetch()}
        errorTitle="The dormant list could not be read"
        empty={
          <EmptyState
            title="Nothing dormant"
            guidance={`Every account on ${targetLabel(target)} belongs to somebody a role still reaches.`}
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
                  <input
                    type="checkbox"
                    aria-label={`Select ${account.account}`}
                    className="size-4 shrink-0 accent-[var(--accent)]"
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
                title="Still a member, and nothing reaches here"
                blurb="Not housekeeping. Removing one of these locks the person out rather than tidying up — give them a role that grants it, or take the access away deliberately."
              />
              {lockouts.map((account) => (
                // Dashed and unselectable, and the reason is in the row. The
                // bulk count never includes these, so the one bulk action in
                // the product can only ever touch accounts that grant nobody
                // anything.
                <div
                  key={account.account}
                  className="row-divider flex flex-wrap items-center gap-[18px] px-5 py-3.5"
                >
                  <span
                    aria-hidden
                    className="size-4 shrink-0 rounded-[4px] border border-dashed border-line-strong"
                  />
                  <AccountLine account={account} />
                  <span className="w-full pl-[34px] text-[13px] text-warn-text">
                    Cannot be removed here — they are still a member.
                  </span>
                </div>
              ))}
            </>
          )}
        </>
      </ListStates>

      {result && <SweepOutcomes result={result} onDismiss={() => setResult(null)} />}

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
            consequence={filesSentence(chosen)}
          />
          <label className="grid gap-1.5 text-[14px]">
            <span>A credential that may delete accounts</span>
            <Input
              type="password"
              autoComplete="off"
              value={elevatedKey}
              onChange={(e) => setElevatedKey(e.target.value)}
            />
            <span className="text-[13px] text-faint">
              {/* Why the operator is being asked at all, said once. It is the
                  reason a compromise of the add-on cannot destroy anybody's
                  files, and without the explanation it reads as friction. */}
              The add-on holds none of its own — its session can read and write
              accounts and cannot remove one. This is used for these removals and kept
              nowhere.
            </span>
          </label>
          <div className="flex gap-2">
            <Button
              variant="dangerConfirm"
              disabled={!acknowledged || !elevatedKey || remove.isPending}
              onClick={() =>
                remove.mutate(
                  { accounts: chosen.map((a) => a.account), elevatedKey },
                  {
                    onSuccess: (outcome) => {
                      setResult(outcome);
                      setSelected(new Set());
                      setAcknowledged(false);
                      setElevatedKey("");
                    },
                  },
                )
              }
            >
              {remove.isPending
                ? "Removing…"
                : `Remove ${chosen.length} ${chosen.length === 1 ? "account" : "accounts"}`}
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
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
            This one does not queue. Each account is checked again as it goes, and one
            that has stopped being dormant since you read this list is refused rather
            than removed.
          </p>
          {remove.error && (
            <p className="text-[13.5px] text-danger-text">
              {remove.error instanceof Error ? remove.error.message : "That could not be applied."}
            </p>
          )}
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
function filesSentence(chosen: DormantAccount[]): string {
  const known = chosen.filter((a) => typeof a.bytes_held === "number");
  if (known.length === chosen.length && known.length > 0) {
    const total = known.reduce((sum, a) => sum + (a.bytes_held ?? 0), 0);
    return `${formatBytes(total)} of their files goes with the accounts.`;
  }
  return "Their home directories and everything in them go with the accounts. Syndra cannot see how much that is on this target.";
}

function formatBytes(bytes: number): string {
  const units = ["bytes", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${unit === 0 ? value : value.toFixed(1)} ${units[unit]}`;
}

function GroupHeading({ title, blurb }: { title: string; blurb: string }) {
  return (
    <div className="row-divider bg-tint-1 px-5 py-2.5">
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
          <UserName id={account.subject_id} fallback={account.display_name || account.subject_id} />
        </span>
      )}
      <span className="flex-1" />
      <span className="text-[13px] text-faint">
        last seen <Relative iso={account.last_seen_at} />
      </span>
    </>
  );
}

/**
 * What the sweep actually did, per account.
 *
 * Four outcomes and only one of them is "removed". An unconfirmed purge is the
 * one place in the product where retrying is not free — the account may be gone
 * — so it is named rather than folded into a failure count somebody would click
 * again.
 */
function SweepOutcomes({ result, onDismiss }: { result: SweepResult; onDismiss: () => void }) {
  const unresolved = result.outcomes.filter((o) => o.outcome !== "removed");
  return (
    <div role="status" className="grid gap-2 border-t border-line px-5 py-4 text-[13.5px]">
      <p className={result.removed > 0 ? "text-muted" : "text-warn-text"}>
        {result.removed} {result.removed === 1 ? "account" : "accounts"} removed.
      </p>
      {unresolved.map((outcome) => (
        <p key={outcome.account} className="text-warn-text">
          <span className="font-mono text-[13px]">{outcome.account}</span> —{" "}
          {outcome.detail ?? outcome.outcome}
        </p>
      ))}
      <div>
        <Button variant="ghost" size="sm" onClick={onDismiss}>
          Dismiss
        </Button>
      </div>
    </div>
  );
}
