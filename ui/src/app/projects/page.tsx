"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";

interface ProjectSummary {
  project: {
    id: string;
    name: string;
    kind: string;
    description: string;
    roles: Array<{ key: string; label: string }>;
  };
  member_count: number;
  bundle_count: number;
  rule_in_count: number;
  rule_out_count: number;
  active_role_keys: string[];
  sample_members: string[];
}

export default function ProjectsView() {
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const res = await fetch("/api/proxy/projects");
        const data = await res.json();
        setProjects(Array.isArray(data) ? data : []);
      } finally {
        setLoading(false);
      }
    }

    load();
  }, []);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Projects</h1>
        <p className="text-muted mt-2">See which roles exist per project, how policies touch them, and which members are currently active in each space.</p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Project-Centric View</CardTitle>
        </CardHeader>

        {loading ? (
          <p className="text-sm text-muted">Loading project summaries...</p>
        ) : (
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
            {projects.map((entry) => (
              <div key={entry.project.id} className="rounded-xl border border-border bg-surfaceHover p-5">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h2 className="text-lg font-semibold text-foreground">{entry.project.name}</h2>
                    <p className="mt-1 text-sm text-muted">{entry.project.description}</p>
                  </div>
                  <Badge variant="secondary">{entry.project.kind}</Badge>
                </div>

                <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Members</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.member_count}</p>
                  </div>
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Bundles</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.bundle_count}</p>
                  </div>
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Rules In</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.rule_in_count}</p>
                  </div>
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Rules Out</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.rule_out_count}</p>
                  </div>
                </div>

                <div className="mt-4">
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Role Catalog</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {entry.project.roles.map((role) => (
                      <Badge key={role.key} variant="outline">
                        {role.key}
                      </Badge>
                    ))}
                  </div>
                </div>

                <div className="mt-4">
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Currently Active</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {entry.active_role_keys.map((role) => (
                      <Badge key={role} variant="outline" className="border-primary/25 text-primary">
                        {role}
                      </Badge>
                    ))}
                  </div>
                </div>

                <div className="mt-4">
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Sample Members</p>
                  <p className="mt-2 text-sm text-foreground">
                    {entry.sample_members.length > 0 ? entry.sample_members.join(", ") : "No active members yet."}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
