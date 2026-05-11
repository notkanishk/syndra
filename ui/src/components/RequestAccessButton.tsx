"use client";

import { useEffect, useState } from "react";

import { Modal } from "@/components/ui/Modal";
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
 * Wraps the canonical <Modal> primitive (focus trap, aria-modal, Esc, click-outside)
 * with a justification textarea and a friendly duration picker; submits to
 * `/api/proxy/requests` and shows toast feedback. For already-active or pending
 * services, links straight to `/requests` since there's nothing to request.
 */
export default function RequestAccessButton({ projectId, serviceName, status }: RequestAccessButtonProps) {
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [project, setProject] = useState<ProjectInfo | null>(null);
  const [justification, setJustification] = useState("");
  const [duration, setDuration] = useState<"7" | "30" | "120" | "0">("30");

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

  if (status !== "No Access") {
    return (
      <a
        href="/requests"
        className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
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
        className="mt-4 inline-flex rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        Request Access
      </button>

      <Modal
        open={open}
        onClose={() => { if (!submitting) setOpen(false); }}
        labelledBy={`request-${projectId}-title`}
        busy={submitting}
      >
        <h2 id={`request-${projectId}-title`} className="text-lg font-semibold text-on-surface">
          Request access to {serviceName}
        </h2>
        <p className="mt-2 text-sm text-on-surface-variant">
          {project
            ? `You'll be requesting the "${defaultRoleLabel}" role. Add a brief justification and pick how long you need it.`
            : "Loading service details…"}
        </p>

        <label className="mt-4 block text-xs font-medium text-on-surface-variant">Why do you need this access?</label>
        <textarea
          value={justification}
          onChange={(e) => setJustification(e.target.value)}
          placeholder="Briefly explain the project or task that needs this access."
          className="mt-2 min-h-[6rem] w-full rounded-lg border border-outline-variant bg-background px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
        />

        <p className="mt-4 text-xs uppercase tracking-[0.18em] text-on-surface-variant">Duration</p>
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
                    ? "border-primary bg-primary-container text-on-primary-container"
                    : "border-outline-variant text-on-surface-variant hover:text-on-surface hover:border-primary"
                }`}
              >
                {opt.label}
              </button>
            );
          })}
        </div>

        <div className="mt-6 flex justify-end gap-3">
          <button
            type="button"
            onClick={() => setOpen(false)}
            disabled={submitting}
            className="rounded-lg border border-outline-variant px-4 py-2 text-sm font-medium text-on-surface transition-colors hover:bg-surface-container-high disabled:opacity-50"
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
      </Modal>
    </>
  );
}
