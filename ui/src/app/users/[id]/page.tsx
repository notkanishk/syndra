import { PersonAccess } from "@/components/people/PersonAccess";
import { getSession } from "@/lib/session";

/**
 * E3 · Person → access. The highest-traffic screen in the product.
 *
 * The same route in both views and for both audiences: a member reaching
 * their own record sees it scoped to themselves, and the backend 403s any
 * cross-user read regardless.
 */
export default async function PersonPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const session = await getSession();
  if (!session) return null;

  return <PersonAccess userId={id} isOperator={session.role === "admin"} />;
}
