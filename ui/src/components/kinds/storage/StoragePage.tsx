"use client";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import { EmptyState, ListStates } from "@/components/states";
import { PageHeader } from "@/components/ui/PageHeader";
import { isStorage } from "@/lib/nav";
import { NOTHING_TO_DO } from "@/lib/outcome";
import { useMyStorage } from "@/lib/queries/useMyStorage";

/**
 * Network storage, the storage kind's page (route `/storage`): one card per
 * system of kind storage, each drawn by `MemberTargetCard`. Every other
 * system Syndra has read has its own row and page, `/systems/{target}`. Present
 * for every member whatever they can reach, because the page answers "can I
 * get access?", which a missing nav row does not.
 */
export function StoragePage() {
  const targets = useMyStorage();
  const storage = targets.data?.filter(isStorage) ?? [];

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Network storage"
        lede="Your account on the makerspace's shared storage, and how to connect to it. Your access follows the roles you hold."
      />
      <ListStates
        isLoading={targets.isLoading}
        error={targets.error}
        isEmpty={storage.length === 0}
        onRetry={() => targets.refetch()}
        errorTitle="Couldn't read your storage access."
        empty={
          <EmptyState
            title="No shared storage to show yet."
            guidance={NOTHING_TO_DO}
          />
        }
      >
        <div className="grid gap-4">
          {storage.map((view) => (
            <MemberTargetCard key={view.target} view={view} />
          ))}
        </div>
      </ListStates>
    </div>
  );
}
