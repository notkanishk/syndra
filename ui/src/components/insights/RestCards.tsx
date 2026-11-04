import type { RestCard } from "@/components/insights/useFindings";
import { EmptyState } from "@/components/states";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Card, CardHeader } from "@/components/ui/Card";

const DOT: Record<RestCard["finding"], StatusTone> = { drift: "danger", send: "warn", single: "warn", unused: "neutral", overlap: "neutral" };

/**
 * Below tablet the answer is the page (N20, 20a). At rest the map lists what
 * is worth a look, each with its link: what the finding chips are on a wider
 * screen.
 */
export function RestCards({ cards }: { cards: RestCard[] }) {
  return (
    <Card>
      <CardHeader title="Worth a look" count={cards.length} />
      {cards.length === 0 && <EmptyState resolved title="Nothing needs a look right now." />}
      {cards.map((card) => (
        <div key={card.finding} className="row-divider flex flex-col gap-[5px] px-5 py-3.5">
          <span className="flex items-center gap-2 text-[12.5px] font-semibold text-muted">
            <StatusDot tone={DOT[card.finding]} />
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
