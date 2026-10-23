"use client";

import Link from "next/link";
import { useRef, useState } from "react";

import { AccessSourceList, orderedSources, type RoleReason } from "@/components/access/AccessSource";
import { ResolvedProjectName } from "@/components/names";
import { UserName } from "@/components/names/UserName";
import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import { EmptyState, ErrorState, ListStates, RowSkeleton } from "@/components/states";
import { Owner } from "@/components/system/ServiceAccountOwner";
import { ServiceAccountDialog, UnmarkDialog } from "@/components/system/ServiceAccountDialogs";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { ApiError } from "@/lib/api-client";
import { formatShortDate, humanizeKey } from "@/lib/format";
import { useCrumb } from "@/lib/page-crumb";
import { kindLabel, useServiceAccount } from "@/lib/queries/useServiceAccounts";
import { useUserAccess, useUserGrants } from "@/lib/queries/useUsers";

/**
 * One service account (A-07 §27, route-service-accounts, 23 Oct 2026): a card of
 * settings, then what it holds. It never asks for access and is never
 * welcomed, so this page has neither; nothing is forbidden by kind otherwise.
 */
export function ServiceAccountPage({ id }: { id: string }) {
  const one = useServiceAccount(id);
  const account = one.data;
  useCrumb(account?.name);
  const [dialog, setDialog] = useState<Dialog>(null);
  // Unmarking succeeds by making this page's own read 404, so the dialog outlives the account
  // it shows: it keeps the last account read, and renders beside whichever state follows.
  const last = useRef(account);
  if (account) last.current = account;

  return (
    <>
      <ServiceAccountView id={id} one={one} dialog={dialog} setDialog={setDialog} />
      {dialog === "person" && last.current && <UnmarkDialog account={last.current} onClose={() => setDialog(null)} />}
    </>
  );
}

type Dialog = "purpose" | "owner" | "person" | null;

function ServiceAccountView({
  id,
  one,
  dialog,
  setDialog,
}: {
  id: string;
  one: ReturnType<typeof useServiceAccount>;
  dialog: Dialog;
  setDialog: (dialog: Dialog) => void;
}) {
  const account = one.data;
  if (one.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={4} avatar={false} label="Loading this service account" />
      </Card>
    );
  }
  // The route is for service accounts only: a person's id gets a way to their own page.
  if (one.error instanceof ApiError && one.error.status === 404) {
    return (
      <Card>
        <EmptyState
          title="That isn't a service account."
          guidance="It may be a person, or it may have been marked as one again."
          action={{ label: "Open it on People", href: `/users/${encodeURIComponent(id)}` }}
        />
      </Card>
    );
  }
  if (one.error || !account) {
    return (
      <ErrorState title="Couldn't load this service account." error={one.error} onRetry={() => one.refetch()} />
    );
  }

  const machine = account.zitadel_type === "machine";

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        eyebrow={
          <Link href="/system/service-accounts" className="hover:text-accent-text hover:underline">
            Service accounts
          </Link>
        }
        title={account.name}
        meta={<MetaRow>{[kindLabel(account.kind), machine ? "Machine key" : "Signs in"]}</MetaRow>}
        lede={account.purpose || undefined}
      />

      <Card>
        <CardHeader title="This account" />
        <SettingRow
          first
          label="Purpose"
          action={
            <Button size="sm" onClick={() => setDialog("purpose")}>
              Change purpose
            </Button>
          }
        >
          {account.purpose || <span className="text-faint">Nothing written yet</span>}
        </SettingRow>
        <SettingRow
          label="Owner"
          action={
            <Button size="sm" onClick={() => setDialog("owner")}>
              Change owner
            </Button>
          }
        >
          <Owner id={account.owner_id} />
        </SettingRow>
        <SettingRow
          label="Kind"
          action={
            machine ? (
              <span className="text-[13px] text-muted">Set by Zitadel</span>
            ) : (
              <Button size="sm" onClick={() => setDialog("person")}>
                Mark as a person
              </Button>
            )
          }
        >
          <span className="font-semibold">{kindLabel(account.kind)}</span>
          <span className="block text-[13px] text-faint">
            {machine ? (
              "A machine account in Zitadel: a key, no sign-in"
            ) : (
              <>
                Marked by <UserName id={account.marked_by} fallback="someone" />
                {account.marked_at ? `, ${formatShortDate(account.marked_at)}` : ""}
              </>
            )}
          </span>
        </SettingRow>
      </Card>

      <Holds userId={account.id} name={account.name} />

      {dialog === "purpose" && <ServiceAccountDialog mode="purpose" account={account} onClose={() => setDialog(null)} />}
      {dialog === "owner" && <ServiceAccountDialog mode="owner" account={account} onClose={() => setDialog(null)} />}
    </div>
  );
}

