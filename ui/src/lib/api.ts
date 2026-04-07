// Server-side fetches go directly to the backend container
const SERVER_API = "http://backend:8080/api/v1";
// Client-side fetches go through our Next.js proxy route
const CLIENT_API = "/api/proxy";

export function getServerApiBase() {
  return SERVER_API;
}

export function getClientApiBase() {
  return CLIENT_API;
}

// --- Server-side fetchers (used in Server Components) ---

export async function fetchBundles() {
  const res = await fetch(`${SERVER_API}/bundles`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch bundles");
  return res.json();
}

export async function fetchMappingRules() {
  const res = await fetch(`${SERVER_API}/rules/mapping`, { cache: "no-store" });
  if (!res.ok) throw new Error("Failed to fetch mapping rules");
  return res.json();
}
