"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import CreateRuleForm from "@/components/CreateRuleForm";

interface MappingRule {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
}

export default function PoliciesView() {
  const [rules, setRules] = useState<MappingRule[]>([]);
  const [loading, setLoading] = useState(true);

  const loadRules = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/proxy/rules/mapping");
      const data = await res.json();
      setRules(Array.isArray(data) ? data : []);
    } catch {
      setRules([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadRules();
  }, [loadRules]);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Policy Engine</h1>
        <p className="text-muted mt-2">
          Define propagation rules: when a user gains a role in one project, automatically grant roles in dependent projects.
        </p>
      </header>

      <Card>
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Active Mapping Rules</CardTitle>
          <CreateRuleForm onCreated={loadRules} />
        </div>

        {loading ? (
          <div className="text-center py-10">
            <div className="inline-block w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin"></div>
            <p className="text-muted mt-3 text-sm">Loading rules...</p>
          </div>
        ) : rules.length === 0 ? (
          <div className="text-center py-10 border border-dashed border-border rounded-lg">
            <p className="text-muted">No rules established. Systems are mutually exclusive.</p>
            <p className="text-xs text-muted mt-1">Click &quot;+ New Logic Rule&quot; above to create one.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {rules.map((rule) => (
              <div
                key={rule.id}
                className="p-4 border border-border rounded-lg bg-surfaceHover flex items-center flex-wrap gap-2 text-sm transition-colors hover:border-primary/50"
              >
                <span className="font-mono text-muted font-semibold">IF</span>
                <Badge variant="outline" className="border-primary text-primary">
                  {rule.source_project}
                </Badge>
                <Badge variant="secondary">{rule.source_role}</Badge>

                <span className="font-mono text-muted font-semibold mx-1">→ THEN ADD</span>

                <Badge variant="outline" className="border-emerald-500 text-emerald-600 dark:text-emerald-400">
                  {rule.target_project}
                </Badge>
                <Badge variant="secondary">{rule.target_role}</Badge>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
