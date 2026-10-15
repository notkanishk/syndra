"use client";

import { useState } from "react";

import { askHref } from "@/components/member/RecentChanges";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Unfold } from "@/components/ui/Unfold";
import { ErrorState, RowSkeleton } from "@/components/states";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";
import type { ProjectCatalog } from "@/lib/types";

/**
 * What else is here (route-me 52b) — the half of a member's page that isn't
 * about them. One search, and every room as a fold: closed, it says how much
 * of it you have; open, its roles with what each lets you do, then "Ask for
 * this →" or "You have this". A search opens the rooms with a match, tints
 * the match, and folds the rest with "No match".
 *
 * Every project is listed, held roles included and marked as held. Hiding what
 * you already have would make the list rearrange itself per person.
 */
export function MemberCatalog({ heldByProject }: { heldByProject: Map<string, Set<string>> }) {
  const catalog = useCatalogProjects();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState<Set<string>>(new Set());

  if (catalog.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={3} avatar={false} label="Loading what's available" />
      </Card>
    );
  }
  if (catalog.error) {
    return (
      <ErrorState
        title="Couldn't load what's available."
        error={catalog.error}
        onRetry={() => catalog.refetch()}
      />
    );
  }

  const projects = catalog.data.filter((project) => project.roles.length > 0);
  if (projects.length === 0) return null;

  const q = query.trim().toLowerCase();
  const matchesOf = (project: ProjectCatalog) =>
    q ? new Set(project.roles.filter((role) => matches(project, role, q)).map((role) => role.key)) : null;

  function search(value: string) {
    setQuery(value);
    const next = value.trim().toLowerCase();
    // A search opens the rooms with a match and folds the rest; clearing it
    // folds everything back.
    setOpen(
      new Set(
        next
          ? projects.filter((p) => p.roles.some((role) => matches(p, role, next))).map((p) => p.id)
          : [],
      ),
    );
  }

  function toggle(id: string) {
    setOpen((was) => {
      const next = new Set(was);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  const nothingFound = q !== "" && projects.every((project) => matchesOf(project)!.size === 0);

  return (
    <div className="mt-1.5 flex flex-col gap-3">
      {/* Stacked: a row here squeezed the heading and its caption side by side. */}
      <div>
        <h2 className="type-section-title">What else is here</h2>
        <p className="mt-1 text-[13.5px] text-faint">
          Everything the makerspace offers. Ask for anything you need; makerspace staff decide.
        </p>
      </div>

      <div className="max-w-[560px]">
        <Input
          type="search"
          value={query}
          onChange={(event) => search(event.target.value)}
          placeholder="Search everything on offer: a machine, a room, a door"
          aria-label="Search everything on offer"
        />
      </div>

      <Card>
        {nothingFound && (
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3.5">
            <span className="flex-1 text-[14px] text-muted">
              Nothing on offer matches &ldquo;{query.trim()}&rdquo;. Ask for access, and say what you need under Why.
            </span>
            <ArrowLink href="/requests?ask">Ask for access</ArrowLink>
          </div>
        )}
        {projects.map((project, i) => {
          const held = heldByProject.get(project.id) ?? new Set<string>();
          const hits = matchesOf(project);
          const isOpen = open.has(project.id);
          // Open in a search, a room shows its matches; opened by hand, all of it.
          const roles = hits && hits.size > 0 ? project.roles.filter((role) => hits.has(role.key)) : project.roles;
          const have = project.roles.filter((role) => held.has(role.key)).length;
          const [count, countClass] = hits
            ? hits.size > 0
              ? [`${hits.size} ${hits.size === 1 ? "match" : "matches"}`, "text-accent-text"]
              : ["No match", "text-faint"]
            : have === project.roles.length
              ? ["You have all of it", "text-ink"]
              : have > 0
                ? [`You have ${have} of ${project.roles.length}`, "text-ink"]
                : [`${project.roles.length} you can ask for`, "text-faint"];

          return (
            <div key={project.id} className={i > 0 || nothingFound ? "row-divider" : ""}>
              <button
                type="button"
                aria-expanded={isOpen}
                onClick={() => toggle(project.id)}
                className="hover-tint flex min-h-[52px] w-full items-center gap-3 px-5 py-2.5 text-left"
              >
                <span
                  aria-hidden
                  className="inline-flex size-7 flex-none items-center justify-center rounded-pill border border-line text-muted"
                >
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className={`motion-press ${isOpen ? "" : "-rotate-90"}`}
                  >
                    <path d="M6 9l6 6 6-6" />
                  </svg>
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-px">
                  <span className="text-[15px] font-semibold">{project.name}</span>
                  {project.description && <span className="text-[13px] text-muted">{project.description}</span>}
                </span>
                <span className={`text-right text-[13px] font-semibold ${countClass}`}>{count}</span>
              </button>
              <Unfold open={isOpen}>
                <div className="pb-2 pl-[60px] pr-5">
                  {roles.map((role) => {
                    const mine = held.has(role.key);
                    return (
                      <div
                        key={role.key}
                        data-match={hits?.has(role.key) || undefined}
                        className={`row-divider flex min-h-[48px] flex-wrap items-center gap-x-3.5 gap-y-1.5 py-2.5 ${
                          hits?.has(role.key) ? "bg-accent-soft" : ""
                        }`}
                      >
                        <span className="flex min-w-0 flex-[1_1_240px] flex-col gap-0.5">
                          <span className="text-[14.5px] font-semibold">{role.label || role.key}</span>
                          {role.description && (
                            <span className="text-[13px] text-muted">{role.description}</span>
                          )}
                        </span>
                        {mine ? (
                          <span className="text-[13px] text-faint">You have this</span>
                        ) : (
                          // A link, not a dialog: the ask belongs on the requests
                          // page beside the answers to earlier asks, and a link
                          // is a sentence one person can send another.
                          <ArrowLink href={askHref(project.id, role.key)}>Ask for this</ArrowLink>
                        )}
                      </div>
                    );
                  })}
                </div>
              </Unfold>
            </div>
          );
        })}
      </Card>
    </div>
  );
}

/** A role matches by its own words, or by its room's name ("a room"). */
function matches(project: ProjectCatalog, role: ProjectCatalog["roles"][number], q: string): boolean {
  return [role.label, role.key, role.description, project.name].some((text) => (text ?? "").toLowerCase().includes(q));
}
