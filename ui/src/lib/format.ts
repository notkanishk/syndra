// Display helpers for role and project references. The backend speaks in
// `project_id:role_key` pairs; the UI surfaces those to non-technical staff
// who recognize project names better than internal keys. This module
// centralizes the human-readable formatting so every view does it the same way.

interface ProjectInfo {
  id: string;
  name: string;
  roles?: Array<{ key: string; label?: string }>;
}

/**
 * Format a project_id + role_key pair into a human label, with the raw pair
 * available as a secondary monospace tag for power users. Returns an object
 * so callers can render the parts independently — `label` typically reads
 * like "{Project Name} · {Role Label}" and `raw` is `{project_id}:{role_key}`.
 */
export function formatRoleRef(
  projectId: string,
  roleKey: string,
  projects: ProjectInfo[],
): { label: string; raw: string } {
  const project = projects.find((p) => p.id === projectId);
  const projectLabel = project?.name ?? projectId;
  const role = project?.roles?.find((r) => r.key === roleKey);
  const roleLabel = role?.label ?? humanizeKey(roleKey);
  return {
    label: `${projectLabel} · ${roleLabel}`,
    raw: `${projectId}:${roleKey}`,
  };
}

/**
 * Convert a snake/kebab/lower role key into a friendlier label. Used as a
 * fallback when the project catalog doesn't carry an explicit `label` for
 * the role.
 */
export function humanizeKey(key: string): string {
  if (!key) return "";
  return key
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

export type UrgencyTone = "critical" | "warning" | "neutral" | "expired";

/**
 * Compute a human countdown ("in 3 days", "in 2 hours", "expired") plus a
 * semantic tone the UI can use to color-code urgency. Returns `null` for an
 * absent expiry (permanent grants).
 */
export function describeExpiry(expiresAt: string | null | undefined, now: Date = new Date()):
  | { countdown: string; tone: UrgencyTone; daysLeft: number }
  | null {
  if (!expiresAt) return null;
  const target = new Date(expiresAt).getTime();
  if (Number.isNaN(target)) return null;
  const ms = target - now.getTime();
  const daysLeft = Math.ceil(ms / (1000 * 60 * 60 * 24));

  if (ms <= 0) {
    return { countdown: "expired", tone: "expired", daysLeft };
  }
  if (daysLeft <= 1) {
    const hours = Math.max(1, Math.round(ms / (1000 * 60 * 60)));
    return {
      countdown: `expires in ${hours} hour${hours === 1 ? "" : "s"}`,
      tone: "critical",
      daysLeft,
    };
  }
  if (daysLeft <= 7) {
    return {
      countdown: `expires in ${daysLeft} day${daysLeft === 1 ? "" : "s"}`,
      tone: "critical",
      daysLeft,
    };
  }
  if (daysLeft <= 14) {
    return { countdown: `expires in ${daysLeft} days`, tone: "warning", daysLeft };
  }
  return { countdown: `expires in ${daysLeft} days`, tone: "neutral", daysLeft };
}
