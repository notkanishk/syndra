"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import { EmptyState, ListStates } from "@/components/states";
import { PageHeader } from "@/components/ui/PageHeader";
import { STORAGE_KIND } from "@/lib/nav";
import { NOTHING_TO_DO } from "@/lib/outcome";
import { useMyStorage } from "@/lib/queries/useMyStorage";

/**
 * `/systems/{target}`: one system's card, by its kind's module (the core card
 * for a kind with none). One address per page: a storage system's is
 * `/storage`, so this sends it there. A system that is not connected, or not
 * read yet (members never see one half-described, owner 11 Nov), is the same
 * page to a member: nothing here, and nothing for them to do.
 */
export function MemberSystemPage({ target }: { target: string }) {
  const router = useRouter();
  const mine = useMyStorage();
  const view = mine.data?.find((t) => t.target === target && t.kind);
  const storage = view?.kind === STORAGE_KIND;

  useEffect(() => {
    if (storage) router.replace("/storage");
  }, [storage, router]);

  if (storage) return null;
  return (
    <div className="flex flex-col gap-4">
      {view && <PageHeader title={view.name} lede="Your account here, and what to do next. Your access follows the roles you hold." />}
      <ListStates
        isLoading={mine.isLoading}
        error={mine.data ? null : mine.error}
        isEmpty={!view}
        onRetry={() => mine.refetch()}
        errorTitle="Couldn't read your access to this system."
        empty={
          <EmptyState
            title="This system isn't connected here."
            guidance={`${NOTHING_TO_DO} Your connected systems are listed in the menu.`}
            action={{ label: "Go to My access", href: "/" }}
          />
        }
      >
        {view && <MemberTargetCard view={view} />}
      </ListStates>
    </div>
  );
}
