"use client";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { CopyButton } from "@/components/ui/CopyButton";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { JsonView } from "@/components/ui/JsonView";
import { Select } from "@/components/ui/Select";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { useApplications, useTokenSimulator, type SimulationResponse } from "@/lib/queries/useApplications";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api-client";

interface UserProfile {
  id: string;
  name: string;
  title: string;
}

/**
 * The /catalog endpoint is the only path members are allowed to read for the
 * application persona list. We pull just the `users` slice and cache it under
 * the broad `catalog` key so the few pages that need persona data share a
 * single network call.
 */
function useCatalogUsers() {
  return useQuery({
    queryKey: ["catalog", "users"],
    queryFn: async (): Promise<UserProfile[]> => {
      const data = await request<{ users?: UserProfile[] }>("/catalog");
      return Array.isArray(data?.users) ? data.users : [];
    },
  });
}

export default function ApplicationsView() {
  const appsQuery = useApplications();
  const usersQuery = useCatalogUsers();
  const applications = useMemo(() => appsQuery.data ?? [], [appsQuery.data]);
  const users = useMemo(() => usersQuery.data ?? [], [usersQuery.data]);
  const loading = appsQuery.isLoading || usersQuery.isLoading;

  const [selectedApp, setSelectedApp] = useState<string>("");
  const [selectedUser, setSelectedUser] = useState<string>("");
  const [compareUser, setCompareUser] = useState<string>("");

  // Wait for the live catalog to land before defaulting selections so we never
  // serialize a stale or demo identifier into the cache key on first render.
  useEffect(() => {
    if (!selectedApp && applications.length > 0) {
      setSelectedApp(applications[0].application.id);
    }
  }, [applications, selectedApp]);
  useEffect(() => {
    if (!selectedUser && users.length > 0) {
      setSelectedUser(users[0].id);
    }
  }, [users, selectedUser]);

  const primarySim = useTokenSimulator(selectedApp, selectedUser);
  const compareSim = useTokenSimulator(selectedApp, compareUser);

  const activeApp = applications.find((app) => app.application.id === selectedApp);

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Applications</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Claim profiles &amp; token simulator
        </h1>
        <p className="text-on-surface-variant mt-2">
          Inspect claim-shaping profiles and preview the exact payload each
          downstream system receives — including a side-by-side diff between
          two users.
        </p>
      </header>

      {loading ? (
        <Card>
          <SkeletonCardList count={3} />
        </Card>
      ) : applications.length === 0 ? (
        <Card>
          <EmptyState
            eyebrow="Empty registry"
            title="No applications registered"
            description="Add an OIDC, API, or SAML application in Zitadel to start shaping its claims. Newly registered apps appear here automatically."
          />
        </Card>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-[1.05fr,1.15fr] gap-6 items-start">
          <Card>
            <CardHeader>
              <CardTitle>Application registry</CardTitle>
            </CardHeader>
            <div className="space-y-3">
              {applications.map((entry) => {
                const isSelected = selectedApp === entry.application.id;
                return (
                  <button
                    key={entry.application.id}
                    type="button"
                    onClick={() => setSelectedApp(entry.application.id)}
                    aria-pressed={isSelected}
                    className={`w-full rounded-card border p-4 text-left transition-colors ${
                      isSelected
                        ? "border-primary-container bg-primary-container/15"
                        : "border-outline-variant bg-surface-container-low hover:border-primary-container/60"
                    }`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-base font-semibold text-on-surface">
                          {entry.application.name}
                        </p>
                        <p className="mt-1 text-sm text-on-surface-variant">
                          {entry.application.description || "No description provided."}
                        </p>
                      </div>
                      <Badge variant="secondary">{entry.assigned_user_count} users</Badge>
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      <Badge variant="outline" className="border-primary-container/40 text-primary-container">
                        {entry.application.claim_name}
                      </Badge>
                      <Badge variant="secondary">{entry.application.format_type}</Badge>
                      <Badge variant="secondary">{entry.application.consumer}</Badge>
                    </div>
                    {entry.consumed_roles.length > 0 && (
                      <div className="mt-3 flex flex-wrap gap-2">
                        {entry.consumed_roles.map((role) => (
                          <Badge key={role} variant="outline">
                            {role}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </button>
                );
              })}
            </div>
          </Card>

          {/*
           * The right column packs three stacked cards. We pin both columns to
           * `items-start` and give this stack `min-h-0` so the simulator card
           * grows to its content height instead of stretching to match the
           * tall registry list — that was the height-mismatch glitch on wide
           * viewports prior to Stage 3.
           */}
          <div className="space-y-6 min-h-0 h-full flex flex-col">
            <Card className="min-h-0">
              <CardHeader>
                <CardTitle>Token Simulator</CardTitle>
              </CardHeader>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <label className="text-sm text-on-surface-variant">
                  Application
                  <Select
                    value={selectedApp}
                    onChange={(event) => setSelectedApp(event.target.value)}
                    className="mt-2 w-full"
                  >
                    {applications.map((entry) => (
                      <option key={entry.application.id} value={entry.application.id}>
                        {entry.application.name}
                      </option>
                    ))}
                  </Select>
                </label>

                <label className="text-sm text-on-surface-variant">
                  User persona
                  <Select
                    value={selectedUser}
                    onChange={(event) => setSelectedUser(event.target.value)}
                    className="mt-2 w-full"
                  >
                    {users.map((user) => (
                      <option key={user.id} value={user.id}>
                        {user.name}
                      </option>
                    ))}
                  </Select>
                </label>

                <label className="text-sm text-on-surface-variant">
                  Compare with (optional)
                  <Select
                    value={compareUser}
                    onChange={(event) => setCompareUser(event.target.value)}
                    className="mt-2 w-full"
                  >
                    <option value="">— off —</option>
                    {users.filter((u) => u.id !== selectedUser).map((user) => (
                      <option key={user.id} value={user.id}>
                        {user.name}
                      </option>
                    ))}
                  </Select>
                </label>
              </div>

              {compareUser ? (
                <div className="mt-4 grid grid-cols-1 lg:grid-cols-2 gap-3">
                  <SimulationPanel
                    title={users.find((u) => u.id === selectedUser)?.name ?? "Primary"}
                    busy={primarySim.isLoading || primarySim.isFetching}
                    simulation={primarySim.data ?? null}
                    diffAgainst={compareSim.data?.custom_claims}
                  />
                  <SimulationPanel
                    title={users.find((u) => u.id === compareUser)?.name ?? "Compare"}
                    busy={compareSim.isLoading || compareSim.isFetching}
                    simulation={compareSim.data ?? null}
                    diffAgainst={primarySim.data?.custom_claims}
                  />
                </div>
              ) : (
                <SimulationPanel
                  title={users.find((u) => u.id === selectedUser)?.name ?? "Token"}
                  busy={primarySim.isLoading || primarySim.isFetching}
                  simulation={primarySim.data ?? null}
                  className="mt-4"
                />
              )}

              {(primarySim.data?.raw_roles?.length ?? 0) > 0 && (
                <div className="mt-4 flex flex-wrap gap-2">
                  <Eyebrow>Raw roles</Eyebrow>
                  <div className="basis-full" />
                  {(primarySim.data?.raw_roles || []).map((role) => (
                    <Badge
                      key={role}
                      variant="outline"
                      className="border-[var(--success)]/30 text-[var(--success)]"
                    >
                      {role}
                    </Badge>
                  ))}
                </div>
              )}
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Claim profile</CardTitle>
              </CardHeader>
              {activeApp ? (
                <div className="space-y-3 text-sm">
                  <div className="rounded-card bg-surface-container-low p-4">
                    <Eyebrow>Consumer</Eyebrow>
                    <p className="mt-2 text-base font-semibold text-on-surface">
                      {activeApp.application.consumer}
                    </p>
                  </div>
                  <div className="rounded-card bg-surface-container-low p-4">
                    <Eyebrow>Role projection</Eyebrow>
                    <p className="mt-2 text-base font-semibold text-on-surface">
                      {activeApp.application.project_id}
                    </p>
                    <p className="mt-1 text-on-surface-variant">
                      Roles are flattened for this project and written into{" "}
                      <code className="text-on-surface">{activeApp.application.claim_name}</code>{" "}
                      using the{" "}
                      <code className="text-on-surface">{activeApp.application.format_type}</code>{" "}
                      formatter.
                    </p>
                  </div>
                </div>
              ) : (
                <p className="text-sm text-on-surface-variant">
                  Select an application to inspect its claim profile.
                </p>
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
    <div
      className={`rounded-card border border-outline-variant bg-surface-container-lowest ${className}`}
    >
      <div className="flex items-center justify-between border-b border-outline-variant px-3 py-2">
        <Eyebrow>{title}</Eyebrow>
        {simulation && <CopyButton text={json} label="Copy JSON" />}
      </div>
      <div className="overflow-x-auto p-4 font-mono">
        {busy || !simulation ? (
          <p className="text-xs text-on-surface-variant">Simulating token…</p>
        ) : (
          <JsonView value={simulation.custom_claims} compareWith={diffAgainst} />
        )}
      </div>
    </div>
  );
}
