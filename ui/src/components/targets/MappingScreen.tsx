"use client";

import Link from "next/link";

import { roleCount } from "@/lib/role-mapping-plan";

import { ProjectName } from "@/components/names";
import { EmptyState } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card } from "@/components/ui/Card";
import { ApiError } from "@/lib/api-client";
import { MappingManagement } from "@/components/targets/MappingManagement";
import { VersionBand } from "@/components/targets/VersionBand";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { targetLabel } from "@/lib/nav";
import { useCrumb } from "@/lib/page-crumb";
import { useMappingHistory, useMappings } from "@/lib/queries/useMappings";
import { useTargets } from "@/lib/queries/useTargets";

/**
 * What roles reach an add-on, and every published version of that answer
 * (design M1–M6).
 *
 * Its own screen rather than two panels on the target page, and the reason is a
 * fact about reach rather than about page length: editing one mapping moves
 * access for everybody holding that role. A fact with that reach has no room to
 * breathe in a table row.
 *
 * The header is the board's (route-system-targets-id-mappings): a way back, a
 * subline saying which system, which project, how many mappings and which
 * version, and a two-sentence lede. The sentence that used to carry the count
 * is the subline now, which is where the target page's census line sends
 * somebody to look.
 */
export function MappingScreen({ target }: { target: string }) {
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  const roster = useTargets();
  useCrumb("Mappings");

  const reach = mappings.data ? roleCount(mappings.data) : undefined;
  const name = targetLabel(target);
  const project = (roster.data ?? []).find((t) => t.target === target)?.project;
  const version = history.data?.current_version;
  // A project Syndra could not name has an empty name, and interpolating it
  // read "holds one of 's roles".
  const owner = project?.name ? `${project.name}'s` : project ? "the project's" : "the system's";
  const noProject = mappings.error instanceof ApiError && mappings.error.code === "PROJECT_REQUIRED";

  return (
    <div className="flex flex-col gap-[18px]">
      <div className="flex flex-col gap-1">
        <Link
          href={`/system/targets/${target}`}
          className="inline-flex min-h-11 items-center text-[13.5px] font-semibold text-accent-text desktop:min-h-6 motion-tint hover:text-ink"
        >
          <span aria-hidden>←&nbsp;</span>
          {name}
        </Link>
        <PageHeader
          title="Mappings"
          meta={
            <MetaRow>
              {[
                name,
                project ? (
                  <span key="project">
                    Zitadel project{" "}
                    <Link
                      href={`/projects/${project.id}`}
                      className="font-semibold text-ink motion-tint hover:text-accent-text"
                    >
                      <ProjectName id={project.id} name={project.name} />
                    </Link>
                  </span>
                ) : null,
                reach === undefined ? null : reach === 0 ? "no mappings" : `${reach} ${reach === 1 ? "mapping" : "mappings"}`,
                version ? `version ${version}` : null,
              ]}
            </MetaRow>
          }
          lede={`A mapping gives everyone who holds one of ${owner} roles an account on ${name}, in a group. Changing one moves access for all of them, so every change is previewed before anything is sent.`}
        />
      </div>

      {noProject ? (
        // Reached by a link before the system has a project: why the project
        // comes first, and the way to choose it. Nothing else renders.
        <Card>
          <EmptyState
            title={`${name} has no Zitadel project yet.`}
            guidance={`A mapping ties one of that project's roles to a ${name} group, so the project comes first. It is chosen on ${name}'s own page.`}
          />
          <div className="px-5 pb-[18px]">
            <ArrowLink href={`/system/targets/${target}`}>Choose it on {name}&apos;s page</ArrowLink>
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
