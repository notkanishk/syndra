"use client";

import Link from "next/link";
import { createContext, useContext } from "react";

/**
 * G14: a name that refers to something with its own page links there. The one
 * place the link is drawn, so every name part looks and reads the same: ink at
 * rest, accent and underline on hover (`link-hover`). The words are the link's
 * accessible name; there is never an icon or an extra label.
 *
 * Never put a name part inside another link or button: pass `link={false}` and
 * let the container be the one link. `names-link.guard.test.tsx` fails on both.
 */
export function NameLink({
  href,
  className = "",
  title,
  children,
}: {
  href: string;
  className?: string;
  title?: string;
  children: React.ReactNode;
}) {
  return (
    <Link ref={warnIfNested} href={href} className={`link-hover ${className}`} title={title}>
      {children}
    </Link>
  );
}

/**
 * Inside a dialog a name is plain words: a link there would be the first stop
 * before Cancel, and one tap from walking away mid-decision. `Modal` provides
 * false; an explicit `link` on a part still wins.
 */
export const NameLinksContext = createContext(true);
export const useNameLinks = () => useContext(NameLinksContext);

const INTERACTIVE = "a, button, summary, [role=button], [role=link], [role=menuitem], [role=tab]";

/**
 * The render-time half of the nesting guard: a name that linked inside another
 * link or control says so, in development and in tests, never in production.
 */
export const NESTED_NAME_LINK = "A name link rendered inside another link or control; pass link={false}.";
function warnIfNested(el: HTMLAnchorElement | null) {
  if (el && process.env.NODE_ENV !== "production" && el.parentElement?.closest(INTERACTIVE)) {
    console.error(NESTED_NAME_LINK, el.textContent);
  }
}

/** Where each named thing lives. */
export const nameHref = {
  user: (id: string) => `/users/${encodeURIComponent(id)}`,
  project: (id: string) => `/projects/${encodeURIComponent(id)}`,
  bundle: (id: string) => `/bundles/${encodeURIComponent(id)}`,
};
