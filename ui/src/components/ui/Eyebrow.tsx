import React from "react";

interface EyebrowProps extends React.HTMLAttributes<HTMLSpanElement> {
  tone?: "default" | "muted" | "primary";
}

/**
 * Uppercase 12px label-cap used as an eyebrow above headlines, on filter
 * sections, and to distinguish source vs derived role rows in the lineage
 * tree. Letter-spacing is 0.1em per the Obsidian Clarity typography contract.
 */
export function Eyebrow({
  tone = "muted",
  className = "",
  children,
  ...props
}: EyebrowProps) {
  const toneClass =
    tone === "primary"
      ? "text-primary-container"
      : tone === "default"
        ? "text-on-surface"
        : "text-on-surface-variant";
  return (
    <span
      className={`inline-block text-[11px] font-semibold uppercase tracking-[0.1em] ${toneClass} ${className}`}
      {...props}
    >
      {children}
    </span>
  );
}
