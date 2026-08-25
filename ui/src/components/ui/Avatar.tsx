/**
 * Avatars are CSS gradients standing in for real photos — there are no images
 * anywhere in this system. Initials are derived from the name so a row is
 * still identifiable while the name resolver is in flight.
 */
const SIZES = {
  // type-floor-exempt: initials on an aria-hidden gradient. The name they
  // abbreviate is printed beside them, so nothing here is read — this is a
  // shape, and the floor governs what somebody has to read.
  row: "h-[30px] w-[30px] text-[11px]",
  list: "h-[34px] w-[34px] text-[12px]",
  header: "h-[62px] w-[62px] text-[24px]",
} as const;

export function Avatar({
  name,
  size = "list",
  className = "",
}: {
  name?: string | null;
  size?: keyof typeof SIZES;
  className?: string;
}) {
  return (
    <span
      aria-hidden
      className={`avatar-fill flex flex-none items-center justify-center rounded-pill font-display font-semibold text-ink/75 ${SIZES[size]} ${className}`}
    >
      {initials(name)}
    </span>
  );
}

export function initials(name?: string | null): string {
  if (!name) return "";
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}
