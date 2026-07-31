import { RequestsScreen } from "@/components/requests/RequestsScreen";
import { getSession } from "@/lib/session";

/**
 * E7 · Requests. Submit (members) and approve/deny (operators), on one route.
 *
 * Members see only their own — enforced at the backend, and the UI matches
 * rather than filtering client-side, because a list filtered in the browser is
 * a list that was still sent to the browser.
 */
export default async function RequestsPage() {
  const session = await getSession();
  if (!session) return null;

  return <RequestsScreen isOperator={session.role === "admin"} userId={session.id} />;
}
