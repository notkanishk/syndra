"use client";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import { EmptyState, ListStates } from "@/components/states";
import { PageHeader } from "@/components/ui/PageHeader";
import { useMyStorage } from "@/lib/queries/useMyStorage";

/**
 * A member's page of systems: one card per system, each drawn by the module of
 * its kind (`MemberTargetCard`). Present for every member whatever they can
 * reach, because the page answers "can I get access?", which a missing nav row
 * does not.
 */
export function MySystems() {
  const targets = useMyStorage();

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Network storage"
        lede="Your account on the makerspace's shared storage, and how to connect to it. Your access follows the roles you hold."
      />
      <ListStates
        isLoading={targets.isLoading}
        error={targets.error}
        isEmpty={(targets.data ?? []).length === 0}
        onRetry={() => targets.refetch()}
        errorTitle="Couldn't read your storage access."
        empty={
          <EmptyState
            title="No shared storage yet."
            guidance="This makerspace has not connected any storage to Syndra. When it does, your account and how to reach it appear here."
          />
        }
      >
        <div className="grid gap-4">
          {(targets.data ?? []).map((view) => (
            <MemberTargetCard key={view.target} view={view} />
          ))}
        </div>
      </ListStates>
    </div>
  );
}
