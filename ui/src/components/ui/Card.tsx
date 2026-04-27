import React from "react";

type CardVariant = "default" | "glass" | "container";

interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: CardVariant;
}

/**
 * Container surface. Three variants:
 * - `default`: Tailwind's solid surface — used for skeletons + fallback paths.
 * - `glass` (Obsidian Clarity hero surface): translucent surface-container,
 *   28px backdrop blur, ambient lift shadow, 1px white-10% top stroke.
 * - `container`: opaque surface-container fill without shadow — for nested
 *   sub-cards inside a glass surface where another lift would feel busy.
 *
 * All variants share the same border radius and padding so callers can swap
 * without layout drift.
 */
export function Card({ className = "", variant = "glass", children, ...props }: CardProps) {
  const variantClass =
    variant === "glass"
      ? "glass-card"
      : variant === "container"
        ? "bg-surface-container rounded-card"
        : "bg-surface-container rounded-card border border-outline-variant";
  return (
    <div className={`${variantClass} p-6 ${className}`} {...props}>
      {children}
    </div>
  );
}

export function CardHeader({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={`flex flex-col space-y-1.5 mb-4 ${className}`} {...props}>
      {children}
    </div>
  );
}

export function CardTitle({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3
      className={`font-semibold leading-none tracking-tight text-on-surface ${className}`}
      {...props}
    >
      {children}
    </h3>
  );
}
