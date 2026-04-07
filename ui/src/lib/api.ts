// Server-side fetches go directly to the backend container
const SERVER_API = "http://backend:8080/api/v1";
// Client-side fetches go through our Next.js proxy route
const CLIENT_API = "/api/proxy";

const API_KEY = process.env.MKAUTH_API_KEY || "";

async function fetchServerJson(path: string) {
  const res = await fetch(`${SERVER_API}${path}`, {
    cache: "no-store",
    headers: { "Authorization": `Bearer ${API_KEY}` },
  });
  if (!res.ok) {
    throw new Error(`Failed to fetch ${path}`);
  }
  return res.json();
}

export function getServerApiBase() {
  return SERVER_API;
}

export function getClientApiBase() {
  return CLIENT_API;
}

// --- Server-side fetchers (used in Server Components) ---

export async function fetchBundles() {
  return fetchServerJson("/bundles");
}

export async function fetchMappingRules() {
  return fetchServerJson("/rules/mapping");
}

export async function fetchCatalog() {
  return fetchServerJson("/catalog");
}

export async function fetchApplications() {
  return fetchServerJson("/applications");
}

export async function fetchProjects() {
  return fetchServerJson("/projects");
}

export async function fetchAudit(limit = 6) {
  return fetchServerJson(`/audit?limit=${limit}`);
}
