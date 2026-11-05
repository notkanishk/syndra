/**
 * A drawn key, for naming a keyboard key in running text (proposed/Kbd.md).
 *
 * Used on the Keys sheet and in Go to's footer, and nowhere else: a button
 * never shows its key. Written as printed on the key: `/`, `Esc`, `Enter`, `↑`.
 */
export function Kbd({ children, className = "" }: { children: string; className?: string }) {
  return (
    <kbd
      className={`inline-flex h-5 min-w-[22px] items-center justify-center rounded-[6px] border border-line-strong bg-tint-1 px-1.5 font-mono text-[12.5px] leading-none text-muted ${className}`}
    >
      {children}
    </kbd>
  );
}

/** The modifier as it is printed on this keyboard: ⌘ on a Mac, Ctrl elsewhere. */
export function modifierKey(): string {
  return typeof navigator !== "undefined" && /mac|iphone|ipad/i.test(navigator.platform || navigator.userAgent) ? "⌘" : "Ctrl";
}
