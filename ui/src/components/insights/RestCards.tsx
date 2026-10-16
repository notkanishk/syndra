import type { RestCard } from "@/components/insights/useFindings";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Card, CardHeader } from "@/components/ui/Card";

const DOT = { drift: "bg-danger", send: "bg-warn", single: "bg-warn", unused: "bg-muted", overlap: "bg-muted" } as const;

/**
 * Below tablet the answer is the page (N20, 20a). At rest the map lists what
 * is worth a look, each with its link: what the finding chips are on a wider
 * screen.
 */
export function RestCards({ cards }: { cards: RestCard[] }) {
  return (
    <Card>
      <CardHeader title="Worth a look" count={cards.length} />
      {cards.length === 0 && (
        <p className="row-divider px-5 py-3.5 text-[13.5px] text-muted">Nothing needs a look right now.</p>
      )}
      {cards.map((card) => (
        <div key={card.finding} className="row-divider flex flex-col gap-[5px] px-5 py-3.5">
          <span className="flex items-center gap-2 text-[12.5px] font-semibold text-muted">
            <span aria-hidden className={`size-2 flex-none rounded-pill ${DOT[card.finding]}`} />
            {card.kind}
          </span>
          <span className="text-[15px] font-semibold [text-wrap:pretty]">{card.title}</span>
          <span className="text-[13px] leading-[1.5] text-muted [text-wrap:pretty]">{card.line}</span>
          <ArrowLink href={card.open.href} className="self-start">
            {card.open.label}
          </ArrowLink>
        </div>
      ))}
    </Card>
  );
}