/** Label 170, value, then a right seat of 200: the settings card's one row shape. */
function SettingRow({
  label,
  action,
  first = false,
  children,
}: {
  label: string;
  action: React.ReactNode;
  first?: boolean;
  children: React.ReactNode;
}) {
  return (
    <CardRow first={first}>
      <span className="w-full text-[13.5px] font-semibold text-muted tablet:w-[170px] tablet:shrink-0">{label}</span>
      <span className="min-w-0 flex-1 text-[14.5px] [overflow-wrap:anywhere]">{children}</span>
      <span className="flex w-full tablet:w-[200px] tablet:shrink-0 tablet:justify-end">{action}</span>
    </CardRow>
  );
}

/**
 * What it holds, read like a person's roles: the same access view, the same
 * source line and the same removal dialog. Nothing here is its own model of
 * access.
 */
function Holds({ userId, name }: { userId: string; name: string }) {
  const access = useUserAccess(userId);
  const grants = useUserGrants(userId);
  const [removal, setRemoval] = useState<Removal | null>(null);
  const grantsByRole = new Map((grants.data ?? []).map((grant) => [`${grant.project_id}::${grant.role_key}`, grant]));
  const grantsResolved = !grants.isLoading && !grants.error;

  const rows = (access.data?.projects ?? []).flatMap((project) =>
    project.effective_role_keys.map((roleKey) => {
      const reasons: RoleReason[] = [...project.source_roles, ...project.derived_roles]
        .filter((role) => role.role_key === roleKey)
        .flatMap((role) => role.reasons);
      return { project, roleKey, reasons };
    }),
  );

  return (
    <Card>
      <CardHeader title="What it holds" count={access.data ? rows.length : undefined} />
      <ListStates
        isLoading={access.isLoading}
        error={access.error}
        isEmpty={rows.length === 0}
        onRetry={() => access.refetch()}
        errorTitle="Couldn't load what this account holds."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Loading what this account holds" />}
        empty={<EmptyState title={`${name} holds no access.`} guidance="Access appears here once a bundle or a direct grant gives it some." />}
      >
        {rows.map(({ project, roleKey, reasons }, index) => {
          const sources = orderedSources(reasons);
          const strongest = sources[0];
          const verb = strongest?.queued ? "Withdraw" : strongest?.kind === "direct" ? "Revoke" : "Remove";
          const roleName = humanizeKey(roleKey);
          return (
            <CardRow key={`${project.project_id}::${roleKey}`} first={index === 0}>
              <span className="min-w-0 flex-1 text-[15px] font-semibold tablet:flex-none tablet:basis-[260px]">
                <Link
                  href={`/projects/${encodeURIComponent(project.project_id)}`}
                  className="hover:text-accent-text hover:underline"
                >
                  <ResolvedProjectName
                    name={project.project_name}
                    resolved={project.project_name_resolved}
                    id={project.project_id}
                  />
                </Link>
                <span aria-hidden className="text-faint"> · </span>
                <span>{roleName}</span>
              </span>
              <span className="order-last w-full min-w-0 tablet:order-none tablet:w-auto tablet:flex-1">
                <AccessSourceList reasons={reasons} />
              </span>
              {strongest && (
                <Button
                  size="sm"
                  variant={verb === "Revoke" ? "danger" : "outline"}
                  aria-label={`${verb} ${roleName} in ${project.project_name}`}
                  onClick={() =>
                    setRemoval({
                      projectId: project.project_id,
                      projectName: project.project_name,
                      roleKey,
                      sources,
                      grantId: grantsByRole.get(`${project.project_id}::${roleKey}`)?.id,
                      grantsResolved,
                    })
                  }
                >
                  {verb}
                </Button>
              )}
            </CardRow>
          );
        })}
      </ListStates>
      <RemovalDialog removal={removal} userId={userId} userName={name} onClose={() => setRemoval(null)} />
    </Card>
  );
}
