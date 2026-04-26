"use client";

import { useEffect, useRef, useState } from "react";

import { SubmitButton } from "@/components/ui/SubmitButton";
import { toastError, toastSuccess } from "@/lib/toast";

interface ProjectInfo {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CatalogResponse {
  projects?: ProjectInfo[];
}

interface RequestAccessButtonProps {
  projectId: string;
  serviceName: string;
  /** "No Access" gets the inline modal; everything else routes to /requests. */
  status: "Active" | "Pending" | "No Access";
}

/**
 * Inline "Request Access" affordance for the member service catalog.
 * Self-contained focus-trapped modal with a justification textarea and a
 * friendly duration picker; submits to `/api/proxy/requests` and shows toast
 * feedback. For already-active or pending services, links straight to
 * `/requests` since there's nothing to request.
 */
export default function RequestAccessButton({ projectId, serviceName, status }: RequestAccessButtonProps) {
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [project, setProject] = useState<ProjectInfo | null>(null);
  const [justification, setJustification] = useState("");
  const [duration, setDuration] = useState<"7" | "30" | "120" | "0">("30");
  const dialogRef = useRef<HTMLDivElement | null>(null);
  const cancelButtonRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/api/proxy/catalog");
        if (!res.ok) return;
        const data: CatalogResponse = await res.json();
        if (cancelled) return;
        const match = (data.projects ?? []).find((p) => p.id === projectId) ?? null;
        setProject(match);
      } catch {
        // Submit will surface the failure if the catalog is genuinely broken.
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, projectId]);

  // Focus management — focus the cancel button on open, restore prior focus on close.
  useEffect(() => {
    if (!open) return;
    const previously = document.activeElement as HTMLElement | null;
    cancelButtonRef.current?.focus();

    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape" && !submitting) {
        e.preventDefault();
        setOpen(false);
      }
    }

    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      previously?.focus();
    };
  }, [open, submitting]);

  if (status !== "No Access") {
    return (
      <a
        href="/requests"
        className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        View Requests
      </a>
    );
  }

  const defaultRole = project?.roles?.[0];
  const defaultRoleKey = defaultRole?.key ?? "";
  const defaultRoleLabel = defaultRole?.label ?? "default role";

  const submit = async () => {
    if (!justification.trim() || !defaultRoleKey) {
      toastError(!justification.trim() ? "Add a justification before submitting." : "No default role available for this service.");
      return;
    }
    setSubmitting(true);
    try {
      const res = await fetch("/api/proxy/requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_id: projectId,
          role_key: defaultRoleKey,
          justification: justification.trim(),
          duration_days: Number.parseInt(duration, 10),
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to submit request.");
      }
      toastSuccess("Request submitted", `Your administrator will review the request for ${serviceName}.`);
      setOpen(false);
      setJustification("");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to submit request.");
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        Request Access
      </button>

      {open && (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby={`request-${projectId}-title`}
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 px-4"
          onClick={(event) => {
            if (event.target === event.currentTarget && !submitting) setOpen(false);
          }}
        >
          <div
            ref={dialogRef}
            className="w-full max-w-md rounded-2xl border border-border bg-surface p-6 shadow-2xl"
          >
            <h2 id={`request-${projectId}-title`} className="text-lg font-semibold text-foreground">
              Request access to {serviceName}
            </h2>
            <p className="mt-2 text-sm text-muted">
              {project
                ? `You'll be requesting the "${defaultRoleLabel}" role. Add a brief justification and pick how long you need it.`
                : "Loading service details…"}
            </p>

            <label className="mt-4 block text-xs font-medium text-muted">Why do you need this access?</label>
            <textarea
              value={justification}
              onChange={(e) => setJustification(e.target.value)}
              placeholder="Briefly explain the project or task that needs this access."
              className="mt-2 min-h-[6rem] w-full rounded-lg border border-border bg-background px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            />

            <p className="mt-4 text-xs uppercase tracking-[0.18em] text-muted">Duration</p>
            <div className="mt-2 flex flex-wrap gap-2">
              {([
                { label: "1 week", value: "7" },
                { label: "1 month", value: "30" },
                { label: "1 semester", value: "120" },
                { label: "Permanent", value: "0" },
              ] as const).map((opt) => {
                const selected = duration === opt.value;
                return (
                  <button
                    type="button"
                    key={opt.value}
                    onClick={() => setDuration(opt.value)}
                    className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary ${
                      selected
                        ? "border-primary bg-primary/10 text-primary"
                        : "border-border text-muted hover:text-foreground hover:border-primary/40"
                    }`}
                  >
                    {opt.label}
                  </button>
                );
              })}
            </div>

            <div className="mt-6 flex justify-end gap-3">
              <button
                ref={cancelButtonRef}
                type="button"
                onClick={() => setOpen(false)}
                disabled={submitting}
                className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-surfaceHover disabled:opacity-50"
              >
                Cancel
              </button>
              <SubmitButton
                isPending={submitting}
                pendingLabel="Submitting…"
                disabled={!justification.trim() || !defaultRoleKey}
                label="Submit request"
                onClick={submit}
              />
            </div>
          </div>
        </div>
      )}
    </>
  );
}
