"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { CopyButton } from "@/components/ui/CopyButton";
import { EmptyState } from "@/components/ui/EmptyState";
import { JsonView } from "@/components/ui/JsonView";
import { SkeletonCardList } from "@/components/ui/Skeleton";

interface UserProfile {
  id: string;
  name: string;
  title: string;
}

interface CatalogResponse {
  users: UserProfile[];
}

interface ApplicationView {
  application: {
    id: string;
    name: string;
    project_id: string;
    description: string;
    consumer: string;
    claim_name: string;
    format_type: string;
  };
  consumed_roles: string[];
  assigned_user_count: number;
}

interface SimulationResponse {
  application: ApplicationView["application"];
  user: UserProfile;
  raw_roles: string[];
  custom_claims: Record<string, unknown>;
}

export default function ApplicationsView() {
  const [applications, setApplications] = useState<ApplicationView[]>([]);
  const [users, setUsers] = useState<UserProfile[]>([]);
  const [selectedApp, setSelectedApp] = useState<string>("");
  const [selectedUser, setSelectedUser] = useState<string>("");
  const [simulation, setSimulation] = useState<SimulationResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [simulating, setSimulating] = useState(false);
  // Compare-with state — when set, fetches a second simulation and shows
  // both panels side-by-side with diff highlighting.
  const [compareUser, setCompareUser] = useState<string>("");
  const [compareSimulation, setCompareSimulation] = useState<SimulationResponse | null>(null);
  const [comparing, setComparing] = useState(false);

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const [appsRes, catalogRes] = await Promise.all([
          fetch("/api/proxy/applications"),
          fetch("/api/proxy/catalog"),
        ]);

        const apps = await appsRes.json();
        const catalog: CatalogResponse = await catalogRes.json();

        const appList = Array.isArray(apps) ? apps : [];
        setApplications(appList);
        setUsers(Array.isArray(catalog?.users) ? catalog.users : []);

        if (appList.length > 0) {
          setSelectedApp(appList[0].application.id);
        }
        if (Array.isArray(catalog?.users) && catalog.users.length > 0) {
          setSelectedUser(catalog.users[0].id);
        }
      } finally {
        setLoading(false);
      }
    }

    load();
  }, []);

  useEffect(() => {
    async function simulate() {
      if (!selectedApp || !selectedUser) {
        return;
      }
      setSimulating(true);
      try {
        const res = await fetch(`/api/proxy/applications/${selectedApp}/simulate?user_id=${selectedUser}`);
        const data = await res.json();
        setSimulation(data);
      } finally {
        setSimulating(false);
      }
    }

    simulate();
  }, [selectedApp, selectedUser]);

  // Run the compare simulation in parallel when a second user is picked.
  useEffect(() => {
    async function simulateCompare() {
      if (!selectedApp || !compareUser) {
        setCompareSimulation(null);
        return;
      }
      setComparing(true);
      try {
        const res = await fetch(`/api/proxy/applications/${selectedApp}/simulate?user_id=${compareUser}`);
        setCompareSimulation(await res.json());
      } finally {
        setComparing(false);
      }
    }

    simulateCompare();
  }, [selectedApp, compareUser]);

  const activeApp = applications.find((app) => app.application.id === selectedApp);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Applications</h1>
        <p className="text-muted mt-2">Inspect claim-shaping profiles and preview the exact payload each downstream system receives.</p>
      </header>

      {loading ? (
        <Card>
          <SkeletonCardList count={3} />
        </Card>
      ) : applications.length === 0 ? (
        <Card>
          <EmptyState
            title="No applications registered"
            description="Add an OIDC, API, or SAML application in Zitadel to start shaping its claims. Newly registered apps appear here automatically."
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-[1.05fr,1.15fr] gap-6">
          <Card>
            <CardHeader>
              <CardTitle>Application-Centric View</CardTitle>
            </CardHeader>
            <div className="space-y-3">
              {applications.map((entry) => (
                <button
                  key={entry.application.id}
                  onClick={() => setSelectedApp(entry.application.id)}
                  className={`w-full rounded-xl border p-4 text-left transition-colors ${
                    selectedApp === entry.application.id
                      ? "border-primary bg-primary/5"
                      : "border-border bg-surfaceHover hover:border-primary/40"
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <p className="text-base font-semibold text-foreground">{entry.application.name}</p>
                      <p className="mt-1 text-sm text-muted">{entry.application.description}</p>
                    </div>
                    <Badge variant="secondary">{entry.assigned_user_count} users</Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <Badge variant="outline" className="border-primary/30 text-primary">
                      {entry.application.claim_name}
                    </Badge>
                    <Badge variant="secondary">{entry.application.format_type}</Badge>
                    <Badge variant="secondary">{entry.application.consumer}</Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {entry.consumed_roles.map((role) => (
                      <Badge key={role} variant="outline">
                        {role}
                      </Badge>
                    ))}
                  </div>
                </button>
              ))}
            </div>
          </Card>

          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Token Simulator</CardTitle>
              </CardHeader>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <label className="text-sm text-muted">
                  Application
                  <select
                    value={selectedApp}
                    onChange={(event) => setSelectedApp(event.target.value)}
                    className="mt-2 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground"
                  >
                    {applications.map((entry) => (
                      <option key={entry.application.id} value={entry.application.id}>
                        {entry.application.name}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="text-sm text-muted">
                  User Persona
                  <select
                    value={selectedUser}
                    onChange={(event) => setSelectedUser(event.target.value)}
                    className="mt-2 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground"
                  >
                    {users.map((user) => (
                      <option key={user.id} value={user.id}>
                        {user.name}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="text-sm text-muted">
                  Compare with (optional)
                  <select
                    value={compareUser}
                    onChange={(event) => setCompareUser(event.target.value)}
                    className="mt-2 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground"
                  >
                    <option value="">— off —</option>
                    {users.filter((u) => u.id !== selectedUser).map((user) => (
                      <option key={user.id} value={user.id}>
                        {user.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              {compareUser ? (
                <div className="mt-4 grid grid-cols-1 lg:grid-cols-2 gap-3">
                  <SimulationPanel
                    title={users.find((u) => u.id === selectedUser)?.name ?? "Primary"}
                    busy={simulating}
                    simulation={simulation}
                    diffAgainst={compareSimulation?.custom_claims}
                  />
                  <SimulationPanel
                    title={users.find((u) => u.id === compareUser)?.name ?? "Compare"}
                    busy={comparing}
                    simulation={compareSimulation}
                    diffAgainst={simulation?.custom_claims}
                  />
                </div>
              ) : (
                <SimulationPanel
                  title={users.find((u) => u.id === selectedUser)?.name ?? "Token"}
                  busy={simulating}
                  simulation={simulation}
                  className="mt-4"
                />
              )}

              <div className="mt-4 flex flex-wrap gap-2">
                {(simulation?.raw_roles || []).map((role) => (
                  <Badge key={role} variant="outline" className="border-emerald-500/30 text-emerald-600 dark:text-emerald-400">
                    {role}
                  </Badge>
                ))}
              </div>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Claim Profile</CardTitle>
              </CardHeader>
              {activeApp ? (
                <div className="space-y-3 text-sm">
                  <div className="rounded-xl border border-border bg-surfaceHover p-4">
                    <p className="text-xs uppercase tracking-[0.24em] text-muted">Consumer</p>
                    <p className="mt-2 text-base font-semibold">{activeApp.application.consumer}</p>
                  </div>
                  <div className="rounded-xl border border-border bg-surfaceHover p-4">
                    <p className="text-xs uppercase tracking-[0.24em] text-muted">Role Projection</p>
                    <p className="mt-2 text-base font-semibold">{activeApp.application.project_id}</p>
                    <p className="mt-1 text-muted">
                      Roles are flattened for this project and written into `{activeApp.application.claim_name}` using the `{activeApp.application.format_type}` formatter.
                    </p>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-muted">Select an application to inspect its claim profile.</p>
              )}
            </Card>
          </div>
        </div>
      )}
    </div>
  );
}

interface SimulationPanelProps {
  title: string;
  busy: boolean;
  simulation: SimulationResponse | null;
  diffAgainst?: Record<string, unknown>;
  className?: string;
}

function SimulationPanel({ title, busy, simulation, diffAgainst, className = "" }: SimulationPanelProps) {
  const json = simulation ? JSON.stringify(simulation.custom_claims, null, 2) : "";
  return (
    <div className={`rounded-xl border border-border bg-background ${className}`}>
      <div className="flex items-center justify-between border-b border-border px-3 py-2">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-muted">{title}</p>
        {simulation && <CopyButton text={json} label="Copy JSON" />}
      </div>
      <div className="overflow-x-auto p-4 font-mono">
        {busy || !simulation ? (
          <p className="text-xs text-muted">Simulating token…</p>
        ) : (
          <JsonView value={simulation.custom_claims} compareWith={diffAgainst} />
        )}
      </div>
    </div>
  );
}
