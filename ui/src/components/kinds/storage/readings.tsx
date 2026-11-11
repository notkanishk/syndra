import { Fragment } from "react";

import { Mono } from "@/components/ui/Badge";
import { sortNames } from "@/lib/sort";

/**
 * What a storage system's health page says in storage words: its services,
 * the shares nobody audits, what its operations do, and what adopting an
 * account leaves alone. The page draws the readings; the words live here.
 */

/** The sharing services by what they do, not by their process name. */
export function serviceName(service: string): string {
  if (service === "cifs") return "File sharing (SMB)";
  if (service === "nfs") return "NFS sharing";
  return service;
}

/** What a stopped sharing service costs a member. */
export const SERVICE_STOPPED = "Nobody can open their shares while it is stopped";

export const UNAUDITED_LABEL = "SMB auditing is off";

/**
 * Auditing off means activity reports are empty, and an empty report is
 * indistinguishable from a member who did nothing. Said where the operator
 * reads health so it is learned before somebody depends on it.
 */
export function UnauditedShares({ shares, name }: { shares: string[]; name: string }) {
  return (
    <>
      {shares.length === 1 ? "Share" : "Shares"}{" "}
      {sortNames(shares).map((s, i, all) => (
        <Fragment key={s}>
          {i > 0 && (i === all.length - 1 ? " and " : ", ")}
          <Mono>{s}</Mono>
        </Fragment>
      ))}{" "}
      {shares.length === 1 ? "has" : "have"} auditing disabled, so a member&rsquo;s activity report comes back empty
      whether or not they used it. Enable it per share in {name}: Shares → SMB → Edit → Advanced.
    </>
  );
}

/** The operation ids this kind's add-on is known to report, in the board's plain verbs. */
export const STORAGE_OPERATION_LABEL: Record<string, string> = {
  "activity.get": "Reads a share's activity",
};

/** What stays when an existing account is assigned to a person. */
export const ADOPT_STAYS = "Its home folder, its shares and its group memberships, as they are.";
