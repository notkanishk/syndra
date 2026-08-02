"use client";

import Link from "next/link";

import { Card, CardHeader } from "@/components/ui/Card";
import { ErrorState, RowSkeleton } from "@/components/states";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";

/**
 * What else is here — the half of a member's page that isn't about them.
 *
 * "My access" answers what you have. Nothing answered what you could ask for,
 * so a member who wanted the laser cutter had to already know that a thing
 * called a laser cutter was on offer, and then describe it in a free-text box.
 * The only path in was a request form that asked you to name what you wanted
 * before showing you what there was.
 *
 * Every project is listed, held roles included and marked as held. Hiding what
 * you already have would make the list rearrange itself per person, and the
 * most useful thing this page can say about a space is often "you already have
 * everything here".
 *
 * No jargon, same as the rest of the member view: role labels and their
 * descriptions, never keys.
 */
export function MemberCatalog({ heldByProject }: { heldByProject: Map<string, Set<string>> }) {
  const catalog = useCatalogProjects();

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

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-baseline gap-3">
        <h2 className="type-section-title">What else is here</h2>
        <span className="text-[13.5px] text-faint">
          Everything the makerspace offers. Ask for anything you need — a lab manager decides.
        </span>
      </div>

      <div className="flex flex-wrap gap-[18px]">
        {projects.map((project) => {
          const held = heldByProject.get(project.id) ?? new Set<string>();
          const hasAll = project.roles.every((role) => held.has(role.key));

          return (
            <Card key={project.id} className="min-w-[420px] flex-1">
              <CardHeader
                title={project.name}
                note={hasAll ? "You have everything here" : project.description || undefined}
              />
              {project.roles.map((role) => {
                const mine = held.has(role.key);
                return (
                  <div
                    key={role.key}
                    className="row-divider flex items-center gap-3.5 px-5 py-3"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-[14.5px] font-semibold">
                        {role.label || role.key}
                      </span>
                      {role.description && (
                        <span className="block text-[13px] leading-[1.5] text-muted">
                          {role.description}
                        </span>
                      )}
                    </span>
                    {mine ? (
                      <span className="shrink-0 text-[13px] text-faint">You have this</span>
                    ) : (
                      // Deep link rather than a dialog here: the ask belongs on
                      // the requests page next to the answers to earlier asks,
                      // and a link is shareable — "ask for this" is a sentence
                      // one person sends another.
                      <Link
                        href={`/requests?project=${encodeURIComponent(
                          project.id,
                        )}&role=${encodeURIComponent(role.key)}`}
                        className="shrink-0 text-[13.5px] font-semibold text-accent-text"
                      >
                        Ask for this →
                      </Link>
                    )}
                  </div>
                );
              })}
            </Card>
          );
        })}
      </div>
    </div>
  );
}
