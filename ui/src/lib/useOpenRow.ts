"use client";

import { useCallback, useState } from "react";

/**
 * One row open at a time, per list.
 *
 * The constraint is the point. Several open disclosures turn a list into a
 * page of unequal blocks that an operator has to re-find their place in every
 * time one moves, and the row they opened first has usually scrolled away by
 * the time the third is open. Opening a row closes the previous one, so the
 * list only ever has one shape more than it started with.
 *
 * Kept out of `CardRow` deliberately: the primitive is dumb and controlled,
 * because the thing that must be shared is the list's state, not the row's.
 */
export function useOpenRow<Id extends string = string>() {
  const [openId, setOpenId] = useState<Id | null>(null);

  const toggle = useCallback((id: Id) => {
    setOpenId((current) => (current === id ? null : id));
  }, []);

  const isOpen = useCallback((id: Id) => openId === id, [openId]);

  return { openId, isOpen, toggle, closeAll: useCallback(() => setOpenId(null), []) };
}
