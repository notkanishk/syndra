"use client";

import Link from "next/link";
import { useState } from "react";

import { HomeLocationRow } from "@/components/kinds/storage/HomeLocation";
import { SettingRow } from "@/components/targets/SettingRow";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardHeaderLink } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { describeFailure, sentence } from "@/lib/outcome";
import { mappedTo } from "@/lib/fields";
import { useFieldSchema, useSystemNames } from "@/lib/queries/useFieldSchema";
import { useMappingHistory, useMappings } from "@/lib/queries/useMappings";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useProjects } from "@/lib/queries/useProjects";
import { useSetTargetProject, useTargets } from "@/lib/queries/useTargets";
import { projectLabel } from "@/components/names/Unresolved";
import { sortByName } from "@/lib/sort";

/**
 * A system's one Zitadel project (3a, frames `noproject` and `projectset`).
 *
 * Its roles decide who gets an account there, so it comes before anything
 * that gives access. A project serves one system: one already in use is
 * listed, disabled and named, rather than hidden.
 */

export function ProjectStep({ target, onSet }: { target: string; onSet: (name: string) => void }) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const schema = useFieldSchema(target);
  const projects = useProjects();
  const roster = useTargets();
  const set = useSetTargetProject(target);
  const [chosen, setChosen] = useState("");

  const usedBy = new Map<string, string>();
  for (const t of roster.data ?? []) {
    if (t.target !== target && t.project) usedBy.set(t.project.id, nameOf(t.target));
  }
  const rows = sortByName(projects.data ?? [], (r) => r.project.name, (r) => r.project.id);

  return (
    <section id="project" aria-labelledby="project-title" className="scroll-mt-4">
      <Card>
        <CardHeader
          title={<span id="project-title">Choose {name}&apos;s Zitadel project</span>}
          note="Step 1 of 2"
        />
        <div className="flex flex-col gap-4 px-5 pb-5">
          <p className="max-w-[72ch] text-[14.5px] leading-[1.55] text-ink [text-wrap:pretty]">
            Its roles decide who gets an account on {name}, so nothing below can give anyone access until it
            is set. Next, you map those roles to {mappedTo(name, schema)}.
          </p>
          <form
            className="flex flex-wrap items-center gap-x-3 gap-y-2.5"
            onSubmit={(e) => {
              e.preventDefault();
              const project = rows.find((r) => r.project.id === chosen)?.project;
              if (project) set.mutate(project.id, { onSuccess: () => onSet(project.name) });
            }}
          >
            <div className="min-w-[220px] flex-[0_1_360px]">
              <Select aria-label="Zitadel project" value={chosen} onChange={(e) => setChosen(e.target.value)}>
                <option value="">{projects.isLoading ? "Reading Zitadel's projects…" : "Choose a project…"}</option>
                {rows.map(({ project }) => {
                  const user = usedBy.get(project.id);
                  const roles = project.roles.length;
                  return (
                    <option key={project.id} value={project.id} disabled={Boolean(user)}>
                      {project.name} ·{" "}
                      {user ? `used by ${user}` : roles === 0 ? "no roles yet" : `${roles} role${roles === 1 ? "" : "s"}`}
                    </option>
                  );
                })}
              </Select>
            </div>
            <span className="flex-none whitespace-nowrap">
              <Button type="submit" variant="accent" size="sm" disabled={!chosen} isPending={set.isPending}>
                Use this project
              </Button>
            </span>
          </form>
          <span className="text-[13px] leading-[1.5] text-muted [text-wrap:pretty]">
            A project serves one system, so one already in use can&apos;t be picked. Changing it later means
            removing its mappings first.
          </span>
          {set.error && <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(set.error))}</p>}
        </div>
      </Card>
    </section>
  );
}

/** The step's result, in its place, for this visit only. */
export function ProjectSet({ target, project }: { target: string; project: string }) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const schema = useFieldSchema(target);
  return (
    <div className="flex flex-col gap-3">
      <ActionOutcome
        word="Chosen"
        outcome={{
          kind: "applied",
          message: `${name} uses ${project}.`,
          detail: `Nobody's access changed: no role reaches ${name} yet. Map ${project}'s roles to ${mappedTo(name, schema)} to give people accounts there.`,
        }}
      />
      <ArrowLink href={`/system/targets/${target}/mappings`} className="self-start whitespace-nowrap">
        Map its roles to {mappedTo(name, schema)}
      </ArrowLink>
    </div>
  );
}

/**
 * The system's settings, once the project is set (A-03 §1): one row per
 * setting, the label in a 170px seat, the value, then its way on in the 200px
 * right seat. The only link to the mappings, and the one place the home
 * location is chosen.
 */
export function SetupStrip({
  target,
  project,
  lastKnown = false,
}: {
  target: string;
  project: { id: string; name: string };
  /** The system isn't answering: a setting that is chosen from what the system lists waits (A-13 §1.5). */
  lastKnown?: boolean;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const schema = useFieldSchema(target);
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  const resolver = useNameResolver();
  const roles = new Set((mappings.data ?? []).map((m) => m.role_key)).size;
  const version = history.data?.current_version;

  return (
    <Card>
      <SettingRow label="Zitadel project">
        <Link href={`/projects/${project.id}`} className="text-[15px] font-semibold text-ink link-hover">
          {projectLabel(project.name || resolver.resolveProject(project.id).value?.name)}
        </Link>
      </SettingRow>
      <SettingRow
        label="Role mappings"
        action={
          <CardHeaderLink href={`/system/targets/${target}/mappings`}>
            {roles === 0 ? "Add the first mapping" : "Mappings"}
          </CardHeaderLink>
        }
      >
        <span className="text-[15px] text-ink">
          {mappings.isLoading
            ? "…"
            : roles === 0
              ? "None yet"
              : `${roles} role${roles === 1 ? "" : "s"} to ${mappedTo(name, schema)}${version ? ` · version ${version}` : ""}`}
        </span>
      </SettingRow>
      <HomeLocationRow target={target} lastKnown={lastKnown} />
    </Card>
  );
}

