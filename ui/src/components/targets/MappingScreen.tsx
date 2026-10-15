"use client";

import Link from "next/link";
import { roleCount } from "@/lib/role-mapping-plan";

import { EmptyState } from "@/components/states";
import { Card } from "@/components/ui/Card";
import { ApiError } from "@/lib/api-client";
import { MappingManagement } from "@/components/targets/MappingManagement";
import { VersionBand } from "@/components/targets/VersionBand";
import { PageHeader } from "@/components/ui/PageHeader";
import { targetLabel } from "@/lib/nav";
import { useCrumb } from "@/lib/page-crumb";
import { useMappingHistory, useMappings } from "@/lib/queries/useMappings";

/**
 * What roles reach an add-on, and every published version of that answer
 * (design M1–M6).
 *
 * Its own screen rather than two panels on the target page, and the reason is a
 * fact about reach rather than about page length: editing one mapping moves
 * access for everybody holding that role. A fact with that reach has no room to
 * breathe in a table row.
 *
 * The lede repeats, in the same words, the two sentences the census line on the
 * target page used to bring somebody here — how many mappings reach this target,
 * and that changing one moves everybody who holds the role. The repetition is
 * the handoff: an operator who clicked because of a sentence should find that
 * sentence at the top of what they clicked into, or the click feels like it went
 * somewhere else.
 */
export function MappingScreen({ target }: { target: string }) {
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  useCrumb("Mappings");

  const reach = mappings.data ? roleCount(mappings.data) : undefined;
  const name = targetLabel(target);
  const noProject = mappings.error instanceof ApiError && mappings.error.code === "PROJECT_REQUIRED";

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Mappings"
        lede={
          <>
            A mapping ties a role to a group on {name} — a group is what decides which
            folders an account can open — so everybody holding the role gets an account in
            that group.{" "}
            {reach !== undefined && (
              <>
                <strong className="font-semibold text-ink">
                  {reach === 0
                    ? "No role is mapped yet."
                    : reach === 1
                      ? "One role is mapped."
                      : `${reach} roles are mapped.`}
                </strong>{" "}
              </>
            )}
            Changing one moves access for everybody holding that role, so every change here
            is previewed before it is applied.
          </>
        }
      />

      {noProject ? (
        // Reached by a link before the system has a project: why the project
        // comes first, and the way to choose it. Nothing else renders.
        <Card>
          <EmptyState
            title={`${name} has no Zitadel project yet.`}
            guidance={`A mapping ties one of that project's roles to a ${name} group, so the project comes first. It is chosen on ${name}'s own page.`}
          />
          <div className="px-5 pb-[18px]">
            <Link
              href={`/system/targets/${target}`}
              className="inline-flex min-h-[44px] items-center text-[13.5px] font-semibold text-accent-text desktop:min-h-0"
            >
              Choose it on {name}&apos;s page →
            </Link>
          </div>
        </Card>
      ) : (
        <>
          <VersionBand target={target} history={history.data} />
          <MappingManagement target={target} />
        </>
      )}
    </div>
  );
}
