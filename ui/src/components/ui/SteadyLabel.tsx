import type { ReactNode } from "react";

/**
 * An option's label that holds its width when the option is chosen and turns
 * semibold (`label-steady` in globals.css). `text` is the words it shows, all
 * of them, a count included; `children` is how it shows them, when that is
 * more than the words alone. Worn by the options that turn semibold when
 * chosen, FilterPills and Tabs, so choosing one never moves another. Segmented
 * and the section pill are semibold throughout and need no reserve.
 */
export function SteadyLabel({ text, children }: { text: string; children?: ReactNode }) {
  return (
    <span className="label-steady" data-label={text}>
      {/* One item, so words and a count stay on one line in the column. */}
      <span>{children ?? text}</span>
    </span>
  );
}
