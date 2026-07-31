"use client";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { UserName } from "@/components/names";
import { useIntents } from "@/lib/queries/useIntents";
import { formatRelative } from "@/lib/format";

/**
 * S10 · System › Hardware sync.
 *
 * Parked pending LLDAP. What it needs is an honest "not connected yet" state,
 * not a spinner: a screen that looks like it is loading forever teaches people
 * that the product is broken, when in fact the integration simply does not
 * exist yet.
 *
 * The intent ledger is real and worth showing — those rows are what the sync
 * service will consume when it arrives, and their existence is the evidence
 * that nothing has been lost while waiting.
 */
export default function HardwareSyncPage() {
  const intents = useIntents();
  const rows = intents.data ?? [];
  const waiting = rows.filter((row) => row.status === "pending").length;

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Hardware sync"
        meta="For the door controllers and shop machines that can't speak OIDC."
      />

      <div className="warn-note flex items-start gap-3.5 px-5 py-4">
        <span
          aria-hidden
          className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-warn-soft text-[12px] font-bold text-warn-text"
        >
          !
        </span>
        <div>
          <div className="text-[15px] font-semibold text-warn-text">Not connected yet.</div>
          <p className="mt-1 max-w-[70ch] text-[14px] leading-[1.55] text-muted">
            The sync service that applies these to LLDAP isn&rsquo;t deployed. MkAuth keeps writing
            the intents below, so nothing is being missed — but no hardware group has changed as a
            result of them. {waiting > 0 ? `${waiting} are waiting.` : ""}
          </p>
        </div>
      </div>

      <Card>
        <CardHeader
          title="Provisioning intents"
          count={rows.length}
          note="What the sync service will apply once it exists."
        />
        <ListStates
          isLoading={intents.isLoading}
          error={intents.error}
          isEmpty={rows.length === 0}
          onRetry={() => intents.refetch()}
          errorTitle="Couldn't load provisioning intents."
          skeleton={<RowSkeleton rows={4} avatar={false} label="Loading intents" />}
          empty={
            <EmptyState
              title="No intents recorded."
              guidance="An intent is written whenever access changes for somebody who needs a hardware group."
            />
          }
        >
          {rows.map((intent) => (
            <div key={intent.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3">
              <Badge tone={intent.action === "remove" ? "danger" : "accent"}>{intent.action}</Badge>
              <div className="min-w-[180px] flex-1 truncate text-[14.5px] font-semibold">
                <UserName id={intent.target_uid} />
              </div>
              <Mono className="w-[200px] shrink-0 truncate text-muted">{intent.lldap_group}</Mono>
              <div className="w-[200px] shrink-0 truncate text-[13.5px] text-faint">
                from {intent.source_project} / {intent.source_role}
              </div>
              <Badge tone={intent.status === "failed" ? "danger" : "neutral"}>
                {intent.status}
              </Badge>
              <div className="w-[110px] shrink-0 text-right text-[13px] text-faint">
                {formatRelative(intent.created_at)}
              </div>
            </div>
          ))}
        </ListStates>
      </Card>
    </div>
  );
}
