"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";

import { MemberTargetCard } from "@/components/kinds/MemberTargetCard";
import { EmptyState, ListStates } from "@/components/states";
import { PageHeader } from "@/components/ui/PageHeader";
import { isStorage } from "@/lib/nav";
import { NOTHING_TO_DO } from "@/lib/outcome";
import { useCrumb } from "@/lib/page-crumb";
import { useMySystems } from "@/lib/queries/useMySystems";
import { ReadAgainError, ReadAgainProvider } from "@/components/ui/ReadFreshness";
import { useMyStorage, useMyStorageAgain } from "@/lib/queries/useMyStorage";

/**
 * `/systems/{target}`: one system's card, by its kind's module (the core card
 * for a kind with none). Whether the page exists is GET /me/systems' answer,
 * the same read the rail draws its row from; the card is GET /me/targets'.
 * One address per page: a storage system's is `/storage`, so this sends it
 * there. A system that is not connected, or not read yet (members never see
 * one half-described, owner 11 Nov), is the same page to a member: nothing
 * here, and nothing for them to do. The crumb is the system's name once that
 * read lands, never an id first.
 */
export function MemberSystemPage({ target }: { target: string }) {
  const router = useRouter();
  const systems = useMySystems();
  const system = systems.data?.find((s) => s.target === target);
  const storage = system !== undefined && isStorage(system);
  const here = system !== undefined && !storage;
  const mine = useMyStorage(here);
  const again = useMyStorageAgain();
  const view = here ? mine.data?.find((t) => t.target === target) : undefined;
  useCrumb(here ? system.name : null);

  useEffect(() => {
    if (storage) router.replace("/storage");
  }, [storage, router]);

  if (storage) return null;
  return (
    <div className="flex flex-col gap-4">
      {here && <PageHeader title={system.name} lede="Your account here, and what to do next. Your access follows the roles you hold." />}
      <ListStates
        isLoading={systems.isLoading || (here && mine.isLoading)}
        error={systems.data ? (mine.data ? null : mine.error) : systems.error}
        isEmpty={!view}
        onRetry={() => (systems.data ? mine.refetch() : systems.refetch())}
        errorTitle="Couldn't read your access to this system."
        empty={
          <EmptyState
            title="This system isn't connected here."
            guidance={`${NOTHING_TO_DO} Your connected systems are listed in the menu.`}
            action={{ label: "Go to My access", href: "/" }}
          />
        }
      >
        {/* The page's heading already names the system: the card does not again. */}
        {view && (
          <ReadAgainProvider value={{ press: again.press, since: mine.dataUpdatedAt }}>
            <ReadAgainError error={again.error} />
            <MemberTargetCard view={view} untitled />
          </ReadAgainProvider>
        )}
      </ListStates>
    </div>
  );
}
