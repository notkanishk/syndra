"use client";

import Link from "next/link";

import { Owner } from "@/components/system/ServiceAccountOwner";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Card, CardColumns, CardRow, NUMBER_COL, type Columns } from "@/components/ui/Card";
import { LeavingRow, useLeavingRows } from "@/lib/motion";
import { PageHeader } from "@/components/ui/PageHeader";
import { ReadFreshness, readStateOfQuery } from "@/components/ui/ReadFreshness";
import {
  howItSignsIn,
  kindLabel,
  serviceAccountHref,
  useServiceAccounts,
} from "@/lib/queries/useServiceAccounts";

/**
 * Service accounts (A-07 §26–31, route-service-accounts, 23 Oct 2026): accounts
 * made for one purpose, not one person. A short list, so no search, A to Z
 * by name (useServiceAccounts).
 *
 * ponytail: the board's "Last used" column and its 90-day stale check are not
 * here. The API can't read last use yet (A-07 §22: boards change to what can be
 * read). Add the column when `GET /service-accounts` carries `last_used_at`.
 */
/** Account, purpose, owner, roles. On a phone: name and Roles first, Purpose and Owner beneath. */
const COLS: Columns = [
  "order-1 min-w-[120px] flex-1 tablet:min-w-[160px] tablet:flex-[1_1_0%]",
  "order-3 w-full min-w-0 tablet:order-2 tablet:w-auto tablet:min-w-[160px] tablet:flex-[2_1_0%]",
  "order-4 w-full tablet:order-3 tablet:w-auto tablet:min-w-[120px] tablet:flex-[1_1_0%]",
  `order-2 w-[64px] flex-none text-right tablet:order-4 ${NUMBER_COL}`,
];

export default function ServiceAccountsPage() {
  const accounts = useServiceAccounts();
  const rows = accounts.data ?? [];
  // S5: an account that leaves on a later read (turned back into a person, removed) fades where
  // it was and closes its gap after, and not under the pointer.
  const leaving = useLeavingRows(rows, (account) => account.id, !accounts.isLoading && !accounts.error);

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Service accounts"
        // eslint-disable-next-line no-restricted-syntax -- "shares" is a verb here (a login the team uses together), not the storage kind's word
        lede="Accounts made for one purpose, not one person: an integration, a device, an agent, or a login the team shares. Each has someone answerable for it."
        meta={accounts.data ? `${rows.length} service ${rows.length === 1 ? "account" : "accounts"}` : undefined}
      />
      {accounts.data && <ReadFreshness state={readStateOfQuery(accounts)} subject="This list" />}

      <Card {...leaving.listProps}>
        <ListStates
          head={
            <CardColumns cols={COLS}>
              <span>
                Account <span aria-hidden>↑</span>
              </span>
              <span>Purpose</span>
              <span>Owner</span>
              <span>Roles</span>
            </CardColumns>
          }
          isLoading={accounts.isLoading}
          error={accounts.error}
          isEmpty={rows.length === 0}
          onRetry={() => accounts.refetch()}
          errorTitle="The list of service accounts could not be read."
          skeleton={<RowSkeleton rows={3} avatar={false} label="Loading service accounts" />}
          empty={
            <EmptyState
              title="No service accounts."
              guidance="A machine account in Zitadel appears here by itself. To make one from an ordinary account, open it on People and choose Mark as a service account."
            />
          }
        >
          {leaving.rows.map((account, index) => (
            <LeavingRow key={account.id} {...leaving.props(account.id)}>
            <CardRow cols={COLS} first={index === 0} data-account-row>
              {/* Phone: name and Roles on the first line, Purpose and Owner stacked beneath. */}
              <span>
                <Link
                  href={serviceAccountHref(account.id)}
                  className="block text-body font-semibold [overflow-wrap:anywhere] link-hover"
                >
                  {account.name}
                </Link>
                <span className="block text-small text-faint">
                  {kindLabel(account.kind)} · {howItSignsIn(account)}
                </span>
              </span>
              <span className="text-body text-muted [text-wrap:pretty]">
                {account.purpose || <span className="text-faint">Nothing written yet</span>}
              </span>
              <span className="text-body">
                <Owner id={account.owner_id} />
              </span>
              <span data-figure className="text-body">
                {account.role_count}
              </span>
            </CardRow>
            </LeavingRow>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
