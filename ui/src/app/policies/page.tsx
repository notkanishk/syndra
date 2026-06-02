"use client";

import { useMemo, useState } from "react";

import CreateRuleForm from "@/components/CreateRuleForm";
import { ProjectName, RoleName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Pulse } from "@/components/ui/Pulse";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api-client";
import { useMappingRules } from "@/lib/queries/useMappingRules";

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

function useCatalogProjects() {
  return useQuery({
    queryKey: ["catalog", "projects"],
    queryFn: async (): Promise<ProjectCatalog[]> => {
      const data = await request<{ projects?: ProjectCatalog[] }>("/catalog");
      return Array.isArray(data?.projects) ? data.projects : [];
    },
  });
}

export default function PoliciesView() {
  const rulesQuery = useMappingRules();
  const projectsQuery = useCatalogProjects();
  const rules = useMemo(() => rulesQuery.data ?? [], [rulesQuery.data]);
  const projects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const loading = rulesQuery.isLoading || projectsQuery.isLoading;

  const [createOpen, setCreateOpen] = useState(false);

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Policy engine</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Mapping rules
        </h1>
        <p className="text-on-surface-variant mt-2">
          Define propagation rules that turn raw grants into downstream
          permissions across software and physical access systems.
        </p>
      </header>

      <Card variant="glass">
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Active mapping rules</CardTitle>
          <Button variant="primary" size="sm" onClick={() => setCreateOpen(true)}>
            + New rule
          </Button>
        </div>

        {loading ? (
          <SkeletonCardList count={3} />
        ) : rules.length === 0 ? (
          <EmptyState
            eyebrow="No rules yet"
            title="No mapping rules yet"
            description="Define a rule to propagate role grants from one project into another. The form opens in a focused modal so you can preview cycle warnings before saving."
            action={{ label: "Create rule", onClick: () => setCreateOpen(true) }}
          />
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {rules.map((rule) => (
              <article
                key={rule.id}
                className="rounded-card border border-outline-variant bg-surface-container-low p-4 transition-colors hover:border-primary-container/50"
              >
                <div className="flex items-center gap-2">
                  <Pulse variant="info" />
                  <Eyebrow tone="primary">Mapping rule</Eyebrow>
                </div>

                <div className="mt-3 flex flex-wrap items-center gap-2 text-sm">
                  <span className="font-mono text-on-surface-variant font-semibold">IF</span>
                  <Badge
                    variant="outline"
                    className="border-primary-container/40 text-primary-container"
                  >
                    <ProjectName id={rule.source_project} />
                  </Badge>
                  <Badge variant="secondary">
                    <RoleName projectId={rule.source_project} roleKey={rule.source_role} />
                  </Badge>

                  <span className="font-mono text-on-surface-variant font-semibold mx-1">
                    THEN ADD
                  </span>

                  <Badge
                    variant="outline"
                    className="border-[var(--success)]/40 text-[var(--success)]"
                  >
                    <ProjectName id={rule.target_project} />
                  </Badge>
                  <Badge variant="secondary">
                    <RoleName projectId={rule.target_project} roleKey={rule.target_role} />
                  </Badge>
                </div>

                <p className="mt-3 text-xs text-on-surface-variant">
                  Users who activate{" "}
                  <ProjectName id={rule.source_project} />:
                  <RoleName projectId={rule.source_project} roleKey={rule.source_role} /> will
                  inherit{" "}
                  <ProjectName id={rule.target_project} />:
                  <RoleName projectId={rule.target_project} roleKey={rule.target_role} /> after
                  the fixed-point pass completes.
                </p>
              </article>
            ))}
          </div>
        )}
      </Card>

      <CreateRuleForm
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        projects={projects}
      />
    </div>
  );
}
