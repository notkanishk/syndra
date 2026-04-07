"use client";

import { useEffect, useState } from "react";

import CreateRuleForm from "@/components/CreateRuleForm";
import { Badge } from "@/components/ui/Badge";
import { Card, CardTitle } from "@/components/ui/Card";

interface MappingRule {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  version?: number;
}

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CatalogResponse {
  projects: ProjectCatalog[];
}

export default function PoliciesView() {
  const [rules, setRules] = useState<MappingRule[]>([]);
  const [projects, setProjects] = useState<ProjectCatalog[]>([]);
  const [loading, setLoading] = useState(true);

  async function loadRules() {
    setLoading(true);
    try {
      const res = await fetch("/api/proxy/rules/mapping");
      const data = await res.json();
      setRules(Array.isArray(data) ? data : []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    async function load() {
      const [rulesRes, catalogRes] = await Promise.all([
        fetch("/api/proxy/rules/mapping"),
        fetch("/api/proxy/catalog"),
      ]);
      const rulesData = await rulesRes.json();
      const catalog: CatalogResponse = await catalogRes.json();
      setRules(Array.isArray(rulesData) ? rulesData : []);
      setProjects(Array.isArray(catalog?.projects) ? catalog.projects : []);
      setLoading(false);
    }

    load();
  }, []);

  const projectName = (projectId: string) => projects.find((project) => project.id === projectId)?.name || projectId;

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Policy Engine</h1>
        <p className="text-muted mt-2">
          Define propagation rules that turn raw grants into downstream permissions across software and physical access systems.
        </p>
      </header>

      <Card>
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Active Mapping Rules</CardTitle>
          <CreateRuleForm onCreated={loadRules} projects={projects} />
        </div>

        {loading ? (
          <div className="text-center py-10">
            <p className="text-muted mt-3 text-sm">Loading rules...</p>
          </div>
        ) : rules.length === 0 ? (
          <div className="text-center py-10 border border-dashed border-border rounded-lg">
            <p className="text-muted">No rules established yet.</p>
            <p className="text-xs text-muted mt-1">Use the seeded project catalog to create a propagation path above.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {rules.map((rule) => (
              <div
                key={rule.id}
                className="flex flex-col gap-2 p-4 border border-border rounded-lg bg-surfaceHover transition-colors hover:border-primary/50"
              >
                <div className="flex items-center flex-wrap gap-2 text-sm">
                  <span className="font-mono text-muted font-semibold">IF</span>
                  <Badge variant="outline" className="border-primary text-primary">
                    {projectName(rule.source_project)}
                  </Badge>
                  <Badge variant="secondary">{rule.source_role}</Badge>

                  <span className="font-mono text-muted font-semibold mx-1">THEN ADD</span>

                  <Badge variant="outline" className="border-emerald-500 text-emerald-600 dark:text-emerald-400">
                    {projectName(rule.target_project)}
                  </Badge>
                  <Badge variant="secondary">{rule.target_role}</Badge>
                </div>
                <p className="text-xs text-muted">
                  Users who activate `{rule.source_project}:{rule.source_role}` will inherit `{rule.target_project}:{rule.target_role}` after the fixed-point pass completes.
                </p>
                <div className="flex items-center justify-between mt-2 pt-2 border-t border-border/50">
                  <span className="text-xs text-muted">Version {rule.version || 1}</span>
                  <button
                    onClick={async () => {
                      await fetch(`/api/proxy/rules/mapping/${rule.id}`, {
                        method: "PUT",
                        headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({}),
                      });
                      loadRules();
                    }}
                    className="text-xs text-primary hover:text-primaryHover font-medium transition-colors"
                  >
                    Bump Version
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
