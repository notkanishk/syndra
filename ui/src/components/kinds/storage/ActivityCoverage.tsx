import { formatList } from "@/lib/format";

/**
 * What a person's activity card says when some shares did not audit them, in
 * storage words, so they live in the storage kind; the card draws the rest.
 */

/** Under "No recorded activity": an empty list that is not the same as nothing having happened. */
export const UNCOVERED_GUIDANCE = "Some shares were not auditing this account, so this is not the same as nothing having happened. See below.";

export function UncoveredNote({ shares, name }: { shares: string[]; name: string }) {
  return (
    <>
      {/* Not "auditing is off". A share can have auditing switched on and
          still record nothing for this person, because TrueNAS scopes it by
          group — so the honest claim is about coverage of THIS account, and
          an operator sent to switch on a setting that is already on would
          find nothing wrong and conclude the gap was imaginary. */}
      Auditing on {formatList(shares)} does not cover {name}, so nothing on {shares.length === 1 ? "that share" : "those shares"} appears
      here whether or not it happened.
    </>
  );
}
