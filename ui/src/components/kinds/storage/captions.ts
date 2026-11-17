import { NOTHING_TO_DO } from "@/lib/outcome";

/** What the storage page is for, under its link in the account sheet. */
export const STORAGE_NOTE = "Your password and folders";

/**
 * The totals card's step to Insights, Usage on a storage system (the owner's ruling of 18 Nov 2026,
 * "allow each share here"): a storage word, so it lives with the kind (`kinds/limits.ts` usageStep),
 * and every other kind keeps the usage card's own words. Copy guide section 15.
 */
export const STORAGE_USAGE_STEP = `${NOTHING_TO_DO} Who used each share is in Insights.`;
