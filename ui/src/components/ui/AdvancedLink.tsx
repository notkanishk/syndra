"use client";

import Link from "next/link";

import { isAdvancedOnly } from "@/lib/nav";
import { useUiView } from "@/lib/ui-view";

/**
 * A link that knows Basic has no rail row for where it goes (A-20 P10).
 *
 * From Basic to an Advanced-only page it sets the view on the click, so the
 * switch glides and the Advanced rail unfolds while the route changes, and the
 * reader lands with the row lit. Anywhere else it is a plain link: Advanced
 * already has the row, and a page both views share needs no switch. It never
 * says "in Advanced": the view changing is the answer, not a warning.
 */
export function AdvancedLink({ href, onClick, ...rest }: React.ComponentProps<typeof Link>) {
  const { view, setView } = useUiView();
  return (
    <Link
      href={href}
      onClick={(event) => {
        onClick?.(event);
        if (view === "basic" && typeof href === "string" && isAdvancedOnly(href)) setView("advanced");
      }}
      {...rest}
    />
  );
}
