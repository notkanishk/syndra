"use client";

import Link from "next/link";

import { Owner } from "@/components/system/ServiceAccountOwner";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Card, CardColumns, CardHeader, CardRow } from "@/components/ui/Card";
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
 * made for one purpose, not one person. A short list, so no search, sorted by
 * name by the backend.
 *
 * ponytail: the board's "Last used" column and its 90-day stale check are not
 * here. The API can't read last use yet (A-07 §22: boards change to what can be
 * read). Add the column when `GET /service-accounts` carries `last_used_at`.
 */
export default function ServiceAccountsPage() {
  const accounts = useServiceAccounts();
  const rows = accounts.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        eyebrow="System"
        title="Service accounts"
        lede="Accounts made for one purpose, not one person: an integration, a device, an agent, or a login the team shares. Each has someone answerable for it."
      />
      {accounts.data && (
        <ReadFreshness state={readStateOfQuery(accounts)} subject="Service accounts" className="-mb-3.5 px-5" />
      )}

      <Card>
        <CardHeader title="Service accounts" count={accounts.data ? rows.length : undefined} />
        <CardColumns>
          <span className="w-[220px] shrink-0">
            Account <span aria-hidden>↑</span>
          </span>
          <span className="min-w-0 flex-1">Purpose</span>
          <span className="w-[170px] shrink-0">Owner</span>
          <span className="w-[110px] shrink-0 text-right">Roles</span>
        </CardColumns>

        <ListStates
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
          {rows.map((account, index) => (
            <CardRow key={account.id} first={index === 0} data-account-row>
              {/* Phone: name and Roles on the first line, Purpose and Owner stacked beneath. */}
              <span className="order-1 min-w-0 flex-1 tablet:w-[220px] tablet:flex-none">
                <Link
                  href={serviceAccountHref(account.id)}
                  className="block text-[15px] font-semibold [overflow-wrap:anywhere] hover:text-accent-text"
                >
                  {account.name}
                </Link>
                <span className="block text-[13px] text-faint">
                  {kindLabel(account.kind)} · {howItSignsIn(account)}
                </span>
              </span>
              <span className="order-3 w-full min-w-0 text-[14px] text-muted [text-wrap:pretty] tablet:order-2 tablet:w-auto tablet:flex-1">
                {account.purpose}
              </span>
              <span className="order-4 w-full text-[14px] tablet:order-3 tablet:w-[170px] tablet:shrink-0">
                <Owner id={account.owner_id} />
              </span>
              <span className="order-2 w-[64px] shrink-0 text-right text-[14px] tabular-nums tablet:order-4 tablet:w-[110px]">
                {account.role_count}
              </span>
            </CardRow>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
