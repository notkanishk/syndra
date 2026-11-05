"use client";

import Link from "next/link";
import { useEffect, useId, useMemo, useRef, useState } from "react";

import { EmptyState } from "@/components/states";
import { Input } from "@/components/ui/Input";
import { Kbd } from "@/components/ui/Kbd";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { flatItems, goTo, type GoToItem } from "@/lib/go-to";
import type { NavEntry } from "@/lib/nav";
import { useNameCatalog } from "@/lib/queries/useNameCatalog";
import { useUiView } from "@/lib/ui-view";

/**
 * Go to: one way to reach anything in Syndra by typing its name
 * (proposed/GoTo.md, A-19 P4). Opens pages, never runs a verb; nothing is
 * pinned and nothing is remembered. The word "Search" belongs to the per-page
 * search, so it is "Go to" here.
 */
type GoToProps = {
  open: boolean;
  onClose: () => void;
  /** Which pages may be listed: the current view's nav tree. */
  nav: NavEntry[];
};

export function GoTo({ open, onClose, nav }: GoToProps) {
  return (
    <Modal open={open} onClose={onClose} size="sm">
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
        },
        query,
      ),
    [nav, audience, catalog, bundles, query],
  );
  const items = flatItems(groups);
  const typed = query.trim() !== "";
  const showIdle = embedded && !typed;
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
    } else if (event.key === "Enter" && current && !showIdle) {
      event.preventDefault();
      // Enter is a press on the selected link, so it navigates exactly as a click does.
      activeRef.current?.click();
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <Input
        autoFocus={!embedded}
        role="combobox"
        aria-label={embedded ? "Go to by name" : "Go to"}
        aria-expanded={!showIdle && items.length > 0}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={!showIdle && current ? optionId(current) : undefined}
        autoComplete="off"
        spellCheck={false}
        placeholder="A person, a project, an app, a bundle, or a page"
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
          setActive(0);
        }}
        onKeyDown={onKeyDown}
      />

      {showIdle ? (
        idle
      ) : (
        <div id={listId} role="listbox" aria-label="Where to go" className="-mx-6 flex flex-col">
          {groups.map(({ group, items: rows }) => (
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
                      <span className="truncate text-[14.5px] font-semibold text-ink">{item.name}</span>
                      <span className="truncate text-[13px] text-muted">{item.descriptor}</span>
                    </span>
                    <span className="flex-none text-[12.5px] text-faint">{item.kind}</span>
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

      {!embedded && (
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[12.5px] text-faint">
          <span className="inline-flex items-center gap-1">
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd> move
          </span>
          <span className="inline-flex items-center gap-1">
            <Kbd>Enter</Kbd> open
          </span>
          <span className="inline-flex items-center gap-1">
            <Kbd>Esc</Kbd> close
          </span>
        </p>
      )}
    </div>
  );
}
