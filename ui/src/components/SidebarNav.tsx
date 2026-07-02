"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

import { Badge } from "./ui/Badge";

interface NavItem {
  href: string;
  label: string;
  /** Optional reactive count badge (pending requests, expiring grants). */
  badge?: number;
}

interface NavSection {
  title: string;
  items: NavItem[];
}

const memberSections: NavSection[] = [
  { title: "Portal", items: [{ href: "/", label: "My Services" }] },
  { title: "Access", items: [{ href: "/requests", label: "My Requests" }] },
];

/**
 * SidebarNav handles the nav rail's active-link highlighting and small
 * count badges (e.g., pending requests). Lives client-side so we can read
 * `usePathname()` and refetch governance counts on revalidation. The outer
 * Sidebar remains a server component so it can mount the async
 * SystemModeBadge in the footer.
 */
export default function SidebarNav({ isAdmin }: { isAdmin: boolean }) {
  const pathname = usePathname();
  const [pendingCount, setPendingCount] = useState<number>(0);
  const [expiringCount, setExpiringCount] = useState<number>(0);
  const [propCount, setPropCount] = useState<number>(0);

  useEffect(() => {
    if (!isAdmin) return;
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/api/proxy/governance/summary");
        if (!res.ok) return;
        const data = await res.json();
        if (cancelled) return;
        setPendingCount(Array.isArray(data?.pending_requests) ? data.pending_requests.length : 0);
        setExpiringCount(Array.isArray(data?.expiring_grants) ? data.expiring_grants.length : 0);
        setPropCount(typeof data?.pending_propagation?.count === "number" ? data.pending_propagation.count : 0);
      } catch {
        // Swallow; sidebar must never break the layout.
      }
    })();
    return () => { cancelled = true; };
  }, [isAdmin]);

  const adminSections: NavSection[] = [
    { title: "Dashboard", items: [{ href: "/", label: "Overview" }] },
    {
      title: "Identity",
      items: [
        { href: "/users", label: "Users & Access" },
        { href: "/applications", label: "Applications" },
        { href: "/projects", label: "Projects" },
      ],
    },
    {
      title: "Policy Engine",
      items: [
        { href: "/bundles", label: "Bundles" },
        { href: "/policies", label: "Mapping Rules" },
        { href: "/graph", label: "God Mode" },
      ],
    },
    {
      title: "Governance",
      items: [
        { href: "/audit", label: "Audit Log", badge: expiringCount > 0 ? expiringCount : undefined },
        { href: "/requests", label: "Access Requests", badge: pendingCount > 0 ? pendingCount : undefined },
      ],
    },
    {
      title: "Operations",
      items: [
        { href: "/zitadel", label: "Zitadel Diagnostics" },
        { href: "/governance/pending", label: "Pending", badge: propCount > 0 ? propCount : undefined },
      ],
    },
    {
      title: "Admin",
      items: [
        { href: "/operations", label: "Operations" },
        { href: "/grants", label: "Grants" },
      ],
    },
  ];

  const sections = isAdmin ? adminSections : memberSections;

  return (
    <nav className="flex-1 px-4 space-y-1">
      {sections.map((section, sIdx) => (
        <div key={section.title} className={sIdx > 0 ? "pt-3" : undefined}>
          <p className="px-3 py-1 text-xs font-semibold text-on-surface-variant uppercase tracking-wider">
            {section.title}
          </p>
          {section.items.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                aria-current={active ? "page" : undefined}
                className={`group flex items-center justify-between rounded-md px-3 py-2 text-sm transition-colors ${
                  active
                    ? "bg-primary-container text-on-primary-container border-l-2 border-primary"
                    : "text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high border-l-2 border-transparent"
                }`}
              >
                <span>{item.label}</span>
                {item.badge !== undefined && (
                  <Badge variant={active ? "default" : "secondary"} className="text-[10px]">
                    {item.badge}
                  </Badge>
                )}
              </Link>
            );
          })}
        </div>
      ))}
    </nav>
  );
}
