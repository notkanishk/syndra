"use client";

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

  const reach = mappings.data?.length;
  const name = targetLabel(target);

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader title="Mappings" />

      <p className="max-w-[80ch] text-[14.5px] leading-[1.6] text-muted">
        A mapping ties a role to what it reaches on {name}.{" "}
        {reach !== undefined && (
          <>
            <strong className="font-semibold text-ink">
              {reach === 0
                ? "Nothing reaches it."
                : reach === 1
                  ? "One reaches it."
                  : `${reach} reach it.`}
            </strong>{" "}
          </>
        )}
        Changing one moves access for everybody holding that role, which is why every
        change here is rehearsed before it lands.
      </p>

      <VersionBand target={target} history={history.data} />

      <MappingManagement target={target} />
    </div>
  );
}
