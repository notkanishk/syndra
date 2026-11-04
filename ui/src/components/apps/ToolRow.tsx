/**
 * One settings row of the app page's tools card (A-15 §6, A-17 §2.9): the 170px
 * label seat, then whatever the row says. Sign-ins, Debug mode and History all
 * sit in it, so the three share one seat, padding and wrap instead of each
 * hand-making its own.
 */
export function ToolRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3.5">
      <span className="w-[170px] flex-none text-[14px] font-semibold">{label}</span>
      {children}
    </div>
  );
}
