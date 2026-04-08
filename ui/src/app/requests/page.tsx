import AdminRequestsView from "@/components/requests/AdminRequestsView";
import UserRequestsView from "@/components/requests/UserRequestsView";
import { getSession } from "@/lib/session";

export default async function RequestsPage() {
  const session = await getSession();
  if (!session) {
    return null;
  }

  if (session.role === "admin") {
    return <AdminRequestsView />;
  }

  return <UserRequestsView session={session} />;
}
