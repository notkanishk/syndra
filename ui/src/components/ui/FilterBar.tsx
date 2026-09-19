"use client";

/**
 * The one row every list screen narrows itself from.
 *
 * Before this there were as many filter layouts as there were lists. People put
 * a search box and two selects in the page header, where they stretched to the
 * full width of the actions area and stacked into three tall rows beside the
 * title. Drift put six controls there and pushed its own queue below the fold.
 * Roles, Operations and Requests each arranged their own. Same job, five
 * shapes, and none of them survived a second control being added.
 *
 * The shape, left to right:
 *
 *   [ leading ]  [ search…              ]        [ Filters · 2 ] [ trailing ]
 *
 * `leading` is whatever decides what the page IS — tabs, usually. Search is
 * next because it is the fastest way to the one row somebody wants, and it
 * stays visible for exactly that reason: a search hidden behind a disclosure is
 * a search nobody uses. Everything that narrows rather than finds goes in the
 * panel, behind a button carrying the count of what is on. `trailing` is for
 * actions on the list itself — entering selection mode, exporting — never for
 * more filters.
 *
 * When a screen has ONE narrowing control, it stays on the row: a panel that
 * hides a single four-way toggle costs a click and hides nothing worth hiding.
 * Two or more go behind the button. That is the whole rule.
 *
 * It sits BELOW the page header, not inside it. The header says what the page
 * is and stays the same size on every screen; this row says what you are
 * currently looking at, and only it grows when a screen gains a control.
 */
export function FilterBar({
  leading,
  search,
  filters,
  trailing,
}: {
  leading?: React.ReactNode;
  search?: React.ReactNode;
  /** A `<FilterPanel>`, or nothing when a screen narrows by search alone. */
  filters?: React.ReactNode;
  trailing?: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2.5">
      {leading}
      {search && <div className="min-w-[220px] flex-1 sm:max-w-[340px]">{search}</div>}
      {/* Pushes the controls apart only when there is room; on a phone
          everything wraps into a column and this collapses to nothing. */}
      <span className="hidden flex-1 sm:block" />
      {filters}
      {trailing}
    </div>
  );
}
