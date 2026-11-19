"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useId, useMemo, useRef, useState } from "react";

import { EmptyState } from "@/components/states";
import { Input } from "@/components/ui/Input";
import { Kbd } from "@/components/ui/Kbd";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { flatItems, goTo, recallLately, type GoToItem } from "@/lib/go-to";
import type { NavEntry } from "@/lib/nav";
import { useNameCatalog } from "@/lib/queries/useNameCatalog";
import { useUiView } from "@/lib/ui-view";

/**
 * Go to: one way to reach anything in Syndra by typing its name
 * (proposed/GoTo.md, A-19 P4). Opens pages, never runs a verb; nothing is
 * pinned. With nothing typed it lists Lately, the last places opened on this
 * device, above Pages, or on the phone above the sheet's own list (A-24 §8, Q13);
 * sign-out forgets them. The word "Search" belongs to the per-page
 * search, so it is "Go to" here.
 *
 * It is opened by a key dozens of times a day, so it appears at once and leaves
 * at once (A-24 §8): a still Modal. Escape clears what is typed, then closes.
 * A place it opens does not arrive (Q14).
 */
type GoToProps = {
  open: boolean;
  onClose: () => void;
  /** Which pages may be listed: the current view's nav tree. */
  nav: NavEntry[];
};

/** What the box is for, by who is asking: a member has pages only, Basic has no Bundles page. */
export const GO_TO_PLACEHOLDER = {
  member: "A page",
  basic: "A person, a project, an app, or a page",
  advanced: "A person, a project, an app, a bundle, or a page",
} as const;

/** What Esc does now, said beside its key: with words typed it clears them first. */
const GO_TO_ESC = { typed: "clear, then close", empty: "close" } as const;

export function GoTo({ open, onClose, nav }: GoToProps) {
  return (
    <Modal open={open} onClose={onClose} size="sm" still>
      <ModalHeader title="Go to" />
      <div className="px-6 pb-5">
        <GoToPanel nav={nav} onGo={onClose} />
      </div>
    </Modal>
  );
}

/**
 * The Input and its grouped list. Inside the dialog it is the whole surface;
 * inside the phone's Go to sheet (`embedded`) it sits above the page list,
 * which stays (as `idle`) until something is typed.
 */
export function GoToPanel({
  nav,
  onGo,
  embedded = false,
  idle,
}: {
  nav: NavEntry[];
  /** Called when a row is opened, so the host can close. */
  onGo: () => void;
  embedded?: boolean;
  /** Embedded only: what the sheet shows while the Input is empty. */
  idle?: React.ReactNode;
}) {
  const { audience } = useUiView();
  const { catalog, bundles } = useNameCatalog();
  const here = usePathname();
  // Read once, as it opens: the places opened before this one.
  const [lately] = useState(recallLately);
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const base = useId();
  const listId = `${base}-list`;

  const groups = useMemo(
    () =>
      goTo(
        {
          nav,
          // A member's list is their own pages and nothing else; Basic has no Bundles page.
          names: audience !== "member",
          bundles: audience === "advanced",
          people: catalog?.users,
          projects: catalog?.projects,
          apps: catalog?.applications,
          bundleList: bundles,
          lately,
          here,
        },
        query,
      ),
    [nav, audience, catalog, bundles, query, lately, here],
  );
  const typed = query.trim() !== "";
  const showIdle = embedded && !typed;
  // The phone sheet's own list is its Pages: with nothing typed it keeps Lately above that list.
  const shown = showIdle ? groups.filter(({ group }) => group === "Lately") : groups;
  const items = flatItems(shown);
  const current = items.length ? items[Math.min(active, items.length - 1)] : undefined;
  const optionId = (item: GoToItem) => `${base}-${item.key.replace(/[^\w-]/g, "_")}`;

  const activeRef = useRef<HTMLAnchorElement | null>(null);
  useEffect(() => {
    activeRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [current?.key]);

  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      if (!items.length) return;
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((index) => (Math.min(index, items.length - 1) + step + items.length) % items.length);
    } else if (event.key === "Enter" && current) {
      event.preventDefault();
      // Enter is a press on the selected link, so it navigates exactly as a click does.
      activeRef.current?.click();
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <Input
        type="search"
        autoFocus={!embedded}
        role="combobox"
        aria-label={embedded ? "Go to by name" : "Go to"}
        aria-expanded={items.length > 0}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={current ? optionId(current) : undefined}
        autoComplete="off"
        spellCheck={false}
        placeholder={GO_TO_PLACEHOLDER[audience]}
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
      />

      {(!showIdle || items.length > 0) && (
        <div id={listId} role="listbox" aria-label="Where to go" className="-mx-6 flex flex-col">
          {shown.map(({ group, items: rows }) => (
            <div key={group} role="group" aria-labelledby={`${base}-${group}`} className="flex flex-col">
              <div id={`${base}-${group}`} className="px-6 pb-1 pt-3">
                <span className="type-nav-group">{group}</span>
              </div>
              {rows.map((item) => {
                const selected = item.key === current?.key;
                return (
                  <Link
                    key={item.key}
                    id={optionId(item)}
                    ref={selected ? activeRef : undefined}
                    href={item.href}
                    replace={embedded}
                    role="option"
                    aria-selected={selected}
                    tabIndex={-1}
                    onClick={onGo}
                    className={`motion-tint flex min-h-[var(--tap)] items-center gap-3 px-6 py-1.5 ${selected ? "bg-tint-2" : ""}`}
                  >
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-body font-semibold text-ink">{item.name}</span>
                      {item.descriptor && <span className="truncate text-small text-muted">{item.descriptor}</span>}
                    </span>
                    <span className="flex-none text-fine text-faint">{item.kind}</span>
                  </Link>
                );
              })}
            </div>
          ))}
          {typed && groups.length === 0 && (
            <div className="px-6">
              <EmptyState title="Nothing by that name." />
            </div>
          )}
        </div>
      )}
      {showIdle && idle}

      {!embedded && (
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-fine text-faint">
          <span className="inline-flex items-center gap-1">
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd> move
          </span>
          <span className="inline-flex items-center gap-1">
            <Kbd>Enter</Kbd> open
          </span>
          <span className="inline-flex items-center gap-1">
            <Kbd>Esc</Kbd> {query !== "" ? GO_TO_ESC.typed : GO_TO_ESC.empty}
          </span>
        </p>
      )}
    </div>
  );
}
