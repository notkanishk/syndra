"use client";


import { roleCount } from "@/lib/role-mapping-plan";

import { ProjectName } from "@/components/names";
import { EmptyState } from "@/components/states";
import { Card } from "@/components/ui/Card";
import { ApiError } from "@/lib/api-client";
import { MappingManagement } from "@/components/targets/MappingManagement";
import { VersionBand } from "@/components/targets/VersionBand";
import { MetaRow, PageHeader } from "@/components/ui/PageHeader";
import { mappedTo, oneMappedTo } from "@/lib/fields";
import { useFieldSchema, useSystemNames } from "@/lib/queries/useFieldSchema";
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
  const nameOf = useSystemNames();
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  const roster = useTargets();
  const schema = useFieldSchema(target);
  useCrumb("Mappings");

  const reach = mappings.data ? roleCount(mappings.data) : undefined;
  const name = nameOf(target);
  const project = (roster.data ?? []).find((t) => t.target === target)?.project;
  const version = history.data?.current_version;
  // A project Syndra could not name has an empty name, and interpolating it
  // read "holds one of 's roles".
  const owner = project?.name ? `${project.name}'s` : project ? "the project's" : "the system's";
  const noProject = mappings.error instanceof ApiError && mappings.error.code === "PROJECT_REQUIRED";

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Mappings"
        meta={
          <MetaRow>
            {[
              name,
              project ? (
                <span key="project">
                  Zitadel project{" "}
                  <ProjectName id={project.id} name={project.name} className="font-semibold text-ink" />
                </span>
              ) : null,
              reach === undefined ? null : reach === 0 ? "no mappings" : `${reach} ${reach === 1 ? "mapping" : "mappings"}`,
              version ? `version ${version}` : null,
            ]}
          </MetaRow>
        }
        lede={`A mapping gives everyone who holds one of ${owner} roles an account on ${name}, through ${mappedTo(name, schema)}. Changing one moves access for all of them, so every change is previewed before anything is sent.`}
      />
      {noProject ? (
        // Reached by a link before the system has a project: why the project
        // comes first, and the way to choose it. Nothing else renders.
        <Card>
          <EmptyState
            title={`${name} has no Zitadel project yet.`}
            guidance={`A mapping ties one of that project's roles to ${oneMappedTo(name, schema)}, so the project comes first. It is chosen on ${name}'s own page.`}
            action={{ label: `Choose it on ${name}'s page`, href: `/system/targets/${target}` }}
          />
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
