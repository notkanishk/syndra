export const API_BASE = "http://localhost:8080/api/v1";

export async function fetchBundles() {
  const res = await fetch(`${API_BASE}/bundles`, {
    cache: "no-store", // Ensure real-time policy fetching
  });
  if (!res.ok) {
    throw new Error("Failed to fetch bundles");
  }
  return res.json();
}

export async function createBundle(name: string, description: string) {
  const res = await fetch(`${API_BASE}/bundles`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ name, description }),
  });
  if (!res.ok) {
    throw new Error("Failed to create bundle");
  }
  return res.json();
}

export async function fetchMappingRules() {
  const res = await fetch(`${API_BASE}/rules/mapping`, {
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error("Failed to fetch mapping rules");
  }
  return res.json();
}

export async function createMappingRule(payload: {
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
}) {
  const res = await fetch(`${API_BASE}/rules/mapping`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const errorBody = await res.json().catch(() => ({}));
    throw new Error(errorBody.message || "Failed to create mapping rule");
  }
  return res.json();
}
