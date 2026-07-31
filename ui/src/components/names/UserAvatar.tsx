"use client";

import { Avatar } from "@/components/ui/Avatar";
import { useNameResolver } from "@/lib/queries/useNameResolver";

/**
 * An avatar for a user id rather than a name.
 *
 * Rows that render a `<UserName id=…>` were pairing it with `<Avatar name={undefined}>`,
 * which draws an empty gradient disc next to a resolved human name — a blank
 * where the initials belong. Resolving both from the same id keeps them in step.
 */
export function UserAvatar({
  id,
  size = "list",
  className = "",
}: {
  id: string | null | undefined;
  size?: "row" | "list" | "header";
  className?: string;
}) {
  const resolver = useNameResolver();
  const name = id ? resolver.resolveUser(id).value?.display_name : undefined;
  return <Avatar name={name} size={size} className={className} />;
}
